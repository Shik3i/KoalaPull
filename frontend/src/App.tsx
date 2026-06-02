import { useState, useEffect, useCallback } from 'react'
import {
  CheckDependencies, DownloadDependencies,
  FetchMetadata, StartDownload, CancelDownload,
  GetSettings, UpdateSettings, SelectDirectory,
} from "../wailsjs/go/main/App"
import { EventsOn, EventsOff } from "../wailsjs/runtime/runtime"
import './style.css'

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

interface VideoMetadata {
  id: string
  title: string
  thumbnail: string
  uploader: string
  duration: number
  formats: FormatInfo[]
  isPlaylist: boolean
  entryCount: number
}

interface QueueItem {
  id: string
  title: string
  thumbnail: string
  status: string
  progress: number
  speed: string
  eta: string
  errorMsg: string
  playlistStatus: string
}

interface DepProgress {
  dependency: string
  progress: number
  status: string
  error?: string
}

interface DownloadProgress {
  downloadId: string
  percent: number
  speed: string
  eta: string
  status: string
  error?: string
  playlistStatus?: string
}

interface FormatOption {
  label: string
  formatId: string
}

interface AppSettings {
  defaultOutputDir: string
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

function App() {
  const [url, setUrl] = useState('')
  const [fetched, setFetched] = useState(false)
  const [fetching, setFetching] = useState(false)
  const [fetchError, setFetchError] = useState('')
  const [metadata, setMetadata] = useState<VideoMetadata | null>(null)
  const [selectedFormat, setSelectedFormat] = useState('bestvideo+bestaudio/best')
  const [formatOptions, setFormatOptions] = useState<FormatOption[]>([])

  const [depsReady, setDepsReady] = useState(false)
  const [checkingDeps, setCheckingDeps] = useState(true)
  const [installingDeps, setInstallingDeps] = useState(false)
  const [depProgress, setDepProgress] = useState<Record<string, number>>({})
  const [depError, setDepError] = useState('')
  const [ytdlpVersion, setYtdlpVersion] = useState('')

  const [queue, setQueue] = useState<QueueItem[]>([])

  const [showSettings, setShowSettings] = useState(false)
  const [defaultOutputDir, setDefaultOutputDir] = useState('')

  // Load settings
  useEffect(() => {
    GetSettings()
      .then((s: AppSettings) => setDefaultOutputDir(s.defaultOutputDir))
      .catch(() => {})
  }, [])

  // Dependency check
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
          setInstallingDeps(true)
        }
      } catch {
        if (!cancelled) {
          setCheckingDeps(false)
          setDepError('Failed to check dependencies.')
        }
      }
    }
    const handleDepProgress = (data: DepProgress) => {
      setDepProgress((prev) => ({ ...prev, [data.dependency]: data.progress }))
      if (data.status === 'error') {
        setDepError(data.error || 'Download failed')
        setInstallingDeps(false)
      }
    }
    EventsOn('dependency-progress', handleDepProgress)
    run()
    return () => {
      cancelled = true
      EventsOff('dependency-progress')
    }
  }, [])

  // Auto-trigger dependency install
  useEffect(() => {
    if (installingDeps && depsReady === false) {
      DownloadDependencies()
        .then(() => {
          setDepsReady(true)
          setInstallingDeps(false)
        })
        .catch((err: any) => {
          setDepError(err?.message || 'Dependency installation failed')
          setInstallingDeps(false)
        })
    }
  }, [installingDeps, depsReady])

  // Listen for download progress
  useEffect(() => {
    const handleDlProgress = (data: DownloadProgress) => {
      setQueue((prev) =>
        prev.map((item) => {
          if (item.id !== data.downloadId) return item
          if (data.status === 'completed') return { ...item, status: 'completed', progress: 100, playlistStatus: '' }
          if (data.status === 'error') {
            const msg = data.error || 'Download failed'
            return { ...item, status: 'error', progress: 0, speed: '', eta: '', errorMsg: msg, playlistStatus: '' }
          }
          if (data.status === 'cancelled') return { ...item, status: 'cancelled', progress: 0, speed: '', eta: '', playlistStatus: '' }
          if (data.status === 'starting') return { ...item, status: 'starting', progress: 0, playlistStatus: data.playlistStatus || '' }
          return {
            ...item, status: data.status,
            progress: Math.round(data.percent), speed: data.speed, eta: data.eta,
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
      const downloadId = await StartDownload(url, selectedFormat, defaultOutputDir)
      setQueue((prev) => [
        ...prev,
        { id: downloadId, title: metadata.title, thumbnail: metadata.thumbnail, status: 'queued', progress: 0, speed: '', eta: '', errorMsg: '', playlistStatus: '' },
      ])
    } catch (err: any) {
      console.error('Failed to start download:', err)
    }
  }

  const handleCancel = (id: string) => {
    CancelDownload(id).catch(() => {})
  }

  const handleClearCompleted = () => {
    setQueue((prev) => prev.filter((item) => !['completed', 'error', 'cancelled'].includes(item.status)))
  }

  const handleChangeFolder = async () => {
    try {
      const dir = await SelectDirectory()
      if (!dir) return
      setDefaultOutputDir(dir)
      await UpdateSettings({ defaultOutputDir: dir })
    } catch {
      // user cancelled
    }
  }

  const statusColors: Record<string, string> = {
    downloading: 'text-accent', starting: 'text-accent',
    queued: 'text-gray-400', completed: 'text-green-400',
    error: 'text-red-400', cancelled: 'text-yellow-400',
  }

  // Loading screen
  if (checkingDeps) {
    return (
      <div className="h-screen flex flex-col items-center justify-center bg-surface text-gray-200">
        <div className="w-8 h-8 rounded-md bg-accent flex items-center justify-center mb-4">
          <span className="text-black font-bold text-sm">KP</span>
        </div>
        <h1 className="text-lg font-semibold tracking-tight mb-1">KoalaPull</h1>
        <p className="text-sm text-gray-500">Checking dependencies...</p>
        <div className="mt-4 w-6 h-6 border-2 border-accent border-t-transparent rounded-full animate-spin" />
      </div>
    )
  }

  // Setup screen
  if (!depsReady) {
    return (
      <div className="h-screen flex flex-col items-center justify-center bg-surface text-gray-200 px-6">
        <div className="w-12 h-12 rounded-xl bg-accent flex items-center justify-center mb-5">
          <span className="text-black font-bold text-lg">KP</span>
        </div>
        <h1 className="text-xl font-semibold tracking-tight mb-1">Setting up KoalaPull</h1>
        <p className="text-sm text-gray-400 mb-6 text-center max-w-sm">
          Downloading required tools to your application data directory.
        </p>
        <div className="w-full max-w-sm space-y-4">
          {['yt-dlp', 'ffmpeg'].map((dep) => {
            const pct = depProgress[dep] ?? 0
            return (
              <div key={dep}>
                <div className="flex items-center justify-between text-xs mb-1">
                  <span className="text-gray-400">{dep}</span>
                  <span className="text-gray-500 font-mono">{pct}%</span>
                </div>
                <div className="w-full h-1.5 bg-surface-lighter rounded-full overflow-hidden">
                  <div className="h-full bg-accent rounded-full transition-all duration-300 ease-out" style={{ width: `${pct}%` }} />
                </div>
              </div>
            )
          })}
        </div>
        {installingDeps && (
          <p className="text-xs text-gray-500 mt-4 font-mono">
            {depProgress['yt-dlp'] === 100 && depProgress['ffmpeg'] === 100 ? 'Finalizing...' : 'Downloading...'}
          </p>
        )}
        {depError && (
          <div className="mt-4 text-center">
            <p className="text-xs text-red-400 mb-2">{depError}</p>
            <button onClick={() => setInstallingDeps(true)} className="btn-primary text-xs px-4 py-1.5">Retry</button>
          </div>
        )}
      </div>
    )
  }

  return (
    <div className="h-screen flex flex-col bg-surface text-gray-200 select-none">
      {/* Header */}
      <header className="flex items-center justify-between px-6 py-3 border-b border-surface-border shrink-0">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded-md bg-accent flex items-center justify-center">
            <span className="text-black font-bold text-sm">KP</span>
          </div>
          <h1 className="text-lg font-semibold tracking-tight">KoalaPull</h1>
        </div>
        <div className="flex items-center gap-3">
          <span className="text-xs text-gray-500 font-mono">{ytdlpVersion ? `v${ytdlpVersion}` : ''}</span>
          <button
            onClick={() => setShowSettings(true)}
            className="text-gray-400 hover:text-gray-200 transition-colors"
            title="Settings"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
            </svg>
          </button>
        </div>
      </header>

      {/* Settings Modal */}
      {showSettings && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60" onClick={() => setShowSettings(false)}>
          <div
            className="bg-surface-light border border-surface-border rounded-lg w-full max-w-md mx-4 p-5"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-sm font-semibold text-gray-200">Settings</h2>
              <button onClick={() => setShowSettings(false)} className="text-gray-500 hover:text-gray-300 text-lg leading-none">&times;</button>
            </div>

            <label className="text-xs text-gray-400 mb-1.5 block">Download Location</label>
            <div className="flex gap-2">
              <div className="flex-1 bg-surface-lighter border border-surface-border rounded-md px-3 py-2 text-xs text-gray-300 truncate">
                {defaultOutputDir}
              </div>
              <button onClick={handleChangeFolder} className="btn-primary text-xs px-3 py-1.5 shrink-0">
                Change
              </button>
            </div>

            <div className="mt-5 flex justify-end">
              <button onClick={() => setShowSettings(false)} className="text-xs text-gray-400 hover:text-gray-200 px-3 py-1.5">
                Close
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Main Content */}
      <main className="flex-1 overflow-y-auto px-6 py-5 space-y-5">
        {/* URL Fetch */}
        <div className="flex gap-2">
          <div className="flex-1 relative">
            <input
              type="text"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') handleFetch() }}
              placeholder="Paste a video or playlist URL..."
              className="input-dark w-full pr-10"
            />
            {url && (
              <button
                onClick={() => setUrl('')}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-300 text-lg leading-none"
              >
                &times;
              </button>
            )}
          </div>
          <button
            onClick={handleFetch}
            disabled={fetching || !url.trim()}
            className="btn-primary shrink-0 flex items-center gap-2"
          >
            {fetching ? (
              <>
                <svg className="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                </svg>
                Fetching
              </>
            ) : 'Fetch'}
          </button>
        </div>

        {fetchError && (
          <div className="bg-red-900/20 border border-red-700/30 rounded-lg px-4 py-3 text-sm text-red-300">
            {fetchError}
          </div>
        )}

        {/* Rich Metadata Card */}
        {fetched && metadata && (
          <div className="bg-surface-light border border-surface-border rounded-lg overflow-hidden">
            <div className="flex gap-4 p-4">
              {metadata.thumbnail ? (
                <img src={metadata.thumbnail} alt={metadata.title} className="w-44 h-24 rounded-md object-cover shrink-0 bg-surface-lighter" />
              ) : (
                <div className="w-44 h-24 bg-surface-lighter rounded-md shrink-0 flex items-center justify-center border border-surface-border">
                  <svg className="w-8 h-8 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M15 10l4.553-2.276A1 1 0 0121 8.618v6.764a1 1 0 01-1.447.894L15 14M5 18h8a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z" />
                  </svg>
                </div>
              )}
              <div className="flex-1 min-w-0">
                <h2 className="text-base font-semibold truncate">{metadata.title}</h2>
                <p className="text-sm text-gray-400 mt-0.5">{metadata.uploader}</p>
                {metadata.isPlaylist && (
                  <span className="inline-block mt-1 text-xs bg-accent/20 text-accent px-2 py-0.5 rounded font-medium">
                    Playlist · {metadata.entryCount} videos
                  </span>
                )}
                <div className="flex flex-wrap gap-2 mt-3">
                  <select value={selectedFormat} onChange={(e) => setSelectedFormat(e.target.value)} className="select-dark text-xs flex-1 min-w-[140px]">
                    {formatOptions.map((opt) => (
                      <option key={opt.formatId} value={opt.formatId}>{opt.label}</option>
                    ))}
                  </select>
                  <select className="select-dark text-xs flex-1 min-w-[100px] disabled:opacity-40 disabled:cursor-not-allowed" defaultValue="none" disabled={metadata.isPlaylist} title={metadata.isPlaylist ? 'Subtitles not supported for playlists' : ''}>
                    <option value="none">No Subs</option>
                    <option value="auto">Auto-generated</option>
                    <option value="embed">Embed All</option>
                  </select>
                  <select className="select-dark text-xs flex-1 min-w-[80px]" defaultValue="mp4">
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

        {/* Download Queue */}
        <div>
          <div className="flex items-center justify-between mb-3">
            <h3 className="text-sm font-medium text-gray-400 uppercase tracking-wider">
              Downloads ({queue.length})
            </h3>
            {queue.some((i) => ['completed', 'error', 'cancelled'].includes(i.status)) && (
              <button onClick={handleClearCompleted} className="text-xs text-gray-500 hover:text-gray-300 transition-colors">
                Clear Completed
              </button>
            )}
          </div>
          {queue.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12 text-gray-600">
              <svg className="w-12 h-12 mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M7 16V4m0 0L3 8m4-4l4 4m6 0v12m0 0l4-4m-4 4l-4-4" />
              </svg>
              <p className="text-sm">No downloads yet</p>
              <p className="text-xs mt-1">Paste a URL above to get started</p>
            </div>
          ) : (
            <div className="space-y-2">
              {[...queue].reverse().map((item) => (
                <div key={item.id} className="bg-surface-light border border-surface-border rounded-lg p-3 flex items-center gap-3">
                  <div className="w-16 h-10 bg-surface-lighter rounded shrink-0 flex items-center justify-center border border-surface-border overflow-hidden">
                    {item.thumbnail ? (
                      <img src={item.thumbnail} alt="" className="w-full h-full object-cover" />
                    ) : (
                      <svg className="w-5 h-5 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
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
                      {item.speed && <span className="text-gray-500">{item.speed}</span>}
                      {item.eta && <span className="text-gray-500">ETA: {item.eta}</span>}
                      {item.playlistStatus && <span className="text-gray-500">{item.playlistStatus}</span>}
                    </div>

                    {/* Progress Bar */}
                    {(item.status === 'downloading' || item.status === 'starting') && (
                      <div className="mt-1.5 w-full h-1 bg-surface-lighter rounded-full overflow-hidden">
                        <div className="h-full bg-accent rounded-full transition-all duration-300 ease-out" style={{ width: `${Math.max(item.progress, 2)}%` }} />
                      </div>
                    )}
                    {item.status === 'completed' && (
                      <div className="mt-1.5 w-full h-1 bg-surface-lighter rounded-full overflow-hidden">
                        <div className="h-full bg-green-400 rounded-full" style={{ width: '100%' }} />
                      </div>
                    )}
                    {item.status === 'error' && (
                      <>
                        <div className="mt-1.5 w-full h-1 bg-surface-lighter rounded-full overflow-hidden">
                          <div className="h-full bg-red-500 rounded-full" style={{ width: '100%' }} />
                        </div>
                        {item.errorMsg && (
                          <p className="mt-1 text-xs text-red-400 truncate">{item.errorMsg}</p>
                        )}
                      </>
                    )}
                    {item.status === 'cancelled' && (
                      <div className="mt-1.5 w-full h-1 bg-surface-lighter rounded-full overflow-hidden">
                        <div className="h-full bg-yellow-500 rounded-full" style={{ width: '100%' }} />
                      </div>
                    )}
                  </div>

                  {item.status === 'completed' && (
                    <svg className="w-5 h-5 text-green-400 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                    </svg>
                  )}
                  {(item.status === 'downloading' || item.status === 'starting') && (
                    <div className="flex items-center gap-1">
                      <svg className="w-5 h-5 text-accent animate-pulse shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 14l-7 7m0 0l-7-7m7 7V3" />
                      </svg>
                      <button onClick={() => handleCancel(item.id)} className="text-gray-500 hover:text-red-400 transition-colors p-0.5" title="Cancel">
                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                        </svg>
                      </button>
                    </div>
                  )}
                  {item.status === 'queued' && (
                    <button onClick={() => handleCancel(item.id)} className="text-gray-500 hover:text-red-400 transition-colors p-0.5 shrink-0" title="Cancel">
                      <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                      </svg>
                    </button>
                  )}
                  {item.status === 'error' && (
                    <svg className="w-5 h-5 text-red-400 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L4.082 16.5c-.77.833.192 2.5 1.732 2.5z" />
                    </svg>
                  )}
                  {item.status === 'cancelled' && (
                    <svg className="w-5 h-5 text-yellow-400 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                    </svg>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      </main>

      {/* Footer */}
      <footer className="flex items-center justify-between px-6 py-2 border-t border-surface-border bg-surface-light shrink-0 text-xs text-gray-500">
        <div className="flex items-center gap-2">
          <span className="inline-block w-2 h-2 rounded-full bg-green-400" />
          <span>Ready</span>
        </div>
        <div className="flex items-center gap-3">
          <span className="text-gray-600">Save to: {defaultOutputDir ? defaultOutputDir.split('\\').pop()?.split('/').pop() : '...'}</span>
          <button className="flex items-center gap-1.5 hover:text-gray-300 transition-colors">
            <span>yt-dlp</span>
            {ytdlpVersion && <span className="font-mono">v{ytdlpVersion}</span>}
          </button>
        </div>
      </footer>
    </div>
  )
}

export default App
