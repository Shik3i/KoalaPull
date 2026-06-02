import { useRef, useState, useEffect, useCallback, useMemo, Component } from 'react'
import {
  CheckDependencies, DownloadDependencies,
  FetchMetadata, StartDownload, CancelDownload,
  GetSettings, UpdateSettings, SelectDirectory,
  GetYtdlpVersion, GetVersionInfo, GetHistory,
  ClearHistory, DeleteHistoryEntry,
  UpdateDependencies, OpenOutputDir, CheckForUpdates,
} from "../wailsjs/go/main/App"
import { EventsOn, EventsOff, ClipboardGetText } from "../wailsjs/runtime/runtime"
import type { main } from "../wailsjs/go/models"
import './style.css'

interface FormatInfo {
  formatId: string; ext: string; width: number; height: number
  vcodec: string; acodec: string; filesize: number; formatNote: string
}

interface VideoMetadata {
  id: string; title: string; thumbnail: string; uploader: string
  duration: number; formats: FormatInfo[]; isPlaylist: boolean; entryCount: number
}

interface QueueItem {
  id: string; title: string; thumbnail: string; status: string
  progress: number; speed: string; eta: string; fileSize: string; errorMsg: string; playlistStatus: string
}

interface DepProgress {
  dependency: string; progress: number; status: string; error?: string
}

interface DownloadProgress {
  downloadId: string; percent: number; speed: string; eta: string; fileSize: string
  status: string; error?: string; playlistStatus?: string
}

interface FormatOption { label: string; formatId: string }
interface AppSettings { defaultOutputDir: string; theme: string; maxConcurrency: number; autoPasteURL: boolean }
interface VersionInfo { ytdlp: string; ffmpeg: string; app: string }

type Tab = 'downloads' | 'history' | 'settings'

const parseBytesRe = /^([\d.]+)\s*([KMG]i?B?b?)?/

function buildFormatOptions(formats: FormatInfo[]): FormatOption[] {
  const options: FormatOption[] = [
    { label: 'Best Video + Audio', formatId: 'bestvideo+bestaudio/best' },
  ]
  const seen = new Set<number>()
  const sorted = [...formats]
    .filter((f) => f.vcodec !== 'none' && f.vcodec !== '' && f.height > 0)
    .sort((a, b) => b.height - a.height)
  for (const f of sorted) {
    if (!seen.has(f.height)) {
      seen.add(f.height)
      const note = f.formatNote ? ` (${f.formatNote})` : ''
      options.push({ label: `${f.height}p${note} · ${f.ext}`, formatId: f.formatId })
    }
  }
  options.push({ label: 'Audio Only', formatId: 'bestaudio/best' })
  return options
}

function HistoryEntries({ entries, search, onDelete, fmtTime }: { entries: main.HistoryEntry[]; search: string; onDelete: (id: string) => void; fmtTime: (t: string) => string }) {
  const filtered = search
    ? entries.filter((e) => {
        const q = search.toLowerCase()
        return (e.title || '').toLowerCase().includes(q) || e.url.toLowerCase().includes(q)
      })
    : entries
  if (filtered.length === 0 && search) {
    return (
      <div className="flex flex-col items-center justify-center py-12" style={{ color: 'var(--text-muted)' }}>
        <p className="text-sm">No results for &quot;{search}&quot;</p>
      </div>
    )
  }
  return (
    <div className="space-y-2">
      {filtered.map((entry) => (
        <HistoryRow key={entry.downloadId} entry={entry} onDelete={onDelete} fmtTime={fmtTime} />
      ))}
    </div>
  )
}

function HistoryRow({ entry, onDelete, fmtTime }: { entry: main.HistoryEntry; onDelete: (id: string) => void; fmtTime: (t: string) => string }) {
  return (
    <div className="rounded-lg p-3.5 flex items-center gap-3" style={{ background: 'var(--color-surface-light)', border: '1px solid var(--color-surface-border)' }}>
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium truncate">{entry.title || 'Untitled'}</p>
        <div className="flex flex-wrap gap-x-4 gap-y-1 mt-1.5 text-xs" style={{ color: 'var(--text-secondary)' }}>
          <span>Started: {fmtTime(entry.startTime)}</span>
          <span>Ended: {fmtTime(entry.endTime)}</span>
          {entry.fileSize && <span>Size: {entry.fileSize}</span>}
          {entry.avgSpeed && <span>Speed: {entry.avgSpeed}</span>}
          <span className={`font-medium ${entry.status === 'completed' ? 'text-green-400' : entry.status === 'cancelled' ? 'text-yellow-400' : 'text-red-400'}`}>
            {entry.status.charAt(0).toUpperCase() + entry.status.slice(1)}
          </span>
        </div>
        {entry.errorMsg && (
          <p className="mt-1 text-xs truncate" style={{ color: '#f87171' }}>{entry.errorMsg}</p>
        )}
      </div>
      <button
        onClick={() => onDelete(entry.downloadId)}
        className="shrink-0 transition-colors p-1 rounded"
        style={{ color: 'var(--text-muted)' }}
        title="Delete entry"
      >
        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
        </svg>
      </button>
    </div>
  )
}

