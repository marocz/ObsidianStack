import { useState, useEffect } from 'react'

type SourceType = 'prometheus' | 'loki' | 'otelcol' | 'http'
type AuthMode   = 'none' | 'basic' | 'apikey' | 'bearer' | 'mtls'

interface FormState {
  id:        string
  type:      SourceType
  endpoint:  string
  authMode:  AuthMode
  // basic auth fields
  username:    string
  passwordEnv: string
  // apikey fields
  header:    string
  keyEnv:    string
  // bearer fields
  tokenEnv:  string
  // mtls fields
  certFile:  string
  keyFile:   string
  caFile:    string
  // tls
  skipVerify: boolean
}

const DEFAULT: FormState = {
  id:          '',
  type:        'prometheus',
  endpoint:    '',
  authMode:    'none',
  username:    '',
  passwordEnv: '',
  header:      'X-API-Key',
  keyEnv:      '',
  tokenEnv:    '',
  certFile:    '/etc/certs/client.crt',
  keyFile:     '/etc/certs/client.key',
  caFile:      '/etc/certs/ca.crt',
  skipVerify:  false,
}

const TYPE_PLACEHOLDER: Record<SourceType, string> = {
  prometheus: 'http://prometheus:9090/metrics',
  loki:       'http://loki:3100/metrics',
  otelcol:    'http://otelcol:8888/metrics',
  http:       'https://your-service/health',
}

const TYPE_HELP: Record<SourceType, string> = {
  prometheus: 'Scrapes Prometheus internal metrics: TSDB ingestion rate, remote write queues, WAL errors.',
  loki:       'Scrapes Loki distributor/ingester metrics: lines received, flush failures, ring health.',
  otelcol:    'Scrapes OTel Collector internal metrics: spans/metrics/logs received vs dropped per pipeline.',
  http:       'Polls an HTTP endpoint. Used for external services (Grafana, SaaS checks, custom health endpoints).',
}

function generateYAML(f: FormState): string {
  const lines: string[] = [
    `    - id: "${f.id || 'my-source'}"`,
    `      type: ${f.type}`,
    `      endpoint: "${f.endpoint || TYPE_PLACEHOLDER[f.type]}"`,
  ]

  if (f.authMode !== 'none') {
    lines.push(`      auth:`)
    lines.push(`        mode: ${f.authMode}`)
    if (f.authMode === 'basic') {
      lines.push(`        username: "${f.username || 'admin'}"`)
      lines.push(`        password_env: ${f.passwordEnv || 'SOURCE_PASSWORD'}`)
    } else if (f.authMode === 'apikey') {
      lines.push(`        header: "${f.header}"`)
      lines.push(`        key_env: ${f.keyEnv || 'SOURCE_API_KEY'}`)
    } else if (f.authMode === 'bearer') {
      lines.push(`        token_env: ${f.tokenEnv || 'SOURCE_BEARER_TOKEN'}`)
    } else if (f.authMode === 'mtls') {
      lines.push(`        cert_file: ${f.certFile}`)
      lines.push(`        key_file: ${f.keyFile}`)
      lines.push(`        ca_file: ${f.caFile}`)
    }
  }

  if (f.skipVerify) {
    lines.push(`      tls:`)
    lines.push(`        insecure_skip_verify: true`)
  }

  return lines.join('\n')
}

interface Props {
  onClose: () => void
}

