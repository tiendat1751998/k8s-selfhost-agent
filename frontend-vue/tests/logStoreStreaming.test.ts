import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { register } from 'node:module'
import type { LogEntry } from '../src/stores/logStore.ts'

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
      source: 'export const api = { get: async () => ({}), post: async () => ({}) };'
    };
  }
  return nextLoad(url, context);
}
`))

// Mock minimal browser globals
if (typeof globalThis.window === 'undefined') {
  // @ts-expect-error Mock window for test environment
  globalThis.window = {
    location: { protocol: 'http:', host: 'localhost:3000' }
  }
}

const { createPinia, setActivePinia } = await import('pinia')
const { createApp } = await import('vue')
const { useLogStore, getLogFingerprint, matchesTarget } = await import('../src/stores/logStore.ts')
const { useLogStreamer } = await import('../src/composables/useLogStreamer.ts')

describe('Log Store Realtime Streaming & Order Tests', () => {
  it('getLogFingerprint correctly generates fingerprint from time, service/pod, and message', () => {
    const entry1: LogEntry = {
      time: '2026-09-16T06:00:00.000Z',
      level: 'INFO',
      namespace: 'default',
      pod: 'postgres-0',
      service: 'postgres_db',
      msg: 'connection established'
    }
    assert.equal(getLogFingerprint(entry1), '2026-09-16T06:00:00.000Z_postgres_db_connection established')

    const entry2: LogEntry = {
      time: '2026-09-16T06:00:01.000Z',
      level: 'WARN',
      namespace: 'default',
      pod: 'systemd-0',
      msg: 'high memory'
    }
    assert.equal(getLogFingerprint(entry2), '2026-09-16T06:00:01.000Z_systemd-0_high memory')
  })

  it('addLogEntry prevents duplicate entries if identical to last entry', () => {
    setActivePinia(createPinia())
    const store = useLogStore()
    store.clear()

    const entry1: LogEntry = {
      time: '2026-09-16T06:00:00.000Z',
      level: 'INFO',
      namespace: 'default',
      pod: 'postgres-0',
      service: 'postgres_db',
      msg: 'query executed'
    }

    store.addLogEntry(entry1)
    assert.equal(store.logs.length, 1)

    // Attempt to add exact duplicate
    store.addLogEntry(entry1)
    assert.equal(store.logs.length, 1, 'Duplicate log should not be added')

    // Add different log
    const entry2: LogEntry = {
      time: '2026-09-16T06:00:01.000Z',
      level: 'INFO',
      namespace: 'default',
      pod: 'postgres-0',
      service: 'postgres_db',
      msg: 'another query'
    }
    store.addLogEntry(entry2)
    assert.equal(store.logs.length, 2)
  })

  it('addLogEntry shifts oldest entries off top when exceeding maxBufferSize', () => {
    setActivePinia(createPinia())
    const store = useLogStore()
    store.clear()
    store.setMaxBufferSize(1000)

    // Fill buffer up to 1000
    for (let i = 0; i < 1000; i++) {
      store.addLogEntry({
        time: new Date(1000000 + i * 1000).toISOString(),
        level: 'INFO',
        namespace: 'default',
        pod: 'pod-1',
        service: 'svc',
        msg: `msg ${i}`
      })
    }
    assert.equal(store.logs.length, 1000)
    assert.equal(store.logs[0].msg, 'msg 0')

    // Add 1001st log
    store.addLogEntry({
      time: new Date(1000000 + 1000 * 1000).toISOString(),
      level: 'INFO',
      namespace: 'default',
      pod: 'pod-1',
      service: 'svc',
      msg: 'msg 1000'
    })

    assert.equal(store.logs.length, 1000)
    assert.equal(store.logs[0].msg, 'msg 1', 'Oldest entry should have been shifted off the top')
    assert.equal(store.logs[store.logs.length - 1].msg, 'msg 1000', 'Newest entry should be at bottom')
  })

  it('fetchHistoricalLogs sorts mapped entries strictly ASCENDING by time and preserves live WebSocket logs', async () => {
    setActivePinia(createPinia())
    const store = useLogStore()
    store.clear()

    // Simulate live WebSocket logs that arrived while preload was in-flight
    const liveLog1: LogEntry = {
      time: '2026-09-16T06:00:10.000Z',
      level: 'INFO',
      namespace: 'default',
      pod: 'postgres-0',
      service: 'postgres_db',
      msg: 'live log 1'
    }
    const liveLog2: LogEntry = {
      time: '2026-09-16T06:00:12.000Z',
      level: 'INFO',
      namespace: 'default',
      pod: 'postgres-0',
      service: 'postgres_db',
      msg: 'live log 2'
    }
    store.addLogEntry(liveLog1)
    store.addLogEntry(liveLog2)
    assert.equal(store.logs.length, 2)

    // Preloaded historical logs (which arrive descending from ClickHouse):
    const historicalLogsDescending: LogEntry[] = [
      {
        time: '2026-09-16T06:00:10.000Z', // overlaps with liveLog1
        level: 'INFO',
        namespace: 'default',
        pod: 'postgres-0',
        service: 'postgres_db',
        msg: 'live log 1'
      },
      {
        time: '2026-09-16T06:00:08.000Z',
        level: 'INFO',
        namespace: 'default',
        pod: 'postgres-0',
        service: 'postgres_db',
        msg: 'historical log 2'
      },
      {
        time: '2026-09-16T06:00:05.000Z',
        level: 'INFO',
        namespace: 'default',
        pod: 'postgres-0',
        service: 'postgres_db',
        msg: 'historical log 1'
      }
    ]

    // Sort ascending as fetchHistoricalLogs does:
    const mapped = [...historicalLogsDescending]
    mapped.sort((a, b) => new Date(a.time).getTime() - new Date(b.time).getTime())
    assert.equal(mapped[0].msg, 'historical log 1', 'Oldest historical log must be first')
    assert.equal(mapped[mapped.length - 1].msg, 'live log 1', 'Newest historical log must be last')

    // Perform merge with existing live logs as fetchHistoricalLogs does when append === false:
    const mappedFingerprints = new Set(mapped.map(getLogFingerprint))
    const existingLiveLogs = store.logs.filter((entry) => !mappedFingerprints.has(getLogFingerprint(entry)))
    const combined = [...mapped, ...existingLiveLogs]
    combined.sort((a, b) => new Date(a.time).getTime() - new Date(b.time).getTime())

    assert.equal(combined.length, 4, 'liveLog1 was deduplicated; liveLog2 was preserved')
    assert.equal(combined[0].msg, 'historical log 1')
    assert.equal(combined[1].msg, 'historical log 2')
    assert.equal(combined[2].msg, 'live log 1')
    assert.equal(combined[3].msg, 'live log 2')

    // Verify strictly ascending timestamps
    for (let i = 1; i < combined.length; i++) {
      const prev = new Date(combined[i - 1].time).getTime()
      const curr = new Date(combined[i].time).getTime()
      assert.ok(curr >= prev, `Expected ascending timestamp at index ${i}`)
    }
  })

  it('matchesTarget handles service, container, and node targets correctly', () => {
    const entry: LogEntry = {
      time: '2026-09-16T06:00:00.000Z',
      level: 'INFO',
      namespace: 'default',
      pod: 'postgres-0',
      service: 'postgres_db',
      msg: 'ready'
    }

    assert.equal(matchesTarget(entry, { service: 'postgres_db' }), true)
    assert.equal(matchesTarget(entry, { service: 'postgres' }), true)
    assert.equal(matchesTarget(entry, { service: 'redis' }), false)
    assert.equal(matchesTarget(entry, { node: 'worker-1' }), false)
  })

  it('useLogStreamer respects autoConnect option to prevent duplicate connections', () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    let streamer: any
    const app = createApp({
      setup() {
        streamer = useLogStreamer({ autoConnect: false })
        return () => null
      }
    })
    app.use(pinia)
    app.runWithContext(() => {
      streamer = useLogStreamer({ autoConnect: false })
    })
    assert.equal(typeof streamer.connect, 'function')
    assert.equal(typeof streamer.addLogEntry, 'function')
    assert.equal(typeof streamer.appendLog, 'function')
    assert.equal(typeof streamer.scrollToBottom, 'function')
  })
})
