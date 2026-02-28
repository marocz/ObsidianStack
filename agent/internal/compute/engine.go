package compute

import (
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/obsidianstack/obsidianstack/agent/internal/scraper"
)

// uptimeWindow is the number of recent scrape outcomes tracked for uptime %.
const uptimeWindow = 20

// signalTypes is the ordered set of signal names the engine tracks.
var signalTypes = []string{"metrics", "logs", "traces"}

// Result is the fully-derived health snapshot for one pipeline source,
// ready to be handed to the gRPC shipper (T007).
type Result struct {
	SourceID      string
	SourceType    string
	Timestamp     time.Time
	State         string
	DropPct       float64
	RecoveryRate  float64
	ThroughputPM  float64 // total items/min across all signal types
	StrengthScore float64
	UptimePct     float64
	Signals       []SignalResult
	ErrorMessage  string            // non-empty when the scrape failed; forwarded to the server
	Extra         map[string]float64 // component-specific metrics (e.g. queue_size, exporter_sent_*)
}

// SignalResult is the per-signal-type breakdown included in Result.Signals.
type SignalResult struct {
	Type       string  // "metrics" | "logs" | "traces"
	ReceivedPM float64 // items received per minute
	DroppedPM  float64 // items dropped per minute
	DropPct    float64 // DroppedPM / (ReceivedPM + DroppedPM) * 100
}

// Engine maintains per-source state across scrape cycles and derives health
// metrics from raw ScrapeResult deltas.
//
// All exported methods are safe for concurrent use.
type Engine struct {
	mu     sync.Mutex
	states map[string]*sourceState
}

// NewEngine returns a ready-to-use Engine.
func NewEngine() *Engine {
	return &Engine{states: make(map[string]*sourceState)}
}

// Process ingests a ScrapeResult and returns derived health metrics.
//
// now is passed explicitly so callers (and tests) control the clock without
// sleeping. Use time.Now() in production.
//
// The first call for a source records the baseline counter values and returns
// a Result with State "unknown" — rates cannot be computed without a delta.
func (e *Engine) Process(res *scraper.ScrapeResult, now time.Time) *Result {
	e.mu.Lock()
	defer e.mu.Unlock()

	st := e.stateFor(res.SourceID)
	success := res.Err == nil
	st.recordScrape(success)

	// Always build a base result so callers always get something back.
	out := &Result{
		SourceID:   res.SourceID,
		SourceType: res.SourceType,
		Timestamp:  now,
		UptimePct:  st.uptimePct(),
	}

	if !success {
		slog.Warn("compute: scrape failed, marking unknown",
			"source", res.SourceID, "err", res.Err)
		out.State = StateUnknown
		out.ErrorMessage = res.Err.Error()
		st.updateBaseline(res, now)
		return out
	}

	if !st.hasBaseline {
		// First successful scrape — store counters but return unknown,
		// since we cannot compute any rates yet.
		out.State = StateUnknown
		st.updateBaseline(res, now)
		return out
	}

	elapsed := now.Sub(st.prevTime).Minutes()
	if elapsed <= 0 {
		elapsed = 1 // guard against zero or negative clock drift
	}

	// Derive per-signal deltas and accumulate totals.
	var totalRecvDelta, totalDropDelta float64
	for _, sig := range signalTypes {
		recvDelta := deltaOf(res.Received[sig], st.prev.Received[sig])
		dropDelta := deltaOf(res.Dropped[sig], st.prev.Dropped[sig])

		totalRecvDelta += recvDelta
		totalDropDelta += dropDelta

		recvPM := recvDelta / elapsed
		dropPM := dropDelta / elapsed

		total := recvDelta + dropDelta
		var sigDropPct float64
		if total > 0 {
			sigDropPct = dropDelta / total * 100
		}

		// Only include signals that have seen any traffic.
		if recvDelta > 0 || dropDelta > 0 {
			out.Signals = append(out.Signals, SignalResult{
				Type:       sig,
				ReceivedPM: recvPM,
				DroppedPM:  dropPM,
				DropPct:    sigDropPct,
			})
		}
	}

	totalDelta := totalRecvDelta + totalDropDelta
	if totalDelta > 0 {
		out.DropPct = totalDropDelta / totalDelta * 100
	}
	out.ThroughputPM = totalRecvDelta / elapsed

	// Recovery rate: percentage of pipeline traffic that was NOT dropped.
	// This is a first-order approximation; a future phase can track explicit
	// retry-success counters for a more precise signal.
	out.RecoveryRate = 100 - out.DropPct

	scoreOut := Compute(Input{
		DropPct:      out.DropPct,
		RecoveryRate: out.RecoveryRate,
		UptimePct:    out.UptimePct,
		// LatencyP95ms and BaselineLatencyMs default to 0 until T011 adds
		// latency data; the latency factor then defaults to 1.0 (full credit).
	})
	out.State = scoreOut.State
	out.StrengthScore = scoreOut.Score

	// Compute per-minute rates for Extra counter fields; copy gauges as-is.
	// Convention: fields ending in "_size" or "_capacity" are gauges (current
	// value). Everything else is a monotonic counter — compute delta/elapsed.
	if len(res.Extra) > 0 {
		out.Extra = make(map[string]float64, len(res.Extra)*2)
		for k, v := range res.Extra {
			if strings.HasSuffix(k, "_size") || strings.HasSuffix(k, "_capacity") {
				out.Extra[k] = v
			} else {
				var prev float64
				if st.prev != nil {
					prev = st.prev.Extra[k]
				}
				out.Extra[k+"_pm"] = deltaOf(v, prev) / elapsed
			}
		}
	}

	st.updateBaseline(res, now)
	return out
}

// sourceState holds per-source counters and uptime history.
type sourceState struct {
	prev        *scraper.ScrapeResult
	prevTime    time.Time
	hasBaseline bool
	history     []bool // circular buffer of scrape outcomes, newest last
}

func (e *Engine) stateFor(id string) *sourceState {
	if st, ok := e.states[id]; ok {
		return st
	}
	st := &sourceState{}
	e.states[id] = st
	return st
}

func (st *sourceState) updateBaseline(res *scraper.ScrapeResult, now time.Time) {
	if res.Err == nil {
		st.prev = res
		st.prevTime = now
		st.hasBaseline = true
	}
}

func (st *sourceState) recordScrape(success bool) {
	if len(st.history) >= uptimeWindow {
		st.history = st.history[1:]
	}
	st.history = append(st.history, success)
}

func (st *sourceState) uptimePct() float64 {
	if len(st.history) == 0 {
		return 100 // assume up before first observation
	}
	var ok int
	for _, s := range st.history {
		if s {
			ok++
		}
	}
	return float64(ok) / float64(len(st.history)) * 100
}

// deltaOf returns the positive counter delta between current and previous.
// If current < previous (counter reset after restart), returns 0.
func deltaOf(current, previous float64) float64 {
	d := current - previous
	if d < 0 {
		return 0
	}
	return d
}
