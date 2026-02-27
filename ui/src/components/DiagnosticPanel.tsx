import { useState, useRef, useEffect } from 'react'
import type { DiagnosticHint } from '../api/types'

// ─── Level config ──────────────────────────────────────────────────────────

const LEVEL: Record<DiagnosticHint['level'], { color: string; bg: string; border: string; dot: string }> = {
  ok:       { color: '#00e676', bg: 'rgba(0,230,118,0.08)',   border: 'rgba(0,230,118,0.2)',   dot: '#00e676' },
  info:     { color: '#00d4ff', bg: 'rgba(0,212,255,0.08)',   border: 'rgba(0,212,255,0.2)',   dot: '#00d4ff' },
  warning:  { color: '#ffab40', bg: 'rgba(255,171,64,0.08)',  border: 'rgba(255,171,64,0.2)',  dot: '#ffab40' },
  critical: { color: '#ff4f6a', bg: 'rgba(255,79,106,0.08)',  border: 'rgba(255,79,106,0.2)',  dot: '#ff4f6a' },
}

// ─── Tooltip ──────────────────────────────────────────────────────────────

function Tooltip({ text, children }: { text: string; children: React.ReactNode }) {
  const [open, setOpen] = useState(false)
  const [pos, setPos] = useState<'top' | 'bottom'>('top')
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open || !ref.current) return
    const rect = ref.current.getBoundingClientRect()
    setPos(rect.top < 200 ? 'bottom' : 'top')
  }, [open])

  return (
    <div
      ref={ref}
      className="relative inline-block"
      onMouseEnter={() => setOpen(true)}
      onMouseLeave={() => setOpen(false)}
    >
      {children}
      {open && (
        <div
          className="absolute z-50 w-72 rounded-lg p-3 shadow-xl"
          style={{
            background: '#111827',
            border: '1px solid #1e2d3d',
            ...(pos === 'top'
              ? { bottom: 'calc(100% + 6px)', left: '50%', transform: 'translateX(-50%)' }
              : { top: 'calc(100% + 6px)', left: '50%', transform: 'translateX(-50%)' }),
          }}
        >
          {/* Arrow */}
          <div
            className="absolute left-1/2 -translate-x-1/2 w-0 h-0"
            style={
              pos === 'top'
                ? { bottom: -5, borderLeft: '5px solid transparent', borderRight: '5px solid transparent', borderTop: '5px solid #1e2d3d' }
                : { top: -5, borderLeft: '5px solid transparent', borderRight: '5px solid transparent', borderBottom: '5px solid #1e2d3d' }
            }
          />
          <p className="text-[12px] leading-relaxed text-[#b8c9e0]">{text}</p>
        </div>
      )}
    </div>
  )
}

// ─── Chip ─────────────────────────────────────────────────────────────────

function DiagChip({ hint }: { hint: DiagnosticHint }) {
  const lc = LEVEL[hint.level]
  return (
    <Tooltip text={hint.detail}>
      <div
        className="flex items-center gap-1.5 px-2 py-1 rounded-full cursor-help select-none transition-all"
        style={{ background: lc.bg, border: `1px solid ${lc.border}` }}
      >
        <span className="h-1.5 w-1.5 rounded-full flex-shrink-0" style={{ background: lc.dot }} />
        <span className="text-[11px] font-medium whitespace-nowrap" style={{ color: lc.color }}>
          {hint.title}
        </span>
      </div>
    </Tooltip>
  )
}

// ─── Expanded detail card ─────────────────────────────────────────────────

function DiagCard({ hint, onClose }: { hint: DiagnosticHint; onClose: () => void }) {
  const lc = LEVEL[hint.level]
  return (
    <div
      className="rounded-lg p-4 mt-3 relative"
      style={{ background: lc.bg, border: `1px solid ${lc.border}` }}
    >
      <button
        onClick={onClose}
        className="absolute top-2 right-2 text-obs-muted hover:text-obs-text text-[14px] leading-none px-1"
      >
        ×
      </button>
      <div className="flex items-center gap-2 mb-2">
        <span className="h-2 w-2 rounded-full" style={{ background: lc.dot }} />
        <span className="text-[12px] font-semibold" style={{ color: lc.color }}>{hint.title}</span>
      </div>
      {/* AI-assistant style explanation */}
      <p className="text-[12px] leading-relaxed text-[#b8c9e0]">{hint.detail}</p>
    </div>
  )
}

// ─── Full panel (chips row + expandable detail) ────────────────────────────

interface DiagnosticPanelProps {
  diagnostics: DiagnosticHint[]
  /** If true, shows chips inline. If false, shows a compact summary chip. */
  inline?: boolean
}