export class ErrorBoundary extends Component<{ children: React.ReactNode }, { hasError: boolean }> {
  state = { hasError: false }
  static getDerivedStateFromError() { return { hasError: true } }
  render() {
    if (this.state.hasError) {
      return (
        <div className="h-screen flex flex-col items-center justify-center px-6" style={{ background: 'var(--color-surface)', color: 'var(--text-primary)' }}>
          <h2 className="text-lg font-semibold mb-2">Something went wrong</h2>
          <p className="text-sm mb-4" style={{ color: 'var(--text-muted)' }}>Please restart the application.</p>
          <button onClick={() => { this.setState({ hasError: false }); window.location.reload() }} className="btn-primary text-sm px-4 py-2">Retry</button>
        </div>
      )
    }
    return this.props.children
  }
}

function App() {
  const urlInputRef = useRef<HTMLInputElement>(null)
  const [activeTab, setActiveTab] = useState<Tab>('downloads')
  const [url, setUrl] = useState('')
  const [fetched, setFetched] = useState(false)
  const [fetching, setFetching] = useState(false)
  const [fetchError, setFetchError] = useState('')
  const [metadata, setMetadata] = useState<VideoMetadata | null>(null)
  const [selectedFormat, setSelectedFormat] = useState('bestvideo+bestaudio/best')
  const [formatOptions, setFormatOptions] = useState<FormatOption[]>([])
  const [selectedContainer, setSelectedContainer] = useState('mp4')
  const [selectedSubs, setSelectedSubs] = useState('none')

  const [depsReady, setDepsReady] = useState(false)
  const [checkingDeps, setCheckingDeps] = useState(true)
  const [installingDeps, setInstallingDeps] = useState(false)
  const [depProgress, setDepProgress] = useState<Record<string, number>>({})
  const [depError, setDepError] = useState('')
  const [ytdlpVersion, setYtdlpVersion] = useState('')

  const [queue, setQueue] = useState<QueueItem[]>([])

  const [defaultOutputDir, setDefaultOutputDir] = useState('')
  const [theme, setTheme] = useState('dark')
  const [maxConcurrency, setMaxConcurrency] = useState(3)
  const [autoPasteEnabled, setAutoPasteEnabled] = useState(false)

  const [history, setHistory] = useState<main.HistoryEntry[]>([])
  const [historySearch, setHistorySearch] = useState('')
  const [historyLoading, setHistoryLoading] = useState(false)
  const [versionInfo, setVersionInfo] = useState<VersionInfo | null>(null)
  const [updatingDeps, setUpdatingDeps] = useState(false)
  const [updatesError, setUpdatesError] = useState('')
  const [updateInfo, setUpdateInfo] = useState<main.UpdateInfo | null>(null)

  const activeCount = queue.filter((i) => ['queued', 'starting', 'downloading'].includes(i.status)).length

  const totalEta = useMemo(() => {
    let rem = 0, spd = 0
    for (const item of queue) {
      if (item.status !== 'downloading') continue
      const sz = parseBytes(item.fileSize)
      const s = parseSpeed(item.speed)
      if (sz > 0 && s > 0) { rem += sz * (1 - item.progress / 100); spd += s }
    }
    return spd > 0 ? formatTotalEta(rem / spd) : ''
  }, [queue])

  const reversedQueue = useMemo(() => queue.slice().reverse(), [queue])

  const tabs: { id: Tab; label: string; icon: string }[] = [
    { id: 'downloads', label: 'Downloads', icon: '⬇' },
    { id: 'history', label: 'History', icon: '⏱' },
    { id: 'settings', label: 'Settings', icon: '⚙' },
  ]

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme)
  }, [theme])

  useEffect(() => {
    GetSettings()
      .then((s: AppSettings) => {
        setDefaultOutputDir(s.defaultOutputDir)
        setTheme(s.theme || 'dark')
        setMaxConcurrency(s.maxConcurrency || 3)
        setAutoPasteEnabled(!!s.autoPasteURL)
      })
      .catch((err) => { console.warn('GetSettings failed:', err) })
  }, [])

  useEffect(() => {
    let cancelled = false
    const run = async () => {
      try {
        const status = await CheckDependencies()
        if (cancelled) return
        if (status.ytDlpInstalled && status.ffmpegInstalled) {
          setDepsReady(true)
          setCheckingDeps(false)
        } else {
          setDepsReady(false)
          setCheckingDeps(false)
        }
      } catch {
        if (!cancelled) { setCheckingDeps(false); setDepError('Failed to check dependencies.') }
      }
    }
    const handleDepProgress = (data: DepProgress) => {
      setDepProgress((prev) => ({ ...prev, [data.dependency]: data.progress }))
      if (data.status === 'error') { setDepError(data.error || 'Download failed'); setInstallingDeps(false) }
    }
    EventsOn('dependency-progress', handleDepProgress)
    run()
    return () => { cancelled = true; EventsOff('dependency-progress') }
  }, [])

  useEffect(() => {
    if (!installingDeps || depsReady) return
    let cancelled = false
    DownloadDependencies()
      .then(() => { if (!cancelled) { setDepsReady(true); setInstallingDeps(false) } })
      .catch((err: any) => { if (!cancelled) { setDepError(err?.message || 'Dependency installation failed'); setInstallingDeps(false) } })
    return () => { cancelled = true }
  }, [installingDeps, depsReady])

  useEffect(() => {
    if (!depsReady) return
    GetYtdlpVersion().then((v: string) => { if (v) setYtdlpVersion(v) }).catch(() => {})
  }, [depsReady])

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && (e.key === 'k' || e.key === 'l')) {
        e.preventDefault()
        urlInputRef.current?.focus()
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [])

  useEffect(() => {
    if (!depsReady || !autoPasteEnabled || activeTab !== 'downloads') return
    const checkClipboard = async () => {
      try {
        const text = await ClipboardGetText()
        if (!text) return
        const trimmed = text.trim()
        const ytRegex = /^(https?:\/\/)?(www\.)?(youtube\.com|youtu\.be)\/.+/
        if (ytRegex.test(trimmed)) {
          setUrl(trimmed)
        }
      } catch { /* clipboard errors are benign */ }
    }
    checkClipboard()
  }, [depsReady, autoPasteEnabled, activeTab])

  useEffect(() => {
    const handleDlProgress = (data: DownloadProgress) => {
      setQueue((prev) =>
        prev.map((item) => {
          if (item.id !== data.downloadId) return item
          if (data.status === 'completed') return { ...item, status: 'completed', progress: 100, playlistStatus: '' }
          if (data.status === 'error') {
            const msg = data.error || 'Download failed'
            return { ...item, status: 'error', progress: 0, speed: '', eta: '', fileSize: '', errorMsg: msg, playlistStatus: '' }
          }
          if (data.status === 'cancelled') return { ...item, status: 'cancelled', progress: 0, speed: '', eta: '', fileSize: '', playlistStatus: '' }
          if (data.status === 'starting') return { ...item, status: 'starting', progress: 0, playlistStatus: data.playlistStatus || '' }
          return {
            ...item, status: data.status,
            progress: Math.round(data.percent), speed: data.speed, eta: data.eta, fileSize: data.fileSize || item.fileSize,
            playlistStatus: data.playlistStatus || '',
          }
        }),
      )
    }
    EventsOn('download-progress', handleDlProgress)
    return () => { EventsOff('download-progress') }
  }, [])

  const handleFetch = async () => {
    if (!url.trim()) return
    setFetching(true)
    setFetchError('')
    setFetched(false)
    setMetadata(null)
    setSelectedFormat('bestvideo+bestaudio/best')
    try {
      const meta = await FetchMetadata(url)
      setMetadata(meta)
      setFormatOptions(buildFormatOptions(meta.formats))
      setFetched(true)
    } catch (err: any) {
      setFetchError(err?.message || 'Failed to fetch metadata')
    } finally {
      setFetching(false)
    }
  }

  const handleAddToQueue = async () => {
    if (!metadata) return
    try {
      const downloadId = await StartDownload(url, selectedFormat, defaultOutputDir, selectedContainer, selectedSubs, metadata.title)
      setQueue((prev) => [
        ...prev,
        { id: downloadId, title: metadata.title, thumbnail: metadata.thumbnail, status: 'queued', progress: 0, speed: '', eta: '', fileSize: '', errorMsg: '', playlistStatus: '' },
      ])
    } catch (err: any) {
      console.error('Failed to start download:', err)
    }
  }

  const handleCancel = (id: string) => { CancelDownload(id).catch((err) => { console.warn('CancelDownload failed:', err) }) }

  const handleClearCompleted = () => {
    setQueue((prev) => prev.filter((item) => !['completed', 'error', 'cancelled'].includes(item.status)))
  }

  const handleChangeFolder = async () => {
    try {
      const dir = await SelectDirectory()
      if (!dir) return
      setDefaultOutputDir(dir)
      await UpdateSettings({ defaultOutputDir: dir, theme, maxConcurrency, autoPasteURL: autoPasteEnabled })
    } catch { /* user cancelled */ }
  }

  const loadHistory = async () => {
    setHistoryLoading(true)
    let cancelled = false
    try {
      const h = await GetHistory()
      if (cancelled) return
      setHistory(h)
    } catch (err) { console.warn('loadHistory failed:', err) }
    if (!cancelled) setHistoryLoading(false)
  }

  const handleClearHistory = async () => {
    try { await ClearHistory(); setHistory([]) } catch (err) { console.warn('ClearHistory failed:', err) }
  }

  const handleDeleteHistoryEntry = async (id: string) => {
    try { await DeleteHistoryEntry(id); setHistory((prev) => prev.filter((e) => e.downloadId !== id)) } catch (err) { console.warn('DeleteHistoryEntry failed:', err) }
  }

  const handleThemeChange = async (newTheme: string) => {
    setTheme(newTheme)
    try {
      await UpdateSettings({ defaultOutputDir, theme: newTheme, maxConcurrency, autoPasteURL: autoPasteEnabled })
    } catch (err) { console.warn('handleThemeChange failed:', err) }
  }

  const handleTabSwitch = (tab: Tab) => {
    setActiveTab(tab)
    if (tab === 'history') loadHistory()
    if (tab === 'settings') {
      GetVersionInfo().then((v: VersionInfo) => setVersionInfo(v)).catch((err) => { console.warn('GetVersionInfo failed:', err) })
      CheckForUpdates().then((u) => setUpdateInfo(u)).catch((err) => { console.warn('CheckForUpdates failed:', err) })
    }
  }

  const statusColors: Record<string, string> = {
    downloading: 'text-accent', starting: 'text-accent',
    queued: 'text-gray-400', completed: 'text-green-400',
    error: 'text-red-400', cancelled: 'text-yellow-400',
  }

