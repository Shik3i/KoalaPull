import { useRef, useState, useEffect, useMemo, Component } from 'react'
import {
  CheckDependencies, DownloadDependencies,
  FetchMetadata, StartDownload, CancelDownload,
  GetSettings, UpdateSettings, SelectDirectory,
  GetAppVersion, GetVersionInfo, GetHistory,
  ClearHistory, DeleteHistoryEntry,
  UpdateDependencies, OpenOutputDir, CheckForUpdates, OpenExternalLink,
} from "../wailsjs/go/main/App"
import { EventsOn, ClipboardGetText } from "../wailsjs/runtime/runtime"
import type { main } from "../wailsjs/go/models"
import { formatTotalEta, parseBytes, parseSpeed } from "./lib/downloadMetrics"
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
interface SupportedSite {
  name: string
  blurb: string
  href: string
}

type Tab = 'downloads' | 'history' | 'settings' | 'help'

let historyRequestId = 0

const supportedSites: SupportedSite[] = [
  { name: 'YouTube', blurb: 'Videos, Shorts, playlists', href: 'https://www.youtube.com' },
  { name: 'Vimeo', blurb: 'Videos, showcases, livestreams', href: 'https://vimeo.com' },
  { name: 'Dailymotion', blurb: 'Videos, channels, playlists', href: 'https://www.dailymotion.com' },
  { name: 'Twitch', blurb: 'Streams, VODs, clips', href: 'https://www.twitch.tv' },
  { name: 'TikTok', blurb: 'Videos, profiles, slideshows', href: 'https://www.tiktok.com' },
  { name: 'Twitter (X)', blurb: 'Posts with video, clips', href: 'https://x.com' },
  { name: 'Instagram', blurb: 'Reels, posts, stories', href: 'https://www.instagram.com' },
  { name: 'Facebook', blurb: 'Videos, reels, live clips', href: 'https://www.facebook.com' },
  { name: 'Reddit', blurb: 'Post videos, clips, embeds', href: 'https://www.reddit.com' },
  { name: 'ARD', blurb: 'Shows, documentaries, news', href: 'https://www.ardmediathek.de' },
  { name: 'ZDF', blurb: 'Shows, documentaries, live streams', href: 'https://www.zdf.de' },
  { name: 'Arte', blurb: 'Documentaries, concerts, films', href: 'https://www.arte.tv' },
  { name: '3sat', blurb: 'Culture shows, docs, concerts', href: 'https://www.3sat.de' },
  { name: 'NDR', blurb: 'Shows, reports, regional clips', href: 'https://www.ndr.de' },
  { name: 'BBC', blurb: 'Programmes, news clips, episodes', href: 'https://www.bbc.com' },
  { name: 'TED', blurb: 'Talks, playlists, event videos', href: 'https://www.ted.com' },
  { name: 'CNN', blurb: 'News videos, interviews, clips', href: 'https://www.cnn.com' },
  { name: 'Discovery', blurb: 'Episodes, clips, trailers', href: 'https://www.discovery.com' },
  { name: 'Bilibili', blurb: 'Videos, series, livestreams', href: 'https://www.bilibili.com' },
  { name: 'Niconico', blurb: 'Videos, livestreams, channels', href: 'https://www.nicovideo.jp' },
  { name: 'Rumble', blurb: 'Videos, channels, livestreams', href: 'https://rumble.com' },
  { name: 'Odysee', blurb: 'Videos, channels, creators', href: 'https://odysee.com' },
  { name: 'SoundCloud', blurb: 'Tracks, sets, podcasts', href: 'https://soundcloud.com' },
  { name: 'Bandcamp', blurb: 'Tracks, albums, releases', href: 'https://bandcamp.com' },
]

function formatAppVersionLabel(version: string): string {
  if (!version) return '...'
  if (version === 'dev' || version.startsWith('v')) return version
  return `v${version}`
}

function siteLogoUrl(site: SupportedSite): string {
  if (site.name === 'Niconico') return 'https://www.nicovideo.jp/favicon.ico'
  const hostname = new URL(site.href).hostname
  return `https://www.google.com/s2/favicons?domain=${encodeURIComponent(hostname)}&sz=64`
}

function SiteMark({ site }: { site: SupportedSite }) {
  return (
    <div
      className="w-10 h-10 rounded-xl shrink-0 flex items-center justify-center overflow-hidden"
      style={{ background: 'var(--color-surface-lighter)', border: '1px solid var(--color-surface-border)' }}
    >
      <img src={siteLogoUrl(site)} alt="" className="w-7 h-7 object-contain" loading="lazy" />
    </div>
  )
}

