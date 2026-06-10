import { memo } from 'react'
import { PlayFile, ShowFileInFolder } from '../../wailsjs/go/main/App'
import type { AppSettings, DownloadPreset } from './SettingsTab'

interface FormatInfo {
  formatId: string
  ext: string
  width: number
  height: number
  vcodec: string
  acodec: string
  filesize: number
  formatNote: string
}

interface PlaylistEntry {
  id: string
  title: string
}

export interface VideoMetadata {
  id: string
  title: string
  thumbnail: string
  uploader: string
  duration: number
  formats: FormatInfo[]
  isPlaylist: boolean
  entryCount: number
  entries?: PlaylistEntry[]
}

export interface QueueItem {
  id: string
  title: string
  thumbnail: string
  status: string
  progress: number
  speed: string
  eta: string
  fileSize: string
  errorMsg: string
  playlistStatus: string
  outputDir: string
  url?: string
  formatId?: string
  container?: string
  subtitle?: string
  preset?: DownloadPreset
  playlistItems?: string
  outputPath?: string
}

const statusColors: Record<string, string> = {
  downloading: 'text-accent',
  starting: 'text-accent',
  retrying: 'text-accent',
  paused: 'text-yellow-400',
  queued: 'text-gray-400',
  completed: 'text-green-400',
  error: 'text-red-400',
  cancelled: 'text-yellow-400',
}

const downloadPresetOptions: Array<{ value: DownloadPreset; label: string; description: string }> = [
  { value: 'best', label: 'Best quality', description: 'Highest quality. Good for power users.' },
  { value: 'compatible', label: 'Compatible for most players', description: 'Safer files. Good for Windows Media Player and phones.' },
  { value: 'audio', label: 'Audio only', description: 'Only sound. Saves as MP3.' },
  { value: 'custom', label: 'Custom', description: 'Show the advanced fields.' },
]

function getPresetDescription(preset: DownloadPreset): string {
  return downloadPresetOptions.find((item) => item.value === preset)?.description || ''
}

function findQueuedSwapIndex(items: QueueItem[], id: string, direction: 'up' | 'down'): number {
  const index = items.findIndex((item) => item.id === id)
  if (index === -1) return -1
  const step = direction === 'up' ? -1 : 1
  for (let cursor = index + step; cursor >= 0 && cursor < items.length; cursor += step) {
    if (items[cursor].status === 'queued') return cursor
  }
  return -1
}

