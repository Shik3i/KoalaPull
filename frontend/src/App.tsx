import { useState } from 'react'
import './style.css'

interface QueueItem {
  id: string
  title: string
  thumbnail: string
  status: 'queued' | 'downloading' | 'completed' | 'error'
  progress: number
  speed: string
  eta: string
}

function App() {
  const [url, setUrl] = useState('')
  const [fetching, setFetching] = useState(false)
  const [fetched, setFetched] = useState(false)
  const [ytdlpVersion, setYtdlpVersion] = useState('2025.03.21')
  const [depsReady, setDepsReady] = useState(true)
  const [queue, setQueue] = useState<QueueItem[]>([
    {
      id: '1',
      title: 'Example Video Title - Full HD 1080p',
      thumbnail: '',
      status: 'downloading',
      progress: 65,
      speed: '4.2 MB/s',
      eta: '00:00:45',
    },
    {
      id: '2',
      title: 'Another Cool Video Name Here',
      thumbnail: '',
      status: 'queued',
      progress: 0,
      speed: '',
      eta: '',
    },
    {
      id: '3',
      title: 'Completed Download Example',
      thumbnail: '',
      status: 'completed',
      progress: 100,
      speed: '',
      eta: '',
    },
  ])

  const handleFetch = () => {
    if (!url.trim()) return
    setFetching(true)
    setTimeout(() => {
      setFetching(false)
      setFetched(true)
    }, 1500)
  }

  const handleAddToQueue = () => {
    setQueue((prev) => [
      ...prev,
      {
        id: String(Date.now()),
        title: 'New Download from URL',
        thumbnail: '',
        status: 'queued',
        progress: 0,
        speed: '',
        eta: '',
      },
    ])
  }

  const statusColors: Record<QueueItem['status'], string> = {
    downloading: 'text-accent',
    queued: 'text-gray-400',
    completed: 'text-green-400',
    error: 'text-red-400',
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
        <span className="text-xs text-gray-500 font-mono">v{ytdlpVersion}</span>
      </header>

      {/* Main Content - Scrollable */}
      <main className="flex-1 overflow-y-auto px-6 py-5 space-y-5">
        {/* Dependency Check Banner */}
        {!depsReady && (
          <div className="flex items-center gap-3 bg-yellow-900/20 border border-yellow-700/30 rounded-lg px-4 py-3 text-sm text-yellow-300">
            <svg className="w-5 h-5 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L4.082 16.5c-.77.833.192 2.5 1.732 2.5z" />
            </svg>
            <span>Downloading yt-dlp &amp; ffmpeg...</span>
          </div>
        )}

        {/* URL Fetch Area */}
        <div className="flex gap-2">
          <div className="flex-1 relative">
            <input
              type="text"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
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
            ) : (
              'Fetch'
            )}
          </button>
        </div>

        {/* Rich Metadata Card */}
        {fetched && (
          <div className="bg-surface-light border border-surface-border rounded-lg overflow-hidden">
            <div className="flex gap-4 p-4">
              {/* Thumbnail Placeholder */}
              <div className="w-44 h-24 bg-surface-lighter rounded-md shrink-0 flex items-center justify-center border border-surface-border">
                <svg className="w-8 h-8 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M15 10l4.553-2.276A1 1 0 0121 8.618v6.764a1 1 0 01-1.447.894L15 14M5 18h8a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z" />
                </svg>
              </div>

              {/* Info & Config */}
              <div className="flex-1 min-w-0">
                <h2 className="text-base font-semibold truncate">Video Title Appears Here - 4K HDR Example</h2>
                <p className="text-sm text-gray-400 mt-0.5">Channel Name</p>

                <div className="flex flex-wrap gap-2 mt-3">
                  <select className="select-dark text-xs flex-1 min-w-[120px]">
                    <option>Best Video + Audio</option>
                    <option>1080p Only</option>
                    <option>720p Only</option>
                    <option>Audio Only</option>
                  </select>
                  <select className="select-dark text-xs flex-1 min-w-[100px]">
                    <option>None</option>
                    <option>Auto-generated</option>
                    <option>Embed All</option>
                  </select>
                  <select className="select-dark text-xs flex-1 min-w-[80px]">
                    <option>MP4</option>
                    <option>MKV</option>
                    <option>MP3</option>
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
          <h3 className="text-sm font-medium text-gray-400 uppercase tracking-wider mb-3">
            Downloads ({queue.length})
          </h3>

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
              {queue.map((item) => (
                <div
                  key={item.id}
                  className="bg-surface-light border border-surface-border rounded-lg p-3 flex items-center gap-3"
                >
                  {/* Thumbnail */}
                  <div className="w-16 h-10 bg-surface-lighter rounded shrink-0 flex items-center justify-center border border-surface-border">
                    <svg className="w-5 h-5 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                  </div>

                  {/* Info */}
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-medium truncate">{item.title}</p>
                    <div className="flex items-center gap-3 mt-1 text-xs">
                      <span className={statusColors[item.status]}>
                        {item.status === 'downloading' && `Downloading - ${item.progress}%`}
                        {item.status === 'queued' && 'Queued'}
                        {item.status === 'completed' && 'Completed'}
                        {item.status === 'error' && 'Error'}
                      </span>
                      {item.speed && <span className="text-gray-500">{item.speed}</span>}
                      {item.eta && <span className="text-gray-500">ETA: {item.eta}</span>}
                    </div>

                    {/* Progress Bar (active only) */}
                    {item.status === 'downloading' && (
                      <div className="mt-1.5 w-full h-1 bg-surface-lighter rounded-full overflow-hidden">
                        <div
                          className="h-full bg-accent rounded-full transition-all duration-300 ease-out"
                          style={{ width: `${item.progress}%` }}
                        />
                      </div>
                    )}
                    {item.status === 'completed' && (
                      <div className="mt-1.5 w-full h-1 bg-surface-lighter rounded-full overflow-hidden">
                        <div className="h-full bg-green-400 rounded-full" style={{ width: '100%' }} />
                      </div>
                    )}
                  </div>

                  {/* Status Icon */}
                  {item.status === 'completed' && (
                    <svg className="w-5 h-5 text-green-400 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                    </svg>
                  )}
                  {item.status === 'downloading' && (
                    <svg className="w-5 h-5 text-accent animate-pulse shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 14l-7 7m0 0l-7-7m7 7V3" />
                    </svg>
                  )}
                  {item.status === 'queued' && (
                    <svg className="w-5 h-5 text-gray-500 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      </main>

      {/* Footer Status Bar */}
      <footer className="flex items-center justify-between px-6 py-2 border-t border-surface-border bg-surface-light shrink-0 text-xs text-gray-500">
        <div className="flex items-center gap-2">
          <span className={`inline-block w-2 h-2 rounded-full ${depsReady ? 'bg-green-400' : 'bg-yellow-400'}`} />
          <span>{depsReady ? 'Ready' : 'Setting up dependencies...'}</span>
        </div>
        <button className="flex items-center gap-1.5 hover:text-gray-300 transition-colors">
          <span>yt-dlp</span>
          <span className="font-mono">v{ytdlpVersion}</span>
          <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
        </button>
      </footer>
    </div>
  )
}

export default App
