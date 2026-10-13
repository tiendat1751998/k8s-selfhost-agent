import { describe, it } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const rootDir = path.resolve(__dirname, '..')

describe('Task 012: Cluster Overview HUD & Symmetric PercentageBar Redesign Specification', () => {
  const percentageBarPath = path.join(rootDir, 'src', 'components', 'ui', 'PercentageBar.vue')
  const overviewHudPath = path.join(rootDir, 'src', 'components', 'overview', 'hud', 'OverviewHud.vue')
  const overviewHudProxyPath = path.join(rootDir, 'src', 'components', 'overview', 'OverviewHud.vue')
  const requestFlowBarPath = path.join(rootDir, 'src', 'components', 'overview', 'hud', 'RequestFlowBar.vue')

  it('1. PercentageBar.vue exists and meets strict design and line constraints', () => {
    assert.ok(fs.existsSync(percentageBarPath), 'PercentageBar.vue must exist')
    const content = fs.readFileSync(percentageBarPath, 'utf-8')
    const lines = content.split('\n').length
    assert.ok(lines < 200, `PercentageBar.vue must be < 200 lines, got ${lines}`)

    // Prop support: value and backward-compatible percentage
    assert.ok(content.includes('value?: number') || content.includes('value: number'), 'Must support value prop')
    assert.ok(content.includes('height'), 'Must support height prop')

    // Track background
    assert.ok(content.includes('rgba(255, 255, 255, 0.08)'), 'Track must use rgba(255, 255, 255, 0.08)')
    assert.ok(content.includes('9999px') || content.includes('rounded-full'), 'Must use pill rounded border-radius')

    // Semantic color thresholds: Emerald < 70%, Amber 70-84%, Rose >= 85%
    assert.ok(content.includes('#10b981'), 'Must include emerald #10b981 for < 70%')
    assert.ok(content.includes('#f59e0b'), 'Must include amber #f59e0b for 70-84%')
    assert.ok(content.includes('#f43f5e'), 'Must include rose #f43f5e for >= 85%')

    // Accessibility
    assert.ok(content.includes('role="progressbar"'), 'Must have role progressbar')
    assert.ok(content.includes('aria-valuenow'), 'Must bind aria-valuenow')
  })

  it('2. OverviewHud.vue meets 5-card symmetrical responsive grid constraints', () => {
    assert.ok(fs.existsSync(overviewHudPath), 'OverviewHud.vue must exist in hud/')
    const content = fs.readFileSync(overviewHudPath, 'utf-8')
    const lines = content.split('\n').length
    assert.ok(lines < 400, `OverviewHud.vue must be < 400 lines, got ${lines}`)

    // Desktop: strict 5-column grid repeat(5, minmax(0, 1fr))
    assert.ok(
      content.includes('grid-template-columns: repeat(5, minmax(0, 1fr))') ||
      content.includes('repeat(5, minmax(0, 1fr))'),
      'Desktop must enforce repeat(5, minmax(0, 1fr))'
    )

    // Tablet: 3+2 symmetry
    assert.ok(
      content.includes('768px') && (content.includes('span 2') && content.includes('span 3')),
      'Tablet (768px) must implement symmetric 3+2 grid layout (span 2 on row 1, span 3 on row 2)'
    )

    // Card anatomy constraints: capped 84px–96px, BaseIcon, uppercase label, tabular-nums
    assert.ok(content.includes('height: 88px') || content.includes('max-height: 92px') || content.includes('max-height: 96px'), 'Card height must be capped at 84px–96px')
    assert.ok(content.includes('tabular-nums'), 'Numbers must use font-variant-numeric: tabular-nums')
    assert.ok(content.includes('text-transform: uppercase'), 'Labels must be uppercase')
    assert.ok(content.includes('BaseIcon'), 'Must use vector BaseIcon')

    // Zero fake multipliers or Math.random()
    assert.strictEqual(content.includes('Math.random()'), false, 'Prohibited Math.random() found in OverviewHud.vue')

    // Zero raw emojis
    const emojiRegex = /[\u{1F300}-\u{1F9FF}]|[\u{2600}-\u{26FF}]|[\u{2700}-\u{27BF}]/u
    assert.strictEqual(emojiRegex.test(content), false, 'Prohibited emojis found in OverviewHud.vue')
  })

  it('3. OverviewHud embeds PercentageBar in CPU, Memory, and Storage cards with used vs allocatable', () => {
    const content = fs.readFileSync(overviewHudPath, 'utf-8')
    assert.ok(content.includes('PercentageBar'), 'Must import and use PercentageBar')
    assert.ok(content.includes('clusterUsedMemBytes'), 'Must handle clusterUsedMemBytes')
    assert.ok(content.includes('clusterTotalMemBytes'), 'Must handle clusterTotalMemBytes')
    assert.ok(content.includes('clusterUsedDiskBytes'), 'Must handle clusterUsedDiskBytes')
    assert.ok(content.includes('clusterTotalDiskBytes'), 'Must handle clusterTotalDiskBytes')
    assert.ok(content.includes('formatBytes'), 'Must format bytes for allocatable metrics')
  })

  it('4. OverviewHud.vue re-export proxy exists in components/overview/', () => {
    assert.ok(fs.existsSync(overviewHudProxyPath), 'OverviewHud.vue proxy must exist in components/overview/')
    const content = fs.readFileSync(overviewHudProxyPath, 'utf-8')
    assert.ok(content.includes('OverviewHud'), 'OverviewHud proxy must reference or re-export OverviewHud')
  })

  it('5. RequestFlowBar.vue is compact, high-signal and has zero emojis', () => {
    assert.ok(fs.existsSync(requestFlowBarPath), 'RequestFlowBar.vue must exist')
    const content = fs.readFileSync(requestFlowBarPath, 'utf-8')
    const lines = content.split('\n').length
    assert.ok(lines < 200, `RequestFlowBar.vue must be < 200 lines, got ${lines}`)

    // High signal telemetry
    assert.ok(content.includes('effectiveHttpRps'), 'Must display effectiveHttpRps')
    assert.ok(content.includes('BaseIcon'), 'Must use BaseIcon')

    // Zero emojis
    const emojiRegex = /[\u{1F300}-\u{1F9FF}]|[\u{2600}-\u{26FF}]|[\u{2700}-\u{27BF}]/u
    assert.strictEqual(emojiRegex.test(content), false, 'Prohibited emojis found in RequestFlowBar.vue')
  })

  it('6. All modified or created files strictly stay < 500 lines', () => {
    const files = [
      percentageBarPath,
      overviewHudPath,
      overviewHudProxyPath,
      requestFlowBarPath,
    ]
    for (const f of files) {
      if (fs.existsSync(f)) {
        const count = fs.readFileSync(f, 'utf-8').split('\n').length
        assert.ok(count < 500, `${path.basename(f)} line count ${count} must be < 500`)
      }
    }
  })
})
