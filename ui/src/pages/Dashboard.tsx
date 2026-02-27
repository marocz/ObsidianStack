import { useState } from 'react'
import { useStore } from '../store/useStore'
import { useHealth } from '../hooks/useHealth'
import { usePipelines } from '../hooks/usePipelines'
import { useSignals } from '../hooks/useSignals'
import { useCerts } from '../hooks/useCerts'
import { useAlerts } from '../hooks/useAlerts'
import type { PipelineResponse, CertEntry, SignalAggregate } from '../api/types'

// ─── Mini donut chart ─────────────────────────────────────────────────────────

function MiniDonut({ dropPct, color }: { dropPct: number; color: string }) {
  const r = 16
  const circ = 2 * Math.PI * r   // ~100.53
  const passed = ((100 - Math.min(100, dropPct)) / 100) * circ
  const dropped = (Math.min(100, dropPct) / 100) * circ
  return (
    <svg width="44" height="44" viewBox="0 0 44 44">
      {/* track */}
      <circle cx="22" cy="22" r={r} fill="none" stroke="rgba(255,255,255,0.06)" strokeWidth="4" />
      {/* passed arc */}
      <circle
        cx="22" cy="22" r={r} fill="none"
        stroke={color} strokeWidth="4"
        strokeDasharray={`${passed} ${circ}`}
        strokeLinecap="round"
        transform="rotate(-90 22 22)"
      />
      {/* dropped arc (starts where passed ends) */}
      {dropped > 0.5 && (
        <circle
          cx="22" cy="22" r={r} fill="none"
          stroke="#ff4f6a" strokeWidth="4"
          strokeDasharray={`${dropped} ${circ}`}
          strokeLinecap="round"
          transform={`rotate(${-90 + (passed / circ) * 360} 22 22)`}
        />
      )}
    </svg>
  )
}

// ─── KPI card ─────────────────────────────────────────────────────────────────

interface KpiCardProps {
  label: string
  value: string | number
  sub?: string
  topColor: string
  delay?: number
}

function KpiCard({ label, value, sub, topColor, delay = 0 }: KpiCardProps) {
  return (
    <div
      className="rounded-lg overflow-hidden animate-fade-in-up"
      style={{
        background: '#0d1117',
        border: '1px solid #1e2d3d',
        animationDelay: `${delay}s`,
      }}
    >
      <div className="h-0.5 w-full" style={{ background: topColor }} />
      <div className="p-4">
        <p className="text-[10px] font-semibold uppercase tracking-[1px] text-obs-muted">{label}</p>
        <p
          className="mt-1 text-[26px] font-syne font-extrabold tabular-nums leading-none"
          style={{ color: topColor }}
        >
          {value}
        </p>
        {sub && (
          <p className="mt-1 text-[10px] text-obs-muted2">{sub}</p>
        )}
      </div>
    </div>
  )
}

function KpiSkeleton({ delay = 0 }: { delay?: number }) {
  return (
    <div
      className="rounded-lg overflow-hidden animate-fade-in-up"
      style={{ background: '#0d1117', border: '1px solid #1e2d3d', animationDelay: `${delay}s` }}
    >
      <div className="h-0.5 w-full bg-white/10" />
      <div className="p-4 space-y-2 animate-pulse">
        <div className="h-2.5 w-16 bg-white/10 rounded" />
        <div className="h-7 w-20 bg-white/10 rounded" />
        <div className="h-2 w-24 bg-white/10 rounded" />
      </div>
    </div>
  )
}

// ─── Signal card ──────────────────────────────────────────────────────────────

interface SignalCardProps {
  label: string
  data: SignalAggregate
  color: string
  delay?: number
}

