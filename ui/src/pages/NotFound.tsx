import { Link } from 'react-router-dom'

export default function NotFound() {
  return (
    <div className="flex flex-col items-center justify-center min-h-[60vh] text-center">
      <p className="text-6xl font-bold text-obsidian-600">404</p>
      <h1 className="mt-4 text-2xl font-semibold text-white">Page not found</h1>
      <p className="mt-2 text-sm text-slate-400">The page you are looking for does not exist.</p>
      <Link
        to="/"
        className="mt-6 rounded-lg bg-obsidian-700 px-4 py-2 text-sm font-medium text-white ring-1 ring-white/10 hover:bg-obsidian-600 transition-colors"
      >
        Back to Dashboard
      </Link>
    </div>
  )
}
