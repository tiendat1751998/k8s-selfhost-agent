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
    assert.match(content, /\.table\s+th,\s*\.table\s+td/i, 'deployments.css must specify .table th, .table td')
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

  test('Line Count Constraints: All modified files must be strictly under 500 lines', () => {
    const files = [
      'src/components/deployments/DeploymentsTable.vue',
      'src/assets/styles/views/deployments.css',
      'src/assets/styles/components/overview-hosts.css',
      'src/components/overview/nodes/NodeTableView.vue',
      'src/components/ui/DataTable.vue',
    ]

    for (const file of files) {
      const filePath = path.join(rootDir, file)
      const lines = fs.readFileSync(filePath, 'utf-8').split('\n').length
      assert.ok(lines < 500, `File ${file} has ${lines} lines, exceeding the 500 lines constraint!`)
    }
  })
})
