import { test, describe } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'

describe('Task 018: Linear Surface Ladder & Tabular-Nums Typography Standard Specifications', () => {
  const rootDir = path.resolve(import.meta.dirname, '..')
  const tokensCssPath = path.join(rootDir, 'src/assets/css/tokens.css')
  const baseCssPath = path.join(rootDir, 'src/assets/css/base.css')

  test('1. File modularity constraints: tokens.css and base.css strictly < 500 lines', () => {
    assert.ok(fs.existsSync(tokensCssPath), 'tokens.css must exist')
    const tokensContent = fs.readFileSync(tokensCssPath, 'utf-8')
    const tokensLineCount = tokensContent.split('\n').length
    assert.ok(tokensLineCount < 500, `tokens.css must be strictly < 500 lines (currently ${tokensLineCount})`)

    assert.ok(fs.existsSync(baseCssPath), 'base.css must exist')
    const baseContent = fs.readFileSync(baseCssPath, 'utf-8')
    const baseLineCount = baseContent.split('\n').length
    assert.ok(baseLineCount < 500, `base.css must be strictly < 500 lines (currently ${baseLineCount})`)
  })

  test('2. Linear 4-tier dark surface ladder and hairline tokens in tokens.css :root', () => {
    const content = fs.readFileSync(tokensCssPath, 'utf-8')

    // Extract :root block before [data-theme="light"]
    const rootBlockMatch = content.match(/:root\s*\{([\s\S]*?)\}/)
    assert.ok(rootBlockMatch, ':root block must exist in tokens.css')
    const rootContent = rootBlockMatch[1]

    // Surface Ladder Level 0 to 3
    assert.match(rootContent, /--color-canvas-dark:\s*#08090a;/, 'Level 0 Canvas must be #08090a')
    assert.match(rootContent, /--color-surface-inset:\s*#0f1011;/, 'Level 1 Inset must be #0f1011')
    assert.match(rootContent, /--color-surface-card:\s*#141516;/, 'Level 2 Card must be #141516')
    assert.match(rootContent, /--color-surface-elevated:\s*#1a1b1e;/, 'Level 3 Elevated must be #1a1b1e')

    // Hairline & Subtle border
    assert.match(rootContent, /--color-hairline:\s*rgba\(255,\s*255,\s*255,\s*0\.08\);/, 'Hairline color must be rgba(255, 255, 255, 0.08)')
    assert.match(rootContent, /--border-subtle:\s*1px\s+solid\s+var\(--color-hairline\);/, 'Subtle border must be 1px solid var(--color-hairline)')

    // Font number token
    assert.match(rootContent, /--font-number:\s*['"]JetBrains Mono['"],\s*['"]Inter['"],\s*monospace;/, '--font-number must be JetBrains Mono, Inter, monospace')
  })

  test('3. Light mode surface ladder and hairline tokens in tokens.css [data-theme="light"]', () => {
    const content = fs.readFileSync(tokensCssPath, 'utf-8')

    const lightBlockMatch = content.match(/\[data-theme=["']light["']\]\s*\{([\s\S]*?)\}/)
    assert.ok(lightBlockMatch, '[data-theme="light"] block must exist in tokens.css')
    const lightContent = lightBlockMatch[1]

    // Light mode surface levels
    assert.match(lightContent, /--color-canvas-dark:\s*#f6f8fa;/, 'Light Canvas must be #f6f8fa')
    assert.match(lightContent, /--color-surface-inset:\s*#edf2f7;/, 'Light Inset must be #edf2f7')
    assert.match(lightContent, /--color-surface-card:\s*#ffffff;/, 'Light Card must be #ffffff')
    assert.match(lightContent, /--color-surface-elevated:\s*#ffffff;/, 'Light Elevated must be #ffffff')
    assert.match(lightContent, /--color-hairline:\s*rgba\(0,\s*0,\s*0,\s*0\.08\);/, 'Light Hairline must be rgba(0, 0, 0, 0.08)')
  })

  test('4. Global tabular-nums enforcement across tables, metrics, badges in base.css', () => {
    const content = fs.readFileSync(baseCssPath, 'utf-8')

    // Check tabular-nums rule in base.css
    assert.match(content, /font-variant-numeric:\s*tabular-nums;/, 'base.css must define font-variant-numeric: tabular-nums')

    // Must target .tabular-nums, table td, .metric-value, .badge, th
    const tabularSelectors = [
      /\.tabular-nums/,
      /table\s+td|td/,
      /\.metric-value/,
      /\.badge/,
      /th/
    ]

    for (const selector of tabularSelectors) {
      assert.match(content, selector, `base.css must include selector ${selector} for tabular-nums`)
    }
  })

  test('5. Negative tracking (-0.025em) on headings, titles, and numeric displays in base.css', () => {
    const content = fs.readFileSync(baseCssPath, 'utf-8')

    // Negative tracking letter-spacing: -0.025em
    assert.match(content, /letter-spacing:\s*-0\.025em;/, 'base.css must include letter-spacing: -0.025em')
  })
})
