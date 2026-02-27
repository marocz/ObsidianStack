import { useQuery } from '@tanstack/react-query'
import { api } from '../api/client'
import type { CertEntry } from '../api/types'

// ─── Helpers ──────────────────────────────────────────────────────────────────

function statusColor(status: CertEntry['status']): string {
  return {
    valid:       '#00e676',
    expiring:    '#ffab40',
    expired:     '#ff4f6a',
    unreachable: '#4a6080',
  }[status] ?? '#4a6080'
}

function statusGlow(status: CertEntry['status']): string {
  return {
    valid:       '0 0 8px rgba(0,230,118,0.4)',
    expiring:    '0 0 8px rgba(255,171,64,0.4)',
    expired:     '0 0 8px rgba(255,79,106,0.4)',
    unreachable: 'none',
  }[status] ?? 'none'
}

function daysBar(daysLeft: number) {
  // 90d = full bar; negative = 0
  const pct = Math.min(100, Math.max(0, (daysLeft / 90) * 100))
  const color =
    daysLeft <= 0  ? '#ff4f6a' :
    daysLeft <= 14 ? '#ff4f6a' :
    daysLeft <= 30 ? '#ffab40' : '#00e676'
  return { pct, color }
}

function formatDate(iso?: string): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' })
}

// ─── Summary cards ────────────────────────────────────────────────────────────

function SummaryCard({
  label, value, color,
}: { label: string; value: number; color: string }) {
  return (
    <div
      className="rounded-lg p-4 flex flex-col gap-1"
      style={{ background: '#0d1117', border: `1px solid ${color}30` }}
    >
      <span className="text-[11px] text-obs-muted uppercase tracking-widest">{label}</span>
      <span className="text-[28px] font-bold" style={{ color }}>{value}</span>
    </div>
  )
}

// ─── Cert row ─────────────────────────────────────────────────────────────────

function CertRow({ cert }: { cert: CertEntry }) {
  const color = statusColor(cert.status)
  const glow = statusGlow(cert.status)
  const bar = daysBar(cert.days_left)

  return (
    <div
      className="rounded-lg p-4 flex flex-col gap-3"
      style={{ background: '#0d1117', border: '1px solid #1e2d3d' }}
    >
      {/* Top row */}
      <div className="flex items-start gap-3">
        {/* Status dot */}
        <span
          className="mt-1 h-2 w-2 rounded-full flex-shrink-0"
          style={{ background: color, boxShadow: glow }}
        />

        {/* Source + endpoint */}
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="text-[13px] font-semibold text-obs-text">{cert.source_id}</span>
            <span
              className="text-[10px] font-semibold px-1.5 py-0.5 rounded-full uppercase tracking-wide"
              style={{ background: `${color}18`, color, border: `1px solid ${color}30` }}
            >
              {cert.status}
            </span>
            {cert.auth_type && cert.auth_type !== 'none' && (
              <span className="text-[10px] px-1.5 py-0.5 rounded-full" style={{ background: 'rgba(0,212,255,0.08)', color: '#00d4ff', border: '1px solid rgba(0,212,255,0.15)' }}>
                {cert.auth_type}
              </span>
            )}
          </div>
          <p className="text-[11px] text-obs-muted mt-0.5 truncate">{cert.endpoint}</p>
        </div>

        {/* Days left */}
        <div className="text-right shrink-0">
          <p className="text-[20px] font-bold leading-none" style={{ color }}>
            {cert.days_left <= 0 ? 'expired' : cert.days_left}
          </p>
          {cert.days_left > 0 && (
            <p className="text-[10px] text-obs-muted mt-0.5">days left</p>
          )}
        </div>
      </div>

      {/* Progress bar */}
      <div className="h-1 rounded-full overflow-hidden" style={{ background: '#1e2d3d' }}>
        <div
          className="h-full rounded-full transition-all duration-500"
          style={{ width: `${bar.pct}%`, background: bar.color }}
        />
      </div>

      {/* Meta row */}
      <div className="flex items-center gap-4 text-[11px] text-obs-muted flex-wrap">
        {cert.issuer && (
          <span>
            <span className="text-obs-muted2 mr-1">Issuer:</span>
            {cert.issuer}
          </span>
        )}
        {cert.not_after && (
          <span>
            <span className="text-obs-muted2 mr-1">Expires:</span>
            {formatDate(cert.not_after)}
          </span>
        )}
      </div>
    </div>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function Certs() {
  const { data: certs, isLoading, error } = useQuery({
    queryKey: ['certs'],
    queryFn: api.certs,
    refetchInterval: 60_000,
  })

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-obs-muted text-[13px] animate-pulse">Loading certificates…</div>
      </div>
    )
  }

  if (error || !certs) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-[#ff4f6a] text-[13px]">Failed to load certificate data</div>
      </div>
    )
  }

  const valid      = certs.filter((c) => c.status === 'valid').length
  const expiring   = certs.filter((c) => c.status === 'expiring').length
  const expired    = certs.filter((c) => c.status === 'expired').length
  const unreachable = certs.filter((c) => c.status === 'unreachable').length

  // Sort: expired first, then expiring (by days_left asc), then valid (by days_left asc)
  const sorted = [...certs].sort((a, b) => {
    const order = { expired: 0, expiring: 1, unreachable: 2, valid: 3 }
    const diff = (order[a.status] ?? 4) - (order[b.status] ?? 4)
    if (diff !== 0) return diff
    return a.days_left - b.days_left
  })

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-[18px] font-semibold text-obs-text">TLS Certificates</h1>
        <p className="text-[12px] text-obs-muted mt-1">
          Certificate health for all HTTPS sources monitored by the agent
        </p>
      </div>

      {/* Summary */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <SummaryCard label="Valid"       value={valid}       color="#00e676" />
        <SummaryCard label="Expiring"    value={expiring}    color="#ffab40" />
        <SummaryCard label="Expired"     value={expired}     color="#ff4f6a" />
        <SummaryCard label="Unreachable" value={unreachable} color="#4a6080" />
      </div>

      {/* Cert list */}
      {sorted.length === 0 ? (
        <div
          className="rounded-lg p-8 text-center"
          style={{ background: '#0d1117', border: '1px solid #1e2d3d' }}
        >
          <p className="text-obs-muted text-[13px]">No HTTPS sources configured.</p>
          <p className="text-obs-muted text-[12px] mt-1">
            Add a source with an <code className="text-obs-accent">https://</code> endpoint to see certificate status.
          </p>
        </div>
      ) : (
        <div className="space-y-3">
          {sorted.map((cert) => (
            <CertRow key={cert.source_id} cert={cert} />
          ))}
        </div>
      )}
    </div>
  )
}
