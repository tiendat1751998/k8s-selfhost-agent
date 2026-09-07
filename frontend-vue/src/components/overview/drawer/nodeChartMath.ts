import type { NodeMetricRollup } from '../../../api/compute'

// Chart Coordinates
export const HIST_Y_BOTTOM = 180
export const HIST_Y_HEIGHT = 160 // 180 - 20
export const HIST_X_LEFT = 45
export const HIST_X_RIGHT = 725
export const HIST_X_WIDTH = 680 // 725 - 45

export const pad = (n: number) => String(n).padStart(2, '0')
export const formatDt = (d: Date) =>
  `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`

export function getSafeDefaultDates() {
  const now = new Date()
  return {
    from: formatDt(new Date(now.getTime() - 24 * 60 * 60 * 1000)),
    to: formatDt(now),
  }
}

export function hPeak(h: NodeMetricRollup): number {
  return h.cpu_peak !== undefined && h.cpu_peak !== null ? h.cpu_peak : h.cpu_percent
}

export function formatBytes(bytes?: number, decimals = 1): string {
  if (!bytes || bytes <= 0 || isNaN(bytes)) return '0 B'
  const k = 1024
  const dm = decimals < 0 ? 0 : decimals
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i] || 'B'}`
}

export function formatPercent(val?: number): string {
  if (val === undefined || val === null || isNaN(val)) return '0%'
  return `${Math.round(val)}%`
}

export function formatIoRate(bytesPerSec?: number): string {
  if (!bytesPerSec || bytesPerSec <= 0 || isNaN(bytesPerSec)) return '0 B/s'
  return `${formatBytes(bytesPerSec)}/s`
}

export function buildLinePath(list: NodeMetricRollup[], valFn: (h: NodeMetricRollup) => number): string {
  if (list.length === 0) return ''
  if (list.length === 1) {
    const y = HIST_Y_BOTTOM - (Math.min(100, Math.max(0, valFn(list[0]))) / 100) * HIST_Y_HEIGHT
    return `M ${HIST_X_LEFT} ${y.toFixed(1)} L ${HIST_X_RIGHT} ${y.toFixed(1)}`
  }
  const step = HIST_X_WIDTH / (list.length - 1)
  return list.map((h, i) => {
    const x = HIST_X_LEFT + i * step
    const y = HIST_Y_BOTTOM - (Math.min(100, Math.max(0, valFn(h))) / 100) * HIST_Y_HEIGHT
    return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`
  }).join(' ')
}

export function buildAreaPath(linePath: string, list: NodeMetricRollup[]): string {
  if (!linePath || list.length === 0) return ''
  const firstX = HIST_X_LEFT
  const lastX = list.length === 1 ? HIST_X_RIGHT : HIST_X_LEFT + (list.length - 1) * (HIST_X_WIDTH / (list.length - 1))
  return `${linePath} L ${lastX.toFixed(1)} ${HIST_Y_BOTTOM} L ${firstX} ${HIST_Y_BOTTOM} Z`
}

export function buildCpuEnvelope(list: NodeMetricRollup[]): string {
  if (list.length === 0) return ''
  if (list.length === 1) {
    const peakVal = Math.max(hPeak(list[0]), list[0].cpu_percent)
    const avgVal = list[0].cpu_percent
    const yPeak = HIST_Y_BOTTOM - (Math.min(100, Math.max(0, peakVal)) / 100) * HIST_Y_HEIGHT
    const yAvg = HIST_Y_BOTTOM - (Math.min(100, Math.max(0, avgVal)) / 100) * HIST_Y_HEIGHT
    return `M ${HIST_X_LEFT} ${yPeak.toFixed(1)} L ${HIST_X_RIGHT} ${yPeak.toFixed(1)} L ${HIST_X_RIGHT} ${yAvg.toFixed(1)} L ${HIST_X_LEFT} ${yAvg.toFixed(1)} Z`
  }
  const step = HIST_X_WIDTH / (list.length - 1)
  const peakPoints = list.map((h, i) => {
    const x = HIST_X_LEFT + i * step
    const peakVal = Math.max(hPeak(h), h.cpu_percent)
    const y = HIST_Y_BOTTOM - (Math.min(100, Math.max(0, peakVal)) / 100) * HIST_Y_HEIGHT
    return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`
  }).join(' ')
  const avgPointsRev = [...list].reverse().map((h, i) => {
    const origIdx = list.length - 1 - i
    const x = HIST_X_LEFT + origIdx * step
    const y = HIST_Y_BOTTOM - (Math.min(100, Math.max(0, h.cpu_percent)) / 100) * HIST_Y_HEIGHT
    return `L ${x.toFixed(1)} ${y.toFixed(1)}`
  }).join(' ')
  return `${peakPoints} ${avgPointsRev} Z`
}

export function buildSpikeMarkers(list: NodeMetricRollup[]): Array<{ index: number; x: number; y: number; time: string; peak: number; avg: number }> {
  if (list.length === 0) return []
  const step = list.length > 1 ? HIST_X_WIDTH / (list.length - 1) : 0
  const markers: Array<{ index: number; x: number; y: number; time: string; peak: number; avg: number }> = []

  list.forEach((h, i) => {
    const peak = Math.max(hPeak(h), h.cpu_percent)
    const avg = h.cpu_percent || 0
    if (peak >= 75 || (peak - avg >= 20 && peak >= 50)) {
      const x = list.length > 1 ? HIST_X_LEFT + i * step : (HIST_X_LEFT + HIST_X_RIGHT) / 2
      const y = HIST_Y_BOTTOM - (Math.min(100, Math.max(0, peak)) / 100) * HIST_Y_HEIGHT
      const d = new Date(h.recorded_at)
      const timeStr = d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
      markers.push({ index: i, x, y, time: timeStr, peak, avg })
    }
  })
  return markers
}

export function buildTimeMarkers(list: NodeMetricRollup[], range?: string): Array<{ x: number; time: string }> {
  if (list.length === 0) return []
  if (list.length === 1) {
    const d = new Date(list[0].recorded_at)
    return [{ x: (HIST_X_LEFT + HIST_X_RIGHT) / 2, time: d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) }]
  }
  const count = Math.min(6, list.length)
  const markers = []
  const step = HIST_X_WIDTH / (list.length - 1)
  const firstD = new Date(list[0].recorded_at)
  const lastD = new Date(list[list.length - 1].recorded_at)
  const spanMs = Math.abs(lastD.getTime() - firstD.getTime())
  const isMultiDay = spanMs > 24 * 60 * 60 * 1000 || range === '7d' || range === '30d'

  for (let i = 0; i < count; i++) {
    const idx = Math.round((i / (count - 1)) * (list.length - 1))
    const d = new Date(list[idx].recorded_at)
    const timeStr = isMultiDay
      ? `${d.getMonth() + 1}/${d.getDate()} ${d.getHours().toString().padStart(2, '0')}:00`
      : d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    markers.push({
      x: HIST_X_LEFT + idx * step,
      time: timeStr,
    })
  }
  return markers
}
