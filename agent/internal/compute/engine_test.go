package compute

import (
	"errors"
	"testing"
	"time"

	"github.com/obsidianstack/obsidianstack/agent/internal/scraper"
)

// baseTime is a fixed reference point so all test timings are deterministic.
var baseTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// tick returns baseTime advanced by n minutes.
func tick(n int) time.Time {
	return baseTime.Add(time.Duration(n) * time.Minute)
}

// makeResult builds a minimal ScrapeResult for the given source and counters.
func makeResult(id, typ string, recv, drop map[string]float64) *scraper.ScrapeResult {
	return &scraper.ScrapeResult{
		SourceID:   id,
		SourceType: typ,
		ScrapedAt:  baseTime,
		Received:   recv,
		Dropped:    drop,
		Extra:      map[string]float64{},
	}
}

// --- First scrape behaviour ---

func TestEngine_FirstScrape_ReturnsUnknown(t *testing.T) {
	e := NewEngine()
	res := makeResult("otel-1", "otelcol",
		map[string]float64{"traces": 1000},
		map[string]float64{"traces": 0},
	)
	out := e.Process(res, tick(0))
	if out.State != StateUnknown {
		t.Errorf("first scrape State = %q, want %q", out.State, StateUnknown)
	}
}

// --- Rate computation from deltas ---

func TestEngine_SecondScrape_ComputesRates(t *testing.T) {
	e := NewEngine()

	// First scrape: establish baseline.
	e.Process(makeResult("otel-1", "otelcol",
		map[string]float64{"traces": 10000, "metrics": 5000},
		map[string]float64{"traces": 0, "metrics": 0},
	), tick(0))

	// Second scrape 1 minute later: 500 new traces received, 50 dropped.
	out := e.Process(makeResult("otel-1", "otelcol",
		map[string]float64{"traces": 10500, "metrics": 5100},
		map[string]float64{"traces": 50, "metrics": 10},
	), tick(1))

	if out.State == StateUnknown {
		t.Fatalf("second scrape should not be unknown, got %q", out.State)
	}

	// DropPct = (50+10) / (500+100+50+10) * 100 = 60/660 ≈ 9.09%
	wantDropPct := 60.0 / 660.0 * 100
	if !almostEqual(out.DropPct, wantDropPct, 0.01) {
		t.Errorf("DropPct = %.4f, want %.4f", out.DropPct, wantDropPct)
	}

	// ThroughputPM = (500+100) / 1 min = 600
	if !almostEqual(out.ThroughputPM, 600, 0.01) {
		t.Errorf("ThroughputPM = %.2f, want 600", out.ThroughputPM)
	}

	// Should have signals for both traces and metrics.
	if len(out.Signals) != 2 {
		t.Errorf("Signals len = %d, want 2", len(out.Signals))
	}
}

func TestEngine_ThroughputScalesWithElapsed(t *testing.T) {
	e := NewEngine()

	e.Process(makeResult("src", "prometheus",
		map[string]float64{"metrics": 0},
		map[string]float64{"metrics": 0},
	), tick(0))

	// 2 minutes later, 600 new samples.
	out := e.Process(makeResult("src", "prometheus",
		map[string]float64{"metrics": 600},
		map[string]float64{"metrics": 0},
	), tick(2))

	// ThroughputPM = 600 / 2 min = 300/min
	if !almostEqual(out.ThroughputPM, 300, 0.01) {
		t.Errorf("ThroughputPM over 2 min = %.2f, want 300", out.ThroughputPM)
	}
}

// --- Counter reset handling ---

func TestEngine_CounterReset_TreatedAsZeroDelta(t *testing.T) {
	e := NewEngine()

	// Baseline with high counter values.
	e.Process(makeResult("src", "otelcol",
		map[string]float64{"traces": 100000},
		map[string]float64{"traces": 500},
	), tick(0))

	// Counter reset (OTel Collector restarted — counters start from 0).
	out := e.Process(makeResult("src", "otelcol",
		map[string]float64{"traces": 50},   // lower than baseline
		map[string]float64{"traces": 2},
	), tick(1))

	// Delta = max(0, 50-100000) = 0 for received.
	// Drop delta = max(0, 2-500) = 0.
	// So DropPct = 0 and no signals with traffic.
	if out.DropPct != 0 {
		t.Errorf("DropPct after counter reset = %.4f, want 0", out.DropPct)
	}
	if len(out.Signals) != 0 {
		t.Errorf("Signals after counter reset: got %d, want 0 (no traffic delta)", len(out.Signals))
	}
}

