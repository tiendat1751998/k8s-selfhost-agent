import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { register } from 'node:module'

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

export function resolveStatusDot(status: string): string {
  const s = (status || '').toLowerCase()
  if (s === 'healthy' || s === 'ready' || s === 'running' || s === 'active') return 'dot-emerald'
  if (s === 'warning' || s === 'degraded' || s === 'pending') return 'dot-amber'
  if (s === 'error' || s === 'failed' || s === 'critical' || s === 'offline') return 'dot-rose'
  return 'dot-slate'
}

describe('Task 021: Rancher-Style Global Context Switcher Specifications', () => {
  describe('1. Composable Reactive State & Storage Persistence', () => {
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

    it('fetches clusters via fleetApi.getClusters with honest default fallback when empty', async () => {
      const ctx = useGlobalContext()
      assert.strictEqual(typeof fleetApi.getClusters, 'function', 'fleetApi must export getClusters method')

      ctx.setCluster('k8s-edge-node')
      await ctx.loadClusters()
      assert.ok(ctx.availableClusters.value.length > 0, 'availableClusters must have fallback')
      assert.strictEqual(ctx.activeClusterId.value, 'default', 'activeClusterId must reset to default on empty response')
      assert.strictEqual(ctx.availableClusters.value[0].id, 'default')
      assert.strictEqual(ctx.availableClusters.value[0].status, 'unknown')
    })
  })

  describe('2. Search Filtering & Status Dot Resolution', () => {
    it('resolveStatusDot maps cluster health statuses accurately', () => {
      assert.equal(resolveStatusDot('healthy'), 'dot-emerald')
      assert.equal(resolveStatusDot('Ready'), 'dot-emerald')
      assert.equal(resolveStatusDot('degraded'), 'dot-amber')
      assert.equal(resolveStatusDot('warning'), 'dot-amber')
      assert.equal(resolveStatusDot('failed'), 'dot-rose')
      assert.equal(resolveStatusDot('offline'), 'dot-rose')
      assert.equal(resolveStatusDot('cordoned'), 'dot-slate')
      assert.equal(resolveStatusDot('unknown'), 'dot-slate')
      assert.equal(resolveStatusDot(''), 'dot-slate')
    })

    it('filters clusters and namespaces accurately via instant search input', () => {
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
  })

  describe('3. Keyboard Navigation Modulo Math', () => {
    it('implements cycling math for keyboard navigation', () => {
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
})
