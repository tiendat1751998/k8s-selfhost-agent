import { describe, it } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const rootDir = path.resolve(__dirname, '..')

describe('SLO View Enterprise Redesign Specifications', () => {
  const sloViewPath = path.join(rootDir, 'src', 'views', 'SLOView.vue')
  const sloCatalogTablePath = path.join(rootDir, 'src', 'components', 'slo', 'SloCatalogTable.vue')
  const useSLOMonitorPath = path.join(rootDir, 'src', 'composables', 'useSLOMonitor.ts')
  const sloTableCssPath = path.join(rootDir, 'src', 'assets', 'styles', 'components', 'slo-table.css')
  const sloKpiCssPath = path.join(rootDir, 'src', 'assets', 'styles', 'components', 'slo-kpi.css')
  const sloCssPath = path.join(rootDir, 'src', 'assets', 'styles', 'views', 'slo.css')

  const deadFiles = [
    path.join(rootDir, 'src', 'components', 'slo', 'SloTable.vue'),
    path.join(rootDir, 'src', 'components', 'slo', 'ErrorBudgetDrawer.vue'),
    path.join(rootDir, 'src', 'components', 'slo', 'CreateEditSloModal.vue'),
  ]

  it('Dead Orphaned Files: all 3 duplicate files are deleted and unreferenced', () => {
    for (const deadFile of deadFiles) {
      assert.strictEqual(fs.existsSync(deadFile), false, `Orphaned file ${path.basename(deadFile)} must not exist on disk`)
    }
  })

  it('SloCatalogTable.vue: eradicates left vertical block and achieves 100% full width', () => {
    const content = fs.readFileSync(sloCatalogTablePath, 'utf-8')
    assert.strictEqual(content.includes('box-header'), false, 'Must eradicate .box-header from table wrapper')
    assert.strictEqual(content.includes('box-title'), false, 'Must eradicate .box-title from table wrapper')
    assert.ok(content.includes('<DataTable'), 'Must render <DataTable> directly inside table box')
  })

  it('SloCatalogTable.vue: provides visual Error Budget gauge and Burn Velocity columns', () => {
    const content = fs.readFileSync(sloCatalogTablePath, 'utf-8')
    assert.ok(content.includes('error_budget'), 'Must define error_budget column')
    assert.ok(content.includes('budget-gauge-cell'), 'Must render budget-gauge-cell container')
    assert.ok(content.includes('budget-gauge-track'), 'Must render budget-gauge-track progress bar')
    assert.ok(content.includes('budget-gauge-fill'), 'Must render budget-gauge-fill with percentage width')
    assert.ok(content.includes('burn_rate'), 'Must define burn_rate column')
    assert.ok(content.includes('burn-badge'), 'Must render burn-badge multiplier')
  })

  it('SloCatalogTable.vue: fixes SLI query column with copy button and no truncation squishing', () => {
    const content = fs.readFileSync(sloCatalogTablePath, 'utf-8')
    assert.ok(content.includes('btn-copy-query'), 'Must render copy query button')
    assert.ok(content.includes('navigator.clipboard.writeText'), 'Must implement clipboard copy')
    assert.ok(content.includes('query-cell'), 'Must render query-cell')
  })

  it('SloCatalogTable.vue: standardizes row actions to 1 primary button + ActionDropdown', () => {
    const content = fs.readFileSync(sloCatalogTablePath, 'utf-8')
    assert.ok(content.includes('ActionDropdown'), 'Must use ActionDropdown component')
    assert.ok(content.includes('table-actions-row'), 'Must use enterprise table-actions-row')
    assert.strictEqual(content.includes('btn-icon-del'), false, 'Must eradicate raw 3 square icon buttons')
  })

  it('SLOView.vue: wired to useSLOMonitor and renders 4-Card KPI Strip', () => {
    const content = fs.readFileSync(sloViewPath, 'utf-8')
    assert.ok(content.includes("useSLOMonitor()"), 'Must consume useSLOMonitor composable')
    assert.ok(content.includes('slo-kpi-grid'), 'Must render slo-kpi-grid container')
    assert.ok(content.includes('Total Objectives'), 'Must render Card 1: Total Objectives')
    assert.ok(content.includes('Healthy Objectives'), 'Must render Card 2: Healthy Objectives')
    assert.ok(content.includes('Active Burn Alerts'), 'Must render Card 3: Active Burn Alerts')
    assert.ok(content.includes('Avg Burn Velocity'), 'Must render Card 4: Avg Burn Velocity')
  })

  it('SLOView.vue: supports both Table view and Card Grid view with RWD', () => {
    const content = fs.readFileSync(sloViewPath, 'utf-8')
    assert.ok(content.includes("viewMode === 'table'"), 'Must support table view')
    assert.ok(content.includes("viewMode === 'grid'"), 'Must support cards grid view')
    assert.ok(content.includes('SloMobileCards'), 'Must render first-class mobile cards stream')
    assert.ok(content.includes('slo-mobile-command-bar'), 'Must render 44px mobile command bar')
  })

  it('useSLOMonitor.ts: exports activeBurnAlerts and contains zero emojis', () => {
    const content = fs.readFileSync(useSLOMonitorPath, 'utf-8')
    assert.ok(content.includes('activeBurnAlerts'), 'Must export activeBurnAlerts')
    assert.ok(content.includes('avgBurnRateNum'), 'Must export avgBurnRateNum')
    assert.strictEqual(content.includes('🚨'), false, 'Must contain zero emojis')
  })

  it('CSS files: slo-table.css, slo-kpi.css, and slo.css follow enterprise constraints', () => {
    const tableCss = fs.readFileSync(sloTableCssPath, 'utf-8')
    assert.strictEqual(tableCss.includes('max-width: 150px'), false, 'Must not constrain query code to 150px')
    assert.ok(tableCss.includes('budget-gauge-cell'), 'Must style budget-gauge-cell')
    assert.ok(tableCss.includes('burn-badge'), 'Must style burn-badge')

    const kpiCss = fs.readFileSync(sloKpiCssPath, 'utf-8')
    assert.ok(kpiCss.includes('.slo-kpi-grid'), 'Must define slo-kpi-grid')
    assert.ok(kpiCss.includes('.slo-kpi-card'), 'Must define slo-kpi-card')

    const sloCss = fs.readFileSync(sloCssPath, 'utf-8')
    assert.ok(sloCss.includes("slo-kpi.css"), 'Must import slo-kpi.css')
  })

  it('Line counts: all target files are strictly under 500 lines', () => {
    const files = [
      sloViewPath,
      sloCatalogTablePath,
      useSLOMonitorPath,
      sloTableCssPath,
      sloKpiCssPath,
      sloCssPath,
    ]

    for (const filePath of files) {
      const lineCount = fs.readFileSync(filePath, 'utf-8').split('\n').length
      assert.ok(
        lineCount < 500,
        `File ${path.basename(filePath)} (${lineCount} lines) must be strictly < 500 lines`
      )
    }
  })
})
