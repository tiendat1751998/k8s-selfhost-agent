import { describe, it } from 'node:test'
import assert from 'node:assert/strict'

export function sanitizeWebSocketConnection(rawUrl: string, token?: string): {
  cleanUrl: string
  protocols: string[]
} {
  const url = new URL(rawUrl)
  // Delete token from query parameters to prevent leakage in proxy/access logs
  url.searchParams.delete('token')

  const protocols: string[] = []
  if (token && token.trim()) {
    // Pass strictly via Sec-WebSocket-Protocol
    protocols.push('bearer', token.trim())
  }

  return {
    cleanUrl: url.toString(),
    protocols
  }
}

export function calculateErrorColspan(columns: Array<{ key: string }>): number {
  return Array.isArray(columns) && columns.length > 0 ? columns.length : 7
}

export function resolveVirtualFpsText(isMobile: boolean, fps: number = 60): string {
  if (isMobile) {
    return `${Math.round(fps)} FPS`
  }
  return `${Math.round(fps)} FPS Virtualized`
}

describe('MCP Audit Remediation: 3 Critical Deficiencies Domain Verification', () => {
  it('Defect A: sanitizeWebSocketConnection strips token from searchParams and passes via bearer protocol', () => {
    const dirtyUrl = 'ws://localhost:8080/api/v1/logs/stream?token=secret-token-xyz&pod=api-0'
    const result = sanitizeWebSocketConnection(dirtyUrl, 'secret-token-xyz')

    assert.equal(result.cleanUrl.includes('token='), false, 'Token must be deleted from clean URL')
    assert.equal(result.cleanUrl, 'ws://localhost:8080/api/v1/logs/stream?pod=api-0')
    assert.deepEqual(result.protocols, ['bearer', 'secret-token-xyz'])
  })

  it('Defect A: sanitizeWebSocketConnection handles URL without query token', () => {
    const cleanUrl = 'wss://k8s.example.com/api/v1/terminal'
    const result = sanitizeWebSocketConnection(cleanUrl, 'my-auth-token')

    assert.equal(result.cleanUrl, cleanUrl)
    assert.deepEqual(result.protocols, ['bearer', 'my-auth-token'])
  })

  it('Defect B: calculateErrorColspan returns exact column count or safe fallback', () => {
    const tableColumns = [
      { key: 'timestamp' },
      { key: 'severity' },
      { key: 'source' },
      { key: 'user' },
      { key: 'action' },
      { key: 'target' },
      { key: 'actions' }
    ]
    assert.equal(calculateErrorColspan(tableColumns), 7)

    const smallColumns = [{ key: 'id' }, { key: 'name' }, { key: 'status' }]
    assert.equal(calculateErrorColspan(smallColumns), 3)

    assert.equal(calculateErrorColspan([]), 7)
  })

  it('Defect C: resolveVirtualFpsText formats compact badge on mobile and full on desktop', () => {
    assert.equal(resolveVirtualFpsText(true, 60), '60 FPS')
    assert.equal(resolveVirtualFpsText(true, 58.7), '59 FPS')
    assert.equal(resolveVirtualFpsText(false, 60), '60 FPS Virtualized')
    assert.equal(resolveVirtualFpsText(false, 59.4), '59 FPS Virtualized')
  })
})
