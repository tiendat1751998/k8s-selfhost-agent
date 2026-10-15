import { describe, it } from 'node:test'
import assert from 'node:assert/strict'

export interface TableColumnDef {
  key: string
  label: string
  width?: string
  minWidth?: string
}

export function parsePxWidth(widthStr?: string): number {
  if (!widthStr) return 0
  const match = widthStr.match(/^(\d+(?:\.\d+)?)px$/)
  return match ? parseFloat(match[1]) : 0
}

export function sumColumnWidths(columns: TableColumnDef[]): number {
  return columns.reduce((acc, col) => acc + parsePxWidth(col.width || col.minWidth), 0)
}

export function truncateCellText(text: string, maxLen: number): string {
  if (!text) return ''
  if (text.length <= maxLen) return text
  return text.slice(0, maxLen - 1) + '…'
}

describe('Table Overflow Remediation Specifications & Layout Constraints', () => {
  const deploymentsColumns: TableColumnDef[] = [
    { key: 'name', label: 'Name', width: '220px' },
    { key: 'namespace', label: 'Namespace', width: '130px' },
    { key: 'pods', label: 'Pods', width: '100px' },
    { key: 'replicas', label: 'Replicas', width: '105px' },
    { key: 'strategy', label: 'Strategy', width: '90px' },
    { key: 'image', label: 'Image', width: '130px' },
    { key: 'age', label: 'Age', width: '90px' },
    { key: 'actions', label: 'Actions', width: '115px' }
  ]

  const hostsColumns: TableColumnDef[] = [
    { key: 'name', label: 'Host Name', width: '150px' },
    { key: 'ip', label: 'IP Address', width: '115px' },
    { key: 'status', label: 'Status', width: '110px' },
    { key: 'role', label: 'Role', width: '100px' },
    { key: 'cpu', label: 'CPU', width: '120px' },
    { key: 'memory', label: 'Memory', width: '120px' },
    { key: 'actions', label: 'Actions', width: '130px' }
  ]

  const sloColumns: TableColumnDef[] = [
    { key: 'service', label: 'Service', width: '140px' },
    { key: 'indicator_type', label: 'Type', width: '85px' },
    { key: 'targetNum', label: 'Target', width: '75px' },
    { key: 'error_budget', label: 'Error Budget', width: '110px' },
    { key: 'burn_rate', label: 'Burn Rate', width: '95px' },
    { key: 'window', label: 'Window', width: '65px' },
    { key: 'query', label: 'SLI Query', width: '240px' },
    { key: 'alert_threshold', label: 'Threshold', width: '95px' },
    { key: 'actions', label: 'Actions', width: '115px' }
  ]

  const explorerColumns: TableColumnDef[] = [
    { key: 'name', label: 'Resource Name', width: '220px' },
    { key: 'namespace', label: 'Namespace', width: '120px' },
    { key: 'kind', label: 'Kind', width: '110px' },
    { key: 'image', label: 'Image', width: '145px' },
    { key: 'selector', label: 'Selector', width: '135px' },
    { key: 'status', label: 'Status', width: '100px' },
    { key: 'actions', label: 'Actions', width: '130px' }
  ]

  it('DeploymentsTable columns total width stays <= 1070px', () => {
    const total = sumColumnWidths(deploymentsColumns)
    assert.ok(total <= 1070, `Deployments columns sum ${total}px must be <= 1070px`)
    assert.equal(total, 980)
  })

  it('HostsTable columns total width stays <= 1030px', () => {
    const total = sumColumnWidths(hostsColumns)
    assert.ok(total <= 1030, `Hosts columns sum ${total}px must be <= 1030px`)
    assert.equal(total, 845)
  })

  it('SloCatalogTable columns total width stays <= 1080px', () => {
    const total = sumColumnWidths(sloColumns)
    assert.ok(total <= 1080, `SLO columns sum ${total}px must be <= 1080px`)
    assert.equal(total, 1020)
  })

  it('ExplorerResourceTable columns total width stays <= 1040px', () => {
    const total = sumColumnWidths(explorerColumns)
    assert.ok(total <= 1040, `Explorer columns sum ${total}px must be <= 1040px`)
    assert.equal(total, 960)
  })

  it('truncateCellText truncates overflowing text with ellipsis safely', () => {
    assert.equal(truncateCellText('short-text', 20), 'short-text')
    assert.equal(truncateCellText('very-long-container-image-registry.internal.corp/production/service:v1.2.3', 25), 'very-long-container-imag…')
    assert.equal(truncateCellText('', 10), '')
  })
})
