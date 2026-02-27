package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

// deliver sends webhook notifications for a to all configured targets.
// Errors are logged but do not affect the caller.
func (e *Engine) deliver(a *Alert) {
	for _, wh := range e.webhooks {
		url := wh.URL()
		if url == "" {
			continue
		}

		var err error
		switch wh.Type {
		case "slack":
			err = e.sendSlack(url, a)
		case "teams":
			err = e.sendTeams(url, a)
		case "pagerduty", "http":
			err = e.sendHTTP(url, a)
		default:
			slog.Warn("alerts: unknown webhook type — skipping", "type", wh.Type)
			continue
		}

		if err != nil {
			slog.Error("alerts: webhook delivery failed",
				"type", wh.Type,
				"rule", a.RuleName,
				"err", err,
			)
		} else {
			slog.Debug("alerts: webhook delivered",
				"type", wh.Type,
				"rule", a.RuleName,
				"state", a.State,
			)
		}
	}
}

func (e *Engine) sendSlack(url string, a *Alert) error {
	body, _ := json.Marshal(map[string]string{
		"text": fmt.Sprintf("*%s* %s", severityLabel(a.Severity), a.Message),
	})
	return e.post(url, body)
}

func (e *Engine) sendTeams(url string, a *Alert) error {
	payload := map[string]interface{}{
		"@type":      "MessageCard",
		"@context":   "http://schema.org/extensions",
		"themeColor": severityColor(a.Severity),
		"summary":    a.RuleName,
		"title":      fmt.Sprintf("ObsidianStack Alert: %s", a.RuleName),
		"text":       a.Message,
	}
	body, _ := json.Marshal(payload)
	return e.post(url, body)
}

func (e *Engine) sendHTTP(url string, a *Alert) error {
	body, _ := json.Marshal(map[string]interface{}{"alert": a})
	return e.post(url, body)
}

func (e *Engine) post(url string, body []byte) error {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func severityLabel(s string) string {
	switch s {
	case "critical":
		return "[CRITICAL]"
	case "warning":
		return "[WARNING]"
	default:
		return "[INFO]"
	}
}

func severityColor(s string) string {
	switch s {
	case "critical":
		return "FF4F6A"
	case "warning":
		return "FFAB40"
	default:
		return "00D4FF"
	}
}
