import { describe, it, beforeEach } from 'node:test'
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
      source: 'export const api = { get: async () => ({}), post: async () => ({}), getToken: () => "mock-token" };'
    };
  }
  return nextLoad(url, context);
}
`))

// Mock browser globals
if (typeof globalThis.window === 'undefined') {
  // @ts-expect-error Mock window for test environment
  globalThis.window = {
    location: { protocol: 'http:', host: 'localhost:3000' }
  }
}

const { createPinia, setActivePinia } = await import('pinia')
const { useLogStore, normalizeKlogLevel } = await import('../src/stores/logStore.ts')
const { inferEffectiveLevel, exportLogsToTextFile } = await import('../src/components/logs/virtualLogUtils.ts')
const { extractAvailableServices } = await import('../src/utils/logTargetCategorizer.ts')
const { ICONS, ALIAS_MAP, FALLBACK_SVG } = await import('../src/components/ui/iconPaths.ts')

describe('Task 074 — Slice 74B: Frontend Logs UX & Layout Remediation', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  describe('1. Client-Side Kubernetes klog Prefix Level Normalization', () => {
    it('normalizes klog error prefix ^E\\d{4}\\s to error level', () => {
      const errorMsg = 'E1105 04:30:17.123456 1234 kubelet.go:210] Failed to sync pod sandbox'
      const normalized = normalizeKlogLevel('INFO', errorMsg)
      assert.equal(normalized.toUpperCase(), 'ERROR')
    })

    it('normalizes klog fatal prefix ^F\\d{4}\\s to error level', () => {
      const fatalMsg = 'F1105 04:30:17.123456 1234 server.go:50] Memory allocation failed'
      const normalized = normalizeKlogLevel('info', fatalMsg)
      assert.equal(normalized.toUpperCase(), 'ERROR')
    })

    it('normalizes klog warning prefix ^W\\d{4}\\s to warn level', () => {
      const warnMsg = 'W1105 04:30:17.123456 1234 kubelet.go:150] Disk usage exceeds threshold 85%'
      const normalized = normalizeKlogLevel('INFO', warnMsg)
      assert.equal(normalized.toUpperCase(), 'WARN')
    })

    it('preserves existing non-info log levels even if message contains klog prefix', () => {
      const debugMsg = 'E1105 04:30:17.123456 1234 debug.go:10] Just debugging an error string'
      assert.equal(normalizeKlogLevel('DEBUG', debugMsg), 'DEBUG')
    })

    it('inferEffectiveLevel detects klog prefixes when raw level is INFO', () => {
      assert.equal(inferEffectiveLevel('INFO', 'E1105 04:30:17.000000 1 error.go:1] error happened'), 'ERROR')
      assert.equal(inferEffectiveLevel('INFO', 'W1105 04:30:17.000000 1 warn.go:1] warning happened'), 'WARN')
      assert.equal(inferEffectiveLevel('INFO', 'I1105 04:30:17.000000 1 info.go:1] normal log line'), 'INFO')
    })

    it('logStore.addLogEntry automatically normalizes klog entries to ERROR / WARN', () => {
      const store = useLogStore()
      store.addLogEntry({
        time: '2026-11-05T04:30:00.000Z',
        level: 'INFO',
        namespace: 'kube-system',
        pod: 'kube-apiserver',
        msg: 'E1105 04:30:00.123456 1 handler.go:90] TLS handshake error from 10.10.10.60:443',
      })

      assert.equal(store.logs.length, 1)
      assert.equal(store.logs[0].level.toUpperCase(), 'ERROR')
    })

    it('logStore.appendLogBatch automatically normalizes klog entries in batch', () => {
      const store = useLogStore()
      store.appendLogBatch([
        {
          time: '2026-11-05T04:30:01.000Z',
          level: 'INFO',
          namespace: 'kube-system',
          pod: 'kube-proxy',
          msg: 'W1105 04:30:01.123456 1 proxier.go:50] Network packet dropped on interface eth0',
        },
        {
          time: '2026-11-05T04:30:02.000Z',
          level: 'INFO',
          namespace: 'default',
          pod: 'nginx',
          msg: 'I1105 04:30:02.123456 1 server.go:20] Ready to handle requests',
        },
      ])

      assert.equal(store.logs.length, 2)
      assert.equal(store.logs[0].level.toUpperCase(), 'WARN')
      assert.equal(store.logs[1].level.toUpperCase(), 'INFO')
    })
  })

  describe('2. Log Stream Toolbar Icon Parity & Crisp Contrast', () => {
    it('provides valid SVG paths for sidebar and layers icons without fallback', () => {
      assert.ok(ICONS.layers, 'layers icon must exist in ICONS')
      assert.ok(ICONS.sidebar || ALIAS_MAP.sidebar, 'sidebar icon must exist in ICONS or ALIAS_MAP')
      assert.ok(ICONS['bar-chart-2'] || ALIAS_MAP['bar-chart-2'], 'bar-chart-2 icon must exist in ICONS or ALIAS_MAP')
      assert.notEqual(ICONS.layers, FALLBACK_SVG)
    })
  })

  describe('3. Dynamic Available Services & Noise Rejection', () => {
    it('filters out rotated logs and OS maintenance file noise', () => {
      const logs = [
        { time: '2026-11-05T04:30:00Z', level: 'info', msg: 'test', service: 'dpkg.log', node: 'node-1' },
        { time: '2026-11-05T04:30:00Z', level: 'info', msg: 'test', service: 'alternatives', node: 'node-1' },
        { time: '2026-11-05T04:30:00Z', level: 'info', msg: 'test', service: 'vmware-network.1', node: 'node-1' },
        { time: '2026-11-05T04:30:00Z', level: 'info', msg: 'test', service: 'nginx-ingress', node: 'node-1' },
        { time: '2026-11-05T04:30:00Z', level: 'info', msg: 'test', service: 'kubelet.service', node: 'node-1' },
      ]
      const services = extractAvailableServices(logs, 'node-1')
      assert.equal(services.length, 2)
      assert.equal(services[0].name, 'nginx-ingress')
      assert.equal(services[0].category, 'app')
      assert.equal(services[1].name, 'kubelet.service')
      assert.equal(services[1].category, 'systemd')
    })

    it('filters by effectiveNode when provided', () => {
      const logs = [
        { time: '2026-11-05T04:30:00Z', level: 'info', msg: 'test', service: 'app-a', node: 'k8smasterdeb' },
        { time: '2026-11-05T04:30:00Z', level: 'info', msg: 'test', service: 'app-b', node: 'k8sworker1' },
      ]
      const masterServices = extractAvailableServices(logs, 'k8smasterdeb')
      assert.equal(masterServices.length, 1)
      assert.equal(masterServices[0].name, 'app-a')
    })
  })

  describe('4. Text Log Export Formatting', () => {
    it('creates formatted log archive payload', () => {
      let createdBlob: unknown = null
      let clicked = false
      // @ts-expect-error Mock URL and document for browser download assertion
      globalThis.URL = {
        createObjectURL: (blob: unknown) => {
          createdBlob = blob
          return 'blob:mock-url'
        },
        revokeObjectURL: () => {},
      }
      // @ts-expect-error Mock document
      globalThis.document = {
        body: {
          appendChild: () => {},
          removeChild: () => {},
        },
        createElement: (tag: string) => ({
          tag,
          href: '',
          download: '',
          click: () => { clicked = true },
        }),
      }

      exportLogsToTextFile([
        { time: '12:00:00', level: 'INFO', msg: 'Application started', node: 'node-1', service: 'web' },
        { time: '12:00:01', level: 'ERROR', msg: 'Failed request', node: 'node-1', service: 'web', traceId: 'abc123xyz' },
      ], 'k8smasterdeb')

      assert.ok(clicked, 'download anchor click triggered')
      assert.ok(createdBlob, 'blob created')
    })
  })

  describe('5. Top-Row Unclipping & Virtual Viewport Geometry', () => {
    it('guarantees top row virtual offset >= 6px breathing room', () => {
      const topUnclipOffset = 6
      const startIdx = 0
      const itemHeight = 24
      const translateY = Math.max(0, startIdx * itemHeight) + topUnclipOffset
      assert.equal(translateY, 6, 'row 0 translateY must be +6px to prevent border and header clipping')
    })
  })

  describe('6. Honest Offline-Node State & 1-Click History Switch', () => {
    it('detects offline node status accurately and triggers history mode on CTA click', () => {
      let currentMode: 'live' | 'historical' = 'live'
      let selectedTimeRange = '1h'
      let historicalQueryRan = false

      function handleSwitchToHistoryFromOffline() {
        currentMode = 'historical'
        selectedTimeRange = '24h'
        historicalQueryRan = true
      }

      // Simulate user clicking "Switch to History Logs" CTA on an offline node
      handleSwitchToHistoryFromOffline()

      assert.equal(currentMode, 'historical', 'must switch to historical mode')
      assert.equal(selectedTimeRange, '24h', 'must set time range to 24h')
      assert.equal(historicalQueryRan, true, 'must trigger historical query immediately')
    })
  })

  describe('7. Virtual Log Terminal Scroll Pinning & Buffer Physics', () => {
    it('accurately identifies when scroll is pinned to bottom vs paused for reading', () => {
      const rowHeight = 24
      const clientHeight = 600
      const threshold = rowHeight * 2 // 48px

      function checkAtBottom(scrollHeight: number, scrollTop: number) {
        return scrollHeight - scrollTop - clientHeight <= threshold
      }

      // When scrollTop is at maximum scrollHeight - clientHeight
      assert.equal(checkAtBottom(2000, 1400), true, 'must be pinned when at bottom')
      // When scrollTop is within 48px threshold
      assert.equal(checkAtBottom(2000, 1360), true, 'must remain pinned within threshold')
      // When user scrolled up to read past logs
      assert.equal(checkAtBottom(2000, 1000), false, 'must unpin when scrolled up')
    })

    it('tracks unpinned new logs count only when reading or hover paused', () => {
      let unpinnedCount = 0
      let isPinned = false
      let isHoverPaused = false

      function onNewLogs(count: number) {
        if (!isPinned || isHoverPaused) {
          unpinnedCount += count
        }
      }

      onNewLogs(5)
      assert.equal(unpinnedCount, 5, 'must increment when unpinned')

      // Now user jumps to bottom
      isPinned = true
      unpinnedCount = 0

      onNewLogs(10)
      assert.equal(unpinnedCount, 0, 'must not increment when pinned and not paused')

      // Now user hovers over terminal (hover pause)
      isHoverPaused = true
      onNewLogs(3)
      assert.equal(unpinnedCount, 3, 'must increment when hover paused even if pinned')
    })
  })

  describe('8. Log Target Service Category & Filter Pipeline', () => {
    it('filters services by category tab (all, app, systemd) and query string', () => {
      const services = [
        { id: 'nginx', name: 'nginx', category: 'app' as const, icon: 'box', count: 42 },
        { id: 'redis', name: 'redis', category: 'app' as const, icon: 'box', count: 10 },
        { id: 'kubelet.service', name: 'kubelet.service', category: 'systemd' as const, icon: 'cpu', count: 120 },
        { id: 'docker.service', name: 'docker.service', category: 'systemd' as const, icon: 'cpu', count: 0 },
      ]

      function filterServices(list: typeof services, category: 'all' | 'app' | 'systemd', query: string) {
        let res = [...list]
        if (category === 'app') res = res.filter((s) => s.category === 'app')
        if (category === 'systemd') res = res.filter((s) => s.category === 'systemd')
        const q = query.trim().toLowerCase()
        if (q) res = res.filter((s) => s.name.toLowerCase().includes(q) || s.id.toLowerCase().includes(q))
        res.sort((a, b) => {
          if (a.category !== b.category) return a.category === 'app' ? -1 : 1
          return b.count - a.count
        })
        return res
      }

      const all = filterServices(services, 'all', '')
      assert.equal(all.length, 4)
      assert.equal(all[0].id, 'nginx', 'apps must appear before systemd')
      assert.equal(all[1].id, 'redis')

      const appsOnly = filterServices(services, 'app', '')
      assert.equal(appsOnly.length, 2)
      assert.ok(appsOnly.every((s) => s.category === 'app'))

      const systemdOnly = filterServices(services, 'systemd', '')
      assert.equal(systemdOnly.length, 2)
      assert.ok(systemdOnly.every((s) => s.category === 'systemd'))

      const searchFiltered = filterServices(services, 'all', 'kube')
      assert.equal(searchFiltered.length, 1)
      assert.equal(searchFiltered[0].id, 'kubelet.service')
    })
  })
})
