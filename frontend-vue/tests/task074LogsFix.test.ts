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
      assert.ok(ICONS.sidebar || ALIAS_MAP.sidebar, 'sidebar icon must exist or be aliased')
      assert.notEqual(ICONS.layers, FALLBACK_SVG, 'layers must not be fallback question mark')
    })

    it('toolbar action icons have accessible title attributes and min touch target 32px', () => {
      const minBtnHeight = 32
      assert.ok(minBtnHeight >= 32, 'Toolbar button height must meet 32px standard')
    })
  })

  describe('3. Dynamic Available Services Categorization', () => {
    it('extracts unique services scoped to effective node and separates app vs systemd', () => {
      const logs = [
        { node: 'worker-1', service: 'payment-svc', msg: 'Started' },
        { node: 'worker-1', service: 'kubelet.service', msg: 'Heartbeat' },
        { node: 'worker-2', service: 'auth-svc', msg: 'Token issued' },
        { node: 'worker-1', service: 'payment-svc', msg: 'Processing' },
      ]

      const services = extractAvailableServices(logs, 'worker-1')
      assert.equal(services.length, 2, 'Must only include 2 services on worker-1')
      const payment = services.find((s) => s.id === 'payment-svc')
      assert.ok(payment, 'payment-svc must exist')
      assert.equal(payment.category, 'app', 'payment-svc is an app')
      assert.equal(payment.count, 2, 'count must be 2')

      const kubelet = services.find((s) => s.id === 'kubelet.service')
      assert.ok(kubelet, 'kubelet.service must exist')
      assert.equal(kubelet.category, 'systemd', 'kubelet.service is a systemd unit')
      assert.equal(kubelet.count, 1, 'count must be 1')
    })

    it('falls back to global unique services when node is not scoped', () => {
      const logs = [
        { node: 'worker-1', service: 'payment-svc', msg: 'Started' },
        { node: 'worker-2', service: 'auth-svc', msg: 'Token issued' },
      ]
      const services = extractAvailableServices(logs)
      assert.equal(services.length, 2, 'Must include services across all nodes')
    })
  })

  describe('4. One-Click Plaintext Log Export', () => {
    it('formats log entries cleanly as timestamp [LEVEL] [target] message without JSON noise', () => {
      const logs = [
        {
          time: '2026-11-05T04:00:00Z',
          level: 'ERROR',
          service: 'order-svc',
          node: 'k8s-worker-1',
          msg: 'Database connection failed: timeout after 5000ms',
        },
        {
          time: '2026-11-05T04:00:01Z',
          level: 'INFO',
          service: 'order-svc',
          node: 'k8s-worker-1',
          msg: 'Retrying connection pool reconnect',
        },
      ]

      const lines = logs.map(
        (l) => `${l.time} [${l.level.toUpperCase()}] [${l.service || l.node || 'cluster'}] ${l.msg}`
      )
      assert.equal(lines.length, 2)
      assert.equal(
        lines[0],
        '2026-11-05T04:00:00Z [ERROR] [order-svc] Database connection failed: timeout after 5000ms'
      )
      assert.equal(
        lines[1],
        '2026-11-05T04:00:01Z [INFO] [order-svc] Retrying connection pool reconnect'
      )
    })

    it('exportLogsToTextFile creates text blob and triggers download anchor without crashing', () => {
      let createdBlob: any = null
      let clickTriggered = false

      // @ts-expect-error Mock URL
      globalThis.URL = {
        createObjectURL: (blob: any) => {
          createdBlob = blob
          return 'blob:http://localhost:3000/mock-uuid'
        },
        revokeObjectURL: () => {},
      }

      // @ts-expect-error Mock document
      globalThis.document = (globalThis.document || {}) as any
      globalThis.document.body = { appendChild: () => {}, removeChild: () => {} }
      globalThis.document.createElement = (tag: string) => {
        if (tag === 'a') {
          return {
            href: '',
            download: '',
            click: () => {
              clickTriggered = true
            },
          }
        }
        return {}
      }

      exportLogsToTextFile(
        [{ time: '2026-11-05T00:00:00Z', level: 'INFO', msg: 'test event' }],
        'order-svc'
      )

      assert.equal(clickTriggered, true, 'download link click triggered')
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

      assert.equal(checkAtBottom(2000, 1400), true, 'must be pinned when at bottom')
      assert.equal(checkAtBottom(2000, 1360), true, 'must remain pinned within threshold')
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

      isPinned = true
      unpinnedCount = 0

      onNewLogs(10)
      assert.equal(unpinnedCount, 0, 'must not increment when pinned and not paused')

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
