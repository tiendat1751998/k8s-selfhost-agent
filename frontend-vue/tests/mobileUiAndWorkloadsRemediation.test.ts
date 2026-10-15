import { describe, it } from 'node:test'
import assert from 'node:assert/strict'

export interface CommandPaletteItem {
  id: string
  title: string
  category: 'Navigation' | 'Workloads' | 'Cluster' | 'Actions'
  shortcut?: string
  keywords?: string[]
}

export function groupCommandsByCategory(
  commands: CommandPaletteItem[],
  searchQuery: string = ''
): Array<{ category: string; items: CommandPaletteItem[] }> {
  const q = searchQuery.trim().toLowerCase()
  const filtered = q
    ? commands.filter(item => {
        const inTitle = item.title.toLowerCase().includes(q)
        const inCategory = item.category.toLowerCase().includes(q)
        const inKeywords = (item.keywords || []).some(k => k.toLowerCase().includes(q))
        return inTitle || inCategory || inKeywords
      })
    : commands

  const groupMap = new Map<string, CommandPaletteItem[]>()
  const categoryOrder: Array<CommandPaletteItem['category']> = ['Navigation', 'Workloads', 'Cluster', 'Actions']

  for (const cat of categoryOrder) {
    groupMap.set(cat, [])
  }

  for (const item of filtered) {
    const list = groupMap.get(item.category) || []
    list.push(item)
    groupMap.set(item.category, list)
  }

  const result: Array<{ category: string; items: CommandPaletteItem[] }> = []
  for (const [category, items] of groupMap.entries()) {
    if (items.length > 0) {
      result.push({ category, items })
    }
  }

  return result
}

export function resolvePwaBannerTitle(viewportWidth: number): string {
  return viewportWidth <= 640 ? 'Install App' : 'Install K8s Control Platform'
}

export function formatWorkloadEmptyStateMessage(filterCount: number): string {
  if (filterCount === 0) {
    return '0 Workloads matching active filters'
  }
  return ''
}

describe('Mobile UI Defect Remediation & Workload Stream', () => {
  const sampleCommands: CommandPaletteItem[] = [
    { id: 'nav-overview', title: 'Go to Overview', category: 'Navigation', keywords: ['dashboard', 'home'] },
    { id: 'nav-deployments', title: 'Go to Deployments', category: 'Navigation', keywords: ['apps', 'workloads'] },
    { id: 'act-restart', title: 'Restart Deployment', category: 'Actions', keywords: ['reboot', 'bounce'] },
    { id: 'act-scale', title: 'Scale Replicas', category: 'Actions', keywords: ['hpa', 'resize'] },
    { id: 'cluster-drain', title: 'Drain Node', category: 'Cluster', keywords: ['maintenance', 'cordon'] }
  ]

  it('groupCommandsByCategory groups commands preserving defined category ordering', () => {
    const groups = groupCommandsByCategory(sampleCommands)
    assert.equal(groups.length, 3)
    assert.equal(groups[0].category, 'Navigation')
    assert.equal(groups[0].items.length, 2)
    assert.equal(groups[1].category, 'Cluster')
    assert.equal(groups[1].items.length, 1)
    assert.equal(groups[2].category, 'Actions')
    assert.equal(groups[2].items.length, 2)
  })

  it('groupCommandsByCategory filters commands by title, category, and keywords', () => {
    const byKeyword = groupCommandsByCategory(sampleCommands, 'bounce')
    assert.equal(byKeyword.length, 1)
    assert.equal(byKeyword[0].items[0].id, 'act-restart')

    const byCategory = groupCommandsByCategory(sampleCommands, 'Actions')
    assert.equal(byCategory.length, 1)
    assert.equal(byCategory[0].items.length, 2)

    const noMatch = groupCommandsByCategory(sampleCommands, 'nonexistent')
    assert.equal(noMatch.length, 0)
  })

  it('resolvePwaBannerTitle adapts title to compact length on mobile viewports', () => {
    assert.equal(resolvePwaBannerTitle(375), 'Install App')
    assert.equal(resolvePwaBannerTitle(640), 'Install App')
    assert.equal(resolvePwaBannerTitle(768), 'Install K8s Control Platform')
    assert.equal(resolvePwaBannerTitle(1440), 'Install K8s Control Platform')
  })

  it('formatWorkloadEmptyStateMessage generates authentic empty state string', () => {
    assert.equal(formatWorkloadEmptyStateMessage(0), '0 Workloads matching active filters')
    assert.equal(formatWorkloadEmptyStateMessage(5), '')
  })
})
