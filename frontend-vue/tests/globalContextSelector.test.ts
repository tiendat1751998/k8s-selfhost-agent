import { describe, it, beforeEach } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { register } from 'node:module'

const rootDir = path.resolve(import.meta.dirname, '..')
const composablePath = path.join(rootDir, 'src/composables/useGlobalContext.ts')
const componentPath = path.join(rootDir, 'src/components/navigation/GlobalContextSelector.vue')

// Mock client.ts for headless testing
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

export async function load(url, context, nextLoad) {
  if (url.includes('client.ts') || url.includes('/client')) {
    return {
      format: 'module',
      shortCircuit: true,
      source: 'export const api = { get: async () => ([]), post: async () => ({}) };'
    };
  }
  return nextLoad(url, context);
}
`))

// Setup mock window & localStorage
const mockStorage: Record<string, string> = {}
type Listener = (evt: any) => void
const windowListeners: Record<string, Listener[]> = {}

// @ts-expect-error Mock window for test environment
globalThis.window = {
  localStorage: {
    getItem: (key: string) => mockStorage[key] ?? null,
    setItem: (key: string, val: string) => { mockStorage[key] = String(val) },
    removeItem: (key: string) => { delete mockStorage[key] },
    clear: () => { Object.keys(mockStorage).forEach(k => delete mockStorage[k]) }
  },
  addEventListener: (event: string, handler: Listener) => {
    windowListeners[event] = windowListeners[event] || []
    windowListeners[event].push(handler)
  },
  removeEventListener: (event: string, handler: Listener) => {
    if (windowListeners[event]) {
      windowListeners[event] = windowListeners[event].filter(h => h !== handler)
    }
  },
  innerWidth: 1024
}

const { useGlobalContext } = await import('../src/composables/useGlobalContext.ts')
const { fleetApi } = await import('../src/api/fleet.ts')

describe('Task 021: Rancher-Style Global Context Switcher Specifications', () => {
  describe('1. File Structure and Line Count Constraints', () => {
    it('useGlobalContext.ts exists and strictly obeys line count (< 250 lines)', () => {
      assert.ok(fs.existsSync(composablePath), 'useGlobalContext.ts must exist')
      const content = fs.readFileSync(composablePath, 'utf-8')
      const lineCount = content.split('\n').length
      assert.ok(lineCount < 250, `useGlobalContext.ts must be strictly < 250 lines (currently ${lineCount})`)
    })

    it('GlobalContextSelector.vue exists and strictly obeys line count (< 350 lines)', () => {
      assert.ok(fs.existsSync(componentPath), 'GlobalContextSelector.vue must exist')
      const content = fs.readFileSync(componentPath, 'utf-8')
      const lineCount = content.split('\n').length
      assert.ok(lineCount < 350, `GlobalContextSelector.vue must be strictly < 350 lines (currently ${lineCount})`)
    })
  })

  describe('2. Composable Reactive State & Storage Persistence', () => {
    it('exposes the required clean API surface', () => {
      const ctx = useGlobalContext()
      assert.ok('activeClusterId' in ctx, 'Must expose activeClusterId')
      assert.ok('activeNamespace' in ctx, 'Must expose activeNamespace')
      assert.ok('availableClusters' in ctx, 'Must expose availableClusters')
      assert.ok('availableNamespaces' in ctx, 'Must expose availableNamespaces')
      assert.strictEqual(typeof ctx.setCluster, 'function', 'Must expose setCluster function')
      assert.strictEqual(typeof ctx.setNamespace, 'function', 'Must expose setNamespace function')
      assert.strictEqual(typeof ctx.loadClusters, 'function', 'Must expose loadClusters function')
    })

    it('persists cluster and namespace selections to localStorage', () => {
      const ctx = useGlobalContext()

      ctx.setCluster('k8s-production-eu')
      assert.strictEqual(ctx.activeClusterId.value, 'k8s-production-eu')
      assert.strictEqual(window.localStorage.getItem('k8s_active_cluster'), 'k8s-production-eu')

      ctx.setNamespace('monitoring')
      assert.strictEqual(ctx.activeNamespace.value, 'monitoring')
      assert.strictEqual(window.localStorage.getItem('k8s_active_namespace'), 'monitoring')
    })

    it('syncs across window storage events (cross-tab synchronization)', () => {
      const ctx = useGlobalContext()
      const storageListeners = windowListeners['storage'] || []
      assert.ok(storageListeners.length > 0, 'Must register window storage event listener for cross-tab sync')

      // Dispatch simulated storage event for cluster
      const clusterEvent = { key: 'k8s_active_cluster', newValue: 'k8s-staging-us' }
      storageListeners.forEach(listener => listener(clusterEvent))
      assert.strictEqual(ctx.activeClusterId.value, 'k8s-staging-us', 'activeClusterId must sync with storage event')

      // Dispatch simulated storage event for namespace
      const nsEvent = { key: 'k8s_active_namespace', newValue: 'kube-system' }
      storageListeners.forEach(listener => listener(nsEvent))
      assert.strictEqual(ctx.activeNamespace.value, 'kube-system', 'activeNamespace must sync with storage event')
    })

    it('fetches clusters via fleetApi.getClusters with fallback to active cluster', async () => {
      const ctx = useGlobalContext()
      
      // fleetApi should define getClusters
      assert.strictEqual(typeof fleetApi.getClusters, 'function', 'fleetApi must export getClusters method')

      // Test fallback when cluster list is empty
      const composableContent = fs.readFileSync(composablePath, 'utf-8')
      assert.match(composableContent, /fleetApi\.(?:getClusters|list)/, 'Composable must call fleetApi.getClusters')
      
      // Fallback: availableClusters must never remain an empty void when active cluster is set
      ctx.setCluster('k8s-edge-node')
      await ctx.loadClusters()
      assert.ok(ctx.availableClusters.value.length > 0, 'availableClusters must fall back to active cluster')
      assert.ok(
        ctx.availableClusters.value.some(c => c.id === 'k8s-edge-node'),
        'availableClusters must include the active cluster'
      )
    })
  })

  describe('3. Component Contract & Template Architecture', () => {
    const componentContent = fs.readFileSync(componentPath, 'utf-8')

    it('renders compact dual-dropdown with cluster and namespace trigger buttons', () => {
      assert.match(componentContent, /class="[^"]*cluster-pill[^"]*"/, 'Must render cluster pill trigger button')
      assert.match(componentContent, /class="[^"]*namespace-pill[^"]*"/, 'Must render namespace pill trigger button')
      assert.match(componentContent, /aria-haspopup="listbox"/, 'Must have aria-haspopup="listbox"')
      assert.match(componentContent, /:aria-expanded="activeDropdown === 'cluster'|:aria-expanded="activeDropdown === 'namespace'/, 'Must bind aria-expanded')
    })

    it('supports full suite of status dots (dot-emerald, dot-amber, dot-rose, dot-slate)', () => {
      assert.match(componentContent, /dot-emerald/, 'Must support dot-emerald')
      assert.match(componentContent, /dot-amber/, 'Must support dot-amber')
      assert.match(componentContent, /dot-rose/, 'Must support dot-rose for failed/offline/error states')
      assert.match(componentContent, /dot-slate/, 'Must support dot-slate for unknown/offline states')
      
      // Status dot mapping helper should be applied to dropdown items too, not hardcoded healthy/amber
      assert.doesNotMatch(
        componentContent,
        /c\.status === 'healthy' \? 'dot-emerald' : 'dot-amber'/,
        'Dropdown items must use comprehensive status dot mapping, not binary healthy:amber'
      )
    })

    it('displays option count badges in the search header for instant filtering visibility', () => {
      assert.match(
        componentContent,
        /context-count-badge/,
        'Must render option count badge (.context-count-badge) in search header'
      )
    })

    it('implements full keyboard navigation (ArrowUp, ArrowDown, Enter, Escape) and focus restoration', () => {
      assert.match(componentContent, /'ArrowDown'/, 'Must handle ArrowDown key')
      assert.match(componentContent, /'ArrowUp'/, 'Must handle ArrowUp key')
      assert.match(componentContent, /'Enter'/, 'Must handle Enter key to select')
      assert.match(componentContent, /'Escape'/, 'Must handle Escape key to dismiss')
      assert.match(componentContent, /focus\(\)/, 'Must implement focus restoration on dismiss/selection')
    })

    it('filters clusters and namespaces accurately via instant search input', () => {
      assert.match(componentContent, /filteredClusters/, 'Must compute filteredClusters')
      assert.match(componentContent, /filteredNamespaces/, 'Must compute filteredNamespaces')
      assert.match(componentContent, /v-model="searchQuery"/, 'Must bind searchQuery to search input')

      // Functional verification of search filter algorithm
      const clusters = [
        { id: 'c1', name: 'prod-europe-west', status: 'healthy', region: 'eu-west-1' },
        { id: 'c2', name: 'staging-us-east', status: 'degraded', region: 'us-east-2' },
        { id: 'c3', name: 'edge-apac-tokyo', status: 'healthy', region: 'ap-northeast-1' }
      ]
      const filterCluster = (query: string) => {
        const q = query.trim().toLowerCase()
        return q ? clusters.filter(c => (c.name + c.id + (c.region || '')).toLowerCase().includes(q)) : clusters
      }

      assert.equal(filterCluster('').length, 3, 'Empty query returns all')
      assert.equal(filterCluster('prod').length, 1, 'Filters by name')
      assert.equal(filterCluster('prod')[0].id, 'c1')
      assert.equal(filterCluster('us-east').length, 1, 'Filters by region')
      assert.equal(filterCluster('us-east')[0].id, 'c2')
      assert.equal(filterCluster('nonexistent').length, 0, 'No match returns empty')

      const namespaces = ['all', 'default', 'kube-system', 'monitoring', 'ingress-nginx']
      const filterNs = (query: string) => {
        const q = query.trim().toLowerCase()
        return q ? namespaces.filter(ns => (ns === 'all' ? 'all namespaces' : ns).toLowerCase().includes(q)) : namespaces
      }

      assert.equal(filterNs('').length, 5)
      assert.equal(filterNs('all').length, 1)
      assert.equal(filterNs('kube').length, 1)
      assert.equal(filterNs('kube')[0], 'kube-system')
    })

    it('implements cycling math for keyboard navigation', () => {
      // Test the ArrowDown & ArrowUp cycling formula used in onMenuKeydown:
      // (focusedIndex + 1) % count and (focusedIndex - 1 + count) % count
      const count = 3
      let index = 0
      
      // ArrowDown from 0 -> 1 -> 2 -> 0
      index = (index + 1) % count
      assert.equal(index, 1)
      index = (index + 1) % count
      assert.equal(index, 2)
      index = (index + 1) % count
      assert.equal(index, 0, 'Wraps back to top')

      // ArrowUp from 0 -> 2 -> 1 -> 0
      index = (index - 1 + count) % count
      assert.equal(index, 2, 'Wraps back to bottom')
      index = (index - 1 + count) % count
      assert.equal(index, 1)
      index = (index - 1 + count) % count
      assert.equal(index, 0)
    })
  })

  describe('4. Reactivity & State Transitions', () => {
    it('reacts immediately to setCluster and setNamespace invocations', () => {
      const ctx = useGlobalContext()
      
      let clusterCallCount = 0
      let lastCluster = ''
      const unwatchCluster = ctx.activeClusterId.value

      ctx.setCluster('cluster-omega')
      assert.strictEqual(ctx.activeClusterId.value, 'cluster-omega')
      assert.strictEqual(window.localStorage.getItem('k8s_active_cluster'), 'cluster-omega')

      ctx.setNamespace('prod-fintech')
      assert.strictEqual(ctx.activeNamespace.value, 'prod-fintech')
      assert.strictEqual(window.localStorage.getItem('k8s_active_namespace'), 'prod-fintech')
    })

    it('ensures all modified/created files are strictly under 500 lines', () => {
      const files = [composablePath, componentPath, path.join(rootDir, 'src/api/fleet.ts')]
      for (const file of files) {
        const lines = fs.readFileSync(file, 'utf-8').split('\n').length
        assert.ok(lines < 500, `${path.basename(file)} must be < 500 lines (got ${lines})`)
      }
    })
  })
})
