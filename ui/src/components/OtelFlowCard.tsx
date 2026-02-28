import { PipelineResponse } from '../api/types'

// ── helpers ──────────────────────────────────────────────────────────────────

function fmt(n: number | undefined): string {
  if (n === undefined || n === 0) return '—'
  if (n >= 1000) return `${(n / 1000).toFixed(1)}k/m`
  return `${n.toFixed(0)}/m`
}

function ex(extra: Record<string, number> | undefined, key: string): number {
  return extra?.[key] ?? 0
}

// ── sub-components ───────────────────────────────────────────────────────────

interface SignalRowProps {
  label: string
  icon: string
  receivedPm: number
  droppedPm: number
  sentPm: number
  refusedPm: number
  failedPm: number
}

function SignalRow({ label, icon, receivedPm, droppedPm, sentPm, refusedPm, failedPm }: SignalRowProps) {
  const hasData = receivedPm > 0 || sentPm > 0
  const hasDrop = droppedPm > 0.1 || failedPm > 0.1
  const hasRefused = refusedPm > 0.1

  return (
    <div className="grid grid-cols-[90px_1fr_24px_1fr_24px_1fr] items-center gap-x-2 py-1 text-sm">
      {/* Signal label */}
      <div className="flex items-center gap-1.5 text-gray-400 font-medium">
        <span>{icon}</span>
        <span>{label}</span>
      </div>

      {/* Received */}
      <div className="text-right">
        <span className={`font-mono ${hasData ? 'text-gray-200' : 'text-gray-600'}`}>
          {fmt(receivedPm)}
        </span>
        {hasRefused && (
          <span className="ml-1 text-xs text-yellow-400" title={`${fmt(refusedPm)} refused at receiver`}>
            −{fmt(refusedPm)}
          </span>
        )}
      </div>

      {/* Arrow */}
      <div className="text-center text-gray-600">→</div>

      {/* Pipeline (processor stage) */}
      <div className="text-center">
        {hasDrop ? (
          <span className="font-mono text-red-400">{fmt(droppedPm)} dropped</span>
        ) : (
          <span className="font-mono text-green-500 text-xs">pass-through</span>
        )}
      </div>

      {/* Arrow */}
      <div className="text-center text-gray-600">→</div>

      {/* Exported */}
      <div className="text-right">
        {hasData ? (
          <span className={`font-mono ${failedPm > 0.1 ? 'text-red-400' : 'text-green-400'}`}>
            {fmt(sentPm)}
            {failedPm > 0.1 && (
              <span className="ml-1 text-xs text-red-400" title={`${fmt(failedPm)} failed to export`}>
                ✗{fmt(failedPm)}
              </span>
            )}
          </span>
        ) : (
          <span className="text-gray-600 font-mono">—</span>
        )}
      </div>
    </div>
  )
}

interface QueueBarProps {
  size: number
  capacity: number
}

function QueueBar({ size, capacity }: QueueBarProps) {
  if (capacity === 0) return null
  const pct = Math.min((size / capacity) * 100, 100)
  const color =
    pct >= 90 ? 'bg-red-500' :
    pct >= 70 ? 'bg-yellow-500' :
    pct >= 30 ? 'bg-blue-400' :
    'bg-green-500'

  return (
    <div className="mt-3 pt-3 border-t border-gray-700">
      <div className="flex items-center justify-between text-xs text-gray-400 mb-1">
        <span className="font-medium">Exporter queue</span>
        <span className="font-mono">
          <span className={pct >= 70 ? (pct >= 90 ? 'text-red-400' : 'text-yellow-400') : 'text-gray-300'}>
            {size.toFixed(0)}
          </span>
          <span className="text-gray-600"> / {capacity.toFixed(0)} slots ({pct.toFixed(0)}%)</span>
        </span>
      </div>
      <div className="h-1.5 rounded-full bg-gray-700 overflow-hidden">
        <div
          className={`h-full rounded-full transition-all duration-500 ${color}`}
          style={{ width: `${pct}%` }}
        />
      </div>
      {pct >= 70 && (
        <p className="mt-1 text-xs text-yellow-400">
          {pct >= 90
            ? 'Queue nearly full — backends can\'t keep up. Data loss is imminent.'
            : 'Queue filling up — monitor for drops if this continues.'}
        </p>
      )}
    </div>
  )
}

// ── main component ───────────────────────────────────────────────────────────

interface OtelFlowCardProps {
  pipeline: PipelineResponse
}

export function OtelFlowCard({ pipeline }: OtelFlowCardProps) {
  const { signals, extra } = pipeline

  const byType: Record<string, { received_pm: number; dropped_pm: number }> = {}
  for (const sig of signals) {
    byType[sig.type] = { received_pm: sig.received_pm, dropped_pm: sig.dropped_pm }
  }

  const rows: { type: string; label: string; icon: string; suffix: string }[] = [
    { type: 'metrics', label: 'Metrics',  icon: '📊', suffix: 'metric_points' },
    { type: 'logs',    label: 'Logs',     icon: '📄', suffix: 'log_records'   },
    { type: 'traces',  label: 'Traces',   icon: '🔍', suffix: 'spans'         },
  ]

  const hasAnyData = rows.some(r =>
    (byType[r.type]?.received_pm ?? 0) > 0 ||
    ex(extra, `exporter_sent_${r.suffix}_pm`) > 0,
  )

  return (
    <div className="mt-3 rounded-lg bg-gray-800/60 border border-gray-700 p-4">
      {/* Header */}
      <div className="flex items-center gap-2 mb-3">
        <span className="text-xs font-semibold uppercase tracking-wider text-gray-400">
          Pipeline flow
        </span>
        <div className="flex-1 h-px bg-gray-700" />
        <span className="text-xs text-gray-500">Receivers → Processors → Exporters</span>
      </div>

      {/* Column headers */}
      <div className="grid grid-cols-[90px_1fr_24px_1fr_24px_1fr] gap-x-2 mb-1">
        <div />
        <div className="text-right text-xs text-gray-500">Received/min</div>
        <div />
        <div className="text-center text-xs text-gray-500">Pipeline</div>
        <div />
        <div className="text-right text-xs text-gray-500">Exported/min</div>
      </div>

      {/* Divider */}
      <div className="border-t border-gray-700 mb-1" />

      {hasAnyData ? (
        rows.map(r => (
          <SignalRow
            key={r.type}
            label={r.label}
            icon={r.icon}
            receivedPm={byType[r.type]?.received_pm ?? 0}
            droppedPm={byType[r.type]?.dropped_pm ?? 0}
            sentPm={ex(extra, `exporter_sent_${r.suffix}_pm`)}
            refusedPm={ex(extra, `receiver_refused_${r.suffix}_pm`)}
            failedPm={ex(extra, `exporter_send_failed_${r.suffix}_pm`)}
          />
        ))
      ) : (
        <p className="py-2 text-sm text-gray-500 text-center">
          No signal traffic yet — waiting for instrumented apps to send data.
        </p>
      )}

      {/* Queue bar */}
      <QueueBar
        size={ex(extra, 'exporter_queue_size')}
        capacity={ex(extra, 'exporter_queue_capacity')}
      />
    </div>
  )
}
