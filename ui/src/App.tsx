import { BrowserRouter, Routes, Route, NavLink, useLocation } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { useWebSocket } from './hooks/useWebSocket'
import { useStore } from './store/useStore'
import { usePipelines } from './hooks/usePipelines'
import Dashboard from './pages/Dashboard'
import NotFound from './pages/NotFound'

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 2, refetchOnWindowFocus: false } },
})

// ─── SVG Icons ────────────────────────────────────────────────────────────────

function HexLogo() {
  return (
    <svg width="26" height="26" viewBox="0 0 28 28" fill="none">
      <path d="M14 2L25 8.5V21.5L14 26L3 21.5V8.5L14 2Z" stroke="#00d4ff" strokeWidth="1.5" />
      <path d="M14 7L21 11V19L14 22L7 19V11L14 7Z" stroke="#00d4ff" strokeWidth="1" opacity="0.4" />
      <circle cx="14" cy="14" r="2" fill="#00d4ff" />
    </svg>
  )
}

function IconOverview() {
  return (
    <svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5">
      <rect x="1" y="1" width="6" height="6" rx="1" />
      <rect x="9" y="1" width="6" height="6" rx="1" />
      <rect x="1" y="9" width="6" height="6" rx="1" />
      <rect x="9" y="9" width="6" height="6" rx="1" />
    </svg>
  )
}

function IconPipelines() {
  return (
    <svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5">
      <path d="M2 4h12M2 8h8M2 12h10" strokeLinecap="round" />
      <circle cx="13" cy="8" r="2" />
    </svg>
  )
}

