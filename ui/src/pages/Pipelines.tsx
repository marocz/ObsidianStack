import { useState } from 'react'
import { useStore } from '../store/useStore'
import { usePipelines } from '../hooks/usePipelines'
import type { PipelineResponse, SignalResponse } from '../api/types'
import { DiagnosticPanel, DiagDrawer } from '../components/DiagnosticPanel'
import { OtelFlowCard } from '../components/OtelFlowCard'

// ─── Helpers ──────────────────────────────────────────────────────────────────

function fmtRate(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000)     return `${(n / 1_000).toFixed(1)}k`
  return n.toFixed(0)
}

function ago(iso: string): string {
  const diff = Math.floor((Date.now() - new Date(iso).getTime()) / 1000)
  if (diff < 60) return `${diff}s ago`
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  return `${Math.floor(diff / 3600)}h ago`
}

// ─── Sub-components ───────────────────────────────────────────────────────────

function StatePill({ state }: { state: string }) {
  const map: Record<string, { bg: string; border: string; dot: string; text: string }> = {
    healthy:  { bg: 'rgba(0,230,118,0.08)',  border: 'rgba(0,230,118,0.25)',  dot: '#00e676', text: '#00e676' },
    degraded: { bg: 'rgba(255,171,64,0.08)', border: 'rgba(255,171,64,0.25)', dot: '#ffab40', text: '#ffab40' },
    critical: { bg: 'rgba(255,79,106,0.08)', border: 'rgba(255,79,106,0.25)', dot: '#ff4f6a', text: '#ff4f6a' },
    unknown:  { bg: 'rgba(74,96,128,0.15)',  border: 'rgba(74,96,128,0.3)',   dot: '#4a6080', text: '#6b8ba8' },
  }
  const c = map[state?.toLowerCase()] ?? map.unknown
  return (
    <span
      className="inline-flex items-center gap-1.5 rounded px-2 py-0.5 text-[11px] font-semibold"
      style={{ background: c.bg, border: `1px solid ${c.border}`, color: c.text }}
    >
      <span className="h-1.5 w-1.5 rounded-full" style={{ background: c.dot, boxShadow: `0 0 4px ${c.dot}` }} />
      {state ?? 'unknown'}
    </span>
  )
}

function DropBar({ pct }: { pct: number }) {
  const clamped = Math.min(100, Math.max(0, pct))
  const color = clamped >= 20 ? '#ff4f6a' : clamped >= 5 ? '#ffab40' : '#00e676'
  return (
    <div className="flex items-center gap-2 min-w-[100px]">
      <div className="flex-1 h-1 rounded-full overflow-hidden" style={{ background: 'rgba(255,255,255,0.06)' }}>
        <div className="h-full rounded-full" style={{ width: `${clamped}%`, background: color }} />
      </div>
      <span className="text-[11px] tabular-nums w-9 text-right" style={{ color }}>
        {clamped.toFixed(1)}%
      </span>
    </div>
  )
}

function ScoreBar({ score }: { score: number }) {
  const color = score >= 85 ? '#00e676' : score >= 60 ? '#ffab40' : '#ff4f6a'
  return (
    <div className="flex items-center gap-2 min-w-[100px]">
      <div className="flex-1 h-1 rounded-full overflow-hidden" style={{ background: 'rgba(255,255,255,0.06)' }}>
        <div className="h-full rounded-full" style={{ width: `${score}%`, background: color }} />
      </div>
      <span className="text-[11px] tabular-nums w-7 text-right" style={{ color }}>
        {Math.round(score)}
      </span>
    </div>
  )
}

function SignalChip({ sig }: { sig: SignalResponse }) {
  const typeColor: Record<string, string> = {
    metrics: '#00d4ff',
    logs:    '#7b61ff',
    traces:  '#ffd740',
  }
  const color = typeColor[sig.type] ?? '#6b8ba8'
  return (
    <div
      className="flex items-center justify-between rounded px-3 py-2 text-[11px]"
      style={{ background: '#111820', border: '1px solid #1e2d3d' }}
    >
      <span className="font-semibold" style={{ color }}>{sig.type}</span>
      <span className="text-obs-text tabular-nums">{fmtRate(sig.received_pm)}/min</span>
      <span style={{ color: sig.drop_pct >= 5 ? '#ffab40' : '#4a6080' }}>
        {sig.drop_pct.toFixed(1)}% drop
      </span>
    </div>
  )
}

// ─── Pipeline row ──────────────────────────────────────────────────────────────

