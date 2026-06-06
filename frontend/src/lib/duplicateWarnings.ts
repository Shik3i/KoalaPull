export interface DuplicateSource {
  url?: string
}

export interface DuplicateSummary {
  queueCount: number
  historyCount: number
}

function normalizeUrl(url: string): string {
  return url.trim().toLowerCase()
}

export function countDuplicateUrls(targetUrl: string, queue: DuplicateSource[], history: DuplicateSource[]): DuplicateSummary {
  const normalizedTarget = normalizeUrl(targetUrl)
  if (!normalizedTarget) {
    return { queueCount: 0, historyCount: 0 }
  }

  const countMatches = (items: DuplicateSource[]) =>
    items.reduce((count, item) => count + (normalizeUrl(item.url || "") === normalizedTarget ? 1 : 0), 0)

  return {
    queueCount: countMatches(queue),
    historyCount: countMatches(history),
  }
}
