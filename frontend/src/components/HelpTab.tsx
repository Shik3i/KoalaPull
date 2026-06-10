import { OpenExternalLink } from '../../wailsjs/go/main/App'

interface SupportedSite {
  name: string
  blurbKey: string
  href: string
}

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

const siteLogoImages = import.meta.glob('../assets/images/sites/*.png', {
  eager: true,
  query: '?url',
  import: 'default',
}) as Record<string, string>

function siteLogoUrl(site: SupportedSite): string | undefined {
  return siteLogoImages[`../assets/images/sites/${site.blurbKey.split('.').pop()}.png`]
}

function SiteMark({ site }: { site: SupportedSite }) {
  const logoUrl = siteLogoUrl(site)
  const initials = site.name
    .replace(/\s*\(.+\)\s*/g, '')
    .slice(0, 2)
    .toUpperCase()
  return (
    <div
      className="w-10 h-10 rounded-xl shrink-0 flex items-center justify-center overflow-hidden"
      style={{ background: 'var(--color-surface-lighter)', border: '1px solid var(--color-surface-border)' }}
    >
      {logoUrl ? (
        <img src={logoUrl} alt="" className="w-7 h-7 object-contain" loading="lazy" />
      ) : (
        <span className="text-xs font-bold" style={{ color: 'var(--color-accent)' }} aria-hidden="true">
          {initials}
        </span>
      )}
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
          <p className="text-xs mt-1 leading-5" style={{ color: 'var(--text-muted)' }}>
            {blurb}
          </p>
        </div>
      </div>
    </div>
  )
}

interface HelpTabProps {
  t: (key: string, params?: Record<string, string | number>) => string
  tt: (key: string, params?: Record<string, string | number>) => string
}

export function HelpTab({ t, tt }: HelpTabProps) {
  return (
    <div className="flex-1 flex flex-col overflow-hidden">
      <div className="px-4 lg:px-8 py-4 lg:py-5 shrink-0">
        <h2 className="text-base lg:text-lg font-semibold" title={tt('helpSteps')}>
          {t('help.title')}
        </h2>
      </div>
      <div className="flex-1 overflow-y-auto px-4 lg:px-8 py-5 lg:py-6">
        <div className="space-y-6">
          <section
            className="rounded-xl p-4 lg:p-6 border"
            style={{ background: 'var(--color-surface-light)', borderColor: 'var(--color-surface-border)' }}
          >
            <h3 className="text-sm lg:text-base font-medium mb-2" style={{ color: 'var(--text-secondary)' }} title={tt('helpSteps')}>
              {t('help.howToTitle')}
            </h3>
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
                      style={{
                        background: 'color-mix(in srgb, var(--color-accent) 18%, transparent)',
                        color: 'var(--color-accent)',
                      }}
                    >
                      {item.step}
                    </div>
                    <div>
                      <p className="text-sm font-semibold">{item.title}</p>
                      <p className="text-xs mt-1 leading-5" style={{ color: 'var(--text-muted)' }}>
                        {item.text}
                      </p>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </section>

          <section className="space-y-4">
            <div className="flex items-end justify-between gap-4">
              <div>
                <h3 className="text-sm lg:text-base font-medium" style={{ color: 'var(--text-secondary)' }} title={tt('supportedSites')}>
                  {t('help.supportedTitle')}
                </h3>
                <p className="text-xs lg:text-sm mt-1 max-w-2xl" style={{ color: 'var(--text-muted)' }}>
                  {t('help.supportedText')}
                </p>
              </div>
              <button
                onClick={() =>
                  OpenExternalLink('https://github.com/yt-dlp/yt-dlp/blob/master/supportedsites.md').catch((err) =>
                    console.warn('OpenExternalLink failed:', err)
                  )
                }
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
                  onClick={() =>
                    OpenExternalLink(site.href).catch((err) => console.warn('OpenExternalLink failed:', err))
                  }
                  className="block text-left"
                  title={tt('openSite', { site: site.name })}
                  aria-label={tt('openSite', { site: site.name })}
                >
                  <SiteBadge site={site} blurb={t(site.blurbKey)} />
                </button>
              ))}
            </div>
          </section>

          <section
            className="rounded-xl p-4 lg:p-6 border"
            style={{ background: 'var(--color-surface-light)', borderColor: 'var(--color-surface-border)' }}
          >
            <h3 className="text-sm lg:text-base font-medium mb-2" style={{ color: 'var(--text-secondary)' }} title={tt('underTheHood')}>
              {t('help.underTheHood')}
            </h3>
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
  )
}