interface PipelineRowProps {
  p: PipelineResponse
  onDiagnose: (p: PipelineResponse) => void
}

function PipelineRow({ p, onDiagnose }: PipelineRowProps) {
  const [expanded, setExpanded] = useState(false)

  // Worst diagnostic level for the inline compact chip
  const worstLevel = p.diagnostics?.reduce<string>((worst, d) => {
    const order = { critical: 0, warning: 1, info: 2, ok: 3 }
    return (order[d.level as keyof typeof order] ?? 9) < (order[worst as keyof typeof order] ?? 9) ? d.level : worst
  }, 'ok') ?? 'ok'

  const levelDot: Record<string, string> = {
    ok: '#00e676', info: '#00d4ff', warning: '#ffab40', critical: '#ff4f6a',
  }

  return (
    <>
      <tr
        className="cursor-pointer transition-colors"
        style={{ borderBottom: expanded ? 'none' : '1px solid rgba(30,45,61,0.5)' }}
        onClick={() => setExpanded((x) => !x)}
        onMouseEnter={e => (e.currentTarget.style.background = 'rgba(255,255,255,0.015)')}
        onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
      >
        <td className="pl-4 pr-2 py-3 w-6">
          <svg
            width="12" height="12" viewBox="0 0 12 12" fill="none"
            stroke="#4a6080" strokeWidth="1.5" strokeLinecap="round"
            style={{ transform: expanded ? 'rotate(90deg)' : 'rotate(0deg)', transition: 'transform 0.15s' }}
          >
            <path d="M4 2l4 4-4 4" />
          </svg>
        </td>

        {/* Source ID + type */}
        <td className="px-3 py-3">
          <div className="flex items-center gap-2">
            <div>
              <p className="text-[12px] font-semibold text-obs-text">{p.source_id}</p>
              <p className="text-[10px] text-obs-muted">{p.source_type}{p.namespace ? ` · ${p.namespace}` : ''}</p>
            </div>
            {/* Diagnostic indicator dot */}
            {p.diagnostics && p.diagnostics.length > 0 && worstLevel !== 'ok' && (
              <span
                className="h-1.5 w-1.5 rounded-full flex-shrink-0"
                style={{ background: levelDot[worstLevel], boxShadow: `0 0 5px ${levelDot[worstLevel]}` }}
                title="Has diagnostic insights — click to expand"
              />
            )}
          </div>
        </td>

        <td className="px-3 py-3"><StatePill state={p.state} /></td>
        <td className="px-3 py-3 text-right">
          <span className="text-[12px] text-obs-text tabular-nums">{fmtRate(p.throughput_per_min)}</span>
          <span className="text-[10px] text-obs-muted ml-0.5">/min</span>
        </td>
        <td className="px-3 py-3"><DropBar pct={p.drop_pct} /></td>
        <td className="px-3 py-3"><ScoreBar score={p.strength_score} /></td>
        <td className="px-3 py-3 text-right">
          <span
            className="text-[12px] tabular-nums"
            style={{ color: p.latency_p50_ms > 500 ? '#ff4f6a' : p.latency_p50_ms > 100 ? '#ffab40' : '#6b8ba8' }}
          >
            {p.latency_p50_ms > 0 ? `${p.latency_p50_ms.toFixed(0)}ms` : '—'}
          </span>
        </td>
        <td className="px-3 py-3 text-right">
          <span className="text-[11px] text-obs-muted">{ago(p.last_seen)}</span>
        </td>
      </tr>

      {/* Expanded: diagnostics + signals + stats */}
      {expanded && (
        <tr style={{ borderBottom: '1px solid rgba(30,45,61,0.5)' }}>
          <td colSpan={8} className="px-4 pb-4 pt-1">
            <div
              className="rounded-lg p-4 space-y-4"
              style={{ background: '#080c10', border: '1px solid #1e2d3d' }}
            >
              {/* Diagnostic chips */}
              {p.diagnostics && p.diagnostics.length > 0 && (
                <div className="space-y-2">
                  <div className="flex items-center justify-between">
                    <p className="text-[10px] uppercase tracking-wide text-obs-muted">
                      Insights — hover for explanation, click to expand
                    </p>
                    <button
                      onClick={(e) => { e.stopPropagation(); onDiagnose(p) }}
                      className="text-[10px] px-2 py-0.5 rounded transition-colors hover:text-obs-accent"
                      style={{ border: '1px solid #1e2d3d', color: '#4a6080' }}
                    >
                      Open full report →
                    </button>
                  </div>
                  <DiagnosticPanel diagnostics={p.diagnostics} />
                </div>
              )}

              {/* Quick stats */}
              <div className="grid grid-cols-3 gap-3 text-[11px]">
                <div>
                  <p className="text-obs-muted">Recovery rate</p>
                  <p className="font-semibold text-obs-text">{p.recovery_rate.toFixed(1)}%</p>
                </div>
                <div>
                  <p className="text-obs-muted">Uptime</p>
                  <p className="font-semibold" style={{ color: p.uptime_pct < 90 ? '#ffab40' : '#e8f1ff' }}>
                    {p.uptime_pct.toFixed(1)}%
                  </p>
                </div>
                <div>
                  <p className="text-obs-muted">P95 latency</p>
                  <p className="font-semibold text-obs-text">
                    {p.latency_p95_ms > 0 ? `${p.latency_p95_ms.toFixed(0)}ms` : '—'}
                  </p>
                </div>
              </div>

              {/* OTel Collector: rich flow card instead of generic signal chips */}
              {p.source_type === 'otelcol' ? (
                <OtelFlowCard pipeline={p} />
              ) : (
                /* Generic signal breakdown for prometheus / loki / etc. */
                p.signals && p.signals.length > 0 && (
                  <div className="space-y-1.5">
                    <p className="text-[10px] uppercase tracking-wide text-obs-muted">Signal breakdown</p>
                    <div className="grid grid-cols-3 gap-2">
                      {p.signals.map((s) => <SignalChip key={s.type} sig={s} />)}
                    </div>
                  </div>
                )
              )}

              {/* Error */}
              {p.error_message && (
                <div
                  className="rounded px-3 py-2 text-[11px] font-mono"
                  style={{ background: 'rgba(255,79,106,0.06)', border: '1px solid rgba(255,79,106,0.2)', color: '#ff4f6a' }}
                >
                  {p.error_message}
                </div>
              )}
            </div>
          </td>
        </tr>
      )}
    </>
  )
}

