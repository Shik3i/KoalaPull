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

export function parseEta(s: string): number {
  if (!s) return 0
  const parts = s.trim().split(':').map(Number)
  if (parts.some(isNaN)) return 0
  if (parts.length === 1) {
    return parts[0]
  }
  if (parts.length === 2) {
    return parts[0] * 60 + parts[1]
  }
  if (parts.length === 3) {
    return parts[0] * 3600 + parts[1] * 60 + parts[2]
  }
  return 0
}

export function formatSpeed(bytesPerSec: number): string {
  if (!bytesPerSec || bytesPerSec < 0) return '0.00B/s'
  if (bytesPerSec >= 1024 * 1024 * 1024) {
    return `${(bytesPerSec / (1024 * 1024 * 1024)).toFixed(2)}GiB/s`
  }
  if (bytesPerSec >= 1024 * 1024) {
    return `${(bytesPerSec / (1024 * 1024)).toFixed(2)}MiB/s`
  }
  if (bytesPerSec >= 1024) {
    return `${(bytesPerSec / 1024).toFixed(2)}KiB/s`
  }
  return `${bytesPerSec.toFixed(2)}B/s`
}

export function formatEta(seconds: number): string {
  if (!seconds || seconds < 0) return '00:00'
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.round(seconds % 60)
  
  const mm = String(m).padStart(2, '0')
  const ss = String(s).padStart(2, '0')
  
  if (h > 0) {
    const hh = String(h).padStart(2, '0')
    return `${hh}:${mm}:${ss}`
  }
  return `${mm}:${ss}`
}

export function formatBytes(bytes: number): string {
  return formatSpeed(bytes).replace('/s', '')
}
