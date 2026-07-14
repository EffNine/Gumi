import { useEffect, useMemo, useState } from 'react'
import {
  Activity, Braces, CheckCircle2, ChevronRight, CircleAlert, Cpu,
  FileSliders, Gauge, HeartPulse, ListTree, RefreshCw, Server,
  ShieldCheck, TerminalSquare, TimerReset, Wrench,
} from 'lucide-react'

type Provider = { name: string; status: string; url: string }
type Status = {
  runtime: { status: string; version: string; mode: string; api_url: string }
  providers: Provider[]
  storage_status: string
  telemetry_enabled: boolean
}
type RequestRow = {
  id: string; created_at: string; runtime_mode: string; provider: string; model: string
  status: string; latency_ms: number; validation_passed: boolean; repair_applied: boolean
  retry_count: number; error_code?: string
}
type Doctor = { status: string; checks: { name: string; status: string; message: string; suggestion?: string }[] }
type Profile = { id: string; name: string; family: string; size: string; context_limit: number; aliases?: string[]; notes?: string[] }
type Page = 'overview' | 'requests' | 'providers' | 'profiles' | 'config' | 'doctor'

const nav: { id: Page; label: string; icon: typeof Activity }[] = [
  { id: 'overview', label: 'Overview', icon: Activity },
  { id: 'requests', label: 'Requests', icon: ListTree },
  { id: 'providers', label: 'Providers', icon: Server },
  { id: 'profiles', label: 'Profiles', icon: Braces },
  { id: 'config', label: 'Config', icon: FileSliders },
  { id: 'doctor', label: 'Doctor', icon: Wrench },
]

async function getJSON<T>(path: string): Promise<T> {
  const response = await fetch(`/api${path}`)
  if (!response.ok) throw new Error(`${response.status} ${response.statusText}`)
  return response.json() as Promise<T>
}

function StatusMark({ status }: { status: string }) {
  const good = ['ok', 'running', 'success', 'repaired'].includes(status)
  return <span className={`status-mark ${good ? 'good' : status === 'warning' || status === 'degraded' ? 'warn' : 'bad'}`}><i />{status}</span>
}

function App() {
  const [page, setPage] = useState<Page>('overview')
  const [status, setStatus] = useState<Status | null>(null)
  const [requests, setRequests] = useState<RequestRow[]>([])
  const [doctor, setDoctor] = useState<Doctor | null>(null)
  const [profiles, setProfiles] = useState<Profile[]>([])
  const [config, setConfig] = useState<object | null>(null)
  const [error, setError] = useState('')
  const [refreshing, setRefreshing] = useState(false)

  const load = async () => {
    setRefreshing(true)
    try {
      const [s, r, d, p, c] = await Promise.all([
        getJSON<Status>('/v1/gumi/status'),
        getJSON<{ data: RequestRow[] }>('/v1/gumi/telemetry/recent'),
        getJSON<Doctor>('/v1/gumi/doctor'),
        getJSON<{ data: Profile[] }>('/v1/gumi/profiles'),
        getJSON<object>('/v1/gumi/config'),
      ])
      setStatus(s); setRequests(r.data ?? []); setDoctor(d); setProfiles(p.data ?? []); setConfig(c); setError('')
    } catch (e) { setError(e instanceof Error ? e.message : 'Runtime unavailable') }
    finally { setRefreshing(false) }
  }

  useEffect(() => { void load(); const id = window.setInterval(load, 15000); return () => clearInterval(id) }, [])

  const metrics = useMemo(() => {
    const success = requests.filter((r) => ['success', 'repaired', 'retried'].includes(r.status)).length
    const avg = requests.length ? Math.round(requests.reduce((sum, r) => sum + r.latency_ms, 0) / requests.length) : 0
    return { success: requests.length ? Math.round((success / requests.length) * 100) : 100, avg, repairs: requests.filter((r) => r.repair_applied).length }
  }, [requests])

  return <div className="app-shell">
    <aside className="sidebar">
      <div className="brand"><span className="brand-mark">G</span><div><strong>Gumi</strong><small>Runtime console</small></div></div>
      <nav>{nav.map(({ id, label, icon: Icon }) => <button key={id} className={page === id ? 'active' : ''} onClick={() => setPage(id)} title={label}><Icon size={17}/><span>{label}</span></button>)}</nav>
      <div className="runtime-foot"><StatusMark status={status?.runtime.status ?? 'offline'} /><small>{status?.runtime.api_url ?? '127.0.0.1:8787'}</small></div>
    </aside>

    <main>
      <header className="topbar">
        <div><p className="eyebrow">Local control plane</p><h1>{nav.find((n) => n.id === page)?.label}</h1></div>
        <button className="icon-button" onClick={() => void load()} title="Refresh runtime data" disabled={refreshing}><RefreshCw size={18} className={refreshing ? 'spin' : ''}/></button>
      </header>

      {error && <div className="error-band"><CircleAlert size={18}/><div><strong>Runtime data unavailable</strong><span>{error}. Start Gumi and refresh this page.</span></div></div>}

      {page === 'overview' && <Overview status={status} requests={requests} metrics={metrics}/>}
      {page === 'requests' && <Requests requests={requests}/>}
      {page === 'providers' && <Providers providers={status?.providers ?? []}/>}
      {page === 'profiles' && <Profiles profiles={profiles}/>}
      {page === 'config' && <ConfigView config={config}/>}
      {page === 'doctor' && <DoctorView doctor={doctor}/>}
    </main>
  </div>
}

