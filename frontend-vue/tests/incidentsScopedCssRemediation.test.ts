import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const rootDir = path.resolve(__dirname, '..')

describe('Incidents UI Scoped CSS Remediation & Responsive Card Layout Specifications', () => {
  const incidentsCssPath = path.join(rootDir, 'src', 'assets', 'styles', 'views', 'incidents.css')
  const incidentsViewPath = path.join(rootDir, 'src', 'views', 'IncidentsView.vue')
  const incidentsComponentsDir = path.join(rootDir, 'src', 'components', 'incidents')

  it('1. Extracted incident components have <style scoped> importing incidents.css', () => {
    const requiredComponents = [
      'IncidentsHeaderBar.vue',
      'IncidentCreatePrModal.vue',
      'IncidentSimulationModal.vue',
      'CreateIncidentModal.vue',
      'IncidentTriageDrawer.vue',
      'IncidentsMobileCards.vue',
      'IncidentsTable.vue',
      'RcaTimelineModal.vue'
    ]

    for (const comp of requiredComponents) {
      const compPath = path.join(incidentsComponentsDir, comp)
      assert.ok(fs.existsSync(compPath), `${comp} must exist`)
      const content = fs.readFileSync(compPath, 'utf-8')
      assert.ok(content.includes('<style scoped>'), `${comp} must have <style scoped> block`)
      assert.ok(
        content.includes("assets/styles/views/incidents.css"),
        `${comp} must import incidents.css`
      )
    }
  })

  it('2. incidents.css enforces responsive desktop-only and mobile-only hiding', () => {
    const cssContent = fs.readFileSync(incidentsCssPath, 'utf-8')
    assert.ok(
      cssContent.includes('.desktop-only { display: none !important; }'),
      'incidents.css must hide .desktop-only on mobile'
    )
    assert.ok(
      cssContent.includes('.incidents-toolbar-sleek.desktop-only { display: none !important; }'),
      'incidents.css must explicitly hide .incidents-toolbar-sleek.desktop-only on mobile'
    )
    assert.ok(
      cssContent.includes('.mobile-only { display: none !important; }'),
      'incidents.css must hide .mobile-only on desktop'
    )
    assert.ok(
      cssContent.includes('.mobile-command-bar.mobile-only { display: none !important; }'),
      'incidents.css must explicitly hide .mobile-command-bar.mobile-only on desktop'
    )
  })

  it('3. IncidentsView.vue layout prevents premature pod name truncation on mobile', () => {
    const viewContent = fs.readFileSync(incidentsViewPath, 'utf-8')
    const cssContent = fs.readFileSync(incidentsCssPath, 'utf-8')

    // card-item-top dedicates full left space to item-pod
    assert.ok(
      viewContent.includes('<span class="item-pod" :title="inc.pod_name">{{ inc.pod_name }}</span>'),
      'IncidentsView.vue must render item-pod directly inside card-item-top'
    )

    // namespace is moved to card-bot-left in card-item-bot
    assert.ok(
      viewContent.includes('<div class="card-bot-left">'),
      'IncidentsView.vue must group namespace in card-bot-left'
    )
    assert.ok(
      viewContent.includes('<span class="type-badge font-mono">{{ inc.type }}</span>'),
      'card-bot-left must contain type-badge'
    )
    assert.ok(
      viewContent.includes('<span class="item-ns font-mono">{{ inc.namespace }}</span>'),
      'card-bot-left must contain item-ns namespace badge'
    )

    // CSS defines card-bot-left and flex properties for item-pod
    assert.ok(cssContent.includes('.card-bot-left'), 'incidents.css must define .card-bot-left')
    assert.ok(cssContent.includes('.item-pod { font-size: 13px; font-weight: 700; color: #fff; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; min-width: 0; flex: 1; }'),
      'incidents.css must configure .item-pod with min-width: 0; flex: 1;'
    )
  })

  it('4. Audited extracted child components have their required scoped stylesheets', () => {
    const audits: { file: string; expectedImports: string[] }[] = [
      {
        file: path.join(rootDir, 'src', 'components', 'helm', 'HelmCatalogToolbar.vue'),
        expectedImports: ['assets/styles/views/helm.css']
      },
      {
        file: path.join(rootDir, 'src', 'components', 'explorer', 'ExplorerToolbar.vue'),
        expectedImports: ['assets/styles/views/explorer.css', 'assets/styles/components/explorer-toolbar.css']
      },
      {
        file: path.join(rootDir, 'src', 'components', 'explorer', 'ExplorerDetailDrawer.vue'),
        expectedImports: ['assets/styles/views/explorer.css', 'assets/styles/components/explorer-details.css']
      },
      {
        file: path.join(rootDir, 'src', 'components', 'explorer', 'ExplorerDeleteModal.vue'),
        expectedImports: ['assets/styles/views/explorer.css']
      },
      {
        file: path.join(rootDir, 'src', 'components', 'explorer', 'ExplorerDrainModal.vue'),
        expectedImports: ['assets/styles/views/explorer.css']
      },
      {
        file: path.join(rootDir, 'src', 'components', 'explorer', 'ExplorerImportModal.vue'),
        expectedImports: ['assets/styles/views/explorer.css']
      },
      {
        file: path.join(rootDir, 'src', 'components', 'hosts', 'InfraHostsHeaderBar.vue'),
        expectedImports: ['assets/styles/views/infra-hosts.css']
      },
      {
        file: path.join(rootDir, 'src', 'components', 'hosts', 'HostControlsBar.vue'),
        expectedImports: ['assets/styles/views/infra-hosts.css']
      },
      {
        file: path.join(rootDir, 'src', 'components', 'hosts', 'HostDetailDrawer.vue'),
        expectedImports: ['assets/styles/views/infra-hosts.css']
      },
      {
        file: path.join(rootDir, 'src', 'components', 'hosts', 'HostAddEditModal.vue'),
        expectedImports: ['assets/styles/views/infra-hosts.css']
      },
      {
        file: path.join(rootDir, 'src', 'components', 'hosts', 'HostDeleteModal.vue'),
        expectedImports: ['assets/styles/views/infra-hosts.css']
      },
      {
        file: path.join(rootDir, 'src', 'components', 'hosts', 'HostFleetTelemetryPanel.vue'),
        expectedImports: ['assets/styles/views/infra-hosts.css']
      },
      {
        file: path.join(rootDir, 'src', 'components', 'hosts', 'HostsGrid.vue'),
        expectedImports: ['assets/styles/views/infra-hosts.css']
      },
      {
        file: path.join(rootDir, 'src', 'components', 'capacity', 'CapacityInspectionDrawer.vue'),
        expectedImports: ['assets/styles/components/capacity-drawers.css']
      },
      {
        file: path.join(rootDir, 'src', 'components', 'capacity', 'AddCapacityPolicyModal.vue'),
        expectedImports: ['assets/styles/components/capacity-drawers.css']
      }
    ]

    for (const audit of audits) {
      assert.ok(fs.existsSync(audit.file), `${path.basename(audit.file)} must exist`)
      const content = fs.readFileSync(audit.file, 'utf-8')
      assert.ok(content.includes('<style scoped>'), `${path.basename(audit.file)} must have <style scoped>`)
      for (const exp of audit.expectedImports) {
        assert.ok(content.includes(exp), `${path.basename(audit.file)} must import ${exp}`)
      }
    }
  })

  it('5. Every modified file stays strictly under 500 lines', () => {
    const filesToCheck = [
      incidentsCssPath,
      incidentsViewPath,
      path.join(incidentsComponentsDir, 'IncidentsHeaderBar.vue'),
      path.join(incidentsComponentsDir, 'IncidentCreatePrModal.vue'),
      path.join(incidentsComponentsDir, 'IncidentSimulationModal.vue'),
      path.join(incidentsComponentsDir, 'CreateIncidentModal.vue'),
      path.join(incidentsComponentsDir, 'IncidentTriageDrawer.vue'),
      path.join(incidentsComponentsDir, 'IncidentsMobileCards.vue'),
      path.join(incidentsComponentsDir, 'IncidentsTable.vue'),
      path.join(incidentsComponentsDir, 'RcaTimelineModal.vue'),
      path.join(rootDir, 'src', 'components', 'helm', 'HelmCatalogToolbar.vue'),
      path.join(rootDir, 'src', 'components', 'explorer', 'ExplorerToolbar.vue'),
      path.join(rootDir, 'src', 'components', 'explorer', 'ExplorerDetailDrawer.vue'),
      path.join(rootDir, 'src', 'components', 'explorer', 'ExplorerDeleteModal.vue'),
      path.join(rootDir, 'src', 'components', 'explorer', 'ExplorerDrainModal.vue'),
      path.join(rootDir, 'src', 'components', 'explorer', 'ExplorerImportModal.vue'),
      path.join(rootDir, 'src', 'components', 'hosts', 'InfraHostsHeaderBar.vue'),
      path.join(rootDir, 'src', 'components', 'hosts', 'HostControlsBar.vue'),
      path.join(rootDir, 'src', 'components', 'hosts', 'HostDetailDrawer.vue'),
      path.join(rootDir, 'src', 'components', 'hosts', 'HostAddEditModal.vue'),
      path.join(rootDir, 'src', 'components', 'hosts', 'HostDeleteModal.vue'),
      path.join(rootDir, 'src', 'components', 'hosts', 'HostFleetTelemetryPanel.vue'),
      path.join(rootDir, 'src', 'components', 'hosts', 'HostsGrid.vue'),
      path.join(rootDir, 'src', 'components', 'capacity', 'CapacityInspectionDrawer.vue'),
      path.join(rootDir, 'src', 'components', 'capacity', 'AddCapacityPolicyModal.vue')
    ]

    for (const filePath of filesToCheck) {
      const lineCount = fs.readFileSync(filePath, 'utf-8').split('\n').length
      assert.ok(lineCount < 500, `${path.basename(filePath)} exceeds 500 lines (${lineCount} lines)`)
    }
  })
})
