import { useStore } from '../store/useStore'
import { useSignals } from '../hooks/useSignals'
import { usePipelines } from '../hooks/usePipelines'
import type { SignalAggregate, PipelineResponse } from '../api/types'

// ─── Helpers ──────────────────────────────────────────────────────────────────

function fmtRate(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000)     return `${(n / 1_000).toFixed(1)}k`
  return n.toFixed(0)
}

// ─── Mini donut ───────────────────────────────────────────────────────────────

function MiniDonut({ dropPct, color }: { dropPct: number; color: string }) {
  const r = 18
  const circ = 2 * Math.PI * r
  const passed = ((100 - Math.min(100, dropPct)) / 100) * circ
  const dropped = (Math.min(100, dropPct) / 100) * circ
  return (
    <svg width="50" height="50" viewBox="0 0 50 50">
      <circle cx="25" cy="25" r={r} fill="none" stroke="rgba(255,255,255,0.06)" strokeWidth="5" />
      <circle
        cx="25" cy="25" r={r} fill="none"
        stroke={color} strokeWidth="5"
        strokeDasharray={`${passed} ${circ}`}
        strokeLinecap="round"
        transform="rotate(-90 25 25)"
      />
      {dropped > 0.5 && (
        <circle
          cx="25" cy="25" r={r} fill="none"
          stroke="#ff4f6a" strokeWidth="5"
          strokeDasharray={`${dropped} ${circ}`}
          strokeLinecap="round"
          transform={`rotate(${-90 + (passed / circ) * 360} 25 25)`}
        />
      )}
    </svg>
  )
}

// ─── Signal card ──────────────────────────────────────────────────────────────

function SignalCard({ label, data, color }: { label: string; data: SignalAggregate; color: string }) {
  return (
    <div
      className="rounded-xl p-5 animate-fade-in-up"
      style={{ background: '#0d1117', border: '1px solid #1e2d3d' }}
    >
      <div className="flex items-start justify-between mb-4">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[1.5px] text-obs-muted">{label}</p>
          <p className="mt-1 text-[28px] font-syne font-extrabold leading-none" style={{ color }}>
            {fmtRate(data.received_pm)}
            <span className="text-[13px] font-mono font-normal text-obs-muted2 ml-1">/min</span>
          </p>
        </div>
        <MiniDonut dropPct={data.drop_pct} color={color} />
      </div>

      <div className="grid grid-cols-2 gap-3 text-[11px] mb-4">
        <div className="rounded-lg p-3" style={{ background: '#111820', border: '1px solid #1e2d3d' }}>
          <p className="text-obs-muted mb-0.5">Received/min</p>
          <p className="font-syne font-bold text-[16px] text-obs-text">{fmtRate(data.received_pm)}</p>
        </div>
        <div className="rounded-lg p-3" style={{ background: '#111820', border: '1px solid #1e2d3d' }}>
          <p className="text-obs-muted mb-0.5">Dropped/min</p>
          <p
            className="font-syne font-bold text-[16px]"
            style={{ color: data.dropped_pm > 0 ? '#ffab40' : '#4a6080' }}
          >
            {fmtRate(data.dropped_pm)}
          </p>
        </div>
      </div>

      {/* Drop rate bar */}
      <div>
        <div className="flex justify-between text-[10px] mb-1">
          <span className="text-obs-muted">Drop rate</span>
          <span
            style={{ color: data.drop_pct >= 20 ? '#ff4f6a' : data.drop_pct >= 5 ? '#ffab40' : '#00e676' }}
          >
            {data.drop_pct.toFixed(2)}%
          </span>
        </div>
        <div className="h-1.5 rounded-full overflow-hidden" style={{ background: 'rgba(255,255,255,0.06)' }}>
          <div
            className="h-full rounded-full"
            style={{
              width: `${Math.min(100, data.drop_pct)}%`,
              background: data.drop_pct >= 20 ? '#ff4f6a' : data.drop_pct >= 5 ? '#ffab40' : '#00e676',
            }}
          />
        </div>
      </div>
    </div>
  )
}

function SignalCardSkeleton() {
  return (
    <div className="rounded-xl p-5 space-y-4 animate-pulse" style={{ background: '#0d1117', border: '1px solid #1e2d3d' }}>
      <div className="flex justify-between">
        <div className="space-y-2">
          <div className="h-2.5 w-16 bg-white/10 rounded" />
          <div className="h-7 w-24 bg-white/10 rounded" />
        </div>
        <div className="h-12 w-12 rounded-full bg-white/10" />
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div className="h-16 rounded-lg bg-white/10" />
        <div className="h-16 rounded-lg bg-white/10" />
      </div>
      <div className="h-1.5 w-full bg-white/10 rounded-full" />
    </div>
  )
}

// ─── Per-pipeline signal table ────────────────────────────────────────────────