export function DiagnosticPanel({ diagnostics, inline = true }: DiagnosticPanelProps) {
  const [activeKey, setActiveKey] = useState<string | null>(null)

  if (!diagnostics || diagnostics.length === 0) return null

  const active = diagnostics.find((d) => d.key === activeKey) ?? null

  // Sort: critical > warning > info > ok
  const order = { critical: 0, warning: 1, info: 2, ok: 3 }
  const sorted = [...diagnostics].sort((a, b) => (order[a.level] ?? 9) - (order[b.level] ?? 9))

  if (!inline) {
    // Compact mode: single chip showing worst issue
    const worst = sorted[0]
    const lc = LEVEL[worst.level]
    return (
      <Tooltip text={worst.detail}>
        <div
          className="flex items-center gap-1.5 px-2 py-0.5 rounded-full cursor-help"
          style={{ background: lc.bg, border: `1px solid ${lc.border}` }}
        >
          <span className="h-1.5 w-1.5 rounded-full" style={{ background: lc.dot }} />
          <span className="text-[10px] font-medium" style={{ color: lc.color }}>
            {diagnostics.length > 1 ? `${diagnostics.length} insights` : worst.title}
          </span>
        </div>
      </Tooltip>
    )
  }

  return (
    <div>
      {/* Chips row */}
      <div className="flex flex-wrap gap-1.5">
        {sorted.map((hint) => (
          <div
            key={hint.key}
            onClick={() => setActiveKey(activeKey === hint.key ? null : hint.key)}
          >
            <DiagChip hint={hint} />
          </div>
        ))}
      </div>

      {/* Expanded detail on click */}
      {active && <DiagCard hint={active} onClose={() => setActiveKey(null)} />}
    </div>
  )
}

// ─── Drawer (full diagnostic view for a pipeline) ─────────────────────────

interface DiagDrawerProps {
  sourceId: string
  sourceType: string
  diagnostics: DiagnosticHint[]
  onClose: () => void
}

export function DiagDrawer({ sourceId, sourceType, diagnostics, onClose }: DiagDrawerProps) {
  const order = { critical: 0, warning: 1, info: 2, ok: 3 }
  const sorted = [...diagnostics].sort((a, b) => (order[a.level] ?? 9) - (order[b.level] ?? 9))

  return (
    <div
      className="fixed inset-y-0 right-0 z-50 w-[420px] flex flex-col shadow-2xl"
      style={{ background: '#0d1117', borderLeft: '1px solid #1e2d3d' }}
    >
      {/* Header */}
      <div
        className="flex items-center justify-between px-5 py-4 border-b"
        style={{ borderColor: '#1e2d3d' }}
      >
        <div>
          <p className="text-[14px] font-semibold text-obs-text">{sourceId}</p>
          <p className="text-[11px] text-obs-muted mt-0.5">{sourceType} · diagnostics</p>
        </div>
        <button
          onClick={onClose}
          className="text-obs-muted hover:text-obs-text transition-colors text-[18px] leading-none px-1"
        >
          ×
        </button>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-5 space-y-3">
        <p className="text-[11px] text-obs-muted leading-relaxed">
          These insights are generated from your live pipeline data. Hover over a chip for a quick
          summary, or read the full explanation below each one.
        </p>

        {sorted.map((hint) => {
          const lc = LEVEL[hint.level]
          return (
            <div
              key={hint.key}
              className="rounded-lg p-4"
              style={{ background: lc.bg, border: `1px solid ${lc.border}` }}
            >
              <div className="flex items-center gap-2 mb-2">
                <span
                  className="h-2 w-2 rounded-full flex-shrink-0"
                  style={{ background: lc.dot, boxShadow: `0 0 6px ${lc.dot}` }}
                />
                <span className="text-[12px] font-semibold" style={{ color: lc.color }}>
                  {hint.title}
                </span>
                {hint.value !== undefined && (
                  <span className="ml-auto text-[11px] font-mono" style={{ color: lc.color }}>
                    {hint.value.toFixed(1)}
                  </span>
                )}
              </div>
              {/* Plain-English explanation */}
              <p className="text-[12px] leading-relaxed text-[#b8c9e0]">{hint.detail}</p>
            </div>
          )
        })}
      </div>

      {/* Footer */}
      <div
        className="px-5 py-3 border-t text-[11px] text-obs-muted"
        style={{ borderColor: '#1e2d3d' }}
      >
        Diagnostics refresh every 15 seconds with the next agent scrape.
      </div>
    </div>
  )
}
