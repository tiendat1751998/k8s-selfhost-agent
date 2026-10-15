import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { register } from 'node:module'

register('data:text/javascript,' + encodeURIComponent(`
export async function resolve(specifier, context, nextResolve) {
  try {
    return await nextResolve(specifier, context);
  } catch (err) {
    if (specifier.startsWith('.')) {
      try {
        return await nextResolve(specifier + '.ts', context);
      } catch {}
    }
    throw err;
  }
}
`))

const {
  calculateErrorBudget,
  calculateBurnRateVelocity,
  getTargetLatencyPercentiles,
  formatLatencyMetric,
  formatPercent,
  getEffectiveBurnRate,
  getBurnRateColor,
  getBudgetBarWidth
} = await import('../src/composables/useSLOMonitor.ts')

describe('SLO View Enterprise Calculations & Domain Math', () => {
  it('calculateErrorBudget calculates remaining budget correctly when actual meets target or reaches 100%', () => {
    // 99.9% target, 100% actual -> 0 errors occurred -> 100% budget remaining
    const perfect = calculateErrorBudget(99.9, 100)
    assert.equal(perfect, 100)

    // 99.9% target, 99.9% actual -> 100% of allowed error consumed -> 0% budget remaining
    const exact = calculateErrorBudget(99.9, 99.9)
    assert.equal(exact, 0)
  })

  it('calculateErrorBudget calculates remaining budget correctly when actual dips below target', () => {
    // 99.0% target, 99.5% actual -> target error rate 1%, actual 0.5% -> consumed 50% -> 50% remaining
    const budget = calculateErrorBudget(99.0, 99.5)
    assert.equal(Math.round(budget), 50)

    // target 99.0, actual 98.0 -> consumed 200% -> 0% remaining
    const depleted = calculateErrorBudget(99.0, 98.0)
    assert.equal(depleted, 0)
  })

  it('calculateErrorBudget handles 100% target boundary', () => {
    assert.equal(calculateErrorBudget(100, 100), 100)
    assert.equal(calculateErrorBudget(100, 99.9), 100)
  })

  it('calculateBurnRateVelocity identifies P1, P2, P3, and Budget Positive status', () => {
    // P1 Extreme Fast Burn (>= 14.4x)
    const p1 = calculateBurnRateVelocity(14.4, '30d')
    assert.equal(p1.statusText, 'P1 Extreme Fast Burn')
    assert.equal(p1.hoursToExhaustion, 50) // 720 / 14.4

    // P2 Slow Burn Warning (>= 6.0x)
    const p2 = calculateBurnRateVelocity(6.0, '30d')
    assert.equal(p2.statusText, 'P2 Slow Burn Warning')
    assert.equal(p2.hoursToExhaustion, 120) // 720 / 6

    // P3 Elevated Depletion (>= 2.0x)
    const p3 = calculateBurnRateVelocity(2.5, '30d')
    assert.equal(p3.statusText, 'P3 Elevated Depletion')

    // Budget Positive (<= 1.0x)
    const normal = calculateBurnRateVelocity(0.8, '30d')
    assert.equal(normal.statusText, 'Budget Positive')
  })

  it('getTargetLatencyPercentiles extracts pure measured target without magic numbers', () => {
    assert.deepEqual(getTargetLatencyPercentiles(undefined), { p50: 0, p90: 0, p99: 0, p999: 0 })
    assert.deepEqual(getTargetLatencyPercentiles(0), { p50: 0, p90: 0, p99: 0, p999: 0 })
    assert.deepEqual(getTargetLatencyPercentiles(150), { p50: 0, p90: 0, p99: 150, p999: 0 })
  })

  it('formatLatencyMetric formats valid numbers with ms and invalid with --', () => {
    assert.equal(formatLatencyMetric(120), '120ms')
    assert.equal(formatLatencyMetric(0), '--')
    assert.equal(formatLatencyMetric(undefined), '--')
  })

  it('formatPercent handles fractional, whole, and invalid values', () => {
    assert.equal(formatPercent(undefined), '0.00%')
    assert.equal(formatPercent(NaN), '0.00%')
    assert.equal(formatPercent(0.9995), '99.95%')
    assert.equal(formatPercent(99.95), '99.95%')
  })

  it('getBurnRateColor maps burn rates to semantic badge classes', () => {
    assert.equal(getBurnRateColor(0.5), 'text-emerald')
    assert.equal(getBurnRateColor(1.0), 'text-emerald')
    assert.equal(getBurnRateColor(2.0), 'text-amber')
    assert.equal(getBurnRateColor(5.0), 'text-rose')
  })

  it('getBudgetBarWidth clamps percentage between 0 and 100', () => {
    assert.equal(getBudgetBarWidth(undefined), 0)
    assert.equal(getBudgetBarWidth(-10), 0)
    assert.equal(getBudgetBarWidth(75.5), 75.5)
    assert.equal(getBudgetBarWidth(150), 100)
  })
})
