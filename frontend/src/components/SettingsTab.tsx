import {
  SelectCookieFile,
  SelectFfmpegPath,
  OpenBinDir,
  OpenExternalLink,
  UpdateDependencies,
} from '../../wailsjs/go/main/App'
import type { main } from '../../wailsjs/go/models'

export type DownloadPreset = 'best' | 'compatible' | 'audio' | 'custom'
export type LanguageCode = 'en' | 'de' | 'fr'

export interface AppSettings {
  defaultOutputDir: string
  theme: string
  maxConcurrency: number
  autoPasteURL: boolean
  language: LanguageCode
  downloadPreset: DownloadPreset
  customFormatId: string
  customContainer: string
  customSubtitle: string
  cookieSource: 'none' | 'browser' | 'file'
  cookieBrowser: string
  cookieFilePath: string
  cookieCachePath: string
  cookieCacheBrowser: string
  cookieCacheUpdated: string
  rateLimitEnabled: boolean
  rateLimitValue: string
  safeModeEnabled: boolean
  customArgs: string
  ffmpegPath: string
  sponsorBlockEnabled: boolean
}

export interface VersionInfo {
  ytdlp: string
  ffmpeg: string
  app: string
}

export interface UpdateInfo {
  ytdlpUpdateAvailable: boolean
  latestYtdlpVersion: string
  koalaPullUpdateAvailable: boolean
  latestKoalaPullVersion: string
  ffmpegUpdateAvailable: boolean
  latestFfmpegVersion: string
}

interface SettingsTabProps {
  showAdvanced: boolean
  setShowAdvanced: (val: boolean) => void
  settingsError: string
  theme: string
  handleThemeChange: (theme: 'dark' | 'light' | 'system') => void
  language: LanguageCode
  handleLanguageChange: (lang: LanguageCode) => void
  defaultOutputDir: string
  handleChangeFolder: () => void
  autoPasteEnabled: boolean
  setAutoPasteEnabled: (val: boolean) => void
  rateLimitEnabled: boolean
  setRateLimitEnabled: (val: boolean) => void
  rateLimitValue: string
  setRateLimitValue: (val: string) => void
  sponsorBlockEnabled: boolean
  setSponsorBlockEnabled: (val: boolean) => void
  maxConcurrency: number
  setMaxConcurrency: (val: number) => void
  cookieSource: 'none' | 'browser' | 'file'
  setCookieSource: (val: 'none' | 'browser' | 'file') => void
  cookieBrowser: string
  setCookieBrowser: (val: string) => void
  cookieFilePath: string
  setCookieFilePath: (val: string) => void
  browserRunning: boolean
  browserError: string
  isCheckingBrowser: boolean
  handleKillBrowser: () => void
  handleCloseBrowserAndFetch: () => void
  ffmpegPath: string
  setFfmpegPath: (val: string) => void
  safeModeEnabled: boolean
  setSafeModeEnabled: (val: boolean) => void
  customArgs: string
  setCustomArgs: (val: string) => void
  appVersion: string
  toolVersionsLoading: boolean
  toolVersions: VersionInfo | null
  updateInfo: UpdateInfo | null
  updateLoading: boolean
  updatingDeps: boolean
  setUpdatingDeps: (val: boolean) => void
  updatesError: string
  setUpdatesError: (val: string) => void
  loadToolVersions: () => Promise<void>
  loadUpdateInfo: () => Promise<void>
  saveSettings: (s: Partial<AppSettings>) => Promise<void>
  t: (key: string, params?: Record<string, string | number>) => string
  tt: (key: string, params?: Record<string, string | number>) => string
}

function formatAppVersionLabel(version: string): string {
  if (!version) return '...'
  if (version === 'dev' || version.startsWith('v')) return version
  return `v${version}`
}

function isChromiumBrowser(b: string) {
  const low = b.toLowerCase()
  return ['chrome', 'edge', 'brave', 'vivaldi', 'opera', 'chromium', 'whale'].includes(low)
}

