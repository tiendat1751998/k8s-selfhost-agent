import { describe, it } from 'node:test'
import assert from 'node:assert/strict'

describe('Task 017: MetricSparkline SVG Path Math & Coordinate Tests', () => {
  function generateSparklinePoints(
    data: number[],
    width = 36,
    height = 12,
    strokeWidth = 1.5
  ): { path: string; fillPath: string } {
    if (!data || data.length === 0) {
      return { path: '', fillPath: '' }
    }

    const pad = strokeWidth
    const w = Math.max(width, 1)
    const h = Math.max(height, 1)
    const usableH = Math.max(h - pad * 2, 1)

    if (data.length === 1) {
      const midY = (h / 2).toFixed(1)
      const path = `M 0 ${midY} L ${w} ${midY}`
      return { path, fillPath: `${path} L ${w} ${h} L 0 ${h} Z` }
    }

    const min = Math.min(...data)
    const max = Math.max(...data)
    const range = max - min

    if (range === 0) {
      const midY = (h / 2).toFixed(1)
      const path = `M 0 ${midY} L ${w} ${midY}`
      return { path, fillPath: `${path} L ${w} ${h} L 0 ${h} Z` }
    }

    const stepX = w / (data.length - 1)
    const coords: [number, number][] = data.map((val, idx) => {
      const x = Number((idx * stepX).toFixed(1))
      const normalized = (val - min) / range
      const y = Number((h - pad - normalized * usableH).toFixed(1))
      return [x, y]
    })

    const path = coords.map(([x, y], idx) => (idx === 0 ? `M ${x} ${y}` : `L ${x} ${y}`)).join(' ')
    const fillPath = `${path} L ${w} ${h} L 0 ${h} Z`

    return { path, fillPath }
  }

  it('generateSparklinePoints returns empty strings for empty dataset', () => {
    const res = generateSparklinePoints([])
    assert.equal(res.path, '')
    assert.equal(res.fillPath, '')
  })

  it('generateSparklinePoints renders horizontal midpoint line for single data point', () => {
    const res = generateSparklinePoints([42], 36, 12)
    assert.equal(res.path, 'M 0 6.0 L 36 6.0')
  })

  it('generateSparklinePoints renders centered horizontal line when all values are equal', () => {
    const res = generateSparklinePoints([100, 100, 100, 100], 36, 12)
    assert.equal(res.path, 'M 0 6.0 L 36 6.0')
  })

  it('generateSparklinePoints maps fluctuating dataset across full width and height with padding', () => {
    const data = [10, 20, 15, 30]
    const res = generateSparklinePoints(data, 30, 20, 2)
    // 4 points -> stepX = 10
    // min = 10, max = 30, range = 20, pad = 2, usableH = 16
    // Point 0 (val 10): x=0, y = 20 - 2 - 0 = 18
    // Point 1 (val 20): x=10, y = 20 - 2 - (0.5 * 16) = 10
    // Point 2 (val 15): x=20, y = 20 - 2 - (0.25 * 16) = 14
    // Point 3 (val 30): x=30, y = 20 - 2 - (1.0 * 16) = 2
    assert.equal(res.path, 'M 0 18 L 10 10 L 20 14 L 30 2')
    assert.equal(res.fillPath, 'M 0 18 L 10 10 L 20 14 L 30 2 L 30 20 L 0 20 Z')
  })
})
