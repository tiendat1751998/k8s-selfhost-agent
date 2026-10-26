import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
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
  enrichSLOItem,
  sortSLOsByUrgency,
  enrichAndSortSLOs,
  deriveFleetPulse,
  calculateErrorBudget,
} = await import('../src/composables/useSLOMonitor.ts')

describe('SloBurnDownBoard Urgency Sorting & Fleet Pulse Tests', () => {
  it('enrichAndSortSLOs prioritizes critical/fast-burn and exhausted budgets above warning and healthy', () => {
    const definitions = [
      { id: 'slo-1', service: 'auth-service', target: 99.9, indicator_type: 'availability', window: '30d', alert_threshold: 1.5, created_at: '' },
      { id: 'slo-2', service: 'checkout-api', target: 99.5, indicator_type: 'latency', window: '30d', alert_threshold: 1.5, created_at: '' },
      { id: 'slo-3', service: 'payment-gateway', target: 99.99, indicator_type: 'availability', window: '30d', alert_threshold: 1.5, created_at: '' },
      { id: 'slo-4', service: 'logging-worker', target: 99.0, indicator_type: 'availability', window: '30d', alert_threshold: 2.0, created_at: '' },
    ]

    const snapshots = [
      // slo-1: Healthy (burn rate 0.4x, 88% budget)
      { id: 's1', slo_id: 'slo-1', service: 'auth-service', target: 99.9, actual: 99.95, burn_rate: 0.4, error_budget: 88.0, budget_status: 'healthy', recorded_at: '' },
      // slo-2: Fast Burn Alert (burn rate 14.4x, alert_threshold 1.5, 40% budget)
      { id: 's2', slo_id: 'slo-2', service: 'checkout-api', target: 99.5, actual: 98.8, burn_rate: 14.4, error_budget: 40.0, budget_status: 'critical', recorded_at: '' },
      // slo-3: Depleted Budget (burn rate 1.2x, 0% budget)
      { id: 's3', slo_id: 'slo-3', service: 'payment-gateway', target: 99.99, actual: 99.8, burn_rate: 1.2, error_budget: 0.0, budget_status: 'critical', recorded_at: '' },
      // slo-4: Elevated/Warning (burn rate 1.8x < 2.0 threshold, 25% budget)
      { id: 's4', slo_id: 'slo-4', service: 'logging-worker', target: 99.0, actual: 99.1, burn_rate: 1.8, error_budget: 25.0, budget_status: 'warning', recorded_at: '' },
    ]

    const sorted = enrichAndSortSLOs(definitions, snapshots)

    // Critical/urgent (checkout-api with 14.4x burn and payment-gateway with 0% budget) MUST come before warning and healthy
    assert.equal(sorted[0].service, 'checkout-api', 'Highest burn rate (14.4x) must be ranked 1st')
    assert.equal(sorted[1].service, 'payment-gateway', 'Depleted budget (0%) must be ranked 2nd')
    assert.equal(sorted[2].service, 'logging-worker', 'Warning status must be ranked 3rd')
    assert.equal(sorted[3].service, 'auth-service', 'Healthy status must be ranked last')
  })

  it('enrichSLOItem correctly marks fastBurn and clamps error_budget between 0 and 100', () => {
    const def = {
      id: 'def-test',
      service: 'search-api',
      target: 99.9,
      indicator_type: 'latency',
      window: '7d',
      alert_threshold: 2.0,
      created_at: '',
    }

    // Test negative budget clamp to 0 and fast burn detection
    const snapExhausted = {
      id: 'snap-1',
      slo_id: 'def-test',
      service: 'search-api',
      target: 99.9,
      actual: 98.0,
      burn_rate: 3.5,
      error_budget: -15.5,
      budget_status: 'critical',
      recorded_at: '',
    }
    const item1 = enrichSLOItem(def, snapExhausted)
    assert.equal(item1.error_budget, 0, 'Negative budget must be clamped to 0')
    assert.equal(item1.isFastBurn, true, 'Burn rate 3.5 >= threshold 2.0 must set isFastBurn=true')
    assert.equal(item1.isExhausted, true, 'Budget <= 0 must set isExhausted=true')

    // Test over 100% budget clamp to 100
    const snapOver = {
      id: 'snap-2',
      slo_id: 'def-test',
      service: 'search-api',
      target: 99.9,
      actual: 100,
      burn_rate: 0.2,
      error_budget: 125,
      budget_status: 'healthy',
      recorded_at: '',
    }
    const item2 = enrichSLOItem(def, snapOver)
    assert.equal(item2.error_budget, 100, 'Budget > 100 must be clamped to 100')
    assert.equal(item2.isFastBurn, false, 'Burn rate 0.2 < threshold 2.0 must set isFastBurn=false')
    assert.equal(item2.isExhausted, false)
  })

  it('deriveFleetPulse calculates fleet reliability statistics deterministically', () => {
    const items = [
      { id: '1', service: 'a', target: 99.9, actual: 99.95, error_budget: 80, burn_rate: 0.5, alert_threshold: 1.5, budget_status: 'healthy', indicator_type: 'availability', window: '30d', query: '', isFastBurn: false, isExhausted: false, def: {} as any },
      { id: '2', service: 'b', target: 99.5, actual: 99.0, error_budget: 0, burn_rate: 15.0, alert_threshold: 1.5, budget_status: 'critical', indicator_type: 'availability', window: '30d', query: '', isFastBurn: true, isExhausted: true, def: {} as any },
      { id: '3', service: 'c', target: 99.9, actual: 99.8, error_budget: 40, burn_rate: 2.5, alert_threshold: 1.5, budget_status: 'critical', indicator_type: 'latency', window: '30d', query: '', isFastBurn: true, isExhausted: false, def: {} as any },
      { id: '4', service: 'd', target: 99.0, actual: 99.1, error_budget: 60, burn_rate: 1.1, alert_threshold: 1.5, budget_status: 'warning', indicator_type: 'error_rate', window: '30d', query: '', isFastBurn: false, isExhausted: false, def: {} as any },
    ]

    const pulse = deriveFleetPulse(items)
    assert.equal(pulse.monitoredCount, 4)
    assert.equal(pulse.healthyCount, 1)
    assert.equal(pulse.fastBurnCount, 2) // items 2 and 3
    assert.equal(pulse.exhaustedCount, 1) // item 2
    // Average budget: (80 + 0 + 40 + 60) / 4 = 180 / 4 = 45.0%
    assert.equal(pulse.avgBudgetRemainingPct, 45.0)
  })

  it('SLOView.vue contains NO duplicate "+ + Add Target" button markup', () => {
    const sloViewPath = path.resolve(import.meta.dirname, '../src/views/SLOView.vue')
    const content = fs.readFileSync(sloViewPath, 'utf-8')
    assert.doesNotMatch(content, /<span>\s*\+\s*Add Target<\/span>/i, 'Must not render literal "+ " inside <span>Add Target</span>')
    assert.doesNotMatch(content, /\+\s*\+\s*Add/i, 'Must not have duplicate plus sign')
  })

  it('Strict file line limits: files must stay < 500 lines and SloBurnDownBoard < 300 lines', () => {
    const filesToCheck = [
      { file: '../src/views/SLOView.vue', maxLines: 500 },
      { file: '../src/composables/useSLOMonitor.ts', maxLines: 500 },
      { file: '../src/components/slo/SloBurnDownBoard.vue', maxLines: 300 },
    ]

    for (const { file, maxLines } of filesToCheck) {
      const fullPath = path.resolve(import.meta.dirname, file)
      if (fs.existsSync(fullPath)) {
        const lines = fs.readFileSync(fullPath, 'utf-8').split('\n').length
        assert.ok(lines <= maxLines, `${file} has ${lines} lines, exceeding limit of ${maxLines}`)
      }
    }
  })
})
