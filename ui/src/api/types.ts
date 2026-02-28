// TypeScript types mirroring the server's JSON API responses.
// Keep in sync with server/internal/api/types.go.

export interface HealthResponse {
  overall_score: number
  state: 'healthy' | 'degraded' | 'critical' | 'unknown'
  pipeline_count: number
  healthy_count: number
  degraded_count: number
  critical_count: number
  unknown_count: number
  alert_count: number
}

export interface SignalResponse {
  type: string
  received_pm: number
  dropped_pm: number
  drop_pct: number
}

export interface DiagnosticHint {
  key: string
  level: 'ok' | 'info' | 'warning' | 'critical'
  title: string
  detail: string
  value?: number
}

export interface PipelineResponse {
  source_id: string
  source_type: string
  node_type?: string
  cluster?: string
  namespace?: string
  state: string
  drop_pct: number
  recovery_rate: number
  throughput_per_min: number
  latency_p50_ms: number
  latency_p95_ms: number
  latency_p99_ms: number
  strength_score: number
  uptime_pct: number
  error_message?: string
  signals: SignalResponse[]
  diagnostics: DiagnosticHint[]
  /** Component-specific metrics. For otelcol: queue_size, queue_capacity,
   *  and _pm rates for exporter_sent_*, receiver_refused_*, exporter_send_failed_* */
  extra?: Record<string, number>
  last_seen: string
}

export interface SignalAggregate {
  received_pm: number
  dropped_pm: number
  drop_pct: number
}

export interface SignalsResponse {
  metrics: SignalAggregate
  logs: SignalAggregate
  traces: SignalAggregate
}

export interface CertEntry {
  source_id: string
  endpoint: string
  auth_type: string
  status: 'valid' | 'expiring' | 'expired' | 'unreachable'
  days_left: number
  issuer?: string
  not_after?: string
}

export interface AlertEntry {
  id: string
  rule_name: string
  source_id: string
  severity: 'critical' | 'warning' | 'info'
  message: string
  value: number
  fired_at: string
  resolved_at?: string
  state: 'firing' | 'resolved'
}

export interface SnapshotResponse {
  pipelines: PipelineResponse[]
  generated_at: string
}

// WebSocket message envelope — matches server/internal/ws Message type.
export interface WsMessage {
  event: 'snapshot'
  data: SnapshotResponse
}
