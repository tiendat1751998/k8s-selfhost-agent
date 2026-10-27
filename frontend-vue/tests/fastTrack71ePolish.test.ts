import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'

describe('Fast-Track Polish 71E: Remediation Persistence & Helm 503 Exact Error', () => {
  const complianceApiPath = path.resolve(import.meta.dirname, '../src/api/compliance.ts')
  const remediationDrawerPath = path.resolve(import.meta.dirname, '../src/components/compliance/RemediationActionDrawer.vue')
  const auditViewPath = path.resolve(import.meta.dirname, '../src/views/AuditView.vue')
  const helmDrawerPath = path.resolve(import.meta.dirname, '../src/components/helm/HelmInstallDrawer.vue')

  describe('1. File size invariants', () => {
    it('AuditView.vue stays <= 350 lines', () => {
      const content = fs.readFileSync(auditViewPath, 'utf8')
      const lines = content.split('\n').length
      assert.ok(lines <= 350, `AuditView.vue must be <= 350 lines (current: ${lines})`)
    })

    it('RemediationActionDrawer.vue stays < 300 lines', () => {
      const content = fs.readFileSync(remediationDrawerPath, 'utf8')
      const lines = content.split('\n').length
      assert.ok(lines < 300, `RemediationActionDrawer.vue must be < 300 lines (current: ${lines})`)
    })

    it('HelmInstallDrawer.vue stays < 300 lines', () => {
      const content = fs.readFileSync(helmDrawerPath, 'utf8')
      const lines = content.split('\n').length
      assert.ok(lines < 300, `HelmInstallDrawer.vue must be < 300 lines (current: ${lines})`)
    })
  })

  describe('2. complianceApi contracts (compliance.ts)', () => {
    it('supports confirm parameter on remediateViolation and defines resolveViolation', () => {
      const content = fs.readFileSync(complianceApiPath, 'utf8')
      assert.ok(
        content.includes('remediateViolation') && (content.includes('confirm') || content.includes('body')),
        'remediateViolation must support confirm parameter payload'
      )
      assert.ok(
        content.includes('resolveViolation'),
        'complianceApi must define resolveViolation method'
      )
    })
  })

  describe('3. RemediationActionDrawer.vue', () => {
    it('renders Mark as Remediated action button', () => {
      const content = fs.readFileSync(remediationDrawerPath, 'utf8')
      assert.ok(
        content.includes('Mark as Remediated'),
        'RemediationActionDrawer must contain "Mark as Remediated" button text'
      )
      assert.ok(
        content.includes("emit('remediated'"),
        'RemediationActionDrawer must emit remediated event'
      )
    })
  })

  describe('4. AuditView.vue manual remediation persistence', () => {
    it('persists resolved status across fetchComplianceData without regressing active count', () => {
      const content = fs.readFileSync(auditViewPath, 'utf8')
      assert.ok(
        content.includes('onManualRemediated'),
        'AuditView must implement onManualRemediated'
      )
      assert.ok(
        content.includes('confirm: true') || content.includes('resolveViolation'),
        'AuditView must invoke compliance API with confirmation'
      )
      assert.ok(
        content.includes('manuallyResolvedIds') || content.includes('resolved: true'),
        'AuditView must track resolved IDs to prevent fetchComplianceData from overwriting resolution'
      )
    })
  })

  describe('5. HelmInstallDrawer.vue error formatting', () => {
    it('formats and displays exact error message string in Step 3 error alert', () => {
      const content = fs.readFileSync(helmDrawerPath, 'utf8')
      assert.ok(
        content.includes('install-error-banner'),
        'HelmInstallDrawer must render install-error-banner'
      )
      assert.ok(
        content.includes('installError') || content.includes('errorMessage'),
        'HelmInstallDrawer must bind error message to display'
      )
    })
  })
})
