package alerts

import (
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	pb "github.com/obsidianstack/obsidianstack/gen/obsidian/v1"

	"github.com/obsidianstack/obsidianstack/server/internal/config"
)

const (
	defaultCooldown    = 15 * time.Minute
	maxHistoryLen      = 200
	recentWindowHours  = 1
)

// Alert represents a single alert event produced by the rule engine.
type Alert struct {
	ID         string     `json:"id"`
	RuleName   string     `json:"rule_name"`
	SourceID   string     `json:"source_id"`
	Severity   string     `json:"severity"`
	Message    string     `json:"message"`
	Value      float64    `json:"value"`
	FiredAt    time.Time  `json:"fired_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
	State      string     `json:"state"` // "firing" | "resolved"
}

// Engine evaluates alert rules against incoming PipelineSnapshots and delivers
// webhook notifications when rules fire or resolve.
//
// Engine is safe for concurrent use.
type Engine struct {
	rules    []config.AlertRule
	webhooks []config.WebhookConfig

	mu       sync.Mutex
	active   map[string]*Alert   // key: "ruleName:sourceID"
	lastFire map[string]time.Time // last fire time per key (for cooldown)
	history  []*Alert             // recently resolved alerts
	client   *http.Client
}

// New creates an Engine from the server alert configuration.
// An Engine with empty rules is valid — Evaluate becomes a no-op.
func New(cfg config.AlertsConfig) *Engine {
	return &Engine{
		rules:    cfg.Rules,
		webhooks: cfg.Webhooks,
		active:   make(map[string]*Alert),
		lastFire: make(map[string]time.Time),
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

// Evaluate tests all configured rules against snap.
// Alerts that fire are stored and webhook delivery is triggered asynchronously.
// Alerts that were firing but whose condition is now false are resolved.
func (e *Engine) Evaluate(snap *pb.PipelineSnapshot) {
	if len(e.rules) == 0 {
		return
	}

	now := time.Now()
	for _, rule := range e.rules {
		key := rule.Name + ":" + snap.SourceId
		fires, value := evalCondition(rule.Condition, snap)

		e.mu.Lock()

		if fires {
			cooldown := rule.Cooldown
			if cooldown <= 0 {
				cooldown = defaultCooldown
			}
			if now.Sub(e.lastFire[key]) > cooldown {
				sev := rule.Severity
				if sev == "" {
					sev = "warning"
				}
				a := &Alert{
					ID:       fmt.Sprintf("%s:%s:%d", rule.Name, snap.SourceId, now.UnixNano()),
					RuleName: rule.Name,
					SourceID: snap.SourceId,
					Severity: sev,
					Value:    value,
					Message: fmt.Sprintf("[%s] %s fired on %s — %s = %.2f",
						sev, rule.Name, snap.SourceId, rule.Condition, value),
					FiredAt: now,
					State:   "firing",
				}
				e.active[key] = a
				e.lastFire[key] = now
				alertCopy := *a
				e.mu.Unlock()

				slog.Warn("alert fired",
					"rule", rule.Name,
					"source", snap.SourceId,
					"value", value,
					"severity", sev,
				)
				go e.deliver(&alertCopy)
			} else {
				e.mu.Unlock()
			}
		} else {
			if a, ok := e.active[key]; ok && a.State == "firing" {
				resolved := now
				a.State = "resolved"
				a.ResolvedAt = &resolved
				delete(e.active, key)

				e.history = append(e.history, a)
				if len(e.history) > maxHistoryLen {
					e.history = e.history[len(e.history)-maxHistoryLen:]
				}
				alertCopy := *a
				e.mu.Unlock()

				slog.Info("alert resolved",
					"rule", rule.Name,
					"source", snap.SourceId,
				)
				go e.deliver(&alertCopy)
			} else {
				e.mu.Unlock()
			}
		}
	}
}

// Active returns copies of all currently firing alerts plus any alerts
// resolved within the past hour, sorted newest first.
func (e *Engine) Active() []*Alert {
	e.mu.Lock()
	defer e.mu.Unlock()

	cutoff := time.Now().Add(-recentWindowHours * time.Hour)
	out := make([]*Alert, 0, len(e.active))

	for _, a := range e.active {
		cp := *a
		out = append(out, &cp)
	}
	for _, a := range e.history {
		if a.ResolvedAt != nil && a.ResolvedAt.After(cutoff) {
			cp := *a
			out = append(out, &cp)
		}
	}
	return out
}
