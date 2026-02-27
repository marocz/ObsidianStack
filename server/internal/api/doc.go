// Package api implements the HTTP REST API for obsidianstack-server.
//
// New(store) returns an http.Handler that serves:
//
//	GET /api/v1/health          — overall score, state, per-state counts
//	GET /api/v1/pipelines       — all live pipelines ([]PipelineResponse)
//	GET /api/v1/pipelines/{id}  — single pipeline; 404 if unknown or stale
//	GET /api/v1/signals         — metrics/logs/traces aggregated across pipelines
//	GET /api/v1/alerts          — active alerts (empty until T021)
//	GET /api/v1/certs           — cert status per source endpoint
//	GET /api/v1/snapshot        — full JSON dump: all live pipelines + generated_at
//
// All endpoints:
//   - Respond with Content-Type: application/json
//   - Return 405 for non-GET methods
//   - Read live entries from the store (stale entries excluded from lists)
//
// JSON types are defined in types.go. No external HTTP framework is used.
package api
