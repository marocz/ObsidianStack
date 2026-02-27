import type {
  AlertEntry,
  CertEntry,
  HealthResponse,
  PipelineResponse,
  SignalsResponse,
  SnapshotResponse,
} from './types'

const BASE = '/api/v1'

async function get<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE}${path}`)
  if (!res.ok) {
    throw new Error(`GET ${path} failed: ${res.status} ${res.statusText}`)
  }
  return res.json() as Promise<T>
}

export const api = {
  health: () => get<HealthResponse>('/health'),
  pipelines: () => get<PipelineResponse[]>('/pipelines'),
  pipeline: (id: string) => get<PipelineResponse>(`/pipelines/${encodeURIComponent(id)}`),
  signals: () => get<SignalsResponse>('/signals'),
  alerts: () => get<AlertEntry[]>('/alerts'),
  certs: () => get<CertEntry[]>('/certs'),
  snapshot: () => get<SnapshotResponse>('/snapshot'),
}
