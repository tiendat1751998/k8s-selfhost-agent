import { describe, it } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const rootDir = path.resolve(__dirname, '..')

describe('Mobile Hosts View Defect Remediation & Card Layout Specifications', () => {
  const headerBarPath = path.join(rootDir, 'src', 'components', 'hosts', 'InfraHostsHeaderBar.vue')
  const mobileCardsPath = path.join(rootDir, 'src', 'components', 'hosts', 'HostsMobileCards.vue')
  const responsiveCssPath = path.join(rootDir, 'src', 'assets', 'styles', 'views', 'infra-hosts-responsive.css')
  const hostsCssPath = path.join(rootDir, 'src', 'assets', 'styles', 'views', 'infra-hosts.css')
  const hostsViewPath = path.join(rootDir, 'src', 'views', 'InfraHostsView.vue')

  it('1. InfraHostsHeaderBar.vue: eliminates btn-micro-fence from 20px strip and keeps clean single-line telemetry', () => {
    assert.ok(fs.existsSync(headerBarPath), 'InfraHostsHeaderBar.vue must exist')
    const content = fs.readFileSync(headerBarPath, 'utf-8')

    // Must NOT contain btn-micro-fence inside hosts-micro-telemetry
    const telemetryMatch = content.match(/<div class="hosts-micro-telemetry[^>]*>([\s\S]*?)<\/div>/)
    assert.ok(telemetryMatch, 'hosts-micro-telemetry must exist')
    const telemetryInner = telemetryMatch[1]

    assert.ok(
      !telemetryInner.includes('btn-micro-fence'),
      'hosts-micro-telemetry must not contain btn-micro-fence button (redundant with fencing-alert-strip)'
    )
    assert.ok(
      telemetryInner.includes('onlineHostsCount'),
      'hosts-micro-telemetry must display online hosts count'
    )
    assert.ok(
      telemetryInner.includes('offlineHostsCount'),
      'hosts-micro-telemetry must display offline hosts count'
    )
  })

  it('2. HostsMobileCards.vue: re-architects card with header, meta row, and footer action bar with fencing', () => {
    assert.ok(fs.existsSync(mobileCardsPath), 'HostsMobileCards.vue must exist')
    const content = fs.readFileSync(mobileCardsPath, 'utf-8')

    // Must import StatusBadge
    assert.match(
      content,
      /import\s+StatusBadge\s+from\s+['"]\.\.\/ui\/StatusBadge\.vue['"]/,
      'HostsMobileCards.vue must import StatusBadge'
    )

    // Must emit fencing event
    assert.match(
      content,
      /\(e:\s*'fencing',\s*host:\s*ComputeHost\)/,
      'HostsMobileCards.vue must declare fencing emit'
    )

    // Header row contains BaseIcon, m-host-name, m-type-chip, and StatusBadge
    assert.ok(
      content.includes('m-card-header-row') || content.includes('m-card-header'),
      'HostsMobileCards.vue must have structured header row'
    )
    assert.ok(
      content.includes('m-type-chip'),
      'HostsMobileCards.vue must render role/type chip with m-type-chip class'
    )
    assert.match(
      content,
      /<StatusBadge[^>]+status=/,
      'HostsMobileCards.vue must render StatusBadge component'
    )

    // Meta row contains endpoint / IP and latency badge or last check
    assert.ok(
      content.includes('m-card-meta-row') || content.includes('m-card-body-row'),
      'HostsMobileCards.vue must have structured meta row'
    )
    assert.ok(
      content.includes('m-endpoint'),
      'HostsMobileCards.vue must render m-endpoint'
    )
    assert.ok(
      content.includes('last_health_check') || content.includes('formatDate'),
      'HostsMobileCards.vue must display last health check when latency is not available'
    )

    // Footer action bar with Test, Edit, Delete, and Fence buttons
    assert.ok(
      content.includes('m-card-footer-row') || content.includes('m-card-footer'),
      'HostsMobileCards.vue must have comfortable footer action bar'
    )
    assert.match(
      content,
      /emit\('fencing',\s*host\)/,
      'HostsMobileCards.vue must emit fencing when Fence button clicked'
    )
  })

  it('3. infra-hosts-responsive.css & infra-hosts.css: responsive card flex, m-type-chip styling, and touch buttons', () => {
    assert.ok(fs.existsSync(responsiveCssPath), 'infra-hosts-responsive.css must exist')
    const css = fs.readFileSync(responsiveCssPath, 'utf-8')

    // Mobile host card must NOT have rigid height: 64px
    assert.ok(
      !css.includes('max-height: 64px;'),
      'infra-hosts-responsive.css must remove rigid max-height: 64px from .mobile-host-card'
    )
    assert.match(
      css,
      /\.mobile-host-card\s*\{[^}]*min-height:\s*80px/s,
      'infra-hosts-responsive.css must allow mobile-host-card to flex with min-height: 80px'
    )

    // m-type-chip must be styled
    assert.match(
      css,
      /\.m-type-chip\s*\{[^}]*font-size:\s*10px/s,
      'infra-hosts-responsive.css must style .m-type-chip with font-size: 10px'
    )

    // mobile-hosts-stream has width 100% and flex column
    assert.match(
      css,
      /\.mobile-hosts-stream\s*\{[^}]*display:\s*flex/s,
      '.mobile-hosts-stream must use flex layout'
    )
  })

  it('4. InfraHostsView.vue: handles fencing event from mobile cards', () => {
    assert.ok(fs.existsSync(hostsViewPath), 'InfraHostsView.vue must exist')
    const content = fs.readFileSync(hostsViewPath, 'utf-8')

    // Must bind @fencing to HostsMobileCards
    assert.match(
      content,
      /<HostsMobileCards[^>]*@fencing=/,
      'InfraHostsView.vue must bind @fencing to HostsMobileCards'
    )
  })

  it('5. Line count constraints: all modified files strictly < 500 lines', () => {
    const files = [
      headerBarPath,
      mobileCardsPath,
      responsiveCssPath,
      hostsCssPath,
      hostsViewPath
    ]

    for (const file of files) {
      if (fs.existsSync(file)) {
        const lines = fs.readFileSync(file, 'utf-8').split('\n').length
        assert.ok(
          lines < 500,
          `${path.basename(file)} must be strictly under 500 lines (current: ${lines})`
        )
      }
    }
  })
})