const SpeedSparkline = ({ history }: { history?: Array<{ speed: number }> }) => {
  if (!history || history.length < 2) return null
  const data = history.map((h) => h.speed)
  if (data.length > 20) {
    data.splice(0, data.length - 20)
  }
  const width = 60
  const height = 18
  const max = Math.max(...data, 1)
  const min = Math.min(...data)
  const range = max - min || 1

  const points = data.map((val, index) => {
    const x = (index / (data.length - 1)) * width
    const y = height - ((val - min) / range) * height
    return `${x},${y}`
  })

  const pathD = `M ${points.join(' L ')}`
  const fillD = `${pathD} L ${width},${height} L 0,${height} Z`

  return (
    <svg width={width} height={height} className="opacity-80 inline-block align-middle ml-2" style={{ overflow: 'visible' }}>
      <path d={fillD} fill="color-mix(in srgb, var(--color-accent) 15%, transparent)" />
      <path d={pathD} fill="none" stroke="var(--color-accent)" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

const QueueRow = memo(
  ({
    item,
    onCancel,
    onRetry,
    onPause,
    onResume,
    onMove,
    onMoveToEdge,
    canMoveUp,
    canMoveDown,
    canMoveTop,
    canMoveBottom,
    onOpenFolder,
    statusColors,
    tt,
    t,
    speedHistory,
  }: {
    item: QueueItem
    onCancel: (id: string) => void
    onRetry: (item: QueueItem) => void
    onPause: (id: string) => void
    onResume: (id: string) => void
    onMove: (id: string, direction: 'up' | 'down') => void
    onMoveToEdge: (id: string, edge: 'top' | 'bottom') => void
    canMoveUp: boolean
    canMoveDown: boolean
    canMoveTop: boolean
    canMoveBottom: boolean
    onOpenFolder: (outputDir: string) => void
    statusColors: Record<string, string>
    tt: (key: string, params?: Record<string, string | number>) => string
    t: any
    speedHistory?: Array<{ speed: number }>
  }) => {
    return (
      <div
        className="rounded-lg p-3 lg:p-4 flex items-center gap-3"
        style={{ background: 'var(--color-surface-light)', border: '1px solid var(--color-surface-border)' }}
      >
        <div
          className="w-16 h-10 rounded shrink-0 flex items-center justify-center overflow-hidden"
          style={{ background: 'var(--color-surface-lighter)', border: '1px solid var(--color-surface-border)' }}
        >
          {item.thumbnail ? (
            <img src={item.thumbnail} alt="" loading="lazy" className="w-full h-full object-cover" />
          ) : (
            <svg className="w-5 h-5" style={{ color: 'var(--text-muted)' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={1.5}
                d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z"
              />
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
              {item.status === 'retrying' && t('downloads.status.retrying')}
              {item.status === 'queued' && t('downloads.status.queued')}
              {item.status === 'completed' && t('downloads.status.completed')}
              {item.status === 'error' && t('downloads.status.error')}
              {item.status === 'cancelled' && t('downloads.status.cancelled')}
              {item.status === 'paused' && t('downloads.status.paused')}
            </span>
            {item.speed && (
              <span style={{ color: 'var(--text-muted)' }} className="inline-flex items-center">
                {item.speed}
                <SpeedSparkline history={speedHistory} />
              </span>
            )}
            {item.eta && <span style={{ color: 'var(--text-muted)' }}>{t('downloads.eta', { eta: item.eta })}</span>}
            {item.playlistStatus && <span style={{ color: 'var(--text-muted)' }}>{item.playlistStatus}</span>}
          </div>
          {(item.status === 'downloading' || item.status === 'starting' || item.status === 'retrying' || item.status === 'paused') && (
            <div className="mt-1.5 w-full h-1 rounded-full overflow-hidden" style={{ background: 'var(--color-surface-lighter)' }}>
              <div
                className="h-full rounded-full transition-all duration-300 ease-out"
                style={{ width: `${Math.max(item.progress, 2)}%`, background: 'var(--color-accent)' }}
              />
            </div>
          )}
          {item.status === 'completed' && (
            <>
              <div className="mt-1.5 w-full h-1 rounded-full overflow-hidden" style={{ background: 'var(--color-surface-lighter)' }}>
                <div className="h-full rounded-full" style={{ width: '100%', background: '#22c55e' }} />
              </div>
              {item.errorMsg && (
                <p className="mt-1 text-xs truncate" style={{ color: '#fbbf24' }}>
                  {item.errorMsg}
                </p>
              )}
            </>
          )}
          {item.status === 'error' && (
            <>
              <div className="mt-1.5 w-full h-1 rounded-full overflow-hidden" style={{ background: 'var(--color-surface-lighter)' }}>
                <div className="h-full rounded-full" style={{ width: '100%', background: '#ef4444' }} />
              </div>
              {item.errorMsg && (
                <p className="mt-1 text-xs truncate" style={{ color: '#f87171' }}>
                  {item.errorMsg}
                </p>
              )}
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
            {item.outputPath && (
              <>
                <button
                  onClick={() => PlayFile(item.outputPath!)}
                  className="icon-button"
                  style={{ color: 'var(--color-accent)' }}
                  title={t('actions.playFile')}
                  aria-label={t('actions.playFile')}
                >
                  <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                    <path d="M8 5v14l11-7z" />
                  </svg>
                </button>
                <button
                  onClick={() => ShowFileInFolder(item.outputPath!)}
                  className="icon-button"
                  style={{ color: 'var(--text-secondary)' }}
                  title={t('actions.showFile')}
                  aria-label={t('actions.showFile')}
                >
                  <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
                    />
                  </svg>
                </button>
              </>
            )}
            <button
              onClick={() => onOpenFolder(item.outputDir)}
              className="icon-button"
              style={{ color: 'var(--text-muted)' }}
              title={tt('openOutputFolder')}
              aria-label={tt('openOutputFolder')}
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M5 19a2 2 0 01-2-2V7a2 2 0 012-2h4l2 2h4a2 2 0 012 2v1M5 19h14a2 2 0 002-2v-5a2 2 0 00-2-2H9a2 2 0 00-2 2v5a2 2 0 01-2 2z"
                />
              </svg>
            </button>
            <svg className="w-5 h-5 shrink-0" style={{ color: '#22c55e' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
            </svg>
          </div>
        )}
        {(item.status === 'downloading' || item.status === 'starting' || item.status === 'retrying') && (
          <div className="flex items-center gap-1">
            <svg
              className="w-5 h-5 animate-pulse shrink-0"
              style={{ color: 'var(--color-accent)' }}
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 14l-7 7m0 0l-7-7m7 7V3" />
            </svg>
            <button
              onClick={() => onPause(item.id)}
              className="icon-button"
              style={{ color: 'var(--text-secondary)' }}
              title={t('actions.pauseDownload')}
              aria-label={t('actions.pauseDownload')}
            >
              <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                <path d="M8 5h3v14H8zM13 5h3v14h-3z" />
              </svg>
            </button>
            <button
              onClick={() => onCancel(item.id)}
              className="icon-button"
              style={{ color: 'var(--text-muted)' }}
              title={tt('cancelDownload')}
              aria-label={tt('cancelDownload')}
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
        )}
        {item.status === 'paused' && (
          <div className="flex items-center gap-1">
            <button
              onClick={() => onResume(item.id)}
              className="icon-button"
              style={{ color: 'var(--color-accent)' }}
              title={t('actions.resumeDownload')}
              aria-label={t('actions.resumeDownload')}
            >
              <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                <path d="M8 5v14l11-7z" />
              </svg>
            </button>
            <button
              onClick={() => onCancel(item.id)}
              className="icon-button"
              style={{ color: 'var(--text-muted)' }}
              title={tt('cancelDownload')}
              aria-label={tt('cancelDownload')}
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
        )}
        {item.status === 'queued' && (
          <div className="flex items-center gap-1 shrink-0">
            <button
              onClick={() => onMoveToEdge(item.id, 'top')}
              disabled={!canMoveTop}
              className="icon-button"
              style={{ color: canMoveTop ? 'var(--text-secondary)' : 'var(--text-muted)' }}
              title={t('actions.moveToTop')}
              aria-label={t('actions.moveToTop')}
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 10l7-7 7 7M5 21l7-7 7 7" />
              </svg>
            </button>
            <button
              onClick={() => onMove(item.id, 'up')}
              disabled={!canMoveUp}
              className="icon-button"
              style={{ color: canMoveUp ? 'var(--text-secondary)' : 'var(--text-muted)' }}
              title={t('actions.moveUp')}
              aria-label={t('actions.moveUp')}
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 15l7-7 7 7" />
              </svg>
            </button>
            <button
              onClick={() => onMove(item.id, 'down')}
              disabled={!canMoveDown}
              className="icon-button"
              style={{ color: canMoveDown ? 'var(--text-secondary)' : 'var(--text-muted)' }}
              title={t('actions.moveDown')}
              aria-label={t('actions.moveDown')}
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
              </svg>
            </button>
            <button
              onClick={() => onMoveToEdge(item.id, 'bottom')}
              disabled={!canMoveBottom}
              className="icon-button"
              style={{ color: canMoveBottom ? 'var(--text-secondary)' : 'var(--text-muted)' }}
              title={t('actions.moveToBottom')}
              aria-label={t('actions.moveToBottom')}
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 3l7 7 7-7M5 14l7 7 7-7" />
              </svg>
            </button>
            <button
              onClick={() => onCancel(item.id)}
              className="icon-button"
              style={{ color: 'var(--text-muted)' }}
              title={tt('cancelDownload')}
              aria-label={tt('cancelDownload')}
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
        )}
        {item.status === 'error' && (
          <div className="flex items-center gap-1">
            <button
              onClick={() => onRetry(item)}
              className="icon-button"
              style={{ color: 'var(--color-accent)' }}
              title={t('actions.retryDownload')}
              aria-label={t('actions.retryDownload')}
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h5M20 20v-5h-5" />
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M20 9a8 8 0 00-13.657-5.657L4 5m16 14l-2.343-2.343A8 8 0 014 15" />
              </svg>
            </button>
            <svg className="w-5 h-5 shrink-0" style={{ color: '#ef4444' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L4.082 16.5c-.77.833.192 2.5 1.732 2.5z"
              />
            </svg>
          </div>
        )}
        {item.status === 'cancelled' && (
          <div className="flex items-center gap-1">
            <button
              onClick={() => onRetry(item)}
              className="icon-button"
              style={{ color: 'var(--color-accent)' }}
              title={t('actions.retryDownload')}
              aria-label={t('actions.retryDownload')}
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h5M20 20v-5h-5" />
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M20 9a8 8 0 00-13.657-5.657L4 5m16 14l-2.343-2.343A8 8 0 014 15" />
              </svg>
            </button>
            <svg className="w-5 h-5 shrink-0" style={{ color: '#eab308' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </div>
        )}
      </div>
    )
  }
)

interface DownloadsTabProps {
  downloadMode: 'single' | 'batch'
  setDownloadMode: (mode: 'single' | 'batch') => void
  url: string
  setUrl: (url: string) => void
  fetching: boolean
  fetched: boolean
  setFetched: (f: boolean) => void
  metadata: VideoMetadata | null
  setMetadata: (m: VideoMetadata | null) => void
  fetchError: string
  setFetchError: (err: string) => void
  handleFetch: () => void
  batchUrls: string
  setBatchUrls: (urls: string) => void
  selectedPreset: DownloadPreset
  setSelectedPreset: (preset: DownloadPreset) => void
  selectedFormat: string
  setSelectedFormat: (format: string) => void
  videoOptions: { value: string; label: string }[]
  audioOptions: { value: string; label: string }[]
  videoId: string
  audioId: string
  handleVideoChange: (nextVideoId: string) => void
  handleAudioChange: (nextAudioId: string) => void
  selectedContainer: string
  setSelectedContainer: (container: string) => void
  selectedSubs: string
  setSelectedSubs: (subs: string) => void
  selectedPlaylistIndices: Record<number, boolean>
  setSelectedPlaylistIndices: (indices: Record<number, boolean> | ((prev: Record<number, boolean>) => Record<number, boolean>)) => void
  addingToQueue: boolean
  handleAddToQueue: () => void
  addQueueError: string
  queue: QueueItem[]
  activeCount: number
  totalEta: string
  handleRetryFailed: () => void
  handleClearFailed: () => void
  handleClearCompleted: () => void
  reversedQueue: QueueItem[]
  handleCancel: (id: string) => void
  handleRetryQueueItem: (item: QueueItem) => void
  handlePauseQueueItem: (id: string) => void
  handleResumeQueueItem: (id: string) => void
  handleMoveQueueItem: (id: string, direction: 'up' | 'down') => void
  handleMoveQueueItemToEdge: (id: string, edge: 'top' | 'bottom') => void
  handleOpenFolder: (outputDir: string) => void
  handleBatchImport: () => void
  batchAdding: boolean
  urlInputRef: React.RefObject<HTMLInputElement>
  progressHistoryRef: React.MutableRefObject<Record<string, Array<{ speed: number }>>>
  saveSettings: (s: Partial<AppSettings>) => Promise<void>
  t: (key: string, params?: Record<string, string | number>) => string
  tt: (key: string, params?: Record<string, string | number>) => string
}

export function DownloadsTab({
  downloadMode,
  setDownloadMode,
  url,
  setUrl,
  fetching,
  fetched,
  setFetched,
  metadata,
  setMetadata,
  fetchError,
  setFetchError,
  handleFetch,
  batchUrls,
  setBatchUrls,
  selectedPreset,
  setSelectedPreset,
  selectedFormat,
  setSelectedFormat,
  videoOptions,
  audioOptions,
  videoId,
  audioId,
  handleVideoChange,
  handleAudioChange,
  selectedContainer,
  setSelectedContainer,
  selectedSubs,
  setSelectedSubs,
  selectedPlaylistIndices,
  setSelectedPlaylistIndices,
  addingToQueue,
  handleAddToQueue,
  addQueueError,
  queue,
  activeCount,
  totalEta,
  handleRetryFailed,
  handleClearFailed,
  handleClearCompleted,
  reversedQueue,
  handleCancel,
  handleRetryQueueItem,
  handlePauseQueueItem,
  handleResumeQueueItem,
  handleMoveQueueItem,
  handleMoveQueueItemToEdge,
  handleOpenFolder,
  handleBatchImport,
  batchAdding,
  urlInputRef,
  progressHistoryRef,
  saveSettings,
  t,
  tt,
}: DownloadsTabProps) {
  return (
    <div className="flex-1 flex flex-col overflow-hidden">
      {/* Download Mode Tabs */}
      <div className="px-4 lg:px-8 pt-4 shrink-0 flex border-b border-[var(--color-surface-border)]">
        <button
          type="button"
          onClick={() => setDownloadMode('single')}
          aria-pressed={downloadMode === 'single'}
          className="px-4 py-2 text-sm font-semibold transition-all border-b-2 outline-none flex items-center gap-1.5"
          style={{
            color: downloadMode === 'single' ? 'var(--color-accent)' : 'var(--text-muted)',
            borderColor: downloadMode === 'single' ? 'var(--color-accent)' : 'transparent',
          }}
        >
          {t('downloads.singleTab') || 'Single URL'}
        </button>
        <button
          type="button"
          onClick={() => setDownloadMode('batch')}
          aria-pressed={downloadMode === 'batch'}
          className="px-4 py-2 text-sm font-semibold transition-all border-b-2 outline-none flex items-center gap-1.5"
          style={{
            color: downloadMode === 'batch' ? 'var(--color-accent)' : 'var(--text-muted)',
            borderColor: downloadMode === 'batch' ? 'var(--color-accent)' : 'transparent',
          }}
        >
          {t('downloads.batchTab') || 'Batch Import'}
        </button>
      </div>

      <div className="px-4 lg:px-8 py-4 lg:py-5 shrink-0">
        {downloadMode === 'single' ? (
          <div className="flex flex-col sm:flex-row gap-2">
            <div className="flex-1 relative">
              <label htmlFor="urlInput" className="sr-only">
                Video URL
              </label>
              <input
                id="urlInput"
                type="text"
                ref={urlInputRef}
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') handleFetch()
                }}
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
                  className="icon-button absolute right-1 top-1/2 -translate-y-1/2 text-lg leading-none"
                  style={{ color: 'var(--text-muted)' }}
                  title={tt('clearUrl')}
                  aria-label={tt('clearUrl')}
                >
                  &times;
                </button>
              )}
            </div>
            <button
              onClick={handleFetch}
              disabled={fetching || !url.trim()}
              className="btn-primary shrink-0 flex items-center gap-2"
              title={tt('fetch')}
              aria-label={tt('fetch')}
            >
              {fetching ? (
                <>
                  <svg className="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                  </svg>{' '}
                  {t('actions.fetching')}
                </>
              ) : (
                t('actions.fetch')
              )}
            </button>
          </div>
        ) : (
          <div className="flex flex-col gap-3">
            <div className="relative">
              <textarea
                id="batchUrlsInput"
                rows={4}
                value={batchUrls}
                onChange={(e) => setBatchUrls(e.target.value)}
                placeholder={t('downloads.batchPlaceholder') || 'Paste URLs here, one per line...'}
                aria-label={t('downloads.batchTab') || 'Batch Import'}
                className="input-dark w-full text-xs font-mono resize-none p-3 rounded-lg border border-[var(--color-surface-border)]"
                style={{ background: 'var(--color-surface-light)' }}
              />
            </div>

            {/* Batch Download Preset Selector */}
            <div className="p-3 rounded-lg border border-[var(--color-surface-border)]" style={{ background: 'var(--color-surface-light)' }}>
              <span className="block text-xs font-semibold mb-2" style={{ color: 'var(--text-secondary)' }}>
                {t('downloads.selectPreset')}
              </span>
              <div className="grid grid-cols-3 gap-2.5">
                {(['best', 'compatible', 'audio'] as const).map((presetOption) => {
                  const isSelected = selectedPreset === presetOption
                  let title = ''
                  if (presetOption === 'best') {
                    title = t('downloads.presetBest')
                  } else if (presetOption === 'compatible') {
                    title = t('downloads.presetCompatible')
                  } else if (presetOption === 'audio') {
                    title = t('downloads.presetAudio')
                  }

                  return (
                    <button
                      key={presetOption}
                      type="button"
                      onClick={async () => {
                        setSelectedPreset(presetOption)
                        try {
                          await saveSettings({ downloadPreset: presetOption })
                        } catch (err) {
                          console.warn('UpdateSettings failed:', err)
                        }
                      }}
                      className="flex flex-col items-center justify-center p-2 rounded-lg border text-center transition-all cursor-pointer outline-none focus-visible:ring-2 focus-visible:ring-accent"
                      style={{
                        background: isSelected
                          ? 'color-mix(in srgb, var(--color-accent) 12%, var(--color-surface-light))'
                          : 'var(--color-surface-light)',
                        borderColor: isSelected ? 'var(--color-accent)' : 'var(--color-surface-border)',
                      }}
                    >
                      <span
                        className="text-[11px] font-bold"
                        style={{ color: isSelected ? 'var(--color-accent)' : 'var(--text-primary)' }}
                      >
                        {title}
                      </span>
                    </button>
                  )
                })}
              </div>
            </div>

            <button
              onClick={handleBatchImport}
              disabled={batchAdding || !batchUrls.trim()}
              className="btn-primary flex items-center justify-center gap-2 py-2.5 text-xs font-bold"
              title={t('downloads.addBatchToQueue')}
            >
              {batchAdding ? (
                <>
                  <svg className="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                  </svg>{' '}
                  {t('common.loading') || 'Processing...'}
                </>
              ) : (
                <>{t('downloads.addBatchToQueue') || 'Add Batch to Queue'}</>
              )}
            </button>
          </div>
        )}
        {fetchError && (
          <div
            className="mt-3 px-4 py-3 rounded-lg text-sm"
            style={{
              background: 'rgba(239,68,68,0.15)',
              border: '1px solid rgba(239,68,68,0.3)',
              color: '#fca5a5',
            }}
          >
            {fetchError}
          </div>
        )}
      </div>

      <div className="flex-1 overflow-y-auto px-4 lg:px-8 py-4 lg:py-6 space-y-4">
        {downloadMode === 'single' && fetched && metadata && (
          <div
            className="rounded-lg overflow-hidden"
            style={{ background: 'var(--color-surface-light)', border: '1px solid var(--color-surface-border)' }}
          >
            <div className="flex flex-col sm:flex-row gap-4 p-4">
              {metadata.thumbnail ? (
                <img
                  src={metadata.thumbnail}
                  alt={metadata.title}
                  loading="lazy"
                  className="w-36 lg:w-52 h-20 lg:h-28 rounded-md object-cover shrink-0"
                  style={{ background: 'var(--color-surface-lighter)' }}
                />
              ) : (
                <div
                  className="w-36 lg:w-52 h-20 lg:h-28 rounded-md shrink-0 flex items-center justify-center"
                  style={{ background: 'var(--color-surface-lighter)', border: '1px solid var(--color-surface-border)' }}
                >
                  <svg className="w-8 h-8" style={{ color: 'var(--text-muted)' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={1.5}
                      d="M15 10l4.553-2.276A1 1 0 0121 8.618v6.764a1 1 0 01-1.447.894L15 14M5 18h8a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z"
                    />
                  </svg>
                </div>
              )}
              <div className="flex-1 min-w-0">
                <h2 className="text-base font-semibold truncate">{metadata.title}</h2>
                <p className="text-sm mt-0.5" style={{ color: 'var(--text-secondary)' }}>
                  {metadata.uploader}
                </p>
                {metadata.isPlaylist && (
                  <span
                    className="inline-block mt-1 text-xs px-2 py-0.5 rounded font-medium"
                    style={{
                      background: 'color-mix(in srgb, var(--color-accent) 20%, transparent)',
                      color: 'var(--color-accent)',
                    }}
                  >
                    {t('downloads.playlistBadge', { count: metadata.entryCount })}
                  </span>
                )}
                <div className="mt-3 space-y-3">
                  <div className="flex flex-wrap gap-2 items-start">
                    <div className="flex-1 min-w-[220px]">
                      <label htmlFor="selectedPreset" className="block text-xs mb-1" style={{ color: 'var(--text-muted)' }}>
                        Preset
                      </label>
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
                          <option key={opt.value} value={opt.value}>
                            {opt.label}
                          </option>
                        ))}
                      </select>
                    </div>
                    <div className="flex-1 min-w-[220px]">
                      {selectedPreset !== 'custom' && (
                        <>
                          <div className="text-xs mb-1 select-none opacity-0" aria-hidden="true">
                            &nbsp;
                          </div>
                          <div
                            className="rounded-md px-3 py-2 text-xs leading-5"
                            style={{
                              background: 'var(--color-surface-lighter)',
                              border: '1px solid var(--color-surface-border)',
                              color: 'var(--text-secondary)',
                            }}
                          >
                            {getPresetDescription(selectedPreset)}
                          </div>
                        </>
                      )}
                    </div>
                  </div>

                  {selectedPreset === 'custom' && (
                    <div className="flex flex-col gap-4 w-full">
                      {/* Video Stream Selection */}
                      <div>
                        <span className="block text-xs font-semibold mb-2" style={{ color: 'var(--text-secondary)' }}>
                          📹 {t('downloads.videoLabel') || 'Video Stream'}
                        </span>
                        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-2.5 max-h-[160px] overflow-y-auto pr-1.5 custom-scrollbar">
                          {videoOptions.map((opt) => {
                            const isSelected = videoId === opt.value
                            let title = opt.label
                            let badge = ''
                            let details = ''

                            if (opt.value === 'bestvideo') {
                              title = t('downloads.bestVideoAudio') || 'Best Quality'
                              badge = '⭐'
                              details = 'Auto-select highest'
                            } else if (opt.value === 'none') {
                              title = t('downloads.noVideo') || 'No Video'
                              badge = '❌'
                              details = 'Audio only'
                            } else {
                              const parts = opt.label.split(' · ')
                              title = parts[0] || ''
                              if (parts.length > 2) {
                                badge = parts[2].toUpperCase()
                                details = parts[1] || ''
                              } else if (parts.length > 1) {
                                badge = parts[1].toUpperCase()
                                details = ''
                              }
                            }

                            return (
                              <button
                                key={opt.value}
                                type="button"
                                onClick={() => handleVideoChange(opt.value)}
                                className="flex flex-col text-left p-2.5 rounded-lg border transition-all relative outline-none focus-visible:ring-2 focus-visible:ring-accent"
                                style={{
                                  background: isSelected
                                    ? 'color-mix(in srgb, var(--color-accent) 12%, var(--color-surface-light))'
                                    : 'var(--color-surface-light)',
                                  borderColor: isSelected ? 'var(--color-accent)' : 'var(--color-surface-border)',
                                }}
                              >
                                <div className="flex items-center justify-between w-full mb-1">
                                  <span
                                    className="text-xs font-bold truncate pr-2"
                                    style={{ color: isSelected ? 'var(--color-accent)' : 'var(--text-primary)' }}
                                  >
                                    {title}
                                  </span>
                                  {badge && (
                                    <span
                                      className="text-[10px] px-1.5 py-0.5 rounded font-mono font-bold"
                                      style={{
                                        background: isSelected ? 'var(--color-accent)' : 'var(--color-surface-lighter)',
                                        color: isSelected ? 'var(--color-surface-light)' : 'var(--text-secondary)',
                                      }}
                                    >
                                      {badge}
                                    </span>
                                  )}
                                </div>
                                {details && (
                                  <span className="text-[10px] truncate" style={{ color: 'var(--text-muted)' }}>
                                    {details}
                                  </span>
                                )}
                                {isSelected && (
                                  <div className="absolute bottom-1 right-1 text-[10px]" style={{ color: 'var(--color-accent)' }}>
                                    ✓
                                  </div>
                                )}
                              </button>
                            )
                          })}
                        </div>
                      </div>

                      {/* Audio Stream Selection */}
                      <div>
                        <span className="block text-xs font-semibold mb-2" style={{ color: 'var(--text-secondary)' }}>
                          🎵 {t('downloads.audioLabel') || 'Audio Stream'}
                        </span>
                        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-2.5 max-h-[160px] overflow-y-auto pr-1.5 custom-scrollbar">
                          {audioOptions.map((opt) => {
                            const isSelected = audioId === opt.value
                            let title = opt.label
                            let badge = ''
                            let details = ''

                            if (opt.value === 'bestaudio') {
                              title = t('downloads.bestAudio') || 'Best Quality'
                              badge = '⭐'
                              details = 'Auto-select highest'
                            } else if (opt.value === 'none') {
                              title = t('downloads.noAudio') || 'No Audio'
                              badge = '❌'
                              details = 'Video only'
                            } else {
                              const parts = opt.label.split(' · ')
                              title = parts[0] || ''
                              if (parts.length > 1) {
                                badge = parts[1].toUpperCase()
                              }
                              if (title.includes(' · ')) {
                                const subparts = title.split(' · ')
                                title = subparts[0]
                                details = subparts.slice(1).join(' · ')
                              }
                            }

                            return (
                              <button
                                key={opt.value}
                                type="button"
                                onClick={() => handleAudioChange(opt.value)}
                                className="flex flex-col text-left p-2.5 rounded-lg border transition-all relative outline-none focus-visible:ring-2 focus-visible:ring-accent"
                                style={{
                                  background: isSelected
                                    ? 'color-mix(in srgb, var(--color-accent) 12%, var(--color-surface-light))'
                                    : 'var(--color-surface-light)',
                                  borderColor: isSelected ? 'var(--color-accent)' : 'var(--color-surface-border)',
                                }}
                              >
                                <div className="flex items-center justify-between w-full mb-1">
                                  <span
                                    className="text-xs font-bold truncate pr-2"
                                    style={{ color: isSelected ? 'var(--color-accent)' : 'var(--text-primary)' }}
                                  >
                                    {title}
                                  </span>
                                  {badge && (
                                    <span
                                      className="text-[10px] px-1.5 py-0.5 rounded font-mono font-bold"
                                      style={{
                                        background: isSelected ? 'var(--color-accent)' : 'var(--color-surface-lighter)',
                                        color: isSelected ? 'var(--color-surface-light)' : 'var(--text-secondary)',
                                      }}
                                    >
                                      {badge}
                                    </span>
                                  )}
                                </div>
                                {details && (
                                  <span className="text-[10px] truncate" style={{ color: 'var(--text-muted)' }}>
                                    {details}
                                  </span>
                                )}
                                {isSelected && (
                                  <div className="absolute bottom-1 right-1 text-[10px]" style={{ color: 'var(--color-accent)' }}>
                                    ✓
                                  </div>
                                )}
                              </button>
                            )
                          })}
                        </div>
                      </div>

                      {/* Subtitle & Container Selection */}
                      <div className="flex flex-wrap gap-2.5">
                        <div className="flex-1 min-w-[140px]">
                          <label htmlFor="customSubtitleSelect" className="block text-xs mb-1 font-semibold" style={{ color: 'var(--text-secondary)' }}>
                            {t('downloads.subtitleSelect') || 'Subtitles'}
                          </label>
                          <select
                            id="customSubtitleSelect"
                            value={selectedSubs}
                            onChange={async (e) => {
                              const next = e.target.value
                              setSelectedSubs(next)
                              try {
                                await saveSettings({ customSubtitle: next })
                              } catch (err) {
                                console.warn('UpdateSettings failed:', err)
                              }
                            }}
                            className="select-dark text-xs w-full"
                            title={tt('subtitleSelect')}
                            aria-label={tt('subtitleSelect')}
                          >
                            <option value="none">{t('downloads.subtitlesNone')}</option>
                            <option value="auto">{t('downloads.subtitlesAuto')}</option>
                            <option value="embed">{t('downloads.subtitlesEmbed')}</option>
                          </select>
                        </div>
                        <div className="flex-1 min-w-[100px]">
                          <label htmlFor="customContainerSelect" className="block text-xs mb-1 font-semibold" style={{ color: 'var(--text-secondary)' }}>
                            {t('downloads.containerSelect') || 'Container'}
                          </label>
                          <select
                            id="customContainerSelect"
                            value={selectedContainer}
                            onChange={async (e) => {
                              const next = e.target.value
                              setSelectedContainer(next)
                              try {
                                    await saveSettings({ customContainer: next })
                              } catch (err) {
                                console.warn('UpdateSettings failed:', err)
                              }
                            }}
                            className="select-dark text-xs w-full"
                            title={tt('containerSelect')}
                            aria-label={tt('containerSelect')}
                          >
                            <optgroup label="Video">
                              <option value="mp4">MP4</option>
                              <option value="mkv">MKV</option>
                              <option value="webm">WEBM</option>
                            </optgroup>
                            <optgroup label="Audio">
                              <option value="mp3">MP3</option>
                              <option value="aac">AAC</option>
                              <option value="m4a">M4A (AAC)</option>
                              <option value="opus">OPUS</option>
                              <option value="flac">FLAC</option>
                              <option value="wav">WAV</option>
                            </optgroup>
                          </select>
                        </div>
                      </div>
                    </div>
                  )}
                  {metadata.isPlaylist && metadata.entries && (
                    <div
                      className="mt-4 p-3 rounded-lg border border-[var(--color-surface-border)]"
                      style={{ background: 'var(--color-surface-light)' }}
                    >
                      <div className="flex items-center justify-between mb-2">
                        <span className="text-xs font-bold" style={{ color: 'var(--text-primary)' }}>
                          📑 Select Playlist Videos ({Object.values(selectedPlaylistIndices).filter(Boolean).length}/
                          {metadata.entries.length})
                        </span>
                        <div className="flex gap-2">
                          <button
                            type="button"
                            onClick={() => {
                              const next: Record<number, boolean> = {}
                              metadata.entries!.forEach((_, i) => {
                                next[i + 1] = true
                              })
                              setSelectedPlaylistIndices(next)
                            }}
                            className="text-[10px] text-accent hover:underline font-semibold"
                          >
                            Select All
                          </button>
                          <span className="text-[10px]" style={{ color: 'var(--text-muted)' }}>
                            |
                          </span>
                          <button
                            type="button"
                            onClick={() => {
                              setSelectedPlaylistIndices({})
                            }}
                            className="text-[10px] text-accent hover:underline font-semibold"
                          >
                            Deselect All
                          </button>
                        </div>
                      </div>
                      <div className="space-y-1 max-h-[160px] overflow-y-auto pr-1.5 custom-scrollbar">
                        {metadata.entries.map((entry, idx) => {
                          const index = idx + 1
                          const isChecked = !!selectedPlaylistIndices[index]
                          return (
                            <label
                              key={entry.id || idx}
                              className="flex items-center gap-2 p-1.5 rounded hover:bg-[var(--color-surface-lighter)] cursor-pointer text-xs select-none transition-colors"
                            >
                              <input
                                type="checkbox"
                                checked={isChecked}
                                onChange={(e) => {
                                  setSelectedPlaylistIndices((prev) => ({
                                    ...prev,
                                    [index]: e.target.checked,
                                  }))
                                }}
                                className="rounded border-[var(--color-surface-border)] text-accent focus:ring-accent"
                              />
                              <span className="font-mono text-[10px]" style={{ color: 'var(--text-muted)' }}>
                                {index}.
                              </span>
                              <span
                                className="truncate flex-1"
                                style={{ color: isChecked ? 'var(--text-primary)' : 'var(--text-muted)' }}
                              >
                                {entry.title || `Video ${index}`}
                              </span>
                            </label>
                          )
                        })}
                      </div>
                    </div>
                  )}
                </div>
                <button
                  onClick={handleAddToQueue}
                  disabled={addingToQueue}
                  className="btn-primary mt-3 text-sm flex items-center gap-1.5"
                  title={tt('addToQueue')}
                  aria-label={tt('addToQueue')}
                >
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
                  <p className="mt-2 text-xs" style={{ color: '#f87171' }}>
                    {addQueueError}
                  </p>
                )}
              </div>
            </div>
          </div>
        )}

        <div>
          <div className="flex items-center mb-3">
            <div className="flex-1 text-center">
              <h3
                className="text-sm font-medium uppercase tracking-wider inline-block"
                style={{ color: 'var(--text-secondary)' }}
                title={tt('downloadList')}
              >
                {t('downloads.summary', { count: queue.length })}
                {activeCount > 0 && (
                  <span className="ml-2 font-normal normal-case" style={{ color: 'var(--text-muted)', fontSize: '0.75rem' }}>
                    {t('downloads.activeSummary', { count: activeCount })}
                    {totalEta ? ` · ${totalEta}` : ''}
                  </span>
                )}
              </h3>
            </div>
            <div className="flex items-center gap-3 shrink-0">
              {queue.some((i) => i.status === 'error' || i.status === 'cancelled') && (
                <>
                  <button
                    onClick={() => {
                      void handleRetryFailed()
                    }}
                    className="text-xs transition-colors"
                    style={{ color: 'var(--color-accent)' }}
                    title={t('actions.retryFailed')}
                    aria-label={t('actions.retryFailed')}
                  >
                    {t('actions.retryFailed')}
                  </button>
                  <button
                    onClick={handleClearFailed}
                    className="text-xs transition-colors"
                    style={{ color: 'var(--text-muted)' }}
                    title={t('actions.clearFailed')}
                    aria-label={t('actions.clearFailed')}
                  >
                    {t('actions.clearFailed')}
                  </button>
                </>
              )}
              {queue.some((i) => ['completed', 'error', 'cancelled'].includes(i.status)) && (
                <button
                  onClick={handleClearCompleted}
                  className="text-xs transition-colors"
                  style={{ color: 'var(--text-muted)' }}
                  title={tt('clearCompleted')}
                  aria-label={tt('clearCompleted')}
                >
                  {t('actions.clearCompleted')}
                </button>
              )}
            </div>
          </div>
          {queue.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12" style={{ color: 'var(--text-muted)' }}>
              <svg className="w-12 h-12 mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M7 16V4m0 0L3 8m4-4l4 4m6 0v12m0 0l4-4m-4 4l-4-4" />
              </svg>
              <p className="text-sm">{t('downloads.emptyTitle')}</p>
              <p className="text-xs mt-1">{t('downloads.emptyText')}</p>
              <div
                className="mt-5 w-full max-w-md rounded-xl p-4 text-left"
                style={{ background: 'var(--color-surface-light)', border: '1px solid var(--color-surface-border)' }}
              >
                <p className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                  {t('downloads.emptyHelperTitle')}
                </p>
                <p className="text-xs mt-1 leading-5" style={{ color: 'var(--text-muted)' }}>
                  {t('downloads.emptyHelperText')}
                </p>
                <div className="mt-3 flex flex-wrap gap-2">
                  <button
                    onClick={() => {
                      setDownloadMode('single')
                      requestAnimationFrame(() => urlInputRef.current?.focus())
                    }}
                    className="btn-primary text-xs px-3 py-1.5"
                  >
                    {t('downloads.emptyHelperSingleCta')}
                  </button>
                  <button
                    onClick={() => setDownloadMode('batch')}
                    className="text-xs px-3 py-1.5 rounded-md transition-colors"
                    style={{
                      background: 'var(--color-surface-lighter)',
                      border: '1px solid var(--color-surface-border)',
                      color: 'var(--text-secondary)',
                    }}
                  >
                    {t('downloads.emptyHelperBatchCta')}
                  </button>
                </div>
              </div>
            </div>
          ) : (
            <div className="space-y-2">
              {reversedQueue.map((item) => {
                const canMoveUp = item.status === 'queued' && findQueuedSwapIndex(reversedQueue, item.id, 'up') !== -1
                const canMoveDown = item.status === 'queued' && findQueuedSwapIndex(reversedQueue, item.id, 'down') !== -1
                const canMoveTop = item.status === 'queued' && reversedQueue.find((entry) => entry.status === 'queued')?.id !== item.id
                const canMoveBottom =
                  item.status === 'queued' &&
                  [...reversedQueue].reverse().find((entry) => entry.status === 'queued')?.id !== item.id
                return (
                  <QueueRow
                    key={item.id}
                    item={item}
                    onCancel={handleCancel}
                    onRetry={handleRetryQueueItem}
                    onPause={handlePauseQueueItem}
                    onResume={handleResumeQueueItem}
                    onMove={handleMoveQueueItem}
                    onMoveToEdge={handleMoveQueueItemToEdge}
                    canMoveUp={canMoveUp}
                    canMoveDown={canMoveDown}
                    canMoveTop={canMoveTop}
                    canMoveBottom={canMoveBottom}
                    onOpenFolder={handleOpenFolder}
                    statusColors={statusColors}
                    tt={tt}
                    t={t}
                    speedHistory={progressHistoryRef.current[item.id]}
                  />
                )
              })}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