// --- Scrape failure handling ---

func TestEngine_ScrapeFailure_ReturnsUnknown(t *testing.T) {
	e := NewEngine()

	// Establish baseline.
	e.Process(makeResult("src", "loki",
		map[string]float64{"logs": 5000},
		map[string]float64{"logs": 10},
	), tick(0))

	// Scrape fails.
	failed := &scraper.ScrapeResult{
		SourceID:   "src",
		SourceType: "loki",
		ScrapedAt:  baseTime,
		Received:   map[string]float64{},
		Dropped:    map[string]float64{},
		Extra:      map[string]float64{},
		Err:        errors.New("connection refused"),
	}
	out := e.Process(failed, tick(1))

	if out.State != StateUnknown {
		t.Errorf("scrape failure State = %q, want %q", out.State, StateUnknown)
	}
}

func TestEngine_ScrapeFailure_DoesNotAdvanceBaseline(t *testing.T) {
	e := NewEngine()

	// Baseline at t=0: 0 received.
	e.Process(makeResult("src", "otelcol",
		map[string]float64{"traces": 1000},
		map[string]float64{"traces": 0},
	), tick(0))

	// Failed scrape at t=1 — baseline should NOT advance.
	failed := &scraper.ScrapeResult{
		SourceID: "src", SourceType: "otelcol",
		Received: map[string]float64{}, Dropped: map[string]float64{},
		Extra: map[string]float64{},
		Err:   errors.New("timeout"),
	}
	e.Process(failed, tick(1))

	// Recovery at t=2: 500 new items since t=0 (baseline should still be t=0 state).
	out := e.Process(makeResult("src", "otelcol",
		map[string]float64{"traces": 1500},
		map[string]float64{"traces": 0},
	), tick(2))

	// elapsed = 2 min, delta = 500 → throughput = 250/min
	if !almostEqual(out.ThroughputPM, 250, 0.1) {
		t.Errorf("ThroughputPM after recovery = %.2f, want 250 (baseline not advanced by failure)", out.ThroughputPM)
	}
}

// --- Uptime tracking ---

func TestEngine_UptimePct_AllSuccess(t *testing.T) {
	e := NewEngine()
	for i := 0; i < 5; i++ {
		out := e.Process(makeResult("src", "otelcol",
			map[string]float64{"traces": float64(i * 100)},
			map[string]float64{},
		), tick(i))
		_ = out
	}
	// All 5 scrapes succeeded.
	last := e.Process(makeResult("src", "otelcol",
		map[string]float64{"traces": 500},
		map[string]float64{},
	), tick(5))
	if last.UptimePct != 100 {
		t.Errorf("UptimePct all success = %.2f, want 100", last.UptimePct)
	}
}

func TestEngine_UptimePct_HalfFailed(t *testing.T) {
	e := NewEngine()
	for i := 0; i < 4; i++ {
		e.Process(makeResult("src", "otelcol",
			map[string]float64{"traces": float64(i * 100)},
			map[string]float64{},
		), tick(i))
	}

	// 4 successes so far; add 4 failures.
	for i := 0; i < 4; i++ {
		failed := &scraper.ScrapeResult{
			SourceID: "src", SourceType: "otelcol",
			Received: map[string]float64{}, Dropped: map[string]float64{},
			Extra: map[string]float64{},
			Err:   errors.New("timeout"),
		}
		e.Process(failed, tick(4+i))
	}

	last := e.Process(makeResult("src", "otelcol",
		map[string]float64{"traces": 900},
		map[string]float64{},
	), tick(8))

	// 5 success out of 9 = 55.56%
	wantUptime := 5.0 / 9.0 * 100
	if !almostEqual(last.UptimePct, wantUptime, 0.1) {
		t.Errorf("UptimePct half-failed = %.2f, want %.2f", last.UptimePct, wantUptime)
	}
}

