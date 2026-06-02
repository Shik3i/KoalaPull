const parseBytesRe = /^([\d.]+)\s*([KMG]i?B?b?)?/

export function parseBytes(s: string): number {
  if (!s) return 0
  const m = s.match(parseBytesRe)
  if (!m) return 0
  const num = parseFloat(m[1])
  const unit = (m[2] || '').toLowerCase()
  if (unit.startsWith('ki') || unit === 'kb') return num * 1024
  if (unit.startsWith('mi') || unit === 'mb') return num * 1024 * 1024
  if (unit.startsWith('gi') || unit === 'gb') return num * 1024 * 1024 * 1024
  return num
}

export function parseSpeed(s: string): number {
  return parseBytes(s.replace('/s', ''))
}

export function formatTotalEta(seconds: number): string {
  if (!seconds || seconds < 1) return ''
  if (seconds < 60) return `<${Math.round(seconds)}s`
  if (seconds < 3600) {
    const m = Math.floor(seconds / 60)
    const s = Math.round(seconds % 60)
    return `~${m}:${String(s).padStart(2, '0')}`
  }
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  return `~${h}:${String(m).padStart(2, '0')}:${String(Math.round(seconds % 60)).padStart(2, '0')}`
}