export function SettingsTab({
  showAdvanced,
  setShowAdvanced,
  settingsError,
  theme,
  handleThemeChange,
  language,
  handleLanguageChange,
  defaultOutputDir,
  handleChangeFolder,
  autoPasteEnabled,
  setAutoPasteEnabled,
  rateLimitEnabled,
  setRateLimitEnabled,
  rateLimitValue,
  setRateLimitValue,
  sponsorBlockEnabled,
  setSponsorBlockEnabled,
  maxConcurrency,
  setMaxConcurrency,
  cookieSource,
  setCookieSource,
  cookieBrowser,
  setCookieBrowser,
  cookieFilePath,
  setCookieFilePath,
  browserRunning,
  browserError,
  isCheckingBrowser,
  handleKillBrowser,
  handleCloseBrowserAndFetch,
  ffmpegPath,
  setFfmpegPath,
  safeModeEnabled,
  setSafeModeEnabled,
  customArgs,
  setCustomArgs,
  appVersion,
  toolVersionsLoading,
  toolVersions,
  updateInfo,
  updateLoading,
  updatingDeps,
  setUpdatingDeps,
  updatesError,
  setUpdatesError,
  loadToolVersions,
  loadUpdateInfo,
  saveSettings,
  t,
  tt,
}: SettingsTabProps) {
  return (
    <div className="flex-1 flex flex-col overflow-hidden">
      <div className="px-4 lg:px-8 py-4 lg:py-5 shrink-0 flex items-center justify-between gap-4">
        <h2 className="text-base lg:text-lg font-semibold" title={tt('languageSelect')}>
          {t('settings.title')}
        </h2>
        <div className="flex items-center gap-2 shrink-0">
          <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
            {t('settings.advanced')}
          </span>
          <button
            role="switch"
            aria-checked={showAdvanced}
            onClick={() => setShowAdvanced(!showAdvanced)}
            className="relative w-10 h-5 rounded-full transition-colors shrink-0"
            style={{
              background: showAdvanced ? 'var(--color-accent)' : 'var(--color-surface-border)',
            }}
            title={showAdvanced ? tt('settingsHideAdvanced') : tt('settingsShowAdvanced')}
            aria-label={showAdvanced ? tt('settingsHideAdvanced') : tt('settingsShowAdvanced')}
          >
            <span
              className="absolute top-0.5 w-4 h-4 rounded-full bg-white transition-transform"
              style={{ left: '2px', transform: showAdvanced ? 'translateX(20px)' : 'translateX(0)' }}
            />
          </button>
        </div>
      </div>
      <div className="flex-1 overflow-y-auto px-4 lg:px-8 py-5 lg:py-6">
        {settingsError && (
          <div role="alert" className="mb-4 p-3 rounded-lg border text-xs text-red-400" style={{ borderColor: '#ef4444' }}>
            {settingsError}
          </div>
        )}
        <div className="grid gap-6 grid-cols-1 md:grid-cols-2 xl:grid-cols-3 items-start">
          {/* Theme */}
          <section
            className="rounded-xl p-4 border"
            style={{ background: 'var(--color-surface-light)', borderColor: 'var(--color-surface-border)' }}
          >
            <h3 className="text-sm font-medium mb-3" style={{ color: 'var(--text-secondary)' }} title={tt('themeDark')}>
              {t('settings.appearance')}
            </h3>
            <div className="flex gap-3">
              {(['dark', 'light', 'system'] as const).map((themeOption) => (
                <button
                  key={themeOption}
                  onClick={() => handleThemeChange(themeOption)}
                  className="flex-1 rounded-lg py-3 px-4 text-xs font-medium transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
                  title={
                    themeOption === 'dark'
                      ? tt('themeDark')
                      : themeOption === 'light'
                      ? tt('themeLight')
                      : tt('themeSystem')
                  }
                  aria-label={
                    themeOption === 'dark'
                      ? tt('themeDark')
                      : themeOption === 'light'
                      ? tt('themeLight')
                      : tt('themeSystem')
                  }
                  aria-pressed={theme === themeOption}
                  style={{
                    background:
                      theme === themeOption ? 'color-mix(in srgb, var(--color-accent) 20%, transparent)' : 'var(--color-surface-lighter)',
                    border: theme === themeOption ? '1px solid var(--color-accent)' : '1px solid var(--color-surface-border)',
                    color: theme === themeOption ? 'var(--color-accent)' : 'var(--text-secondary)',
                  }}
                >
                  {themeOption === 'dark'
                    ? `\u{1F312} ${t('settings.themeDark')}`
                    : themeOption === 'light'
                    ? `\u{2600} ${t('settings.themeLight')}`
                    : `\u{2699} ${t('settings.themeSystem')}`}
                </button>
              ))}
            </div>
          </section>

          <section
            className="rounded-xl p-4 border"
            style={{ background: 'var(--color-surface-light)', borderColor: 'var(--color-surface-border)' }}
          >
            <label
              htmlFor="languageSelect"
              className="block text-sm font-medium mb-3"
              style={{ color: 'var(--text-secondary)' }}
              title={tt('languageSelect')}
            >
              {t('settings.language')}
            </label>
            <select
              id="languageSelect"
              value={language}
              onChange={(e) => {
                void handleLanguageChange(e.target.value as LanguageCode)
              }}
              className="select-dark text-sm w-full"
              title={tt('languageSelect')}
              aria-label={tt('languageSelect')}
            >
              <option value="en">🇺🇸 {t('settings.languageEnglish')}</option>
              <option value="de">🇩🇪 {t('settings.languageGerman')}</option>
              <option value="fr">🇫🇷 {t('settings.languageFrench')}</option>
            </select>
          </section>

          {/* Download Location */}
          <section
            className="rounded-xl p-4 border"
            style={{ background: 'var(--color-surface-light)', borderColor: 'var(--color-surface-border)' }}
          >
            <h3 className="text-sm font-medium mb-3" style={{ color: 'var(--text-secondary)' }} title={tt('downloadLocation')}>
              {t('settings.downloadLocation')}
            </h3>
            <div className="flex gap-2">
              <div
                className="flex-1 rounded-md px-3 py-2 text-xs leading-5 break-all whitespace-normal"
                style={{
                  background: 'var(--color-surface-lighter)',
                  border: '1px solid var(--color-surface-border)',
                  color: 'var(--text-secondary)',
                }}
                title={tt('downloadLocation')}
              >
                {defaultOutputDir}
              </div>
              <button
                onClick={handleChangeFolder}
                className="btn-primary text-xs px-3 py-1.5 shrink-0"
                title={tt('changeFolder')}
                aria-label={tt('changeFolder')}
              >
                {t('actions.change')}
              </button>
            </div>
          </section>

          {/* Auto-Paste URL */}
          <section
            className="rounded-xl p-4 border"
            style={{ background: 'var(--color-surface-light)', borderColor: 'var(--color-surface-border)' }}
          >
            <div className="flex items-center justify-between">
              <div>
                <h3 className="text-sm font-medium" style={{ color: 'var(--text-secondary)' }} title={tt('autoPaste')}>
                  {t('settings.autoPasteTitle')}
                </h3>
                <p className="text-xs mt-0.5" style={{ color: 'var(--text-muted)' }} title={tt('autoPaste')}>
                  {t('settings.autoPasteDescription')}
                </p>
              </div>
              <button
                role="switch"
                aria-checked={autoPasteEnabled}
                onClick={async () => {
                  const next = !autoPasteEnabled
                  setAutoPasteEnabled(next)
                  try {
                    await saveSettings({ autoPasteURL: next })
                  } catch (err) {
                    console.warn('UpdateSettings failed:', err)
                  }
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

          {/* Speed Limit */}
          <section
            className="rounded-xl p-4 border"
            style={{ background: 'var(--color-surface-light)', borderColor: 'var(--color-surface-border)' }}
          >
            <div className="flex items-center justify-between mb-3">
              <div>
                <h3
                  className="text-sm font-medium"
                  style={{ color: 'var(--text-secondary)' }}
                  title={t('settings.speedLimitTitle')}
                >
                  {t('settings.speedLimitTitle')}
                </h3>
                <p
                  className="text-xs mt-0.5"
                  style={{ color: 'var(--text-muted)' }}
                  title={t('settings.speedLimitDescription')}
                >
                  {t('settings.speedLimitDescription')}
                </p>
              </div>
              <button
                role="switch"
                aria-checked={rateLimitEnabled}
                onClick={async () => {
                  const next = !rateLimitEnabled
                  setRateLimitEnabled(next)
                  try {
                    await saveSettings({ rateLimitEnabled: next })
                  } catch (err) {
                    console.warn('UpdateSettings failed:', err)
                  }
                }}
                className="relative w-10 h-5 rounded-full transition-colors shrink-0"
                style={{
                  background: rateLimitEnabled ? 'var(--color-accent)' : 'var(--color-surface-border)',
                }}
                title={t('settings.speedLimitTitle')}
                aria-label={t('settings.speedLimitTitle')}
              >
                <span
                  className="absolute top-0.5 w-4 h-4 rounded-full bg-white transition-transform"
                  style={{ left: '2px', transform: rateLimitEnabled ? 'translateX(20px)' : 'translateX(0)' }}
                />
              </button>
            </div>
            <div className="flex items-center gap-3">
              <label htmlFor="rateLimitValueInput" className="text-xs" style={{ color: 'var(--text-muted)', minWidth: '7rem' }}>
                {t('settings.speedLimitLabel')}
              </label>
              <input
                id="rateLimitValueInput"
                type="text"
                disabled={!rateLimitEnabled}
                value={rateLimitValue}
                onChange={async (e) => {
                  const val = e.target.value
                  setRateLimitValue(val)
                  try {
                    await saveSettings({ rateLimitValue: val })
                  } catch (err) {
                    console.warn('UpdateSettings failed:', err)
                  }
                }}
                className="input-dark text-xs w-20 text-center"
                placeholder="1"
                title={t('settings.speedLimitLabel')}
                aria-label={t('settings.speedLimitLabel')}
              />
            </div>
          </section>

          {/* SponsorBlock */}
          <section
            className="rounded-xl p-4 border"
            style={{ background: 'var(--color-surface-light)', borderColor: 'var(--color-surface-border)' }}
          >
            <div className="flex items-center justify-between">
              <div>
                <h3 className="text-sm font-medium" style={{ color: 'var(--text-secondary)' }} title={tt('sponsorBlockTitle')}>
                  {t('settings.sponsorBlockTitle')}
                </h3>
                <p className="text-xs mt-0.5" style={{ color: 'var(--text-muted)' }} title={tt('sponsorBlockTitle')}>
                  {t('settings.sponsorBlockDescription')}
                </p>
              </div>
              <button
                role="switch"
                aria-checked={sponsorBlockEnabled}
                onClick={async () => {
                  const next = !sponsorBlockEnabled
                  setSponsorBlockEnabled(next)
                  try {
                    await saveSettings({ sponsorBlockEnabled: next })
                  } catch (err) {
                    console.warn('UpdateSettings failed:', err)
                  }
                }}
                className="relative w-10 h-5 rounded-full transition-colors shrink-0 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
                style={{
                  background: sponsorBlockEnabled ? 'var(--color-accent)' : 'var(--color-surface-border)',
                }}
                title={tt('sponsorBlockTitle')}
                aria-label={tt('sponsorBlockTitle')}
              >
                <span
                  className="absolute top-0.5 w-4 h-4 rounded-full bg-white transition-transform"
                  style={{ left: '2px', transform: sponsorBlockEnabled ? 'translateX(20px)' : 'translateX(0)' }}
                />
              </button>
            </div>
          </section>

          {showAdvanced && (
            <>
              {/* Max Concurrency */}
              <section
                className="rounded-xl p-4 border"
                style={{ background: 'var(--color-surface-light)', borderColor: 'var(--color-surface-border)' }}
              >
                <h3 className="text-sm font-medium mb-3" style={{ color: 'var(--text-secondary)' }} title={tt('maxConcurrency')}>
                  {t('settings.downloads')}
                </h3>
                <div className="flex items-center gap-3">
                  <label
                    htmlFor="maxConcurrency"
                    className="text-xs"
                    style={{ color: 'var(--text-muted)', minWidth: '7rem' }}
                    title={tt('maxConcurrency')}
                  >
                    {t('settings.maxParallelDownloads')}
                  </label>
                  <input
                    id="maxConcurrency"
                    type="number"
                    min={1}
                    max={10}
                    value={maxConcurrency}
                    onChange={async (e) => {
                      const v = Math.max(1, Math.min(10, parseInt(e.target.value) || 1))
                      setMaxConcurrency(v)
                      try {
                        await saveSettings({ maxConcurrency: v })
                      } catch (err) {
                        console.warn('UpdateSettings failed:', err)
                      }
                    }}
                    className="input-dark text-xs w-16 text-center"
                    title={tt('maxConcurrency')}
                    aria-label={tt('maxConcurrency')}
                  />
                </div>
              </section>

              {/* YouTube Cookies & Private Videos */}
              <section
                className="rounded-xl p-4 border md:col-span-2 xl:col-span-3"
                style={{ background: 'var(--color-surface-light)', borderColor: 'var(--color-surface-border)' }}
              >
                <h3 className="text-sm font-medium mb-1" style={{ color: 'var(--text-secondary)' }} title={tt('cookiesTitle')}>
                  {t('settings.cookiesTitle')}
                </h3>
                <p className="text-xs mb-4" style={{ color: 'var(--text-muted)' }}>
                  {t('settings.cookiesDescription')}
                </p>

                <div className="grid gap-4 grid-cols-1 md:grid-cols-2">
                  <div>
                    <label
                      htmlFor="cookieSourceSelect"
                      className="block text-xs font-medium mb-1.5"
                      style={{ color: 'var(--text-secondary)' }}
                      title={tt('cookiesSource')}
                    >
                      {t('settings.cookiesSource')}
                    </label>
                    <select
                      id="cookieSourceSelect"
                      value={cookieSource}
                      onChange={async (e) => {
                        const val = e.target.value as 'none' | 'browser' | 'file'
                        setCookieSource(val)
                        try {
                          await saveSettings({ cookieSource: val })
                        } catch (err) {
                          console.warn('UpdateSettings failed:', err)
                        }
                      }}
                      className="select-dark text-xs w-full"
                      title={tt('cookiesSource')}
                      aria-label={tt('cookiesSource')}
                    >
                      <option value="none">{t('settings.cookiesSourceNone')}</option>
                      <option value="browser">{t('settings.cookiesSourceBrowser')}</option>
                      <option value="file">{t('settings.cookiesSourceFile')}</option>
                    </select>
                  </div>

                  {cookieSource === 'browser' && (
                    <div>
                      <label
                        htmlFor="cookieBrowserSelect"
                        className="block text-xs font-medium mb-1.5"
                        style={{ color: 'var(--text-secondary)' }}
                        title={tt('cookiesBrowserLabel')}
                      >
                        {t('settings.cookiesBrowserLabel')}
                      </label>
                      <select
                        id="cookieBrowserSelect"
                        value={cookieBrowser}
                        onChange={async (e) => {
                          const val = e.target.value
                          setCookieBrowser(val)
                          try {
                            await saveSettings({ cookieBrowser: val })
                          } catch (err) {
                            console.warn('UpdateSettings failed:', err)
                          }
                        }}
                        className="select-dark text-xs w-full"
                        title={tt('cookiesBrowserLabel')}
                        aria-label={tt('cookiesBrowserLabel')}
                      >
                        <option value="chrome">Chrome</option>
                        <option value="firefox">Firefox</option>
                        <option value="safari">Safari</option>
                        <option value="edge">Edge</option>
                        <option value="brave">Brave</option>
                        <option value="vivaldi">Vivaldi</option>
                        <option value="opera">Opera</option>
                        <option value="chromium">Chromium</option>
                        <option value="whale">Whale</option>
                      </select>
                    </div>
                  )}

                  {cookieSource === 'file' && (
                    <div>
                      <label
                        htmlFor="cookieFilePathInput"
                        className="block text-xs font-medium mb-1.5"
                        style={{ color: 'var(--text-secondary)' }}
                        title={tt('cookiesFileLabel')}
                      >
                        {t('settings.cookiesFileLabel')}
                      </label>
                      <div className="flex gap-2">
                        <input
                          id="cookieFilePathInput"
                          type="text"
                          readOnly
                          value={cookieFilePath}
                          placeholder={t('settings.cookiesFilePlaceholder')}
                          className="input-dark text-xs flex-1 truncate"
                          title={tt('cookiesFileLabel')}
                          aria-label={tt('cookiesFileLabel')}
                        />
                        <button
                          onClick={async () => {
                            try {
                              const file = await SelectCookieFile()
                              if (file) {
                                setCookieFilePath(file)
                                await saveSettings({ cookieFilePath: file })
                              }
                            } catch (err) {
                              console.warn('SelectCookieFile failed:', err)
                            }
                          }}
                          className="btn-primary text-xs px-3 py-1.5 shrink-0"
                          title={tt('cookiesFileLabel')}
                          aria-label={tt('cookiesFileLabel')}
                        >
                          {t('actions.change')}
                        </button>
                      </div>
                    </div>
                  )}
                </div>

                {cookieSource === 'browser' && isChromiumBrowser(cookieBrowser) && (
                  <div
                    className="mt-4 p-3 rounded-lg border text-xs"
                    style={{
                      background: 'color-mix(in srgb, var(--color-accent) 5%, transparent)',
                      borderColor: 'var(--color-surface-border)',
                    }}
                  >
                    <div className="flex items-start gap-2.5">
                      <span className="text-base leading-none shrink-0" style={{ color: '#fbbf24' }}>
                        &#9888;&#FE0F;
                      </span>
                      <div className="flex-1 space-y-2">
                        <p style={{ color: 'var(--text-secondary)' }}>{t('settings.cookiesBrowserRunningWarning')}</p>
                        {browserError && (
                          <p role="alert" className="text-red-400">
                            {browserError}
                          </p>
                        )}
                        {browserRunning ? (
                          <div className="flex flex-wrap items-center gap-2">
                            <span className="font-semibold text-red-500">{cookieBrowser} is running</span>
                            <button
                              onClick={handleKillBrowser}
                              disabled={isCheckingBrowser}
                              className="btn-primary text-xs px-2.5 py-1"
                              title={tt('cookiesCloseBrowser')}
                              aria-label={tt('cookiesCloseBrowser')}
                              style={{ background: '#ef4444', borderColor: '#ef4444' }}
                            >
                              {isCheckingBrowser ? t('common.loading') : t('settings.cookiesCloseBrowser')}
                            </button>
                            <button
                              onClick={handleCloseBrowserAndFetch}
                              disabled={isCheckingBrowser || isCheckingBrowser}
                              className="btn-primary text-xs px-2.5 py-1 disabled:opacity-50 disabled:cursor-not-allowed"
                              title={t('settings.cookiesCloseAndFetch')}
                              aria-label={t('settings.cookiesCloseAndFetch')}
                            >
                              {isCheckingBrowser ? t('common.loading') : t('settings.cookiesCloseAndFetch')}
                            </button>
                          </div>
                        ) : (
                          <span className="text-green-500 font-semibold">{cookieBrowser} is closed</span>
                        )}
                      </div>
                    </div>
                  </div>
                )}
              </section>
            </>
          )}

          {/* ffmpeg Path (Advanced) */}
          {showAdvanced && (
            <section
              className="rounded-xl p-4 border md:col-span-2 xl:col-span-3"
              style={{ background: 'var(--color-surface-light)', borderColor: 'var(--color-surface-border)' }}
            >
              <h3 className="text-sm font-medium mb-1" style={{ color: 'var(--text-secondary)' }}>
                {t('settings.ffmpegPathTitle')}
              </h3>
              <p className="text-xs mb-3" style={{ color: 'var(--text-muted)' }}>
                {t('settings.ffmpegPathDescription')}
              </p>
              <div className="flex gap-2">
                <input
                  id="ffmpegPathInput"
                  type="text"
                  value={ffmpegPath}
                  onChange={async (e) => {
                    const val = e.target.value
                    setFfmpegPath(val)
                    try {
                      await saveSettings({ ffmpegPath: val })
                    } catch (err) {
                      console.warn('UpdateSettings failed:', err)
                    }
                  }}
                  className="input-dark text-xs flex-1 truncate"
                  placeholder={t('settings.ffmpegPathPlaceholder')}
                  title={t('settings.ffmpegPathTitle')}
                  aria-label={t('settings.ffmpegPathTitle')}
                />
                <button
                  onClick={async () => {
                    try {
                      const file = await SelectFfmpegPath()
                      if (file) {
                        setFfmpegPath(file)
                        await saveSettings({ ffmpegPath: file })
                      }
                    } catch (err) {
                      console.warn('SelectFfmpegPath failed:', err)
                    }
                  }}
                  className="btn-primary text-xs px-3 py-1.5 shrink-0"
                  title={tt('settingsBrowseFfmpeg')}
                  aria-label={tt('settingsBrowseFfmpeg')}
                >
                  {t('actions.change')}
                </button>
                {ffmpegPath && (
                  <button
                    onClick={async () => {
                      setFfmpegPath('')
                      try {
                        await saveSettings({ ffmpegPath: '' })
                      } catch (err) {
                        console.warn('UpdateSettings failed:', err)
                      }
                    }}
                    className="text-xs px-3 py-1.5 shrink-0 rounded-lg border transition-colors"
                    style={{ borderColor: 'var(--color-surface-border)', color: 'var(--text-muted)' }}
                    title={tt('settingsClearFfmpegPath')}
                    aria-label={tt('settingsClearFfmpegPath')}
                  >
                    {t('actions.clear')}
                  </button>
                )}
              </div>
            </section>
          )}

          {/* Custom Arguments (Advanced) */}
          {showAdvanced && (
            <section
              className="rounded-xl p-4 border md:col-span-2 xl:col-span-3"
              style={{ background: 'var(--color-surface-light)', borderColor: 'var(--color-surface-border)' }}
            >
              <h3 className="text-sm font-medium mb-1" style={{ color: 'var(--text-secondary)' }}>
                {t('settings.customArgsTitle')}
              </h3>
              <p className="text-xs mb-3" style={{ color: 'var(--text-muted)' }}>
                {t('settings.customArgsDescription')}
              </p>
              <div
                className="flex items-center justify-between mb-3 gap-3 rounded-lg px-3 py-2"
                style={{
                  background: 'var(--color-surface-lighter)',
                  border: '1px solid var(--color-surface-border)',
                }}
              >
                <div>
                  <p className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
                    {t('settings.safeModeTitle')}
                  </p>
                  <p className="text-xs mt-0.5" style={{ color: 'var(--text-muted)' }}>
                    {t('settings.safeModeDescription')}
                  </p>
                </div>
                <button
                  role="switch"
                  aria-checked={safeModeEnabled}
                  onClick={async () => {
                    const next = !safeModeEnabled
                    setSafeModeEnabled(next)
                    try {
                      await saveSettings({ safeModeEnabled: next })
                    } catch (err) {
                      console.warn('UpdateSettings failed:', err)
                    }
                  }}
                  className="relative w-10 h-5 rounded-full transition-colors shrink-0"
                  style={{
                    background: safeModeEnabled ? 'var(--color-accent)' : 'var(--color-surface-border)',
                  }}
                  title={t('settings.safeModeTitle')}
                  aria-label={t('settings.safeModeTitle')}
                >
                  <span
                    className="absolute top-0.5 w-4 h-4 rounded-full bg-white transition-transform"
                    style={{ left: '2px', transform: safeModeEnabled ? 'translateX(20px)' : 'translateX(0)' }}
                  />
                </button>
              </div>
              <textarea
                id="customArgsInput"
                value={customArgs}
                onChange={async (e) => {
                  const val = e.target.value
                  setCustomArgs(val)
                  try {
                    await saveSettings({ customArgs: val })
                  } catch (err) {
                    console.warn('UpdateSettings failed:', err)
                  }
                }}
                className="input-dark text-xs w-full h-20 font-mono resize-y"
                placeholder={t('settings.customArgsPlaceholder')}
                title={t('settings.customArgsTitle')}
                aria-label={t('settings.customArgsTitle')}
              />
            </section>
          )}

          {showAdvanced && (
            <section
              className="md:col-span-2 xl:col-span-3 rounded-xl p-4 border"
              style={{ background: 'var(--color-surface-light)', borderColor: 'var(--color-surface-border)' }}
            >
              <div className="flex items-center justify-between mb-3 gap-2">
                <h3 className="text-sm font-medium" style={{ color: 'var(--text-secondary)' }} title={tt('versionKoalaPull')}>
                  {t('settings.versions')}
                </h3>
                <button
                  onClick={() => OpenBinDir().catch((err: any) => console.warn('OpenBinDir failed:', err))}
                  className="btn-primary text-xs px-2.5 py-1.5 flex items-center gap-1.5 shrink-0"
                  title={tt('openBinFolder')}
                  aria-label={tt('openBinFolder')}
                >
                  <span>{t('settings.openBinFolder')}</span>
                </button>
              </div>
              <div className="space-y-2 text-sm">
                <div className="flex items-center gap-2">
                  <span className="w-20" style={{ color: 'var(--text-muted)' }}>
                    KoalaPull
                  </span>
                  <button
                    onClick={() => OpenExternalLink('https://github.com/Shik3i/KoalaPull').catch(() => {})}
                    className="font-mono hover:underline inline-flex items-center gap-1"
                    style={{ color: 'var(--color-accent)' }}
                    title={tt('versionKoalaPull')}
                    aria-label={tt('versionKoalaPull')}
                  >
                    <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                      <path d="M12 0C5.37 0 0 5.37 0 12c0 5.3 3.44 9.8 8.21 11.38.6.11.79-.26.79-.58v-2.23c-3.34.73-4.03-1.41-4.03-1.41-.55-1.39-1.33-1.76-1.33-1.76-1.09-.75.08-.73.08-.73 1.21.08 1.84 1.24 1.84 1.24 1.07 1.83 2.81 1.3 3.49 1 .11-.78.42-1.31.76-1.61-2.66-.3-5.47-1.33-5.47-5.93 0-1.31.47-2.38 1.24-3.22-.12-.3-.53-1.53.12-3.18 0 0 1-.32 3.3 1.23.96-.27 1.98-.4 3-.4s2.05.13 3.01.4c2.29-1.55 3.3-1.23 3.3-1.23.65 1.65.24 2.88.12 3.18.77.84 1.24 1.91 1.24 3.22 0 4.61-2.81 5.62-5.48 5.92.43.37.82 1.1.82 2.22v3.29c0 .32.19.69.8.57C20.56 21.8 24 17.3 24 12c0-6.63-5.37-12-12-12z" />
                    </svg>
                    {appVersion ? formatAppVersionLabel(appVersion) : '-'}
                  </button>
                </div>
                <div className="flex items-center gap-2">
                  <span className="w-20" style={{ color: 'var(--text-muted)' }}>
                    yt-dlp
                  </span>
                  <button
                    type="button"
                    onClick={() => OpenExternalLink('https://github.com/yt-dlp/yt-dlp').catch(() => {})}
                    className="font-mono hover:underline inline-flex items-center gap-1"
                    style={{ color: 'var(--color-accent)', background: 'none', border: 'none', padding: 0 }}
                    title={tt('versionYtdlp')}
                    aria-label={tt('versionYtdlp')}
                  >
                    {toolVersionsLoading ? t('common.loading') : toolVersions?.ytdlp || '-'}
                    <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24" style={{ opacity: 0.6 }}>
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
                      />
                    </svg>
                  </button>
                </div>
                <div className="flex items-center gap-2">
                  <span className="w-20" style={{ color: 'var(--text-muted)' }}>
                    ffmpeg
                  </span>
                  <button
                    type="button"
                    onClick={() =>
                      OpenExternalLink(
                        navigator.platform.includes('Mac')
                          ? 'https://evermeet.cx/ffmpeg/'
                          : 'https://github.com/BtbN/FFmpeg-Builds'
                      ).catch(() => {})
                    }
                    className="font-mono hover:underline inline-flex items-center gap-1"
                    style={{ color: 'var(--color-accent)', background: 'none', border: 'none', padding: 0 }}
                    title={tt('versionFfmpeg')}
                    aria-label={tt('versionFfmpeg')}
                  >
                    {toolVersionsLoading ? t('common.loading') : toolVersions?.ffmpeg || '-'}
                    <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24" style={{ opacity: 0.6 }}>
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
                      />
                    </svg>
                  </button>
                </div>
              </div>
            </section>
          )}

          {/* Updates */}
          <section
            className="md:col-span-2 xl:col-span-3 rounded-xl p-4 border"
            style={{ background: 'var(--color-surface-light)', borderColor: 'var(--color-surface-border)' }}
          >
            <h3 className="text-sm font-medium mb-3" style={{ color: 'var(--text-secondary)' }} title={tt('redownloadDependencies')}>
              {t('settings.updates')}
            </h3>
            <div className="text-xs space-y-3" style={{ color: 'var(--text-muted)' }}>
              {updateInfo ? (
                <>
                  {/* KoalaPull */}
                  <div>
                    {updateInfo.koalaPullUpdateAvailable ? (
                      <div>
                        <span style={{ color: '#fbbf24' }}>
                          {t('updates.koalaAvailable', { version: updateInfo.latestKoalaPullVersion })}
                        </span>
                        <span className="ml-2" style={{ color: 'var(--text-muted)' }}>
                          {t('common.currentVersion', { version: appVersion ? formatAppVersionLabel(appVersion) : '?' })}
                        </span>
                        <button
                          onClick={() => OpenExternalLink('https://github.com/Shik3i/KoalaPull/releases/latest').catch(() => {})}
                          className="btn-primary text-xs px-3 py-1 ml-3"
                          title={tt('viewRelease')}
                          aria-label={tt('viewRelease')}
                        >
                          {t('actions.viewRelease')}
                        </button>
                      </div>
                    ) : (
                      <p>{t('updates.koalaCurrent', { version: appVersion ? formatAppVersionLabel(appVersion) : '?' })}</p>
                    )}
                  </div>

                  {/* yt-dlp */}
                  <div>
                    {updateInfo.ytdlpUpdateAvailable ? (
                      <div>
                        <span style={{ color: '#fbbf24' }}>
                          {t('updates.ytdlpAvailable', { version: updateInfo.latestYtdlpVersion })}
                        </span>
                        <span className="ml-2" style={{ color: 'var(--text-muted)' }}>
                          {t('common.currentVersion', { version: toolVersions?.ytdlp || '?' })}
                        </span>
                      </div>
                    ) : (
                      <p>{t('updates.ytdlpCurrent', { version: toolVersions?.ytdlp || '?' })}</p>
                    )}
                  </div>

                  {/* ffmpeg */}
                  <div>
                    {updateInfo.ffmpegUpdateAvailable ? (
                      <div>
                        <span style={{ color: '#fbbf24' }}>{t('updates.ffmpegAvailable')}</span>
                        <span className="ml-2" style={{ color: 'var(--text-muted)' }}>
                          {t('common.currentVersion', { version: toolVersions?.ffmpeg || '?' })}
                        </span>
                      </div>
                    ) : (
                      <p>{t('updates.ffmpegCurrent', { version: toolVersions?.ffmpeg || '?' })}</p>
                    )}
                  </div>

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
                    {updatingDeps ? t('actions.updating') : t('actions.redownloadAll')}
                  </button>
                </>
              ) : updateLoading ? (
                <p>{t('updates.checking')}</p>
              ) : (
                <p>{t('updates.unavailable')}</p>
              )}
              {updatesError && (
                <p className="mt-2" style={{ color: '#f87171' }}>
                  {updatesError}
                </p>
              )}
            </div>
          </section>
        </div>
      </div>
    </div>
  )
}
