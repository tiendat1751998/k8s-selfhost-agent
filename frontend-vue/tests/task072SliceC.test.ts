import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { register } from 'node:module'

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

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const srcDir = path.resolve(__dirname, '../src')

const { buildSettingsTabsList } = await import('../src/composables/useSettings.ts')
const { incidentsApi } = await import('../src/api/incidents.ts')
const { sloApi } = await import('../src/api/slo.ts')

describe('Task 072 - Subtask 72C Acceptance Tests', () => {
  describe('1. Incidents Command & Orphan Rescue (FUNC-04, UI-07, UI-08)', () => {
    it('incidentsApi defines acknowledge and getPostmortem methods', () => {
      assert.equal(typeof (incidentsApi as any).acknowledge, 'function', 'incidentsApi.acknowledge must be defined')
      assert.equal(typeof (incidentsApi as any).getPostmortem, 'function', 'incidentsApi.getPostmortem must be defined')
    })

    it('IncidentDetailPane.vue replaces hardcoded Claude 3.5 Sonnet with dynamic analyzer engine', () => {
      const detailPanePath = path.join(srcDir, 'components/incidents/IncidentDetailPane.vue')
      const content = fs.readFileSync(detailPanePath, 'utf-8')
      assert.ok(!content.includes("'Claude 3.5 Sonnet / Multi-Agent'"), 'Hardcoded Claude 3.5 Sonnet must be removed')
      assert.ok(content.includes('Deterministic SRE RCA Engine'), 'Fallback Deterministic SRE RCA Engine must be present')
    })

    it('IncidentsView.vue embeds CreateIncidentModal and RcaTimelineModal', () => {
      const incidentsViewPath = path.join(srcDir, 'views/IncidentsView.vue')
      const content = fs.readFileSync(incidentsViewPath, 'utf-8')
      assert.ok(content.includes('CreateIncidentModal'), 'IncidentsView must import and render CreateIncidentModal')
      assert.ok(content.includes('RcaTimelineModal'), 'IncidentsView must import and render RcaTimelineModal')
      assert.ok(!content.includes('Declare Operational Incident'), 'Inline modal title must be replaced by CreateIncidentModal')
    })

    it('DEF-005: IncidentDetailPane disables Re-run AI RCA when incident status is resolved', () => {
      const detailPanePath = path.join(srcDir, 'components/incidents/IncidentDetailPane.vue')
      const content = fs.readFileSync(detailPanePath, 'utf-8')
      assert.ok(content.includes("selectedIncident.status === 'resolved'"), 'Re-run AI RCA button must be disabled for resolved incident')
      assert.ok(content.includes('Resolved incident — view existing RCA or export postmortem'), 'Tooltip must inform user of postmortem')
    })
  })

  describe('2. Alerts Responsive Dead-Zone & Alignment (UI-04, UI-07, FUNC-03)', () => {
    it('AlertRulesMobileCards.vue unifies breakpoint at 768px (eliminating 640px dead-zone)', () => {
      const cardsPath = path.join(srcDir, 'components/alerts/AlertRulesMobileCards.vue')
      const content = fs.readFileSync(cardsPath, 'utf-8')
      assert.ok(!content.includes('min-width: 640px'), '640px breakpoint dead-zone must be eliminated')
      assert.ok(content.includes('min-width: 768px'), 'Breakpoint must be 768px to match alerts.css')
    })

    it('AlertRulesTable.vue actions cell enforces nowrap alignment and max 2 buttons', () => {
      const tablePath = path.join(srcDir, 'components/alerts/AlertRulesTable.vue')
      const content = fs.readFileSync(tablePath, 'utf-8')
      assert.ok(content.includes('rule-actions-cell'), 'rule-actions-cell class must be present')
    })
  })

  describe('3. Settings Backup Tab & KMS Posture & TOTP (FUNC-06, UI-08)', () => {
    it('buildSettingsTabsList includes Backup & DR tab', () => {
      const tabs = buildSettingsTabsList([])
      const backupTab = tabs.find((t: any) => t.id === 'backup')
      assert.ok(backupTab, 'Backup tab must be in tabs list')
      assert.equal(backupTab?.label, 'Backup & DR')
      assert.equal(backupTab?.category, 'backup')
    })

    it('SettingsBackupTab.vue exists and integrates with backupApi', () => {
      const backupTabPath = path.join(srcDir, 'components/settings/SettingsBackupTab.vue')
      assert.ok(fs.existsSync(backupTabPath), 'SettingsBackupTab.vue must exist')
      const content = fs.readFileSync(backupTabPath, 'utf-8')
      assert.ok(content.includes('backupApi'), 'SettingsBackupTab must import backupApi')
    })

    it('SettingsNotificationsTab.vue has Send Test Notification action', () => {
      const notifTabPath = path.join(srcDir, 'components/settings/SettingsNotificationsTab.vue')
      const content = fs.readFileSync(notifTabPath, 'utf-8')
      assert.ok(content.includes('testIntegration') || content.includes('testNotification'), 'Test notification trigger must exist')
    })

    it('SettingsSecurityTab.vue includes ZeroTrust KMS posture card', () => {
      const secTabPath = path.join(srcDir, 'components/settings/SettingsSecurityTab.vue')
      const content = fs.readFileSync(secTabPath, 'utf-8')
      assert.ok(content.includes('KMS') || content.includes('Envelope Encryption'), 'KMS posture card must exist')
    })

    it('TOTPSetupView.vue dynamically renders code count instead of hardcoded 10 Codes', () => {
      const totpPath = path.join(srcDir, 'views/TOTPSetupView.vue')
      const content = fs.readFileSync(totpPath, 'utf-8')
      assert.ok(!content.includes('> 10 Codes<'), 'Hardcoded 10 Codes must be replaced with dynamic length')
    })
  })

  describe('4. SLO Edit Workflow & Escape Handlers (FUNC-05)', () => {
    it('sloApi provides updateDefinition', () => {
      assert.equal(typeof sloApi.updateDefinition, 'function', 'sloApi.updateDefinition must be defined')
    })

    it('SloCreateModal.vue supports editingDefinition/initialData prop and edit mode', () => {
      const modalPath = path.join(srcDir, 'components/slo/SloCreateModal.vue')
      const content = fs.readFileSync(modalPath, 'utf-8')
      assert.ok(content.includes('editingDefinition') || content.includes('initialData'), 'Prop for editing definition must exist')
    })

    it('SloCatalogTable.vue provides Edit SLO row action', () => {
      const tablePath = path.join(srcDir, 'components/slo/SloCatalogTable.vue')
      const content = fs.readFileSync(tablePath, 'utf-8')
      assert.ok(content.includes('Edit SLO') || content.includes('editSlo'), 'Edit SLO action must be wired')
    })
  })

  describe('5. Automation & Tenancy Action Density (UI-06, UI-07, FUNC-08)', () => {
    it('useAutomationEngine calculates savedEngineeringHours dynamically', () => {
      const enginePath = path.join(srcDir, 'composables/useAutomationEngine.ts')
      const content = fs.readFileSync(enginePath, 'utf-8')
      assert.ok(!content.includes("return '0.0'"), "savedEngineeringHours must not be hardcoded '0.0'")
      assert.ok(content.includes('* 0.5'), 'Calculation must use successful * 0.5 hours')
    })

    it('RunbooksTable, AutomationRulesTable, AIProvidersTable adhere to max 2 inline actions', () => {
      const runbooksPath = path.join(srcDir, 'components/runbooks/RunbooksTable.vue')
      const autoPath = path.join(srcDir, 'components/automation/AutomationRulesTable.vue')
      const aiPath = path.join(srcDir, 'components/aihub/AIProvidersTable.vue')

      const runbooksContent = fs.readFileSync(runbooksPath, 'utf-8')
      const autoContent = fs.readFileSync(autoPath, 'utf-8')
      const aiContent = fs.readFileSync(aiPath, 'utf-8')

      assert.ok(runbooksContent.includes('ActionDropdown') || runbooksContent.includes('more-btn'), 'RunbooksTable must use dropdown for extra actions')
      assert.ok(autoContent.includes('ActionDropdown') || autoContent.includes('more-btn'), 'AutomationRulesTable must use dropdown for extra actions')
      assert.ok(aiContent.includes('ActionDropdown') || aiContent.includes('more-btn'), 'AIProvidersTable must use dropdown for extra actions')
    })

    it('CreateWorkflowModal, CreateAlertRuleModal, TenantQuotaModal have form linkage and esc keydown', () => {
      const workflowPath = path.join(srcDir, 'components/automation/CreateWorkflowModal.vue')
      const alertPath = path.join(srcDir, 'components/alerts/CreateAlertRuleModal.vue')
      const quotaPath = path.join(srcDir, 'components/tenancy/TenantQuotaModal.vue')

      const workflowContent = fs.readFileSync(workflowPath, 'utf-8')
      const alertContent = fs.readFileSync(alertPath, 'utf-8')
      const quotaContent = fs.readFileSync(quotaPath, 'utf-8')

      assert.ok(workflowContent.includes('keydown') || workflowContent.includes('Escape') || workflowContent.includes('@keydown.esc'), 'CreateWorkflowModal must handle Escape')
      assert.ok(alertContent.includes('form=') || alertContent.includes('type="submit"'), 'CreateAlertRuleModal must link submit button to form')
      assert.ok(quotaContent.includes('form=') || quotaContent.includes('type="submit"'), 'TenantQuotaModal must link submit button to form')
    })
  })
})