// ─── Skeleton ─────────────────────────────────────────────────────────────────

function RowSkeleton() {
  return (
    <tr style={{ borderBottom: '1px solid rgba(30,45,61,0.5)' }}>
      {[6, 100, 70, 60, 100, 100, 50, 40].map((w, i) => (
        <td key={i} className="px-3 py-3">
          <div className="h-3 rounded bg-white/10 animate-pulse" style={{ width: `${w}%` }} />
        </td>
      ))}
    </tr>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

type SortKey = 'source_id' | 'state' | 'throughput_per_min' | 'drop_pct' | 'strength_score'

export default function Pipelines() {
  const liveSnapshot = useStore((s) => s.liveSnapshot)
  const { data: fetched, isLoading } = usePipelines()

  const pipelines = liveSnapshot?.pipelines ?? fetched ?? []

  const [sortKey, setSortKey] = useState<SortKey>('strength_score')
  const [sortAsc, setSortAsc] = useState(false)
  const [filter, setFilter] = useState('')
  const [drawerPipeline, setDrawerPipeline] = useState<PipelineResponse | null>(null)

  function toggleSort(key: SortKey) {
    if (sortKey === key) setSortAsc((x) => !x)
    else { setSortKey(key); setSortAsc(false) }
  }

  const filtered = pipelines.filter((p) =>
    filter === '' ||
    p.source_id.toLowerCase().includes(filter.toLowerCase()) ||
    p.source_type.toLowerCase().includes(filter.toLowerCase())
  )

  const sorted = [...filtered].sort((a, b) => {
    const av = a[sortKey] as string | number
    const bv = b[sortKey] as string | number
    const cmp = av < bv ? -1 : av > bv ? 1 : 0
    return sortAsc ? cmp : -cmp
  })

  function SortTh({ label, col }: { label: string; col: SortKey }) {
    const active = sortKey === col
    return (
      <th className="px-3 pb-2 pt-1 text-left cursor-pointer select-none" onClick={() => toggleSort(col)}>
        <span
          className="flex items-center gap-1 text-[10px] font-semibold uppercase tracking-[1px]"
          style={{ color: active ? '#00d4ff' : '#4a6080' }}
        >
          {label}
          <svg width="8" height="8" viewBox="0 0 8 8" fill="none">
            <path d="M4 1v6M1 4l3-3 3 3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"
              style={{ opacity: active && !sortAsc ? 1 : 0.3 }} />
            <path d="M4 7V1M1 4l3 3 3-3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"
              style={{ opacity: active && sortAsc ? 1 : 0.3 }} />
          </svg>
        </span>
      </th>
    )
  }

  const healthyCount  = pipelines.filter((p) => p.state === 'healthy').length
  const degradedCount = pipelines.filter((p) => p.state === 'degraded').length
  const criticalCount = pipelines.filter((p) => p.state === 'critical').length

  return (
    <>
      <div className="space-y-5">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="font-syne text-[22px] font-bold text-obs-text" style={{ letterSpacing: '-0.3px' }}>
              Pipelines
            </h1>
            <p className="text-[11px] text-obs-muted mt-0.5">
              {liveSnapshot
                ? `Live · updated ${new Date(liveSnapshot.generated_at).toLocaleTimeString()}`
                : 'REST API · 15s'}
            </p>
          </div>

          <div className="flex items-center gap-2">
            {[
              { label: `${healthyCount} healthy`,  color: '#00e676' },
              { label: `${degradedCount} degraded`, color: '#ffab40' },
              { label: `${criticalCount} critical`, color: '#ff4f6a' },
            ].map(({ label, color }) => (
              <span
                key={label}
                className="text-[10px] font-semibold px-2.5 py-1 rounded"
                style={{ background: `${color}12`, border: `1px solid ${color}30`, color }}
              >
                {label}
              </span>
            ))}
          </div>
        </div>

        {/* Filter */}
        <input
          type="text"
          placeholder="Filter by source ID or type..."
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          className="w-full rounded px-3 py-2 text-[12px] text-obs-text outline-none"
          style={{ background: '#0d1117', border: '1px solid #1e2d3d', fontFamily: '"JetBrains Mono", monospace' }}
          onFocus={(e) => (e.currentTarget.style.borderColor = 'rgba(0,212,255,0.4)')}
          onBlur={(e)  => (e.currentTarget.style.borderColor = '#1e2d3d')}
        />

        {/* Table */}
        <div className="rounded-lg overflow-hidden" style={{ background: '#0d1117', border: '1px solid #1e2d3d' }}>
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr style={{ borderBottom: '1px solid #1e2d3d' }}>
                  <th className="pl-4 pr-2 pb-2 pt-1 w-6" />
                  <SortTh label="Pipeline"   col="source_id" />
                  <SortTh label="Status"     col="state" />
                  <SortTh label="Throughput" col="throughput_per_min" />
                  <SortTh label="Drop rate"  col="drop_pct" />
                  <SortTh label="Score"      col="strength_score" />
                  <th className="px-3 pb-2 pt-1 text-left text-[10px] font-semibold uppercase tracking-[1px] text-obs-muted">Latency</th>
                  <th className="px-3 pb-2 pt-1 text-left text-[10px] font-semibold uppercase tracking-[1px] text-obs-muted">Last seen</th>
                </tr>
              </thead>
              <tbody>
                {isLoading && !liveSnapshot ? (
                  Array.from({ length: 5 }).map((_, i) => <RowSkeleton key={i} />)
                ) : sorted.length === 0 ? (
                  <tr>
                    <td colSpan={8} className="py-16 text-center">
                      <p className="text-[12px] text-obs-muted">
                        {filter ? 'No pipelines match the filter.' : 'No pipelines reporting yet.'}
                      </p>
                    </td>
                  </tr>
                ) : (
                  sorted.map((p) => (
                    <PipelineRow
                      key={p.source_id}
                      p={p}
                      onDiagnose={setDrawerPipeline}
                    />
                  ))
                )}
              </tbody>
            </table>
          </div>
        </div>

        <p className="text-[10px] text-obs-muted text-right">
          {sorted.length} of {pipelines.length} pipeline{pipelines.length !== 1 ? 's' : ''}
          {filter && ` matching "${filter}"`} · click row to expand · click insight chips for explanation
        </p>
      </div>

      {/* Diagnostic drawer */}
      {drawerPipeline && (
        <>
          {/* Backdrop */}
          <div
            className="fixed inset-0 z-40"
            style={{ background: 'rgba(0,0,0,0.5)' }}
            onClick={() => setDrawerPipeline(null)}
          />
          <DiagDrawer
            sourceId={drawerPipeline.source_id}
            sourceType={drawerPipeline.source_type}
            diagnostics={drawerPipeline.diagnostics ?? []}
            onClose={() => setDrawerPipeline(null)}
          />
        </>
      )}
    </>
  )
}
