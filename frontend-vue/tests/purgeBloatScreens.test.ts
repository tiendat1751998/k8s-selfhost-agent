import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { navGroups, type NavGroup, type NavItem } from '../src/config/navigation.ts'

const PURGED_PATHS = [
  '/promotions',
  '/drift',
  '/compliance',
  '/backup',
  '/ai-hub',
  '/changes',
  '/reports',
  '/scaffolder',
  '/plugins',
  '/catalog',
  '/ecosystem',
  '/cost',
  '/capacity'
]

describe('Navigation Architecture & Bloat Screen Elimination Tests', () => {
  it('A. navigation.ts: navGroups strictly excludes all deprecated bloat items', () => {
    const allNavPaths = navGroups.flatMap(g => g.items.map(i => i.path))
    for (const purged of PURGED_PATHS) {
      assert.equal(
        allNavPaths.includes(purged),
        false,
        `navGroups must not contain purged route: ${purged}`
      )
    }
  })

  it('B. navigation.ts: maintains exactly 5 standardized enterprise functional groups', () => {
    const expectedGroups = [
      {
        key: 'observability',
        title: 'Observability & Incidents',
        items: ['/', '/incidents', '/slo', '/logs']
      },
      {
        key: 'compute',
        title: 'Compute & Workloads',
        items: ['/fleet', '/hosts', '/deployments', '/explorer', '/helm']
      },
      {
        key: 'governance',
        title: 'Governance & Security',
        items: ['/audit']
      },
      {
        key: 'automation',
        title: 'Automation & Operations',
        items: ['/automation', '/runbooks']
      },
      {
        key: 'management',
        title: 'Platform Management',
        items: ['/tenancy', '/alerts', '/settings']
      }
    ]

    assert.equal(navGroups.length, expectedGroups.length, 'Must have exactly 5 nav groups')
    for (let i = 0; i < expectedGroups.length; i++) {
      assert.equal(navGroups[i].key, expectedGroups[i].key)
      const currentPaths = navGroups[i].items.map(it => it.path)
      assert.deepEqual(currentPaths, expectedGroups[i].items)
    }
  })

  it('C. navigation item schema: all items have valid label, icon, path, and no missing fields', () => {
    for (const group of navGroups) {
      assert.ok(group.key && group.key.length > 0)
      assert.ok(group.label && group.label.length > 0)
      assert.ok(Array.isArray(group.items) && group.items.length > 0)

      for (const item of group.items) {
        assert.ok(item.path.startsWith('/'), `Path must start with slash: ${item.path}`)
        assert.ok(item.name && item.name.length > 0, `Item must have name: ${item.path}`)
        assert.ok(item.icon && item.icon.length > 0, `Item must have icon: ${item.path}`)
      }
    }
  })

  it('D. breadcrumb resolution: resolves authentic breadcrumbs for all active nav items', () => {
    function getBreadcrumb(path: string): { group: string; name: string } | null {
      for (const g of navGroups) {
        const item = g.items.find(i => i.path === path)
        if (item) return { group: g.label, name: item.name }
      }
      return null
    }

    const overviewBc = getBreadcrumb('/')
    assert.ok(overviewBc)
    assert.equal(overviewBc.group, 'OBSERVABILITY & OPS')
    assert.equal(overviewBc.name, 'Overview')

    const fleetBc = getBreadcrumb('/fleet')
    assert.ok(fleetBc)
    assert.equal(fleetBc.group, 'COMPUTE & FLEET')

    const runbooksBc = getBreadcrumb('/runbooks')
    assert.ok(runbooksBc)
    assert.equal(runbooksBc.group, 'AUTOMATION')

    assert.equal(getBreadcrumb('/promotions'), null, 'Purged route must have no active breadcrumb')
  })
})