function IconSignals() {
  return (
    <svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5">
      <path d="M1 11 L4 6 L7 9 L10 3 L13 7 L15 5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

function IconCerts() {
  return (
    <svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5">
      <path d="M8 1L10.5 3H14V6.5L15 8L14 9.5V13H10.5L8 15L5.5 13H2V9.5L1 8L2 6.5V3H5.5L8 1Z" />
      <path d="M6 8L7.5 9.5L10 6.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

function IconSettings() {
  return (
    <svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5">
      <circle cx="8" cy="8" r="2.5" />
      <path d="M8 1v2M8 13v2M1 8h2M13 8h2M3.05 3.05l1.41 1.41M11.54 11.54l1.41 1.41M3.05 12.95l1.41-1.41M11.54 4.46l1.41-1.41" strokeLinecap="round" />
    </svg>
  )
}

// ─── State dot ────────────────────────────────────────────────────────────────

function StateDot({ state }: { state: string }) {
  const config: Record<string, { color: string; glow: string }> = {
    healthy:  { color: '#00e676', glow: '0 0 6px #00e676' },
    degraded: { color: '#ffab40', glow: '0 0 6px #ffab40' },
    critical: { color: '#ff4f6a', glow: '0 0 6px #ff4f6a' },
    unknown:  { color: '#4a6080', glow: 'none' },
  }
  const c = config[state.toLowerCase()] ?? config.unknown
  return (
    <span
      className="inline-block h-1.5 w-1.5 rounded-full flex-shrink-0"
      style={{ background: c.color, boxShadow: c.glow }}
    />
  )
}

// ─── Sidebar ──────────────────────────────────────────────────────────────────

interface NavItemProps {
  to: string
  icon: React.ReactNode
  label: string
  badge?: string
  badgeColor?: string
  disabled?: boolean
}

function NavItem({ to, icon, label, badge, badgeColor = '#ff4f6a', disabled = false }: NavItemProps) {
  if (disabled) {
    return (
      <div className="flex items-center gap-2.5 rounded-md px-2 py-[7px] text-obs-muted cursor-not-allowed select-none">
        <span className="opacity-50">{icon}</span>
        <span className="flex-1 text-[12px]">{label}</span>
      </div>
    )
  }
  return (
    <NavLink
      to={to}
      end={to === '/'}
      className={({ isActive }) =>
        `flex items-center gap-2.5 rounded-md px-2 py-[7px] transition-all duration-150 ${
          isActive
            ? 'text-obs-accent bg-[rgba(0,212,255,0.06)] border border-[rgba(0,212,255,0.15)]'
            : 'text-obs-muted2 hover:text-obs-text hover:bg-white/[0.03] border border-transparent'
        }`
      }
    >
      <span className="opacity-70 group-[.active]:opacity-100">{icon}</span>
      <span className="flex-1 text-[12px]">{label}</span>
      {badge && (
        <span
          className="text-[10px] font-semibold px-1.5 py-0.5 rounded-full"
          style={{ background: `${badgeColor}20`, color: badgeColor, border: `1px solid ${badgeColor}30` }}
        >
          {badge}
        </span>
      )}
    </NavLink>
  )
}

function Sidebar() {
  const liveSnapshot = useStore((s) => s.liveSnapshot)
  const { data: pipelines } = usePipelines()
  const sources = liveSnapshot?.pipelines ?? pipelines ?? []

  return (
    <aside
      className="w-[220px] shrink-0 border-r overflow-y-auto"
      style={{ background: '#0d1117', borderColor: '#1e2d3d', minHeight: 'calc(100vh - 56px)' }}
    >
      {/* MONITORING */}
      <div className="px-4 pt-5 pb-3">
        <p className="text-[10px] font-semibold tracking-[1.5px] uppercase text-obs-muted mb-2">Monitoring</p>
        <nav className="space-y-0.5">
          <NavItem to="/"          icon={<IconOverview />}   label="Overview" />
          <NavItem to="/pipelines" icon={<IconPipelines />}  label="Pipelines" disabled />
          <NavItem to="/signals"   icon={<IconSignals />}    label="Signals"   disabled />
        </nav>
      </div>

      {/* SECURITY */}
      <div className="px-4 pb-3">
        <p className="text-[10px] font-semibold tracking-[1.5px] uppercase text-obs-muted mb-2">Security</p>
        <nav className="space-y-0.5">
          <NavItem to="/certs" icon={<IconCerts />}    label="Certificates" disabled />
          <NavItem to="/settings" icon={<IconSettings />} label="Settings"  disabled />
        </nav>
      </div>

      {/* SOURCES */}
      {sources.length > 0 && (
        <div className="px-4 pb-5">
          <p className="text-[10px] font-semibold tracking-[1.5px] uppercase text-obs-muted mb-2">Sources</p>
          <div className="space-y-1">
            {sources.map((p) => (
              <div
                key={p.source_id}
                className="flex items-center gap-2 px-2 py-1.5 rounded-md hover:bg-white/[0.025] transition-colors cursor-default"
              >
                <StateDot state={p.state} />
                <div className="flex-1 min-w-0">
                  <p className="text-[11px] text-obs-text truncate">{p.source_id}</p>
                  <p className="text-[10px] text-obs-muted">{p.source_type}</p>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {sources.length === 0 && (
        <div className="px-4 pb-5">
          <p className="text-[10px] font-semibold tracking-[1.5px] uppercase text-obs-muted mb-2">Sources</p>
          <p className="text-[11px] text-obs-muted px-2">No sources yet</p>
        </div>
      )}
    </aside>
  )
}

// ─── Header ───────────────────────────────────────────────────────────────────

function Header() {
  const wsConnected = useStore((s) => s.wsConnected)
  const [utc, setUtc] = useState('')
  const location = useLocation()

  useEffect(() => {
    const tick = () => setUtc(new Date().toUTCString().replace('GMT', 'UTC').split(' ').slice(1, 5).join(' '))
    tick()
    const id = setInterval(tick, 1000)
    return () => clearInterval(id)
  }, [])

  // Derive page title from route
  const pageTitle: Record<string, string> = { '/': 'Overview', '/pipelines': 'Pipelines', '/signals': 'Signals', '/certs': 'Certificates' }
  const title = pageTitle[location.pathname] ?? 'ObsidianStack'

  return (
    <header
      className="sticky top-0 z-50 h-14 flex items-center justify-between px-5 border-b"
      style={{ background: 'rgba(8,12,16,0.95)', backdropFilter: 'blur(12px)', borderColor: '#1e2d3d' }}
    >
      {/* Logo */}
      <div className="flex items-center gap-2.5">
        <HexLogo />
        <span className="font-syne text-[17px] font-extrabold tracking-tight" style={{ letterSpacing: '-0.5px' }}>
          Obsidian<span style={{ color: '#00d4ff' }}>Stack</span>
        </span>
      </div>

      {/* Right cluster */}
      <div className="flex items-center gap-4">
        <span className="text-[11px] text-obs-muted2 hidden sm:block">{utc}</span>

        {/* Live / Polling badge */}
        {wsConnected ? (
          <div
            className="flex items-center gap-1.5 px-2.5 py-1 rounded text-[11px] font-semibold"
            style={{ background: 'rgba(0,230,118,0.08)', border: '1px solid rgba(0,230,118,0.2)', color: '#00e676' }}
          >
            <span className="h-1.5 w-1.5 rounded-full animate-pulse-dot" style={{ background: '#00e676' }} />
            LIVE
          </div>
        ) : (
          <div
            className="flex items-center gap-1.5 px-2.5 py-1 rounded text-[11px] font-semibold text-obs-muted2"
            style={{ background: 'rgba(255,255,255,0.03)', border: '1px solid rgba(255,255,255,0.08)' }}
          >
            <span className="h-1.5 w-1.5 rounded-full bg-obs-muted" />
            POLLING
          </div>
        )}

        {/* Buttons */}
        <div className="hidden sm:flex items-center gap-2">
          <button
            className="px-3 py-1.5 rounded text-[11px] font-semibold text-obs-muted2 transition-colors hover:text-obs-accent"
            style={{ border: '1px solid #243447' }}
          >
            + Add Source
          </button>
          <button
            className="px-3 py-1.5 rounded text-[11px] font-semibold transition-colors hover:brightness-110"
            style={{ background: '#00d4ff', color: '#080c10' }}
          >
            Export JSON
          </button>
        </div>
      </div>
    </header>
  )
}

// ─── Shell ────────────────────────────────────────────────────────────────────

function AppShell() {
  useWebSocket()

  return (
    <div className="flex flex-col min-h-screen" style={{ background: '#080c10', color: '#e8f1ff' }}>
      <Header />
      <div className="flex flex-1">
        <Sidebar />
        <main className="flex-1 overflow-y-auto p-6" style={{ background: '#080c10' }}>
          <Routes>
            <Route path="/" element={<Dashboard />} />
            <Route path="*" element={<NotFound />} />
          </Routes>
        </main>
      </div>
    </div>
  )
}

// ─── App ──────────────────────────────────────────────────────────────────────

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <AppShell />
      </BrowserRouter>
    </QueryClientProvider>
  )
}
