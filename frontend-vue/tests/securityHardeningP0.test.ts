import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const rootDir = path.resolve(__dirname, '..')

describe('Phase 1 — Emergency P0 Security & Frontend Auth Hardening Specifications', () => {
  const auditViewPath = path.join(rootDir, 'src', 'views', 'AuditView.vue')
  const tenancyRbacPath = path.join(rootDir, 'src', 'composables', 'useTenancyRbac.ts')
  const logStorePath = path.join(rootDir, 'src', 'stores', 'logStore.ts')
  const useWebSocketPath = path.join(rootDir, 'src', 'composables', 'useWebSocket.ts')
  const usePodTerminalPath = path.join(rootDir, 'src', 'composables', 'usePodTerminal.ts')
  const routerPath = path.join(rootDir, 'src', 'router', 'index.ts')
  const pluginLoaderPath = path.join(rootDir, 'src', 'services', 'pluginLoader.ts')

  it('1. AuditView: eliminates fake CVE resolution & silent error hiding in catch block', () => {
    const content = fs.readFileSync(auditViewPath, 'utf-8')

    // Find handleResolveFinding implementation
    const fnMatch = content.match(/async\s+function\s+handleResolveFinding[\s\S]*?finally\s*\{/g)
    assert.ok(fnMatch, 'handleResolveFinding function must be present')
    const fnBody = fnMatch[0]

    // Catch block must NOT mutate findings to resolved
    const catchBlockMatch = fnBody.match(/catch\s*(\([^\)]*\))?\s*\{([\s\S]*?)\}/)
    assert.ok(catchBlockMatch, 'catch block must be present')
    const catchBlock = catchBlockMatch[2]

    assert.strictEqual(
      catchBlock.includes("status: 'resolved'"),
      false,
      'Catch block must not mark finding status as resolved on API error'
    )
    assert.strictEqual(
      catchBlock.includes('status: "resolved"'),
      false,
      'Catch block must not mark finding status as resolved on API error'
    )

    // Catch block must set error notification
    assert.ok(
      catchBlock.includes('findingsError') || catchBlock.includes('statusMessage'),
      'Catch block must surface error notification'
    )
  })

  it('2. useTenancyRbac: eliminates fake member/org revocation & optimistic deletions in catch block', () => {
    const content = fs.readFileSync(tenancyRbacPath, 'utf-8')

    // Check handleDeleteOrg
    const deleteOrgMatch = content.match(/async\s+function\s+handleDeleteOrg[\s\S]*?finally\s*\{/g)
    assert.ok(deleteOrgMatch, 'handleDeleteOrg function must be present')
    const deleteOrgBody = deleteOrgMatch[0]
    const deleteOrgCatch = deleteOrgBody.match(/catch\s*(\([^\)]*\))?\s*\{([\s\S]*?)\}/)?.[2] || ''

    assert.strictEqual(
      deleteOrgCatch.includes('organizations.value = organizations.value.filter'),
      false,
      'handleDeleteOrg catch block must not delete organizations locally on failure'
    )
    assert.strictEqual(
      deleteOrgCatch.includes('purged locally'),
      false,
      'handleDeleteOrg catch block must not report purged locally'
    )

    // Check removeMember
    const removeMemberMatch = content.match(/async\s+function\s+removeMember[\s\S]*?finally\s*\{/g)
    assert.ok(removeMemberMatch, 'removeMember function must be present')
    const removeMemberBody = removeMemberMatch[0]
    const removeMemberCatch = removeMemberBody.match(/catch\s*(\([^\)]*\))?\s*\{([\s\S]*?)\}/)?.[2] || ''

    assert.strictEqual(
      removeMemberCatch.includes('members.value = members.value.filter'),
      false,
      'removeMember catch block must not filter out members locally on failure'
    )
    assert.strictEqual(
      removeMemberCatch.includes('revoked locally'),
      false,
      'removeMember catch block must not report revoked locally'
    )

    // Check handleInviteMember
    const inviteMemberMatch = content.match(/async\s+function\s+handleInviteMember[\s\S]*?finally\s*\{/g)
    assert.ok(inviteMemberMatch, 'handleInviteMember function must be present')
    const inviteMemberBody = inviteMemberMatch[0]
    const inviteMemberCatch = inviteMemberBody.match(/catch\s*(\([^\)]*\))?\s*\{([\s\S]*?)\}/)?.[2] || ''

    assert.strictEqual(
      inviteMemberCatch.includes('members.value.push'),
      false,
      'handleInviteMember catch block must not push member locally on failure'
    )
    assert.strictEqual(
      inviteMemberCatch.includes('saved locally'),
      false,
      'handleInviteMember catch block must not report saved locally'
    )

    // Check handleUpdateQuota
    const updateQuotaMatch = content.match(/async\s+function\s+handleUpdateQuota[\s\S]*?finally\s*\{/g)
    assert.ok(updateQuotaMatch, 'handleUpdateQuota function must be present')
    const updateQuotaBody = updateQuotaMatch[0]
    const updateQuotaCatch = updateQuotaBody.match(/catch\s*(\([^\)]*\))?\s*\{([\s\S]*?)\}/)?.[2] || ''

    assert.strictEqual(
      updateQuotaCatch.includes('Quota updated locally'),
      false,
      'handleUpdateQuota catch block must not claim quota updated locally on failure'
    )
  })

  it('3. logStore & WebSocket composables: removes token leak from URL query parameters', () => {
    const logStoreContent = fs.readFileSync(logStorePath, 'utf-8')

    // Must NOT append token to query string in logStore
    assert.strictEqual(
      logStoreContent.includes("params.append('token', token)"),
      false,
      "logStore must not append token to URL query parameters"
    )
    assert.strictEqual(
      logStoreContent.includes('params.append("token", token)'),
      false,
      "logStore must not append token to URL query parameters"
    )

    // Must use WebSocket subprotocol or clean URL
    assert.ok(
      logStoreContent.includes("['bearer', token]") || logStoreContent.includes('["bearer", token]') || logStoreContent.includes("protocols"),
      "logStore must pass token via WebSocket subprotocol Sec-WebSocket-Protocol"
    )

    // Also verify useWebSocket and usePodTerminal don't leak token in query
    const wsComposableContent = fs.readFileSync(useWebSocketPath, 'utf-8')
    assert.strictEqual(
      wsComposableContent.includes("?token="),
      false,
      "useWebSocket must not format token into ?token= query parameter"
    )

    const podTermContent = fs.readFileSync(usePodTerminalPath, 'utf-8')
    assert.strictEqual(
      podTermContent.includes("&token="),
      false,
      "usePodTerminal must not append token into &token= query parameter"
    )
  })

  it('4. router/index.ts: navigation guard validates token structure and verifies active admin session', () => {
    const routerContent = fs.readFileSync(routerPath, 'utf-8')
    const validatorContent = fs.readFileSync(path.join(rootDir, 'src', 'router', 'tokenValidator.ts'), 'utf-8')

    // Guard must not blindly trust `isAuthenticated = !!token` without validation
    assert.strictEqual(
      routerContent.includes('const isAuthenticated = !!token'),
      false,
      'Navigation guard must validate token structure, not blindly check !!token'
    )

    // Guard uses token validation with inspection of parts and expiration
    assert.ok(
      routerContent.includes('parseAndValidateToken'),
      'Navigation guard must call parseAndValidateToken'
    )
    assert.ok(
      validatorContent.includes('split') && validatorContent.includes('exp'),
      'Token validator must inspect token parts and expiration'
    )

    // Sensitive admin routes must require admin role / active session verification
    assert.ok(
      routerContent.includes('requiresAdmin') || routerContent.includes('isPrivilegedAdminRole'),
      'Navigation guard must check admin authorization for sensitive routes'
    )
  })

  it('4b. parseAndValidateToken: correctly rejects invalid/expired tokens and accepts valid JWT', async () => {
    const { parseAndValidateToken, isPrivilegedAdminRole } = await import('../src/router/tokenValidator.ts')

    // Null or empty
    assert.strictEqual(parseAndValidateToken(null), null)
    assert.strictEqual(parseAndValidateToken(''), null)
    assert.strictEqual(parseAndValidateToken('invalid-non-jwt-string'), null)
    assert.strictEqual(parseAndValidateToken('part1.part2'), null)

    // Expired token
    const pastExp = Math.floor(Date.now() / 1000) - 3600
    const expiredPayload = Buffer.from(JSON.stringify({ sub: 'user-1', role: 'admin', exp: pastExp })).toString('base64url')
    const expiredToken = `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.${expiredPayload}.dummy_sig`
    assert.strictEqual(parseAndValidateToken(expiredToken), null, 'Expired token must return null')

    // Valid unexpired token
    const futureExp = Math.floor(Date.now() / 1000) + 3600
    const validPayload = Buffer.from(JSON.stringify({ sub: 'user-1', role: 'admin', exp: futureExp })).toString('base64url')
    const validToken = `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.${validPayload}.dummy_sig`
    const claims = parseAndValidateToken(validToken)
    assert.ok(claims, 'Valid token must parse claims')
    assert.strictEqual(claims?.sub, 'user-1')
    assert.strictEqual(claims?.role, 'admin')

    // Role checks
    assert.strictEqual(isPrivilegedAdminRole('admin'), true)
    assert.strictEqual(isPrivilegedAdminRole('Platform Admin'), true)
    assert.strictEqual(isPrivilegedAdminRole('platform_admin'), true)
    assert.strictEqual(isPrivilegedAdminRole('developer'), false)
    assert.strictEqual(isPrivilegedAdminRole('viewer'), false)
    assert.strictEqual(isPrivilegedAdminRole(undefined), false)
  })

  it('5. pluginLoader: replaces DOM script tag injection with sandboxed iframe and strict CSP', () => {
    const content = fs.readFileSync(pluginLoaderPath, 'utf-8')

    // Must not inject script tag into document.head
    assert.strictEqual(
      content.includes("document.head.appendChild(script)"),
      false,
      "pluginLoader must not inject script tags into document.head"
    )

    // Must use sandboxed iframe
    assert.ok(
      content.includes('iframe') && content.includes('sandbox'),
      'pluginLoader must use sandboxed iframe'
    )

    // Must strictly avoid allow-same-origin to prevent token theft
    assert.strictEqual(
      content.includes('allow-same-origin'),
      false,
      'pluginLoader iframe sandbox must never include allow-same-origin'
    )

    // Must include CSP
    assert.ok(
      content.includes('Content-Security-Policy'),
      'pluginLoader iframe must include Content-Security-Policy header or meta tag'
    )
  })

  it('6. Line count constraints: all modified files strictly stay < 500 lines', () => {
    const tokenValidatorPath = path.join(rootDir, 'src', 'router', 'tokenValidator.ts')
    const filesToAudit = [
      auditViewPath,
      tenancyRbacPath,
      logStorePath,
      useWebSocketPath,
      usePodTerminalPath,
      routerPath,
      tokenValidatorPath,
      pluginLoaderPath,
      __filename
    ]

    for (const file of filesToAudit) {
      const lineCount = fs.readFileSync(file, 'utf-8').split('\n').length
      assert.ok(
        lineCount < 500,
        `File ${path.basename(file)} (${lineCount} lines) must be strictly < 500 lines`
      )
    }
  })
})