function SignalCard({ label, data, color, delay = 0 }: SignalCardProps) {
  return (
    <div
      className="rounded-lg p-4 animate-fade-in-up"
      style={{
        background: '#111820',
        border: '1px solid #1e2d3d',
        animationDelay: `${delay}s`,
      }}
    >
      <div className="flex items-start justify-between">
        <div>
          <p className="text-[10px] font-semibold uppercase tracking-[1px] text-obs-muted">{label}</p>
          <p className="mt-1 text-[22px] font-syne font-extrabold leading-none" style={{ color }}>
            {fmtRate(data.received_pm)}
            <span className="text-[11px] font-mono font-normal text-obs-muted2 ml-1">/min</span>
          </p>
        </div>
        <MiniDonut dropPct={data.drop_pct} color={color} />
      </div>

      <div className="mt-3 grid grid-cols-2 gap-2 text-[10px]">
        <div>
          <p className="text-obs-muted">Dropped/min</p>
          <p className="font-semibold" style={{ color: data.dropped_pm > 0 ? '#ffab40' : '#4a6080' }}>
            {fmtRate(data.dropped_pm)}
          </p>
        </div>
        <div>
          <p className="text-obs-muted">Drop rate</p>
          <p
            className="font-semibold"
            style={{ color: data.drop_pct >= 20 ? '#ff4f6a' : data.drop_pct >= 5 ? '#ffab40' : '#00e676' }}
          >
            {data.drop_pct.toFixed(1)}%
          </p>
        </div>
      </div>

      {/* drop bar */}
      <div className="mt-3 h-1 rounded-full overflow-hidden" style={{ background: 'rgba(255,255,255,0.06)' }}>
        <div
          className="h-full rounded-full transition-all duration-1000"
          style={{
            width: `${Math.min(100, data.drop_pct)}%`,
            background: data.drop_pct >= 20 ? '#ff4f6a' : data.drop_pct >= 5 ? '#ffab40' : '#00e676',
          }}
        />
      </div>
    </div>
  )
}

function SignalCardSkeleton({ delay = 0 }: { delay?: number }) {
  return (
    <div
      className="rounded-lg p-4 space-y-3 animate-pulse"
      style={{ background: '#111820', border: '1px solid #1e2d3d', animationDelay: `${delay}s` }}
    >
      <div className="flex justify-between">
        <div className="space-y-1.5">
          <div className="h-2.5 w-14 bg-white/10 rounded" />
          <div className="h-6 w-20 bg-white/10 rounded" />
        </div>
        <div className="h-11 w-11 rounded-full bg-white/10" />
      </div>
      <div className="h-1 w-full bg-white/10 rounded" />
    </div>
  )
}

// ─── Status pill ──────────────────────────────────────────────────────────────

function StatePill({ state }: { state: string }) {
  const map: Record<string, { bg: string; border: string; dot: string; text: string }> = {
    healthy:  { bg: 'rgba(0,230,118,0.08)',  border: 'rgba(0,230,118,0.25)',  dot: '#00e676', text: '#00e676' },
    degraded: { bg: 'rgba(255,171,64,0.08)', border: 'rgba(255,171,64,0.25)', dot: '#ffab40', text: '#ffab40' },
    critical: { bg: 'rgba(255,79,106,0.08)', border: 'rgba(255,79,106,0.25)', dot: '#ff4f6a', text: '#ff4f6a' },
    unknown:  { bg: 'rgba(74,96,128,0.15)',  border: 'rgba(74,96,128,0.3)',   dot: '#4a6080', text: '#6b8ba8' },
  }
  const c = map[state.toLowerCase()] ?? map.unknown
  return (
    <span
      className="inline-flex items-center gap-1.5 rounded px-2 py-0.5 text-[11px] font-semibold"
      style={{ background: c.bg, border: `1px solid ${c.border}`, color: c.text }}
    >
      <span className="h-1.5 w-1.5 rounded-full" style={{ background: c.dot, boxShadow: `0 0 4px ${c.dot}` }} />
      {state}
    </span>
  )
}

// ─── Drop rate bar ────────────────────────────────────────────────────────────

function DropBar({ pct }: { pct: number }) {
  const clamped = Math.min(100, Math.max(0, pct))
  const color = clamped >= 20 ? '#ff4f6a' : clamped >= 5 ? '#ffab40' : '#00e676'
  return (
    <div className="flex items-center gap-2 min-w-[90px]">
      <div className="flex-1 h-1 rounded-full overflow-hidden" style={{ background: 'rgba(255,255,255,0.06)' }}>
        <div
          className="h-full rounded-full transition-all duration-1000"
          style={{ width: `${clamped}%`, background: color }}
        />
      </div>
      <span className="text-[11px] tabular-nums w-8 text-right" style={{ color }}>
        {clamped.toFixed(1)}%
      </span>
    </div>
  )
}

// ─── Pipeline row ─────────────────────────────────────────────────────────────

