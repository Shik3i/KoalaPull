import { useMemo, memo } from 'react'
import { PlayFile, ShowFileInFolder } from '../../wailsjs/go/main/App'
import type { main } from '../../wailsjs/go/models'

const maxVisibleHistoryEntries = 500

interface HistoryTabProps {
  history: main.HistoryEntryView[]
  historySearch: string
  setHistorySearch: (search: string) => void
  historyLoading: boolean
  historyError: string
  handleClearHistory: () => void
  handleDeleteHistoryEntry: (id: string) => void
  handleReuseHistoryURL: (url: string) => void
  fmtTime: (t: string) => string
  t: (key: string, params?: Record<string, string | number>) => string
  tt: (key: string) => string
}

function HistoryEntries({
  entries,
  search,
  onDelete,
  onReuse,
  fmtTime,
  t,
}: {
  entries: main.HistoryEntryView[]
  search: string
  onDelete: (id: string) => void
  onReuse: (url: string) => void
  fmtTime: (t: string) => string
  t: (key: string, params?: Record<string, string | number>) => string
}) {
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
        <div
          className="rounded-lg px-3 py-2 text-xs"
          style={{
            background: 'var(--color-surface-light)',
            border: '1px solid var(--color-surface-border)',
            color: 'var(--text-muted)',
          }}
        >
          {t('history.showingLimited', { shown: visible.length, total: filtered.length })}
        </div>
      )}
      {visible.map((entry) => (
        <HistoryRow key={entry.downloadId} entry={entry} onDelete={onDelete} onReuse={onReuse} fmtTime={fmtTime} t={t} />
      ))}
    </div>
  )
}

const HistoryRow = memo(
  ({
    entry,
    onDelete,
    onReuse,
    fmtTime,
    t,
  }: {
    entry: main.HistoryEntryView
    onDelete: (id: string) => void
    onReuse: (url: string) => void
    fmtTime: (t: string) => string
    t: (key: string, params?: Record<string, string | number>) => string
  }) => {
    const statusKey = `downloads.status.${entry.status}`
    return (
      <div
        className="rounded-lg p-3.5 lg:p-4 flex items-center gap-3"
        style={{ background: 'var(--color-surface-light)', border: '1px solid var(--color-surface-border)' }}
      >
        <div className="flex-1 min-w-0">
          <p className="text-sm font-medium truncate">{entry.title || t('common.untitled')}</p>
          <div className="flex flex-wrap gap-x-4 gap-y-1 mt-1.5 text-xs" style={{ color: 'var(--text-secondary)' }}>
            <span>{t('history.started', { time: fmtTime(entry.startTime) })}</span>
            <span>{t('history.ended', { time: fmtTime(entry.endTime) })}</span>
            {entry.fileSize && <span>{t('history.size', { size: entry.fileSize })}</span>}
            {entry.avgSpeed && <span>{t('history.speed', { speed: entry.avgSpeed })}</span>}
            <span
              className={`font-medium ${
                entry.status === 'completed'
                  ? 'text-green-400'
                  : entry.status === 'cancelled'
                  ? 'text-yellow-400'
                  : 'text-red-400'
              }`}
            >
              {t(statusKey)}
            </span>
          </div>
          {entry.errorMsg && (
            <p className="mt-1 text-xs truncate" style={{ color: '#f87171' }}>
              {entry.errorMsg}
            </p>
          )}
        </div>
        <div className="shrink-0 flex items-center gap-1">
          {entry.status === 'completed' && entry.outputPath && (
            <>
              <button
                onClick={() => entry.outputPath && PlayFile(entry.outputPath)}
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
                onClick={() => entry.outputPath && ShowFileInFolder(entry.outputPath)}
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
            onClick={() => onReuse(entry.url)}
            className="icon-button"
            style={{ color: 'var(--color-accent)' }}
            title={t('actions.useAgain')}
            aria-label={t('actions.useAgain')}
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v6h6M20 20v-6h-6M5.5 15a7 7 0 0011.5 2M18.5 9A7 7 0 007 7" />
            </svg>
          </button>
          <button
            onClick={() => onDelete(entry.downloadId)}
            className="icon-button"
            style={{ color: 'var(--text-muted)' }}
            title={t('actions.deleteEntry')}
            aria-label={t('actions.deleteEntry')}
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
              />
            </svg>
          </button>
        </div>
      </div>
    )
  }
)

export function HistoryTab({
  history,
  historySearch,
  setHistorySearch,
  historyLoading,
  historyError,
  handleClearHistory,
  handleDeleteHistoryEntry,
  handleReuseHistoryURL,
  fmtTime,
  t,
  tt,
}: HistoryTabProps) {
  return (
    <div className="flex-1 flex flex-col overflow-hidden">
      <div className="flex flex-col sm:flex-row sm:items-center gap-2 px-4 lg:px-8 py-4 shrink-0">
        <div className="flex-1 text-center">
          <h2 className="text-base font-semibold inline-block" title={tt('searchHistory')}>
            {t('history.title')}
          </h2>
        </div>
        <div className="flex items-center gap-3 w-full sm:w-auto shrink-0">
          {(history.length > 0 || historyError) && (
            <>
              <input
                type="text"
                value={historySearch}
                onChange={(e) => setHistorySearch(e.target.value)}
                placeholder={t('history.searchPlaceholder')}
                className="input-dark text-xs w-full sm:w-44 lg:w-56"
                title={tt('searchHistory')}
                aria-label={tt('searchHistory')}
              />
              <button
                onClick={() => {
                  if (window.confirm(t('history.clearConfirm'))) handleClearHistory()
                }}
                className="btn-primary text-xs px-3 py-1.5"
                title={tt('clearHistory')}
                aria-label={tt('clearHistory')}
              >
                {t('actions.clearAll')}
              </button>
            </>
          )}
        </div>
      </div>
      <div className="flex-1 overflow-y-auto px-4 lg:px-8 py-4 lg:py-6">
        {historyError && (
          <div role="alert" className="mb-4 p-3 rounded-lg border text-xs text-red-400" style={{ borderColor: '#ef4444' }}>
            {historyError}
          </div>
        )}
        {historyLoading ? (
          <div className="flex flex-col items-center justify-center py-16" style={{ color: 'var(--text-muted)' }}>
            <div
              className="w-6 h-6 border-2 rounded-full animate-spin mb-3"
              style={{ borderColor: 'var(--color-accent)', borderTopColor: 'transparent' }}
            />
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
          <HistoryEntries
            entries={history}
            search={historySearch}
            onDelete={handleDeleteHistoryEntry}
            onReuse={handleReuseHistoryURL}
            fmtTime={fmtTime}
            t={t}
          />
        )}
      </div>
    </div>
  )
}
