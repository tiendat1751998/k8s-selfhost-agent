export function getUtilizationColor(pct: number): string {
  if (pct >= 85) return 'rose'
  if (pct >= 65) return 'amber'
  return 'emerald'
}

export function formatPercent(val?: number): string {
  if (val === undefined || val === null || isNaN(val)) return '0%'
  return `${Math.round(val)}%`
}

export function formatBytes(bytes?: number, decimals = 1): string {
  if (!bytes || bytes <= 0 || isNaN(bytes)) return '0 B'
  const k = 1024
  const dm = decimals < 0 ? 0 : decimals
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i] || 'B'}`
}

export function formatIoRate(bytesPerSec?: number): string {
  if (!bytesPerSec || bytesPerSec <= 0 || isNaN(bytesPerSec)) return '0 B/s'
  return `${formatBytes(bytesPerSec)}/s`
}
