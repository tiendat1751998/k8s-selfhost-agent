import { describe, it } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const rootDir = path.resolve(__dirname, '..')

describe('Task 013: Capacity & Headroom Matrix Anti-Neon Redesign Specifications', () => {
  const nodeHeadroomTablePath = path.join(rootDir, 'src', 'components', 'capacity', 'NodeHeadroomTable.vue')

  it('1. NodeHeadroomTable.vue exists and meets strict line limit (< 400 lines)', () => {
    assert.ok(fs.existsSync(nodeHeadroomTablePath), 'NodeHeadroomTable.vue must exist')
    const content = fs.readFileSync(nodeHeadroomTablePath, 'utf-8')
    const lineCount = content.split('\n').length
    assert.ok(lineCount < 400, `NodeHeadroomTable.vue must be strictly < 400 lines (currently ${lineCount})`)
  })

  it('2. HEALTH column: eradicates all interactive buttons and displays clean StatusBadge', () => {
    const content = fs.readFileSync(nodeHeadroomTablePath, 'utf-8')

    // Column header exists
    assert.ok(content.includes('<th>Health</th>'), 'Must have explicit Health column header')

    // Eradicate interactive buttons from HEALTH column or anywhere inline
    assert.strictEqual(content.includes('⚡ Rebalance'), false, 'Must not render ⚡ Rebalance button')
    assert.strictEqual(content.includes('🔍 Inspect'), false, 'Must not render 🔍 Inspect inline button')
    assert.strictEqual(content.includes('~~Rebalance~~'), false, 'Must not contain strikethrough rebalance')

    // Uses StatusBadge
    assert.ok(content.includes('<StatusBadge'), 'Must render StatusBadge in table')
    assert.ok(content.includes(':status='), 'StatusBadge must bind :status')

    // Status logic supports Ready, Cordoned, High Commitment
    assert.ok(content.includes('Ready') || content.includes('ready'), 'Must support Ready status')
    assert.ok(content.includes('Cordoned') || content.includes('cordoned'), 'Must support Cordoned status')
    assert.ok(content.includes('High Commitment') || content.includes('high'), 'Must support High Commitment status')
  })

  it('3. ACTIONS column: standardizes on ActionDropdown with fixed 40px width and required actions', () => {
    const content = fs.readFileSync(nodeHeadroomTablePath, 'utf-8')

    // Colgroup and cell constraints
    assert.ok(content.includes('width: 40px'), 'Actions column width in colgroup must be fixed 40px')

    // Uses ActionDropdown with handleAction($event, node)
    assert.ok(content.includes('<ActionDropdown'), 'Must render ActionDropdown')
    assert.ok(content.includes(':items="nodeActions"'), 'Must bind :items="nodeActions"')
    assert.ok(
      content.includes('@select="handleAction($event, node)"') ||
      content.includes('handleAction($event, node)'),
      'Must handle select with handleAction($event, node)'
    )

    // Standardized actions: Rebalance Pods, Inspect Allocations, Drain Node (destructive)
    assert.ok(content.includes('Rebalance Pods'), 'nodeActions must contain Rebalance Pods')
    assert.ok(content.includes('Inspect Allocations'), 'nodeActions must contain Inspect Allocations')
    assert.ok(content.includes('Drain Node'), 'nodeActions must contain Drain Node')
    assert.ok(
      content.includes('isDestructive: true') || content.includes("variant: 'danger'"),
      'Drain Node must be marked as destructive'
    )
  })

  it('4. Unified Semantic Allocation Bars: enforces single color spectrum across CPU and Memory', () => {
    const content = fs.readFileSync(nodeHeadroomTablePath, 'utf-8')

    // Semantic threshold colors
    assert.ok(content.includes('#06b6d4'), 'Normal allocation (<70%) must use #06b6d4 (Cyan/Slate)')
    assert.ok(content.includes('#f59e0b'), 'Caution allocation (70%-84.9%) must use #f59e0b (Warm Amber)')
    assert.ok(content.includes('#f43f5e'), 'Critical allocation (>=85%) must use #f43f5e (Rose/Coral)')

    // Progress bar height 4px - 6px
    assert.ok(
      content.includes(':height="4"') ||
      content.includes(':height="5"') ||
      content.includes(':height="6"'),
      'PercentageBar must use clean height between 4px and 6px'
    )

    // Allocatable subtext
    assert.ok(content.includes('cpuAllocatedCores') && content.includes('cpuTotalCores'), 'Must show CPU allocated/total cores subtext')
    assert.ok(content.includes('memAllocatedGiB') && content.includes('memTotalGiB'), 'Must show Memory allocated/total GiB subtext')
  })

  it('5. Table Layout & Anti-Neon CSS: responsive fixed table layout without horizontal blowout', () => {
    const content = fs.readFileSync(nodeHeadroomTablePath, 'utf-8')

    // Responsive table-layout fixed
    assert.ok(content.includes('table-layout: fixed'), 'Must enforce table-layout: fixed')
    assert.ok(content.includes('node-table-wrapper'), 'Must have node-table-wrapper')
    assert.ok(content.includes('overflow-x: auto'), 'Wrapper must handle overflow-x safely')

    // Total file line count constraint (< 400 lines)
    const lineCount = content.split('\n').length
    assert.ok(lineCount < 400, `File must be strictly < 400 lines (currently ${lineCount})`)
  })
})
