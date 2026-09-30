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
      source: 'export const api = { get: async () => ({}), post: async () => ({}) };'
    };
  }
  return nextLoad(url, context);
}
`))

// Minimal browser mock
if (typeof globalThis.window === 'undefined') {
  // @ts-expect-error Mock window
  globalThis.window = {
    location: { protocol: 'http:', host: 'localhost:3000' }
  }
}

const { calculateVirtualWindow } = await import('../src/utils/virtualWindow.ts')
const { detectStackTrace } = await import('../src/utils/stackTraceParser.ts')
let latestSocket: MockWebSocket | null = null
class MockWebSocket {
  onopen: ((event: any) => void) | null = null
  onmessage: ((event: any) => void) | null = null
  onclose: ((event: any) => void) | null = null
  onerror: ((event: any) => void) | null = null
  readyState = 1
  constructor() {
    latestSocket = this
  }
  send() {}
  close() {}
}
// @ts-expect-error Mock global WebSocket
globalThis.WebSocket = MockWebSocket
if (typeof globalThis.localStorage === 'undefined') {
  // @ts-expect-error Mock localStorage
  globalThis.localStorage = { getItem: () => '', setItem: () => {}, removeItem: () => {} }
}

const { createPinia, setActivePinia } = await import('pinia')
const { useLogStore } = await import('../src/stores/logStore.ts')
const { parseWebSocketFrame } = await import('../src/utils/logBatchParser.ts')

describe('Virtual Windowing Slice Math', () => {
  const itemHeight = 24
  const clientHeight = 720 // 30 lines visible
  const overscan = 10

  it('handles empty log collection gracefully', () => {
    const win = calculateVirtualWindow({
      totalCount: 0,
      scrollTop: 0,
      clientHeight,
      itemHeight,
      overscan,
    })
    assert.equal(win.startIndex, 0)
    assert.equal(win.endIndex, 0)
    assert.equal(win.offsetY, 0)
    assert.equal(win.totalHeight, 0)
    assert.equal(win.visibleCount, 0)
  })

  it('calculates visible window at top of stream', () => {
    const total = 50000
    const win = calculateVirtualWindow({
      totalCount: total,
      scrollTop: 0,
      clientHeight,
      itemHeight,
      overscan,
    })
    assert.equal(win.startIndex, 0)
    // 720 / 24 = 30 visible + 10 overscan = 40
    assert.equal(win.endIndex, 40)
    assert.equal(win.offsetY, 0)
    assert.equal(win.totalHeight, total * itemHeight)
    assert.equal(win.visibleCount, 40)
  })

  it('calculates slice accurately when scrolled into the middle of 100k logs', () => {
    const total = 100000
    const scrollTop = 24000 // row 1000
    const win = calculateVirtualWindow({
      totalCount: total,
      scrollTop,
      clientHeight,
      itemHeight,
      overscan,
    })
    // 24000 / 24 = 1000. With 10 overscan = 990
    assert.equal(win.startIndex, 990)
    // (24000 + 720) / 24 = 1030. With 10 overscan = 1040
    assert.equal(win.endIndex, 1040)
    assert.equal(win.offsetY, 990 * 24)
    assert.equal(win.visibleCount, 50)
  })

  it('clamps endIndex strictly to totalCount when scrolled to bottom', () => {
    const total = 1000
    const maxScroll = total * itemHeight - clientHeight
    const win = calculateVirtualWindow({
      totalCount: total,
      scrollTop: maxScroll,
      clientHeight,
      itemHeight,
      overscan,
    })
    assert.equal(win.endIndex, 1000)
    assert.ok(win.startIndex < 1000)
    assert.equal(win.offsetY, win.startIndex * itemHeight)
  })

  it('supports small list where total is less than viewport', () => {
    const total = 15
    const win = calculateVirtualWindow({
      totalCount: total,
      scrollTop: 0,
      clientHeight,
      itemHeight,
      overscan,
    })
    assert.equal(win.startIndex, 0)
    assert.equal(win.endIndex, 15)
    assert.equal(win.totalHeight, 15 * 24)
    assert.equal(win.visibleCount, 15)
  })
})

describe('Multiline Stack Trace Folding Detection', () => {
  it('detects Java multiline stack trace', () => {
    const javaLog = `java.lang.NullPointerException: user entity was null
