import { describe, it } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const rootDir = path.resolve(__dirname, '..')

describe('Login View Clean & Authentic Specifications', () => {
  const loginViewPath = path.join(rootDir, 'src', 'views', 'LoginView.vue')
  const loginFormCardPath = path.join(rootDir, 'src', 'components', 'auth', 'LoginFormCard.vue')
  const loginHeroPath = path.join(rootDir, 'src', 'components', 'auth', 'LoginBrandingHero.vue')
  const useLoginAuthPath = path.join(rootDir, 'src', 'composables', 'useLoginAuth.ts')
  const loginCssPath = path.join(rootDir, 'src', 'assets', 'styles', 'views', 'login.css')

  it('LoginFormCard: eliminates fake SSO grid and raw text icons', () => {
    const content = fs.readFileSync(loginFormCardPath, 'utf-8')
    assert.strictEqual(content.includes('sso-divider'), false, 'Must not contain sso-divider')
    assert.strictEqual(content.includes('sso-providers-grid'), false, 'Must not contain sso-providers-grid')
    assert.strictEqual(content.includes('ssoProviders'), false, 'Must not define or use ssoProviders')
    assert.strictEqual(content.includes('ssoLogin'), false, 'Must not emit ssoLogin')
    assert.strictEqual(content.includes('provider.icon'), false, 'Must not render raw text provider.icon')
  })

  it('useLoginAuth: eliminates dead SSO handler', () => {
    const content = fs.readFileSync(useLoginAuthPath, 'utf-8')
    assert.strictEqual(content.includes('handleSso'), false, 'Must not contain handleSso or handleSsoLogin')
    assert.strictEqual(content.includes('/auth/sso/'), false, 'Must not contain dead /auth/sso/ endpoint')
  })

  it('LoginView: eliminates marketing fluff and adds authentic enterprise footer & status pill', () => {
    const content = fs.readFileSync(loginViewPath, 'utf-8')
    assert.strictEqual(content.includes('FIPS 140-3 Enforced'), false, 'Must not contain FIPS 140-3 Enforced')
    assert.strictEqual(content.includes('ZeroTrust MFA Gate'), false, 'Must not contain ZeroTrust MFA Gate')
    assert.strictEqual(content.includes('SOC2 Audit Stream'), false, 'Must not contain SOC2 Audit Stream')
    assert.strictEqual(content.includes('Dual-Sync DR • Trivy Gate • Real-Time Stream'), false, 'Must not contain fake footer stream')
    assert.strictEqual(content.includes('handleSsoLogin'), false, 'Must not bind handleSsoLogin')

    // Authentic replacements
    assert.ok(
      content.includes('© 2026 K8sControl • Enterprise Hybrid Control Plane'),
      'Must contain authentic copyright footer'
    )
    assert.ok(
      content.includes('System Online • TLS v1.3'),
      'Must contain authentic System Online • TLS v1.3 status pill'
    )
  })

  it('LoginBrandingHero: replaces military buzzwords with authentic architecture features', () => {
    const content = fs.readFileSync(loginHeroPath, 'utf-8')
    assert.strictEqual(content.includes('Air-Gapped ZeroTrust'), false, 'Must not contain Air-Gapped ZeroTrust')
    assert.strictEqual(content.includes('Dual-Sync DR Gateway'), false, 'Must not contain Dual-Sync DR Gateway')
    assert.strictEqual(content.includes('Trivy Supply Chain Gate'), false, 'Must not contain Trivy Supply Chain Gate')
    assert.strictEqual(content.includes('ONLINE (DR-READY)'), false, 'Must not contain ONLINE (DR-READY)')

    // Authentic features
    assert.ok(content.includes('Multi-Cluster Mesh'), 'Must contain Multi-Cluster Mesh')
    assert.ok(content.includes('Centralized orchestration & distributed edge node telemetry.'), 'Must contain Multi-Cluster Mesh desc')
    assert.ok(content.includes('Unified Observability'), 'Must contain Unified Observability')
    assert.ok(content.includes('High-throughput distributed logs & automated RCA diagnostics.'), 'Must contain Unified Observability desc')
    assert.ok(content.includes('Enterprise Governance'), 'Must contain Enterprise Governance')
    assert.ok(content.includes('Strict multi-tenancy isolation & RBAC security controls.'), 'Must contain Enterprise Governance desc')

    // Clean status nodes
    assert.ok(content.includes('SYNCHRONIZED'), 'Must contain SYNCHRONIZED status')
    assert.ok(content.includes('ONLINE'), 'Must contain ONLINE status')
  })

  it('login.css: fixes vertical overflow and aligns containers centered', () => {
    const content = fs.readFileSync(loginCssPath, 'utf-8')
    assert.ok(content.includes('align-items: center'), 'login-container or login-page must align center')
    assert.ok(content.includes('justify-content: center'), 'login-container or login-page must justify center')
    assert.ok(content.includes('padding: 32px 32px'), 'cards must have 32px 32px padding')
  })

  it('Line counts: all target files are strictly under 500 lines', () => {
    const files = [loginViewPath, loginFormCardPath, loginHeroPath, useLoginAuthPath, loginCssPath]
    for (const file of files) {
      const lineCount = fs.readFileSync(file, 'utf-8').split('\n').length
      assert.ok(lineCount < 500, `${path.basename(file)} must be under 500 lines (was ${lineCount})`)
    }
  })
})
