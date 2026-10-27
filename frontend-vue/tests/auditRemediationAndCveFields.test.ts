import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'

describe('Task 070 Wave 2C: Audit Remediation Drawer & Vulnerability Fields', () => {
  const drawerPath = path.resolve(import.meta.dirname, '../src/components/compliance/RemediationActionDrawer.vue')
  const auditViewPath = path.resolve(import.meta.dirname, '../src/views/AuditView.vue')
  const findingsTablePath = path.resolve(import.meta.dirname, '../src/components/audit/VulnerabilityFindingsTable.vue')
  const complianceApiPath = path.resolve(import.meta.dirname, '../src/api/compliance.ts')

  describe('1. RemediationActionDrawer component exists and complies with line limits', () => {
    it('exists and stays under 300 lines', () => {
      assert.ok(fs.existsSync(drawerPath), 'RemediationActionDrawer.vue must exist')
      const content = fs.readFileSync(drawerPath, 'utf8')
      const lineCount = content.split('\n').length
      assert.ok(lineCount < 300, `RemediationActionDrawer.vue must be <300 lines (current: ${lineCount})`)
    })

    it('renders summary and copyable commands', () => {
      assert.ok(fs.existsSync(drawerPath), 'RemediationActionDrawer.vue must exist')
      const content = fs.readFileSync(drawerPath, 'utf8')
      assert.ok(content.includes('summary'), 'Drawer must display remediation summary')
      assert.ok(content.includes('commands') || content.includes('cmd'), 'Drawer must display remediation commands')
      assert.ok(content.includes('copy') || content.includes('Copy'), 'Drawer must support copying remediation commands')
    })
  })

  describe('2. compliance.ts api definition', () => {
    it('defines RemediationResponse and updates remediateViolation return type', () => {
      const content = fs.readFileSync(complianceApiPath, 'utf8')
      assert.ok(
        content.includes('RemediationResponse') || content.includes('manual_action_required'),
        'compliance.ts must type remediation response with status, summary, and commands'
      )
    })
  })

  describe('3. AuditView.vue wiring', () => {
    it('integrates RemediationActionDrawer and stays under 350 lines', () => {
      const content = fs.readFileSync(auditViewPath, 'utf8')
      const lineCount = content.split('\n').length
      assert.ok(lineCount <= 350, `AuditView.vue must be <=350 lines (current: ${lineCount})`)
      assert.ok(
        content.includes('RemediationActionDrawer') || content.includes('manual_action_required'),
        'AuditView.vue must handle manual_action_required remediation responses'
      )
    })
  })

  describe('4. VulnerabilityFindingsTable.vue field cascade and design tokens', () => {
    it('uses fallback cascades for cve_id, package_name, and fixed_version', () => {
      const content = fs.readFileSync(findingsTablePath, 'utf8')
      assert.ok(
        content.includes('cve_id') || content.includes('rule_id'),
        'Findings table must check cve_id / rule_id'
      )
      assert.ok(
        content.includes('package_name') || content.includes('resource'),
        'Findings table must check package_name / resource'
      )
      assert.ok(
        content.includes('fixed_version'),
        'Findings table must check fixed_version'
      )
    })

    it('has zero hardcoded hex colors in style block', () => {
      const content = fs.readFileSync(findingsTablePath, 'utf8')
      const styleMatch = content.match(/<style[^>]*>([\s\S]*?)<\/style>/)
      assert.ok(styleMatch, 'Style block must exist')
      const styleBlock = styleMatch[1]
      const hexMatches = styleBlock.match(/#[0-9a-fA-F]{3,8}\b/g)
      assert.equal(hexMatches, null, `Found hardcoded hex in style block: ${hexMatches?.join(', ')}`)
    })
  })
})
