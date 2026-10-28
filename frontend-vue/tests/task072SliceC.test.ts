import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { register } from 'node:module'
import { createPinia, setActivePinia } from 'pinia'

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

setActivePinia(createPinia())

const { buildSettingsTabsList } = await import('../src/composables/useSettings.ts')
const { incidentsApi } = await import('../src/api/incidents.ts')
const { sloApi } = await import('../src/api/slo.ts')
const { isSimulationAllowed, useIncidents } = await import('../src/composables/useIncidents.ts')
const { useAutomationEngine } = await import('../src/composables/useAutomationEngine.ts')

describe('Task 072 - Subtask 72C Behavioral Acceptance Tests (Zero fs.readFileSync)', () => {
  describe('1. Incidents Command & RCA Protection (FUNC-04, UI-07, UI-08)', () => {
    it('incidentsApi defines acknowledge and getPostmortem methods', () => {
      assert.equal(typeof (incidentsApi as any).acknowledge, 'function', 'incidentsApi.acknowledge must be defined')
      assert.equal(typeof (incidentsApi as any).getPostmortem, 'function', 'incidentsApi.getPostmortem must be defined')
    })

    it('isSimulationAllowed honors debug=1 query safely', () => {
      assert.equal(isSimulationAllowed({ debug: '1' }), true)
      assert.equal(isSimulationAllowed({ debug: '0' }), false)
      assert.equal(isSimulationAllowed(null), false)
      assert.equal(isSimulationAllowed(undefined), false)
    })

    it('DEF-005: useIncidents prevents AI RCA execution when incident status is resolved', async () => {
      const { triggerAIAnalysis, toastMessage } = useIncidents()
      const resolvedIncident: any = {
        id: 'inc-resolved-1',
        pod_name: 'payment-svc-78f9',
        namespace: 'prod',
        cluster_name: 'prod-cluster',
        type: 'CrashLoopBackOff',
        severity: 'critical',
        status: 'resolved',
        created_at: new Date().toISOString()
      }

      await triggerAIAnalysis(resolvedIncident)
      assert.ok(toastMessage.value, 'Toast message should be populated')
      assert.equal(toastMessage.value?.type, 'error')
      assert.match(toastMessage.value?.text || '', /already resolved/i)
      assert.equal(resolvedIncident.status, 'resolved', 'Incident status must not mutate to analyzing')
    })
  })

  describe('2. Settings Tabs List & Backup Integration (FUNC-06, UI-08)', () => {
    it('buildSettingsTabsList includes Backup & DR tab with backup category', () => {
      const tabs = buildSettingsTabsList([])
      const backupTab = tabs.find((t: any) => t.id === 'backup')
      assert.ok(backupTab, 'Backup tab must be in tabs list')
      assert.equal(backupTab?.label, 'Backup & DR')
      assert.equal(backupTab?.category, 'backup')
    })
  })

  describe('3. SLO Edit Workflow Contracts (FUNC-05)', () => {
    it('sloApi provides updateDefinition', () => {
      assert.equal(typeof sloApi.updateDefinition, 'function', 'sloApi.updateDefinition must be defined')
    })
  })

  describe('4. Automation Engine Calculation (UI-06, FUNC-08)', () => {
    it('useAutomationEngine calculates savedEngineeringHours dynamically based on runs', () => {
      const engine = useAutomationEngine()
      engine.executions.value = [
        { id: 'ex-1', rule_id: 'rule-1', result: 'success' },
        { id: 'ex-2', rule_id: 'rule-1', result: 'success' },
        { id: 'ex-3', rule_id: 'rule-2', result: 'failed' },
        { id: 'ex-4', rule_id: 'rule-2', result: 'success' }
      ] as any

      assert.equal(engine.healingSuccessRate.value, 75)
      assert.equal(engine.savedEngineeringHours.value, '1.5', '3 successful * 0.5h = 1.5h')
    })
  })
})
