import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const rootDir = path.resolve(__dirname, '..')

describe('Hyperscale Phase 3: Overview Density Matrix & Responsive Grid Wiring', () => {
  const overviewViewPath = path.join(rootDir, 'src/views/OverviewView.vue')
  const topologySectionPath = path.join(rootDir, 'src/components/overview/OverviewTopologySection.vue')
  const virtualGridPath = path.join(rootDir, 'src/components/overview/virtual/VirtualNodeGrid.vue')

  it('1. OverviewView.vue and OverviewTopologySection.vue expand nodeViewMode to table | grid | matrix', () => {
    const overviewContent = fs.readFileSync(overviewViewPath, 'utf-8')
    const topologyContent = fs.readFileSync(topologySectionPath, 'utf-8')

    // OverviewView checks
    assert.ok(
      overviewContent.includes("'grid' | 'table' | 'matrix'") ||
      overviewContent.includes("'table' | 'grid' | 'matrix'"),
      "OverviewView.vue must expand nodeViewMode to include 'matrix'"
    )
    assert.ok(
      overviewContent.includes('>= 500') && overviewContent.includes("'matrix'"),
      "OverviewView.vue must auto-switch nodeViewMode to 'matrix' if nodes >= 500"
    )

    // TopologySection checks
    assert.ok(
      topologyContent.includes('ClusterDensityMatrix'),
      'OverviewTopologySection.vue must import ClusterDensityMatrix'
    )
    assert.ok(
      topologyContent.includes('Matrix (10k)'),
      'OverviewTopologySection.vue must render Matrix (10k) toggle button'
    )
    assert.ok(
      topologyContent.includes("nodeViewMode === 'matrix'"),
      'OverviewTopologySection.vue must mount ClusterDensityMatrix when nodeViewMode === matrix'
    )
    assert.ok(
      topologyContent.includes('selectNode') || topologyContent.includes('select-node'),
      'OverviewTopologySection.vue must emit selectNode when cell clicked in matrix'
    )
  })

  it('2. VirtualNodeGrid.vue calculates responsive columns based on viewport/container width', () => {
    const content = fs.readFileSync(virtualGridPath, 'utf-8')
    assert.ok(
      content.includes('640') && content.includes('1024'),
      'VirtualNodeGrid.vue must compute responsive breakpoints for 640px and 1024px'
    )
    const lineCount = content.split('\n').length
    assert.ok(lineCount < 300, `VirtualNodeGrid.vue must stay < 300 lines (currently ${lineCount})`)
  })
})

describe('Hyperscale Phase 3: InfraHostsView & FleetView >1,000 Node Pagination / Windowing', () => {
  const hostsGridPath = path.join(rootDir, 'src/components/hosts/HostsGrid.vue')
  const hostsTablePath = path.join(rootDir, 'src/components/hosts/HostsTable.vue')
  const hostsMobilePath = path.join(rootDir, 'src/components/hosts/HostsMobileCards.vue')
  const fleetGridPath = path.join(rootDir, 'src/components/fleet/FleetClustersGrid.vue')

  it('1. HostsGrid, HostsTable, HostsMobileCards, and FleetClustersGrid bound page to 48 items with pagination controls', () => {
    const files = [
      { name: 'HostsGrid.vue', path: hostsGridPath },
      { name: 'HostsTable.vue', path: hostsTablePath },
      { name: 'HostsMobileCards.vue', path: hostsMobilePath },
      { name: 'FleetClustersGrid.vue', path: fleetGridPath }
    ]

    for (const f of files) {
      const content = fs.readFileSync(f.path, 'utf-8')
      assert.ok(
        content.includes('48'),
        `${f.name} must define page size of 48 items`
      )
      assert.ok(
        content.includes('Showing') && (content.includes('Prev') || content.includes('Previous')),
        `${f.name} must render pagination controls with Showing X-Y of N and Prev/Next`
      )
    }
  })
})

describe('Hyperscale Phase 3: Modularity Invariant — 100% of Components < 300 Lines & Views <= 350 Lines', () => {
  function getFiles(dir: string, ext: string): string[] {
    let results: string[] = []
    const list = fs.readdirSync(dir)
    for (const file of list) {
      const fullPath = path.join(dir, file)
      const stat = fs.statSync(fullPath)
      if (stat.isDirectory()) {
        results = results.concat(getFiles(fullPath, ext))
      } else if (file.endsWith(ext)) {
        results.push(fullPath)
      }
    }
    return results
  }

  it('1. ZERO .vue files in src/components/ exceed 300 lines', () => {
    const components = getFiles(path.join(rootDir, 'src/components'), '.vue')
    const overLimit: Array<{ file: string; lines: number }> = []

    for (const comp of components) {
      const lines = fs.readFileSync(comp, 'utf-8').split('\n').length
      if (lines >= 300) {
        overLimit.push({ file: path.relative(rootDir, comp), lines })
      }
    }

    assert.equal(
      overLimit.length,
      0,
      `All components in src/components must be < 300 lines. Found ${overLimit.length} violating files:\n${overLimit.map(o => ` - ${o.file}: ${o.lines} lines`).join('\n')}`
    )
  })

  it('2. ZERO .vue files in src/views/ exceed 350 lines', () => {
    const views = getFiles(path.join(rootDir, 'src/views'), '.vue')
    const overLimit: Array<{ file: string; lines: number }> = []

    for (const v of views) {
      const lines = fs.readFileSync(v, 'utf-8').split('\n').length
      if (lines > 350) {
        overLimit.push({ file: path.relative(rootDir, v), lines })
      }
    }

    assert.equal(
      overLimit.length,
      0,
      `All views in src/views must be <= 350 lines. Found ${overLimit.length} violating files:\n${overLimit.map(o => ` - ${o.file}: ${o.lines} lines`).join('\n')}`
    )
  })
})
