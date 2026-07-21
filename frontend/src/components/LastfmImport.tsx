import { useEffect, useMemo, useRef, useState } from 'react'
import { CancelMusicMatching, FetchLastfmTracks, OpenExternalLink, ResolveMusicTracks } from '../../wailsjs/go/main/App'
import type { main } from '../../wailsjs/go/models'

interface LastfmImportProps {
  username: string
  apiKey: string
  onCredentialsChange: (username: string, apiKey: string) => Promise<void>
  onStageUrls: (urls: string[]) => void
  t: (key: string, params?: Record<string, string | number>) => string
}

export function selectedMatchURLs(matches: main.MusicMatchResult[], selected: Record<number, boolean>): string[] {
  return matches.flatMap((match, index) => selected[index] && match.url ? [match.url] : [])
}

function lastfmErrorMessage(error: unknown, t: LastfmImportProps['t']): string {
  const message = error instanceof Error ? error.message : String(error)
  if (/API key/i.test(message)) return t('lastfm.invalidApiKey')
  if (/user not found|user.*unavailable/i.test(message)) return t('lastfm.userUnavailable')
  if (/rate limit/i.test(message)) return t('lastfm.rateLimited')
  return message
}

export function LastfmImport({ username, apiKey, onCredentialsChange, onStageUrls, t }: LastfmImportProps) {
  const [localUsername, setLocalUsername] = useState(username)
  const [localApiKey, setLocalApiKey] = useState(apiKey)
  const [source, setSource] = useState('top')
  const [period, setPeriod] = useState('1month')
  const [limit, setLimit] = useState(25)
  const [showApiKey, setShowApiKey] = useState(false)
  const [tracks, setTracks] = useState<main.LastfmTrack[]>([])
  const [selected, setSelected] = useState<Record<number, boolean>>({})
  const [loading, setLoading] = useState(false)
  const [resolving, setResolving] = useState(false)
  const [error, setError] = useState('')
  const [hasLoaded, setHasLoaded] = useState(false)
  const [matches, setMatches] = useState<main.MusicMatchResult[]>([])
  const [selectedMatches, setSelectedMatches] = useState<Record<number, boolean>>({})
  const [matchDrafts, setMatchDrafts] = useState<Record<number, { artist: string, title: string }>>({})
  const [retryingIndex, setRetryingIndex] = useState<number | null>(null)
  const currentJobRef = useRef('')
  const retryJobRef = useRef('')
  const mountedRef = useRef(true)
  const selectedCount = useMemo(() => Object.values(selected).filter(Boolean).length, [selected])
  const successfulMatches = useMemo(() => matches.filter((match) => !!match.url), [matches])
  const failedMatches = useMemo(() => matches.filter((match) => !match.url), [matches])
  const selectedMatchCount = useMemo(() => selectedMatchURLs(matches, selectedMatches).length, [matches, selectedMatches])

  useEffect(() => { setLocalUsername(username); setLocalApiKey(apiKey) }, [username, apiKey])
  useEffect(() => {
    mountedRef.current = true
    return () => {
      mountedRef.current = false
      if (currentJobRef.current) void CancelMusicMatching(currentJobRef.current)
      if (retryJobRef.current) void CancelMusicMatching(retryJobRef.current)
    }
  }, [])

  const loadTracks = async () => {
    setLoading(true)
    setError('')
    try {
      const result = await FetchLastfmTracks(localUsername.trim(), localApiKey.trim(), source, period, limit)
      if (!mountedRef.current) return
      setTracks(result)
      setSelected(Object.fromEntries(result.map((_, index) => [index, true])))
      setMatches([])
      setSelectedMatches({})
      setMatchDrafts({})
      setHasLoaded(true)
      try {
        await onCredentialsChange(localUsername.trim(), localApiKey.trim())
      } catch (err) {
        if (mountedRef.current) setError(err instanceof Error ? err.message : String(err))
      }
    } catch (err) {
      if (mountedRef.current) setError(lastfmErrorMessage(err, t))
    } finally {
      if (mountedRef.current) setLoading(false)
    }
  }

  const stageSelected = async () => {
    setResolving(true)
    setError('')
    setMatches([])
    const chosen = tracks.filter((_, index) => selected[index])
    const jobID = `music-${Date.now()}-${Math.random().toString(36).slice(2)}`
    currentJobRef.current = jobID
    try {
      const results = await ResolveMusicTracks(jobID, chosen)
      if (!mountedRef.current || currentJobRef.current !== jobID) return
      setMatches(results)
      setSelectedMatches(Object.fromEntries(results.map((match, index) => [index, !!match.url])))
      setMatchDrafts(Object.fromEntries(results.map((match, index) => [index, { artist: match.artist, title: match.title }])))
    } catch (err) {
      if (mountedRef.current && currentJobRef.current === jobID) setError(err instanceof Error ? err.message : String(err))
    } finally {
      if (mountedRef.current && currentJobRef.current === jobID) {
        currentJobRef.current = ''
        setResolving(false)
      }
    }
  }

  const retryMatch = async (index: number) => {
    const draft = matchDrafts[index]
    if (!draft?.artist.trim() || !draft.title.trim() || retryingIndex !== null || resolving) return
    const jobID = `music-retry-${Date.now()}-${Math.random().toString(36).slice(2)}`
    retryJobRef.current = jobID
    setRetryingIndex(index)
    setError('')
    try {
      const [result] = await ResolveMusicTracks(jobID, [{ artist: draft.artist.trim(), title: draft.title.trim() }])
      if (!mountedRef.current || retryJobRef.current !== jobID || !result) return
      setMatches((current) => current.map((match, matchIndex) => matchIndex === index ? result : match))
      setSelectedMatches((current) => ({ ...current, [index]: !!result.url }))
    } catch (err) {
      if (mountedRef.current && retryJobRef.current === jobID) setError(err instanceof Error ? err.message : String(err))
    } finally {
      if (mountedRef.current && retryJobRef.current === jobID) {
        retryJobRef.current = ''
        setRetryingIndex(null)
      }
    }
  }

  const cancelMatching = async () => {
    const jobID = currentJobRef.current
    currentJobRef.current = ''
    setResolving(false)
    if (jobID) await CancelMusicMatching(jobID)
  }

  return (
    <section className="rounded-xl border p-4 space-y-4" style={{ background: 'var(--color-surface-light)', borderColor: 'var(--color-surface-border)' }} aria-labelledby="lastfm-title">
      <div>
        <h2 id="lastfm-title" className="text-sm font-semibold">{t('lastfm.title')}</h2>
        <p className="text-xs mt-1 leading-5" style={{ color: 'var(--text-muted)' }}>{t('lastfm.description')}</p>
      </div>
      <div className="grid gap-3 md:grid-cols-2">
        <label className="text-xs font-medium">{t('lastfm.username')}
          <input className="input-dark w-full mt-1" value={localUsername} onChange={(event) => { setLocalUsername(event.target.value); setError('') }} autoComplete="off" />
        </label>
        <div>
          <label className="text-xs font-medium" htmlFor="lastfm-api-key">{t('lastfm.apiKey')}</label>
          <div className="relative mt-1">
            <input id="lastfm-api-key" className="input-dark w-full pr-16 font-mono" value={localApiKey} onChange={(event) => { setLocalApiKey(event.target.value); setError('') }} type={showApiKey ? 'text' : 'password'} autoComplete="off" />
            <button type="button" className="absolute inset-y-0 right-2 text-[11px]" style={{ color: 'var(--color-accent)' }} aria-label={showApiKey ? t('lastfm.hideApiKey') : t('lastfm.showApiKey')} onClick={() => setShowApiKey((current) => !current)}>{showApiKey ? t('lastfm.hide') : t('lastfm.show')}</button>
          </div>
          <button type="button" className="mt-1 text-[11px]" style={{ color: 'var(--color-accent)' }} onClick={() => void OpenExternalLink('https://www.last.fm/api/account/create').catch((err) => setError(err instanceof Error ? err.message : String(err)))}>{t('lastfm.getApiKey')}</button>
        </div>
        <label className="text-xs font-medium">{t('lastfm.source')}
          <select className="select-dark w-full mt-1" value={source} onChange={(event) => { setSource(event.target.value); setError('') }}>
            <option value="top">{t('lastfm.topTracks')}</option><option value="loved">{t('lastfm.lovedTracks')}</option><option value="recent">{t('lastfm.recentTracks')}</option>
          </select>
        </label>
        <label className="text-xs font-medium">{t('lastfm.period')}
          <select className="select-dark w-full mt-1" value={period} onChange={(event) => setPeriod(event.target.value)} disabled={source === 'loved'}>
            <option value="7day">{t('lastfm.week')}</option><option value="1month">{t('lastfm.month')}</option><option value="3month">{t('lastfm.threeMonths')}</option><option value="6month">{t('lastfm.sixMonths')}</option><option value="12month">{t('lastfm.year')}</option><option value="overall">{t('lastfm.overall')}</option>
          </select>
        </label>
        <label className="text-xs font-medium">{t('lastfm.trackCount')}
          <select className="select-dark w-full mt-1" value={limit} onChange={(event) => setLimit(Number(event.target.value))}>
            <option value={10}>10</option><option value={25}>25</option><option value={50}>50</option>
          </select>
        </label>
      </div>
      <button className="btn-primary text-xs" onClick={loadTracks} disabled={loading || resolving || retryingIndex !== null || !localUsername.trim() || !localApiKey.trim()}>{loading ? t('common.loading') : t('lastfm.load')}</button>
      {error && <div role="alert" className="rounded-lg border border-red-500/40 bg-red-500/10 p-3 text-xs text-red-300">{error}</div>}
      {hasLoaded && tracks.length === 0 && <div role="status" className="rounded-lg border p-4 text-center text-xs" style={{ borderColor: 'var(--color-surface-border)', color: 'var(--text-muted)' }}>{t('lastfm.empty')}</div>}
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
        <div className="flex flex-wrap gap-2">
          <button className="btn-primary text-xs" onClick={stageSelected} disabled={resolving || loading || selectedCount === 0}>{resolving ? t('lastfm.matching', { total: selectedCount }) : t('lastfm.stage')}</button>
          {resolving && <button className="rounded-md border px-3 py-2 text-xs" style={{ borderColor: 'var(--color-surface-border)' }} onClick={() => void cancelMatching()}>{t('actions.cancel')}</button>}
        </div>
        {matches.length > 0 && <div className="space-y-3 rounded-lg border p-3" style={{ borderColor: 'var(--color-surface-border)' }}>
          <p role="status" className="text-xs">{t('lastfm.matchSummary', { matched: successfulMatches.length, failed: failedMatches.length })}</p>
          <div className="grid gap-2 sm:grid-cols-2">
            {matches.map((match, index) => <article key={`${match.artist}-${match.title}-${index}`} className="space-y-2 rounded-lg border p-2.5" style={{ borderColor: selectedMatches[index] ? 'var(--color-accent)' : 'var(--color-surface-border)', background: 'var(--color-surface-lighter)' }}>
              <div className="flex gap-3">
                {match.thumbnail ? <img src={match.thumbnail} alt="" className="h-12 w-16 shrink-0 rounded object-cover" loading="lazy" /> : <div className="h-12 w-16 shrink-0 rounded" style={{ background: 'var(--color-surface-border)' }} />}
                <div className="min-w-0 flex-1">
                  {match.url ? <label className="flex items-start gap-2 text-xs font-medium"><input type="checkbox" className="mt-0.5 accent-[var(--color-accent)]" checked={!!selectedMatches[index]} onChange={(event) => setSelectedMatches((current) => ({ ...current, [index]: event.target.checked }))} /><span className="truncate">{match.matchedTitle || match.title}</span></label> : <p className="text-xs font-medium text-red-300">{t('lastfm.noMatch')}</p>}
                  <p className="text-[11px] truncate" style={{ color: 'var(--text-muted)' }}>{match.uploader || match.error}</p>
                </div>
              </div>
              <div className="grid grid-cols-2 gap-2">
                <label className="text-[10px]" style={{ color: 'var(--text-muted)' }}>{t('lastfm.searchArtist')}<input className="input-dark w-full mt-1 py-1.5 text-xs" value={matchDrafts[index]?.artist ?? match.artist} onChange={(event) => setMatchDrafts((current) => ({ ...current, [index]: { artist: event.target.value, title: current[index]?.title ?? match.title } }))} /></label>
                <label className="text-[10px]" style={{ color: 'var(--text-muted)' }}>{t('lastfm.searchTitle')}<input className="input-dark w-full mt-1 py-1.5 text-xs" value={matchDrafts[index]?.title ?? match.title} onChange={(event) => setMatchDrafts((current) => ({ ...current, [index]: { artist: current[index]?.artist ?? match.artist, title: event.target.value } }))} /></label>
              </div>
              <button className="rounded-md border px-2 py-1.5 text-[11px]" style={{ borderColor: 'var(--color-surface-border)' }} disabled={resolving || retryingIndex !== null || !matchDrafts[index]?.artist.trim() || !matchDrafts[index]?.title.trim()} onClick={() => void retryMatch(index)}>{retryingIndex === index ? t('lastfm.retrying') : t('lastfm.retryMatch')}</button>
            </article>)}
          </div>
          <button className="btn-primary text-xs" disabled={selectedMatchCount === 0 || retryingIndex !== null} onClick={() => onStageUrls(selectedMatchURLs(matches, selectedMatches))}>{t('lastfm.reviewMatches', { count: selectedMatchCount })}</button>
        </div>}
      </>}
    </section>
  )
}
