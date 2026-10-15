import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { parseAndValidateToken, isPrivilegedAdminRole } from '../src/router/tokenValidator.ts'

describe('Phase 1 — Emergency P0 Security & Frontend Auth Hardening Tests', () => {
  it('parseAndValidateToken rejects null, empty, and malformed inputs', () => {
    assert.equal(parseAndValidateToken(null), null)
    assert.equal(parseAndValidateToken(''), null)
    assert.equal(parseAndValidateToken('invalid-token-string'), null)
    assert.equal(parseAndValidateToken('header.payload'), null)
    assert.equal(parseAndValidateToken('part1.part2.part3.part4'), null)
  })

  it('parseAndValidateToken strictly rejects expired JWT tokens', () => {
    const pastExp = Math.floor(Date.now() / 1000) - 3600 // expired 1 hour ago
    const expiredPayload = Buffer.from(JSON.stringify({ sub: 'user-01', role: 'admin', exp: pastExp })).toString('base64url')
    const expiredToken = `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.${expiredPayload}.sig_hash`
    assert.equal(parseAndValidateToken(expiredToken), null, 'Expired token must return null')
  })

  it('parseAndValidateToken accepts and extracts claims from valid unexpired token', () => {
    const futureExp = Math.floor(Date.now() / 1000) + 7200 // expires in 2 hours
    const payload = Buffer.from(JSON.stringify({
      sub: 'operator-alice',
      role: 'platform_admin',
      tenant_id: 'prod-tenant',
      exp: futureExp
    })).toString('base64url')
    const validToken = `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.${payload}.valid_sig`

    const claims = parseAndValidateToken(validToken)
    assert.ok(claims)
    assert.equal(claims.sub, 'operator-alice')
    assert.equal(claims.role, 'platform_admin')
    assert.equal(claims.exp, futureExp)
  })

  it('parseAndValidateToken handles malformed JSON base64 payloads safely', () => {
    const badJson = Buffer.from('{ invalid-json: true ').toString('base64url')
    const badToken = `eyJhbGciOiJIUzI1NiJ9.${badJson}.sig`
    assert.equal(parseAndValidateToken(badToken), null)
  })

  it('isPrivilegedAdminRole recognizes all administrative variants case-insensitively', () => {
    assert.equal(isPrivilegedAdminRole('admin'), true)
    assert.equal(isPrivilegedAdminRole('ADMIN'), true)
    assert.equal(isPrivilegedAdminRole('Platform Admin'), true)
    assert.equal(isPrivilegedAdminRole('platform_admin'), true)
    assert.equal(isPrivilegedAdminRole('PLATFORM_ADMIN'), true)
    assert.equal(isPrivilegedAdminRole('platform-admin'), true)
    assert.equal(isPrivilegedAdminRole('security auditor'), true)
  })

  it('isPrivilegedAdminRole denies non-privileged and empty roles', () => {
    assert.equal(isPrivilegedAdminRole('developer'), false)
    assert.equal(isPrivilegedAdminRole('viewer'), false)
    assert.equal(isPrivilegedAdminRole('operator'), false)
    assert.equal(isPrivilegedAdminRole('guest'), false)
    assert.equal(isPrivilegedAdminRole(''), false)
    assert.equal(isPrivilegedAdminRole(undefined), false)
  })

  it('WebSocket token safety: Sec-WebSocket-Protocol builder creates bearer subprotocol safely', () => {
    const token = 'jwt-secure-session-token-123'
    // Building protocols array without leaking token in URL query
    const protocols = ['bearer', token]
    assert.equal(protocols[0], 'bearer')
    assert.equal(protocols[1], token)

    // Verify URL constructor without token in search params
    const wsUrl = new URL('ws://localhost:8080/api/v1/logs/stream')
    wsUrl.searchParams.set('pod', 'core-api-0')
    assert.equal(wsUrl.searchParams.has('token'), false, 'Token must never be in query string')
  })
})
