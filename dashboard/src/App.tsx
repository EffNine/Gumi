import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  Activity, Braces, CheckCircle2, ChevronRight, ChevronDown, CircleAlert, Cpu,
  FileSliders, Gauge, HeartPulse, ListTree, RefreshCw, Server,
  ShieldCheck, TerminalSquare, TimerReset, Wrench, Moon, Sun,
  MessageSquare, Box, ScrollText, Send, Square, Trash2,
  Database, TrendingUp, Bookmark, Plus, X, AlertTriangle,
  ArrowRight, Clock, Thermometer, Zap, Info, Layers,
  Sparkles, Target, Eye, EyeOff, Hash, BarChart3,
  HelpCircle, Keyboard, Check, Pencil, Plug, Wifi,
  WifiOff, Loader, Settings, Bug, Play, ExternalLink, Search,
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
type Page = 'overview' | 'requests' | 'providers' | 'profiles' | 'playground' | 'models' | 'logs' | 'config' | 'doctor' | 'memory' | 'analytics'

type Model = { id: string; object: string; created: number; owned_by: string }
type ModelRegistryEntry = { alias: string; provider: string; model_id: string; context_length: number; enabled: boolean; default: boolean }
type LMStudioModelEntry = { model: string; type: string; path: string; size: string }
type LMStudioLoadResponse = { type: string; instance_id: string; load_time_seconds: number; status: string; load_config?: any }

type LogEntry = { timestamp: string; level: string; message: string; fields?: string }

type MemoryFact = { id: string; key: string; value: string; source: string; confidence: number; session_id: string; created_at: string; updated_at: string; accessed_at: string; access_count: number; ttl_seconds: number }
type MemoryStatus = { enabled: boolean; database_path: string; facts_count?: number; model_fit_entries?: number; injection_budget?: number }
type ModelFitEntry = { model_id: string; difficulty: number; task_type: string; attempts: number; successes: number; avg_latency_ms: number; avg_retries: number; last_updated: string }
type SelfTuningSnapshot = {
  generated_at?: string
  total_attempts?: number
  rule_overrides?: Array<{ rule_name: string; reason?: string; prefer?: string; min_coding?: string; min_context?: number }>
  model_boosts?: Record<string, number>
  model_demotes?: Record<string, number>
  adjustments?: Array<{ kind: string; target: string; from: string; to: string; reason?: string }>
}
type SelfTuningResponse = {
  enabled: boolean
  reason?: string
  snapshot?: SelfTuningSnapshot
  config?: { enabled?: boolean; persist_snapshot?: boolean; warmup_attempts?: number; epsilon?: number }
}

type AnalyticsData = {
  requests_over_time: { date: string; count: number; avg_latency: number; success_rate: number }[]
  provider_usage: { provider: string; count: number; avg_latency: number }[]
  model_usage: { model: string; count: number }[]
}

/* ─── Tooltip ─── */
function Tooltip({ text, shortcut, children, position = 'top' }: { text: string; shortcut?: string; children: React.ReactNode; position?: 'top' | 'bottom' | 'left' | 'right' }) {
  return (
    <span className={`tooltip-wrapper tooltip-${position}`} tabIndex={0}>
      {children}
      <span className="tooltip-content">
        {text}
        {shortcut && <kbd className="tooltip-kbd">{shortcut}</kbd>}
      </span>
    </span>
  )
}

const nav: { id: Page; label: string; icon: typeof Activity; group: string }[] = [
  { id: 'overview', label: 'Overview', icon: Activity, group: 'Monitor' },
  { id: 'playground', label: 'Playground', icon: MessageSquare, group: 'Develop' },
  { id: 'requests', label: 'Requests', icon: ListTree, group: 'Monitor' },
  { id: 'analytics', label: 'Analytics', icon: Gauge, group: 'Monitor' },
  { id: 'providers', label: 'Providers', icon: Server, group: 'Configure' },
  { id: 'models', label: 'Models', icon: Box, group: 'Configure' },
  { id: 'memory', label: 'Memory', icon: Braces, group: 'Configure' },
  { id: 'profiles', label: 'Profiles', icon: FileSliders, group: 'Configure' },
  { id: 'logs', label: 'Logs', icon: ScrollText, group: 'Develop' },
  { id: 'config', label: 'Config', icon: Wrench, group: 'Configure' },
  { id: 'doctor', label: 'Doctor', icon: HeartPulse, group: 'Maintain' },
]

const navGroups = ['Monitor', 'Develop', 'Configure', 'Maintain'] as const

async function getJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`/api${path}`, init)
  if (!response.ok) throw new Error(`${response.status} ${response.statusText}`)
  return response.json() as Promise<T>
}

/* ─── Model Registry API helpers ─── */
async function getModelRegistry(): Promise<{ enabled: boolean; models: ModelRegistryEntry[] }> {
  return getJSON('/v1/gumi/models')
}

async function createRegistryModel(entry: Omit<ModelRegistryEntry, 'default'> & { default?: boolean }): Promise<{ enabled: boolean; models: ModelRegistryEntry[] }> {
  return getJSON('/v1/gumi/models', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(entry),
  })
}

