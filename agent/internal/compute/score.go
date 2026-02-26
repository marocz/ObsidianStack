package compute

// Weight constants for the strength score formula.
// They must sum to 1.0.
const (
	weightDrop     = 0.40
	weightLatency  = 0.30
	weightRecovery = 0.20
	weightUptime   = 0.10
)

// State constants returned by the score calculator.
const (
	StateHealthy  = "healthy"
	StateDegraded = "degraded"
	StateCritical = "critical"
	StateUnknown  = "unknown"
)

// Thresholds that map a score to a health state.
const (
	ThresholdHealthy  = 85.0
	ThresholdDegraded = 60.0
)

// Input holds the normalised values fed into the strength score formula.
// All percentage fields are in the range 0–100.
type Input struct {
	// DropPct is the percentage of pipeline items that were dropped.
	// 0 = no drops (perfect), 100 = everything dropped.
	DropPct float64

	// LatencyP95ms is the observed P95 export latency in milliseconds.
	// Set to 0 if no latency data is available.
	LatencyP95ms float64

	// BaselineLatencyMs is the expected / acceptable P95 latency.
	// When non-zero, the latency factor is 1 - clamp(P95/Baseline, 0, 1).
	// When zero, the latency factor defaults to 1.0 (full credit — no penalty).
	BaselineLatencyMs float64

	// RecoveryRate is the percentage of pipeline operations that succeeded
	// after accounting for retries and backpressure resolution.
	// 100 = everything recovered, 0 = nothing recovered.
	RecoveryRate float64

	// UptimePct is the percentage of recent scrape cycles that returned
	// valid data. 100 = always reachable, 0 = never reachable.
	UptimePct float64
}

// Output is the result of the strength score calculation.
type Output struct {
	// Score is the composite health score in the range 0–100.
	Score float64

	// State is the health state derived from Score.
	// One of: "healthy", "degraded", "critical", "unknown".
	State string

	// The four factor values (each 0–1) used to compute Score.
	// Useful for rendering per-dimension breakdowns in the UI.
	DropFactor     float64
	LatencyFactor  float64
	RecoveryFactor float64
	UptimeFactor   float64
}

// Compute calculates the pipeline strength score from the given inputs.
//
// Formula (per ARCHITECTURE.md):
//
//	score = (
//	    (1 - drop_pct/100)      * 0.40  +
//	    (1 - latency_ratio)     * 0.30  +   // latency_ratio = P95/baseline, capped at 1
//	    recovery_rate/100       * 0.20  +
//	    uptime_pct/100          * 0.10
//	) * 100
//
// If UptimePct is 0 and there is no signal data, the state is "unknown".
func Compute(in Input) Output {
	// No data at all → unknown.
	if in.UptimePct == 0 && in.DropPct == 0 && in.RecoveryRate == 0 {
		return Output{State: StateUnknown}
	}

	dropFactor := 1 - clamp01(in.DropPct/100)

	latencyFactor := 1.0
	if in.BaselineLatencyMs > 0 {
		latencyFactor = 1 - clamp01(in.LatencyP95ms/in.BaselineLatencyMs)
	}

	recoveryFactor := clamp01(in.RecoveryRate / 100)
	uptimeFactor := clamp01(in.UptimePct / 100)

	score := (dropFactor*weightDrop +
		latencyFactor*weightLatency +
		recoveryFactor*weightRecovery +
		uptimeFactor*weightUptime) * 100

	return Output{
		Score:          score,
		State:          stateFromScore(score),
		DropFactor:     dropFactor,
		LatencyFactor:  latencyFactor,
		RecoveryFactor: recoveryFactor,
		UptimeFactor:   uptimeFactor,
	}
}

// stateFromScore maps a numeric score to a named health state.
func stateFromScore(score float64) string {
	switch {
	case score >= ThresholdHealthy:
		return StateHealthy
	case score >= ThresholdDegraded:
		return StateDegraded
	default:
		return StateCritical
	}
}

// clamp01 restricts v to the range [0, 1].
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
