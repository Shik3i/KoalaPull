import { useMemo, useState } from 'react'
import { FetchLastfmTracks, ResolveMusicTrack } from '../../wailsjs/go/main/App'
import type { main } from '../../wailsjs/go/models'

interface LastfmImportProps {
  username: string
  apiKey: string
  onCredentialsChange: (username: string, apiKey: string) => Promise<void>
  onStageUrls: (urls: string[]) => void
  t: (key: string, params?: Record<string, string | number>) => string
}

export function LastfmImport({ username, apiKey, onCredentialsChange, onStageUrls, t }: LastfmImportProps) {
  const [localUsername, setLocalUsername] = useState(username)
  const [localApiKey, setLocalApiKey] = useState(apiKey)
  const [source, setSource] = useState('top')
  const [period, setPeriod] = useState('1month')
  const [tracks, setTracks] = useState<main.LastfmTrack[]>([])
  const [selected, setSelected] = useState<Record<number, boolean>>({})
  const [loading, setLoading] = useState(false)
  const [resolving, setResolving] = useState(false)
  const [progress, setProgress] = useState(0)
  const [error, setError] = useState('')
  const selectedCount = useMemo(() => Object.values(selected).filter(Boolean).length, [selected])

  const loadTracks = async () => {
    setLoading(true)
    setError('')
    try {
      await onCredentialsChange(localUsername.trim(), localApiKey.trim())
      const result = await FetchLastfmTracks(localUsername.trim(), localApiKey.trim(), source, period, 25)
      setTracks(result)
      setSelected(Object.fromEntries(result.map((_, index) => [index, true])))
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setLoading(false)
    }
  }

  const stageSelected = async () => {
    setResolving(true)
    setError('')
    setProgress(0)
    const chosen = tracks.filter((_, index) => selected[index])
    const urls: string[] = []
    try {
      for (let index = 0; index < chosen.length; index += 1) {
        const track = chosen[index]
        urls.push(await ResolveMusicTrack(track.artist, track.title))
        setProgress(index + 1)
      }
      onStageUrls(urls)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setResolving(false)
    }
  }

  return (
    <section className="rounded-xl border p-4 space-y-4" style={{ background: 'var(--color-surface-light)', borderColor: 'var(--color-surface-border)' }} aria-labelledby="lastfm-title">
      <div>
        <h2 id="lastfm-title" className="text-sm font-semibold">{t('lastfm.title')}</h2>
        <p className="text-xs mt-1 leading-5" style={{ color: 'var(--text-muted)' }}>{t('lastfm.description')}</p>
      </div>
      <div className="grid gap-3 md:grid-cols-2">
        <label className="text-xs font-medium">{t('lastfm.username')}
          <input className="input-dark w-full mt-1" value={localUsername} onChange={(event) => setLocalUsername(event.target.value)} autoComplete="off" />
        </label>
        <label className="text-xs font-medium">{t('lastfm.apiKey')}
          <input className="input-dark w-full mt-1 font-mono" value={localApiKey} onChange={(event) => setLocalApiKey(event.target.value)} type="password" autoComplete="off" />
        </label>
        <label className="text-xs font-medium">{t('lastfm.source')}
          <select className="select-dark w-full mt-1" value={source} onChange={(event) => setSource(event.target.value)}>
            <option value="top">{t('lastfm.topTracks')}</option><option value="loved">{t('lastfm.lovedTracks')}</option><option value="recent">{t('lastfm.recentTracks')}</option>
          </select>
        </label>
        <label className="text-xs font-medium">{t('lastfm.period')}
          <select className="select-dark w-full mt-1" value={period} onChange={(event) => setPeriod(event.target.value)} disabled={source !== 'top'}>
            <option value="7day">{t('lastfm.week')}</option><option value="1month">{t('lastfm.month')}</option><option value="12month">{t('lastfm.year')}</option><option value="overall">{t('lastfm.overall')}</option>
          </select>
        </label>
      </div>
      <button className="btn-primary text-xs" onClick={loadTracks} disabled={loading || !localUsername.trim() || !localApiKey.trim()}>{loading ? t('common.loading') : t('lastfm.load')}</button>
      {error && <div role="alert" className="rounded-lg border border-red-500/40 bg-red-500/10 p-3 text-xs text-red-300">{error}</div>}
      {tracks.length > 0 && <>
        <div className="flex items-center justify-between gap-3">
          <p className="text-xs" aria-live="polite">{t('lastfm.selected', { count: selectedCount })}</p>
          <button className="text-xs" style={{ color: 'var(--color-accent)' }} onClick={() => setSelected(Object.fromEntries(tracks.map((_, index) => [index, selectedCount !== tracks.length])))}>{selectedCount === tracks.length ? t('lastfm.selectNone') : t('lastfm.selectAll')}</button>
        </div>
        <div className="max-h-72 overflow-y-auto rounded-lg border divide-y" style={{ borderColor: 'var(--color-surface-border)' }}>
          {tracks.map((track, index) => <label key={`${track.artist}-${track.title}`} className="flex items-start gap-3 p-3 cursor-pointer hover:bg-white/5">
            <input type="checkbox" className="mt-0.5 accent-[var(--color-accent)]" checked={!!selected[index]} onChange={(event) => setSelected((current) => ({ ...current, [index]: event.target.checked }))} />
            <span className="min-w-0"><span className="block text-sm truncate">{track.title}</span><span className="block text-xs truncate" style={{ color: 'var(--text-muted)' }}>{track.artist}{track.plays ? ` · ${t('lastfm.plays', { count: track.plays })}` : ''}</span></span>
          </label>)}
        </div>
        <div className="rounded-lg p-3 text-xs leading-5" style={{ background: 'color-mix(in srgb, var(--color-accent) 8%, transparent)', color: 'var(--text-secondary)' }}>{t('lastfm.reviewNotice')}</div>
        <button className="btn-primary text-xs" onClick={stageSelected} disabled={resolving || selectedCount === 0}>{resolving ? t('lastfm.matching', { done: progress, total: selectedCount }) : t('lastfm.stage')}</button>
      </>}
    </section>
  )
}