function Overview({ status, requests, metrics }: { status: Status | null; requests: RequestRow[]; metrics: { success: number; avg: number; repairs: number } }) {
  const stages = ['Gateway', 'Context', 'Prompt', 'Provider', 'Validate', 'Repair']
  return <div className="page-stack">
    <section className="metric-strip">
      <div><span>Runtime</span><strong>{status?.runtime.mode ?? 'stabilized'}</strong><StatusMark status={status?.runtime.status ?? 'offline'}/></div>
      <div><span>Success rate</span><strong>{metrics.success}%</strong><small>last {requests.length} requests</small></div>
      <div><span>Average latency</span><strong>{metrics.avg.toLocaleString()} ms</strong><small>gateway to response</small></div>
      <div><span>Repairs</span><strong>{metrics.repairs}</strong><small>local corrections</small></div>
    </section>

    <section className="pipeline-section">
      <div className="section-heading"><div><p className="eyebrow">Adaptive runtime</p><h2>Intelligence pipeline</h2></div><span className="live-label"><i/>live</span></div>
      <div className="pipeline-rail">{stages.map((stage, index) => <div className="pipeline-node" key={stage}><span>{String(index + 1).padStart(2, '0')}</span><strong>{stage}</strong>{index < stages.length - 1 && <ChevronRight size={16}/>}</div>)}</div>
    </section>

    <section className="split-section">
      <div><div className="section-heading"><h2>Provider health</h2></div><div className="provider-list">{status?.providers.map((p) => <div className="provider-row" key={p.name}><Cpu size={18}/><div><strong>{p.name}</strong><small>{p.url}</small></div><StatusMark status={p.status}/></div>)}</div></div>
      <div><div className="section-heading"><h2>Recent activity</h2></div><div className="activity-list">{requests.slice(0,5).map((r) => <div key={r.id}><span className={`activity-icon ${r.status}`}><Activity size={15}/></span><div><strong>{r.model || 'local:auto'}</strong><small>{r.provider || 'provider unresolved'} · {r.latency_ms} ms</small></div><time>{new Date(r.created_at).toLocaleTimeString([], {hour:'2-digit', minute:'2-digit'})}</time></div>)}{requests.length === 0 && <Empty text="Requests will appear here after your first completion."/>}</div></div>
    </section>
  </div>
}

function Requests({ requests }: { requests: RequestRow[] }) {
  return <section><div className="section-heading"><div><p className="eyebrow">Metadata only</p><h2>Recent requests</h2></div><span>{requests.length} records</span></div><div className="table-wrap"><table><thead><tr><th>Request</th><th>Provider / model</th><th>Mode</th><th>Latency</th><th>Stability</th><th>Status</th></tr></thead><tbody>{requests.map((r) => <tr key={r.id}><td><code>{r.id.slice(0,18)}</code><small>{new Date(r.created_at).toLocaleString()}</small></td><td><strong>{r.provider || '—'}</strong><small>{r.model || '—'}</small></td><td>{r.runtime_mode}</td><td>{r.latency_ms} ms</td><td><span className="flags">{r.validation_passed && <ShieldCheck size={15}/>} {r.repair_applied && <Wrench size={15}/>} {r.retry_count > 0 && <TimerReset size={15}/>}</span></td><td><StatusMark status={r.status}/></td></tr>)}</tbody></table>{requests.length === 0 && <Empty text="No request metadata stored yet."/>}</div></section>
}

function Providers({ providers }: { providers: Provider[] }) {
  return <section><div className="section-heading"><div><p className="eyebrow">Local inference</p><h2>Provider adapters</h2></div></div><div className="provider-grid">{providers.map((p) => <article key={p.name}><div className="provider-title"><Server size={20}/><h3>{p.name}</h3><StatusMark status={p.status}/></div><dl><dt>Endpoint</dt><dd><code>{p.url}</code></dd><dt>Boundary</dt><dd>Local network only</dd></dl></article>)}</div></section>
}

function Profiles({ profiles }: { profiles: Profile[] }) {
  return <section><div className="section-heading"><div><p className="eyebrow">Intelligence packs</p><h2>Model profiles</h2></div><span>{profiles.length} loaded</span></div><div className="profile-list">{profiles.map((p) => <article key={p.id}><div><span className="profile-family">{p.family}</span><h3>{p.name || p.id}</h3><code>{p.id}</code></div><dl><dt>Size</dt><dd>{p.size || 'unknown'}</dd><dt>Context</dt><dd>{p.context_limit ? `${p.context_limit.toLocaleString()} tokens` : 'profile default'}</dd><dt>Aliases</dt><dd>{p.aliases?.slice(0,2).join(', ') || 'fallback match'}</dd></dl></article>)}</div></section>
}

function ConfigView({ config }: { config: object | null }) {
  return <section><div className="section-heading"><div><p className="eyebrow">Resolved and redacted</p><h2>Runtime configuration</h2></div></div><pre className="config-code">{JSON.stringify(config ?? {}, null, 2)}</pre></section>
}

function DoctorView({ doctor }: { doctor: Doctor | null }) {
  return <section><div className="doctor-hero"><div className={`doctor-glyph ${doctor?.status ?? 'warning'}`}>{doctor?.status === 'ok' ? <CheckCircle2/> : <HeartPulse/>}</div><div><p className="eyebrow">System diagnostics</p><h2>{doctor?.status === 'ok' ? 'Runtime checks passed' : 'Runtime needs attention'}</h2></div></div><div className="check-list">{doctor?.checks.map((c) => <div key={c.name}><StatusMark status={c.status}/><div><strong>{c.name.replaceAll('_',' ')}</strong><p>{c.message}</p>{c.suggestion && <small>{c.suggestion}</small>}</div></div>)}</div></section>
}

function Empty({ text }: { text: string }) { return <div className="empty"><Gauge size={22}/><span>{text}</span></div> }

export default App
