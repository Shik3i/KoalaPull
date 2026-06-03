import { useRef, useState, useEffect, useMemo, useCallback, Component, memo } from 'react'
import {
  CheckDependencies, DownloadDependencies,
  FetchMetadata, StartDownloadWithPreset, CancelDownload,
  GetSettings, UpdateSettings, SelectDirectory,
  GetAppVersion, GetVersionInfo, GetHistory,
  ClearHistory, DeleteHistoryEntry,
  UpdateDependencies, OpenOutputDir, CheckForUpdates, OpenExternalLink,
} from "../wailsjs/go/main/App"
import { EventsOn, ClipboardGetText } from "../wailsjs/runtime/runtime"
import type { main } from "../wailsjs/go/models"
import { formatTotalEta, parseBytes, parseSpeed } from "./lib/downloadMetrics"
import { createTranslator, getLanguageLocale, isSupportedLanguage, type LanguageCode } from "./lib/i18n"
import appIcon from './assets/images/app-icon.png'
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
type DownloadPreset = 'best' | 'compatible' | 'audio' | 'custom'
interface AppSettings {
  defaultOutputDir: string
  theme: string
  maxConcurrency: number
  autoPasteURL: boolean
  language: LanguageCode
  downloadPreset: DownloadPreset
  customFormatId: string
  customContainer: string
  customSubtitle: string
}
interface VersionInfo { ytdlp: string; ffmpeg: string; app: string }
interface SupportedSite {
  name: string
  blurbKey: string
  href: string
}

type Tab = 'downloads' | 'history' | 'settings' | 'help'

let historyRequestId = 0
const maxVisibleHistoryEntries = 500

const supportedSites: SupportedSite[] = [
  { name: 'YouTube', blurbKey: 'supportedSites.youtube', href: 'https://www.youtube.com' },
  { name: 'Vimeo', blurbKey: 'supportedSites.vimeo', href: 'https://vimeo.com' },
  { name: 'Dailymotion', blurbKey: 'supportedSites.dailymotion', href: 'https://www.dailymotion.com' },
  { name: 'Twitch', blurbKey: 'supportedSites.twitch', href: 'https://www.twitch.tv' },
  { name: 'TikTok', blurbKey: 'supportedSites.tiktok', href: 'https://www.tiktok.com' },
  { name: 'Twitter (X)', blurbKey: 'supportedSites.twitter', href: 'https://x.com' },
  { name: 'Instagram', blurbKey: 'supportedSites.instagram', href: 'https://www.instagram.com' },
  { name: 'Facebook', blurbKey: 'supportedSites.facebook', href: 'https://www.facebook.com' },
  { name: 'Reddit', blurbKey: 'supportedSites.reddit', href: 'https://www.reddit.com' },
  { name: 'ARD', blurbKey: 'supportedSites.ard', href: 'https://www.ardmediathek.de' },
  { name: 'ZDF', blurbKey: 'supportedSites.zdf', href: 'https://www.zdf.de' },
  { name: 'Arte', blurbKey: 'supportedSites.arte', href: 'https://www.arte.tv' },
  { name: '3sat', blurbKey: 'supportedSites.3sat', href: 'https://www.3sat.de' },
  { name: 'NDR', blurbKey: 'supportedSites.ndr', href: 'https://www.ndr.de' },
  { name: 'BBC', blurbKey: 'supportedSites.bbc', href: 'https://www.bbc.com' },
  { name: 'TED', blurbKey: 'supportedSites.ted', href: 'https://www.ted.com' },
  { name: 'CNN', blurbKey: 'supportedSites.cnn', href: 'https://www.cnn.com' },
  { name: 'Discovery', blurbKey: 'supportedSites.discovery', href: 'https://www.discovery.com' },
  { name: 'Bilibili', blurbKey: 'supportedSites.bilibili', href: 'https://www.bilibili.com' },
  { name: 'Niconico', blurbKey: 'supportedSites.niconico', href: 'https://www.nicovideo.jp' },
  { name: 'Rumble', blurbKey: 'supportedSites.rumble', href: 'https://rumble.com' },
  { name: 'Odysee', blurbKey: 'supportedSites.odysee', href: 'https://odysee.com' },
  { name: 'SoundCloud', blurbKey: 'supportedSites.soundcloud', href: 'https://soundcloud.com' },
  { name: 'Bandcamp', blurbKey: 'supportedSites.bandcamp', href: 'https://bandcamp.com' },
]

const defaultCustomFormatId = 'bestvideo+bestaudio/best'
const defaultCustomContainer = 'mp4'
const defaultCustomSubtitle = 'none'
const defaultDownloadPreset: DownloadPreset = 'compatible'

const downloadPresetOptions: Array<{ value: DownloadPreset; label: string; description: string }> = [
  { value: 'best', label: 'Best quality', description: 'Highest quality. Good for power users.' },
  { value: 'compatible', label: 'Compatible for most players', description: 'Safer files. Good for Windows Media Player and phones.' },
  { value: 'audio', label: 'Audio only', description: 'Only sound. Saves as MP3.' },
  { value: 'custom', label: 'Custom', description: 'Show the advanced fields.' },
]

function isDownloadPreset(value: string): value is DownloadPreset {
  return value === 'best' || value === 'compatible' || value === 'audio' || value === 'custom'
}

function resolveDownloadChoice(
  preset: DownloadPreset,
  customFormatId: string,
  customContainer: string,
  customSubtitle: string,
): { formatId: string; container: string; subtitle: string } {
  switch (preset) {
    case 'best':
      return { formatId: defaultCustomFormatId, container: 'mkv', subtitle: 'none' }
    case 'audio':
      return { formatId: 'bestaudio/best', container: 'mp3', subtitle: 'none' }
    case 'custom':
      return {
        formatId: customFormatId || defaultCustomFormatId,
        container: customContainer || defaultCustomContainer,
        subtitle: customSubtitle || defaultCustomSubtitle,
      }
    case 'compatible':
    default:
      return { formatId: defaultCustomFormatId, container: 'mp4', subtitle: 'none' }
  }
}

function getPresetDescription(preset: DownloadPreset): string {
  return downloadPresetOptions.find((item) => item.value === preset)?.description || ''
}

function formatAppVersionLabel(version: string): string {
  if (!version) return '...'
  if (version === 'dev' || version.startsWith('v')) return version
  return `v${version}`
}

