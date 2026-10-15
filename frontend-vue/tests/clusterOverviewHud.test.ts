import { describe, it } from 'node:test'
import assert from 'node:assert/strict'

export interface ClusterResourceSummary {
  usedCpuCores: number
  totalCpuCores: number
  usedMemBytes: number
  totalMemBytes: number
  usedDiskBytes: number
  totalDiskBytes: number
}

export function resolvePercentageBarColor(val: number): string {
  if (val >= 85) return '#f43f5e' // rose
  if (val >= 70) return '#f59e0b' // amber
  return '#10b981' // emerald
}

export function clampPercentage(val: number): number {
  if (Number.isNaN(val) || !Number.isFinite(val)) return 0
  return Math.min(100, Math.max(0, Number(val.toFixed(1))))
}

export function formatAllocatableBytes(bytes: number): string {
  if (bytes <= 0 || !Number.isFinite(bytes)) return '0 B'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  const idx = Math.min(i, units.length - 1)
  const val = bytes / Math.pow(1024, idx)
  return `${val >= 10 || idx === 0 ? Math.round(val) : Number(val.toFixed(1))} ${units[idx]}`
}

export function calculateClusterResourceCommitment(used: number, total: number): {
  pct: number
  color: string
  label: string
} {
  const pct = total > 0 ? clampPercentage((used / total) * 100) : 0
  const color = resolvePercentageBarColor(pct)
  const label = `${pct}%`
  return { pct, color, label }
}

export function resolveGridSymmetry(viewportWidth: number): {
  columns: number
  layoutMode: 'mobile-single' | 'tablet-symmetric-3-2' | 'desktop-5-col'
} {
  if (viewportWidth < 640) {
    return { columns: 1, layoutMode: 'mobile-single' }
  }
  if (viewportWidth < 1024) {
    return { columns: 6, layoutMode: 'tablet-symmetric-3-2' } // 3 columns (span 2) on top, 2 columns (span 3) below
  }
  return { columns: 5, layoutMode: 'desktop-5-col' }
}

export function formatHttpRps(rps: number): string {
  if (rps <= 0 || !Number.isFinite(rps)) return '0 req/s'
  if (rps >= 1000) {
    return `${(rps / 1000).toFixed(1)}k req/s`
  }
  return `${Math.round(rps)} req/s`
}

describe('Task 012: Cluster Overview HUD & Symmetric PercentageBar Logic Tests', () => {
  it('PercentageBar color mapping: emerald < 70%, amber 70-84%, rose >= 85%', () => {
    assert.equal(resolvePercentageBarColor(0), '#10b981')
    assert.equal(resolvePercentageBarColor(45.5), '#10b981')
    assert.equal(resolvePercentageBarColor(69.9), '#10b981')
    assert.equal(resolvePercentageBarColor(70), '#f59e0b')
    assert.equal(resolvePercentageBarColor(80), '#f59e0b')
    assert.equal(resolvePercentageBarColor(84.9), '#f59e0b')
    assert.equal(resolvePercentageBarColor(85), '#f43f5e')
    assert.equal(resolvePercentageBarColor(99.9), '#f43f5e')
    assert.equal(resolvePercentageBarColor(120), '#f43f5e')
  })

  it('clampPercentage bounds input strictly between 0 and 100', () => {
    assert.equal(clampPercentage(-15), 0)
    assert.equal(clampPercentage(0), 0)
    assert.equal(clampPercentage(42.345), 42.3)
    assert.equal(clampPercentage(100), 100)
    assert.equal(clampPercentage(150), 100)
    assert.equal(clampPercentage(NaN), 0)
    assert.equal(clampPercentage(Infinity), 0)
  })

  it('formatAllocatableBytes formats byte thresholds cleanly', () => {
    assert.equal(formatAllocatableBytes(0), '0 B')
    assert.equal(formatAllocatableBytes(512), '512 B')
    assert.equal(formatAllocatableBytes(1024), '1 KiB')
    assert.equal(formatAllocatableBytes(1048576), '1 MiB')
    assert.equal(formatAllocatableBytes(1073741824), '1 GiB')
    assert.equal(formatAllocatableBytes(68719476736), '64 GiB')
    assert.equal(formatAllocatableBytes(1099511627776), '1 TiB')
  })

  it('calculateClusterResourceCommitment computes percentages and color status', () => {
    const normal = calculateClusterResourceCommitment(32, 64)
    assert.equal(normal.pct, 50)
    assert.equal(normal.color, '#10b981')
    assert.equal(normal.label, '50%')

    const warning = calculateClusterResourceCommitment(75, 100)
    assert.equal(warning.pct, 75)
    assert.equal(warning.color, '#f59e0b')

    const critical = calculateClusterResourceCommitment(95, 100)
    assert.equal(critical.pct, 95)
    assert.equal(critical.color, '#f43f5e')

    const zeroTotal = calculateClusterResourceCommitment(0, 0)
    assert.equal(zeroTotal.pct, 0)
    assert.equal(zeroTotal.color, '#10b981')
  })

  it('resolveGridSymmetry adapts layout mode for mobile, tablet (3+2), and desktop (5-col)', () => {
    assert.deepEqual(resolveGridSymmetry(375), { columns: 1, layoutMode: 'mobile-single' })
    assert.deepEqual(resolveGridSymmetry(768), { columns: 6, layoutMode: 'tablet-symmetric-3-2' })
    assert.deepEqual(resolveGridSymmetry(820), { columns: 6, layoutMode: 'tablet-symmetric-3-2' })
    assert.deepEqual(resolveGridSymmetry(1280), { columns: 5, layoutMode: 'desktop-5-col' })
    assert.deepEqual(resolveGridSymmetry(1920), { columns: 5, layoutMode: 'desktop-5-col' })
  })

  it('formatHttpRps formats real measured RPS rates cleanly without synthetic multipliers', () => {
    assert.equal(formatHttpRps(0), '0 req/s')
    assert.equal(formatHttpRps(45), '45 req/s')
    assert.equal(formatHttpRps(1250), '1.3k req/s')
    assert.equal(formatHttpRps(24000), '24.0k req/s')
    assert.equal(formatHttpRps(-5), '0 req/s')
  })
})