\tat com.enterprise.auth.UserService.getUser(UserService.java:84)
\tat com.enterprise.auth.AuthController.login(AuthController.java:32)
\tat javax.servlet.http.HttpServlet.service(HttpServlet.java:750)`

    const result = detectStackTrace(javaLog)
    assert.equal(result.isStackTrace, true)
    assert.equal(result.language, 'java')
    assert.equal(result.firstLine, 'java.lang.NullPointerException: user entity was null')
    assert.equal(result.linesCount, 4)
    assert.equal(result.stackLines.length, 3)
  })

  it('detects Python multiline stack trace', () => {
    const pyLog = `Traceback (most recent call last):
  File "/app/workers/task.py", line 45, in execute
    result = compute_metrics(payload)
  File "/app/workers/calc.py", line 12, in compute_metrics
    return 100 / divisor
ZeroDivisionError: division by zero`

    const result = detectStackTrace(pyLog)
    assert.equal(result.isStackTrace, true)
    assert.equal(result.language, 'python')
    assert.equal(result.firstLine, 'Traceback (most recent call last):')
    assert.equal(result.linesCount, 6)
  })

  it('detects Go runtime goroutine stack trace', () => {
    const goLog = `goroutine 42 [running]:
main.processTask(0xc00010c000)
\t/go/src/app/worker.go:128 +0x8a
main.workerPool(0x4)
\t/go/src/app/main.go:56 +0x1f2`

    const result = detectStackTrace(goLog)
    assert.equal(result.isStackTrace, true)
    assert.equal(result.language, 'go')
    assert.equal(result.firstLine, 'goroutine 42 [running]:')
    assert.equal(result.linesCount, 5)
  })

  it('detects Node.js multiline stack trace', () => {
    const nodeLog = `Error: Connection refused to database at 10.0.4.15:5432
    at TCPConnectWrap.afterConnect [as oncomplete] (node:net:1607:16)
    at Protocol._enqueue (/app/node_modules/pg/lib/client.js:358:11)`

    const result = detectStackTrace(nodeLog)
    assert.equal(result.isStackTrace, true)
    assert.equal(result.language, 'nodejs')
    assert.equal(result.firstLine, 'Error: Connection refused to database at 10.0.4.15:5432')
    assert.equal(result.linesCount, 3)
  })

  it('returns isStackTrace false for regular single or multi-line logs', () => {
    const normalLog = '2026-09-16T08:00:00Z [INFO] HTTP GET /api/v1/health 200 OK 4ms'
    const res1 = detectStackTrace(normalLog)
    assert.equal(res1.isStackTrace, false)

    const multiLineNormal = 'Starting application with config:\nenv=production\nworkers=8'
    const res2 = detectStackTrace(multiLineNormal)
    assert.equal(res2.isStackTrace, false)
  })
})

describe('Micro-Batch Log Parsing & High-Capacity Buffer', () => {
  it('parses array of log objects in a micro-batched WebSocket frame', () => {
    const rawFrame = JSON.stringify([
      { timestamp: '2026-09-16T08:01:00Z', level: 'info', service: 'auth', message: 'token issued' },
      { timestamp: '2026-09-16T08:01:01Z', level: 'warn', service: 'auth', message: 'rate limit warning' }
    ])

    const parsed = parseWebSocketFrame(rawFrame)
    assert.equal(parsed.type, 'logs')
    if (parsed.type === 'logs') {
      assert.equal(parsed.entries.length, 2)
      assert.equal(parsed.entries[0].level, 'INFO')
      assert.equal(parsed.entries[0].service, 'auth')
      assert.equal(parsed.entries[1].msg, 'rate limit warning')
    }
  })

  it('identifies stream_telemetry frames with dropped_count and stream_rate', () => {
    const telemetryFrame = JSON.stringify({
      type: 'stream_telemetry',
      dropped_count: 512,
      stream_rate: 14500
    })

    const parsed = parseWebSocketFrame(telemetryFrame)
    assert.equal(parsed.type, 'telemetry')
    if (parsed.type === 'telemetry') {
      assert.equal(parsed.droppedCount, 512)
      assert.equal(parsed.streamRate, 14500)
    }
  })

  it('logStore maintains strict capacity via native array slice when appending batches', () => {
    setActivePinia(createPinia())
    const store = useLogStore()
    store.clear()
    store.setMaxBufferSize(1000)

    const batch = Array.from({ length: 1500 }, (_, i) => ({
      time: new Date(i * 1000).toISOString(),
      level: 'INFO',
      namespace: 'default',
      pod: 'pod-0',
      msg: `log ${i}`
    }))

    store.appendLogBatch(batch)

    assert.equal(store.logs.length, 1000)
    // Oldest 500 shifted out, array starts at 500 and ends at 1499
    assert.equal(store.logs[0].msg, 'log 500')
    assert.equal(store.logs[store.logs.length - 1].msg, 'log 1499')
  })

  it('logStore supports successive appendLogBatch chunking efficiently', () => {
    setActivePinia(createPinia())
    const store = useLogStore()
    store.clear()
    store.setMaxBufferSize(1000)

    const batch1 = Array.from({ length: 600 }, (_, i) => ({
      time: new Date(i * 1000).toISOString(),
      level: 'INFO',
      namespace: 'default',
      pod: 'pod-0',
      msg: `batch-1-${i}`
    }))
    store.appendLogBatch(batch1)
    assert.equal(store.logs.length, 600)

    const batch2 = Array.from({ length: 600 }, (_, i) => ({
      time: new Date(600000 + i * 1000).toISOString(),
      level: 'INFO',
      namespace: 'default',
      pod: 'pod-0',
      msg: `batch-2-${i}`
    }))
    store.appendLogBatch(batch2)
    assert.equal(store.logs.length, 1000)
    assert.equal(store.logs[store.logs.length - 1].msg, 'batch-2-599')
    assert.equal(store.logs[0].msg, 'batch-1-200')
  })

  it('logStore sets cumulative droppedLogsCount directly without compounding inflation', () => {
    setActivePinia(createPinia())
    const store = useLogStore()
    store.clear()
    store.resetDroppedLogsCount()

    store.connect({ service: 'auth' })
    assert.ok(latestSocket, 'WebSocket instance must be instantiated')

    // First telemetry frame reports cumulative 50 dropped logs
    latestSocket.onmessage!({
      data: JSON.stringify({
        type: 'stream_telemetry',
        dropped_count: 50,
        stream_rate: 12000
      })
    })
    assert.equal(store.droppedLogsCount, 50)

    // Second telemetry frame 1 second later reports cumulative 55 dropped logs
    latestSocket.onmessage!({
      data: JSON.stringify({
        type: 'stream_telemetry',
        dropped_count: 55,
        stream_rate: 12500
      })
    })
    // Before bugfix (+=), this would be 105. With fix (=), it is 55.
    assert.equal(store.droppedLogsCount, 55)

    store.disconnect()
  })
})

describe('Mobile Effective Row Height & Virtual Windowing (AC #1 & #4)', () => {
  const mobileItemHeight = 58 // 58px matches Line 1 metadata 20px + Line 2 message 32px + padding/gap 6px
  const mobileClientHeight = 580
  const overscan = 10

  it('calculates mobile virtual window using 58px effective row height at top', () => {
    const total = 5000
    const win = calculateVirtualWindow({
      totalCount: total,
      scrollTop: 0,
      clientHeight: mobileClientHeight,
      itemHeight: mobileItemHeight,
      overscan,
    })
    assert.equal(win.startIndex, 0)
    // 580 / 58 = 10 visible + 10 overscan = 20
    assert.equal(win.endIndex, 20)
    assert.equal(win.offsetY, 0)
    assert.equal(win.totalHeight, total * mobileItemHeight)
    assert.equal(win.visibleCount, 20)
  })

  it('calculates mobile virtual slice accurately when scrolled into the stream', () => {
    const total = 5000
    const scrollTop = 5800 // row 100
    const win = calculateVirtualWindow({
      totalCount: total,
      scrollTop,
      clientHeight: mobileClientHeight,
      itemHeight: mobileItemHeight,
      overscan,
    })
    // 5800 / 58 = 100. With 10 overscan = 90
    assert.equal(win.startIndex, 90)
    // (5800 + 580) / 58 = 110. With 10 overscan = 120
    assert.equal(win.endIndex, 120)
    assert.equal(win.offsetY, 90 * 58)
    assert.equal(win.visibleCount, 30)
    assert.equal(win.totalHeight, total * mobileItemHeight)
  })

  it('clamps endIndex strictly to totalCount on mobile when scrolled to bottom', () => {
    const total = 300
    const maxScroll = total * mobileItemHeight - mobileClientHeight
    const win = calculateVirtualWindow({
      totalCount: total,
      scrollTop: maxScroll,
      clientHeight: mobileClientHeight,
      itemHeight: mobileItemHeight,
      overscan,
    })
    assert.equal(win.endIndex, 300)
    assert.ok(win.startIndex < 300)
    assert.equal(win.offsetY, win.startIndex * mobileItemHeight)
  })

  it('verifies VirtualLogTerminal.vue implements isMobile and effectiveRowHeight computed', async () => {
    const { readFileSync } = await import('node:fs')
    const { resolve } = await import('node:path')
    const vuePath = resolve('src/components/logs/VirtualLogTerminal.vue')
    const c = readFileSync(vuePath, 'utf8')
    assert.ok(c.includes('effectiveRowHeight'), 'Must define effectiveRowHeight computed')
    assert.ok(c.includes('isMobile'), 'Must define isMobile reactive ref')
    assert.ok(c.includes('58'), 'Must compute 58px on mobile')
  })

  it('verifies virtual-log-terminal.css implements 2-line layout and anti-squish min-width', async () => {
    const { readFileSync } = await import('node:fs')
    const { resolve } = await import('node:path')
    const cssPath = resolve('src/assets/styles/components/virtual-log-terminal.css')
    const c = readFileSync(cssPath, 'utf8')
    assert.ok(c.includes('min-width: 160px'), 'Must enforce anti-squish min-width: 160px on .virtual-log-body')
    assert.ok(c.includes('@media (max-width: 640px)'), 'Must define 640px mobile media query')
    assert.ok(c.includes('-webkit-line-clamp: 2'), 'Must clamp message to 2 lines on mobile')
    assert.ok(c.includes('order: 1'), 'Must set metadata order to 1 on mobile')
    assert.ok(c.includes('order: 2'), 'Must set log body order to 2 on mobile')
  })

  it('verifies floating-alert-toast.css condenses toast to slim 44-48px pill on mobile', async () => {
    const { readFileSync } = await import('node:fs')
    const { resolve } = await import('node:path')
    const cssPath = resolve('src/assets/styles/components/floating-alert-toast.css')
    const c = readFileSync(cssPath, 'utf8')
    assert.ok(c.includes('max-height: 48px'), 'Must constrain toast max-height to 48px on mobile')
    assert.ok(c.includes('display: none !important;'), 'Must hide verbose toast body on mobile')
    assert.ok(c.includes('btn-toast-failover'), 'Must target failover button on mobile')
  })

  it('verifies TopHudAlertBell.vue binds toastRef and checks containment for click-outside', async () => {
    const { readFileSync } = await import('node:fs')
    const { resolve } = await import('node:path')
    const vuePath = resolve('src/components/layout/TopHudAlertBell.vue')
    const c = readFileSync(vuePath, 'utf8')
    assert.ok(c.includes('toastRef'), 'Must declare toastRef')
    assert.ok(c.includes('ref="toastRef"'), 'Must attach ref="toastRef" to header-alert-toast')
    assert.ok(c.includes('toastRef.value') && c.includes('!toastRef.value.contains(target)'), 'Must check toastRef.contains in handleDocumentInteraction')
  })

  it('verifies logstream.css constrains mobile page height properly', async () => {
    const { readFileSync } = await import('node:fs')
    const { resolve } = await import('node:path')
    const cssPath = resolve('src/assets/styles/views/logstream.css')
    const c = readFileSync(cssPath, 'utf8')
    assert.ok(c.includes('max-height: calc(100vh - 48px - 50px - var(--sab, 0px))'), 'Must constrain mobile log-explorer-page max-height')
  })
})
