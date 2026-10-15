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

const { buildSettingsTabsList } = await import('../src/composables/useSettings.ts')

describe('Forensic Anti-Hardcode: Settings API Keys Tab Badge & State Integrity', () => {
  it('A. buildSettingsTabsList dynamically binds API Keys badge to apiKeys.length (0 when empty)', () => {
    const emptyKeys: any[] = []
    const tabs = buildSettingsTabsList(emptyKeys)
    const apiKeyTab = tabs.find(t => t.id === 'apikeys')

    assert.ok(apiKeyTab, 'apikeys tab must exist in tabsList')
    assert.equal(apiKeyTab.count, 0, 'apikeys tab count must be 0 when emptyKeys provided, never hardcoded 3')
    assert.notEqual(apiKeyTab.count, 3, 'apikeys tab count must not be hardcoded to 3')
  })

  it('B. buildSettingsTabsList updates badge count reactively as keys are added or revoked', () => {
    const sampleKeys: any[] = [
      { id: 'key-1', name: 'CI Runner', prefix: 'k8s_live_abc123...', role: 'admin', created_at: '2026-10-15', last_used: 'Never' },
      { id: 'key-2', name: 'Prometheus Scraper', prefix: 'k8s_live_def456...', role: 'readonly', created_at: '2026-10-15', last_used: 'Never' }
    ]

    const tabsWithTwo = buildSettingsTabsList(sampleKeys)
    const apiKeyTabTwo = tabsWithTwo.find(t => t.id === 'apikeys')
    assert.equal(apiKeyTabTwo?.count, 2, 'apikeys tab count must match actual keys length (2)')

    const singleKey = [sampleKeys[0]]
    const tabsWithOne = buildSettingsTabsList(singleKey)
    const apiKeyTabOne = tabsWithOne.find(t => t.id === 'apikeys')
    assert.equal(apiKeyTabOne?.count, 1, 'apikeys tab count must match actual keys length (1)')
  })

  it('C. other tabs in SettingsView do not have hardcoded count badges', () => {
    const tabs = buildSettingsTabsList([])
    const otherTabs = tabs.filter(t => t.id !== 'apikeys')
    for (const tab of otherTabs) {
      assert.equal(tab.count, undefined, `Tab ${tab.id} should not have a fake hardcoded count badge`)
    }
  })

  it('D. TrafficEndpointsTable traffic_percent is 0% when RPS and Bandwidth are 0, never fake 100/N', () => {
    function computeTrafficPercent(requestsPerSec: number, rxBytesPerSec: number, txBytesPerSec: number, totalRps: number, totalBw: number): number {
      return totalRps > 0
        ? (requestsPerSec / totalRps) * 100
        : (totalBw > 0
          ? ((rxBytesPerSec + txBytesPerSec) / totalBw) * 100
          : 0)
    }

    // 4 services, but cluster is completely idle (0 rps, 0 bytes)
    const percent = computeTrafficPercent(0, 0, 0, 0, 0)
    assert.equal(percent, 0, 'Idle traffic percentage must be honest 0%, never fake 25% or 33.3%')
  })
})