function fmtTime(t: string): string {
  const d = new Date(t)
  return d.toLocaleDateString() + ' ' + d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

function parseBytes(s: string): number {
  if (!s) return 0
  const m = s.match(parseBytesRe)
  if (!m) return 0
  const num = parseFloat(m[1])
  const unit = (m[2] || '').toLowerCase()
  if (unit.startsWith('ki') || unit === 'kb') return num * 1024
  if (unit.startsWith('mi') || unit === 'mb') return num * 1024 * 1024
  if (unit.startsWith('gi') || unit === 'gb') return num * 1024 * 1024 * 1024
  return num
}

function parseSpeed(s: string): number {
  return parseBytes(s.replace('/s', ''))
}

function formatTotalEta(seconds: number): string {
  if (!seconds || seconds < 1) return ''
  if (seconds < 60) return `<${Math.round(seconds)}s`
  if (seconds < 3600) {
    const m = Math.floor(seconds / 60)
    const s = Math.round(seconds % 60)
    return `~${m}:${String(s).padStart(2, '0')}`
  }
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  return `~${h}:${String(m).padStart(2, '0')}:${String(Math.round(seconds % 60)).padStart(2, '0')}`
}

  // --- Loading Screen ---
  if (checkingDeps) {
    return (
      <div className="h-screen flex flex-col items-center justify-center" style={{ background: 'var(--color-surface)', color: 'var(--text-primary)' }}>
        <div className="w-8 h-8 rounded-md flex items-center justify-center mb-4" style={{ background: 'var(--color-accent)' }}>
          <span className="text-black font-bold text-sm">KP</span>
        </div>
        <h1 className="text-lg font-semibold tracking-tight mb-1">KoalaPull</h1>
        <p style={{ color: 'var(--text-muted)', fontSize: '0.875rem' }}>Checking dependencies...</p>
        <div className="mt-4 w-6 h-6 border-2 rounded-full animate-spin" style={{ borderColor: 'var(--color-accent)', borderTopColor: 'transparent' }} />
      </div>
    )
  }

  // --- Setup Screen ---
  if (!depsReady) {
    return (
      <div className="h-screen flex flex-col items-center justify-center px-6" style={{ background: 'var(--color-surface)', color: 'var(--text-primary)' }}>
        <div className="w-12 h-12 rounded-xl flex items-center justify-center mb-5" style={{ background: 'var(--color-accent)' }}>
          <span className="text-black font-bold text-lg">KP</span>
        </div>
        <h1 className="text-xl font-semibold tracking-tight mb-1">KoalaPull Setup</h1>
        <p className="text-sm mb-6 text-center max-w-sm" style={{ color: 'var(--text-secondary)' }}>
          Required tools (yt-dlp, ffmpeg) need to be downloaded first.
        </p>
        <div className="w-full max-w-sm space-y-4">
          {['yt-dlp', 'ffmpeg'].map((dep) => {
            const pct = depProgress[dep] ?? 0
            return (
              <div key={dep}>
                <div className="flex items-center justify-between text-xs mb-1">
                  <span style={{ color: 'var(--text-secondary)' }}>{dep}</span>
                  <span className="font-mono" style={{ color: 'var(--text-muted)' }}>{pct}%</span>
                </div>
                <div className="w-full h-1.5 rounded-full overflow-hidden" style={{ background: 'var(--color-surface-lighter)' }}>
                  <div className="h-full rounded-full transition-all duration-300 ease-out" style={{ width: `${pct}%`, background: 'var(--color-accent)' }} />
                </div>
              </div>
            )
          })}
        </div>
        {installingDeps ? (
          <p className="text-xs mt-4 font-mono" style={{ color: 'var(--text-muted)' }}>
            {depProgress['yt-dlp'] === 100 && depProgress['ffmpeg'] === 100 ? 'Finalizing...' : 'Downloading...'}
          </p>
        ) : !depError ? (
          <button onClick={() => setInstallingDeps(true)} className="btn-primary text-sm px-5 py-2 mt-2">
            Download &amp; Install
          </button>
        ) : null}
        {depError && (
          <div className="mt-4 text-center">
            <p className="text-xs mb-2" style={{ color: '#f87171' }}>{depError}</p>
            <button onClick={() => setInstallingDeps(true)} className="btn-primary text-xs px-4 py-1.5">Retry</button>
          </div>
        )}
      </div>
    )
  }

  // --- Main App ---
  return (
    <div className="h-screen flex" style={{ background: 'var(--color-surface)', color: 'var(--text-primary)' }}>
      {/* Sidebar */}
      <aside className="w-52 shrink-0 flex flex-col border-r" style={{ background: 'var(--color-surface-light)', borderColor: 'var(--color-surface-border)' }}>
        <div className="flex items-center gap-2.5 px-5 py-4 border-b" style={{ borderColor: 'var(--color-surface-border)' }}>
          <div className="w-7 h-7 rounded-md flex items-center justify-center" style={{ background: 'var(--color-accent)' }}>
            <span className="text-black font-bold text-xs">KP</span>
          </div>
          <span className="font-semibold text-sm">KoalaPull</span>
        </div>
        <nav className="flex-1 px-3 py-4 space-y-1" style={{ color: 'var(--text-secondary)' }}>
          {tabs.map((t) => (
            <div key={t.id} onClick={() => handleTabSwitch(t.id)} className={`sidebar-tab ${activeTab === t.id ? 'active' : ''}`}>
              <span className="text-base">{t.icon}</span>
              <span className="flex-1">{t.label}</span>
              {t.id === 'downloads' && activeCount > 0 && (
                <span className="text-xs font-medium px-1.5 py-0.5 rounded-full" style={{ background: 'var(--color-accent)', color: '#000' }}>
                  {activeCount}
                </span>
              )}
              {t.id === 'settings' && updateInfo?.ytdlpUpdateAvailable && (
                <span className="text-xs font-medium px-1.5 py-0.5 rounded-full" style={{ background: '#fbbf24', color: '#000' }}>
                  1
                </span>
              )}
            </div>
          ))}
        </nav>
        <div className="px-5 py-3 border-t text-xs" style={{ borderColor: 'var(--color-surface-border)', color: 'var(--text-muted)' }}>
          <div className="flex items-center gap-1.5 mb-1">
            <span className="inline-block w-1.5 h-1.5 rounded-full" style={{ background: '#22c55e' }} />
            <span>yt-dlp {ytdlpVersion ? `v${ytdlpVersion}` : ''}</span>
          </div>
          <div style={{ color: 'var(--text-muted)' }}>Save: {defaultOutputDir ? defaultOutputDir.split('/').pop() : '...'}</div>
        </div>
      </aside>

      {/* Main Content */}
      <main className="flex-1 flex flex-col overflow-hidden">
        {/* --- Downloads Tab --- */}
        {activeTab === 'downloads' && (
          <div className="flex-1 flex flex-col overflow-hidden">
            <div className="px-6 py-4 border-b shrink-0" style={{ borderColor: 'var(--color-surface-border)' }}>
              <div className="flex gap-2">
                <div className="flex-1 relative">
                  <input
                    type="text" value={url}
                    onChange={(e) => setUrl(e.target.value)}
                    onKeyDown={(e) => { if (e.key === 'Enter') handleFetch() }}
                    placeholder="Paste a video or playlist URL..."
                    className="input-dark w-full pr-10"
                  />
                  {url && (
                    <button
                      onClick={() => setUrl('')}
                      className="absolute right-3 top-1/2 -translate-y-1/2 text-lg leading-none"
                      style={{ color: 'var(--text-muted)' }}
                    >&times;</button>
                  )}
                </div>
                <button onClick={handleFetch} disabled={fetching || !url.trim()} className="btn-primary shrink-0 flex items-center gap-2">
                  {fetching ? (
                    <><svg className="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
                      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                    </svg> Fetching</>
                  ) : 'Fetch'}
                </button>
              </div>
              {fetchError && (
                <div className="mt-3 px-4 py-3 rounded-lg text-sm" style={{ background: 'rgba(239,68,68,0.15)', border: '1px solid rgba(239,68,68,0.3)', color: '#fca5a5' }}>
                  {fetchError}
                </div>
              )}
            </div>

            <div className="flex-1 overflow-y-auto px-6 py-4 space-y-4">
              {fetched && metadata && (
                <div className="rounded-lg overflow-hidden" style={{ background: 'var(--color-surface-light)', border: '1px solid var(--color-surface-border)' }}>
                  <div className="flex gap-4 p-4">
                    {metadata.thumbnail ? (
                      <img src={metadata.thumbnail} alt={metadata.title} loading="lazy" className="w-44 h-24 rounded-md object-cover shrink-0" style={{ background: 'var(--color-surface-lighter)' }} />
                    ) : (
                      <div className="w-44 h-24 rounded-md shrink-0 flex items-center justify-center" style={{ background: 'var(--color-surface-lighter)', border: '1px solid var(--color-surface-border)' }}>
                        <svg className="w-8 h-8" style={{ color: 'var(--text-muted)' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M15 10l4.553-2.276A1 1 0 0121 8.618v6.764a1 1 0 01-1.447.894L15 14M5 18h8a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z" />
                        </svg>
                      </div>
                    )}
                    <div className="flex-1 min-w-0">
                      <h2 className="text-base font-semibold truncate">{metadata.title}</h2>
                      <p className="text-sm mt-0.5" style={{ color: 'var(--text-secondary)' }}>{metadata.uploader}</p>
                      {metadata.isPlaylist && (
                        <span className="inline-block mt-1 text-xs px-2 py-0.5 rounded font-medium" style={{ background: 'color-mix(in srgb, var(--color-accent) 20%, transparent)', color: 'var(--color-accent)' }}>
                          Playlist · {metadata.entryCount} videos
                        </span>
                      )}
                      <div className="flex flex-wrap gap-2 mt-3">
                        <select value={selectedFormat} onChange={(e) => setSelectedFormat(e.target.value)} className="select-dark text-xs flex-1 min-w-[140px]">
                          {formatOptions.map((opt) => (
                            <option key={opt.formatId} value={opt.formatId}>{opt.label}</option>
                          ))}
                        </select>
                        <select value={selectedSubs} onChange={(e) => setSelectedSubs(e.target.value)} className="select-dark text-xs flex-1 min-w-[100px]">
                          <option value="none">No Subs</option>
                          <option value="auto">Auto-generated</option>
                          <option value="embed">Embed All</option>
                        </select>
                        <select value={selectedContainer} onChange={(e) => setSelectedContainer(e.target.value)} className="select-dark text-xs flex-1 min-w-[80px]">
                          <option value="mp4">MP4</option>
                          <option value="mkv">MKV</option>
                          <option value="mp3">MP3</option>
                        </select>
                      </div>
                      <button onClick={handleAddToQueue} className="btn-primary mt-3 text-sm flex items-center gap-1.5">
                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
                        </svg>
                        Add to Queue
                      </button>
                    </div>
                  </div>
                </div>
              )}

              <div>
                <div className="flex items-center justify-between mb-3">
                  <h3 className="text-sm font-medium uppercase tracking-wider" style={{ color: 'var(--text-secondary)' }}>
                    Downloads ({queue.length})
                    {activeCount > 0 && (
                      <span className="ml-2 font-normal normal-case" style={{ color: 'var(--text-muted)', fontSize: '0.75rem' }}>
                        {activeCount} active{totalEta ? ` · ${totalEta}` : ''}
                      </span>
                    )}
                  </h3>
                  {queue.some((i) => ['completed', 'error', 'cancelled'].includes(i.status)) && (
                    <button onClick={handleClearCompleted} className="text-xs transition-colors" style={{ color: 'var(--text-muted)' }}>Clear Completed</button>
                  )}
                </div>
                {queue.length === 0 ? (
                  <div className="flex flex-col items-center justify-center py-12" style={{ color: 'var(--text-muted)' }}>
                    <svg className="w-12 h-12 mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M7 16V4m0 0L3 8m4-4l4 4m6 0v12m0 0l4-4m-4 4l-4-4" />
                    </svg>
                    <p className="text-sm">No downloads yet</p>
                    <p className="text-xs mt-1">Paste a URL above to get started</p>
                  </div>
                ) : (
                  <div className="space-y-2">
                    {reversedQueue.map((item) => (
                      <div key={item.id} className="rounded-lg p-3 flex items-center gap-3" style={{ background: 'var(--color-surface-light)', border: '1px solid var(--color-surface-border)' }}>
                        <div className="w-16 h-10 rounded shrink-0 flex items-center justify-center overflow-hidden" style={{ background: 'var(--color-surface-lighter)', border: '1px solid var(--color-surface-border)' }}>
                          {item.thumbnail ? (
                            <img src={item.thumbnail} alt="" loading="lazy" className="w-full h-full object-cover" />
                          ) : (
                            <svg className="w-5 h-5" style={{ color: 'var(--text-muted)' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                            </svg>
                          )}
                        </div>
                        <div className="flex-1 min-w-0">
                          <p className="text-sm font-medium truncate">{item.title}</p>
                          <div className="flex items-center gap-3 mt-1 text-xs">
                            <span className={statusColors[item.status]}>
                              {item.status === 'downloading' && `Downloading - ${item.progress}%`}
                              {item.status === 'starting' && 'Starting...'}
                              {item.status === 'queued' && 'Queued'}
                              {item.status === 'completed' && 'Completed'}
                              {item.status === 'error' && 'Error'}
                              {item.status === 'cancelled' && 'Cancelled'}
                            </span>
                            {item.speed && <span style={{ color: 'var(--text-muted)' }}>{item.speed}</span>}
                            {item.eta && <span style={{ color: 'var(--text-muted)' }}>ETA: {item.eta}</span>}
                            {item.playlistStatus && <span style={{ color: 'var(--text-muted)' }}>{item.playlistStatus}</span>}
                          </div>
                          {(item.status === 'downloading' || item.status === 'starting') && (
                            <div className="mt-1.5 w-full h-1 rounded-full overflow-hidden" style={{ background: 'var(--color-surface-lighter)' }}>
                              <div className="h-full rounded-full transition-all duration-300 ease-out" style={{ width: `${Math.max(item.progress, 2)}%`, background: 'var(--color-accent)' }} />
                            </div>
                          )}
                          {item.status === 'completed' && (
                            <div className="mt-1.5 w-full h-1 rounded-full overflow-hidden" style={{ background: 'var(--color-surface-lighter)' }}>
                              <div className="h-full rounded-full" style={{ width: '100%', background: '#22c55e' }} />
                            </div>
                          )}
                          {item.status === 'error' && (
                            <>
                              <div className="mt-1.5 w-full h-1 rounded-full overflow-hidden" style={{ background: 'var(--color-surface-lighter)' }}>
                                <div className="h-full rounded-full" style={{ width: '100%', background: '#ef4444' }} />
                              </div>
                              {item.errorMsg && <p className="mt-1 text-xs truncate" style={{ color: '#f87171' }}>{item.errorMsg}</p>}
                            </>
                          )}
                          {item.status === 'cancelled' && (
                            <div className="mt-1.5 w-full h-1 rounded-full overflow-hidden" style={{ background: 'var(--color-surface-lighter)' }}>
                              <div className="h-full rounded-full" style={{ width: '100%', background: '#eab308' }} />
                            </div>
                          )}
                        </div>
                        {item.status === 'completed' && (
                          <div className="flex items-center gap-1.5">
                            <button onClick={() => OpenOutputDir().catch((err) => { console.warn('OpenOutputDir failed:', err) })} className="transition-colors p-1 rounded" style={{ color: 'var(--text-muted)' }} title="Open output folder">
                              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 19a2 2 0 01-2-2V7a2 2 0 012-2h4l2 2h4a2 2 0 012 2v1M5 19h14a2 2 0 002-2v-5a2 2 0 00-2-2H9a2 2 0 00-2 2v5a2 2 0 01-2 2z" />
                              </svg>
                            </button>
                            <svg className="w-5 h-5 shrink-0" style={{ color: '#22c55e' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                            </svg>
                          </div>
                        )}
                        {(item.status === 'downloading' || item.status === 'starting') && (
                          <div className="flex items-center gap-1">
                            <svg className="w-5 h-5 animate-pulse shrink-0" style={{ color: 'var(--color-accent)' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 14l-7 7m0 0l-7-7m7 7V3" />
                            </svg>
                            <button onClick={() => handleCancel(item.id)} className="transition-colors p-0.5" style={{ color: 'var(--text-muted)' }} title="Cancel">
                              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                              </svg>
                            </button>
                          </div>
                        )}
                        {item.status === 'queued' && (
                          <button onClick={() => handleCancel(item.id)} className="transition-colors p-0.5 shrink-0" style={{ color: 'var(--text-muted)' }} title="Cancel">
                            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                            </svg>
                          </button>
                        )}
                        {item.status === 'error' && (
                          <svg className="w-5 h-5 shrink-0" style={{ color: '#ef4444' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L4.082 16.5c-.77.833.192 2.5 1.732 2.5z" />
                          </svg>
                        )}
                        {item.status === 'cancelled' && (
                          <svg className="w-5 h-5 shrink-0" style={{ color: '#eab308' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                          </svg>
                        )}
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
          </div>
        )}

        {/* --- History Tab --- */}
        {activeTab === 'history' && (
          <div className="flex-1 flex flex-col overflow-hidden">
            <div className="flex items-center justify-between px-6 py-4 border-b shrink-0" style={{ borderColor: 'var(--color-surface-border)' }}>
              <h2 className="text-base font-semibold">Download History</h2>
              <div className="flex items-center gap-3">
                {history.length > 0 && (
                  <>
                    <input
                      type="text" value={historySearch}
                      onChange={(e) => setHistorySearch(e.target.value)}
                      placeholder="Search history..."
                      className="input-dark text-xs w-44"
                    />
                    <button
                      onClick={() => { if (window.confirm('Clear all download history?')) handleClearHistory() }}
                      className="btn-primary text-xs px-3 py-1.5"
                    >Clear All</button>
                  </>
                )}
              </div>
            </div>
            <div className="flex-1 overflow-y-auto px-6 py-4">
              {historyLoading ? (
                <div className="flex flex-col items-center justify-center py-16" style={{ color: 'var(--text-muted)' }}>
                  <div className="w-6 h-6 border-2 rounded-full animate-spin mb-3" style={{ borderColor: 'var(--color-accent)', borderTopColor: 'transparent' }} />
                  <p className="text-sm">Loading history...</p>
                </div>
              ) : history.length === 0 ? (
                <div className="flex flex-col items-center justify-center py-16" style={{ color: 'var(--text-muted)' }}>
                  <svg className="w-12 h-12 mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                  <p className="text-sm">No download history yet</p>
                </div>
              ) : (
                <HistoryEntries entries={history} search={historySearch} onDelete={handleDeleteHistoryEntry} fmtTime={fmtTime} />
              )}
            </div>
          </div>
        )}

        {/* --- Settings Tab --- */}
        {activeTab === 'settings' && (
          <div className="flex-1 flex flex-col overflow-hidden">
            <div className="px-6 py-4 border-b shrink-0" style={{ borderColor: 'var(--color-surface-border)' }}>
              <h2 className="text-base font-semibold">Settings</h2>
            </div>
            <div className="flex-1 overflow-y-auto px-6 py-5 space-y-6 max-w-2xl">
              {/* Theme */}
              <section>
                <h3 className="text-sm font-medium mb-3" style={{ color: 'var(--text-secondary)' }}>Appearance</h3>
                <div className="flex gap-3">
                  {['dark', 'light'].map((t) => (
                    <button
                      key={t}
                      onClick={() => handleThemeChange(t)}
                      className="flex-1 rounded-lg py-3 px-4 text-sm font-medium transition-colors"
                      style={{
                        background: theme === t ? 'color-mix(in srgb, var(--color-accent) 20%, transparent)' : 'var(--color-surface-lighter)',
                        border: theme === t ? '1px solid var(--color-accent)' : '1px solid var(--color-surface-border)',
                        color: theme === t ? 'var(--color-accent)' : 'var(--text-secondary)',
                      }}
                    >
                      {t === 'dark' ? '🌙 Dark' : '☀ Light'}
                    </button>
                  ))}
                </div>
              </section>

              {/* Download Location */}
              <section>
                <h3 className="text-sm font-medium mb-3" style={{ color: 'var(--text-secondary)' }}>Download Location</h3>
                <div className="flex gap-2">
                  <div className="flex-1 rounded-md px-3 py-2 text-xs truncate" style={{ background: 'var(--color-surface-lighter)', border: '1px solid var(--color-surface-border)', color: 'var(--text-secondary)' }}>
                    {defaultOutputDir}
                  </div>
                  <button onClick={handleChangeFolder} className="btn-primary text-xs px-3 py-1.5 shrink-0">Change</button>
                </div>
              </section>

              {/* Max Concurrency */}
              <section>
                <h3 className="text-sm font-medium mb-3" style={{ color: 'var(--text-secondary)' }}>Downloads</h3>
                <div className="flex items-center gap-3">
                  <label className="text-xs" style={{ color: 'var(--text-muted)', minWidth: '7rem' }}>Max parallel downloads</label>
                  <input
                    type="number" min={1} max={10}
                    value={maxConcurrency}
                    onChange={async (e) => {
                      const v = Math.max(1, Math.min(10, parseInt(e.target.value) || 1))
                      setMaxConcurrency(v)
                      try { await UpdateSettings({ defaultOutputDir, theme, maxConcurrency: v, autoPasteURL: autoPasteEnabled }) } catch (err) { console.warn('UpdateSettings failed:', err) }
                    }}
                    className="input-dark text-xs w-16 text-center"
                  />
                </div>
              </section>

              {/* Auto-Paste URL */}
              <section>
                <div className="flex items-center justify-between">
                  <div>
                    <h3 className="text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>Auto-paste URL</h3>
                    <p className="text-xs mt-0.5" style={{ color: 'var(--text-muted)' }}>Automatically paste YouTube URLs from clipboard when opening the Downloads tab</p>
                  </div>
                  <button
                    onClick={async () => {
                      const next = !autoPasteEnabled
                      setAutoPasteEnabled(next)
                      try { await UpdateSettings({ defaultOutputDir, theme, maxConcurrency, autoPasteURL: next }) } catch (err) { console.warn('UpdateSettings failed:', err) }
                    }}
                    className="relative w-10 h-5 rounded-full transition-colors shrink-0"
                    style={{
                      background: autoPasteEnabled ? 'var(--color-accent)' : 'var(--color-surface-border)',
                    }}
                  >
                    <span
                      className="absolute top-0.5 w-4 h-4 rounded-full bg-white transition-transform"
                      style={{ transform: autoPasteEnabled ? 'translateX(20px)' : 'translateX(2px)' }}
                    />
                  </button>
                </div>
              </section>

              {/* Version Info */}
              <section>
                <h3 className="text-sm font-medium mb-3" style={{ color: 'var(--text-secondary)' }}>Versions</h3>
                <div className="space-y-2 text-sm">
                  <div className="flex items-center gap-2">
                    <span className="w-20" style={{ color: 'var(--text-muted)' }}>KoalaPull</span>
                    <a
                      href="https://github.com/Shik3i/KoalaPull"
                      target="_blank"
                      rel="noopener noreferrer"
                      className="font-mono hover:underline inline-flex items-center gap-1"
                      style={{ color: 'var(--color-accent)' }}
                    >
                      <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                        <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
                      </svg>
                      {versionInfo?.app || '-'}
                    </a>
                  </div>
                  <div className="flex items-center gap-2">
                    <span className="w-20" style={{ color: 'var(--text-muted)' }}>yt-dlp</span>
                    <a
                      href="https://github.com/yt-dlp/yt-dlp"
                      target="_blank"
                      rel="noopener noreferrer"
                      className="font-mono hover:underline inline-flex items-center gap-1"
                      style={{ color: 'var(--color-accent)' }}
                    >
                      {versionInfo?.ytdlp || '-'}
                      <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24" style={{ opacity: 0.6 }}>
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                      </svg>
                    </a>
                  </div>
                  <div className="flex items-center gap-2">
                    <span className="w-20" style={{ color: 'var(--text-muted)' }}>ffmpeg</span>
                    <a
                      href={navigator.platform.includes('Mac') ? 'https://evermeet.cx/ffmpeg/' : 'https://github.com/BtbN/FFmpeg-Builds'}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="font-mono hover:underline inline-flex items-center gap-1"
                      style={{ color: 'var(--color-accent)' }}
                    >
                      {versionInfo?.ffmpeg || '-'}
                      <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24" style={{ opacity: 0.6 }}>
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                      </svg>
                    </a>
                  </div>
                </div>
              </section>

              {/* Updates */}
              <section>
                <h3 className="text-sm font-medium mb-3" style={{ color: 'var(--text-secondary)' }}>Updates</h3>
                <div className="text-xs" style={{ color: 'var(--text-muted)' }}>
                  {updateInfo ? (
                    <div className="space-y-2">
                      {updateInfo.ytdlpUpdateAvailable ? (
                        <div className="flex items-center gap-2" style={{ color: '#fbbf24' }}>
                          <span>yt-dlp {updateInfo.latestYtdlpVersion} available</span>
                          <span className="text-xs" style={{ color: 'var(--text-muted)' }}>(current: v{versionInfo?.ytdlp || '?'})</span>
                        </div>
                      ) : (
                        <p>yt-dlp is up to date (v{versionInfo?.ytdlp || '?'})</p>
                      )}
                      <button
                        onClick={async () => {
                          setUpdatingDeps(true)
                          setUpdatesError('')
                          try {
                            await UpdateDependencies()
                            const v = await GetVersionInfo()
                            setVersionInfo(v)
                            if (v.ytdlp) setYtdlpVersion(v.ytdlp)
                            const u = await CheckForUpdates()
                            setUpdateInfo(u)
                          } catch (err: any) {
                            setUpdatesError(err?.message || 'Update failed')
                          }
                          setUpdatingDeps(false)
                        }}
                        disabled={updatingDeps}
                        className="btn-primary text-xs px-4 py-1.5 mt-2"
                      >
                        {updatingDeps ? 'Updating...' : updateInfo.ytdlpUpdateAvailable ? 'Download Update' : 'Re-download'}
                      </button>
                    </div>
                  ) : (
                    <p>Loading...</p>
                  )}
                  {updatesError && (
                    <p className="mt-2" style={{ color: '#f87171' }}>{updatesError}</p>
                  )}
                </div>
              </section>
            </div>
          </div>
        )}
      </main>
    </div>
  )
}

export default App
