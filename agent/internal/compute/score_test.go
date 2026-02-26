package compute

import (
	"math"
	"testing"
)

// almostEqual returns true if a and b are within epsilon of each other.
func almostEqual(a, b, epsilon float64) bool {
	return math.Abs(a-b) < epsilon
}

// --- Compute() table-driven tests ---

func TestCompute_States(t *testing.T) {
	tests := []struct {
		name      string
		in        Input
		wantState string
		wantScore float64 // approximate; use -1 to skip
	}{
		{
			name:      "perfect pipeline — no drops, full uptime",
			in:        Input{DropPct: 0, RecoveryRate: 100, UptimePct: 100},
			wantState: StateHealthy,
			wantScore: 100,
		},
		{
			name:      "healthy threshold — exactly 85",
			in:        Input{DropPct: 0, RecoveryRate: 75, UptimePct: 100},
			wantState: StateHealthy,
			// drop=1.0*0.4 + lat=1.0*0.3 + rec=0.75*0.2 + up=1.0*0.1 = 0.4+0.3+0.15+0.1 = 0.95 → 95
			wantScore: 95,
		},
		{
			name: "degraded — score lands between 60 and 84",
			// drop=5% → drop_factor=0.95; rec=80; up=90; no latency
			// score = (0.95*0.4 + 1.0*0.3 + 0.80*0.2 + 0.90*0.1) * 100
			//       = (0.38 + 0.30 + 0.16 + 0.09) * 100 = 83.0
			in:        Input{DropPct: 5, RecoveryRate: 80, UptimePct: 90},
			wantState: StateDegraded,
			wantScore: 83,
		},
		{
			name: "critical — high drop rate",
			// drop=40% → drop_factor=0.60; rec=60; up=70
			// score = (0.60*0.4 + 1.0*0.3 + 0.60*0.2 + 0.70*0.1) * 100
			//       = (0.24 + 0.30 + 0.12 + 0.07) * 100 = 73.0 → degraded?
			// Let's use drop=60% to push into critical:
			// score = (0.40*0.4 + 1.0*0.3 + 0.40*0.2 + 0.50*0.1) * 100
			//       = (0.16 + 0.30 + 0.08 + 0.05) * 100 = 59.0
			in:        Input{DropPct: 60, RecoveryRate: 40, UptimePct: 50},
			wantState: StateCritical,
			wantScore: 59,
		},
		{
			name:      "unknown — no data at all",
			in:        Input{DropPct: 0, RecoveryRate: 0, UptimePct: 0},
			wantState: StateUnknown,
			wantScore: -1,
		},
		{
			name:      "score exactly 85 — boundary healthy",
			in:        Input{DropPct: 0, RecoveryRate: 50, UptimePct: 100},
			wantState: StateHealthy,
			// (1.0*0.4 + 1.0*0.3 + 0.5*0.2 + 1.0*0.1)*100 = (0.4+0.3+0.1+0.1)*100 = 90
			wantScore: 90,
		},
		{
			name: "score exactly 60 — boundary degraded",
			// Need score = 60: try drop=100%, rec=100%, up=100%
			// (0.0*0.4 + 1.0*0.3 + 1.0*0.2 + 1.0*0.1)*100 = 60
			in:        Input{DropPct: 100, RecoveryRate: 100, UptimePct: 100},
			wantState: StateDegraded,
			wantScore: 60,
		},
		{
			name: "score just below 60 — critical",
			// drop=100%, rec=50%, up=100%
			// (0.0*0.4 + 1.0*0.3 + 0.5*0.2 + 1.0*0.1)*100 = (0+0.3+0.1+0.1)*100 = 50
			in:        Input{DropPct: 100, RecoveryRate: 50, UptimePct: 100},
			wantState: StateCritical,
			wantScore: 50,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := Compute(tc.in)

			if out.State != tc.wantState {
				t.Errorf("State = %q, want %q (score=%.2f)", out.State, tc.wantState, out.Score)
			}
			if tc.wantScore >= 0 && !almostEqual(out.Score, tc.wantScore, 0.01) {
				t.Errorf("Score = %.4f, want %.4f", out.Score, tc.wantScore)
			}
		})
	}
}