function siteLogoUrl(site: SupportedSite): string {
  if (site.name === 'Niconico') return 'https://www.nicovideo.jp/favicon.ico'
  try {
    const hostname = new URL(site.href).hostname
    return `https://www.google.com/s2/favicons?domain=${encodeURIComponent(hostname)}&sz=64`
  } catch { return '' }
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

function SiteBadge({ site, blurb }: { site: SupportedSite; blurb: string }) {
  return (
    <div
      className="group h-full rounded-2xl p-3.5 lg:p-4 border transition-colors"
      style={{ background: 'var(--color-surface-light)', borderColor: 'var(--color-surface-border)' }}
    >
      <div className="flex items-start gap-3">
        <SiteMark site={site} />
        <div className="min-w-0">
          <p className="text-sm font-semibold leading-5">{site.name}</p>
          <p className="text-xs mt-1 leading-5" style={{ color: 'var(--text-muted)' }}>{blurb}</p>
        </div>
      </div>
    </div>
  )
}

function AppLogo({ sizeClass }: { sizeClass: string }) {
  return (
    <div className={`${sizeClass} shrink-0 flex items-center justify-center overflow-hidden`}>
      <img src={appIcon} alt="" className="w-full h-full object-cover" draggable={false} />
    </div>
  )
}

function buildFormatOptions(formats: FormatInfo[], t: (key: string, params?: Record<string, string | number>) => string): FormatOption[] {
  const options: FormatOption[] = [
    { label: t('downloads.bestVideoAudio'), formatId: 'bestvideo+bestaudio/best' },
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
  options.push({ label: t('downloads.audioOnly'), formatId: 'bestaudio/best' })
  return options
}

function HistoryEntries({ entries, search, onDelete, fmtTime, t }: { entries: main.HistoryEntry[]; search: string; onDelete: (id: string) => void; fmtTime: (t: string) => string; t: (key: string, params?: Record<string, string | number>) => string }) {
  const filtered = useMemo(() => {
    if (!search) return entries
    const q = search.toLowerCase()
    return entries.filter((e) => (e.title || '').toLowerCase().includes(q) || e.url.toLowerCase().includes(q))
  }, [entries, search])
  const visible = filtered.slice(0, maxVisibleHistoryEntries)
  if (filtered.length === 0 && search) {
    return (
      <div className="flex flex-col items-center justify-center py-12" style={{ color: 'var(--text-muted)' }}>
        <p className="text-sm">{t('history.noResults', { query: search })}</p>
      </div>
    )
  }
  return (
    <div className="space-y-2">
      {filtered.length > visible.length && (
        <div className="rounded-lg px-3 py-2 text-xs" style={{ background: 'var(--color-surface-light)', border: '1px solid var(--color-surface-border)', color: 'var(--text-muted)' }}>
          {t('history.showingLimited', { shown: visible.length, total: filtered.length })}
        </div>
      )}
      {visible.map((entry) => (
        <HistoryRow key={entry.downloadId} entry={entry} onDelete={onDelete} fmtTime={fmtTime} t={t} />
      ))}
    </div>
  )
}

const HistoryRow = memo(({ entry, onDelete, fmtTime, t }: { entry: main.HistoryEntry; onDelete: (id: string) => void; fmtTime: (t: string) => string; t: (key: string, params?: Record<string, string | number>) => string }) => {
  const statusKey = `downloads.status.${entry.status}`
  return (
    <div className="rounded-lg p-3.5 lg:p-4 flex items-center gap-3" style={{ background: 'var(--color-surface-light)', border: '1px solid var(--color-surface-border)' }}>
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium truncate">{entry.title || t('common.untitled')}</p>
        <div className="flex flex-wrap gap-x-4 gap-y-1 mt-1.5 text-xs" style={{ color: 'var(--text-secondary)' }}>
          <span>{t('history.started', { time: fmtTime(entry.startTime) })}</span>
          <span>{t('history.ended', { time: fmtTime(entry.endTime) })}</span>
          {entry.fileSize && <span>{t('history.size', { size: entry.fileSize })}</span>}
          {entry.avgSpeed && <span>{t('history.speed', { speed: entry.avgSpeed })}</span>}
          <span className={`font-medium ${entry.status === 'completed' ? 'text-green-400' : entry.status === 'cancelled' ? 'text-yellow-400' : 'text-red-400'}`}>
            {t(statusKey)}
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
        title={t('actions.deleteEntry')}
        aria-label={t('actions.deleteEntry')}
      >
        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
        </svg>
      </button>
    </div>
  )
})

const QueueRow = memo(({
  item,
  onCancel,
  onOpenFolder,
  statusColors,
  tt,
  t
}: {
  item: QueueItem
  onCancel: (id: string) => void
  onOpenFolder: () => void
  statusColors: Record<string, string>
  tt: (key: string) => string
  t: any
}) => {
  return (
    <div className="rounded-lg p-3 lg:p-4 flex items-center gap-3" style={{ background: 'var(--color-surface-light)', border: '1px solid var(--color-surface-border)' }}>
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
            {item.status === 'downloading' && t('downloads.status.downloading', { percent: item.progress })}
            {item.status === 'starting' && t('downloads.status.starting')}
            {item.status === 'queued' && t('downloads.status.queued')}
            {item.status === 'completed' && t('downloads.status.completed')}
            {item.status === 'error' && t('downloads.status.error')}
            {item.status === 'cancelled' && t('downloads.status.cancelled')}
          </span>
          {item.speed && <span style={{ color: 'var(--text-muted)' }}>{item.speed}</span>}
          {item.eta && <span style={{ color: 'var(--text-muted)' }}>{t('downloads.eta', { eta: item.eta })}</span>}
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
          <button onClick={onOpenFolder} className="transition-colors p-1 rounded" style={{ color: 'var(--text-muted)' }} title={tt('openOutputFolder')} aria-label={tt('openOutputFolder')}>
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
          <button onClick={() => onCancel(item.id)} className="transition-colors p-0.5" style={{ color: 'var(--text-muted)' }} title={tt('cancelDownload')} aria-label={tt('cancelDownload')}>
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      )}
      {item.status === 'queued' && (
        <button onClick={() => onCancel(item.id)} className="transition-colors p-0.5 shrink-0" style={{ color: 'var(--text-muted)' }} title={tt('cancelDownload')} aria-label={tt('cancelDownload')}>
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
  )
})

export class ErrorBoundary extends Component<{ children: React.ReactNode }, { hasError: boolean }> {
  state = { hasError: false }
  static getDerivedStateFromError() { return { hasError: true } }
  render() {
    const documentLang = document.documentElement.lang.slice(0, 2)
    const t = createTranslator(documentLang === 'de' || documentLang === 'fr' ? documentLang : 'en')
    if (this.state.hasError) {
      return (
        <div className="h-screen flex flex-col items-center justify-center px-6" style={{ background: 'var(--color-surface)', color: 'var(--text-primary)' }}>
          <h2 className="text-lg font-semibold mb-2">{t('app.errorTitle')}</h2>
          <p className="text-sm mb-4" style={{ color: 'var(--text-muted)' }}>{t('app.errorText')}</p>
          <button onClick={() => { this.setState({ hasError: false }); window.location.reload() }} className="btn-primary text-sm px-4 py-2">{t('app.retry')}</button>
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
  const [selectedPreset, setSelectedPreset] = useState<DownloadPreset>(defaultDownloadPreset)
  const [selectedFormat, setSelectedFormat] = useState(defaultCustomFormatId)
  const [formatOptions, setFormatOptions] = useState<FormatOption[]>([])
  const [selectedContainer, setSelectedContainer] = useState(defaultCustomContainer)
  const [selectedSubs, setSelectedSubs] = useState(defaultCustomSubtitle)

  const [depsReady, setDepsReady] = useState(false)
  const [checkingDeps, setCheckingDeps] = useState(true)
  const [installingDeps, setInstallingDeps] = useState(false)
  const [depProgress, setDepProgress] = useState<Record<string, number>>({})
  const [depError, setDepError] = useState('')

  const [queue, setQueue] = useState<QueueItem[]>([])
  const pendingProgressRef = useRef<Record<string, DownloadProgress>>({})

  const [defaultOutputDir, setDefaultOutputDir] = useState('')
  const [theme, setTheme] = useState('dark')
  const [maxConcurrency, setMaxConcurrency] = useState(3)
  const [autoPasteEnabled, setAutoPasteEnabled] = useState(false)
  const [language, setLanguage] = useState<LanguageCode>('en')

  const [history, setHistory] = useState<main.HistoryEntry[]>([])
  const [historySearch, setHistorySearch] = useState('')
  const [historyLoading, setHistoryLoading] = useState(false)
  const [appVersion, setAppVersion] = useState('')
  const [toolVersions, setToolVersions] = useState<VersionInfo | null>(null)
  const [toolVersionsLoading, setToolVersionsLoading] = useState(true)
  const [addingToQueue, setAddingToQueue] = useState(false)
  const [addQueueError, setAddQueueError] = useState('')
  const [updatingDeps, setUpdatingDeps] = useState(false)
  const [updatesError, setUpdatesError] = useState('')
  const [updateInfo, setUpdateInfo] = useState<main.UpdateInfo | null>(null)
  const [updateLoading, setUpdateLoading] = useState(true)
  const t = useMemo(() => createTranslator(language), [language])
  const tRef = useRef(t)
  tRef.current = t
  const tt = (key: string, params?: Record<string, string | number>) => t(`tooltips.${key}`, params)

  const settingsRef = useRef<AppSettings>({
    defaultOutputDir: '', theme: 'dark', maxConcurrency: 3, autoPasteURL: false,
    language: 'en', downloadPreset: defaultDownloadPreset,
    customFormatId: defaultCustomFormatId, customContainer: defaultCustomContainer, customSubtitle: defaultCustomSubtitle,
  })
  useEffect(() => {
    settingsRef.current = { defaultOutputDir, theme, maxConcurrency, autoPasteURL: autoPasteEnabled, language, downloadPreset: selectedPreset, customFormatId: selectedFormat, customContainer: selectedContainer, customSubtitle: selectedSubs }
  })

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

  const tabs = useMemo<{ id: Tab; label: string; icon: string }[]>(() => [
    { id: 'downloads', label: t('tabs.downloads'), icon: '⬇' },
    { id: 'history', label: t('tabs.history'), icon: '⏱' },
    { id: 'settings', label: t('tabs.settings'), icon: '⚙' },
    { id: 'help', label: t('tabs.help'), icon: '?' },
  ], [t])

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme)
  }, [theme])

  useEffect(() => {
    document.documentElement.lang = getLanguageLocale(language)
    fmtTimeRef.current.clear()
  }, [language])

  useEffect(() => {
    GetSettings()
      .then((s) => {
        setDefaultOutputDir(s.defaultOutputDir)
        setTheme(s.theme || 'dark')
        setMaxConcurrency(s.maxConcurrency || 3)
        setAutoPasteEnabled(!!s.autoPasteURL)
        setLanguage(isSupportedLanguage(s.language) ? s.language : 'en')
        setSelectedPreset(isDownloadPreset(s.downloadPreset) ? s.downloadPreset : defaultDownloadPreset)
        setSelectedFormat(s.customFormatId || defaultCustomFormatId)
        setSelectedContainer(s.customContainer || defaultCustomContainer)
        setSelectedSubs(s.customSubtitle || defaultCustomSubtitle)
      })
      .catch((err) => { console.warn('GetSettings failed:', err) })
  }, [])

  const saveSettings = async (next: Partial<AppSettings>) => {
    await UpdateSettings({ ...settingsRef.current, ...next })
  }

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
      setUpdatesError(t('updates.unavailable'))
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
        if (!cancelled) { setCheckingDeps(false); setDepError(tRef.current('errors.checkDependenciesFailed')) }
      }
    }
    const handleDepProgress = (data: DepProgress) => {
      setDepProgress((prev) => ({ ...prev, [data.dependency]: data.progress }))
      if (data.status === 'error') { setDepError(data.error || tRef.current('errors.downloadFailed')); setInstallingDeps(false) }
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
      .catch((err: any) => { if (!cancelled) { setDepError(err?.message || tRef.current('errors.dependencyInstallFailed')); setInstallingDeps(false) } })
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
          setUrl((prev) => prev || trimmed)
        }
      } catch { /* clipboard errors are benign */ }
    }
    checkClipboard()
    window.addEventListener('focus', checkClipboard)
    return () => window.removeEventListener('focus', checkClipboard)
  }, [depsReady, autoPasteEnabled, activeTab])

  useEffect(() => {
    if (!metadata) return
    setFormatOptions(buildFormatOptions(metadata.formats || [], t))
  }, [metadata, t])

  useEffect(() => {
    if (selectedPreset !== 'custom') return
    if (formatOptions.length === 0) return
    if (!formatOptions.some((opt) => opt.formatId === selectedFormat)) {
      setSelectedFormat(formatOptions[0]?.formatId || defaultCustomFormatId)
    }
  }, [formatOptions, selectedPreset, selectedFormat])

  useEffect(() => {
    const handleDlProgress = (data: DownloadProgress) => {
      setQueue((prev) => {
        const idx = prev.findIndex((item) => item.id === data.downloadId)
        if (idx === -1) {
          pendingProgressRef.current[data.downloadId] = data
          return prev
        }
        const item = prev[idx]
        let nextItem: QueueItem
        if (data.status === 'completed') {
          nextItem = { ...item, status: 'completed', progress: 100, playlistStatus: '' }
        } else if (data.status === 'error') {
          const msg = data.error || t('errors.downloadFailed')
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
  }, [t])

  const handleFetch = async () => {
    if (!url.trim()) return
    const requestId = ++fetchRequestIdRef.current
    setFetching(true)
    setFetchError('')
    setFetched(false)
    setMetadata(null)
    try {
      const meta = await FetchMetadata(url)
      if (requestId !== fetchRequestIdRef.current) return
      setMetadata(meta)
      setFormatOptions(buildFormatOptions(meta.formats || [], t))
      setFetched(true)
    } catch (err: any) {
      if (requestId !== fetchRequestIdRef.current) return
      setFetchError(err?.message || t('errors.fetchMetadataFailed'))
    } finally {
      if (requestId === fetchRequestIdRef.current) setFetching(false)
    }
  }

  const handleAddToQueue = async () => {
    if (!metadata || addingToQueue) return
    setAddingToQueue(true)
    setAddQueueError('')
    try {
      const choice = resolveDownloadChoice(selectedPreset, selectedFormat, selectedContainer, selectedSubs)
      const downloadId = await StartDownloadWithPreset(url, choice.formatId, defaultOutputDir, choice.container, choice.subtitle, metadata.title, selectedPreset)
      setQueue((prev) => {
        const pending = pendingProgressRef.current[downloadId]
        const newItem: QueueItem = {
          id: downloadId,
          title: metadata.title,
          thumbnail: metadata.thumbnail,
          status: pending ? pending.status : 'queued',
          progress: pending ? Math.round(pending.percent) : 0,
          speed: pending ? pending.speed : '',
          eta: pending ? pending.eta : '',
          fileSize: pending ? pending.fileSize : '',
          errorMsg: pending ? (pending.error || '') : '',
          playlistStatus: pending ? (pending.playlistStatus || '') : '',
        }
        delete pendingProgressRef.current[downloadId]
        return [
          ...prev,
          newItem,
        ]
      })
    } catch (err: any) {
      console.error('Failed to start download:', err)
      setAddQueueError(err?.message || t('errors.startDownloadFailed'))
    } finally {
      setAddingToQueue(false)
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
      await saveSettings({ defaultOutputDir: dir })
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
      await saveSettings({ theme: newTheme })
    } catch (err) { console.warn('handleThemeChange failed:', err) }
  }

  const handleLanguageChange = async (nextLanguage: LanguageCode) => {
    setLanguage(nextLanguage)
    try {
      await saveSettings({ language: nextLanguage })
    } catch (err) { console.warn('handleLanguageChange failed:', err) }
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

const fmtTimeRef = useRef<Map<string, string>>(new Map())
const fmtTime = useCallback((t: string): string => {
  const cached = fmtTimeRef.current.get(t)
  if (cached) return cached
  const d = new Date(t)
  if (Number.isNaN(d.getTime())) return '-'
  const locale = getLanguageLocale(language)
  const formatted = d.toLocaleDateString(locale) + ' ' + d.toLocaleTimeString(locale, { hour: '2-digit', minute: '2-digit' })
  if (fmtTimeRef.current.size > 500) fmtTimeRef.current.clear()
  fmtTimeRef.current.set(t, formatted)
  return formatted
}, [language])

  // --- Loading Screen ---
  if (checkingDeps) {
    return (
      <div className="h-screen flex flex-col items-center justify-center" style={{ background: 'var(--color-surface)', color: 'var(--text-primary)' }}>
        <AppLogo sizeClass="w-8 h-8 mb-4" />
        <h1 className="text-lg font-semibold tracking-tight mb-1">{t('app.name')}</h1>
        <p style={{ color: 'var(--text-muted)', fontSize: '0.875rem' }}>{t('setup.checkingDependencies')}</p>
        <div className="mt-4 w-6 h-6 border-2 rounded-full animate-spin" style={{ borderColor: 'var(--color-accent)', borderTopColor: 'transparent' }} />
      </div>
    )
  }

  // --- Setup Screen ---
  if (!depsReady) {
    return (
      <div className="h-screen flex flex-col items-center justify-center px-6" style={{ background: 'var(--color-surface)', color: 'var(--text-primary)' }}>
        <AppLogo sizeClass="w-12 h-12 mb-5" />
        <h1 className="text-xl font-semibold tracking-tight mb-1">{t('setup.title')}</h1>
        <p className="text-sm mb-6 text-center max-w-sm" style={{ color: 'var(--text-secondary)' }}>
          {t('setup.description')}
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
            {depProgress['yt-dlp'] === 100 && depProgress['ffmpeg'] === 100 ? t('setup.finalizing') : t('setup.downloading')}
          </p>
        ) : !depError ? (
          <button onClick={() => { setDepError(''); setDepProgress({}); setInstallingDeps(true) }} className="btn-primary text-sm px-5 py-2 mt-2">
            {t('actions.downloadInstall')}
          </button>
        ) : null}
        {depError && (
          <div className="mt-4 text-center">
            <p className="text-xs mb-2" style={{ color: '#f87171' }}>{depError}</p>
            <button onClick={() => { setDepError(''); setDepProgress({}); setInstallingDeps(true) }} className="btn-primary text-xs px-4 py-1.5">{t('app.retry')}</button>
          </div>
        )}
      </div>
    )
  }

  // --- Main App ---
  return (
    <div className="h-screen flex min-w-[720px]" style={{ background: 'var(--color-surface)', color: 'var(--text-primary)' }}>
      {/* Sidebar */}
      <aside className="w-52 shrink-0 flex flex-col border-r" style={{ background: 'var(--color-surface-light)', borderColor: 'var(--color-surface-border)' }}>
        <div className="flex items-center gap-1.5 px-5 py-4" style={{ background: 'var(--color-surface-light)', borderBottom: '1px solid var(--color-surface-border)' }}>
          <AppLogo sizeClass="w-16 h-16" />
          <span className="font-bold text-2xl tracking-tight">{t('app.name')}</span>
        </div>
        <nav className="flex-1 px-3 py-4 space-y-1" style={{ color: 'var(--text-secondary)' }}>
          {tabs.map((tab) => (
            <button
              key={tab.id}
              type="button"
              onClick={() => handleTabSwitch(tab.id)}
              className={`sidebar-tab w-full text-left ${activeTab === tab.id ? 'active' : ''}`}
              title={tt(`tabs.${tab.id}`)}
              aria-label={tt(`tabs.${tab.id}`)}
              aria-current={activeTab === tab.id ? 'page' : undefined}
            >
              <span className="text-base">{tab.icon}</span>
              <span className="flex-1">{tab.label}</span>
              {tab.id === 'downloads' && activeCount > 0 && (
                <span className="text-xs font-medium px-1.5 py-0.5 rounded-full" style={{ background: 'var(--color-accent)', color: '#000' }}>
                  {activeCount}
                </span>
              )}
              {tab.id === 'settings' && (updateInfo?.ytdlpUpdateAvailable || updateInfo?.koalaPullUpdateAvailable) && (
                <span className="text-xs font-medium px-1.5 py-0.5 rounded-full" style={{ background: '#fbbf24', color: '#000' }}>
                  {(updateInfo?.ytdlpUpdateAvailable ? 1 : 0) + (updateInfo?.koalaPullUpdateAvailable ? 1 : 0)}
                </span>
              )}
            </button>
          ))}
        </nav>
        <div className="px-4 py-3 border-t" style={{ borderColor: 'var(--color-surface-border)' }}>
          <button
            onClick={() => OpenExternalLink('https://github.com/Shik3i/KoalaPull').catch((err) => { console.warn('OpenExternalLink failed:', err) })}
            className="w-full flex items-center gap-2 rounded-lg px-2 py-1.5 transition-colors text-left"
            style={{ color: 'var(--text-muted)' }}
            title={t('app.githubTitle')}
            aria-label={`Visit KoalaPull Github repository. Current version: ${appVersion}`}
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
            <div className="px-4 lg:px-8 py-4 lg:py-5 shrink-0">
              <div className="flex gap-2">
                <div className="flex-1 relative">
                  <label htmlFor="urlInput" className="sr-only">Video URL</label>
                  <input
                    id="urlInput"
                    type="text" value={url}
                    onChange={(e) => setUrl(e.target.value)}
                    onKeyDown={(e) => { if (e.key === 'Enter') handleFetch() }}
                    placeholder={t('downloads.urlPlaceholder')}
                    className="input-dark w-full pr-10 lg:text-sm"
                    title={tt('urlInput')}
                    aria-label={tt('urlInput')}
                  />
                  {url && (
                    <button
                      onClick={() => {
                        setUrl('')
                        setFetched(false)
                        setMetadata(null)
                        setFetchError('')
                      }}
                      className="absolute right-3 top-1/2 -translate-y-1/2 text-lg leading-none"
                      style={{ color: 'var(--text-muted)' }}
                      title={tt('clearUrl')}
                      aria-label={tt('clearUrl')}
                    >&times;</button>
                  )}
                </div>
                <button onClick={handleFetch} disabled={fetching || !url.trim()} className="btn-primary shrink-0 flex items-center gap-2" title={tt('fetch')} aria-label={tt('fetch')}>
                  {fetching ? (
                    <><svg className="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
                      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                    </svg> {t('actions.fetching')}</>
                  ) : t('actions.fetch')}
                </button>
              </div>
              {fetchError && (
                <div className="mt-3 px-4 py-3 rounded-lg text-sm" style={{ background: 'rgba(239,68,68,0.15)', border: '1px solid rgba(239,68,68,0.3)', color: '#fca5a5' }}>
                  {fetchError}
                </div>
              )}
            </div>

            <div className="flex-1 overflow-y-auto px-4 lg:px-8 py-4 lg:py-6 space-y-4">
              {fetched && metadata && (
                <div className="rounded-lg overflow-hidden" style={{ background: 'var(--color-surface-light)', border: '1px solid var(--color-surface-border)' }}>
                  <div className="flex gap-4 p-4">
                    {metadata.thumbnail ? (
                      <img src={metadata.thumbnail} alt={metadata.title} loading="lazy" className="w-36 lg:w-52 h-20 lg:h-28 rounded-md object-cover shrink-0" style={{ background: 'var(--color-surface-lighter)' }} />
                    ) : (
                      <div className="w-36 lg:w-52 h-20 lg:h-28 rounded-md shrink-0 flex items-center justify-center" style={{ background: 'var(--color-surface-lighter)', border: '1px solid var(--color-surface-border)' }}>
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
                          {t('downloads.playlistBadge', { count: metadata.entryCount })}
                        </span>
                      )}
                      <div className="mt-3 space-y-3">
                        <div className="flex flex-wrap gap-2 items-start">
                          <div className="flex-1 min-w-[220px]">
                            <label htmlFor="selectedPreset" className="block text-xs mb-1" style={{ color: 'var(--text-muted)' }}>Preset</label>
                            <select
                              id="selectedPreset"
                              value={selectedPreset}
                              onChange={async (e) => {
                                const next = e.target.value as DownloadPreset
                                setSelectedPreset(next)
                                try {
                                  await saveSettings({ downloadPreset: next })
                                } catch (err) {
                                  console.warn('UpdateSettings failed:', err)
                                }
                              }}
                              className="select-dark text-xs w-full"
                              title="Download preset"
                              aria-label="Download preset"
                            >
                              {downloadPresetOptions.map((opt) => (
                                <option key={opt.value} value={opt.value}>{opt.label}</option>
                              ))}
                            </select>
                          </div>
                          <div className="flex-1 min-w-[220px] rounded-md px-3 py-2 text-xs leading-5" style={{ background: 'var(--color-surface-lighter)', border: '1px solid var(--color-surface-border)', color: 'var(--text-secondary)' }}>
                            {getPresetDescription(selectedPreset)}
                          </div>
                        </div>

                        {selectedPreset === 'custom' && (
                          <div className="flex flex-wrap gap-2">
                            <label htmlFor="customFormatSelect" className="sr-only">Format</label>
                            <select
                              id="customFormatSelect"
                              value={selectedFormat}
                              onChange={async (e) => {
                                const next = e.target.value
                                setSelectedFormat(next)
                                try { await saveSettings({ customFormatId: next }) } catch (err) { console.warn('UpdateSettings failed:', err) }
                              }}
                              className="select-dark text-xs flex-1 min-w-[140px]"
                              title={tt('formatSelect')}
                              aria-label={tt('formatSelect')}
                            >
                              {formatOptions.map((opt) => (
                                <option key={opt.formatId} value={opt.formatId}>{opt.label}</option>
                              ))}
                            </select>
                            <label htmlFor="customSubtitleSelect" className="sr-only">Subtitles</label>
                            <select
                              id="customSubtitleSelect"
                              value={selectedSubs}
                              onChange={async (e) => {
                                const next = e.target.value
                                setSelectedSubs(next)
                                try { await saveSettings({ customSubtitle: next }) } catch (err) { console.warn('UpdateSettings failed:', err) }
                              }}
                              className="select-dark text-xs flex-1 min-w-[100px]"
                              title={tt('subtitleSelect')}
                              aria-label={tt('subtitleSelect')}
                            >
                              <option value="none">{t('downloads.subtitlesNone')}</option>
                              <option value="auto">{t('downloads.subtitlesAuto')}</option>
                              <option value="embed">{t('downloads.subtitlesEmbed')}</option>
                            </select>
                            <label htmlFor="customContainerSelect" className="sr-only">Container</label>
                            <select
                              id="customContainerSelect"
                              value={selectedContainer}
                              onChange={async (e) => {
                                const next = e.target.value
                                setSelectedContainer(next)
                                try { await saveSettings({ customContainer: next }) } catch (err) { console.warn('UpdateSettings failed:', err) }
                              }}
                              className="select-dark text-xs flex-1 min-w-[80px]"
                              title={tt('containerSelect')}
                              aria-label={tt('containerSelect')}
                            >
                              <option value="mp4">MP4</option>
                              <option value="mkv">MKV</option>
                              <option value="mp3">MP3</option>
                            </select>
                          </div>
                        )}
                      </div>
                      <button onClick={handleAddToQueue} disabled={addingToQueue} className="btn-primary mt-3 text-sm flex items-center gap-1.5" title={tt('addToQueue')} aria-label={tt('addToQueue')}>
                        {addingToQueue ? (
                          <svg className="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
                            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                          </svg>
                        ) : (
                          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
                          </svg>
                        )}
                        {addingToQueue ? '...' : t('actions.addToQueue')}
                      </button>
                      {addQueueError && (
                        <p className="mt-2 text-xs" style={{ color: '#f87171' }}>{addQueueError}</p>
                      )}
                    </div>
                  </div>
                </div>
              )}

              <div>
                <div className="flex items-center mb-3">
                  <div className="flex-1 text-center">
                    <h3 className="text-sm font-medium uppercase tracking-wider inline-block" style={{ color: 'var(--text-secondary)' }} title={tt('downloadList')}>
                      {t('downloads.summary', { count: queue.length })}
                      {activeCount > 0 && (
                        <span className="ml-2 font-normal normal-case" style={{ color: 'var(--text-muted)', fontSize: '0.75rem' }}>
                          {t('downloads.activeSummary', { count: activeCount })}{totalEta ? ` · ${totalEta}` : ''}
                        </span>
                      )}
                    </h3>
                  </div>
                  {queue.some((i) => ['completed', 'error', 'cancelled'].includes(i.status)) && (
                    <button onClick={handleClearCompleted} className="text-xs transition-colors shrink-0" style={{ color: 'var(--text-muted)' }} title={tt('clearCompleted')} aria-label={tt('clearCompleted')}>{t('actions.clearCompleted')}</button>
                  )}
                </div>
                {queue.length === 0 ? (
                  <div className="flex flex-col items-center justify-center py-12" style={{ color: 'var(--text-muted)' }}>
                    <svg className="w-12 h-12 mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M7 16V4m0 0L3 8m4-4l4 4m6 0v12m0 0l4-4m-4 4l-4-4" />
                    </svg>
                    <p className="text-sm">{t('downloads.emptyTitle')}</p>
                    <p className="text-xs mt-1">{t('downloads.emptyText')}</p>
                  </div>
                ) : (
                  <div className="space-y-2">
                    {reversedQueue.map((item) => (
                      <QueueRow
                        key={item.id}
                        item={item}
                        onCancel={handleCancel}
                        onOpenFolder={() => OpenOutputDir().catch((err) => { console.warn('OpenOutputDir failed:', err) })}
                        statusColors={statusColors}
                        tt={tt}
                        t={t}
                      />
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
            <div className="flex flex-col sm:flex-row sm:items-center gap-2 px-4 lg:px-8 py-4 shrink-0">
              <div className="flex-1 text-center">
                <h2 className="text-base font-semibold inline-block" title={tt('searchHistory')}>{t('history.title')}</h2>
              </div>
              <div className="flex items-center gap-3 w-full sm:w-auto shrink-0">
                {history.length > 0 && (
                  <>
                    <input
                      type="text" value={historySearch}
                      onChange={(e) => setHistorySearch(e.target.value)}
                      placeholder={t('history.searchPlaceholder')}
                      className="input-dark text-xs w-full sm:w-44 lg:w-56"
                      title={tt('searchHistory')}
                      aria-label={tt('searchHistory')}
                    />
                    <button
                      onClick={() => { if (window.confirm(t('history.clearConfirm'))) handleClearHistory() }}
                      className="btn-primary text-xs px-3 py-1.5"
                      title={tt('clearHistory')}
                      aria-label={tt('clearHistory')}
                    >{t('actions.clearAll')}</button>
                  </>
                )}
              </div>
            </div>
            <div className="flex-1 overflow-y-auto px-4 lg:px-8 py-4 lg:py-6">
              {historyLoading ? (
                <div className="flex flex-col items-center justify-center py-16" style={{ color: 'var(--text-muted)' }}>
                  <div className="w-6 h-6 border-2 rounded-full animate-spin mb-3" style={{ borderColor: 'var(--color-accent)', borderTopColor: 'transparent' }} />
                  <p className="text-sm">{t('history.loading')}</p>
                </div>
              ) : history.length === 0 ? (
                <div className="flex flex-col items-center justify-center py-16" style={{ color: 'var(--text-muted)' }}>
                  <svg className="w-12 h-12 mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                  <p className="text-sm">{t('history.empty')}</p>
                </div>
              ) : (
                <HistoryEntries entries={history} search={historySearch} onDelete={handleDeleteHistoryEntry} fmtTime={fmtTime} t={t} />
              )}
            </div>
          </div>
        )}

        {/* --- Settings Tab --- */}
        {activeTab === 'settings' && (
          <div className="flex-1 flex flex-col overflow-hidden">
            <div className="px-4 lg:px-8 py-4 lg:py-5 shrink-0">
              <h2 className="text-base lg:text-lg font-semibold" title={tt('languageSelect')}>{t('settings.title')}</h2>
            </div>
            <div className="flex-1 overflow-y-auto px-4 lg:px-8 py-5 lg:py-6">
              <div className="max-w-4xl grid gap-6 grid-cols-1 md:grid-cols-2 xl:grid-cols-3 items-start">
              {/* Theme */}
              <section className="rounded-xl p-4 border" style={{ background: 'var(--color-surface-light)', borderColor: 'var(--color-surface-border)' }}>
                <h3 className="text-sm font-medium mb-3" style={{ color: 'var(--text-secondary)' }} title={tt('themeDark')}>{t('settings.appearance')}</h3>
                <div className="flex gap-3">
                  {(['dark', 'light'] as const).map((themeOption) => (
                    <button
                      key={themeOption}
                      onClick={() => handleThemeChange(themeOption)}
                      className="flex-1 rounded-lg py-3 px-4 text-sm font-medium transition-colors"
                      title={themeOption === 'dark' ? tt('themeDark') : tt('themeLight')}
                      aria-label={themeOption === 'dark' ? tt('themeDark') : tt('themeLight')}
                      style={{
                        background: theme === themeOption ? 'color-mix(in srgb, var(--color-accent) 20%, transparent)' : 'var(--color-surface-lighter)',
                        border: theme === themeOption ? '1px solid var(--color-accent)' : '1px solid var(--color-surface-border)',
                        color: theme === themeOption ? 'var(--color-accent)' : 'var(--text-secondary)',
                      }}
                    >
                      {themeOption === 'dark' ? `🌙 ${t('settings.themeDark')}` : `☀ ${t('settings.themeLight')}`}
                    </button>
                  ))}
                </div>
              </section>

              <section className="rounded-xl p-4 border" style={{ background: 'var(--color-surface-light)', borderColor: 'var(--color-surface-border)' }}>
                <label htmlFor="languageSelect" className="block text-sm font-medium mb-3" style={{ color: 'var(--text-secondary)' }} title={tt('languageSelect')}>{t('settings.language')}</label>
                <select id="languageSelect" value={language} onChange={(e) => { void handleLanguageChange(e.target.value as LanguageCode) }} className="select-dark text-sm w-full" title={tt('languageSelect')} aria-label={tt('languageSelect')}>
                  <option value="en">{t('settings.languageEnglish')}</option>
                  <option value="de">{t('settings.languageGerman')}</option>
                  <option value="fr">{t('settings.languageFrench')}</option>
                </select>
              </section>

              {/* Download Location */}
              <section className="rounded-xl p-4 border" style={{ background: 'var(--color-surface-light)', borderColor: 'var(--color-surface-border)' }}>
                <h3 className="text-sm font-medium mb-3" style={{ color: 'var(--text-secondary)' }} title={tt('downloadLocation')}>{t('settings.downloadLocation')}</h3>
                <div className="flex gap-2">
                  <div className="flex-1 rounded-md px-3 py-2 text-xs leading-5 break-all whitespace-normal" style={{ background: 'var(--color-surface-lighter)', border: '1px solid var(--color-surface-border)', color: 'var(--text-secondary)' }} title={tt('downloadLocation')}>
                    {defaultOutputDir}
                  </div>
                  <button onClick={handleChangeFolder} className="btn-primary text-xs px-3 py-1.5 shrink-0" title={tt('changeFolder')} aria-label={tt('changeFolder')}>{t('actions.change')}</button>
                </div>
              </section>

              {/* Max Concurrency */}
              <section className="rounded-xl p-4 border" style={{ background: 'var(--color-surface-light)', borderColor: 'var(--color-surface-border)' }}>
                <h3 className="text-sm font-medium mb-3" style={{ color: 'var(--text-secondary)' }} title={tt('maxConcurrency')}>{t('settings.downloads')}</h3>
                <div className="flex items-center gap-3">
                  <label htmlFor="maxConcurrency" className="text-xs" style={{ color: 'var(--text-muted)', minWidth: '7rem' }} title={tt('maxConcurrency')}>{t('settings.maxParallelDownloads')}</label>
                  <input
                    id="maxConcurrency"
                    type="number" min={1} max={10}
                    value={maxConcurrency}
                    onChange={async (e) => {
                      const v = Math.max(1, Math.min(10, parseInt(e.target.value) || 1))
                      setMaxConcurrency(v)
                      try { await saveSettings({ maxConcurrency: v }) } catch (err) { console.warn('UpdateSettings failed:', err) }
                    }}
                    className="input-dark text-xs w-16 text-center"
                    title={tt('maxConcurrency')}
                    aria-label={tt('maxConcurrency')}
                  />
                </div>
              </section>

              {/* Auto-Paste URL */}
              <section className="rounded-xl p-4 border" style={{ background: 'var(--color-surface-light)', borderColor: 'var(--color-surface-border)' }}>
                <div className="flex items-center justify-between">
                  <div>
                    <h3 className="text-sm font-medium" style={{ color: 'var(--text-secondary)' }} title={tt('autoPaste')}>{t('settings.autoPasteTitle')}</h3>
                    <p className="text-xs mt-0.5" style={{ color: 'var(--text-muted)' }} title={tt('autoPaste')}>{t('settings.autoPasteDescription')}</p>
                  </div>
                  <button
                    role="switch"
                    aria-checked={autoPasteEnabled}
                    onClick={async () => {
                      const next = !autoPasteEnabled
                      setAutoPasteEnabled(next)
                      try { await saveSettings({ autoPasteURL: next }) } catch (err) { console.warn('UpdateSettings failed:', err) }
                    }}
                    className="relative w-10 h-5 rounded-full transition-colors shrink-0"
                    style={{
                      background: autoPasteEnabled ? 'var(--color-accent)' : 'var(--color-surface-border)',
                    }}
                    title={tt('autoPaste')}
                    aria-label={tt('autoPaste')}
                  >
                    <span
                      className="absolute top-0.5 w-4 h-4 rounded-full bg-white transition-transform"
                      style={{ left: '2px', transform: autoPasteEnabled ? 'translateX(20px)' : 'translateX(0)' }}
                    />
                  </button>
                </div>
              </section>

              {/* Version Info */}
              <section className="md:col-span-2 xl:col-span-3 rounded-xl p-4 border" style={{ background: 'var(--color-surface-light)', borderColor: 'var(--color-surface-border)' }}>
                <h3 className="text-sm font-medium mb-3" style={{ color: 'var(--text-secondary)' }} title={tt('versionKoalaPull')}>{t('settings.versions')}</h3>
                <div className="space-y-2 text-sm">
                  <div className="flex items-center gap-2">
                    <span className="w-20" style={{ color: 'var(--text-muted)' }}>KoalaPull</span>
                    <button
                      onClick={() => OpenExternalLink('https://github.com/Shik3i/KoalaPull').catch(() => {})}
                      className="font-mono hover:underline inline-flex items-center gap-1"
                      style={{ color: 'var(--color-accent)' }}
                      title={tt('versionKoalaPull')}
                      aria-label={tt('versionKoalaPull')}
                    >
                      <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                        <path d="M12 0C5.37 0 0 5.37 0 12c0 5.3 3.44 9.8 8.21 11.38.6.11.79-.26.79-.58v-2.23c-3.34.73-4.03-1.41-4.03-1.41-.55-1.39-1.33-1.76-1.33-1.76-1.09-.75.08-.73.08-.73 1.21.08 1.84 1.24 1.84 1.24 1.07 1.83 2.81 1.3 3.49 1 .11-.78.42-1.31.76-1.61-2.66-.3-5.47-1.33-5.47-5.93 0-1.31.47-2.38 1.24-3.22-.12-.3-.53-1.53.12-3.18 0 0 1-.32 3.3 1.23.96-.27 1.98-.4 3-.4s2.05.13 3.01.4c2.29-1.55 3.3-1.23 3.3-1.23.65 1.65.24 2.88.12 3.18.77.84 1.24 1.91 1.24 3.22 0 4.61-2.81 5.62-5.48 5.92.43.37.82 1.1.82 2.22v3.29c0 .32.19.69.8.57C20.56 21.8 24 17.3 24 12c0-6.63-5.37-12-12-12z"/>
                      </svg>
                      {appVersion ? formatAppVersionLabel(appVersion) : '-'}
                    </button>
                  </div>
                  <div className="flex items-center gap-2">
                    <span className="w-20" style={{ color: 'var(--text-muted)' }}>yt-dlp</span>
                    <button
                      type="button"
                      onClick={() => OpenExternalLink('https://github.com/yt-dlp/yt-dlp').catch(() => {})}
                      className="font-mono hover:underline inline-flex items-center gap-1"
                      style={{ color: 'var(--color-accent)', background: 'none', border: 'none', padding: 0 }}
                      title={tt('versionYtdlp')}
                      aria-label={tt('versionYtdlp')}
                    >
                      {toolVersionsLoading ? t('common.loading') : (toolVersions?.ytdlp || '-')}
                      <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24" style={{ opacity: 0.6 }}>
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                      </svg>
                    </button>
                  </div>
                  <div className="flex items-center gap-2">
                    <span className="w-20" style={{ color: 'var(--text-muted)' }}>ffmpeg</span>
                    <button
                      type="button"
                      onClick={() => OpenExternalLink(navigator.platform.includes('Mac') ? 'https://evermeet.cx/ffmpeg/' : 'https://github.com/BtbN/FFmpeg-Builds').catch(() => {})}
                      className="font-mono hover:underline inline-flex items-center gap-1"
                      style={{ color: 'var(--color-accent)', background: 'none', border: 'none', padding: 0 }}
                      title={tt('versionFfmpeg')}
                      aria-label={tt('versionFfmpeg')}
                    >
                      {toolVersionsLoading ? t('common.loading') : (toolVersions?.ffmpeg || '-')}
                      <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24" style={{ opacity: 0.6 }}>
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                      </svg>
                    </button>
                  </div>
                </div>
              </section>

              {/* Updates */}
              <section className="md:col-span-2 xl:col-span-3 rounded-xl p-4 border" style={{ background: 'var(--color-surface-light)', borderColor: 'var(--color-surface-border)' }}>
                <h3 className="text-sm font-medium mb-3" style={{ color: 'var(--text-secondary)' }} title={tt('redownloadDependencies')}>{t('settings.updates')}</h3>
                <div className="text-xs space-y-3" style={{ color: 'var(--text-muted)' }}>
                  {updateInfo ? (
                    <>
                      {/* KoalaPull */}
                      <div>
                        {updateInfo.koalaPullUpdateAvailable ? (
                          <div>
                            <span style={{ color: '#fbbf24' }}>{t('updates.koalaAvailable', { version: updateInfo.latestKoalaPullVersion })}</span>
                            <span className="ml-2" style={{ color: 'var(--text-muted)' }}>{t('common.currentVersion', { version: appVersion ? formatAppVersionLabel(appVersion) : '?' })}</span>
                            <button
                              onClick={() => OpenExternalLink('https://github.com/Shik3i/KoalaPull/releases/latest').catch(() => {})}
                              className="btn-primary text-xs px-3 py-1 ml-3"
                              title={tt('viewRelease')}
                              aria-label={tt('viewRelease')}
                            >{t('actions.viewRelease')}</button>
                          </div>
                        ) : (
                          <p>{t('updates.koalaCurrent', { version: appVersion ? formatAppVersionLabel(appVersion) : '?' })}</p>
                        )}
                      </div>

                      {/* yt-dlp */}
                      <div>
                      {updateInfo.ytdlpUpdateAvailable ? (
                          <div>
                            <span style={{ color: '#fbbf24' }}>{t('updates.ytdlpAvailable', { version: updateInfo.latestYtdlpVersion })}</span>
                            <span className="ml-2" style={{ color: 'var(--text-muted)' }}>{t('common.currentVersion', { version: toolVersions?.ytdlp || '?' })}</span>
                          </div>
                        ) : (
                          <p>{t('updates.ytdlpCurrent', { version: toolVersions?.ytdlp || '?' })}</p>
                        )}
                        <button
                          onClick={async () => {
                            setUpdatingDeps(true)
                            setUpdatesError('')
                            try {
                              await UpdateDependencies()
                              await Promise.all([loadToolVersions(), loadUpdateInfo()])
                            } catch (err: any) {
                              setUpdatesError(err?.message || t('errors.updateFailed'))
                            } finally {
                              setUpdatingDeps(false)
                            }
                          }}
                          disabled={updatingDeps}
                          className="btn-primary text-xs px-4 py-1.5 mt-2"
                          title={tt('redownloadDependencies')}
                          aria-label={tt('redownloadDependencies')}
                        >
                          {updatingDeps ? t('actions.updating') : updateInfo.ytdlpUpdateAvailable ? t('actions.downloadUpdate') : t('actions.redownload')}
                        </button>
                      </div>
                    </>
                  ) : updateLoading ? (
                    <p>{t('updates.checking')}</p>
                  ) : (
                    <p>{t('updates.unavailable')}</p>
                  )}
                  {updatesError && (
                    <p className="mt-2" style={{ color: '#f87171' }}>{updatesError}</p>
                  )}
                </div>
              </section>
              </div>
            </div>
          </div>
        )}

        {/* --- Help Tab --- */}
        {activeTab === 'help' && (
          <div className="flex-1 flex flex-col overflow-hidden">
            <div className="px-4 lg:px-8 py-4 lg:py-5 shrink-0">
              <h2 className="text-base lg:text-lg font-semibold" title={tt('helpSteps')}>{t('help.title')}</h2>
            </div>
            <div className="flex-1 overflow-y-auto px-4 lg:px-8 py-5 lg:py-6">
              <div className="space-y-6 max-w-6xl">
                <section className="rounded-xl p-4 lg:p-6 border" style={{ background: 'var(--color-surface-light)', borderColor: 'var(--color-surface-border)' }}>
                  <h3 className="text-sm lg:text-base font-medium mb-2" style={{ color: 'var(--text-secondary)' }} title={tt('helpSteps')}>{t('help.howToTitle')}</h3>
                  <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                    {[
                      {
                        step: '1',
                        title: t('help.steps.oneTitle'),
                        text: t('help.steps.oneText'),
                      },
                      {
                        step: '2',
                        title: t('help.steps.twoTitle'),
                        text: t('help.steps.twoText'),
                      },
                      {
                        step: '3',
                        title: t('help.steps.threeTitle'),
                        text: t('help.steps.threeText'),
                      },
                      {
                        step: '4',
                        title: t('help.steps.fourTitle'),
                        text: t('help.steps.fourText'),
                      },
                    ].map((item) => (
                      <div
                        key={item.step}
                        className="rounded-xl border p-3 lg:p-4"
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
                      <h3 className="text-sm lg:text-base font-medium" style={{ color: 'var(--text-secondary)' }} title={tt('supportedSites')}>{t('help.supportedTitle')}</h3>
                      <p className="text-xs lg:text-sm mt-1 max-w-2xl" style={{ color: 'var(--text-muted)' }}>
                        {t('help.supportedText')}
                      </p>
                    </div>
                    <button
                      onClick={() => OpenExternalLink('https://github.com/yt-dlp/yt-dlp/blob/master/supportedsites.md').catch((err) => { console.warn('OpenExternalLink failed:', err) })}
                      className="text-xs hover:underline shrink-0"
                      style={{ color: 'var(--color-accent)' }}
                      title={tt('viewAllSites')}
                      aria-label={tt('viewAllSites')}
                    >
                      {t('actions.viewAllSites')}
                    </button>
                  </div>

                  <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 2xl:grid-cols-6">
                    {supportedSites.map((site) => (
                      <button
                        key={site.name}
                        onClick={() => OpenExternalLink(site.href).catch((err) => { console.warn('OpenExternalLink failed:', err) })}
                        className="block text-left"
                        title={tt('openSite', { site: site.name })}
                        aria-label={tt('openSite', { site: site.name })}
                      >
                        <SiteBadge site={site} blurb={t(site.blurbKey)} />
                      </button>
                    ))}
                  </div>
                </section>

                <section className="rounded-xl p-4 lg:p-6 border" style={{ background: 'var(--color-surface-light)', borderColor: 'var(--color-surface-border)' }}>
                  <h3 className="text-sm lg:text-base font-medium mb-2" style={{ color: 'var(--text-secondary)' }} title={tt('underTheHood')}>{t('help.underTheHood')}</h3>
                  <div className="space-y-3 text-sm lg:text-base leading-6 lg:leading-7" style={{ color: 'var(--text-secondary)' }}>
                    <p>{t('help.underTheHoodText.one')}</p>
                    <p>{t('help.underTheHoodText.two')}</p>
                    <p>{t('help.underTheHoodText.three')}</p>
                    <p>{t('help.underTheHoodText.four')}</p>
                    <p>{t('help.underTheHoodText.five')}</p>
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
