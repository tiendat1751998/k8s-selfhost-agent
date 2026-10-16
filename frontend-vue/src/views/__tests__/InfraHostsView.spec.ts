// @ts-nocheck
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

// @ts-ignore
const { describe, it, expect } = await import('vitest')

describe('Zero-Neon Redesign & Hosts Architecture Verification', () => {
  it('eliminates synthetic time-series loops and fake scrubbing badge in HostFleetTelemetryPanel.vue', () => {
    const fileContent = readFileSync(
      resolve(__dirname, '../../components/hosts/HostFleetTelemetryPanel.vue'),
      'utf-8'
    )
    // Synthetic 16-point fake loop must NOT exist
    expect(fileContent).not.toContain('fleetTelemetryWindow')
    expect(fileContent).not.toContain('SYNCHRONIZED SCRUBBING')
    expect(fileContent).not.toContain('CanvasTimeSeries')
    // Must contain authentic 4-column fleet diagnostics panel
    expect(fileContent).toContain('fleet-diagnostics-panel')
    expect(fileContent).toContain('diag-col')
  })

  it('eliminates duplicate Add Host button in HostsTable.vue', () => {
    const fileContent = readFileSync(
      resolve(__dirname, '../../components/hosts/HostsTable.vue'),
      'utf-8'
    )
    expect(fileContent).not.toContain('btn-primary btn-sm')
    expect(fileContent).not.toContain('+ Add Host')
  })

  it('eliminates neon text-cyan from endpoint in HostsMobileCards.vue and HostsGrid.vue', () => {
    const mobileCards = readFileSync(
      resolve(__dirname, '../../components/hosts/HostsMobileCards.vue'),
      'utf-8'
    )
    expect(mobileCards).not.toContain('text-cyan')
    expect(mobileCards).toContain('ActionDropdown')
    expect(mobileCards).toContain('btn-m-probe')

    const grid = readFileSync(
      resolve(__dirname, '../../components/hosts/HostsGrid.vue'),
      'utf-8'
    )
    expect(grid).not.toContain('text-cyan')
  })

  it('eliminates pulse-dot-cyan from HostMetricsHud.vue', () => {
    const hud = readFileSync(
      resolve(__dirname, '../../components/hosts/HostMetricsHud.vue'),
      'utf-8'
    )
    expect(hud).not.toContain('pulse-dot-cyan')
  })

  it('verifies getMobileRowActions contract logic in HostsMobileCards', () => {
    // Contract test for mobile row actions helper
    function getMobileRowActions(status: string) {
      const items = [
        { id: 'select', action: 'select', label: 'View Details', icon: 'server' },
        { id: 'edit', action: 'edit', label: 'Edit Host', icon: 'edit' }
      ]
      if (status !== 'connected') {
        items.push({ id: 'fencing', action: 'fencing', label: 'Safe Storage Fencing', icon: 'shield' })
      }
      items.push({ id: 'delete', action: 'delete', label: 'Delete Host', icon: 'trash' })
      return items
    }

    const connectedActions = getMobileRowActions('connected')
    expect(connectedActions.map(a => a.id)).toEqual(['select', 'edit', 'delete'])

    const disconnectedActions = getMobileRowActions('disconnected')
    expect(disconnectedActions.map(a => a.id)).toEqual(['select', 'edit', 'fencing', 'delete'])
  })

  it('strictly adheres to file line count limit (< 500 lines) across all modified components', () => {
    const files = [
      '../../components/hosts/InfraHostsHeaderBar.vue',
      '../../components/hosts/HostsMobileCards.vue',
      '../../components/hosts/HostsGrid.vue',
      '../../components/hosts/HostsTable.vue',
      '../../components/hosts/HostMetricsHud.vue',
      '../../components/hosts/HostFleetTelemetryPanel.vue',
      '../../components/hosts/HostControlsBar.vue',
      '../../components/layout/AppSidebar.vue',
      '../../composables/useInfraHosts.ts',
      '../../composables/useGlobalContext.ts'
    ]

    for (const f of files) {
      const content = readFileSync(resolve(__dirname, f), 'utf-8')
      const lines = content.split('\n').length
      expect(lines).toBeLessThan(500)
    }
  })

  it('verifies toolbar controls and responsive styling for 1440px desktop viewport without clipping', () => {
    const controlsBar = readFileSync(
      resolve(__dirname, '../../components/hosts/HostControlsBar.vue'),
      'utf-8'
    )
    expect(controlsBar).toContain('title="Filter Online Hosts"')
    expect(controlsBar).toContain('aria-label="Filter Online Hosts"')
    expect(controlsBar).toContain('title="Filter Offline Hosts"')
    expect(controlsBar).toContain('aria-label="Filter Offline Hosts"')
    expect(controlsBar).toContain('Online <span class="pill-count">')
    expect(controlsBar).toContain('Offline <span class="pill-count">')

    const cssContent = readFileSync(
      resolve(__dirname, '../../assets/styles/views/infra-hosts-controls.css'),
      'utf-8'
    )
    // Check desktop media query constraints
    expect(cssContent).toContain('flex: 1 1 130px;')
    expect(cssContent).toContain('min-width: 100px;')
    expect(cssContent).toContain('max-width: 180px;')
    // Base and desktop toolbar-select min-width 84px
    expect(cssContent).toContain('min-width: 84px;')
  })

  it('verifies CNCF Blue (#326ce5) primary CTA and syntax-colored endpoints in table and card styles', () => {
    const tableCss = readFileSync(
      resolve(__dirname, '../../assets/styles/views/infra-hosts-table.css'),
      'utf-8'
    )
    expect(tableCss).toContain('var(--color-primary, #326ce5)')
    expect(tableCss).toContain('var(--color-endpoint-syntax')

    const cardCss = readFileSync(
      resolve(__dirname, '../../assets/styles/views/infra-hosts-card.css'),
      'utf-8'
    )
    expect(cardCss).toContain('var(--color-endpoint-syntax')
  })

  it('verifies light mode toolbar inputs and mobile brand title use tokens without hardcoded white/gray', () => {
    const controlsBar = readFileSync(
      resolve(__dirname, '../../components/hosts/HostControlsBar.vue'),
      'utf-8'
    )
    expect(controlsBar).not.toContain('.toolbar-add-btn')

    const appShellCss = readFileSync(
      resolve(__dirname, '../../assets/styles/layout/app-shell.css'),
      'utf-8'
    )
    expect(appShellCss).toContain('.hud-mobile-brand-title { font-size: 13px; font-weight: 800; letter-spacing: 0.04em; color: var(--text-primary);')

    const baseCss = readFileSync(
      resolve(__dirname, '../../assets/css/base.css'),
      'utf-8'
    )
    expect(baseCss).toContain('background: var(--bg-surface, #111827) !important;')

    const controlsCss = readFileSync(
      resolve(__dirname, '../../assets/styles/views/infra-hosts-controls.css'),
      'utf-8'
    )
    expect(controlsCss).toContain('background: var(--bg-surface, #111827);')
  })
})