func TestEngine_UptimePct_RollingWindow(t *testing.T) {
	e := NewEngine()

	// Fill beyond the window size with failures.
	for i := 0; i < uptimeWindow+5; i++ {
		failed := &scraper.ScrapeResult{
			SourceID: "src", SourceType: "otelcol",
			Received: map[string]float64{}, Dropped: map[string]float64{},
			Extra: map[string]float64{},
			Err:   errors.New("down"),
		}
		e.Process(failed, tick(i))
	}

	// Now add 5 successes.
	for i := 0; i < 5; i++ {
		e.Process(makeResult("src", "otelcol",
			map[string]float64{"traces": float64((uptimeWindow + 5 + i) * 10)},
			map[string]float64{},
		), tick(uptimeWindow+5+i))
	}

	last := e.Process(makeResult("src", "otelcol",
		map[string]float64{"traces": 999},
		map[string]float64{},
	), tick(uptimeWindow+11))

	// Window holds the last uptimeWindow=20 scrapes.
	// 15 failures + 5 successes + 1 final success = 6 successes / 20 = 30%
	wantUptime := 6.0 / float64(uptimeWindow) * 100
	if !almostEqual(last.UptimePct, wantUptime, 0.5) {
		t.Errorf("UptimePct rolling = %.2f, want %.2f", last.UptimePct, wantUptime)
	}
}

// --- Multiple independent sources ---

func TestEngine_MultiSource_Independent(t *testing.T) {
	e := NewEngine()

	// Establish baselines for two sources.
	e.Process(makeResult("otel", "otelcol",
		map[string]float64{"traces": 1000}, map[string]float64{"traces": 0},
	), tick(0))
	e.Process(makeResult("prom", "prometheus",
		map[string]float64{"metrics": 5000}, map[string]float64{"metrics": 0},
	), tick(0))

	// OTel gets 500 traces with 50 dropped.
	otelOut := e.Process(makeResult("otel", "otelcol",
		map[string]float64{"traces": 1500}, map[string]float64{"traces": 50},
	), tick(1))

	// Prometheus gets 1000 samples with no drops.
	promOut := e.Process(makeResult("prom", "prometheus",
		map[string]float64{"metrics": 6000}, map[string]float64{"metrics": 0},
	), tick(1))

	// OTel: drop = 50/(500+50)*100 ≈ 9.09%
	wantOTelDrop := 50.0 / 550.0 * 100
	if !almostEqual(otelOut.DropPct, wantOTelDrop, 0.01) {
		t.Errorf("otel DropPct = %.4f, want %.4f", otelOut.DropPct, wantOTelDrop)
	}

	// Prometheus: no drops.
	if promOut.DropPct != 0 {
		t.Errorf("prom DropPct = %.4f, want 0", promOut.DropPct)
	}
}

// --- Strength score integration ---

func TestEngine_PerfectPipeline_HealthyScore(t *testing.T) {
	e := NewEngine()

	e.Process(makeResult("src", "otelcol",
		map[string]float64{"traces": 0}, map[string]float64{"traces": 0},
	), tick(0))

	// 1000 received, 0 dropped → perfect.
	out := e.Process(makeResult("src", "otelcol",
		map[string]float64{"traces": 1000}, map[string]float64{"traces": 0},
	), tick(1))

	if out.State != StateHealthy {
		t.Errorf("perfect pipeline State = %q, want %q (score=%.2f)", out.State, StateHealthy, out.StrengthScore)
	}
	if out.StrengthScore < 85 {
		t.Errorf("perfect pipeline StrengthScore = %.2f, want >= 85", out.StrengthScore)
	}
}

func TestEngine_HighDropRate_CriticalScore(t *testing.T) {
	e := NewEngine()

	e.Process(makeResult("src", "loki",
		map[string]float64{"logs": 0}, map[string]float64{"logs": 0},
	), tick(0))

	// 100 received, 900 dropped → 90% drop rate.
	out := e.Process(makeResult("src", "loki",
		map[string]float64{"logs": 100}, map[string]float64{"logs": 900},
	), tick(1))

	if out.State != StateCritical {
		t.Errorf("90%% drop rate State = %q, want %q (score=%.2f)", out.State, StateCritical, out.StrengthScore)
	}
}

// --- deltaOf ---

func TestDeltaOf(t *testing.T) {
	tests := []struct {
		curr, prev, want float64
	}{
		{100, 50, 50},    // normal increment
		{50, 50, 0},      // no change
		{30, 50, 0},      // counter reset → 0, not -20
		{0, 0, 0},        // all zero
		{1000, 0, 1000},  // first reading after reset
	}
	for _, tc := range tests {
		got := deltaOf(tc.curr, tc.prev)
		if got != tc.want {
			t.Errorf("deltaOf(%.0f, %.0f) = %.0f, want %.0f", tc.curr, tc.prev, got, tc.want)
		}
	}
}