function PipelineSignalRow({ p, type }: { p: PipelineResponse; type: string }) {
  const sig = p.signals?.find((s) => s.type === type)
  if (!sig) return null
  const typeColor: Record<string, string> = { metrics: '#00d4ff', logs: '#7b61ff', traces: '#ffd740' }
  const color = typeColor[type] ?? '#6b8ba8'
  return (
    <tr
      className="transition-colors"
      style={{ borderBottom: '1px solid rgba(30,45,61,0.4)' }}
      onMouseEnter={e => (e.currentTarget.style.background = 'rgba(255,255,255,0.012)')}
      onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
    >
      <td className="px-4 py-2.5">
        <p className="text-[12px] font-semibold text-obs-text">{p.source_id}</p>
        <p className="text-[10px] text-obs-muted">{p.source_type}</p>
      </td>
      <td className="px-4 py-2.5 text-right">
        <span className="text-[12px] text-obs-text tabular-nums">{fmtRate(sig.received_pm)}</span>
        <span className="text-[10px] text-obs-muted ml-0.5">/min</span>
      </td>
      <td className="px-4 py-2.5 text-right">
        <span
          className="text-[12px] tabular-nums"
          style={{ color: sig.dropped_pm > 0 ? '#ffab40' : '#4a6080' }}
        >
          {fmtRate(sig.dropped_pm)}/min
        </span>
      </td>
      <td className="px-4 py-2.5">
        <div className="flex items-center gap-2">
          <div className="flex-1 h-1 rounded-full overflow-hidden" style={{ background: 'rgba(255,255,255,0.06)' }}>
            <div
              className="h-full rounded-full"
              style={{
                width: `${Math.min(100, sig.drop_pct)}%`,
                background: sig.drop_pct >= 20 ? '#ff4f6a' : sig.drop_pct >= 5 ? '#ffab40' : color,
              }}
            />
          </div>
          <span className="text-[11px] tabular-nums w-9 text-right" style={{ color: sig.drop_pct >= 5 ? '#ffab40' : '#4a6080' }}>
            {sig.drop_pct.toFixed(1)}%
          </span>
        </div>
      </td>
    </tr>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function Signals() {
  const liveSnapshot = useStore((s) => s.liveSnapshot)
  const { data: signals, isLoading } = useSignals()
  const { data: fetched } = usePipelines()
  const pipelines = liveSnapshot?.pipelines ?? fetched ?? []

  const SIGNAL_TYPES = [
    { type: 'metrics', color: '#00d4ff', label: 'Metrics' },
    { type: 'logs',    color: '#7b61ff', label: 'Logs' },
    { type: 'traces',  color: '#ffd740', label: 'Traces' },
  ]

  return (
    <div className="space-y-6">

      {/* Page header */}
      <div>
        <h1 className="font-syne text-[22px] font-bold text-obs-text" style={{ letterSpacing: '-0.3px' }}>
          Signals
        </h1>
        <p className="text-[11px] text-obs-muted mt-0.5">
          Aggregated metrics, logs and traces across all monitored pipelines
        </p>
      </div>

      {/* Summary cards */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        {isLoading ? (
          SIGNAL_TYPES.map((s) => <SignalCardSkeleton key={s.type} />)
        ) : signals ? (
          SIGNAL_TYPES.map((s) => (
            <SignalCard
              key={s.type}
              label={s.label}
              data={(signals as Record<string, SignalAggregate>)[s.type]}
              color={s.color}
            />
          ))
        ) : (
          <div
            className="col-span-3 rounded-xl p-12 text-center"
            style={{ background: '#0d1117', border: '1px solid #1e2d3d' }}
          >
            <p className="text-[12px] text-obs-muted">No signal data. Start the agent to begin reporting.</p>
          </div>
        )}
      </div>

      {/* Per-pipeline breakdown */}
      {pipelines.some((p) => p.signals?.length) && (
        <div className="space-y-4">
          <p className="text-[10px] font-semibold uppercase tracking-[1.5px] text-obs-muted">
            Per-pipeline breakdown
          </p>

          <div className="grid grid-cols-1 gap-4 xl:grid-cols-3">
            {SIGNAL_TYPES.map((st) => {
              const rows = pipelines.filter((p) => p.signals?.some((s) => s.type === st.type))
              if (rows.length === 0) return null
              return (
                <div
                  key={st.type}
                  className="rounded-lg overflow-hidden"
                  style={{ background: '#0d1117', border: '1px solid #1e2d3d' }}
                >
                  <div
                    className="flex items-center gap-2 px-4 py-3"
                    style={{ borderBottom: '1px solid #1e2d3d' }}
                  >
                    <span
                      className="h-2 w-2 rounded-full"
                      style={{ background: st.color, boxShadow: `0 0 6px ${st.color}` }}
                    />
                    <span className="font-syne text-[13px] font-bold text-obs-text">{st.label}</span>
                  </div>
                  <table className="w-full">
                    <thead>
                      <tr style={{ borderBottom: '1px solid rgba(30,45,61,0.6)' }}>
                        {['Pipeline', 'Recv/min', 'Drop/min', 'Drop %'].map((h) => (
                          <th
                            key={h}
                            className={`px-4 pb-2 pt-1.5 text-[10px] font-semibold uppercase tracking-[1px] text-obs-muted ${h !== 'Pipeline' ? 'text-right' : 'text-left'}`}
                          >
                            {h}
                          </th>
                        ))}
                      </tr>
                    </thead>
                    <tbody>
                      {rows.map((p) => <PipelineSignalRow key={p.source_id} p={p} type={st.type} />)}
                    </tbody>
                  </table>
                </div>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}
