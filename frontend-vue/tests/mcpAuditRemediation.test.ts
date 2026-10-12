import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const rootDir = path.resolve(__dirname, '..')

describe('MCP Audit Remediation: 3 Critical Deficiencies Verification', () => {
  const logStorePath = path.join(rootDir, 'src', 'stores', 'logStore.ts')
  const usePodTerminalPath = path.join(rootDir, 'src', 'composables', 'usePodTerminal.ts')
  const useWebSocketPath = path.join(rootDir, 'src', 'composables', 'useWebSocket.ts')
  const dataTablePath = path.join(rootDir, 'src', 'components', 'ui', 'DataTable.vue')
  const dataTableCssPath = path.join(rootDir, 'src', 'assets', 'styles', 'components', 'ui', 'data-table.css')
  const auditTrailTablePath = path.join(rootDir, 'src', 'components', 'audit', 'AuditTrailTable.vue')
  const vulnTablePath = path.join(rootDir, 'src', 'components', 'audit', 'VulnerabilityFindingsTable.vue')
  const virtualLogTerminalPath = path.join(rootDir, 'src', 'components', 'logs', 'VirtualLogTerminal.vue')
  const virtualLogTerminalCssPath = path.join(rootDir, 'src', 'assets', 'styles', 'components', 'virtual-log-terminal.css')

  it('Defect A: logStore, usePodTerminal, and useWebSocket sanitize URL and delete token from query string', () => {
    const logStoreContent = fs.readFileSync(logStorePath, 'utf-8')
    const podTermContent = fs.readFileSync(usePodTerminalPath, 'utf-8')
    const wsContent = fs.readFileSync(useWebSocketPath, 'utf-8')

    // logStore must sanitize params and URL to prevent any token in query string
    assert.ok(
      logStoreContent.includes("searchParams.delete('token')") || logStoreContent.includes("params.delete('token')"),
      'logStore must delete token from query params'
    )
    assert.strictEqual(
      logStoreContent.includes("params.append('token'"),
      false,
      'logStore must never append token to query params'
    )

    // usePodTerminal must sanitize and delete token from query string
    assert.ok(
      podTermContent.includes("searchParams.delete('token')"),
      'usePodTerminal must delete token from URL searchParams'
    )

    // useWebSocket must sanitize and delete token from query string
    assert.ok(
      wsContent.includes("searchParams.delete('token')"),
      'useWebSocket must delete token from URL searchParams'
    )

    // All three must pass token via WebSocket subprotocol
    for (const [name, content] of [
      ['logStore', logStoreContent],
      ['usePodTerminal', podTermContent],
      ['useWebSocket', wsContent],
    ]) {
      assert.ok(
        content.includes("['bearer', token]") || content.includes('["bearer", token]'),
        `${name} must pass token strictly via Sec-WebSocket-Protocol ['bearer', token]`
      )
    }
  })

  it('Defect B: DataTable error state preserves table-cell display and spans all 7 columns', () => {
    const dataTableContent = fs.readFileSync(dataTablePath, 'utf-8')
    const dataTableCss = fs.readFileSync(dataTableCssPath, 'utf-8')
    const auditTableContent = fs.readFileSync(auditTrailTablePath, 'utf-8')
    const vulnTableContent = fs.readFileSync(vulnTablePath, 'utf-8')

    // Audit tables have 7 columns
    assert.ok(auditTableContent.includes('key: \'actions\'') && auditTableContent.includes('key: \'timestamp\''))
    assert.ok(vulnTableContent.includes('key: \'actions\'') && vulnTableContent.includes('key: \'id\''))

    // DataTable must bind colspan to columns.length
    assert.ok(
      dataTableContent.includes(':colspan="columns.length') || dataTableContent.includes(':colspan="columns.length || 7"'),
      'DataTable error row must bind colspan to total columns'
    )

    // DataTable must wrap inner error content so td preserves table-cell formatting
    assert.ok(
      dataTableContent.includes('cell-error-content') || dataTableContent.includes('cell-error-inner'),
      'DataTable must use inner container for flex alignment'
    )

    // data-table.css must NOT set display: flex directly on .cell-error
    const cellErrorCss = dataTableCss.match(/\.cell-error\s*\{([^}]+)\}/)?.[1] || ''
    assert.strictEqual(
      cellErrorCss.includes('display: flex'),
      false,
      '.cell-error on td must not have display: flex, which breaks colspan in table rendering engines'
    )

    // data-table.css must have flex layout on .cell-error-content
    assert.ok(
      dataTableCss.includes('.cell-error-content') || dataTableCss.includes('.cell-error-inner'),
      'data-table.css must style inner error wrapper'
    )
  })

  it('Defect C: VirtualLogTerminal provides responsive status badge without character truncation on mobile', () => {
    const vueContent = fs.readFileSync(virtualLogTerminalPath, 'utf-8')
    const cssContent = fs.readFileSync(virtualLogTerminalCssPath, 'utf-8')

    // Must have responsive text classes or structure for mobile vs desktop
    assert.ok(
      vueContent.includes('fps-text-desktop') && vueContent.includes('fps-text-mobile'),
      'VirtualLogTerminal must provide separate desktop and mobile text elements'
    )
    assert.ok(
      vueContent.includes('60 FPS Virtualized') && vueContent.includes('60 FPS'),
      'VirtualLogTerminal must render compact 60 FPS on mobile'
    )

    // CSS must include media queries toggling text and preventing flex shrink/truncation
    assert.ok(
      cssContent.includes('.fps-text-mobile') && cssContent.includes('.fps-text-desktop'),
      'CSS must style fps-text-mobile and fps-text-desktop'
    )
    assert.ok(
      cssContent.includes('flex-shrink: 0') && cssContent.includes('white-space: nowrap'),
      'virtual-fps-badge must enforce flex-shrink: 0 and white-space: nowrap'
    )
  })

  it('Line counts: all modified files remain strictly < 500 lines', () => {
    const files = [
      logStorePath,
      usePodTerminalPath,
      useWebSocketPath,
      dataTablePath,
      dataTableCssPath,
      virtualLogTerminalPath,
      virtualLogTerminalCssPath,
      __filename,
    ]

    for (const f of files) {
      const lines = fs.readFileSync(f, 'utf-8').split('\n').length
      assert.ok(lines < 500, `File ${path.basename(f)} (${lines} lines) must be strictly < 500 lines`)
    }
  })
})