func TestCompute_LatencyFactor(t *testing.T) {
	tests := []struct {
		name              string
		p95ms, baselineMs float64
		wantLatFactor     float64
	}{
		{"no baseline — full credit", 500, 0, 1.0},
		{"at baseline — full credit", 100, 100, 0.0},   // 100/100 = 1.0, factor = 1-1 = 0
		{"half baseline — half credit", 50, 100, 0.5},  // 50/100 = 0.5, factor = 1-0.5
		{"well under baseline", 10, 100, 0.9},
		{"exceeds baseline — clamped to 0", 200, 100, 0.0}, // capped at 1 before inversion
		{"far exceeds — still 0", 10000, 100, 0.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := Compute(Input{
				DropPct:           0,
				RecoveryRate:      100,
				UptimePct:         100,
				LatencyP95ms:      tc.p95ms,
				BaselineLatencyMs: tc.baselineMs,
			})
			if !almostEqual(out.LatencyFactor, tc.wantLatFactor, 0.001) {
				t.Errorf("LatencyFactor = %.4f, want %.4f", out.LatencyFactor, tc.wantLatFactor)
			}
		})
	}
}

func TestCompute_FactorClamping(t *testing.T) {
	// Inputs outside 0-100 should be clamped, not cause negative scores.
	t.Run("drop_pct > 100 clamped", func(t *testing.T) {
		out := Compute(Input{DropPct: 150, RecoveryRate: 100, UptimePct: 100})
		if out.DropFactor != 0 {
			t.Errorf("DropFactor with drop_pct=150 = %.4f, want 0", out.DropFactor)
		}
		if out.Score < 0 {
			t.Errorf("Score should not be negative, got %.4f", out.Score)
		}
	})

	t.Run("recovery_rate > 100 clamped to 1.0 factor", func(t *testing.T) {
		out := Compute(Input{DropPct: 0, RecoveryRate: 200, UptimePct: 100})
		if out.RecoveryFactor != 1.0 {
			t.Errorf("RecoveryFactor with recovery=200 = %.4f, want 1.0", out.RecoveryFactor)
		}
	})

	t.Run("uptime_pct > 100 clamped to 1.0 factor", func(t *testing.T) {
		out := Compute(Input{DropPct: 0, RecoveryRate: 100, UptimePct: 120})
		if out.UptimeFactor != 1.0 {
			t.Errorf("UptimeFactor with uptime=120 = %.4f, want 1.0", out.UptimeFactor)
		}
	})
}

func TestCompute_ScoreInRange(t *testing.T) {
	// Property test: score is always in [0, 100] for any valid-ish inputs.
	cases := []Input{
		{DropPct: 0, RecoveryRate: 100, UptimePct: 100},
		{DropPct: 100, RecoveryRate: 0, UptimePct: 0},
		{DropPct: 50, RecoveryRate: 50, UptimePct: 50},
		{DropPct: 0.001, RecoveryRate: 99.999, UptimePct: 99.9},
		{DropPct: 99.9, RecoveryRate: 0.1, UptimePct: 1},
	}
	for _, in := range cases {
		out := Compute(in)
		if out.Score < 0 || out.Score > 100 {
			t.Errorf("Score %.4f out of [0,100] for input %+v", out.Score, in)
		}
	}
}

func TestCompute_FactorsSumToScore(t *testing.T) {
	// Verify the factors reconstruct the score correctly.
	in := Input{
		DropPct:           10,
		RecoveryRate:      85,
		UptimePct:         95,
		LatencyP95ms:      120,
		BaselineLatencyMs: 200,
	}
	out := Compute(in)
	reconstructed := (out.DropFactor*weightDrop +
		out.LatencyFactor*weightLatency +
		out.RecoveryFactor*weightRecovery +
		out.UptimeFactor*weightUptime) * 100

	if !almostEqual(out.Score, reconstructed, 0.0001) {
		t.Errorf("Score %.6f != reconstructed %.6f from factors", out.Score, reconstructed)
	}
}

// --- clamp01 ---

func TestClamp01(t *testing.T) {
	tests := []struct{ in, want float64 }{
		{-1, 0}, {0, 0}, {0.5, 0.5}, {1, 1}, {1.5, 1},
	}
	for _, tc := range tests {
		if got := clamp01(tc.in); got != tc.want {
			t.Errorf("clamp01(%.2f) = %.2f, want %.2f", tc.in, got, tc.want)
		}
	}
}

// --- stateFromScore ---

func TestStateFromScore(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{100, StateHealthy},
		{85, StateHealthy},
		{84.99, StateDegraded},
		{60, StateDegraded},
		{59.99, StateCritical},
		{0, StateCritical},
	}
	for _, tc := range tests {
		got := stateFromScore(tc.score)
		if got != tc.want {
			t.Errorf("stateFromScore(%.2f) = %q, want %q", tc.score, got, tc.want)
		}
	}
}