function PipelineRow({ p }: { p: PipelineResponse }) {
  return (
    <tr className="transition-colors" style={{ borderBottom: '1px solid rgba(30,45,61,0.5)' }}
        onMouseEnter={e => (e.currentTarget.style.background = 'rgba(255,255,255,0.01)')}
        onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
    >
      <td className="px-4 py-2.5">
        <p className="text-[12px] font-semibold text-obs-text">{p.source_id}</p>
        {p.node_type && <p className="text-[10px] text-obs-muted">{p.node_type}</p>}
      </td>
      <td className="px-4 py-2.5">
        <StatePill state={p.state} />
      </td>
      <td className="px-4 py-2.5 text-right">
        <span className="text-[12px] text-obs-text tabular-nums">{fmtRate(p.throughput_per_min)}</span>
        <span className="text-[10px] text-obs-muted ml-0.5">/min</span>
      </td>
      <td className="px-4 py-2.5">
        <DropBar pct={p.drop_pct} />
      </td>
      <td className="px-4 py-2.5 text-right">
        <span
          className="text-[12px] tabular-nums"
          style={{ color: p.latency_p50_ms > 500 ? '#ff4f6a' : p.latency_p50_ms > 100 ? '#ffab40' : '#6b8ba8' }}
        >
          {p.latency_p50_ms.toFixed(0)}ms
        </span>
      </td>
    </tr>
  )
}

function PipelineTableSkeleton() {
  return (
    <>
      {Array.from({ length: 4 }).map((_, i) => (
        <tr key={i} style={{ borderBottom: '1px solid rgba(30,45,61,0.5)' }}>
          {[90, 70, 50, 100, 40].map((w, j) => (
            <td key={j} className="px-4 py-2.5">
              <div className="h-3 rounded bg-white/10 animate-pulse" style={{ width: `${w}%` }} />
            </td>
          ))}
        </tr>
      ))}
    </>
  )
}

// ─── Cert entry ───────────────────────────────────────────────────────────────

function CertRow({ cert }: { cert: CertEntry }) {
  const statusColor: Record<string, string> = {
    valid:       '#00e676',
    expiring:    '#ffab40',
    expired:     '#ff4f6a',
    unreachable: '#4a6080',
  }
  const color = statusColor[cert.status] ?? '#4a6080'
  const iconBg: Record<string, string> = {
    valid:       'rgba(0,230,118,0.1)',
    expiring:    'rgba(255,171,64,0.1)',
    expired:     'rgba(255,79,106,0.1)',
    unreachable: 'rgba(74,96,128,0.1)',
  }
  return (
    <div
      className="flex items-center gap-3 rounded p-2.5 transition-colors"
      style={{ background: '#111820', border: '1px solid #1e2d3d' }}
    >
      <div
        className="h-8 w-8 rounded flex items-center justify-center text-[14px] flex-shrink-0"
        style={{ background: iconBg[cert.status] ?? iconBg.unreachable, color }}
      >
        {cert.status === 'valid' ? '✓' : cert.status === 'expiring' ? '!' : '✕'}
      </div>
      <div className="flex-1 min-w-0">
        <p className="text-[12px] font-semibold text-obs-text truncate">{cert.source_id}</p>
        <p className="text-[10px] text-obs-muted truncate">{cert.endpoint}</p>
      </div>
      <div className="text-right">
        <p className="text-[12px] font-semibold" style={{ color }}>{cert.days_left}d</p>
        <p className="text-[10px] text-obs-muted">{cert.status}</p>
      </div>
    </div>
  )
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

function fmtRate(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000)     return `${(n / 1_000).toFixed(1)}k`
  return n.toFixed(0)
}

type TimeRange = '1H' | '6H' | '24H' | '7D'

// ─── Dashboard ────────────────────────────────────────────────────────────────

