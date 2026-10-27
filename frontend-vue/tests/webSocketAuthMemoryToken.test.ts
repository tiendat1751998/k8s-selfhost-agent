import { describe, it, beforeEach } from 'node:test'
import assert from 'node:assert/strict'
import { register } from 'node:module'

// Custom module resolver to allow extensionless TS imports in node --test
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

const storage = new Map<string, string>()
const mockLocalStorage = {
  getItem: (key: string) => storage.get(key) ?? null,
  setItem: (key: string, val: string) => { storage.set(key, String(val)) },
  removeItem: (key: string) => { storage.delete(key) },
  clear: () => { storage.clear() },
}

let lastWsUrl: string | null = null
let lastWsProtocols: string | string[] | undefined = undefined

class MockWebSocket {
  url: string
  protocols?: string | string[]
  readyState = 0
  onopen: (() => void) | null = null
  onmessage: ((e: any) => void) | null = null
  onerror: (() => void) | null = null
  onclose: (() => void) | null = null

  constructor(url: string, protocols?: string | string[]) {
    this.url = url
    this.protocols = protocols
    lastWsUrl = url
    lastWsProtocols = protocols
  }

  close() {
    this.readyState = 3
  }
}

if (typeof globalThis.window === 'undefined') {
  ;(globalThis as any).window = {
    location: { protocol: 'http:', host: 'localhost:3000', origin: 'http://localhost:3000' },
  }
}
;(globalThis as any).localStorage = mockLocalStorage
;(globalThis as any).WebSocket = MockWebSocket

const { createPinia, setActivePinia } = await import('pinia')
const { api } = await import('../src/api/client.ts')
const { useWebSocket } = await import('../src/composables/useWebSocket.ts')

describe('Wave 1 Gate Fix — WebSocket Auth with In-Memory Bearer Token', () => {
  beforeEach(() => {
    storage.clear()
    setActivePinia(createPinia())
    api.clearToken()
    lastWsUrl = null
    lastWsProtocols = undefined
  })

  it('connects WebSocket using bearer token from api.getToken() when localStorage is empty', () => {
    // Purged localStorage - strictly empty
    assert.equal(mockLocalStorage.getItem('k8s_token'), null)

    // Set token in memory
    api.setToken('test-memory-jwt-token')
    assert.equal(api.getToken(), 'test-memory-jwt-token')

    const wsInstance = useWebSocket({ url: 'ws://localhost:3000/ws' })
    wsInstance.connect()

    assert.ok(lastWsProtocols, 'WebSocket protocols must be provided')
    assert.deepEqual(lastWsProtocols, ['bearer', 'test-memory-jwt-token'], 'WebSocket must connect with bearer protocol containing in-memory token')
    assert.equal(lastWsUrl, 'ws://localhost:3000/ws', 'URL should not have query token leak')

    wsInstance.disconnect()
  })

  it('connects WebSocket without bearer protocol when no token exists anywhere', () => {
    api.clearToken()
    assert.equal(api.getToken(), null)
    assert.equal(mockLocalStorage.getItem('k8s_token'), null)

    const wsInstance = useWebSocket({ url: 'ws://localhost:3000/ws' })
    wsInstance.connect()

    assert.equal(lastWsProtocols, undefined, 'No protocols should be passed when unauthenticated')
    assert.equal(lastWsUrl, 'ws://localhost:3000/ws')

    wsInstance.disconnect()
  })
})