async function updateRegistryModel(alias: string, entry: ModelRegistryEntry): Promise<{ enabled: boolean; models: ModelRegistryEntry[] }> {
  return getJSON(`/v1/gumi/models/${encodeURIComponent(alias)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(entry),
  })
}

async function deleteRegistryModel(alias: string): Promise<{ enabled: boolean; models: ModelRegistryEntry[] }> {
  return getJSON(`/v1/gumi/models/${encodeURIComponent(alias)}`, {
    method: 'DELETE',
  })
}

async function setDefaultRegistryModel(alias: string): Promise<{ enabled: boolean; models: ModelRegistryEntry[] }> {
  return getJSON(`/v1/gumi/models/${encodeURIComponent(alias)}/default`, {
    method: 'POST',
  })
}

/* ─── Status descriptions ─── */
const statusDescriptions: Record<string, string> = {
  ok: 'All systems operating normally',
  running: 'Service is active and processing',
  success: 'Request completed successfully',
  repaired: 'Request completed after auto-correction',
  retried: 'Request succeeded after one or more retries',
  failed: 'Request could not be completed',
  warning: 'System is operating in a degraded state',
  degraded: 'Performance or reliability is reduced',
  offline: 'Service is not reachable',
  error: 'An error occurred during processing',
}

const stageDescriptions: Record<string, string> = {
  gateway: 'Routes incoming requests to the appropriate handler. First point of contact for all API calls.',
  context: 'Assembles session context, memory facts, and conversation history for the request.',
  prompt: 'Optimizes the prompt with system instructions, templates, and context injection strategies.',
  provider: 'Sends the prepared request to the LLM provider for inference. The core generation step.',
  validate: 'Checks the model output against expected schemas, quality thresholds, and safety rules.',
  repair: 'Automatically corrects validation failures by re-prompting or transforming the output.',
}

const confidenceDescriptions: Record<string, string> = {
  high: 'Confidence ≥ 0.8 — Fact is highly reliable and frequently reinforced',
  medium: 'Confidence 0.4–0.8 — Fact has moderate reliability, may need reinforcement',
  low: 'Confidence < 0.4 — Fact is speculative or based on limited observations',
}

const modeDescriptions: Record<string, string> = {
  direct: 'Direct mode: Requests pass through without adaptive processing. Fastest but no intelligence layer.',
  stabilized: 'Stabilized mode: Default runtime with validation and repair. Balances speed and reliability.',
  'gumi-structured': 'Structured mode: Full intelligence pipeline with context assembly, prompt optimization, and repair. Highest quality output.',
}

function StatusMark({ status }: { status: string }) {
  const good = ['ok', 'running', 'success', 'repaired'].includes(status)
  const cls = good ? 'good' : status === 'warning' || status === 'degraded' ? 'warn' : 'bad'
  return (
    <Tooltip text={statusDescriptions[status] || `Status: ${status}`}>
      <span className={`status-mark ${cls}`}><i />{status}</span>
    </Tooltip>
  )
}

function Empty({ text, icon: Icon = Gauge }: { text: string; icon?: typeof Gauge }) {
  return (
    <div className="empty">
      <Icon size={22} strokeWidth={1.5} />
      <span>{text}</span>
    </div>
  )
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
  const [darkMode, setDarkMode] = useState(() => localStorage.getItem('gumi-theme') === 'dark')
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null)

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', darkMode ? 'dark' : 'light')
    localStorage.setItem('gumi-theme', darkMode ? 'dark' : 'light')
  }, [darkMode])

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
      setLastUpdated(new Date())
    } catch (e) { setError(e instanceof Error ? e.message : 'Runtime unavailable') }
    finally { setRefreshing(false) }
  }

  useEffect(() => { void load(); const id = window.setInterval(load, 15000); return () => clearInterval(id) }, [])

  const runtimeOnline = status?.runtime.status === 'running'

  const metrics = useMemo(() => {
    const success = requests.filter((r) => ['success', 'repaired', 'retried'].includes(r.status)).length
    const avg = requests.length ? Math.round(requests.reduce((sum, r) => sum + r.latency_ms, 0) / requests.length) : 0
    return { success: requests.length ? Math.round((success / requests.length) * 100) : 100, avg, repairs: requests.filter((r) => r.repair_applied).length }
  }, [requests])

  return <div className="app-shell">
    <aside className="sidebar">
      <div className="brand"><span className="brand-mark">G</span><div><strong>Gumi</strong><small>Runtime console</small></div></div>
      <nav>
        {navGroups.map(group => (
          <div key={group} className="nav-group">
            <span className="nav-group-label">{group}</span>
            {nav.filter(n => n.group === group).map(({ id, label, icon: Icon }) => (
              <button key={id} className={page === id ? 'active' : ''} onClick={() => setPage(id)} title={label}>
                <Icon size={17} /><span>{label}</span>
              </button>
            ))}
          </div>
        ))}
      </nav>
      <div className="sidebar-footer">
        <button className="theme-toggle" onClick={() => setDarkMode(d => !d)}>
          {darkMode ? <Sun size={15} /> : <Moon size={15} />}{darkMode ? 'Light' : 'Dark'} mode
        </button>
        <div className={`runtime-status ${runtimeOnline ? 'online' : 'offline'}`}>
          <i />{runtimeOnline ? 'Connected' : 'Offline'}
          <small style={{ fontWeight: 400, textTransform: 'none', marginLeft: 4 }}>{status?.runtime.api_url ?? ''}</small>
        </div>
      </div>
    </aside>

    <main>
      <header className="topbar">
        <div>
          <p className="eyebrow">Local control plane</p>
          <h1>{nav.find((n) => n.id === page)?.label}</h1>
        </div>
        <div className="topbar-actions">
          {lastUpdated && (
            <Tooltip text={`Last refreshed at ${lastUpdated.toLocaleTimeString()}`}>
              <span className="last-updated">
                <Clock size={13} />
                {lastUpdated.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
              </span>
            </Tooltip>
          )}
          <Tooltip text="Refresh all runtime data" shortcut="R">
            <button className="icon-button" onClick={() => void load()} disabled={refreshing}>
              <RefreshCw size={18} className={refreshing ? 'spin' : ''} />
            </button>
          </Tooltip>
        </div>
      </header>

      {error && <div className="error-band"><CircleAlert size={18} /><div><strong>Runtime data unavailable</strong><span>{error}. Start Gumi and refresh this page.</span></div></div>}

      {page === 'overview' && <Overview status={status} requests={requests} metrics={metrics} runtimeOnline={runtimeOnline} />}
      {page === 'playground' && <Playground status={status} />}
      {page === 'requests' && <Requests requests={requests} />}
      {page === 'providers' && <Providers />}
      {page === 'models' && <ModelManagement />}
      {page === 'profiles' && <ProfilesView profiles={profiles} />}
      {page === 'analytics' && <Analytics requests={requests} />}
      {page === 'memory' && <MemoryView />}
      {page === 'logs' && <LiveLogs />}
      {page === 'config' && <ConfigView config={config} />}
      {page === 'doctor' && <DoctorView doctor={doctor} />}
    </main>
  </div>
}

/* ═══════════════════════════════════════════════
   OVERVIEW
   ═══════════════════════════════════════════════ */
function Overview({ status, requests, metrics, runtimeOnline }: { status: Status | null; requests: RequestRow[]; metrics: { success: number; avg: number; repairs: number }; runtimeOnline: boolean }) {
  const stages = [
    { id: 'gateway', label: 'Gateway', desc: 'Request routing and authentication' },
    { id: 'context', label: 'Context', desc: 'Session assembly and memory injection' },
    { id: 'prompt', label: 'Prompt', desc: 'System instruction and prompt optimization' },
    { id: 'provider', label: 'Provider', desc: 'LLM inference via configured provider' },
    { id: 'validate', label: 'Validate', desc: 'Output schema and quality validation' },
    { id: 'repair', label: 'Repair', desc: 'Automatic correction of validation failures' },
  ]

  const mode = status?.runtime.mode ?? 'stabilized'
  const activeStage = mode === 'direct' ? 3 : mode === 'gumi-structured' ? 5 : 4

  return <div className="page-stack">
    <section className="metric-strip">
      <div>
        <div className="metric-icon"><Activity size={18} /></div>
        <span>Runtime</span>
        <div className="metric-value">
          <strong>{mode}</strong>
          <Tooltip text={modeDescriptions[mode] || `Runtime mode: ${mode}`}>
            <span className={`mode-badge ${mode}`}>{mode}</span>
          </Tooltip>
        </div>
        <StatusMark status={runtimeOnline ? 'ok' : 'offline'} />
      </div>
      <div>
        <div className="metric-icon"><TrendingUp size={18} /></div>
        <span>Success rate</span>
        <div className="metric-value">
          <strong>{metrics.success}%</strong>
          <span className={`metric-trend ${metrics.success >= 80 ? 'up' : 'down'}`}>{metrics.success >= 80 ? '✓' : '⚠'}</span>
        </div>
        <small>last {requests.length} requests</small>
      </div>
      <div>
        <div className="metric-icon"><TimerReset size={18} /></div>
        <span>Average latency</span>
        <strong>{metrics.avg.toLocaleString()} ms</strong>
        <small>gateway to response</small>
      </div>
      <div>
        <div className="metric-icon"><Wrench size={18} /></div>
        <span>Repairs</span>
        <strong>{metrics.repairs}</strong>
        <small>local corrections</small>
      </div>
    </section>

    <section className="pipeline-section">
      <div className="section-heading">
        <div><p className="eyebrow">Adaptive runtime</p><h2>Intelligence pipeline</h2></div>
        <div style={{ display: 'flex', gap: 12, alignItems: 'center' }}>
          <span className="live-label"><i />live</span>
          <Tooltip text={modeDescriptions[mode] || ''}>
            <span style={{ fontSize: 11, color: 'var(--muted)', cursor: 'help' }}>
              Mode: <strong style={{ textTransform: 'capitalize' }}>{mode}</strong>
            </span>
          </Tooltip>
        </div>
      </div>
      <div className="pipeline-rail">
        {stages.map((stage, idx) => {
          const isActive = idx === activeStage
          const isCompleted = idx < activeStage
          const isError = false
          return (
            <div
              key={stage.id}
              className={`pipeline-node ${isActive ? 'active' : ''} ${isCompleted ? 'done' : ''} ${isError ? 'error' : ''}`}
            >
              <div className="pipeline-node-header">
                <span className={`pipeline-dot ${isActive ? 'pulse' : ''} ${isCompleted ? 'completed' : ''} ${isError ? 'error' : ''}`} />
                <span className="node-index">{String(idx + 1).padStart(2, '0')}</span>
              </div>
              <Tooltip text={stageDescriptions[stage.id] || stage.desc}>
                <span className="node-label">{stage.label}</span>
              </Tooltip>
              <span className="node-desc">{stage.desc}</span>
              {isActive && <span className="node-active-indicator">ACTIVE</span>}
              {isCompleted && <span className="node-completed-indicator">✓ DONE</span>}
              {idx < stages.length - 1 && (
                <span className="pipeline-arrow"><ArrowRight size={14} /></span>
              )}
            </div>
          )
        })}
      </div>
      <div className="pipeline-progress-track">
        <div className="pipeline-progress-fill" style={{ width: `${((activeStage + 1) / stages.length) * 100}%` }} />
      </div>
    </section>

    <section className="split-section">
      <div>
        <div className="section-heading"><h2>Provider health</h2></div>
        <div className="provider-list">
          {status?.providers.map((p) => (
            <div className="provider-row" key={p.name}>
              <Cpu size={18} />
              <div><strong>{p.name}</strong><small>{p.url}</small></div>
              <StatusMark status={p.status} />
            </div>
          ))}
          {(!status?.providers || status.providers.length === 0) && <Empty text="No providers configured." icon={Server} />}
        </div>
      </div>
      <div>
        <div className="section-heading"><h2>Recent activity</h2></div>
        <div className="activity-list">
          {requests.slice(0, 5).map((r) => (
            <div key={r.id}>
              <Tooltip text={statusDescriptions[r.status] || `Status: ${r.status}`}>
                <span className={`activity-icon ${r.status}`}><Activity size={15} /></span>
              </Tooltip>
              <div><strong>{r.model || 'local:auto'}</strong><small>{r.provider || 'provider unresolved'} · {r.latency_ms} ms</small></div>
              <time>{new Date(r.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}</time>
            </div>
          ))}
          {requests.length === 0 && <Empty text="Requests will appear here after your first completion." icon={ListTree} />}
        </div>
      </div>
    </section>
  </div>
}

/* ═══════════════════════════════════════════════
   PLAYGROUND
   ═══════════════════════════════════════════════ */

/** Parse the provider prefix from a Gumi model ID (e.g. "lmstudio:qwen3-8b" → "lmstudio"). */
function providerFromModel(model: string): string | null {
  const idx = model.indexOf(':')
  return idx > 0 ? model.slice(0, idx) : null
}

/** Replace the provider prefix in a model ID, keeping the model name. */
function setModelProvider(model: string, newProvider: string): string {
  const idx = model.indexOf(':')
  if (idx > 0) return newProvider + model.slice(idx)
  // If the model is just a name with no prefix, prepend the provider.
  if (model && model !== 'auto') return `${newProvider}:${model}`
  return `${newProvider}:auto`
}

function Playground({ status }: { status: Status | null }) {
  const [provider, setProvider] = useState('lmstudio')
  const [model, setModel] = useState('local:auto')
  const [prompt, setPrompt] = useState('')
  const [response, setResponse] = useState('')
  const [streaming, setStreaming] = useState(false)
  const [abortController, setAbortController] = useState<AbortController | null>(null)
  const [latency, setLatency] = useState(0)
  const [tokens, setTokens] = useState(0)
  const [activeModel, setActiveModel] = useState('')
  const [error, setError] = useState('')
  const responseRef = useRef<HTMLDivElement>(null)

  /* ─── Model auto-fetch ─── */
  const [availableModels, setAvailableModels] = useState<Model[]>([])
  const [modelsLoading, setModelsLoading] = useState(false)
  const [modelsError, setModelsError] = useState('')
  const [manualMode, setManualMode] = useState(false)
  const [dropdownOpen, setDropdownOpen] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')
  const dropdownRef = useRef<HTMLDivElement>(null)
  const searchInputRef = useRef<HTMLInputElement>(null)

  const fetchModels = useCallback(async () => {
    setModelsLoading(true)
    setModelsError('')
    try {
      const result = await getJSON<{ object: string; data: Model[] }>('/v1/models')
      setAvailableModels(result.data ?? [])
    } catch (e: any) {
      setModelsError(e instanceof Error ? e.message : 'Failed to fetch models')
      setAvailableModels([])
    } finally {
      setModelsLoading(false)
    }
  }, [])

  // Fetch models on mount
  useEffect(() => { void fetchModels() }, [fetchModels])

  // Re-fetch when provider changes (to pick up provider-specific models)
  useEffect(() => { void fetchModels() }, [provider, fetchModels])

  // Close dropdown on outside click
  useEffect(() => {
    const handleClick = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setDropdownOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [])

  const providerList = useMemo(() => {
    return (status?.providers?.filter(p => p.status === 'ok' || p.status === 'degraded').map(p => p.name)
      ?? ['lmstudio', 'ollama', 'openai_compatible_local'])
  }, [status])

  /* ─── Group models by provider ─── */
  const groupedModels = useMemo(() => {
    const groups: Record<string, Model[]> = {}
    for (const m of availableModels) {
      const p = providerFromModel(m.id) ?? 'other'
      if (!groups[p]) groups[p] = []
      groups[p].push(m)
    }
    // Sort models within each group
    for (const p of Object.keys(groups)) {
      groups[p].sort((a, b) => a.id.localeCompare(b.id))
    }
    return groups
  }, [availableModels])

  /* ─── Filtered models for the selected provider ─── */
  const filteredModels = useMemo(() => {
    const all = groupedModels[provider] ?? []
    if (!searchQuery) return all
    const q = searchQuery.toLowerCase()
    return all.filter(m => m.id.toLowerCase().includes(q))
  }, [groupedModels, provider, searchQuery])

  /* ─── Sync provider state with model field ─── */
  const handleProviderChange = (newProvider: string) => {
    setProvider(newProvider)
    setModel(prev => setModelProvider(prev, newProvider))
    setSearchQuery('')
    setDropdownOpen(false)
  }

  const handleModelSelect = (modelId: string) => {
    setModel(modelId)
    setSearchQuery('')
    setDropdownOpen(false)
    // Sync provider dropdown from model prefix
    const p = providerFromModel(modelId)
    if (p && providerList.includes(p)) {
      setProvider(p)
    }
  }

  const handleManualModelChange = (newModel: string) => {
    setModel(newModel)
    const p = providerFromModel(newModel)
    if (p && providerList.includes(p)) {
      setProvider(p)
    }
  }

  const resolvedModel = useMemo(() => {
    // If model already has a provider prefix (e.g. "lmstudio:qwen3-8b"), use it.
    if (model.includes(':')) return model
    // Otherwise prepend the selected provider.
    return `${provider}:${model || 'auto'}`
  }, [model, provider])

  const handleSend = async () => {
    if (!prompt.trim() || streaming) return
    setStreaming(true)
    setResponse('')
    setLatency(0)
    setTokens(0)
    setError('')
    setActiveModel(resolvedModel)

    const controller = new AbortController()
    setAbortController(controller)
    const startTime = Date.now()

    try {
      const resp = await fetch('/api/v1/chat/completions', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          model: resolvedModel,
          messages: [{ role: 'user', content: prompt }],
          stream: true,
        }),
        signal: controller.signal,
      })

      if (!resp.ok) {
        const errData = await resp.text()
        setError(`HTTP ${resp.status}: ${errData}`)
        setStreaming(false)
        return
      }

      const reader = resp.body?.getReader()
      if (!reader) { setStreaming(false); return }

      const decoder = new TextDecoder()
      let buffer = ''
      let fullResponse = ''
      let tokenCount = 0

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop() ?? ''

        for (const line of lines) {
          const trimmed = line.trim()
          if (!trimmed || !trimmed.startsWith('data: ')) continue
          const data = trimmed.slice(6)
          if (data === '[DONE]') continue
          try {
            const chunk = JSON.parse(data)
            const content = chunk?.choices?.[0]?.delta?.content
            if (content) {
              fullResponse += content
              tokenCount++
              setResponse(fullResponse)
              setTokens(tokenCount)
            }
          } catch { /* skip malformed chunks */ }
        }
      }

      setLatency(Date.now() - startTime)
    } catch (e: any) {
      if (e.name !== 'AbortError') {
        setError(e instanceof Error ? e.message : 'Request failed')
      }
    } finally {
      setStreaming(false)
      setAbortController(null)
    }
  }

  const handleStop = () => {
    abortController?.abort()
    setStreaming(false)
  }

  useEffect(() => {
    if (responseRef.current) {
      responseRef.current.scrollTop = responseRef.current.scrollHeight
    }
  }, [response])

  const selectedModelDisplay = model || (filteredModels.length > 0 ? filteredModels[0].id : `${provider}:auto`)

  return <div className="playground-layout">
    <div className="playground-controls">
      <div className="playground-model-config">
        <Tooltip text="Select the provider backend for inference">
          <select value={provider} onChange={e => handleProviderChange(e.target.value)}>
            {providerList.map(p => <option key={p} value={p}>{p}</option>)}
          </select>
        </Tooltip>

        {manualMode ? (
          /* ─── Manual text input ─── */
          <Tooltip text="Type a model identifier manually (e.g. auto, qwen3-8b, or provider:model)">
            <input
              type="text"
              value={model}
              onChange={e => handleManualModelChange(e.target.value)}
              placeholder="Model name (e.g. auto, qwen3-8b)"
              className="playground-model-input"
            />
          </Tooltip>
        ) : (
          /* ─── Searchable model dropdown ─── */
          <div className="model-dropdown-wrapper" ref={dropdownRef}>
            <Tooltip text="Select a model from the available list. Type to search.">
              <button
                className="model-dropdown-trigger"
                onClick={() => { setDropdownOpen(d => !d); if (!dropdownOpen) setTimeout(() => searchInputRef.current?.focus(), 0) }}
                disabled={modelsLoading}
              >
                {modelsLoading ? (
                  <><Loader size={14} className="spin" /> Loading models...</>
                ) : modelsError ? (
                  <><CircleAlert size={14} style={{ color: 'var(--red)' }} /> {modelsError}</>
                ) : (
                  <><Box size={14} /> {model || (filteredModels.length > 0 ? filteredModels[0].id : `${provider}:auto`)}</>
                )}
                <ChevronDown size={14} className={`model-dropdown-chevron ${dropdownOpen ? 'open' : ''}`} />
              </button>
            </Tooltip>

            {dropdownOpen && !modelsLoading && !modelsError && (
              <div className="model-dropdown-menu">
                <div className="model-dropdown-search">
                  <Search size={14} />
                  <input
                    ref={searchInputRef}
                    type="text"
                    value={searchQuery}
                    onChange={e => setSearchQuery(e.target.value)}
                    placeholder="Search models..."
                    onKeyDown={e => {
                      if (e.key === 'Enter' && filteredModels.length > 0) {
                        handleModelSelect(filteredModels[0].id)
                      }
                      if (e.key === 'Escape') setDropdownOpen(false)
                    }}
                  />
                </div>
                <div className="model-dropdown-list">
                  {filteredModels.length === 0 && (
                    <div className="model-dropdown-empty">
                      {searchQuery ? 'No models match your search.' : `No models found for ${provider}.`}
                    </div>
                  )}
                  {filteredModels.map(m => (
                    <button
                      key={m.id}
                      className={`model-dropdown-item ${m.id === model ? 'selected' : ''}`}
                      onClick={() => handleModelSelect(m.id)}
                    >
                      <span className="model-dropdown-item-name">{m.id}</span>
                      <span className="model-dropdown-item-owner">{m.owned_by}</span>
                    </button>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}

        <Tooltip text={manualMode ? 'Switch to model dropdown' : 'Switch to manual model input'}>
          <button
            className="icon-button model-toggle-btn"
            onClick={() => setManualMode(m => !m)}
            title={manualMode ? 'Use dropdown' : 'Manual input'}
          >
            {manualMode ? <ListTree size={14} /> : <Pencil size={14} />}
          </button>
        </Tooltip>

        <Tooltip text="Resolved model ID sent to the provider">
          <span style={{ fontSize: 11, color: 'var(--muted)', fontFamily: 'monospace', cursor: 'help' }}>
            ↦ {resolvedModel}
          </span>
        </Tooltip>
      </div>
      {streaming ? (
        <button className="stop-btn" onClick={handleStop}><Square size={14} /> Stop</button>
      ) : (
        <Tooltip text="Send the prompt to the model" shortcut="⌘⏎">
          <button className="send-btn" onClick={handleSend} disabled={!prompt.trim()}>
            <Send size={14} /> Send
          </button>
        </Tooltip>
      )}
    </div>
    <div className="playground-main">
      <div className="playground-input-area">
        <textarea
          value={prompt}
          onChange={e => setPrompt(e.target.value)}
          placeholder="Enter a prompt to test the model..."
          disabled={streaming}
          onKeyDown={e => { if (e.metaKey && e.key === 'Enter') handleSend() }}
        />
        <div className="playground-metrics">
          <span>Model: {activeModel || '—'}</span>
          <span>Prompt: {prompt.length} chars</span>
          {latency > 0 && <span>Latency: {latency} ms</span>}
          {tokens > 0 && <span>Tokens: ~{tokens}</span>}
        </div>
      </div>
      <div className="playground-output">
        <div
          ref={responseRef}
          className={`playground-response ${!response && !streaming && !error ? 'empty' : ''}`}
        >
          {error ? <span style={{ color: 'var(--red)' }}>Error: {error}</span> :
           response ? <>{response}</> :
           streaming ? <><span className="streaming-cursor" /></> :
           <span>Response will appear here...</span>}
          {streaming && !error && <span className="streaming-cursor" />}
        </div>
      </div>
    </div>
  </div>
}

/* ═══════════════════════════════════════════════
   MODEL MANAGEMENT — Registry + LM Studio
   ═══════════════════════════════════════════════ */
function ModelManagement() {
  // ── Registry state ──
  const [registryModels, setRegistryModels] = useState<ModelRegistryEntry[]>([])
  const [registryLoading, setRegistryLoading] = useState(false)
  const [registryError, setRegistryError] = useState('')
  const [editingAlias, setEditingAlias] = useState<string | null>(null)
  const [form, setForm] = useState<ModelRegistryEntry>({
    alias: '', provider: '', model_id: '', context_length: 4096, enabled: true, default: false,
  })
  const [formError, setFormError] = useState('')

  // ── LM Studio state ──
  const [lmStudioModels, setLmStudioModels] = useState<LMStudioModelEntry[]>([])
  const [loadedID, setLoadedID] = useState('')
  const [loading, setLoading] = useState(false)
  const [loadStatus, setLoadStatus] = useState<{ type: string; msg: string } | null>(null)
  const [loadModelId, setLoadModelId] = useState('')
  const [contextLen, setContextLen] = useState(4096)

  const fetchRegistry = useCallback(async () => {
    setRegistryLoading(true)
    setRegistryError('')
    try {
      const resp = await getModelRegistry()
      setRegistryModels(resp.models ?? [])
    } catch (e: any) {
      setRegistryError(e instanceof Error ? e.message : 'Failed to fetch registry')
    } finally {
      setRegistryLoading(false)
    }
  }, [])

  const fetchLmStudio = useCallback(async () => {
    try {
      const [list, loaded] = await Promise.all([
        getJSON<{ data: LMStudioModelEntry[] }>('/v1/gumi/providers/lmstudio/models'),
        getJSON<{ loaded_instance_id: string }>('/v1/gumi/providers/lmstudio/loaded'),
      ])
      setLmStudioModels(list.data ?? [])
      setLoadedID(loaded.loaded_instance_id ?? '')
    } catch { /* LM Studio may not be available */ }
  }, [])

  useEffect(() => { void fetchRegistry(); void fetchLmStudio() }, [fetchRegistry, fetchLmStudio])

  // ── Registry CRUD ──
  const resetForm = () => {
    setForm({ alias: '', provider: '', model_id: '', context_length: 4096, enabled: true, default: false })
    setEditingAlias(null)
    setFormError('')
  }

  const startEdit = (entry: ModelRegistryEntry) => {
    setForm({ ...entry })
    setEditingAlias(entry.alias)
    setFormError('')
  }

  const handleSubmit = async () => {
    setFormError('')
    if (!form.alias.trim()) { setFormError('Alias is required'); return }
    if (!form.provider.trim()) { setFormError('Provider is required'); return }
    if (!form.model_id.trim()) { setFormError('Model ID is required'); return }

    try {
      if (editingAlias) {
        await updateRegistryModel(editingAlias, form)
      } else {
        await createRegistryModel(form)
      }
      resetForm()
      await fetchRegistry()
    } catch (e: any) {
      setFormError(e instanceof Error ? e.message : 'Operation failed')
    }
  }

  const handleDelete = async (alias: string) => {
    try {
      await deleteRegistryModel(alias)
      await fetchRegistry()
    } catch (e: any) {
      setRegistryError(e instanceof Error ? e.message : 'Delete failed')
    }
  }

  const handleSetDefault = async (alias: string) => {
    try {
      await setDefaultRegistryModel(alias)
      await fetchRegistry()
    } catch (e: any) {
      setRegistryError(e instanceof Error ? e.message : 'Set default failed')
    }
  }

  const handleToggleEnabled = async (entry: ModelRegistryEntry) => {
    const updated = { ...entry, enabled: !entry.enabled }
    try {
      await updateRegistryModel(entry.alias, updated)
      await fetchRegistry()
    } catch (e: any) {
      setRegistryError(e instanceof Error ? e.message : 'Toggle failed')
    }
  }

  // ── LM Studio handlers ──
  const handleLoad = async (modelId: string) => {
    setLoading(true)
    setLoadStatus({ type: 'loading', msg: `Loading ${modelId}...` })
    setLoadModelId(modelId)
    try {
      const result = await getJSON<LMStudioLoadResponse>(`/v1/gumi/providers/lmstudio/models/load`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          model_id: modelId,
          config: { context_length: contextLen },
        }),
      })
      setLoadStatus({ type: 'success', msg: `Loaded in ${result.load_time_seconds?.toFixed(1) ?? '?'}s (instance: ${result.instance_id?.slice(0, 12)}...)` })
      setLoadedID(result.instance_id ?? '')
    } catch (e: any) {
      setLoadStatus({ type: 'error', msg: e instanceof Error ? e.message : 'Failed to load model' })
    } finally { setLoading(false); setLoadModelId('') }
  }

  const handleUnload = async () => {
    if (!loadedID) return
    setLoading(true)
    setLoadStatus({ type: 'loading', msg: 'Unloading...' })
    try {
      await getJSON('/v1/gumi/providers/lmstudio/models/unload', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ instance_id: loadedID }),
      })
      setLoadedID('')
      setLoadStatus({ type: 'success', msg: 'Model unloaded' })
    } catch (e: any) {
      setLoadStatus({ type: 'error', msg: e instanceof Error ? e.message : 'Failed to unload' })
    } finally { setLoading(false) }
  }

  return <div className="page-stack">
    {/* ── Toolbar ── */}
    <section style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
      <Tooltip text="Refresh registry and LM Studio models">
        <button className="icon-button" onClick={() => { void fetchRegistry(); void fetchLmStudio() }}><RefreshCw size={16} /></button>
      </Tooltip>
      {loadedID && (
        <span className="loaded-model-badge"><i />Loaded: {loadedID.slice(0, 24)}...</span>
      )}
      {loadedID && (
        <button className="unload-btn" onClick={handleUnload} disabled={loading}>Unload</button>
      )}
      {registryError && <span style={{ color: 'var(--red)', fontSize: 12 }}>{registryError}</span>}
    </section>

    {/* ── Two-column layout ── */}
    <div className="model-mgmt-grid" style={{ marginTop: 16 }}>
      {/* ── Left: Registered Models ── */}
      <div className="model-mgmt-card">
        <h3><Box size={18} /> Registered Models</h3>
        <div className="model-list">
          {registryLoading && registryModels.length === 0 && (
            <div style={{ padding: 12, color: 'var(--muted)', fontSize: 12 }}>Loading...</div>
          )}
          {!registryLoading && registryModels.length === 0 && (
            <Empty text="No models registered. Use the form on the right to add one." />
          )}
          {registryModels.map((entry) => (
            <div className="model-item" key={entry.alias}>
              <div style={{ flex: 1, minWidth: 0 }}>
                <div className="model-name" style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                  {entry.alias}
                  {entry.default && <span className="provider-default-badge" style={{ fontSize: 9 }}>Default</span>}
                </div>
                <div className="model-size">
                  {entry.provider} / {entry.model_id}
                  {entry.context_length > 0 && ` · ${entry.context_length.toLocaleString()} ctx`}
                </div>
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                <Tooltip text={entry.enabled ? 'Disable model' : 'Enable model'}>
                  <button
                    className="icon-button"
                    style={{ width: 28, height: 28 }}
                    onClick={() => handleToggleEnabled(entry)}
                  >
                    {entry.enabled ? <Check size={13} style={{ color: 'var(--green)' }} /> : <X size={13} style={{ color: 'var(--muted)' }} />}
                  </button>
                </Tooltip>
                {!entry.default && (
                  <Tooltip text="Set as default">
                    <button className="icon-button" style={{ width: 28, height: 28 }} onClick={() => handleSetDefault(entry.alias)}>
                      <Target size={13} />
                    </button>
                  </Tooltip>
                )}
                <Tooltip text="Edit model">
                  <button className="icon-button" style={{ width: 28, height: 28 }} onClick={() => startEdit(entry)}>
                    <Pencil size={13} />
                  </button>
                </Tooltip>
                <Tooltip text="Delete model">
                  <button className="icon-button" style={{ width: 28, height: 28 }} onClick={() => handleDelete(entry.alias)}>
                    <Trash2 size={13} style={{ color: 'var(--red)' }} />
                  </button>
                </Tooltip>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* ── Right: Add / Edit Model Form ── */}
      <div className="model-mgmt-card">
        <h3><Cpu size={18} /> {editingAlias ? `Edit: ${editingAlias}` : 'Add Model'}</h3>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          <label style={{ fontSize: 12, display: 'flex', flexDirection: 'column', gap: 3 }}>
            Alias
            <input
              type="text"
              value={form.alias}
              onChange={e => setForm(f => ({ ...f, alias: e.target.value }))}
              placeholder="e.g. my-model"
              style={{ padding: '7px 10px', border: '1px solid var(--line)', borderRadius: 6, background: 'var(--surface)', fontSize: 12 }}
              disabled={!!editingAlias}
            />
          </label>
          <label style={{ fontSize: 12, display: 'flex', flexDirection: 'column', gap: 3 }}>
            Provider
            <input
              type="text"
              value={form.provider}
              onChange={e => setForm(f => ({ ...f, provider: e.target.value }))}
              placeholder="e.g. ollama, lmstudio"
              style={{ padding: '7px 10px', border: '1px solid var(--line)', borderRadius: 6, background: 'var(--surface)', fontSize: 12 }}
            />
          </label>
          <label style={{ fontSize: 12, display: 'flex', flexDirection: 'column', gap: 3 }}>
            Model ID
            <input
              type="text"
              value={form.model_id}
              onChange={e => setForm(f => ({ ...f, model_id: e.target.value }))}
              placeholder="e.g. llama3, qwen3-8b"
              style={{ padding: '7px 10px', border: '1px solid var(--line)', borderRadius: 6, background: 'var(--surface)', fontSize: 12 }}
            />
          </label>
          <label style={{ fontSize: 12, display: 'flex', flexDirection: 'column', gap: 3 }}>
            Context Length
            <input
              type="number"
              value={form.context_length}
              onChange={e => setForm(f => ({ ...f, context_length: Number(e.target.value) }))}
              min={1024} max={131072} step={1024}
              style={{ padding: '7px 10px', border: '1px solid var(--line)', borderRadius: 6, background: 'var(--surface)', fontSize: 12 }}
            />
          </label>
          <div style={{ display: 'flex', gap: 16 }}>
            <label style={{ fontSize: 12, display: 'flex', alignItems: 'center', gap: 6, cursor: 'pointer' }}>
              <input
                type="checkbox"
                checked={form.enabled}
                onChange={e => setForm(f => ({ ...f, enabled: e.target.checked }))}
              />
              Enabled
            </label>
            <label style={{ fontSize: 12, display: 'flex', alignItems: 'center', gap: 6, cursor: 'pointer' }}>
              <input
                type="checkbox"
                checked={form.default}
                onChange={e => setForm(f => ({ ...f, default: e.target.checked }))}
              />
              Default
            </label>
          </div>
          {formError && <div style={{ color: 'var(--red)', fontSize: 12 }}>{formError}</div>}
          <div style={{ display: 'flex', gap: 8 }}>
            <button className="send-btn" onClick={handleSubmit} style={{ fontSize: 12, padding: '7px 16px' }}>
              {editingAlias ? 'Update' : 'Create'}
            </button>
            {editingAlias && (
              <button className="unload-btn" onClick={resetForm} style={{ fontSize: 12, padding: '7px 16px' }}>
                Cancel
              </button>
            )}
          </div>
        </div>
      </div>
    </div>

    {/* ── LM Studio Native Models section ── */}
    <section style={{ marginTop: 24 }}>
      <div className="section-heading">
        <div><p className="eyebrow">Local inference</p><h2>LM Studio Native Models</h2></div>
        <Tooltip text="Refresh available models">
          <button className="icon-button" onClick={fetchLmStudio}><RefreshCw size={16} /></button>
        </Tooltip>
      </div>
      <div className="model-mgmt-grid">
        <div className="model-mgmt-card">
          <h3><Box size={18} /> Available Models</h3>
          <div className="model-list">
            {lmStudioModels.length === 0 && <Empty text="No models found. Is LM Studio running?" />}
            {lmStudioModels.map((m) => (
              <div className="model-item" key={m.model}>
                <div>
                  <div className="model-name">{m.model}</div>
                  <div className="model-size">{m.size || 'unknown size'}{m.type ? ` · ${m.type}` : ''}</div>
                </div>
                <button
                  className="load-btn"
                  onClick={() => handleLoad(m.model)}
                  disabled={loading}
                >{loading && loadModelId === m.model ? 'Loading...' : 'Load'}</button>
              </div>
            ))}
          </div>
        </div>
        <div className="model-mgmt-card">
          <h3><Cpu size={18} /> Load Configuration</h3>
          <div className="model-params">
            <label style={{ fontSize: 12, display: 'flex', flexDirection: 'column', gap: 3 }}>
              Context length
              <input type="number" value={contextLen} onChange={e => setContextLen(Number(e.target.value))} min={1024} max={131072} step={1024}
                style={{ padding: '7px 10px', border: '1px solid var(--line)', borderRadius: 6, background: 'var(--surface)', fontSize: 12 }} />
            </label>
          </div>
          {loadStatus && (
            <div className={`model-load-status ${loadStatus.type}`}>{loadStatus.msg}</div>
          )}
        </div>
      </div>
    </section>
  </div>
}

/* ═══════════════════════════════════════════════
   REQUESTS
   ═══════════════════════════════════════════════ */
function Requests({ requests }: { requests: RequestRow[] }) {
  return <section>
    <div className="section-heading">
      <div><p className="eyebrow">Metadata only</p><h2>Recent requests</h2></div>
      <span>{requests.length} records</span>
    </div>
    <div className="table-wrap">
      <table>
        <thead><tr><th>Request</th><th>Provider / model</th><th>Mode</th><th>Latency</th><th>Stability</th><th>Status</th></tr></thead>
        <tbody>
          {requests.map((r) => (
            <tr key={r.id}>
              <td><code>{r.id.slice(0, 18)}</code><small>{new Date(r.created_at).toLocaleString()}</small></td>
              <td><strong>{r.provider || '—'}</strong><small>{r.model || '—'}</small></td>
              <td><span className={`mode-badge ${r.runtime_mode}`}>{r.runtime_mode}</span></td>
              <td>
                <Tooltip text={`Response time: ${r.latency_ms}ms`}>
                  <span className="latency-cell">{r.latency_ms} ms</span>
                </Tooltip>
              </td>
              <td>
                <span className="flags">
                  {r.validation_passed && <Tooltip text="Validation passed"><ShieldCheck size={15} /></Tooltip>}
                  {r.repair_applied && <Tooltip text="Repair was applied"><Wrench size={15} /></Tooltip>}
                  {r.retry_count > 0 && <Tooltip text={`Retried ${r.retry_count} time(s)`}><TimerReset size={15} /></Tooltip>}
                </span>
              </td>
              <td><StatusMark status={r.status} /></td>
            </tr>
          ))}
        </tbody>
      </table>
      {requests.length === 0 && <Empty text="No request metadata stored yet." icon={ListTree} />}
    </div>
  </section>
}

/* ═══════════════════════════════════════════════
   PROVIDERS — Full configuration UI
   ═══════════════════════════════════════════════ */

type ProviderSettings = {
  name: string
  enabled: boolean
  url: string
  default_model: string
  timeout_seconds: number
}

type FullConfig = {
  provider_default: string
  providers: ProviderSettings[] | Record<string, Omit<ProviderSettings, 'name'>>
  runtime?: { mode?: string; log_level?: string; host?: string; port?: number }
  telemetry?: { log_prompts?: boolean; log_responses?: boolean; local_telemetry?: boolean }
}

function normalizeProviders(providers: ProviderSettings[] | Record<string, Omit<ProviderSettings, 'name'>> | undefined): ProviderSettings[] {
  if (!providers) return []
  if (Array.isArray(providers)) return providers
  return Object.entries(providers).map(([name, settings]) => ({ name, ...settings }))
}

const DEFAULT_URLS: Record<string, string> = {
  ollama: 'http://localhost:11434',
  lmstudio: 'http://localhost:1234/v1',
  openai_compatible_local: 'http://localhost:8000/v1',
}

const PROVIDER_NAMES: Record<string, string> = {
  ollama: 'Ollama',
  lmstudio: 'LM Studio',
  openai_compatible_local: 'OpenAI Compatible',
}

function Providers() {
  const [providers, setProviders] = useState<ProviderSettings[]>([])
  const [providerStatuses, setProviderStatuses] = useState<Record<string, { status: string; lastChecked: Date | null; error: string | null }>>({})
  const [defaultProvider, setDefaultProvider] = useState('lmstudio')
  const [doctor, setDoctor] = useState<Doctor | null>(null)
  const [saving, setSaving] = useState(false)
  const [saveMsg, setSaveMsg] = useState<{ type: string; msg: string } | null>(null)
  const [runningDiag, setRunningDiag] = useState(false)
  const [editingProvider, setEditingProvider] = useState<string | null>(null)
  const [editForm, setEditForm] = useState<ProviderSettings | null>(null)

  const loadProviders = useCallback(async () => {
    try {
      const [cfg, doc] = await Promise.all([
        getJSON<FullConfig>('/v1/gumi/config'),
        getJSON<Doctor>('/v1/gumi/doctor'),
      ])
      const normalized = normalizeProviders(cfg.providers)
      setProviders(normalized)
      setDefaultProvider(cfg.provider_default ?? 'lmstudio')
      setDoctor(doc)

      // Merge doctor check results into provider statuses
      const statuses: Record<string, { status: string; lastChecked: Date | null; error: string | null }> = {}
      for (const p of normalized) {
        statuses[p.name] = { status: 'unknown', lastChecked: null, error: null }
      }
      if (doc?.checks) {
        for (const check of doc.checks) {
          const pName = check.name.replace('provider_', '')
          if (statuses[pName]) {
            statuses[pName] = {
              status: check.status,
              lastChecked: new Date(),
              error: check.status !== 'ok' ? check.message : null,
            }
          }
        }
      }
      setProviderStatuses(statuses)
    } catch { /* runtime may be offline */ }
  }, [])

  useEffect(() => { void loadProviders() }, [loadProviders])

  const startEdit = (p: ProviderSettings) => {
    setEditingProvider(p.name)
    setEditForm({ ...p })
  }

  const cancelEdit = () => {
    setEditingProvider(null)
    setEditForm(null)
  }

  const updateEditField = (field: keyof ProviderSettings, value: any) => {
    if (!editForm) return
    setEditForm({ ...editForm, [field]: value })
  }

  const saveProvider = async (originalName: string) => {
    if (!editForm) return
    setSaving(true)
    setSaveMsg(null)
    try {
      const updatedProviders = providers.map(p =>
        p.name === originalName ? { ...editForm } : p
      )
      const payload = {
        provider_default: defaultProvider,
        providers: updatedProviders,
      }
      await getJSON('/v1/gumi/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })
      setProviders(updatedProviders)
      setEditingProvider(null)
      setEditForm(null)
      setSaveMsg({ type: 'success', msg: `${editForm.name} settings saved.` })
      setTimeout(() => setSaveMsg(null), 3000)
      // Refresh doctor to get new status
      void loadProviders()
    } catch (e: any) {
      setSaveMsg({ type: 'error', msg: e instanceof Error ? e.message : 'Save failed' })
    } finally { setSaving(false) }
  }

  const saveAllProviders = async () => {
    setSaving(true)
    setSaveMsg(null)
    try {
      const payload = {
        provider_default: defaultProvider,
        providers,
      }
      await getJSON('/v1/gumi/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })
      setSaveMsg({ type: 'success', msg: 'All provider settings saved.' })
      setTimeout(() => setSaveMsg(null), 3000)
      void loadProviders()
    } catch (e: any) {
      setSaveMsg({ type: 'error', msg: e instanceof Error ? e.message : 'Save failed' })
    } finally { setSaving(false) }
  }

  const checkProvider = async (name: string) => {
    setProviderStatuses(prev => ({
      ...prev,
      [name]: { ...prev[name], status: 'checking', lastChecked: null, error: null },
    }))
    try {
      const doc = await getJSON<Doctor>('/v1/gumi/doctor')
      setDoctor(doc)
      const check = doc.checks?.find(c => c.name === `provider_${name}` || c.name === name)
      setProviderStatuses(prev => ({
        ...prev,
        [name]: {
          status: check?.status ?? 'unknown',
          lastChecked: new Date(),
          error: check?.status !== 'ok' ? (check?.message ?? null) : null,
        },
      }))
    } catch (e: any) {
      setProviderStatuses(prev => ({
        ...prev,
        [name]: { status: 'error', lastChecked: new Date(), error: e instanceof Error ? e.message : 'Check failed' },
      }))
    }
  }

  const runDiagnostics = async () => {
    setRunningDiag(true)
    try {
      const doc = await getJSON<Doctor>('/v1/gumi/doctor')
      setDoctor(doc)
      if (doc?.checks) {
        const statuses = { ...providerStatuses }
        for (const check of doc.checks) {
          const pName = check.name.replace('provider_', '')
          if (statuses[pName]) {
            statuses[pName] = {
              status: check.status,
              lastChecked: new Date(),
              error: check.status !== 'ok' ? check.message : null,
            }
          }
        }
        setProviderStatuses(statuses)
      }
    } catch { /* ignore */ }
    finally { setRunningDiag(false) }
  }

  const statusColor = (status: string) => {
    switch (status) {
      case 'ok': case 'running': return 'var(--green)'
      case 'degraded': case 'warning': return 'var(--amber)'
      case 'error': case 'offline': case 'failed': return 'var(--red)'
      case 'checking': return 'var(--blue)'
      default: return 'var(--muted)'
    }
  }

  const statusIcon = (status: string) => {
    switch (status) {
      case 'ok': case 'running': return <Wifi size={14} />
      case 'degraded': case 'warning': return <AlertTriangle size={14} />
      case 'error': case 'offline': case 'failed': return <WifiOff size={14} />
      case 'checking': return <Loader size={14} className="spin" />
      default: return <HelpCircle size={14} />
    }
  }

  const statusLabel = (status: string) => {
    switch (status) {
      case 'ok': return 'Connected'
      case 'running': return 'Running'
      case 'degraded': return 'Degraded'
      case 'warning': return 'Warning'
      case 'error': return 'Error'
      case 'offline': return 'Offline'
      case 'failed': return 'Failed'
      case 'checking': return 'Checking...'
      default: return 'Unknown'
    }
  }

  return <div className="page-stack">
    <section>
      <div className="section-heading">
        <div><p className="eyebrow">Local inference</p><h2>Provider adapters</h2></div>
        <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
          {saveMsg && <div className={`config-toast ${saveMsg.type}`} style={{ margin: 0 }}>{saveMsg.msg}</div>}
          <Tooltip text="Run full diagnostics on all providers">
            <button className="icon-button" onClick={runDiagnostics} disabled={runningDiag}>
              <Bug size={16} className={runningDiag ? 'spin' : ''} />
            </button>
          </Tooltip>
          <Tooltip text="Refresh provider data">
            <button className="icon-button" onClick={loadProviders}><RefreshCw size={16} /></button>
          </Tooltip>
          <Tooltip text="Save all provider changes">
            <button className="save-config-btn" onClick={saveAllProviders} disabled={saving} style={{ fontSize: 12, padding: '7px 16px' }}>
              {saving ? 'Saving...' : 'Save Changes'}
            </button>
          </Tooltip>
        </div>
      </div>

      {/* Default provider selector */}
      <div className="provider-default-bar">
        <span>Default provider:</span>
        <select value={defaultProvider} onChange={e => setDefaultProvider(e.target.value)}>
          {providers.map(p => (
            <option key={p.name} value={p.name}>{PROVIDER_NAMES[p.name] || p.name}</option>
          ))}
        </select>
        <span className="provider-default-hint">The default provider is used when no provider is specified in a request.</span>
      </div>

      {/* Provider cards */}
      <div className="provider-config-grid">
        {providers.map((p) => {
          const ps = providerStatuses[p.name] || { status: 'unknown', lastChecked: null, error: null }
          const isEditing = editingProvider === p.name
          const isDefault = defaultProvider === p.name

          return (
            <article key={p.name} className={`provider-config-card ${isDefault ? 'default' : ''}`}>
              <div className="provider-config-header">
                <div className="provider-config-title">
                  <Server size={20} />
                  <h3>{PROVIDER_NAMES[p.name] || p.name}</h3>
                  {isDefault && <span className="provider-default-badge">Default</span>}
                </div>
                <div className="provider-config-status">
                  <span className="provider-status-indicator" style={{ color: statusColor(ps.status) }}>
                    {statusIcon(ps.status)}
                    {statusLabel(ps.status)}
                  </span>
                  {ps.lastChecked && (
                    <span className="provider-last-checked">
                      <Clock size={11} />
                      {ps.lastChecked.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                    </span>
                  )}
                </div>
              </div>

              {isEditing && editForm ? (
                <div className="provider-edit-form">
                  <div className="provider-edit-row">
                    <label>
                      <span>URL</span>
                      <input
                        type="text"
                        value={editForm.url}
                        onChange={e => updateEditField('url', e.target.value)}
                        placeholder={DEFAULT_URLS[p.name] || 'http://localhost:1234/v1'}
                      />
                    </label>
                    <label className="provider-toggle-label">
                      <span>Enabled</span>
                      <label className="toggle-switch">
                        <input
                          type="checkbox"
                          checked={editForm.enabled}
                          onChange={e => updateEditField('enabled', e.target.checked)}
                        />
                        <span className="toggle-slider" />
                      </label>
                    </label>
                  </div>
                  <div className="provider-edit-row">
                    <label>
                      <span>Default model</span>
                      <input
                        type="text"
                        value={editForm.default_model}
                        onChange={e => updateEditField('default_model', e.target.value)}
                        placeholder="local:auto"
                      />
                    </label>
                    <label>
                      <span>Timeout (seconds)</span>
                      <div className="provider-timeout-input">
                        <input
                          type="range"
                          min={5}
                          max={300}
                          step={5}
                          value={editForm.timeout_seconds}
                          onChange={e => updateEditField('timeout_seconds', Number(e.target.value))}
                        />
                        <input
                          type="number"
                          min={5}
                          max={300}
                          value={editForm.timeout_seconds}
                          onChange={e => updateEditField('timeout_seconds', Number(e.target.value))}
                        />
                      </div>
                    </label>
                  </div>
                  <div className="provider-edit-actions">
                    <button className="send-btn" onClick={() => saveProvider(p.name)} disabled={saving} style={{ fontSize: 12, padding: '6px 14px' }}>
                      {saving ? 'Saving...' : <><Check size={13} /> Save</>}
                    </button>
                    <button className="provider-cancel-btn" onClick={cancelEdit}>Cancel</button>
                    <button className="provider-test-btn" onClick={() => checkProvider(p.name)}>
                      <Play size={12} /> Test Connection
                    </button>
                  </div>
                  {ps.error && (
                    <div className="provider-error-msg">
                      <CircleAlert size={14} />
                      {ps.error}
                    </div>
                  )}
                </div>
              ) : (
                <div className="provider-config-body">
                  <div className="provider-config-details">
                    <div className="provider-detail-row">
                      <span className="provider-detail-label">URL</span>
                      <code className="provider-detail-value">{p.url || DEFAULT_URLS[p.name] || '—'}</code>
                    </div>
                    <div className="provider-detail-row">
                      <span className="provider-detail-label">Model</span>
                      <span className="provider-detail-value">{p.default_model || 'local:auto'}</span>
                    </div>
                    <div className="provider-detail-row">
                      <span className="provider-detail-label">Timeout</span>
                      <span className="provider-detail-value">{p.timeout_seconds || 60}s</span>
                    </div>
                    <div className="provider-detail-row">
                      <span className="provider-detail-label">Status</span>
                      <span className="provider-detail-value">
                        <span className={`provider-enabled-dot ${p.enabled ? 'on' : 'off'}`} />
                        {p.enabled ? 'Enabled' : 'Disabled'}
                      </span>
                    </div>
                  </div>
                  {ps.error && (
                    <div className="provider-error-msg">
                      <CircleAlert size={14} />
                      {ps.error}
                    </div>
                  )}
                  <div className="provider-config-actions">
                    <Tooltip text="Edit provider settings">
                      <button className="provider-action-btn" onClick={() => startEdit(p)}>
                        <Pencil size={13} /> Edit
                      </button>
                    </Tooltip>
                    <Tooltip text="Test connection to this provider">
                      <button className="provider-action-btn" onClick={() => checkProvider(p.name)}>
                        <Play size={12} /> Check Now
                      </button>
                    </Tooltip>
                  </div>
                </div>
              )}
            </article>
          )
        })}
        {providers.length === 0 && <Empty text="No providers configured." />}
      </div>
    </section>

    {/* Diagnostics / Quick Setup helper */}
    <section className="provider-diagnostics-section">
      <div className="section-heading">
        <div><p className="eyebrow">Troubleshooting</p><h2>Connection diagnostics</h2></div>
        <Tooltip text="Run all health checks">
          <button className="send-btn" onClick={runDiagnostics} disabled={runningDiag} style={{ fontSize: 12, padding: '7px 16px' }}>
            <Bug size={14} /> {runningDiag ? 'Running...' : 'Run Diagnostics'}
          </button>
        </Tooltip>
      </div>

      <div className="diagnostics-grid">
        <div className="diagnostics-card">
          <h4><Info size={15} /> Quick tips</h4>
          <ul className="diagnostics-tips">
            <li><strong>Check the URL</strong> — Make sure the provider URL is correct and reachable from this machine.</li>
            <li><strong>Is the provider running?</strong> — Ensure the provider service (Ollama, LM Studio, etc.) is started.</li>
            <li><strong>Network check</strong> — If using a remote provider, verify network connectivity and firewall rules.</li>
            <li><strong>API key</strong> — Some providers require an API key. Check environment variables.</li>
            <li><strong>Model availability</strong> — The default model must be downloaded and available on the provider.</li>
          </ul>
        </div>

        <div className="diagnostics-card">
          <h4><HeartPulse size={15} /> Health check results</h4>
          {doctor ? (
            <div className="doctor-check-list">
              {doctor.checks?.map((c) => (
                <div key={c.name} className="doctor-check-row">
                  <StatusMark status={c.status} />
                  <div className="doctor-check-info">
                    <strong>{c.name.replaceAll('_', ' ')}</strong>
                    <p>{c.message}</p>
                    {c.suggestion && <small>{c.suggestion}</small>}
                  </div>
                </div>
              ))}
              {(!doctor.checks || doctor.checks.length === 0) && (
                <span style={{ color: 'var(--muted)', fontSize: 12 }}>No health checks available.</span>
              )}
            </div>
          ) : (
            <span style={{ color: 'var(--muted)', fontSize: 12 }}>Run diagnostics to see results.</span>
          )}
        </div>
      </div>
    </section>
  </div>
}

/* ═══════════════════════════════════════════════
   PROFILES
   ═══════════════════════════════════════════════ */
function ProfilesView({ profiles }: { profiles: Profile[] }) {
  return <section>
    <div className="section-heading"><div><p className="eyebrow">Intelligence packs</p><h2>Model profiles</h2></div><span>{profiles.length} loaded</span></div>
    <div className="profile-list">
      {profiles.map((p) => (
        <article key={p.id}>
          <div><span className="profile-family">{p.family}</span><h3>{p.name || p.id}</h3><code>{p.id}</code></div>
          <dl><dt>Size</dt><dd>{p.size || 'unknown'}</dd><dt>Context</dt><dd>{p.context_limit ? `${p.context_limit.toLocaleString()} tokens` : 'profile default'}</dd><dt>Aliases</dt><dd>{p.aliases?.slice(0, 2).join(', ') || 'fallback match'}</dd></dl>
        </article>
      ))}
    </div>
  </section>
}

/* ═══════════════════════════════════════════════
   ANALYTICS
   ═══════════════════════════════════════════════ */
function Analytics({ requests }: { requests: RequestRow[] }) {
  const latencyBins = useMemo(() => {
    if (!requests.length) return []
    const sorted = [...requests].sort((a, b) => a.latency_ms - b.latency_ms)
    const bins: { range: string; count: number; color: string }[] = []
    const thresholds = [100, 500, 1000, 2000, 5000]
    let prev = 0
    for (const t of thresholds) {
      const count = sorted.filter(r => r.latency_ms >= prev && r.latency_ms < t).length
      bins.push({ range: `${prev}–${t}ms`, count, color: t <= 500 ? 'var(--green)' : t <= 2000 ? 'var(--amber)' : 'var(--red)' })
      prev = t
    }
    const over = sorted.filter(r => r.latency_ms >= prev).length
    if (over) bins.push({ range: `${prev}+ms`, count: over, color: 'var(--red)' })
    return bins
  }, [requests])

  const providerStats = useMemo(() => {
    const map = new Map<string, { count: number; latencies: number[] }>()
    for (const r of requests) {
      const p = r.provider || 'unknown'
      if (!map.has(p)) map.set(p, { count: 0, latencies: [] })
      const s = map.get(p)!
      s.count++
      s.latencies.push(r.latency_ms)
    }
    return Array.from(map.entries()).map(([provider, s]) => ({
      provider,
      count: s.count,
      avgLatency: s.latencies.reduce((a, b) => a + b, 0) / s.latencies.length || 0,
      pct: Math.round((s.count / requests.length) * 100) || 0,
    }))
  }, [requests])

  const maxCount = Math.max(...latencyBins.map(b => b.count), 1)

  const successRate = requests.length
    ? Math.round((requests.filter(r => ['success', 'repaired', 'retried'].includes(r.status)).length / requests.length) * 100)
    : 100

  return <div className="page-stack">
    <section className="metric-strip">
      <div>
        <div className="metric-icon"><BarChart3 size={18} /></div>
        <span>Total requests</span>
        <strong>{requests.length}</strong>
        <small>all time</small>
      </div>
      <div>
        <div className="metric-icon"><TrendingUp size={18} /></div>
        <span>Success rate</span>
        <strong style={{ color: successRate >= 80 ? 'var(--green)' : 'var(--red)' }}>{successRate}%</strong>
        <small>across all requests</small>
      </div>
      <div>
        <div className="metric-icon"><TimerReset size={18} /></div>
        <span>Avg latency</span>
        <strong>{requests.length ? Math.round(requests.reduce((s, r) => s + r.latency_ms, 0) / requests.length).toLocaleString() : 0} ms</strong>
        <small>gateway to response</small>
      </div>
      <div>
        <div className="metric-icon"><Wrench size={18} /></div>
        <span>Repair rate</span>
        <strong>{requests.length ? Math.round((requests.filter(r => r.repair_applied).length / requests.length) * 100) : 0}%</strong>
        <small>auto-corrections</small>
      </div>
    </section>

    <section>
      <div className="section-heading"><h2>Latency distribution</h2></div>
      <div className="bar-chart">
        {latencyBins.length === 0 && <Empty text="No data yet." icon={BarChart3} />}
        {latencyBins.map((bin) => (
          <div className="bar-row" key={bin.range}>
            <span className="bar-label">{bin.range}</span>
            <div className="bar-track">
              <div className="bar-fill" style={{ width: `${(bin.count / maxCount) * 100}%`, background: bin.color }} />
            </div>
            <span className="bar-value">{bin.count}</span>
          </div>
        ))}
      </div>
    </section>

    <section className="split-section">
      <div>
        <div className="section-heading"><h2>Provider breakdown</h2></div>
        <div className="provider-list">
          {providerStats.map((s) => (
            <div className="provider-row" key={s.provider}>
              <Cpu size={18} />
              <div><strong>{s.provider}</strong><small>{s.count} requests · {Math.round(s.avgLatency).toLocaleString()} ms avg</small></div>
              <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--muted)' }}>{s.pct}%</span>
            </div>
          ))}
          {providerStats.length === 0 && <Empty text="No request data." icon={BarChart3} />}
        </div>
      </div>
      <div>
        <div className="section-heading"><h2>Recent trends</h2></div>
        <div className="activity-list">
          {requests.slice(0, 10).map((r) => (
            <div key={r.id}>
              <Tooltip text={statusDescriptions[r.status] || `Status: ${r.status}`}>
                <span className={`activity-icon ${r.status}`}><TrendingUp size={15} /></span>
              </Tooltip>
              <div><strong>{r.model || 'local:auto'}</strong><small>{r.provider} · {r.latency_ms} ms {r.validation_passed ? '✓' : '✗'} {r.repair_applied ? '(repaired)' : ''}</small></div>
              <time>{new Date(r.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}</time>
            </div>
          ))}
          {requests.length === 0 && <Empty text="No requests yet." icon={BarChart3} />}
        </div>
      </div>
    </section>
  </div>
}

/* ═══════════════════════════════════════════════
   MEMORY
   ═══════════════════════════════════════════════ */
function MemoryView() {
  const [facts, setFacts] = useState<MemoryFact[]>([])
  const [status, setStatus] = useState<MemoryStatus | null>(null)
  const [modelFit, setModelFit] = useState<ModelFitEntry[]>([])
  const [selfTuning, setSelfTuning] = useState<SelfTuningResponse | null>(null)
  const [loading, setLoading] = useState(false)
  const [tab, setTab] = useState<'facts' | 'model-fit' | 'self-tuning' | 'status'>('facts')
  const [newKey, setNewKey] = useState('')
  const [newValue, setNewValue] = useState('')
  const [actionMsg, setActionMsg] = useState<{ type: string; msg: string } | null>(null)

  const fetchAll = useCallback(async () => {
    setLoading(true)
    try {
      const [factsRes, statusRes, fitRes, tuningRes] = await Promise.all([
        getJSON<{ enabled: boolean; facts: MemoryFact[] }>('/v1/gumi/memory/facts').catch(() => ({ enabled: false, facts: [] })),
        getJSON<MemoryStatus>('/v1/gumi/memory/status').catch(() => null),
        getJSON<{ enabled: boolean; entries: ModelFitEntry[] }>('/v1/gumi/memory/model-fit').catch(() => ({ enabled: false, entries: [] })),
        getJSON<SelfTuningResponse>('/v1/gumi/self-tuning').catch(() => ({ enabled: false, reason: 'unavailable' })),
      ])
      setFacts(factsRes.facts ?? [])
      setStatus(statusRes)
      setModelFit(fitRes.entries ?? [])
      setSelfTuning(tuningRes)
    } catch { /* memory may be unavailable */ }
    finally { setLoading(false) }
  }, [])

  useEffect(() => { void fetchAll() }, [fetchAll])

  const handleCreateFact = async () => {
    if (!newKey.trim() || !newValue.trim()) return
    setActionMsg(null)
    try {
      await getJSON('/v1/gumi/memory/facts', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: newKey.trim(), value: newValue.trim() }),
      })
      setNewKey(''); setNewValue('')
      setActionMsg({ type: 'success', msg: 'Fact created' })
      void fetchAll()
    } catch (e: any) {
      setActionMsg({ type: 'error', msg: e instanceof Error ? e.message : 'Failed' })
    }
  }

  const handleClear = async () => {
    if (!window.confirm('Clear all memory data? This cannot be undone.')) return
    setActionMsg(null)
    try {
      await getJSON('/v1/gumi/memory/clear', { method: 'POST' })
      setActionMsg({ type: 'success', msg: 'Memory cleared' })
      void fetchAll()
    } catch (e: any) {
      setActionMsg({ type: 'error', msg: e instanceof Error ? e.message : 'Failed' })
    }
  }

  const formatBytes = (b: number) => b < 1024 ? `${b} B` : b < 1024 * 1024 ? `${(b / 1024).toFixed(1)} KB` : `${(b / (1024 * 1024)).toFixed(1)} MB`

  function confidenceLevel(conf: number): 'high' | 'medium' | 'low' {
    if (conf >= 0.8) return 'high'
    if (conf >= 0.4) return 'medium'
    return 'low'
  }

  function accessHeat(count: number): 'cold' | 'warm' | 'hot' {
    if (count >= 20) return 'hot'
    if (count >= 5) return 'warm'
    return 'cold'
  }

  function ttlStatus(ttl: number): 'ok' | 'warning' | 'critical' {
    if (ttl <= 0) return 'critical'
    if (ttl < 300) return 'warning'
    return 'ok'
  }

  function formatTTL(seconds: number): string {
    if (seconds <= 0) return 'Expired'
    if (seconds < 60) return `${seconds}s`
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`
    return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`
  }

  return <div className="page-stack">
    <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap', paddingBottom: 8 }}>
      <div className="section-heading" style={{ margin: 0, flex: 1 }}>
        <div><p className="eyebrow">Knowledge engine</p><h2>Memory</h2></div>
      </div>
      <Tooltip text="Refresh memory data">
        <button className="icon-button" onClick={fetchAll}><RefreshCw size={16} /></button>
      </Tooltip>
      <Tooltip text="Delete all stored memory facts">
        <button className="send-btn" style={{ background: 'var(--red)', fontSize: 11, padding: '5px 14px' }} onClick={handleClear}><Trash2 size={12} /> Clear All</button>
      </Tooltip>
    </div>

    <div className="tabs" style={{ display: 'flex', gap: 4, borderBottom: '1px solid var(--line)', marginBottom: 16 }}>
      {(['facts', 'model-fit', 'self-tuning', 'status'] as const).map(t => (
        <button key={t} className={`tab ${tab === t ? 'active' : ''}`} onClick={() => setTab(t)}
          style={{ padding: '8px 16px', border: 0, borderBottom: tab === t ? '2px solid var(--blue)' : '2px solid transparent', background: 'transparent', cursor: 'pointer', fontSize: 12, fontWeight: tab === t ? 600 : 400, color: tab === t ? 'var(--blue)' : 'var(--muted)' }}>
          {t === 'facts' ? 'Facts' : t === 'model-fit' ? 'Model Fit' : t === 'self-tuning' ? 'Self-Tuning' : 'Status'}
        </button>
      ))}
    </div>

    {actionMsg && (
      <div className={`config-toast ${actionMsg.type}`} style={{ marginBottom: 12 }}>{actionMsg.msg}</div>
    )}

    {tab === 'facts' && <section>
      <div className="split-section">
        <div>
          <h4 style={{ fontSize: 13, margin: '0 0 10px' }}>Existing Facts</h4>
          <div className="provider-list" style={{ maxHeight: 400, overflowY: 'auto' }}>
            {facts.length === 0 && <Empty text="No memory facts stored." icon={Bookmark} />}
            {facts.map((f, i) => {
              const conf = confidenceLevel(f.confidence)
              const heat = accessHeat(f.access_count)
              const ttl = ttlStatus(f.ttl_seconds)
              return (
                <div className="memory-fact-row" key={i}>
                  <Bookmark size={16} />
                  <div className="memory-fact-info">
                    <strong>{f.key}</strong>
                    <div className="memory-fact-meta">
                      <Tooltip text={confidenceDescriptions[conf]}>
                        <span className={`confidence-badge ${conf}`}>
                          {conf === 'high' ? 'High' : conf === 'medium' ? 'Med' : 'Low'}
                        </span>
                      </Tooltip>
                      <Tooltip text={`Accessed ${f.access_count} times — ${heat === 'hot' ? 'frequently used' : heat === 'warm' ? 'moderately used' : 'rarely accessed'}`}>
                        <span className={`access-heat ${heat}`}>
                          {heat === 'hot' ? <Zap size={11} /> : heat === 'warm' ? <Activity size={11} /> : <EyeOff size={11} />}
                          {f.access_count}
                        </span>
                      </Tooltip>
                      <Tooltip text={`TTL remaining: ${formatTTL(f.ttl_seconds)}`}>
                        <span className={`ttl-indicator ${ttl}`}>
                          <Clock size={11} />
                          {formatTTL(f.ttl_seconds)}
                        </span>
                      </Tooltip>
                    </div>
                    <div className="confidence-bar-track">
                      <div className={`confidence-bar-fill ${conf}`} style={{ width: `${Math.round(f.confidence * 100)}%` }} />
                    </div>
                  </div>
                  <code className="memory-fact-value" title={f.value}>{f.value}</code>
                </div>
              )
            })}
          </div>
        </div>
        <div>
          <h4 style={{ fontSize: 13, margin: '0 0 10px' }}>Add Fact</h4>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            <input type="text" placeholder="Key" value={newKey} onChange={e => setNewKey(e.target.value)} style={{ padding: '7px 10px', border: '1px solid var(--line)', borderRadius: 6, background: 'var(--surface)' }} />
            <input type="text" placeholder="Value" value={newValue} onChange={e => setNewValue(e.target.value)} style={{ padding: '7px 10px', border: '1px solid var(--line)', borderRadius: 6, background: 'var(--surface)' }} />
            <button className="send-btn" onClick={handleCreateFact} disabled={!newKey.trim() || !newValue.trim()}><Plus size={14} /> Create Fact</button>
          </div>
        </div>
      </div>
    </section>}

    {tab === 'model-fit' && <section>
      <div className="table-wrap">
        <table>
          <thead><tr><th>Model</th><th>Task</th><th>Difficulty</th><th>Attempts</th><th>Successes</th><th>Avg Latency</th></tr></thead>
          <tbody>
            {modelFit.length === 0 && <tr><td colSpan={6} style={{ textAlign: 'center', color: 'var(--muted)', padding: 24 }}>No model-fit data yet.</td></tr>}
            {modelFit.map((entry, i) => (
              <tr key={i}>
                <td><code>{entry.model_id}</code></td>
                <td>{entry.task_type}</td>
                <td>
                  <Tooltip text={`Difficulty ${entry.difficulty}/5 — ${entry.difficulty <= 2 ? 'Easy' : entry.difficulty <= 3 ? 'Moderate' : 'Hard'}`}>
                    <span className="difficulty-stars">
                      {Array.from({ length: 5 }, (_, j) => (
                        <span key={j} className={`star ${j < entry.difficulty ? 'filled' : ''}`}>★</span>
                      ))}
                    </span>
                  </Tooltip>
                </td>
                <td>{entry.attempts}</td>
                <td>{entry.successes}</td>
                <td>{Math.round(entry.avg_latency_ms).toLocaleString()} ms</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>}

    {tab === 'self-tuning' && <section>
      {!selfTuning?.enabled ? (
        <Empty text={selfTuning?.reason || 'Self-tuning is not enabled.'} icon={Activity} />
      ) : (
        <div className="page-stack">
          <div className="memory-status-cards">
            <div className="memory-status-card">
              <div className="memory-status-info">
                <span className="memory-status-label">Status</span>
                <strong className="text-green">Enabled</strong>
              </div>
            </div>
            <div className="memory-status-card">
              <div className="memory-status-info">
                <span className="memory-status-label">Total attempts</span>
                <strong>{selfTuning.snapshot?.total_attempts ?? 0}</strong>
              </div>
            </div>
            <div className="memory-status-card">
              <div className="memory-status-info">
                <span className="memory-status-label">Persist snapshot</span>
                <strong>{selfTuning.config?.persist_snapshot ? 'On' : 'Off'}</strong>
              </div>
            </div>
            <div className="memory-status-card">
              <div className="memory-status-info">
                <span className="memory-status-label">Epsilon</span>
                <strong>{selfTuning.config?.epsilon ?? '—'}</strong>
              </div>
            </div>
          </div>

          <h4 style={{ fontSize: 13, margin: '12px 0 8px' }}>Rule overrides</h4>
          <div className="table-wrap">
            <table>
              <thead><tr><th>Rule</th><th>Prefer</th><th>Min coding</th><th>Min context</th><th>Reason</th></tr></thead>
              <tbody>
                {(selfTuning.snapshot?.rule_overrides ?? []).length === 0 && (
                  <tr><td colSpan={5} style={{ textAlign: 'center', color: 'var(--muted)', padding: 24 }}>No rule overrides yet.</td></tr>
                )}
                {(selfTuning.snapshot?.rule_overrides ?? []).map((o, i) => (
                  <tr key={i}>
                    <td><code>{o.rule_name}</code></td>
                    <td>{o.prefer || '—'}</td>
                    <td>{o.min_coding || '—'}</td>
                    <td>{o.min_context ?? '—'}</td>
                    <td>{o.reason || '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <div className="split-section" style={{ marginTop: 16 }}>
            <div>
              <h4 style={{ fontSize: 13, margin: '0 0 8px' }}>Model boosts</h4>
              <div className="provider-list">
                {Object.keys(selfTuning.snapshot?.model_boosts ?? {}).length === 0 && <Empty text="No boosts." icon={Zap} />}
                {Object.entries(selfTuning.snapshot?.model_boosts ?? {}).map(([model, score]) => (
                  <div className="memory-fact-row" key={model}>
                    <strong><code>{model}</code></strong>
                    <span className="text-green">+{score.toFixed(3)}</span>
                  </div>
                ))}
              </div>
            </div>
            <div>
              <h4 style={{ fontSize: 13, margin: '0 0 8px' }}>Model demotes</h4>
              <div className="provider-list">
                {Object.keys(selfTuning.snapshot?.model_demotes ?? {}).length === 0 && <Empty text="No demotes." icon={Activity} />}
                {Object.entries(selfTuning.snapshot?.model_demotes ?? {}).map(([model, score]) => (
                  <div className="memory-fact-row" key={model}>
                    <strong><code>{model}</code></strong>
                    <span style={{ color: 'var(--red)' }}>-{Math.abs(score).toFixed(3)}</span>
                  </div>
                ))}
              </div>
            </div>
          </div>

          <h4 style={{ fontSize: 13, margin: '16px 0 8px' }}>Recent adjustments</h4>
          <div className="table-wrap">
            <table>
              <thead><tr><th>Kind</th><th>Target</th><th>From</th><th>To</th><th>Reason</th></tr></thead>
              <tbody>
                {(selfTuning.snapshot?.adjustments ?? []).length === 0 && (
                  <tr><td colSpan={5} style={{ textAlign: 'center', color: 'var(--muted)', padding: 24 }}>No adjustments recorded.</td></tr>
                )}
                {(selfTuning.snapshot?.adjustments ?? []).map((a, i) => (
                  <tr key={i}>
                    <td>{a.kind}</td>
                    <td><code>{a.target}</code></td>
                    <td>{a.from}</td>
                    <td>{a.to}</td>
                    <td>{a.reason || '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {loading && <p style={{ color: 'var(--muted)', fontSize: 12 }}>Refreshing…</p>}
        </div>
      )}
    </section>}

    {tab === 'status' && <section>
      {status ? (
        <div className="memory-status-cards">
          <div className="memory-status-card">
            <div className="memory-status-icon"><Database size={22} /></div>
            <div className="memory-status-info">
              <span className="memory-status-label">Memory engine</span>
              <strong className={status.enabled ? 'text-green' : 'text-muted'}>{status.enabled ? 'Enabled' : 'Disabled'}</strong>
            </div>
          </div>
          <div className="memory-status-card">
            <div className="memory-status-icon"><Bookmark size={22} /></div>
            <div className="memory-status-info">
              <span className="memory-status-label">Facts stored</span>
              <strong>{status.facts_count ?? '—'}</strong>
            </div>
          </div>
          <div className="memory-status-card">
            <div className="memory-status-icon"><Hash size={22} /></div>
            <div className="memory-status-info">
              <span className="memory-status-label">Model-fit records</span>
              <strong>{status.model_fit_entries ?? '—'}</strong>
            </div>
          </div>
          <div className="memory-status-card">
            <div className="memory-status-icon"><Zap size={22} /></div>
            <div className="memory-status-info">
              <span className="memory-status-label">Injection budget</span>
              <strong>{status.injection_budget ? `${status.injection_budget} tok` : '—'}</strong>
            </div>
          </div>
        </div>
      ) : (
        <Empty text="Memory status unavailable." icon={Database} />
      )}
      {status?.database_path && (
        <p style={{ fontSize: 11, color: 'var(--muted)', marginTop: 8 }}>DB: {status.database_path}</p>
      )}
    </section>}
  </div>
}

/* ═══════════════════════════════════════════════
   LIVE LOGS
   ═══════════════════════════════════════════════ */
function LiveLogs() {
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [filter, setFilter] = useState('all')
  const [connected, setConnected] = useState(false)
  const logRef = useRef<HTMLDivElement>(null)
  const [autoScroll, setAutoScroll] = useState(true)

  useEffect(() => {
    const evtSource = new EventSource('/api/v1/gumi/logs/stream')
    evtSource.onopen = () => setConnected(true)
    evtSource.onmessage = (event) => {
      try {
        const entry = JSON.parse(event.data) as LogEntry
        setLogs(prev => [...prev.slice(-500), entry])
      } catch { /* ignore parse errors */ }
    }
    evtSource.onerror = () => setConnected(false)
    return () => evtSource.close()
  }, [])

  useEffect(() => {
    if (autoScroll && logRef.current) {
      logRef.current.scrollTop = logRef.current.scrollHeight
    }
  }, [logs, autoScroll])

  const filtered = useMemo(() =>
    filter === 'all' ? logs : logs.filter(l => l.level === filter),
    [logs, filter]
  )

  const clearLogs = () => setLogs([])

  return <div className="logs-container">
    <div className="logs-controls">
      <span className={`log-stream-indicator`}><i />{connected ? 'Connected' : 'Disconnected'}</span>
      <select value={filter} onChange={e => setFilter(e.target.value)}>
        <option value="all">All levels</option>
        <option value="INFO">INFO</option>
        <option value="ERROR">ERROR</option>
        <option value="DEBUG">DEBUG</option>
      </select>
      <label style={{ fontSize: 12, display: 'flex', alignItems: 'center', gap: 4, color: 'var(--muted)' }}>
        <input type="checkbox" checked={autoScroll} onChange={e => setAutoScroll(e.target.checked)} />
        Auto-scroll
      </label>
      <Tooltip text="Clear all log entries">
        <button className="icon-button" onClick={clearLogs}><Trash2 size={14} /></button>
      </Tooltip>
      <span style={{ fontSize: 11, color: 'var(--muted)', marginLeft: 'auto' }}>{logs.length} entries</span>
    </div>
    <div className="log-viewport" ref={logRef}>
      {filtered.length === 0 && <div className="log-empty">Waiting for log entries...</div>}
      {filtered.map((entry, i) => (
        <div className="log-entry" key={i}>
          <span className="log-time">{entry.timestamp?.slice(11, 19) ?? ''}</span>
          <span className={`log-level ${entry.level}`}>{entry.level}</span>
          <span className="log-msg">{entry.message}</span>
          {entry.fields && <span className="log-fields">{entry.fields}</span>}
        </div>
      ))}
    </div>
  </div>
}

/* ═══════════════════════════════════════════════
   CONFIG — Structured settings form with YAML advanced toggle
   ═══════════════════════════════════════════════ */
function ConfigView({ config }: { config: object | null }) {
  const [showAdvanced, setShowAdvanced] = useState(false)
  const [yamlText, setYamlText] = useState('')
  const [saveStatus, setSaveStatus] = useState<{ type: string; msg: string } | null>(null)
  const [saving, setSaving] = useState(false)

  // Structured form state
  const [runtimeMode, setRuntimeMode] = useState('stabilized')
  const [logLevel, setLogLevel] = useState('info')
  const [host, setHost] = useState('127.0.0.1')
  const [port, setPort] = useState(8787)
  const [logPrompts, setLogPrompts] = useState(false)
  const [logResponses, setLogResponses] = useState(false)
  const [localTelemetry, setLocalTelemetry] = useState(true)

  useEffect(() => {
    if (config) {
      const c = config as FullConfig
      setRuntimeMode(c.runtime?.mode ?? 'stabilized')
      setLogLevel(c.runtime?.log_level ?? 'info')
      setHost(c.runtime?.host ?? '127.0.0.1')
      setPort(c.runtime?.port ?? 8787)
      setLogPrompts(c.telemetry?.log_prompts ?? false)
      setLogResponses(c.telemetry?.log_responses ?? false)
      setLocalTelemetry(c.telemetry?.local_telemetry ?? true)
      setYamlText(JSON.stringify({ ...c, providers: normalizeProviders(c.providers) }, null, 2))
    }
  }, [config])

  const buildConfigPayload = useCallback((): FullConfig => {
    const c = config as FullConfig | null
    return {
      provider_default: c?.provider_default ?? 'lmstudio',
      providers: normalizeProviders(c?.providers),
      runtime: {
        mode: runtimeMode,
        log_level: logLevel,
        host,
        port,
      },
      telemetry: {
        log_prompts: logPrompts,
        log_responses: logResponses,
        local_telemetry: localTelemetry,
      },
    }
  }, [config, runtimeMode, logLevel, host, port, logPrompts, logResponses, localTelemetry])

  const handleSave = async () => {
    setSaving(true)
    setSaveStatus(null)
    try {
      let payload: any
      if (showAdvanced) {
        payload = JSON.parse(yamlText)
      } else {
        payload = buildConfigPayload()
      }
      await getJSON('/v1/gumi/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })
      setSaveStatus({ type: 'success', msg: 'Configuration saved successfully.' })
      setTimeout(() => setSaveStatus(null), 3000)
    } catch (e: any) {
      setSaveStatus({ type: 'error', msg: e instanceof Error ? e.message : 'Save failed' })
    } finally { setSaving(false) }
  }

  const modeOptions = [
    { value: 'direct', label: 'Direct', desc: 'Fastest — no adaptive processing. Requests pass through directly.' },
    { value: 'stabilized', label: 'Stabilized', desc: 'Balanced — validation and repair for reliability.' },
    { value: 'gumi-structured', label: 'Gumi Structured', desc: 'Full intelligence pipeline — context assembly, prompt optimization, repair.' },
  ]

  const logOptions = ['debug', 'info', 'warn', 'error']

  return <div className="config-editor">
    <div className="config-editor-form">
      <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 8 }}>
        <div className="section-heading" style={{ margin: 0, flex: 1 }}>
          <div><p className="eyebrow">Resolved and redacted</p><h2>Runtime configuration</h2></div>
        </div>
        <Tooltip text={showAdvanced ? 'Switch to structured form' : 'Switch to YAML editor'}>
          <button className="icon-button" onClick={() => setShowAdvanced(!showAdvanced)}>
            <FileSliders size={16} />
          </button>
        </Tooltip>
      </div>

      {showAdvanced ? (
        /* ── Advanced YAML editor ── */
        <>
          <textarea
            value={yamlText}
            onChange={e => setYamlText(e.target.value)}
            spellCheck={false}
          />
          <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
            <button className="save-config-btn" onClick={handleSave} disabled={saving}>
              {saving ? 'Saving...' : 'Save Config'}
            </button>
            {saveStatus && <div className={`config-toast ${saveStatus.type}`} style={{ margin: 0 }}>{saveStatus.msg}</div>}
          </div>
        </>
      ) : (
        /* ── Structured settings form ── */
        <div className="config-structured-form">
          {/* Runtime settings */}
          <div className="config-section-card">
            <h4><Settings size={16} /> Runtime</h4>
            <div className="config-field-group">
              <label className="config-field">
                <span className="config-field-label">Mode</span>
                <select value={runtimeMode} onChange={e => setRuntimeMode(e.target.value)}>
                  {modeOptions.map(m => <option key={m.value} value={m.value}>{m.label}</option>)}
                </select>
                <span className="config-field-hint">{modeOptions.find(m => m.value === runtimeMode)?.desc}</span>
              </label>
              <label className="config-field">
                <span className="config-field-label">Log level</span>
                <select value={logLevel} onChange={e => setLogLevel(e.target.value)}>
                  {logOptions.map(l => <option key={l} value={l}>{l.toUpperCase()}</option>)}
                </select>
              </label>
            </div>
            <div className="config-field-group">
              <label className="config-field config-field-readonly">
                <span className="config-field-label">Host</span>
                <input type="text" value={host} readOnly />
                <span className="config-field-hint">Read-only — set via environment variable</span>
              </label>
              <label className="config-field config-field-readonly">
                <span className="config-field-label">Port</span>
                <input type="number" value={port} readOnly />
                <span className="config-field-hint">Read-only — set via environment variable</span>
              </label>
            </div>
          </div>

          {/* Telemetry settings */}
          <div className="config-section-card">
            <h4><Database size={16} /> Telemetry</h4>
            <div className="config-field-group">
              <label className="config-toggle-row">
                <span className="config-toggle-label">
                  <strong>Log prompts</strong>
                  <small>Record prompt content in telemetry logs</small>
                </span>
                <label className="toggle-switch">
                  <input type="checkbox" checked={logPrompts} onChange={e => setLogPrompts(e.target.checked)} />
                  <span className="toggle-slider" />
                </label>
              </label>
              <label className="config-toggle-row">
                <span className="config-toggle-label">
                  <strong>Log responses</strong>
                  <small>Record response content in telemetry logs</small>
                </span>
                <label className="toggle-switch">
                  <input type="checkbox" checked={logResponses} onChange={e => setLogResponses(e.target.checked)} />
                  <span className="toggle-slider" />
                </label>
              </label>
              <label className="config-toggle-row">
                <span className="config-toggle-label">
                  <strong>Local telemetry</strong>
                  <small>Store telemetry data locally for dashboard display</small>
                </span>
                <label className="toggle-switch">
                  <input type="checkbox" checked={localTelemetry} onChange={e => setLocalTelemetry(e.target.checked)} />
                  <span className="toggle-slider" />
                </label>
              </label>
            </div>
          </div>

          <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
            <button className="save-config-btn" onClick={handleSave} disabled={saving}>
              {saving ? 'Saving...' : 'Save Config'}
            </button>
            {saveStatus && <div className={`config-toast ${saveStatus.type}`} style={{ margin: 0 }}>{saveStatus.msg}</div>}
          </div>
        </div>
      )}
    </div>

    <div className="config-sidebar">
      <div className="config-sidebar-card">
        <h4>Quick reference</h4>
        <p style={{ fontSize: 12, color: 'var(--muted)', lineHeight: 1.5 }}>
          Configure providers, models, and profiles from their dedicated pages. Environment variables override file config:
        </p>
        <code style={{ display: 'block', fontSize: 11, background: 'var(--code-bg)', padding: 8, borderRadius: 4, marginTop: 8, lineHeight: 1.6 }}>
          GUMI_PROVIDER_DEFAULT=lmstudio{'\n'}
          GUMI_LMSTUDIO_URL=http://192.168.0.164:1234/v1{'\n'}
          GUMI_DEFAULT_MODEL=local:auto
        </code>
      </div>
      <div className="config-sidebar-card">
        <h4>Current config (JSON)</h4>
        <pre className="config-code config-code-compact">{JSON.stringify(config ?? {}, null, 2)}</pre>
      </div>
    </div>
  </div>
}

/* ═══════════════════════════════════════════════
   DOCTOR
   ═══════════════════════════════════════════════ */
function DoctorView({ doctor }: { doctor: Doctor | null }) {
  return <section>
    <div className="doctor-hero">
      <div className={`doctor-glyph ${doctor?.status ?? 'warning'}`}>{doctor?.status === 'ok' ? <CheckCircle2 /> : <HeartPulse />}</div>
      <div><p className="eyebrow">System diagnostics</p><h2>{doctor?.status === 'ok' ? 'Runtime checks passed' : 'Runtime needs attention'}</h2></div>
    </div>
    <div className="check-list">
      {doctor?.checks.map((c) => (
        <div key={c.name}>
          <StatusMark status={c.status} />
          <div>
            <strong>{c.name.replaceAll('_', ' ')}</strong>
            <p>{c.message}</p>
            {c.suggestion && <small>{c.suggestion}</small>}
          </div>
        </div>
      ))}
    </div>
  </section>
}

export default App
