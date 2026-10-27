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
  deriveFleetPulse,
  formatPercent,
  formatBurnVelocityBadge,
} = await import('../src/composables/useSLOMonitor.ts')

const {
  complianceApi,
} = await import('../src/api/compliance.ts')

describe('T069-5: Honest SLO States & Live Compliance Scan', () => {
  const dummyDef = {
    id: 'slo-test-1',
    service: 'auth-gateway',
    target: 99.9,
    indicator_type: 'availability',
    window: '30d',
    alert_threshold: 1.5,
    created_at: '2026-10-27T00:00:00Z',
  }

  it('honestly marks unmeasured state when actual < 0 or budget_status === unmeasured or snap is missing', () => {
    // 1. Snapshot with actual = -1
    const snapNeg = {
      id: 'snap-neg',
      slo_id: dummyDef.id,
      service: dummyDef.service,
      target: 99.9,
      actual: -1,
      burn_rate: 0,
      error_budget: 0,
      budget_status: 'unmeasured',
      recorded_at: '',
    }
    const item1 = enrichSLOItem(dummyDef, snapNeg)
    assert.equal(item1.isUnmeasured, true, 'actual < 0 must set isUnmeasured=true')
    assert.equal(item1.budget_status, 'unmeasured')
    assert.equal(item1.actual, -1)
    assert.equal(item1.isFastBurn, false)
    assert.equal(item1.isExhausted, false)

    // 2. Snapshot with budget_status = 'unmeasured'
    const snapStatus = {
      id: 'snap-unm',
      slo_id: dummyDef.id,
      service: dummyDef.service,
      target: 99.9,
      actual: 0,
      burn_rate: 0,
      error_budget: 0,
      budget_status: 'unmeasured',
      recorded_at: '',
    }
    const item2 = enrichSLOItem(dummyDef, snapStatus)
    assert.equal(item2.isUnmeasured, true, 'budget_status unmeasured must set isUnmeasured=true')
    assert.equal(item2.isFastBurn, false)
    assert.equal(item2.isExhausted, false)

    // 3. No snapshot provided
    const item3 = enrichSLOItem(dummyDef, undefined)
    assert.equal(item3.isUnmeasured, true, 'missing snapshot must default to unmeasured')
    assert.equal(item3.budget_status, 'unmeasured')
    assert.equal(item3.actual, -1)
  })

  it('formatPercent formats negative/unmeasured actual as -- and zero as 0.00%', () => {
    assert.equal(formatPercent(-1), '--', 'Negative value represents unmeasured and must return --')
    assert.equal(formatPercent(-0.5), '--')
    assert.equal(formatPercent(0), '0.00%', 'Genuinely zero must render 0.00%')
    assert.equal(formatPercent(99.9), '99.90%')
  })

  it('correctly identifies genuinely DOWN targets (actual = 0, burn_rate = 1000)', () => {
    const snapDown = {
      id: 'snap-down',
      slo_id: dummyDef.id,
      service: dummyDef.service,
      target: 99.9,
      actual: 0,
      burn_rate: 1000,
      error_budget: 0,
      budget_status: 'critical',
      recorded_at: '',
    }
    const itemDown = enrichSLOItem(dummyDef, snapDown)
    assert.equal(itemDown.isUnmeasured, false, 'actual = 0 is a measured down state')
    assert.equal(itemDown.isExhausted, true, 'actual = 0 must mark isExhausted=true')
    assert.equal(itemDown.isFastBurn, true, '1000x burn rate must mark isFastBurn=true')
    assert.equal(itemDown.budget_status, 'critical')
    assert.equal(itemDown.error_budget, 0)
    assert.equal(itemDown.actual, 0)
  })

  it('formatBurnVelocityBadge prevents layout overflow for extreme burn rates (>= 100x)', () => {
    // Under 100x exhausted (DOWN target)
    const exhaustedUnder100 = formatBurnVelocityBadge(2.5, true, false)
    assert.equal(exhaustedUnder100.label, '2.5x Burn')
    assert.equal(exhaustedUnder100.cls, 'pill-burn-exhausted')

    // Over or equal 100x exhausted (DOWN target)
    const extreme = formatBurnVelocityBadge(1000, true, false)
    assert.equal(extreme.label, '1000x Burn')
    assert.equal(extreme.cls, 'pill-burn-exhausted')

    // Normal non-exhausted elevated
    const elevated = formatBurnVelocityBadge(2.5, false, false)
    assert.equal(elevated.label, '2.5x Elevated')
    assert.equal(elevated.cls, 'pill-burn-elevated')

    // Unmeasured
    const unmeasured = formatBurnVelocityBadge(0, false, true)
    assert.equal(unmeasured.label, 'Unmeasured')
    assert.equal(unmeasured.cls, 'pill-burn-unmeasured')
  })

  it('deriveFleetPulse strictly excludes unmeasured targets from AVG ERROR BUDGET calculation', () => {
    const items = [
      // 1 UP target: 100% budget
      {
        id: '1', service: 'up-service', target: 99.9, actual: 100, error_budget: 100,
        burn_rate: 0.2, alert_threshold: 1.5, budget_status: 'healthy', indicator_type: 'availability',
        window: '30d', query: '', isFastBurn: false, isExhausted: false, isUnmeasured: false, def: {} as any,
      },
      // 1 DOWN target: 0% budget
      {
        id: '2', service: 'down-service', target: 99.9, actual: 0, error_budget: 0,
        burn_rate: 1000, alert_threshold: 1.5, budget_status: 'critical', indicator_type: 'availability',
        window: '30d', query: '', isFastBurn: true, isExhausted: true, isUnmeasured: false, def: {} as any,
      },
      // 1 UNMEASURED target: 0 samples
      {
        id: '3', service: 'new-service', target: 99.9, actual: -1, error_budget: 0,
        burn_rate: 0, alert_threshold: 1.5, budget_status: 'unmeasured', indicator_type: 'availability',
        window: '30d', query: '', isFastBurn: false, isExhausted: false, isUnmeasured: true, def: {} as any,
      },
    ]

    const pulse = deriveFleetPulse(items)
    assert.equal(pulse.monitoredCount, 3, 'Monitored count includes all 3 targets')
    assert.equal(pulse.exhaustedCount, 1, 'Depleted count must be 1 (only real measured down target)')
    assert.equal(pulse.fastBurnCount, 1, 'Fast burn count must be 1')
    assert.equal(pulse.healthyCount, 1, 'Healthy count must be 1')
    assert.equal(pulse.unmeasuredCount, 1, 'Unmeasured count must be 1')

    // Average error budget MUST be calculated only from measured items:
    // (100 + 0) / 2 = 50.0% — NOT (100 + 0 + 0) / 3 = 33.3% or (100 + 0 + 100) / 3 = 66.7%!
    assert.equal(pulse.avgBudgetRemainingPct, 50.0, 'AVG ERROR BUDGET must strictly exclude unmeasured targets')
  })

  it('complianceApi.triggerScan falls back to /audit/run when /compliance/scan returns 404', async () => {
    let calledEndpoints: string[] = []
    const mockApiClient = {
      async post(url: string) {
        calledEndpoints.push(url)
        if (url === '/compliance/scan') {
          const err: any = new Error('404 Not Found')
          err.status = 404
          throw err
        }
        if (url === '/audit/run') {
          return { success: true, message: 'Audit run triggered' }
        }
        throw new Error('Unexpected URL')
      }
    }

    const res = await complianceApi.triggerScan(mockApiClient as any)
    assert.ok(res)
    assert.deepEqual(calledEndpoints, ['/compliance/scan', '/audit/run'], 'Must attempt /compliance/scan first and fallback to /audit/run on 404')
  })

  it('Strict line counts: SLO and Audit files satisfy constraints (< 300 lines component, <= 350 lines views, < 500 lines ts/css)', () => {
    const checks = [
      { file: '../src/views/SLOView.vue', max: 350 },
      { file: '../src/views/AuditView.vue', max: 350 },
      { file: '../src/components/slo/SloBurnDownBoard.vue', max: 300 },
      { file: '../src/composables/useSLOMonitor.ts', max: 500 },
      { file: '../src/composables/useCompliance.ts', max: 500 },
      { file: '../src/api/compliance.ts', max: 500 },
      { file: '../src/types/slo.ts', max: 500 },
    ]

    for (const { file, max } of checks) {
      const fullPath = path.resolve(import.meta.dirname, file)
      if (fs.existsSync(fullPath)) {
        const lines = fs.readFileSync(fullPath, 'utf-8').split('\n').length
        assert.ok(lines <= max, `${file} has ${lines} lines, exceeding max ${max}`)
      }
    }
  })
})
