import { useStore } from '../store/useStore'
import { useHealth } from '../hooks/useHealth'
import { usePipelines } from '../hooks/usePipelines'

function StatusBadge({ state }: { state: string }) {
  const colours: Record<string, string> = {
    healthy: 'bg-green-500/20 text-green-400 ring-green-500/30',
    degraded: 'bg-yellow-500/20 text-yellow-400 ring-yellow-500/30',
    critical: 'bg-red-500/20 text-red-400 ring-red-500/30',
    unknown: 'bg-slate-500/20 text-slate-400 ring-slate-500/30',
  }
  const cls = colours[state.toLowerCase()] ?? colours.unknown
  return (
    <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ring-1 ring-inset ${cls}`}>
      {state}
    </span>
  )
}

function KpiCard({ label, value, sub }: { label: string; value: string | number; sub?: string }) {
  return (
    <div className="rounded-xl bg-obsidian-800 p-5 ring-1 ring-white/5">
      <p className="text-xs font-medium text-slate-400 uppercase tracking-wide">{label}</p>
      <p className="mt-2 text-3xl font-semibold text-white">{value}</p>
      {sub && <p className="mt-1 text-xs text-slate-500">{sub}</p>}
    </div>
  )
}

export default function Dashboard() {
  const wsConnected = useStore((s) => s.wsConnected)
  const liveSnapshot = useStore((s) => s.liveSnapshot)

  const { data: health, isLoading: healthLoading } = useHealth()
  const { data: pipelines, isLoading: pipelinesLoading } = usePipelines()

  const displayPipelines = liveSnapshot?.pipelines ?? pipelines ?? []

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-white">Dashboard</h1>
        <div className="flex items-center gap-2 text-sm">
          <span
            className={`h-2 w-2 rounded-full ${wsConnected ? 'bg-green-400 animate-pulse' : 'bg-slate-500'}`}
          />
          <span className="text-slate-400">{wsConnected ? 'Live' : 'Polling'}</span>
        </div>
      </div>

      {/* KPI row */}
      {healthLoading ? (
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="h-24 animate-pulse rounded-xl bg-obsidian-800" />
          ))}
        </div>
      ) : health ? (
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
          <KpiCard label="Health Score" value={`${Math.round(health.overall_score)}%`} sub={health.state} />
          <KpiCard label="Pipelines" value={health.pipeline_count} sub={`${health.healthy_count} healthy`} />
          <KpiCard label="Degraded" value={health.degraded_count} />
          <KpiCard label="Active Alerts" value={health.alert_count} />
        </div>
      ) : null}

      {/* Pipeline table */}
      <div className="rounded-xl bg-obsidian-800 ring-1 ring-white/5 overflow-hidden">
        <div className="px-5 py-4 border-b border-white/5">
          <h2 className="text-sm font-semibold text-white">Pipelines</h2>
        </div>

        {pipelinesLoading && !liveSnapshot ? (
          <div className="divide-y divide-white/5">
            {Array.from({ length: 3 }).map((_, i) => (
              <div key={i} className="h-12 animate-pulse bg-obsidian-700 mx-5 my-3 rounded" />
            ))}
          </div>
        ) : displayPipelines.length === 0 ? (
          <p className="px-5 py-10 text-center text-sm text-slate-500">No pipelines reporting yet.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm text-left">
              <thead>
                <tr className="text-xs text-slate-500 uppercase border-b border-white/5">
                  <th className="px-5 py-3 font-medium">Source</th>
                  <th className="px-5 py-3 font-medium">Type</th>
                  <th className="px-5 py-3 font-medium">State</th>
                  <th className="px-5 py-3 font-medium text-right">Throughput/min</th>
                  <th className="px-5 py-3 font-medium text-right">Drop %</th>
                  <th className="px-5 py-3 font-medium text-right">P95 ms</th>
                  <th className="px-5 py-3 font-medium text-right">Score</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-white/5">
                {displayPipelines.map((p) => (
                  <tr key={p.source_id} className="hover:bg-white/[0.02] transition-colors">
                    <td className="px-5 py-3 font-mono text-xs text-white">{p.source_id}</td>
                    <td className="px-5 py-3 text-slate-400">{p.source_type}</td>
                    <td className="px-5 py-3">
                      <StatusBadge state={p.state} />
                    </td>
                    <td className="px-5 py-3 text-right text-slate-300">{p.throughput_per_min.toFixed(0)}</td>
                    <td className="px-5 py-3 text-right text-slate-300">{p.drop_pct.toFixed(1)}%</td>
                    <td className="px-5 py-3 text-right text-slate-300">{p.latency_p95_ms.toFixed(1)}</td>
                    <td className="px-5 py-3 text-right">
                      <span
                        className={
                          p.strength_score >= 80
                            ? 'text-green-400'
                            : p.strength_score >= 50
                              ? 'text-yellow-400'
                              : 'text-red-400'
                        }
                      >
                        {Math.round(p.strength_score)}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}
