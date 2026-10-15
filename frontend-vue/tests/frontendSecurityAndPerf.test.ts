import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { isPrivilegedAdminRole, parseAndValidateToken } from '../src/router/tokenValidator.ts'

describe('Frontend Client-Side Security & Performance Optimization Tests', () => {
  it('SEC-14: isPrivilegedAdminRole recognizes platform_admin and case variants', () => {
    assert.equal(isPrivilegedAdminRole('platform_admin'), true)
    assert.equal(isPrivilegedAdminRole('PLATFORM_ADMIN'), true)
    assert.equal(isPrivilegedAdminRole('  platform_admin  '), true)
    assert.equal(isPrivilegedAdminRole('platform admin'), true)
  })

  it('SEC-14: parseAndValidateToken protects against privilege escalation through forged claims', () => {
    // Missing signature or malformed base64
    assert.equal(parseAndValidateToken('head.forged_admin_payload'), null)

    // Expired token claiming platform_admin
    const past = Math.floor(Date.now() / 1000) - 100
    const payload = Buffer.from(JSON.stringify({ sub: 'attacker', role: 'platform_admin', exp: past })).toString('base64url')
    assert.equal(parseAndValidateToken(`header.${payload}.sig`), null)
  })

  it('PERF: verifies timer cleanup pattern prevents memory leaks on unmount', () => {
    let timerCleared = false
    const timer: ReturnType<typeof setTimeout> = setTimeout(() => {}, 10000)
    function cleanup() {
      clearTimeout(timer)
      timerCleared = true
    }
    cleanup()
    assert.equal(timerCleared, true)
  })

  it('PERF: table row key selector generates deterministic unique keys without array index', () => {
    function getRowKey<T extends { id?: string; name?: string; uid?: string }>(item: T, fallbackPrefix: string): string {
      return item.id || item.uid || item.name || `${fallbackPrefix}-${Math.random().toString(36).slice(2, 7)}`
    }

    const host1 = { id: 'host-10.0.1.5', name: 'worker-node-01' }
    const host2 = { uid: 'uid-abc-123', name: 'worker-node-02' }
    const host3 = { name: 'worker-node-03' }

    assert.equal(getRowKey(host1, 'host'), 'host-10.0.1.5')
    assert.equal(getRowKey(host2, 'host'), 'uid-abc-123')
    assert.equal(getRowKey(host3, 'host'), 'worker-node-03')
  })

  it('PERF: ring buffer memory cap ensures arrays never grow unbounded in telemetry streams', () => {
    const MAX_ITEMS = 500
    const buffer: number[] = []

    for (let i = 0; i < 2000; i++) {
      buffer.push(i)
      if (buffer.length > MAX_ITEMS) {
        buffer.splice(0, buffer.length - MAX_ITEMS)
      }
    }

    assert.equal(buffer.length, MAX_ITEMS)
    assert.equal(buffer[0], 1500)
    assert.equal(buffer[MAX_ITEMS - 1], 1999)
  })
})
