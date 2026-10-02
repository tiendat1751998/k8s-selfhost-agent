import { test, describe } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'

describe('Table Overflow Remediation Specifications', () => {
  const rootDir = path.resolve(import.meta.dirname, '..')

  test('DeploymentsTable.vue: column widths calibrated with fixed layout & max-width <= 1070px', () => {
    const filePath = path.join(rootDir, 'src/components/deployments/DeploymentsTable.vue')
    const content = fs.readFileSync(filePath, 'utf-8')

    // Verify column calibrations
    assert.match(content, /key:\s*'strategy'[^}]+width:\s*'90px'/, 'Strategy column must be shaved to 90px')
    assert.match(content, /key:\s*'image'[^}]+width:\s*'130px'/, 'Image column must be shaved to 130px')
    assert.match(content, /key:\s*'replicas'[^}]+width:\s*'105px'/, 'Replicas column must be shaved to 105px')
    assert.match(content, /key:\s*'actions'[^}]+width:\s*'115px'/, 'Actions column must be set to 115px')

    // Verify fixed table layout and max-width in scoped CSS
    assert.match(content, /table-layout:\s*fixed/i, 'Must enforce table-layout: fixed')
    assert.match(content, /max-width:\s*1070px/i, 'Must constrain table to max-width: 1070px')
    assert.match(content, /padding:\s*8px\s+10px/i, 'Must set padding to 8px 10px')
  })

  test('deployments.css: table layout fixed, max-width <= 1070px, and padding 8px 10px', () => {
    const filePath = path.join(rootDir, 'src/assets/styles/views/deployments.css')
    const content = fs.readFileSync(filePath, 'utf-8')

    assert.match(content, /max-width:\s*1070px/i, 'deployments.css must contain max-width: 1070px constraint')
    assert.match(content, /\.deployments-table-wrap\s+th,\s*\.deployments-table-wrap\s+td/i, 'deployments.css must scope padding to .deployments-table-wrap')
    assert.match(content, /padding:\s*8px\s+10px/i, 'Must set table padding to 8px 10px')
  })

  test('overview-hosts.css: column widths shaved by 55px (sum <= 975px), max-width <= 1030px', () => {
    const filePath = path.join(rootDir, 'src/assets/styles/components/overview-hosts.css')
    const content = fs.readFileSync(filePath, 'utf-8')

    // Verify column width reductions
    assert.match(content, /\.col-name\s*\{[^}]*width:\s*150px/i, '.col-name must be shaved to 150px (saves 20px)')
    assert.match(content, /\.col-ip\s*\{[^}]*width:\s*115px/i, '.col-ip must be shaved to 115px (saves 15px)')
    assert.match(content, /\.col-actions\s*\{[^}]*width:\s*130px/i, '.col-actions must be shaved to 130px (saves 20px)')

    // Verify table layout and max-width
    assert.match(content, /table-layout:\s*fixed/i, 'overview-hosts.css must enforce table-layout: fixed')
    assert.match(content, /max-width:\s*1030px/i, 'overview-hosts.css must enforce max-width: 1030px')

    // Verify padding reduced to 8px
    assert.match(content, /\.node-table\s+th[^{]*\{[^}]*padding:\s*6px\s+8px/i, 'th padding must be 8px horizontal')
    assert.match(content, /\.node-table\s+td[^{]*\{[^}]*padding:\s*2px\s+8px/i, 'td padding must be 8px horizontal')
  })

  test('NodeTableView.vue: scoped styles enforce table-layout: fixed and max-width 1030px', () => {
    const filePath = path.join(rootDir, 'src/components/overview/nodes/NodeTableView.vue')
    const content = fs.readFileSync(filePath, 'utf-8')

    assert.match(content, /table-layout:\s*fixed/i, 'NodeTableView scoped CSS must enforce table-layout: fixed')
    assert.match(content, /max-width:\s*1030px/i, 'NodeTableView scoped CSS must enforce max-width: 1030px')
  })

  test('SloCatalogTable.vue & slo-table.css: calibrated columns, fixed layout & max-width <= 1080px', () => {
    const tablePath = path.join(rootDir, 'src/components/slo/SloCatalogTable.vue')
    const cssPath = path.join(rootDir, 'src/assets/styles/components/slo-table.css')
    const tableContent = fs.readFileSync(tablePath, 'utf-8')
    const cssContent = fs.readFileSync(cssPath, 'utf-8')

    // Verify column definitions in SloCatalogTable
    assert.match(tableContent, /key:\s*'service'[^}]+width:\s*'140px'/, 'Service column must be 140px')
    assert.match(tableContent, /key:\s*'indicator_type'[^}]+width:\s*'85px'/, 'Indicator type column must be 85px')
    assert.match(tableContent, /key:\s*'targetNum'[^}]+width:\s*'75px'/, 'Target column must be 75px')
    assert.match(tableContent, /key:\s*'error_budget'[^}]+width:\s*'110px'/, 'Error budget column must be 110px')
    assert.match(tableContent, /key:\s*'burn_rate'[^}]+width:\s*'95px'/, 'Burn rate column must be 95px')
    assert.match(tableContent, /key:\s*'window'[^}]+width:\s*'65px'/, 'Window column must be 65px')
    assert.match(tableContent, /key:\s*'query'[^}]+width:\s*'240px'/, 'Query column must be 240px')
    assert.match(tableContent, /key:\s*'alert_threshold'[^}]+width:\s*'95px'/, 'Threshold column must be 95px')
    assert.match(tableContent, /key:\s*'actions'[^}]+width:\s*'115px'/, 'Actions column must be 115px')

    // Verify table-layout fixed and max-width in slo-table.css
    assert.match(cssContent, /table-layout:\s*fixed/i, 'slo-table.css must enforce table-layout: fixed')
    assert.match(cssContent, /max-width:\s*1080px/i, 'slo-table.css must enforce max-width: 1080px')
    assert.match(cssContent, /max-width:\s*240px/i, 'slo-table.css must clamp query-cell to 240px')
    assert.strictEqual(cssContent.includes('.table-box .data-table'), false, 'slo-table.css must purge bare generic selector')

    // Verify Vue 3 scoped :deep penetration for DataTable child component
    assert.match(tableContent, /:deep\([^)]*\.slo-table-container/i, 'SloCatalogTable must use :deep() penetration for child DataTable')
    assert.match(tableContent, /:deep\(\.data-table\)/i, 'SloCatalogTable must include :deep(.data-table)')
    assert.match(tableContent, /table-layout:\s*fixed/i, 'SloCatalogTable must enforce table-layout: fixed inside :deep')
  })

  test('ExplorerResourceTable.vue & explorer.css: fixed layout, max-width <= 1040px, and column shaving', () => {
    const tablePath = path.join(rootDir, 'src/components/explorer/ExplorerResourceTable.vue')
    const columnsPath = path.join(rootDir, 'src/composables/explorerColumns.ts')
    const cssPath = path.join(rootDir, 'src/assets/styles/views/explorer.css')
    const tableContent = fs.readFileSync(tablePath, 'utf-8')
    const columnsContent = fs.readFileSync(columnsPath, 'utf-8')
    const cssContent = fs.readFileSync(cssPath, 'utf-8')

    // Verify explorer-table-wrap and cell-selector-text
    assert.match(tableContent, /explorer-table-wrap/, 'ExplorerResourceTable must use explorer-table-wrap class')
    assert.match(tableContent, /cell-selector-text/, 'ExplorerResourceTable must clamp selector text')
    assert.match(columnsContent, /key:\s*'actions'[^}]+width:\s*'130px'/, 'Actions column must be 130px in explorerColumns')

    // Verify CSS rules
    assert.match(cssContent, /table-layout:\s*fixed/i, 'explorer.css must enforce table-layout: fixed')
    assert.match(cssContent, /max-width:\s*1040px/i, 'explorer.css must enforce max-width: 1040px')
    assert.match(cssContent, /\.cell-image-text\s*\{[^}]*max-width:\s*145px/i, 'explorer.css must constrain cell-image-text to 145px')
    assert.match(cssContent, /\.cell-selector-text\s*\{[^}]*max-width:\s*135px/i, 'explorer.css must constrain cell-selector-text to 135px')
    assert.strictEqual(cssContent.includes('.table-box table'), false, 'explorer.css must purge bare generic selector')

    // Verify Vue 3 scoped :deep penetration for DataTable child component
    assert.match(tableContent, /:deep\([^)]*\.explorer-table-wrap/i, 'ExplorerResourceTable must use :deep() penetration for child DataTable')
    assert.match(tableContent, /:deep\(\.data-table\)/i, 'ExplorerResourceTable must include :deep(.data-table)')
    assert.match(tableContent, /table-layout:\s*fixed/i, 'ExplorerResourceTable must enforce table-layout: fixed inside :deep')
  })

  test('Line Count Constraints: All modified files must be strictly under 500 lines', () => {
    const files = [
      'src/components/deployments/DeploymentsTable.vue',
      'src/assets/styles/views/deployments.css',
      'src/assets/styles/components/overview-hosts.css',
      'src/components/overview/nodes/NodeTableView.vue',
      'src/components/ui/DataTable.vue',
      'src/components/slo/SloCatalogTable.vue',
      'src/assets/styles/components/slo-table.css',
      'src/components/explorer/ExplorerResourceTable.vue',
      'src/composables/explorerColumns.ts',
      'src/views/ExplorerView.vue',
      'src/assets/styles/views/explorer.css',
    ]

    for (const file of files) {
      const filePath = path.join(rootDir, file)
      const lines = fs.readFileSync(filePath, 'utf-8').split('\n').length
      assert.ok(lines < 500, `File ${file} has ${lines} lines, exceeding the 500 lines constraint!`)
    }
  })
})
