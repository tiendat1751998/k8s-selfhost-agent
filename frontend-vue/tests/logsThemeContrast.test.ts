import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

describe('Logs View Dual-Theme & Contrast Specifications (LES-003)', () => {
  const radarCssPath = path.resolve(__dirname, '../src/assets/styles/components/log-error-radar.css')
  const virtualTerminalCssPath = path.resolve(__dirname, '../src/assets/styles/components/virtual-log-terminal.css')
  const terminalCssPath = path.resolve(__dirname, '../src/assets/styles/views/logstream-terminal.css')

  it('Blind Error Radar: zero hardcoded #f1f5f9, #080d1a, or rgba(15, 23, 42) and uses semantic tokens', () => {
    assert.ok(fs.existsSync(radarCssPath), 'log-error-radar.css must exist')
    const content = fs.readFileSync(radarCssPath, 'utf-8')

    // Must not contain hardcoded dark colors breaking light mode
    assert.ok(!content.includes('#f1f5f9'), 'Must not contain hardcoded #f1f5f9 (breaks light mode title/buttons)')
    assert.ok(!content.includes('#080d1a'), 'Must not contain hardcoded #080d1a (breaks light mode card surface)')
    assert.ok(!content.includes('rgba(15, 23, 42'), 'Must not contain hardcoded rgba(15, 23, 42) (breaks elevated header in light mode)')

    // Must use semantic tokens
    assert.ok(content.includes('var(--text-primary)'), 'Must use var(--text-primary) for high contrast text')
    assert.ok(content.includes('var(--bg-card') || content.includes('var(--color-surface-card'), 'Must use semantic card surface')
    assert.ok(content.includes('var(--bg-elevated') || content.includes('var(--color-surface-elevated'), 'Must use semantic elevated surface')
  })

  it('Virtual Log Terminal: zero hardcoded pitch-black #080c16, rgba(15, 23, 42), or #f8fafc text', () => {
    assert.ok(fs.existsSync(virtualTerminalCssPath), 'virtual-log-terminal.css must exist')
    const content = fs.readFileSync(virtualTerminalCssPath, 'utf-8')

    assert.ok(!content.includes('#080c16'), 'Must not contain hardcoded pitch-black #080c16')
    assert.ok(!content.includes('rgba(15, 23, 42'), 'Must not contain rgba(15, 23, 42)')
    assert.ok(!content.includes('color: #f8fafc'), 'Log message must not use hardcoded white color: #f8fafc')

    // Must use semantic tokens
    assert.ok(content.includes('var(--bg-card') || content.includes('var(--bg-surface'), 'Terminal window/viewport must use semantic surface')
    assert.ok(content.includes('var(--text-primary)'), 'Log message text must use var(--text-primary)')
  })

  it('LogStream Terminal stylesheet: zero hardcoded #080c16 or rgba(15, 23, 42)', () => {
    assert.ok(fs.existsSync(terminalCssPath), 'logstream-terminal.css must exist')
    const content = fs.readFileSync(terminalCssPath, 'utf-8')

    assert.ok(!content.includes('#080c16'), 'Must not contain hardcoded pitch-black #080c16')
    assert.ok(!content.includes('rgba(15, 23, 42'), 'Must not contain rgba(15, 23, 42)')
    assert.ok(!content.includes('color: #f8fafc'), 'Log message must not use hardcoded white color: #f8fafc')
  })

  it('File modularity: all touched stylesheets must strictly stay < 500 lines', () => {
    for (const p of [radarCssPath, virtualTerminalCssPath, terminalCssPath]) {
      const lines = fs.readFileSync(p, 'utf-8').split('\n').length
      assert.ok(lines < 500, `${path.basename(p)} line count (${lines}) must be < 500 lines`)
    }
  })
})