export default function AddSourceModal({ onClose }: Props) {
  const [form, setForm]       = useState<FormState>(DEFAULT)
  const [copied, setCopied]   = useState(false)
  const [step, setStep]       = useState<1 | 2>(1)

  // Close on Escape
  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [onClose])

  function set<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((f) => ({ ...f, [key]: value }))
  }

  const yaml = generateYAML(form)

  function copyYAML() {
    navigator.clipboard.writeText(yaml).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

  const isValid = form.id.trim() !== '' && form.endpoint.trim() !== ''

  return (
    /* Backdrop */
    <div
      className="fixed inset-0 z-[100] flex items-center justify-center p-4"
      style={{ background: 'rgba(8,12,16,0.85)', backdropFilter: 'blur(4px)' }}
      onClick={(e) => { if (e.target === e.currentTarget) onClose() }}
    >
      <div
        className="w-full max-w-xl rounded-xl overflow-hidden shadow-2xl"
        style={{ background: '#0d1117', border: '1px solid #1e2d3d' }}
      >
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-4" style={{ borderBottom: '1px solid #1e2d3d' }}>
          <div>
            <h2 className="font-syne text-[15px] font-bold text-obs-text">Add Source</h2>
            <p className="text-[11px] text-obs-muted mt-0.5">
              Configure a pipeline component to monitor
            </p>
          </div>
          <button
            onClick={onClose}
            className="h-7 w-7 rounded flex items-center justify-center text-obs-muted hover:text-obs-text transition-colors"
            style={{ background: 'rgba(255,255,255,0.04)' }}
          >
            ✕
          </button>
        </div>

        {/* Step tabs */}
        <div className="flex px-5 pt-4 gap-1">
          {([1, 2] as const).map((s) => (
            <button
              key={s}
              onClick={() => { if (s === 2 && !isValid) return; setStep(s) }}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[11px] font-semibold transition-all"
              style={
                step === s
                  ? { background: 'rgba(0,212,255,0.1)', color: '#00d4ff', border: '1px solid rgba(0,212,255,0.2)' }
                  : { color: '#4a6080', background: 'transparent', border: '1px solid transparent' }
              }
            >
              <span
                className="h-4 w-4 rounded-full text-[9px] font-bold flex items-center justify-center"
                style={{ background: step === s ? '#00d4ff' : '#1e2d3d', color: step === s ? '#080c10' : '#4a6080' }}
              >
                {s}
              </span>
              {s === 1 ? 'Configure' : 'Apply'}
            </button>
          ))}
        </div>

        <div className="px-5 pb-5 pt-4 space-y-4">
          {step === 1 && (
            <>
              {/* Source type */}
              <div className="grid grid-cols-4 gap-2">
                {(['prometheus', 'loki', 'otelcol', 'http'] as SourceType[]).map((t) => (
                  <button
                    key={t}
                    onClick={() => { set('type', t); set('endpoint', '') }}
                    className="rounded-lg p-3 text-center transition-all"
                    style={
                      form.type === t
                        ? { background: 'rgba(0,212,255,0.08)', border: '1px solid rgba(0,212,255,0.25)', color: '#00d4ff' }
                        : { background: '#111820', border: '1px solid #1e2d3d', color: '#6b8ba8' }
                    }
                  >
                    <p className="text-[11px] font-semibold">{t}</p>
                  </button>
                ))}
              </div>

              <p className="text-[11px] text-obs-muted px-1">{TYPE_HELP[form.type]}</p>

              {/* ID + Endpoint */}
              <div className="grid grid-cols-2 gap-3">
                <Field label="Source ID" required>
                  <Input
                    value={form.id}
                    placeholder="prometheus-prod"
                    onChange={(v) => set('id', v)}
                  />
                </Field>
                <Field label="Endpoint URL" required>
                  <Input
                    value={form.endpoint}
                    placeholder={TYPE_PLACEHOLDER[form.type]}
                    onChange={(v) => set('endpoint', v)}
                  />
                </Field>
              </div>

              {/* Auth mode */}
              <Field label="Authentication">
                <div className="flex gap-2">
                  {(['none', 'basic', 'apikey', 'bearer', 'mtls'] as AuthMode[]).map((m) => (
                    <button
                      key={m}
                      onClick={() => set('authMode', m)}
                      className="flex-1 py-1.5 rounded text-[11px] font-semibold transition-all"
                      style={
                        form.authMode === m
                          ? { background: 'rgba(123,97,255,0.12)', border: '1px solid rgba(123,97,255,0.3)', color: '#7b61ff' }
                          : { background: '#111820', border: '1px solid #1e2d3d', color: '#4a6080' }
                      }
                    >
                      {m}
                    </button>
                  ))}
                </div>
              </Field>

              {/* Conditional auth fields */}
              {form.authMode === 'basic' && (
                <div className="grid grid-cols-2 gap-3">
                  <Field label="Username">
                    <Input value={form.username} placeholder="admin" onChange={(v) => set('username', v)} />
                  </Field>
                  <Field label="Password env var">
                    <Input value={form.passwordEnv} placeholder="PROM_PASSWORD" onChange={(v) => set('passwordEnv', v)} />
                  </Field>
                </div>
              )}
              {form.authMode === 'apikey' && (
                <div className="grid grid-cols-2 gap-3">
                  <Field label="Header name">
                    <Input value={form.header} placeholder="X-API-Key" onChange={(v) => set('header', v)} />
                  </Field>
                  <Field label="Key env var">
                    <Input value={form.keyEnv} placeholder="MY_API_KEY" onChange={(v) => set('keyEnv', v)} />
                  </Field>
                </div>
              )}
              {form.authMode === 'bearer' && (
                <Field label="Token env var">
                  <Input value={form.tokenEnv} placeholder="MY_BEARER_TOKEN" onChange={(v) => set('tokenEnv', v)} />
                </Field>
              )}
              {form.authMode === 'mtls' && (
                <div className="space-y-2">
                  <Field label="Client cert path">
                    <Input value={form.certFile} placeholder="/etc/certs/client.crt" onChange={(v) => set('certFile', v)} />
                  </Field>
                  <Field label="Client key path">
                    <Input value={form.keyFile} placeholder="/etc/certs/client.key" onChange={(v) => set('keyFile', v)} />
                  </Field>
                  <Field label="CA cert path">
                    <Input value={form.caFile} placeholder="/etc/certs/ca.crt" onChange={(v) => set('caFile', v)} />
                  </Field>
                </div>
              )}

              {/* TLS skip verify */}
              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  checked={form.skipVerify}
                  onChange={(e) => set('skipVerify', e.target.checked)}
                  className="accent-obs-accent"
                />
                <span className="text-[11px] text-obs-muted2">Skip TLS verification (dev only)</span>
              </label>

              <button
                onClick={() => setStep(2)}
                disabled={!isValid}
                className="w-full py-2.5 rounded-lg text-[12px] font-semibold transition-all"
                style={
                  isValid
                    ? { background: '#00d4ff', color: '#080c10' }
                    : { background: '#111820', color: '#4a6080', cursor: 'not-allowed', border: '1px solid #1e2d3d' }
                }
              >
                Generate config →
              </button>
            </>
          )}

          {step === 2 && (
            <>
              <div
                className="rounded-lg overflow-hidden"
                style={{ background: '#060a0f', border: '1px solid #1e2d3d' }}
              >
                {/* Code header */}
                <div className="flex items-center justify-between px-4 py-2.5" style={{ borderBottom: '1px solid #1e2d3d' }}>
                  <span className="text-[10px] font-semibold text-obs-muted uppercase tracking-wide">agent config.yaml</span>
                  <button
                    onClick={copyYAML}
                    className="text-[11px] font-semibold px-2.5 py-1 rounded transition-colors"
                    style={
                      copied
                        ? { background: 'rgba(0,230,118,0.12)', color: '#00e676', border: '1px solid rgba(0,230,118,0.2)' }
                        : { background: 'rgba(255,255,255,0.05)', color: '#6b8ba8', border: '1px solid #1e2d3d' }
                    }
                  >
                    {copied ? '✓ Copied' : 'Copy'}
                  </button>
                </div>

                {/* YAML */}
                <pre
                  className="px-4 py-4 text-[12px] leading-relaxed overflow-x-auto"
                  style={{ fontFamily: '"JetBrains Mono", monospace', color: '#e8f1ff' }}
                >
                  <span style={{ color: '#4a6080' }}># Add this block under agent.sources in your config.yaml{'\n'}</span>
                  <span style={{ color: '#4a6080' }}>agent:{'\n'}  sources:{'\n'}</span>
                  {yaml.split('\n').map((line, i) => (
                    <span key={i}>
                      {colorizeYAMLLine(line)}{'\n'}
                    </span>
                  ))}
                </pre>
              </div>

              {/* Apply instructions */}
              <div
                className="rounded-lg p-4 space-y-3"
                style={{ background: 'rgba(0,212,255,0.04)', border: '1px solid rgba(0,212,255,0.12)' }}
              >
                <p className="text-[11px] font-semibold text-obs-accent">How to apply</p>
                <ol className="space-y-2 text-[11px] text-obs-muted2">
                  <li className="flex gap-2">
                    <span className="text-obs-accent font-bold flex-shrink-0">1.</span>
                    Copy the snippet above and paste it into your agent's <code className="text-obs-accent px-1 rounded" style={{ background: 'rgba(0,212,255,0.08)' }}>config.yaml</code> under <code className="text-obs-accent px-1 rounded" style={{ background: 'rgba(0,212,255,0.08)' }}>agent.sources</code>
                  </li>
                  <li className="flex gap-2">
                    <span className="text-obs-accent font-bold flex-shrink-0">2.</span>
                    Save the file — the agent hot-reloads automatically within seconds, no restart needed.
                  </li>
                  <li className="flex gap-2">
                    <span className="text-obs-accent font-bold flex-shrink-0">3.</span>
                    The new pipeline will appear on this dashboard after the next scrape interval (default 15s).
                  </li>
                </ol>
              </div>

              <div className="flex gap-2">
                <button
                  onClick={() => setStep(1)}
                  className="flex-1 py-2 rounded text-[12px] font-semibold transition-colors"
                  style={{ background: '#111820', border: '1px solid #1e2d3d', color: '#6b8ba8' }}
                >
                  ← Edit
                </button>
                <button
                  onClick={onClose}
                  className="flex-1 py-2 rounded text-[12px] font-semibold transition-colors hover:brightness-110"
                  style={{ background: '#00d4ff', color: '#080c10' }}
                >
                  Done
                </button>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  )
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

function Field({ label, required, children }: { label: string; required?: boolean; children: React.ReactNode }) {
  return (
    <div className="space-y-1.5">
      <label className="text-[10px] font-semibold uppercase tracking-wide text-obs-muted">
        {label}{required && <span style={{ color: '#ff4f6a' }}> *</span>}
      </label>
      {children}
    </div>
  )
}

function Input({ value, placeholder, onChange }: { value: string; placeholder?: string; onChange: (v: string) => void }) {
  return (
    <input
      type="text"
      value={value}
      placeholder={placeholder}
      onChange={(e) => onChange(e.target.value)}
      className="w-full rounded px-3 py-2 text-[12px] text-obs-text outline-none transition-colors"
      style={{
        background: '#111820',
        border: '1px solid #1e2d3d',
        fontFamily: '"JetBrains Mono", monospace',
      }}
      onFocus={(e) => (e.currentTarget.style.borderColor = 'rgba(0,212,255,0.4)')}
      onBlur={(e) => (e.currentTarget.style.borderColor = '#1e2d3d')}
    />
  )
}

function colorizeYAMLLine(line: string): React.ReactNode {
  // Very minimal YAML colorizer: key: value
  const keyMatch = line.match(/^(\s+)(- )?(\w[\w_-]*)(:.*)$/)
  if (!keyMatch) return <span style={{ color: '#e8f1ff' }}>{line}</span>

  const [, indent, bullet, key, rest] = keyMatch
  const valueColor = rest.startsWith(': "') ? '#00e676' :
                     rest.startsWith(': ') && !rest.startsWith(': "') ? '#ffab40' : '#e8f1ff'
  return (
    <>
      <span style={{ color: '#e8f1ff' }}>{indent}{bullet ?? ''}</span>
      <span style={{ color: '#00d4ff' }}>{key}</span>
      <span style={{ color: valueColor }}>{rest}</span>
    </>
  )
}
