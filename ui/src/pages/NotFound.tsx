import { Link } from 'react-router-dom'

export default function NotFound() {
  return (
    <div className="flex flex-col items-center justify-center min-h-[60vh] text-center">
      <p className="font-syne text-6xl font-extrabold" style={{ color: '#1e2d3d' }}>404</p>
      <h1 className="mt-4 font-syne text-2xl font-bold text-obs-text">Page not found</h1>
      <p className="mt-2 text-[12px] text-obs-muted">The page you are looking for does not exist.</p>
      <Link
        to="/"
        className="mt-6 rounded px-4 py-2 text-[12px] font-semibold transition-colors hover:brightness-110"
        style={{ background: '#0d1117', border: '1px solid #243447', color: '#00d4ff' }}
      >
        ← Back to Overview
      </Link>
    </div>
  )
}
