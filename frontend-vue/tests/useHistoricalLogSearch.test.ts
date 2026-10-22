import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { register } from 'node:module'
import { ref } from 'vue'

// Register loader hook for headless node resolution of typescript extensions
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

const { useHistoricalLogSearch } = await import('../src/composables/useHistoricalLogSearch.ts')
import type { LogEntry } from '../src/stores/logStore.ts'
import type { LogTarget } from '../src/composables/useLogTargets.ts'

describe('useHistoricalLogSearch targetFilteredLogs isolation', () => {
  it('filters noise, mutes services, filters by category, and isolates node + service', () => {
    const logs = ref<LogEntry[]>([
      {
        time: '2026-10-22T00:00:01Z',
        level: 'INFO',
        node: 'k8smater133',
        service: 'init.scope',
        msg: 'noise line 1'
      },
      {
        time: '2026-10-22T00:00:02Z',
        level: 'INFO',
        node: 'k8smater133',
        service: 'traefik.service',
        msg: 'traefik normal log'
      },
      {
        time: '2026-10-22T00:00:03Z',
        level: 'INFO',
        node: 'k8smater133',
        service: 'postgres_db',
        msg: 'SELECT 1;'
      },
      {
        time: '2026-10-22T00:00:04Z',
        level: 'INFO',
        node: 'k8sworker',
        service: 'postgres_db',
        msg: 'worker postgres'
      }
    ])

    const mockStore: any = {
      get logs() { return logs.value },
      fetchHistoricalLogs: async () => ({}),
      fetchHistogram: async () => ([]),
      histogram: ref([]),
      isHistoricalLoading: ref(false),
      isHistogramLoading: ref(false),
      totalHistoricalCount: ref(0),
      hasMoreHistorical: ref(false),
    }

    const selectedTarget = ref<LogTarget>({
      type: 'node',
      id: 'k8smater133',
      name: 'k8smater133',
    })
    const searchKeyword = ref('')
    const selectedLevel = ref('ALL')

    const searchComposable = useHistoricalLogSearch(
      mockStore,
      selectedTarget,
      searchKeyword,
      selectedLevel
    )

    // 1. By default, init.scope noise is filtered out, leaving traefik.service and postgres_db on k8smater133
    const filtered1 = searchComposable.targetFilteredLogs.value
    assert.equal(filtered1.length, 2)
    assert.equal(filtered1.some(l => l.service === 'init.scope'), false)
    assert.equal(filtered1.some(l => l.node === 'k8sworker'), false)

    // 2. Muting traefik.service excludes it
    searchComposable.mutedServices.value = ['traefik.service']
    const filtered2 = searchComposable.targetFilteredLogs.value
    assert.equal(filtered2.length, 1)
    assert.equal(filtered2[0].service, 'postgres_db')

    // 3. Category filter: Apps Only
    searchComposable.mutedServices.value = []
    searchComposable.sourceCategory.value = 'app'
    const filteredApps = searchComposable.targetFilteredLogs.value
    assert.equal(filteredApps.length, 1)
    assert.equal(filteredApps[0].service, 'postgres_db')

    // 4. Category filter: SystemD Only
    searchComposable.sourceCategory.value = 'systemd'
    const filteredSystemd = searchComposable.targetFilteredLogs.value
    assert.equal(filteredSystemd.length, 1)
    assert.equal(filteredSystemd[0].service, 'traefik.service')

    // 5. Selecting specific child service
    searchComposable.sourceCategory.value = 'all'
    searchComposable.activeServiceSubFilter.value = 'postgres_db'
    const filteredService = searchComposable.targetFilteredLogs.value
    assert.equal(filteredService.length, 1)
    assert.equal(filteredService[0].service, 'postgres_db')
  })
})