function SiteBadge({ site }: { site: SupportedSite }) {
  return (
    <div
      className="group h-full rounded-2xl p-3.5 border transition-colors"
      style={{ background: 'var(--color-surface-light)', borderColor: 'var(--color-surface-border)' }}
    >
      <div className="flex items-start gap-3">
        <SiteMark site={site} />
        <div className="min-w-0">
          <p className="text-sm font-semibold leading-5">{site.name}</p>
          <p className="text-xs mt-1 leading-5" style={{ color: 'var(--text-muted)' }}>{site.blurb}</p>
        </div>
      </div>
    </div>
  )
}

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
  const fetchRequestIdRef = useRef(0)
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

  const [queue, setQueue] = useState<QueueItem[]>([])

  const [defaultOutputDir, setDefaultOutputDir] = useState('')
  const [theme, setTheme] = useState('dark')
  const [maxConcurrency, setMaxConcurrency] = useState(3)
  const [autoPasteEnabled, setAutoPasteEnabled] = useState(false)

  const [history, setHistory] = useState<main.HistoryEntry[]>([])
  const [historySearch, setHistorySearch] = useState('')
  const [historyLoading, setHistoryLoading] = useState(false)
  const [appVersion, setAppVersion] = useState('')
  const [toolVersions, setToolVersions] = useState<VersionInfo | null>(null)
  const [toolVersionsLoading, setToolVersionsLoading] = useState(true)
  const [updatingDeps, setUpdatingDeps] = useState(false)
  const [updatesError, setUpdatesError] = useState('')
  const [updateInfo, setUpdateInfo] = useState<main.UpdateInfo | null>(null)
  const [updateLoading, setUpdateLoading] = useState(true)

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
    { id: 'help', label: 'Help', icon: '?' },
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

  const loadAppVersion = async () => {
    try {
      const v = await GetAppVersion()
      setAppVersion(v || '')
    } catch (err) {
      console.warn('loadAppVersion failed:', err)
    }
  }

  const loadToolVersions = async () => {
    try {
      const v = await GetVersionInfo()
      setToolVersions(v)
    } catch (err) {
      console.warn('loadToolVersions failed:', err)
    } finally {
      setToolVersionsLoading(false)
    }
  }

  const loadUpdateInfo = async () => {
    try {
      const u = await CheckForUpdates()
      setUpdateInfo(u)
      setUpdatesError('')
    } catch (err) {
      console.warn('loadUpdateInfo failed:', err)
      setUpdatesError('Update check unavailable')
    } finally {
      setUpdateLoading(false)
    }
  }

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
    const offDepProgress = EventsOn('dependency-progress', handleDepProgress)
    run()
    return () => { cancelled = true; offDepProgress() }
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
    void loadAppVersion()
  }, [])

  useEffect(() => {
    if (!depsReady) return
    void loadToolVersions()
    void loadUpdateInfo()
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
      setQueue((prev) => {
        const idx = prev.findIndex((item) => item.id === data.downloadId)
        if (idx === -1) return prev
        const item = prev[idx]
        let nextItem: QueueItem
        if (data.status === 'completed') {
          nextItem = { ...item, status: 'completed', progress: 100, playlistStatus: '' }
        } else if (data.status === 'error') {
          const msg = data.error || 'Download failed'
          nextItem = { ...item, status: 'error', progress: 0, speed: '', eta: '', fileSize: '', errorMsg: msg, playlistStatus: '' }
        } else if (data.status === 'cancelled') {
          nextItem = { ...item, status: 'cancelled', progress: 0, speed: '', eta: '', fileSize: '', playlistStatus: '' }
        } else if (data.status === 'starting') {
          nextItem = { ...item, status: 'starting', progress: 0, playlistStatus: data.playlistStatus || '' }
        } else {
          nextItem = {
            ...item, status: data.status,
            progress: Math.round(data.percent), speed: data.speed, eta: data.eta, fileSize: data.fileSize || item.fileSize,
            playlistStatus: data.playlistStatus || '',
          }
        }
        const next = [...prev]
        next[idx] = nextItem
        return next
      })
    }
    const offDownloadProgress = EventsOn('download-progress', handleDlProgress)
    return () => { offDownloadProgress() }
  }, [])

  const handleFetch = async () => {
    if (!url.trim()) return
    const requestId = ++fetchRequestIdRef.current
    setFetching(true)
    setFetchError('')
    setFetched(false)
    setMetadata(null)
    setSelectedFormat('bestvideo+bestaudio/best')
    try {
      const meta = await FetchMetadata(url)
      if (requestId !== fetchRequestIdRef.current) return
      setMetadata(meta)
      setFormatOptions(buildFormatOptions(meta.formats || []))
      setFetched(true)
    } catch (err: any) {
      if (requestId !== fetchRequestIdRef.current) return
      setFetchError(err?.message || 'Failed to fetch metadata')
    } finally {
      if (requestId === fetchRequestIdRef.current) setFetching(false)
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
    const id = ++historyRequestId
    setHistoryLoading(true)
    try {
      const h = await GetHistory()
      if (id !== historyRequestId) return
      setHistory(h)
    } catch (err) { console.warn('loadHistory failed:', err) }
    if (id === historyRequestId) setHistoryLoading(false)
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
  }

  const statusColors: Record<string, string> = {
    downloading: 'text-accent', starting: 'text-accent',
    queued: 'text-gray-400', completed: 'text-green-400',
    error: 'text-red-400', cancelled: 'text-yellow-400',
  }

function fmtTime(t: string): string {
  const d = new Date(t)
  if (Number.isNaN(d.getTime())) return '-'
  return d.toLocaleDateString() + ' ' + d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
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
          <button onClick={() => { setDepError(''); setDepProgress({}); setInstallingDeps(true) }} className="btn-primary text-sm px-5 py-2 mt-2">
            Download &amp; Install
          </button>
        ) : null}
        {depError && (
          <div className="mt-4 text-center">
            <p className="text-xs mb-2" style={{ color: '#f87171' }}>{depError}</p>
            <button onClick={() => { setDepError(''); setDepProgress({}); setInstallingDeps(true) }} className="btn-primary text-xs px-4 py-1.5">Retry</button>
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
              {t.id === 'settings' && (updateInfo?.ytdlpUpdateAvailable || updateInfo?.koalaPullUpdateAvailable) && (
                <span className="text-xs font-medium px-1.5 py-0.5 rounded-full" style={{ background: '#fbbf24', color: '#000' }}>
                  {(updateInfo?.ytdlpUpdateAvailable ? 1 : 0) + (updateInfo?.koalaPullUpdateAvailable ? 1 : 0)}
                </span>
              )}
            </div>
          ))}
        </nav>
        <div className="px-4 py-3 border-t" style={{ borderColor: 'var(--color-surface-border)' }}>
          <button
            onClick={() => OpenExternalLink('https://github.com/Shik3i/KoalaPull').catch((err) => { console.warn('OpenExternalLink failed:', err) })}
            className="w-full flex items-center gap-2 rounded-lg px-2 py-1.5 transition-colors text-left"
            style={{ color: 'var(--text-muted)' }}
            title="Open KoalaPull on GitHub"
          >
            <svg className="w-4 h-4 shrink-0" fill="currentColor" viewBox="0 0 24 24" aria-hidden="true">
              <path d="M12 0C5.37 0 0 5.37 0 12c0 5.3 3.44 9.8 8.21 11.38.6.11.79-.26.79-.58v-2.23c-3.34.73-4.03-1.41-4.03-1.41-.55-1.39-1.33-1.76-1.33-1.76-1.09-.75.08-.73.08-.73 1.21.08 1.84 1.24 1.84 1.24 1.07 1.83 2.81 1.3 3.49 1 .11-.78.42-1.31.76-1.61-2.66-.3-5.47-1.33-5.47-5.93 0-1.31.47-2.38 1.24-3.22-.12-.3-.53-1.53.12-3.18 0 0 1-.32 3.3 1.23.96-.27 1.98-.4 3-.4s2.05.13 3.01.4c2.29-1.55 3.3-1.23 3.3-1.23.65 1.65.24 2.88.12 3.18.77.84 1.24 1.91 1.24 3.22 0 4.61-2.81 5.62-5.48 5.92.43.37.82 1.1.82 2.22v3.29c0 .32.19.69.8.57C20.56 21.8 24 17.3 24 12c0-6.63-5.37-12-12-12z" />
            </svg>
            <span className="font-mono text-xs">{formatAppVersionLabel(appVersion)}</span>
          </button>
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
                  <div className="flex-1 rounded-md px-3 py-2 text-xs leading-5 break-all whitespace-normal" style={{ background: 'var(--color-surface-lighter)', border: '1px solid var(--color-surface-border)', color: 'var(--text-secondary)' }}>
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
                    aria-label={autoPasteEnabled ? 'Disable auto-paste' : 'Enable auto-paste'}
                  >
                    <span
                      className="absolute top-0.5 w-4 h-4 rounded-full bg-white transition-transform"
                      style={{ left: '2px', transform: autoPasteEnabled ? 'translateX(20px)' : 'translateX(0)' }}
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
                    <button
                      onClick={() => OpenExternalLink('https://github.com/Shik3i/KoalaPull').catch(() => {})}
                      className="font-mono hover:underline inline-flex items-center gap-1"
                      style={{ color: 'var(--color-accent)' }}
                    >
                      <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                        <path d="M12 0C5.37 0 0 5.37 0 12c0 5.3 3.44 9.8 8.21 11.38.6.11.79-.26.79-.58v-2.23c-3.34.73-4.03-1.41-4.03-1.41-.55-1.39-1.33-1.76-1.33-1.76-1.09-.75.08-.73.08-.73 1.21.08 1.84 1.24 1.84 1.24 1.07 1.83 2.81 1.3 3.49 1 .11-.78.42-1.31.76-1.61-2.66-.3-5.47-1.33-5.47-5.93 0-1.31.47-2.38 1.24-3.22-.12-.3-.53-1.53.12-3.18 0 0 1-.32 3.3 1.23.96-.27 1.98-.4 3-.4s2.05.13 3.01.4c2.29-1.55 3.3-1.23 3.3-1.23.65 1.65.24 2.88.12 3.18.77.84 1.24 1.91 1.24 3.22 0 4.61-2.81 5.62-5.48 5.92.43.37.82 1.1.82 2.22v3.29c0 .32.19.69.8.57C20.56 21.8 24 17.3 24 12c0-6.63-5.37-12-12-12z"/>
                      </svg>
                      {appVersion ? formatAppVersionLabel(appVersion) : '-'}
                    </button>
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
                      {toolVersionsLoading ? 'Loading...' : (toolVersions?.ytdlp || 'Unavailable')}
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
                      {toolVersions?.ffmpeg || '-'}
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
                <div className="text-xs space-y-3" style={{ color: 'var(--text-muted)' }}>
                  {updateInfo ? (
                    <>
                      {/* KoalaPull */}
                      <div>
                        {updateInfo.koalaPullUpdateAvailable ? (
                          <div>
                            <span style={{ color: '#fbbf24' }}>KoalaPull {updateInfo.latestKoalaPullVersion} available</span>
                            <span className="ml-2" style={{ color: 'var(--text-muted)' }}>(current: {appVersion ? formatAppVersionLabel(appVersion) : '?'})</span>
                            <button
                              onClick={() => OpenExternalLink('https://github.com/Shik3i/KoalaPull/releases/latest').catch(() => {})}
                              className="btn-primary text-xs px-3 py-1 ml-3"
                            >View Release</button>
                          </div>
                        ) : (
                          <p>KoalaPull is up to date ({appVersion ? formatAppVersionLabel(appVersion) : '?'})</p>
                        )}
                      </div>

                      {/* yt-dlp */}
                      <div>
                      {updateInfo.ytdlpUpdateAvailable ? (
                          <div>
                            <span style={{ color: '#fbbf24' }}>yt-dlp {updateInfo.latestYtdlpVersion} available</span>
                            <span className="ml-2" style={{ color: 'var(--text-muted)' }}>(current: {toolVersions?.ytdlp || '?'})</span>
                          </div>
                        ) : (
                          <p>yt-dlp is up to date ({toolVersions?.ytdlp || '?'})</p>
                        )}
                        <button
                          onClick={async () => {
                            setUpdatingDeps(true)
                            setUpdatesError('')
                            try {
                              await UpdateDependencies()
                              await Promise.all([loadToolVersions(), loadUpdateInfo()])
                            } catch (err: any) {
                              setUpdatesError(err?.message || 'Update failed')
                            } finally {
                              setUpdatingDeps(false)
                            }
                          }}
                          disabled={updatingDeps}
                          className="btn-primary text-xs px-4 py-1.5 mt-2"
                        >
                          {updatingDeps ? 'Updating...' : updateInfo.ytdlpUpdateAvailable ? 'Download Update' : 'Re-download'}
                        </button>
                      </div>
                    </>
                  ) : updateLoading ? (
                    <p>Checking updates...</p>
                  ) : (
                    <p>Update check unavailable</p>
                  )}
                  {updatesError && (
                    <p className="mt-2" style={{ color: '#f87171' }}>{updatesError}</p>
                  )}
                </div>
              </section>
            </div>
          </div>
        )}

        {/* --- Help Tab --- */}
        {activeTab === 'help' && (
          <div className="flex-1 flex flex-col overflow-hidden">
            <div className="px-6 py-4 border-b shrink-0" style={{ borderColor: 'var(--color-surface-border)' }}>
              <h2 className="text-base font-semibold">Help</h2>
            </div>
            <div className="flex-1 overflow-y-auto px-6 py-5">
              <div className="space-y-6 max-w-4xl">
                <section className="rounded-xl p-4 border" style={{ background: 'var(--color-surface-light)', borderColor: 'var(--color-surface-border)' }}>
                  <h3 className="text-sm font-medium mb-2" style={{ color: 'var(--text-secondary)' }}>How to download a video</h3>
                  <div className="grid gap-3 sm:grid-cols-2">
                    {[
                      {
                        step: '1',
                        title: 'Find your video',
                        text: 'Copy the link of the video or playlist you want to download from your browser.',
                      },
                      {
                        step: '2',
                        title: 'Fetch the data',
                        text: 'Paste the link into the KoalaPull search bar and click "Fetch".',
                      },
                      {
                        step: '3',
                        title: 'Choose your format',
                        text: 'Pick the video quality you want, or choose audio only if you just want MP3.',
                      },
                      {
                        step: '4',
                        title: 'Download',
                        text: 'Add it to the queue. KoalaPull handles the rest for you.',
                      },
                    ].map((item) => (
                      <div
                        key={item.step}
                        className="rounded-xl border p-3"
                        style={{ background: 'var(--color-surface)', borderColor: 'var(--color-surface-border)' }}
                      >
                        <div className="flex items-start gap-3">
                          <div
                            className="w-8 h-8 rounded-full flex items-center justify-center text-sm font-semibold shrink-0"
                            style={{ background: 'color-mix(in srgb, var(--color-accent) 18%, transparent)', color: 'var(--color-accent)' }}
                          >
                            {item.step}
                          </div>
                          <div>
                            <p className="text-sm font-semibold">{item.title}</p>
                            <p className="text-xs mt-1 leading-5" style={{ color: 'var(--text-muted)' }}>{item.text}</p>
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                </section>

                <section className="space-y-4">
                  <div className="flex items-end justify-between gap-4">
                    <div>
                      <h3 className="text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>Popular supported sites</h3>
                      <p className="text-xs mt-1 max-w-2xl" style={{ color: 'var(--text-muted)' }}>
                        A curated snapshot of what yt-dlp can handle across video, social, broadcasters, and audio.
                      </p>
                    </div>
                    <button
                      onClick={() => OpenExternalLink('https://github.com/yt-dlp/yt-dlp/blob/master/supportedsites.md').catch((err) => { console.warn('OpenExternalLink failed:', err) })}
                      className="text-xs hover:underline shrink-0"
                      style={{ color: 'var(--color-accent)' }}
                    >
                      View all 1000+ supported sites
                    </button>
                  </div>

                  <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
                    {supportedSites.map((site) => (
                      <button
                        key={site.name}
                        onClick={() => OpenExternalLink(site.href).catch((err) => { console.warn('OpenExternalLink failed:', err) })}
                        className="block text-left"
                        title={`Open ${site.name}`}
                      >
                        <SiteBadge site={site} />
                      </button>
                    ))}
                  </div>
                </section>

                <section className="rounded-xl p-4 border" style={{ background: 'var(--color-surface-light)', borderColor: 'var(--color-surface-border)' }}>
                  <h3 className="text-sm font-medium mb-2" style={{ color: 'var(--text-secondary)' }}>Under the Hood</h3>
                  <div className="space-y-3 text-sm leading-6" style={{ color: 'var(--text-secondary)' }}>
                    <p>
                      KoalaPull is a graphical user interface, or GUI. It gives you buttons, menus, and progress bars so you do not need to type terminal commands.
                    </p>
                    <p>
                      In the background, KoalaPull uses two open-source tools: <span className="font-mono">yt-dlp</span> for downloading media, and <span className="font-mono">FFmpeg</span> for processing, converting, and merging video and audio formats.
                    </p>
                    <p>
                      KoalaPull manages these tools for you automatically, so you get their power without needing to learn or use the command line.
                    </p>
                  </div>
                </section>
              </div>
            </div>
          </div>
        )}
      </main>
    </div>
  )
}

export default App
