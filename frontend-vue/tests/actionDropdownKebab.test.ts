import { describe, it } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const rootDir = path.resolve(__dirname, '..')

describe('Task 011: Enterprise Table Actions Kebab Menu & Drawer Standardization Specifications', () => {
  const actionDropdownPath = path.join(rootDir, 'src', 'components', 'ui', 'ActionDropdown.vue')
  const fleetClustersTablePath = path.join(rootDir, 'src', 'components', 'fleet', 'FleetClustersTable.vue')
  const nodeHeadroomTablePath = path.join(rootDir, 'src', 'components', 'capacity', 'NodeHeadroomTable.vue')
  const nodeTableViewPath = path.join(rootDir, 'src', 'components', 'overview', 'nodes', 'NodeTableView.vue')

  it('1. ActionDropdown.vue meets enterprise standards and line constraints', () => {
    assert.ok(fs.existsSync(actionDropdownPath), 'ActionDropdown.vue must exist')
    const content = fs.readFileSync(actionDropdownPath, 'utf-8')
    const lineCount = content.split('\n').length
    assert.ok(lineCount < 250, `ActionDropdown.vue must be strictly < 250 lines (currently ${lineCount})`)

    // Props & Types
    assert.ok(content.includes('ActionItem'), 'Must export/define ActionItem interface')
    assert.ok(content.includes('ariaLabel') || content.includes('aria-label'), 'Must support aria-label / ariaLabel prop')
    assert.ok(content.includes('isDestructive') || content.includes('variant-danger'), 'Must support destructive actions')

    // Accessibility & Behavior
    assert.ok(content.includes('more-vertical'), 'Must use more-vertical icon as kebab trigger')
    assert.ok(content.includes('Teleport'), 'Must teleport dropdown menu to body')
    assert.ok(content.includes('aria-haspopup="true"'), 'Must specify aria-haspopup="true"')
    assert.ok(content.includes('aria-expanded'), 'Must bind aria-expanded')
    assert.ok(content.includes('Escape'), 'Must support Escape dismissal')
    assert.ok(content.includes('emit(\'select\'') || content.includes('emit("select"'), 'Must emit select event')
  })

  it('2. FleetClustersTable.vue standardizes row actions with ActionDropdown kebab menu', () => {
    assert.ok(fs.existsSync(fleetClustersTablePath), 'FleetClustersTable.vue must exist')
    const content = fs.readFileSync(fleetClustersTablePath, 'utf-8')
    const lineCount = content.split('\n').length
    assert.ok(lineCount < 500, `FleetClustersTable.vue must be strictly < 500 lines (currently ${lineCount})`)

    // Kebab ActionDropdown integration
    assert.ok(content.includes('ActionDropdown'), 'Must import and use ActionDropdown')
    assert.ok(content.includes('clusterActions'), 'Must define clusterActions with standardized actions')
    assert.ok(content.includes('handleAction'), 'Must handle action selection via handleAction')

    // Standardized actions: Inspect, Probe, Rebalance, Cordon / Drain, Evict
    assert.ok(content.includes('Inspect'), 'clusterActions must include Inspect')
    assert.ok(content.includes('Probe'), 'clusterActions must include Probe')
    assert.ok(content.includes('Rebalance'), 'clusterActions must include Rebalance')
    assert.ok(content.includes('Cordon'), 'clusterActions must include Cordon / Drain')
    assert.ok(content.includes('Evict'), 'clusterActions must include Evict (destructive)')

    // Eradicate inline action buttons from cell
    assert.strictEqual(content.includes('btn-primary btn-xs'), false, 'Must eradicate inline Details/Probe/Upgrade buttons')
    assert.strictEqual(content.includes('title="Cluster Details"\n            @click='), false, 'Must not render inline cluster details button')

    // 40px fixed width actions column
    assert.ok(content.includes("width: '40px'") || content.includes('width: "40px"'), 'Actions column width must be fixed 40px')
  })

  it('3. NodeHeadroomTable.vue uses ActionDropdown with 40px actions column and zero inline buttons', () => {
    assert.ok(fs.existsSync(nodeHeadroomTablePath), 'NodeHeadroomTable.vue must exist')
    const content = fs.readFileSync(nodeHeadroomTablePath, 'utf-8')
    const lineCount = content.split('\n').length
    assert.ok(lineCount < 500, `NodeHeadroomTable.vue must be strictly < 500 lines (currently ${lineCount})`)

    assert.ok(content.includes('ActionDropdown'), 'Must import and use ActionDropdown')
    assert.ok(content.includes('width: 40px'), 'Actions column colgroup width must be fixed 40px')
    assert.strictEqual(content.includes('⚡ Rebalance'), false, 'Must not have inline rebalance button')
    assert.strictEqual(content.includes('🔍 Inspect'), false, 'Must not have inline inspect button')
  })

  it('4. NodeTableView.vue eradicates inline action buttons and standardizes on ActionDropdown', () => {
    assert.ok(fs.existsSync(nodeTableViewPath), 'NodeTableView.vue must exist')
    const content = fs.readFileSync(nodeTableViewPath, 'utf-8')
    const lineCount = content.split('\n').length
    assert.ok(lineCount < 500, `NodeTableView.vue must be strictly < 500 lines (currently ${lineCount})`)

    assert.ok(content.includes('ActionDropdown'), 'Must import and use ActionDropdown')
    // Eradicate inline buttons
    assert.strictEqual(content.includes('btn-logs'), false, 'Must eradicate inline btn-logs')
    assert.strictEqual(content.includes('btn-details'), false, 'Must eradicate inline btn-details')
    assert.strictEqual(content.includes('sre-btn'), false, 'Must eradicate sre-btn inline buttons')
  })

  it('5. All modified files strictly obey line count limits (< 500 lines)', () => {
    const files = [
      actionDropdownPath,
      fleetClustersTablePath,
      nodeHeadroomTablePath,
      nodeTableViewPath,
    ]
    for (const file of files) {
      const lines = fs.readFileSync(file, 'utf-8').split('\n').length
      assert.ok(lines < 500, `${path.basename(file)} must be < 500 lines, was ${lines}`)
    }
  })
})
