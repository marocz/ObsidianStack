package api

// HealthResponse is the payload for GET /api/v1/health.
type HealthResponse struct {
	OverallScore  float64 `json:"overall_score"`
	State         string  `json:"state"`
	PipelineCount int     `json:"pipeline_count"`
	HealthyCount  int     `json:"healthy_count"`
	DegradedCount int     `json:"degraded_count"`
	CriticalCount int     `json:"critical_count"`
	UnknownCount  int     `json:"unknown_count"`
	AlertCount    int     `json:"alert_count"`
}

// PipelineResponse is one pipeline entry in GET /api/v1/pipelines or
// GET /api/v1/pipelines/{id}.
type PipelineResponse struct {
	SourceID         string           `json:"source_id"`
	SourceType       string           `json:"source_type"`
	NodeType         string           `json:"node_type,omitempty"`
	Cluster          string           `json:"cluster,omitempty"`
	Namespace        string           `json:"namespace,omitempty"`
	State            string           `json:"state"`
	DropPct          float64          `json:"drop_pct"`
	RecoveryRate     float64          `json:"recovery_rate"`
	ThroughputPerMin float64          `json:"throughput_per_min"`
	LatencyP50Ms     float64          `json:"latency_p50_ms"`
	LatencyP95Ms     float64          `json:"latency_p95_ms"`
	LatencyP99Ms     float64          `json:"latency_p99_ms"`
	StrengthScore    float64          `json:"strength_score"`
	UptimePct        float64          `json:"uptime_pct"`
	ErrorMessage     string           `json:"error_message,omitempty"`
	Signals          []SignalResponse  `json:"signals"`
	Diagnostics      []DiagnosticHint `json:"diagnostics"`
	LastSeen         string           `json:"last_seen"` // RFC3339
}

// SignalResponse is one signal type's stats within a pipeline.
type SignalResponse struct {
	Type       string  `json:"type"`
	ReceivedPM float64 `json:"received_pm"`
	DroppedPM  float64 `json:"dropped_pm"`
	DropPct    float64 `json:"drop_pct"`
}

// SignalAggregate is the totals for one signal type across all live pipelines.
type SignalAggregate struct {
	ReceivedPM float64 `json:"received_pm"`
	DroppedPM  float64 `json:"dropped_pm"`
	DropPct    float64 `json:"drop_pct"`
}

// SignalsResponse is the payload for GET /api/v1/signals.
type SignalsResponse struct {
	Metrics SignalAggregate `json:"metrics"`
	Logs    SignalAggregate `json:"logs"`
	Traces  SignalAggregate `json:"traces"`
}

// SnapshotResponse is the payload for GET /api/v1/snapshot.
type SnapshotResponse struct {
	Pipelines   []PipelineResponse `json:"pipelines"`
	GeneratedAt string             `json:"generated_at"` // RFC3339
}

// errorResponse is a generic JSON error body.
type errorResponse struct {
	Error string `json:"error"`
}
