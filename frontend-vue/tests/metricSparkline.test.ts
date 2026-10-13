import { test, describe } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'

describe('Task 017: Datadog & Grafana Telemetry HUD with Inline Sparklines Specification', () => {
  const rootDir = path.resolve(import.meta.dirname, '..')
  const metricSparklinePath = path.join(rootDir, 'src/components/ui/MetricSparkline.vue')
  const overviewHudPath = path.join(rootDir, 'src/components/overview/hud/OverviewHud.vue')

  test('1. MetricSparkline.vue exists and strictly obeys line count constraints (< 250 lines)', () => {
    assert.ok(fs.existsSync(metricSparklinePath), 'MetricSparkline.vue must exist in src/components/ui/')
    const content = fs.readFileSync(metricSparklinePath, 'utf-8')
    const lineCount = content.split('\n').length
    assert.ok(lineCount < 250, `MetricSparkline.vue must be strictly < 250 lines (currently ${lineCount})`)
  })

  test('2. MetricSparkline props contract supports data, width, height, color, strokeWidth, fill with correct defaults', () => {
    assert.ok(fs.existsSync(metricSparklinePath), 'MetricSparkline.vue must exist')
    const content = fs.readFileSync(metricSparklinePath, 'utf-8')

    // Props contract
    assert.match(content, /data:\s*number\[\]/, 'Props must declare data: number[]')
    assert.match(content, /width\?:?\s*(number\s*\|\s*string|string\s*\|\s*number)/, 'Props must declare width?: number | string')
    assert.match(content, /height\?:?\s*(number\s*\|\s*string|string\s*\|\s*number)/, 'Props must declare height?: number | string')
    assert.match(content, /color\?:?\s*string/, 'Props must declare color?: string')
    assert.match(content, /strokeWidth\?:?\s*(number\s*\|\s*string|string\s*\|\s*number)/, 'Props must declare strokeWidth?: number | string')
    assert.match(content, /fill\?:?\s*boolean/, 'Props must declare fill?: boolean')

    // Defaults verification
    assert.match(content, /width:\s*36/, 'Default width must be 36')
    assert.match(content, /height:\s*12/, 'Default height must be 12')
    assert.match(content, /color:\s*['"]#10b981['"]/, "Default color must be '#10b981'")
    assert.match(content, /strokeWidth:\s*1\.5/, 'Default strokeWidth must be 1.5')
    assert.match(content, /fill:\s*false/, 'Default fill must be false')
  })

  test('3. MetricSparkline handles edge cases: empty array, single point, all equal values', () => {
    assert.ok(fs.existsSync(metricSparklinePath), 'MetricSparkline.vue must exist')
    const content = fs.readFileSync(metricSparklinePath, 'utf-8')

    // Empty array handling
    assert.match(content, /length\s*===\s*0|!valid\.length|!raw\.length/, 'Must handle empty array safely')

    // Single point handling
    assert.match(content, /length\s*===\s*1/, 'Must handle single data point explicitly')

    // Equal values handling (baseline)
    assert.match(content, /max\s*===\s*min|range\s*===\s*0/, 'Must handle min === max / all equal values baseline')

    // Midpoint baseline calculation
    assert.match(content, /h\s*\/\s*2|height.*\/.*2/, 'Equal values or single point must render centered baseline')
  })

  test('4. SVG path rendering features round linecaps, linejoins, and subpixel coordinates', () => {
    assert.ok(fs.existsSync(metricSparklinePath), 'MetricSparkline.vue must exist')
    const content = fs.readFileSync(metricSparklinePath, 'utf-8')

    // SVG styling and attributes
    assert.match(content, /stroke-linecap=['"]round['"]/, 'Must feature round linecaps')
    assert.match(content, /stroke-linejoin=['"]round['"]/, 'Must feature round linejoins')
    assert.match(content, /viewBox/, 'Must specify SVG viewBox')

    // Fill gradient support
    assert.match(content, /linearGradient/i, 'Must support linearGradient when fill is enabled')
  })

  test('5. OverviewHud.vue integrates MetricSparkline across all 5 telemetry cards', () => {
    assert.ok(fs.existsSync(overviewHudPath), 'OverviewHud.vue must exist')
    const content = fs.readFileSync(overviewHudPath, 'utf-8')

    // Line count check
    const lineCount = content.split('\n').length
    assert.ok(lineCount < 350, `OverviewHud.vue must be strictly < 350 lines (currently ${lineCount})`)

    // Component import
    assert.match(content, /import\s+MetricSparkline\s+from\s+['"].*MetricSparkline\.vue['"]/, 'OverviewHud must import MetricSparkline')

    // Ad-hoc computeSparkline helper must be eradicated
    assert.doesNotMatch(content, /function\s+computeSparkline/, 'Ad-hoc computeSparkline helper must be removed')

    // Must render MetricSparkline in cards
    const sparklineOccurrences = (content.match(/<MetricSparkline/g) || []).length
    assert.strictEqual(sparklineOccurrences, 5, 'OverviewHud must render MetricSparkline across all 5 cards')
  })

  test('6. HUD cards maintain 88px-96px height constraints and tabular numbers typography', () => {
    const content = fs.readFileSync(overviewHudPath, 'utf-8')

    // Height constraint
    assert.match(content, /height:\s*88px/, 'HUD card height must be set to 88px')

    // Tabular numbers
    assert.match(content, /tabular-nums/, 'Must enforce tabular-nums typography')
  })
})