export default function Dashboard({ onAddSource }: { onAddSource?: () => void }) {
  const [timeRange, setTimeRange] = useState<TimeRange>('1H')

  const liveSnapshot = useStore((s) => s.liveSnapshot)

  const { data: health,    isLoading: healthLoading    } = useHealth()
  const { data: pipelines, isLoading: pipelinesLoading } = usePipelines()
  const { data: signals,   isLoading: signalsLoading   } = useSignals()
  const { data: certs                                   } = useCerts()
  const { data: alerts                                  } = useAlerts()

  const displayPipelines = liveSnapshot?.pipelines ?? pipelines ?? []

  // Derived KPI values from pipeline data
  const avgDrop = displayPipelines.length
    ? displayPipelines.reduce((s, p) => s + p.drop_pct, 0) / displayPipelines.length
    : 0
  const avgLatency = displayPipelines.length
    ? displayPipelines.reduce((s, p) => s + p.latency_p50_ms, 0) / displayPipelines.length
    : 0
  const totalEventsMin = displayPipelines.reduce((s, p) => s + p.throughput_per_min, 0)

  return (
    <div className="space-y-5">

      {/* ── Page header ──────────────────────────────────────────────────── */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="font-syne text-[22px] font-bold text-obs-text" style={{ letterSpacing: '-0.3px' }}>
            Pipeline Health
          </h1>
          <p className="text-[11px] text-obs-muted mt-0.5">
            {liveSnapshot
              ? `Live · updated ${new Date(liveSnapshot.generated_at).toLocaleTimeString()}`
              : 'Polling REST API · 15s interval'}
          </p>
        </div>

        {/* Time range selector */}
        <div
          className="flex items-center gap-0.5 p-0.5 rounded"
          style={{ background: '#0d1117', border: '1px solid #1e2d3d' }}
        >
          {(['1H', '6H', '24H', '7D'] as TimeRange[]).map((t) => (
            <button
              key={t}
              onClick={() => setTimeRange(t)}
              className="px-3 py-1 rounded text-[11px] font-semibold transition-all"
              style={
                timeRange === t
                  ? { background: '#00d4ff', color: '#080c10' }
                  : { color: '#6b8ba8', background: 'transparent' }
              }
            >
              {t}
            </button>
          ))}
        </div>
      </div>

      {/* ── KPI grid (6 cols) ─────────────────────────────────────────────── */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-6">
        {healthLoading ? (
          Array.from({ length: 6 }).map((_, i) => (
            <KpiSkeleton key={i} delay={i * 0.05} />
          ))
        ) : (
          <>
            <KpiCard
              label="Overall Health"
              value={health ? `${Math.round(health.overall_score)}%` : '—'}
              sub={health?.state}
              topColor="#00e676"
              delay={0.05}
            />
            <KpiCard
              label="Data Dropped"
              value={`${avgDrop.toFixed(1)}%`}
              sub="avg across pipelines"
              topColor="#ff4f6a"
              delay={0.10}
            />
            <KpiCard
              label="Avg Latency"
              value={`${Math.round(avgLatency)}ms`}
              sub="P50 across pipelines"
              topColor="#00d4ff"
              delay={0.15}
            />
            <KpiCard
              label="Events/min"
              value={fmtRate(totalEventsMin)}
              sub="total throughput"
              topColor="#7b61ff"
              delay={0.20}
            />
            <KpiCard
              label="Degraded"
              value={health?.degraded_count ?? '—'}
              sub={`of ${health?.pipeline_count ?? 0} pipelines`}
              topColor="#ffab40"
              delay={0.25}
            />
            <KpiCard
              label="Active Alerts"
              value={health?.alert_count ?? '—'}
              sub={health?.alert_count === 0 ? 'all clear' : 'need attention'}
              topColor="#ffd740"
              delay={0.30}
            />
          </>
        )}
      </div>

      {/* ── Signal breakdown ──────────────────────────────────────────────── */}
      <div>
        <p className="text-[10px] font-semibold uppercase tracking-[1.5px] text-obs-muted mb-3">
          Signal Breakdown
        </p>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
          {signalsLoading ? (
            Array.from({ length: 3 }).map((_, i) => <SignalCardSkeleton key={i} delay={i * 0.05} />)
          ) : signals ? (
            <>
              <SignalCard label="Metrics" data={signals.metrics} color="#00d4ff" delay={0.05} />
              <SignalCard label="Logs"    data={signals.logs}    color="#7b61ff" delay={0.10} />
              <SignalCard label="Traces"  data={signals.traces}  color="#ffd740" delay={0.15} />
            </>
          ) : (
            <div
              className="col-span-3 rounded-lg p-8 text-center text-[12px] text-obs-muted"
              style={{ background: '#111820', border: '1px solid #1e2d3d' }}
            >
              No signal data. Start the agent to begin reporting.
            </div>
          )}
        </div>
      </div>

      {/* ── Pipeline table + right col ────────────────────────────────────── */}
      <div className="grid grid-cols-1 gap-3 xl:grid-cols-[2fr_1fr]">

        {/* Pipeline table */}
        <div className="rounded-lg overflow-hidden" style={{ background: '#0d1117', border: '1px solid #1e2d3d' }}>
          <div
            className="flex items-center justify-between px-4 py-3"
            style={{ borderBottom: '1px solid #1e2d3d' }}
          >
            <div className="flex items-center gap-2">
              <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="#00d4ff" strokeWidth="1.5">
                <path d="M2 4h12M2 8h8M2 12h10" strokeLinecap="round" />
                <circle cx="13" cy="8" r="2" fill="#00d4ff" stroke="none" />
              </svg>
              <span className="font-syne text-[13px] font-bold text-obs-text">Pipelines</span>
            </div>
            <span
              className="text-[10px] font-semibold px-2 py-0.5 rounded-full"
              style={{ background: 'rgba(0,212,255,0.08)', border: '1px solid rgba(0,212,255,0.2)', color: '#00d4ff' }}
            >
              {displayPipelines.length}
            </span>
          </div>

          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr style={{ borderBottom: '1px solid #1e2d3d' }}>
                  {['Pipeline', 'Status', 'Throughput', 'Drop Rate', 'Latency'].map((h) => (
                    <th
                      key={h}
                      className="px-4 pb-3 pt-2 text-[10px] font-semibold uppercase tracking-[1px] text-obs-muted text-left"
                    >
                      {h}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {pipelinesLoading && !liveSnapshot ? (
                  <PipelineTableSkeleton />
                ) : displayPipelines.length === 0 ? (
                  <tr>
                    <td colSpan={5} className="px-4 py-12 text-center">
                      <p className="text-[12px] text-obs-muted">No pipelines reporting yet.</p>
                      {onAddSource && (
                        <button
                          onClick={onAddSource}
                          className="mt-3 px-4 py-1.5 rounded text-[11px] font-semibold transition-colors hover:brightness-110"
                          style={{ background: 'rgba(0,212,255,0.1)', border: '1px solid rgba(0,212,255,0.2)', color: '#00d4ff' }}
                        >
                          + Add your first source
                        </button>
                      )}
                    </td>
                  </tr>
                ) : (
                  displayPipelines.map((p) => <PipelineRow key={p.source_id} p={p} />)
                )}
              </tbody>
            </table>
          </div>
        </div>

        {/* Right column */}
        <div className="space-y-3">

          {/* Certs panel */}
          <div className="rounded-lg overflow-hidden" style={{ background: '#0d1117', border: '1px solid #1e2d3d' }}>
            <div className="flex items-center justify-between px-4 py-3" style={{ borderBottom: '1px solid #1e2d3d' }}>
              <div className="flex items-center gap-2">
                <svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="#7b61ff" strokeWidth="1.5">
                  <path d="M8 1L10.5 3H14V6.5L15 8L14 9.5V13H10.5L8 15L5.5 13H2V9.5L1 8L2 6.5V3H5.5L8 1Z" />
                  <path d="M6 8L7.5 9.5L10 6.5" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
                <span className="font-syne text-[13px] font-bold text-obs-text">Certificates</span>
              </div>
              <span
                className="text-[10px] font-semibold px-2 py-0.5 rounded-full"
                style={{ background: 'rgba(123,97,255,0.08)', border: '1px solid rgba(123,97,255,0.2)', color: '#7b61ff' }}
              >
                {certs?.length ?? 0}
              </span>
            </div>
            <div className="p-3 space-y-2">
              {certs && certs.length > 0 ? (
                certs.map((c) => <CertRow key={`${c.source_id}-${c.endpoint}`} cert={c} />)
              ) : (
                <p className="text-center text-[11px] text-obs-muted py-4">
                  No certs tracked yet
                </p>
              )}
            </div>
          </div>

          {/* Alerts feed */}
          <div className="rounded-lg overflow-hidden" style={{ background: '#0d1117', border: '1px solid #1e2d3d' }}>
            <div className="flex items-center justify-between px-4 py-3" style={{ borderBottom: '1px solid #1e2d3d' }}>
              <div className="flex items-center gap-2">
                <svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="#ffd740" strokeWidth="1.5">
                  <path d="M8 1L1 14h14L8 1Z" />
                  <path d="M8 6v4M8 12v.5" strokeLinecap="round" />
                </svg>
                <span className="font-syne text-[13px] font-bold text-obs-text">Alerts</span>
              </div>
              {alerts && alerts.length > 0 && (
                <span
                  className="text-[10px] font-semibold px-2 py-0.5 rounded-full"
                  style={{ background: 'rgba(255,79,106,0.08)', border: '1px solid rgba(255,79,106,0.2)', color: '#ff4f6a' }}
                >
                  {alerts.length}
                </span>
              )}
            </div>
            <div className="p-3">
              {!alerts || alerts.length === 0 ? (
                <div className="text-center py-4">
                  <p className="text-[12px]" style={{ color: '#00e676' }}>All clear</p>
                  <p className="text-[11px] text-obs-muted mt-1">No active alerts</p>
                </div>
              ) : (
                <p className="text-[12px] text-obs-muted text-center py-2">{alerts.length} active alerts</p>
              )}
            </div>
          </div>

        </div>
      </div>
    </div>
  )
}
