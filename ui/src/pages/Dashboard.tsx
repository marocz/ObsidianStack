import { useStore } from '../store/useStore'
import { useHealth } from '../hooks/useHealth'
import { usePipelines } from '../hooks/usePipelines'
import { useSignals } from '../hooks/useSignals'
import type { PipelineResponse, SignalAggregate } from '../api/types'

// ─── Primitive atoms ───────────────────────────────────────────────────────────

function StateChip({ state }: { state: string }) {
  const cls: Record<string, string> = {
    healthy:  'bg-emerald-500/15 text-emerald-400 ring-emerald-500/25',
    degraded: 'bg-amber-500/15   text-amber-400   ring-amber-500/25',
    critical: 'bg-red-500/15     text-red-400     ring-red-500/25',
    unknown:  'bg-slate-500/15   text-slate-400   ring-slate-500/25',
  }
  const c = cls[state.toLowerCase()] ?? cls.unknown
  return (
    <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ring-1 ring-inset ${c}`}>
      {state}
    </span>
  )
}

function DropBar({ pct }: { pct: number }) {
  const clamped = Math.min(100, Math.max(0, pct))
  const colour =
    clamped < 5  ? 'bg-emerald-500' :
    clamped < 20 ? 'bg-amber-500'   : 'bg-red-500'
  return (
    <div className="flex items-center gap-2">
      <div className="h-1.5 w-16 rounded-full bg-white/10 overflow-hidden">
        <div className={`h-full rounded-full ${colour}`} style={{ width: `${clamped}%` }} />
      </div>
      <span className={`text-xs tabular-nums ${clamped >= 20 ? 'text-red-400' : clamped >= 5 ? 'text-amber-400' : 'text-slate-400'}`}>
        {clamped.toFixed(1)}%
      </span>
    </div>
  )
}

function ScoreCell({ score }: { score: number }) {
  const s = Math.round(score)
  const cls =
    s >= 80 ? 'text-emerald-400' :
    s >= 50 ? 'text-amber-400'   : 'text-red-400'
  return (
    <div className="flex items-center gap-1.5">
      <div className="h-1.5 w-10 rounded-full bg-white/10 overflow-hidden">
        <div
          className={`h-full rounded-full ${s >= 80 ? 'bg-emerald-500' : s >= 50 ? 'bg-amber-500' : 'bg-red-500'}`}
          style={{ width: `${s}%` }}
        />
      </div>
      <span className={`text-xs tabular-nums font-medium ${cls}`}>{s}</span>
    </div>
  )
}

// ─── KPI card ─────────────────────────────────────────────────────────────────

interface KpiProps {
  label: string
  value: string | number
  sub?: string
  accent?: 'green' | 'yellow' | 'red' | 'default'
}
function KpiCard({ label, value, sub, accent = 'default' }: KpiProps) {
  const valueColour: Record<string, string> = {
    green:   'text-emerald-400',
    yellow:  'text-amber-400',
    red:     'text-red-400',
    default: 'text-white',
  }
  return (
    <div className="rounded-xl bg-obsidian-800 p-5 ring-1 ring-white/5 flex flex-col gap-1">
      <p className="text-xs font-medium text-slate-400 uppercase tracking-wide">{label}</p>
      <p className={`text-3xl font-semibold tabular-nums ${valueColour[accent]}`}>{value}</p>
      {sub && <p className="text-xs text-slate-500">{sub}</p>}
    </div>
  )
}

function KpiSkeleton() {
  return (
    <div className="rounded-xl bg-obsidian-800 p-5 ring-1 ring-white/5 space-y-2 animate-pulse">
      <div className="h-3 w-20 bg-white/10 rounded" />
      <div className="h-8 w-16 bg-white/10 rounded" />
      <div className="h-2.5 w-24 bg-white/10 rounded" />
    </div>
  )
}

// ─── Signal breakdown card ─────────────────────────────────────────────────────

function SignalCard({ label, data }: { label: string; data: SignalAggregate }) {
  const dropColour =
    data.drop_pct >= 20 ? 'text-red-400' :
    data.drop_pct >= 5  ? 'text-amber-400' : 'text-emerald-400'

  return (
    <div className="rounded-xl bg-obsidian-800 p-5 ring-1 ring-white/5">
      <div className="flex items-start justify-between">
        <p className="text-xs font-medium text-slate-400 uppercase tracking-wide">{label}</p>
        <span className={`text-xs font-semibold tabular-nums ${dropColour}`}>
          {data.drop_pct.toFixed(1)}% drop
        </span>
      </div>

      <div className="mt-3 space-y-2">
        {/* recv/min */}
        <div className="flex justify-between text-xs">
          <span className="text-slate-500">Received/min</span>
          <span className="text-white tabular-nums">{fmtRate(data.received_pm)}</span>
        </div>
        {/* dropped/min */}
        <div className="flex justify-between text-xs">
          <span className="text-slate-500">Dropped/min</span>
          <span className={`tabular-nums ${data.dropped_pm > 0 ? 'text-amber-400' : 'text-slate-400'}`}>
            {fmtRate(data.dropped_pm)}
          </span>
        </div>
      </div>

      {/* drop rate bar */}
      <div className="mt-3 h-1.5 rounded-full bg-white/10 overflow-hidden">
        <div
          className={`h-full rounded-full transition-all duration-500 ${
            data.drop_pct >= 20 ? 'bg-red-500' :
            data.drop_pct >= 5  ? 'bg-amber-500' : 'bg-emerald-500'
          }`}
          style={{ width: `${Math.min(100, data.drop_pct)}%` }}
        />
      </div>
    </div>
  )
}

function SignalCardSkeleton() {
  return (
    <div className="rounded-xl bg-obsidian-800 p-5 ring-1 ring-white/5 space-y-3 animate-pulse">
      <div className="flex justify-between">
        <div className="h-3 w-16 bg-white/10 rounded" />
        <div className="h-3 w-12 bg-white/10 rounded" />
      </div>
      <div className="space-y-2">
        <div className="h-2.5 w-full bg-white/10 rounded" />
        <div className="h-2.5 w-3/4 bg-white/10 rounded" />
      </div>
      <div className="h-1.5 w-full bg-white/10 rounded" />
    </div>
  )
}

// ─── Pipeline table row ────────────────────────────────────────────────────────

function PipelineRow({ p }: { p: PipelineResponse }) {
  return (
    <tr className="hover:bg-white/[0.025] transition-colors">
      <td className="px-4 py-3">
        <div className="flex flex-col gap-0.5">
          <span className="font-mono text-xs text-white leading-tight">{p.source_id}</span>
          {p.cluster && (
            <span className="text-[10px] text-slate-500">{p.cluster}{p.namespace ? ` / ${p.namespace}` : ''}</span>
          )}
        </div>
      </td>
      <td className="px-4 py-3 text-xs text-slate-400">{p.source_type}</td>
      <td className="px-4 py-3"><StateChip state={p.state} /></td>
      <td className="px-4 py-3 text-xs tabular-nums text-slate-300 text-right">{fmtRate(p.throughput_per_min)}</td>
      <td className="px-4 py-3"><DropBar pct={p.drop_pct} /></td>
      <td className="px-4 py-3">
        <div className="flex flex-col gap-0.5 text-right">
          <span className="text-xs tabular-nums text-slate-300">{p.latency_p95_ms.toFixed(1)}</span>
          <span className="text-[10px] tabular-nums text-slate-500">p99: {p.latency_p99_ms.toFixed(1)}</span>
        </div>
      </td>
      <td className="px-4 py-3 text-xs tabular-nums text-slate-300 text-right">
        {p.recovery_rate < 0.001 ? '—' : `${(p.recovery_rate * 100).toFixed(1)}%`}
      </td>
      <td className="px-4 py-3 text-xs tabular-nums text-slate-300 text-right">
        {p.uptime_pct.toFixed(1)}%
      </td>
      <td className="px-4 py-3"><ScoreCell score={p.strength_score} /></td>
    </tr>
  )
}

function PipelineTableSkeleton() {
  return (
    <>
      {Array.from({ length: 4 }).map((_, i) => (
        <tr key={i} className="border-b border-white/5 animate-pulse">
          {Array.from({ length: 9 }).map((__, j) => (
            <td key={j} className="px-4 py-3">
              <div className="h-3 rounded bg-white/10" style={{ width: `${40 + Math.random() * 40}%` }} />
            </td>
          ))}
        </tr>
      ))}
    </>
  )
}

// ─── Error banner ──────────────────────────────────────────────────────────────

function ErrorBanner({ message }: { message: string }) {
  return (
    <div className="rounded-xl border border-red-500/20 bg-red-500/5 px-5 py-4 text-sm text-red-400 flex items-start gap-3">
      <span className="mt-0.5 text-red-500">⚠</span>
      <span>{message}</span>
    </div>
  )
}

// ─── Helpers ───────────────────────────────────────────────────────────────────

function fmtRate(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000)     return `${(n / 1_000).toFixed(1)}k`
  return n.toFixed(0)
}

// ─── Dashboard ────────────────────────────────────────────────────────────────

export default function Dashboard() {
  const wsConnected  = useStore((s) => s.wsConnected)
  const liveSnapshot = useStore((s) => s.liveSnapshot)

  const { data: health,    isLoading: healthLoading,    error: healthError    } = useHealth()
  const { data: pipelines, isLoading: pipelinesLoading, error: pipelinesError } = usePipelines()
  const { data: signals,   isLoading: signalsLoading                          } = useSignals()

  const displayPipelines = liveSnapshot?.pipelines ?? pipelines ?? []
  const anyError = healthError || pipelinesError

  // derive health accent colour
  const healthAccent: 'green' | 'yellow' | 'red' | 'default' =
    !health ? 'default' :
    health.overall_score >= 80 ? 'green' :
    health.overall_score >= 50 ? 'yellow' : 'red'

  return (
    <div className="space-y-6">

      {/* ── Header ─────────────────────────────────────────────────────────── */}
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold text-white">Dashboard</h1>
        <div className="flex items-center gap-2 text-xs text-slate-400">
          <span className={`h-2 w-2 rounded-full transition-colors ${wsConnected ? 'bg-emerald-400 animate-pulse' : 'bg-slate-600'}`} />
          {wsConnected ? 'Live · WebSocket' : 'Polling · REST'}
        </div>
      </div>

      {/* ── Error banner ───────────────────────────────────────────────────── */}
      {anyError && (
        <ErrorBanner message="Could not reach the ObsidianStack server. Ensure it is running on :8080 and the agent is shipping data." />
      )}

      {/* ── KPI row ────────────────────────────────────────────────────────── */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
        {healthLoading ? (
          Array.from({ length: 5 }).map((_, i) => <KpiSkeleton key={i} />)
        ) : health ? (
          <>
            <KpiCard
              label="Health Score"
              value={`${Math.round(health.overall_score)}%`}
              sub={health.state}
              accent={healthAccent}
            />
            <KpiCard
              label="Pipelines"
              value={health.pipeline_count}
              sub={`${health.healthy_count} healthy`}
            />
            <KpiCard
              label="Degraded"
              value={health.degraded_count}
              accent={health.degraded_count > 0 ? 'yellow' : 'default'}
            />
            <KpiCard
              label="Critical"
              value={health.critical_count}
              accent={health.critical_count > 0 ? 'red' : 'default'}
            />
            <KpiCard
              label="Active Alerts"
              value={health.alert_count}
              accent={health.alert_count > 0 ? 'red' : 'default'}
            />
          </>
        ) : null}
      </div>

      {/* ── Signal breakdown ───────────────────────────────────────────────── */}
      <div>
        <h2 className="mb-3 text-xs font-semibold uppercase tracking-wide text-slate-500">Signal Telemetry</h2>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
          {signalsLoading ? (
            Array.from({ length: 3 }).map((_, i) => <SignalCardSkeleton key={i} />)
          ) : signals ? (
            <>
              <SignalCard label="Metrics" data={signals.metrics} />
              <SignalCard label="Logs"    data={signals.logs}    />
              <SignalCard label="Traces"  data={signals.traces}  />
            </>
          ) : (
            <div className="col-span-3 rounded-xl bg-obsidian-800 ring-1 ring-white/5 px-5 py-8 text-center text-sm text-slate-500">
              No signal data available yet.
            </div>
          )}
        </div>
      </div>

      {/* ── Pipeline table ─────────────────────────────────────────────────── */}
      <div>
        <h2 className="mb-3 text-xs font-semibold uppercase tracking-wide text-slate-500">Pipelines</h2>
        <div className="rounded-xl bg-obsidian-800 ring-1 ring-white/5 overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-sm text-left">
              <thead>
                <tr className="border-b border-white/5 text-[10px] text-slate-500 uppercase tracking-wide">
                  <th className="px-4 py-3 font-medium">Source</th>
                  <th className="px-4 py-3 font-medium">Type</th>
                  <th className="px-4 py-3 font-medium">State</th>
                  <th className="px-4 py-3 font-medium text-right">Recv/min</th>
                  <th className="px-4 py-3 font-medium">Drop %</th>
                  <th className="px-4 py-3 font-medium text-right">P95/P99 ms</th>
                  <th className="px-4 py-3 font-medium text-right">Recovery</th>
                  <th className="px-4 py-3 font-medium text-right">Uptime</th>
                  <th className="px-4 py-3 font-medium">Score</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-white/5">
                {pipelinesLoading && !liveSnapshot ? (
                  <PipelineTableSkeleton />
                ) : displayPipelines.length === 0 ? (
                  <tr>
                    <td colSpan={9} className="px-4 py-12 text-center text-sm text-slate-500">
                      No pipelines reporting yet. Deploy the agent and point it at this server.
                    </td>
                  </tr>
                ) : (
                  displayPipelines.map((p) => <PipelineRow key={p.source_id} p={p} />)
                )}
              </tbody>
            </table>
          </div>

          {/* last-updated footer */}
          {liveSnapshot && (
            <div className="border-t border-white/5 px-4 py-2 text-right text-[10px] text-slate-600">
              Last snapshot: {new Date(liveSnapshot.generated_at).toLocaleTimeString()}
            </div>
          )}
        </div>
      </div>

    </div>
  )
}
