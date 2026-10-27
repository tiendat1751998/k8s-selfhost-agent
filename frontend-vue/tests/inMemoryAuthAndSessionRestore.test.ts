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

// Provide minimal localStorage shim and window mock for Node environment
const storage = new Map<string, string>()
const mockLocalStorage = {
  getItem: (key: string) => storage.get(key) ?? null,
  setItem: (key: string, val: string) => { storage.set(key, String(val)) },
  removeItem: (key: string) => { storage.delete(key) },
  clear: () => { storage.clear() },
}

if (typeof globalThis.window === 'undefined') {
  ;(globalThis as any).window = {
    location: { protocol: 'http:', host: 'localhost:3000', pathname: '/', search: '' },
    setInterval: () => 1,
    clearInterval: () => {},
  }
}
;(globalThis as any).localStorage = mockLocalStorage

const { createPinia, setActivePinia } = await import('pinia')
const { api } = await import('../src/api/client.ts')
const { useAuthStore } = await import('../src/stores/authStore.ts')

describe('SEC-MED-02 — In-Memory JWT & Session Restoration Tests', () => {
  beforeEach(() => {
    storage.clear()
    setActivePinia(createPinia())
    api.clearToken()
  })

  it('api.setToken stores token strictly in memory, never in localStorage', () => {
    api.setToken('jwt-access-token-abc')
    assert.equal(api.getToken(), 'jwt-access-token-abc')
    assert.equal(mockLocalStorage.getItem('k8s_token'), null, 'k8s_token must NOT be in localStorage')
    assert.equal(mockLocalStorage.getItem('k8s_auth_token'), null)
  })

  it('api.clearToken clears in-memory token and removes legacy keys from localStorage', () => {
    mockLocalStorage.setItem('k8s_token', 'legacy-leak')
    mockLocalStorage.setItem('k8s_auth_token', 'legacy-leak-2')
    mockLocalStorage.setItem('k8s_user', '{"id":"usr-1"}')

    api.setToken('active-token')
    assert.equal(api.getToken(), 'active-token')

    api.clearToken()
    assert.equal(api.getToken(), null)
    assert.equal(mockLocalStorage.getItem('k8s_token'), null)
    assert.equal(mockLocalStorage.getItem('k8s_auth_token'), null)
    assert.equal(mockLocalStorage.getItem('k8s_user'), null)
  })

  it('useAuthStore initializes with in-memory null token and purges legacy localStorage tokens', () => {
    mockLocalStorage.setItem('k8s_token', 'stale-token')
    mockLocalStorage.setItem('k8s_auth_token', 'stale-token-2')

    const store = useAuthStore()
    assert.equal(store.token, null, 'token must start as null in memory on boot')
    assert.equal(store.isAuthenticated, false)
    assert.equal(mockLocalStorage.getItem('k8s_token'), null, 'stale k8s_token must be purged on boot')
    assert.equal(mockLocalStorage.getItem('k8s_auth_token'), null, 'stale k8s_auth_token must be purged on boot')
  })

  it('setAuth stores token only in memory and configures api client', () => {
    const store = useAuthStore()
    const userInfo = { id: 'usr-42', email: 'alice@example.com', role: 'admin', tenant_id: 'tenant-prod' }
    store.setAuth('new-access-token-xyz', userInfo)

    assert.equal(store.token, 'new-access-token-xyz')
    assert.equal(store.isAuthenticated, true)
    assert.equal(store.user?.id, 'usr-42')
    assert.equal(api.getToken(), 'new-access-token-xyz')
    assert.equal(mockLocalStorage.getItem('k8s_token'), null, 'k8s_token must NEVER be written to localStorage')
    assert.equal(mockLocalStorage.getItem('k8s_auth_token'), null)
  })

  it('logout purges in-memory token and cleans legacy localStorage keys', async () => {
    const store = useAuthStore()
    store.setAuth('token-to-revoke', { id: 'usr-1', role: 'viewer', tenant_id: 'default' })
    mockLocalStorage.setItem('k8s_token', 'residual')

    await store.logout()

    assert.equal(store.token, null)
    assert.equal(store.user, null)
    assert.equal(store.isAuthenticated, false)
    assert.equal(api.getToken(), null)
    assert.equal(mockLocalStorage.getItem('k8s_token'), null)
    assert.equal(mockLocalStorage.getItem('k8s_auth_token'), null)
    assert.equal(mockLocalStorage.getItem('k8s_user'), null)
  })

  it('restoreSession returns true immediately if token already present in memory', async () => {
    const store = useAuthStore()
    store.token = 'existing-memory-token'
    const success = await store.restoreSession()
    assert.equal(success, true)
  })
})
