import appIcon from '../assets/images/app-icon.png'
import { formatBytes } from '../lib/downloadMetrics'

export interface DepProgress {
  dependency: string
  progress: number
  status: string
  error?: string
  bytesTotal?: number
  bytesRead?: number
  speed?: string
  eta?: string
}

interface SetupScreenProps {
  checkingDeps: boolean
  depsReady: boolean
  installingDeps: boolean
  depProgress: Record<string, DepProgress>
  depError: string
  setDepError: (err: string) => void
  setDepProgress: (progress: Record<string, DepProgress>) => void
  setInstallingDeps: (installing: boolean) => void
  t: (key: string, params?: Record<string, string | number>) => string
}

function AppLogo({ sizeClass }: { sizeClass: string }) {
  return (
    <div className={`${sizeClass} shrink-0 flex items-center justify-center overflow-hidden`}>
      <img src={appIcon} alt="" className="w-full h-full object-cover" draggable={false} />
    </div>
  )
}

export function SetupScreen({
  checkingDeps,
  depsReady,
  installingDeps,
  depProgress,
  depError,
  setDepError,
  setDepProgress,
  setInstallingDeps,
  t,
}: SetupScreenProps) {
  if (checkingDeps) {
    return (
      <div className="h-screen flex flex-col items-center justify-center px-6" style={{ background: 'var(--color-surface)', color: 'var(--text-primary)' }}>
        <AppLogo sizeClass="w-28 h-28 mb-4" />
        <h1 className="text-lg font-semibold tracking-tight mb-1">{t('app.name')}</h1>
        <p style={{ color: 'var(--text-muted)', fontSize: '0.875rem' }}>{t('setup.checkingDependencies')}</p>
        <div className="mt-4 w-6 h-6 border-2 rounded-full animate-spin" style={{ borderColor: 'var(--color-accent)', borderTopColor: 'transparent' }} />
      </div>
    )
  }

  if (depsReady) {
    return null
  }

  return (
    <div className="h-screen flex flex-col items-center justify-center px-6" style={{ background: 'var(--color-surface)', color: 'var(--text-primary)' }}>
      <AppLogo sizeClass="w-48 h-48 mb-6" />
      <h1 className="text-xl font-semibold tracking-tight mb-1">{t('setup.title')}</h1>
      <p className="text-sm mb-6 text-center max-w-sm" style={{ color: 'var(--text-secondary)' }}>
        {t('setup.description')}
      </p>
      <div className="w-full max-w-sm space-y-4">
        {['yt-dlp', 'ffmpeg'].map((dep) => {
          const progressObj = depProgress[dep]
          const pct = progressObj?.progress ?? 0
          const eta = progressObj?.eta
          const speed = progressObj?.speed
          const bytesRead = progressObj?.bytesRead
          const bytesTotal = progressObj?.bytesTotal

          let progressLabel = `${pct}%`
          if (speed || eta) {
            const parts: string[] = []
            if (speed) parts.push(speed)
            if (eta) parts.push(eta)
            progressLabel = `${pct}% (${parts.join(', ')})`
          }

          let bytesLabel = ''
          if (bytesRead && bytesTotal) {
            bytesLabel = `${formatBytes(bytesRead)} / ${formatBytes(bytesTotal)}`
          }

          return (
            <div key={dep}>
              <div className="flex items-center justify-between text-xs mb-1">
                <span style={{ color: 'var(--text-secondary)' }}>{dep}</span>
                <span className="font-mono" style={{ color: 'var(--text-muted)' }}>
                  {bytesLabel ? `${bytesLabel} · ` : ''}{progressLabel}
                </span>
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
          {depProgress['yt-dlp']?.progress === 100 && depProgress['ffmpeg']?.progress === 100
            ? t('setup.finalizing')
            : t('setup.downloading')}
        </p>
      ) : !depError ? (
        <button
          onClick={() => {
            setDepError('')
            setDepProgress({})
            setInstallingDeps(true)
          }}
          className="btn-primary text-sm px-5 py-2 mt-2"
        >
          {t('actions.downloadInstall')}
        </button>
      ) : null}
      {depError && (
        <div className="mt-4 text-center">
          <p className="text-xs mb-2" style={{ color: '#f87171' }}>{depError}</p>
          <button
            onClick={() => {
              setDepError('')
              setDepProgress({})
              setInstallingDeps(true)
            }}
            className="btn-primary text-xs px-4 py-1.5"
          >
            {t('app.retry')}
          </button>
        </div>
      )}
    </div>
  )
}
