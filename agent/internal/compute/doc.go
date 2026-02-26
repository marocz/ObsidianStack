// Package compute derives pipeline health metrics from raw scraper output.
//
// score.go provides the pure Compute(Input) function that calculates the
// composite strength score (0–100) using the formula from ARCHITECTURE.md:
// drop_rate(40%) + latency(30%) + recovery(20%) + uptime(10%).
//
// engine.go provides the stateful Engine that maintains per-source counter
// baselines and derives per-minute rates from deltas between scrape cycles.
// Engine.Process accepts an injectable time.Time so tests are deterministic.
//
// Health state thresholds: Healthy ≥85, Degraded 60–84, Critical <60, Unknown.
package compute
