import { describe, it } from 'node:test'
import assert from 'node:assert/strict'

export interface ActionItem {
  id?: string
  action?: string
  label: string
  icon?: string
  variant?: 'default' | 'danger' | 'warning'
  isDestructive?: boolean
  disabled?: boolean
  separator?: boolean
  tooltip?: string
}

describe('Task 011: ActionDropdown Logic & Keyboard Navigation Tests', () => {
  function getSelectableIndices(items: ActionItem[]): number[] {
    return items
      .map((item, idx) => (!item.separator && !item.disabled ? idx : -1))
      .filter((idx) => idx !== -1)
  }

  function getNextSelectableIndex(currentIndex: number, direction: 'next' | 'prev', items: ActionItem[]): number {
    const selectable = getSelectableIndices(items)
    if (selectable.length === 0) return -1
    const currentPos = selectable.indexOf(currentIndex)
    if (direction === 'next') {
      return currentPos === -1 || currentPos === selectable.length - 1
        ? selectable[0]
        : selectable[currentPos + 1]
    } else {
      return currentPos <= 0
        ? selectable[selectable.length - 1]
        : selectable[currentPos - 1]
    }
  }

  function calculateDropdownPosition(
    rect: { top: number; bottom: number; left: number; right: number },
    viewport: { innerWidth: number; innerHeight: number },
    menuSize: { width: number; height: number },
    align: 'left' | 'right' = 'right'
  ): { top: number; left: number } {
    let top = rect.bottom + 4
    if (viewport.innerHeight - rect.bottom < menuSize.height && rect.top > viewport.innerHeight - rect.bottom) {
      top = Math.max(8, rect.top - menuSize.height - 4)
    }
    let left = align === 'left' ? rect.left : rect.right - menuSize.width
    if (left + menuSize.width > viewport.innerWidth - 8) {
      left = viewport.innerWidth - menuSize.width - 8
    }
    if (left < 8) left = 8
    return { top: Math.round(top), left: Math.round(left) }
  }

  const standardClusterActions: ActionItem[] = [
    { action: 'inspect', label: 'Inspect Cluster', icon: 'eye' },
    { action: 'probe', label: 'Execute Health Probe', icon: 'activity' },
    { action: 'rebalance', label: 'Rebalance Workloads', icon: 'refresh-cw' },
    { separator: true, label: '' },
    { action: 'cordon', label: 'Cordon & Drain Node', icon: 'shield-off', variant: 'warning' },
    { action: 'evict', label: 'Evict Cluster Pods', icon: 'trash-2', variant: 'danger', isDestructive: true }
  ]

  it('getSelectableIndices skips separators and disabled items', () => {
    const indices = getSelectableIndices(standardClusterActions)
    // indices 0 (inspect), 1 (probe), 2 (rebalance), 4 (cordon), 5 (evict)
    assert.deepEqual(indices, [0, 1, 2, 4, 5])
    assert.equal(indices.includes(3), false, 'Separator at index 3 must not be selectable')
  })

  it('getNextSelectableIndex navigates forward and wraps to start', () => {
    assert.equal(getNextSelectableIndex(-1, 'next', standardClusterActions), 0)
    assert.equal(getNextSelectableIndex(0, 'next', standardClusterActions), 1)
    assert.equal(getNextSelectableIndex(2, 'next', standardClusterActions), 4, 'Skips separator at index 3')
    assert.equal(getNextSelectableIndex(5, 'next', standardClusterActions), 0, 'Wraps to first item')
  })

  it('getNextSelectableIndex navigates backward and wraps to end', () => {
    assert.equal(getNextSelectableIndex(0, 'prev', standardClusterActions), 5, 'Wraps to last item')
    assert.equal(getNextSelectableIndex(4, 'prev', standardClusterActions), 2, 'Skips separator going backwards')
  })

  it('calculateDropdownPosition calculates viewport-safe top and left coordinates', () => {
    // Normal case with plenty of viewport room
    const rect = { top: 100, bottom: 130, left: 500, right: 540 }
    const viewport = { innerWidth: 1920, innerHeight: 1080 }
    const menuSize = { width: 170, height: 140 }

    const pos = calculateDropdownPosition(rect, viewport, menuSize, 'right')
    assert.equal(pos.top, 134)
    assert.equal(pos.left, 540 - 170) // 370
  })

  it('calculateDropdownPosition flips upward when near viewport bottom boundary', () => {
    // Near bottom (bottom at 1050, viewport height 1080 -> only 30px remaining)
    const rect = { top: 1020, bottom: 1050, left: 500, right: 540 }
    const viewport = { innerWidth: 1920, innerHeight: 1080 }
    const menuSize = { width: 170, height: 140 }

    const pos = calculateDropdownPosition(rect, viewport, menuSize, 'right')
    // Flips above button: top <= rect.top - height - 4
    assert.ok(pos.top < rect.top)
    assert.equal(pos.top, 1020 - 140 - 4)
  })

  it('standardClusterActions defines required enterprise actions with destructive tagging', () => {
    const evictAction = standardClusterActions.find(a => a.action === 'evict')
    assert.ok(evictAction)
    assert.equal(evictAction.isDestructive, true)
    assert.equal(evictAction.variant, 'danger')

    const inspectAction = standardClusterActions.find(a => a.action === 'inspect')
    assert.ok(inspectAction)
    assert.equal(inspectAction.isDestructive, undefined)
  })
})
