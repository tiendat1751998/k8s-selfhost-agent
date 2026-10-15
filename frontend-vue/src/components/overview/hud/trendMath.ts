export function computeCeiling(rawMax: number): number {
  if (rawMax <= 0 || isNaN(rawMax)) return 10
  const padded = rawMax * 1.25
  if (padded <= 10) return 10
  if (padded <= 20) return Math.ceil(padded)
  if (padded <= 50) return Math.ceil(padded / 5) * 5
  if (padded <= 200) return Math.ceil(padded / 10) * 10
  if (padded <= 1000) return Math.ceil(padded / 50) * 50
  if (padded <= 10000) return Math.ceil(padded / 500) * 500
  return Math.ceil(padded / 1000) * 1000
}

export function buildSmoothSpline(
  points: Array<{ x: number; y: number }>,
  smoothing = 0.18,
  minY = 10,
  maxY = 194
): string {
  if (!points || points.length === 0) return ''
  if (points.length === 1) return `M ${points[0].x.toFixed(1)} ${points[0].y.toFixed(1)}`
  if (points.length === 2) return `M ${points[0].x.toFixed(1)} ${points[0].y.toFixed(1)} L ${points[1].x.toFixed(1)} ${points[1].y.toFixed(1)}`

  let d = `M ${points[0].x.toFixed(1)} ${points[0].y.toFixed(1)}`
  for (let i = 0; i < points.length - 1; i++) {
    const p0 = points[Math.max(0, i - 1)]
    const p1 = points[i]
    const p2 = points[i + 1]
    const p3 = points[Math.min(points.length - 1, i + 2)]

    const minLocalX = Math.min(p1.x, p2.x), maxLocalX = Math.max(p1.x, p2.x)
    const cp1x = Math.max(minLocalX, Math.min(maxLocalX, p1.x + ((p2.x - p0.x) / 6) * (1 - smoothing)))
    const cp2x = Math.max(minLocalX, Math.min(maxLocalX, p2.x - ((p3.x - p1.x) / 6) * (1 - smoothing)))
    const cp1y = Math.max(minY, Math.min(maxY, p1.y + ((p2.y - p0.y) / 6) * (1 - smoothing)))
    const cp2y = Math.max(minY, Math.min(maxY, p2.y - ((p3.y - p1.y) / 6) * (1 - smoothing)))

    d += ` C ${cp1x.toFixed(1)} ${cp1y.toFixed(1)}, ${cp2x.toFixed(1)} ${cp2y.toFixed(1)}, ${p2.x.toFixed(1)} ${p2.y.toFixed(1)}`
  }
  return d
}

export function formatMetricRate(val: number, withUnit = false): string {
  if (val === undefined || val === null || isNaN(val)) return withUnit ? '0 rps' : '0'
  let str = '0'
  if (val >= 1000000) str = `${(val / 1000000).toFixed(1)}M`
  else if (val >= 1000) str = `${(val / 1000).toFixed(1)}k`
  else if (val >= 100) str = Math.round(val).toString()
  else if (val > 0) str = val.toFixed(1)
  return withUnit ? `${str} rps` : str
}
