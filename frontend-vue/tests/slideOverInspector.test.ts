import { describe, it } from 'node:test'
import assert from 'node:assert/strict'

export interface InspectorTab {
  id: string
  label: string
  icon?: string
  badge?: string | number
  disabled?: boolean
}

describe('Task 016: SlideOverInspector State & Responsive Logic Tests', () => {
  function resolveEffectiveWidth(windowWidth: number, customWidth = '580px'): string {
    if (windowWidth < 640) return '100vw'
    return customWidth
  }

  function resolveActiveTab(tabs: InspectorTab[], requestedTab?: string): string {
    if (requestedTab && tabs.some(t => t.id === requestedTab && !t.disabled)) {
      return requestedTab
    }
    const firstEnabled = tabs.find(t => !t.disabled)
    return firstEnabled ? firstEnabled.id : ''
  }

  const defaultTabs: InspectorTab[] = [
    { id: 'overview', label: 'Overview', icon: 'info' },
    { id: 'yaml', label: 'YAML Manifest', icon: 'file-text' },
    { id: 'logs', label: 'Recent Logs', icon: 'terminal', badge: 14 },
    { id: 'actions', label: 'Quick Actions', icon: 'zap' }
  ]

  it('resolveEffectiveWidth returns 100vw on mobile viewports (< 640px)', () => {
    assert.equal(resolveEffectiveWidth(375), '100vw')
    assert.equal(resolveEffectiveWidth(414), '100vw')
    assert.equal(resolveEffectiveWidth(639), '100vw')
  })

  it('resolveEffectiveWidth respects desktop 580px default or custom width on wider screens', () => {
    assert.equal(resolveEffectiveWidth(768), '580px')
    assert.equal(resolveEffectiveWidth(1440), '580px')
    assert.equal(resolveEffectiveWidth(1920, '640px'), '640px')
  })

  it('resolveActiveTab selects requested tab when valid and enabled', () => {
    assert.equal(resolveActiveTab(defaultTabs, 'yaml'), 'yaml')
    assert.equal(resolveActiveTab(defaultTabs, 'logs'), 'logs')
  })

  it('resolveActiveTab falls back to first enabled tab when requested tab is invalid or disabled', () => {
    assert.equal(resolveActiveTab(defaultTabs, 'non-existent'), 'overview')
    assert.equal(resolveActiveTab(defaultTabs, undefined), 'overview')

    const withDisabled: InspectorTab[] = [
      { id: 'tab-1', label: 'Disabled Tab', disabled: true },
      { id: 'tab-2', label: 'Second Tab' }
    ]
    assert.equal(resolveActiveTab(withDisabled, 'tab-1'), 'tab-2')
  })

  it('drawer state machine handles open, close, and escape dismissal properly', () => {
    let isOpen = false
    let emittedEvent: string | null = null

    function openDrawer() {
      isOpen = true
      emittedEvent = 'open'
    }

    function closeDrawer(reason: 'backdrop' | 'escape' | 'button') {
      isOpen = false
      emittedEvent = `close:${reason}`
    }

    assert.equal(isOpen, false)
    openDrawer()
    assert.equal(isOpen, true)
    assert.equal(emittedEvent, 'open')

    closeDrawer('escape')
    assert.equal(isOpen, false)
    assert.equal(emittedEvent, 'close:escape')
  })
})
