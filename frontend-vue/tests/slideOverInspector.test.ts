import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const rootDir = path.resolve(__dirname, '..')

describe('Task 016: Unified Slide-Over Contextual Inspector Drawer Specification', () => {
  const inspectorPath = path.join(rootDir, 'src', 'components', 'ui', 'SlideOverInspector.vue')

  it('1. SlideOverInspector.vue exists and strictly obeys line count constraints (< 400 lines)', () => {
    assert.ok(fs.existsSync(inspectorPath), 'SlideOverInspector.vue must exist')
    const content = fs.readFileSync(inspectorPath, 'utf-8')
    const lineCount = content.split('\n').length
    assert.ok(
      lineCount < 400,
      `SlideOverInspector.vue must be strictly < 400 lines (currently ${lineCount} lines)`
    )
  })

  it('2. Props contract supports isOpen, title, subtitle, status, icon, tabs, activeTab, and width with defaults', () => {
    const content = fs.readFileSync(inspectorPath, 'utf-8')

    // Props definitions
    assert.match(content, /isOpen:\s*boolean/, 'Must define isOpen: boolean prop')
    assert.match(content, /title:\s*string/, 'Must define title: string prop')
    assert.match(content, /subtitle\?:\s*string/, 'Must define optional subtitle?: string prop')
    assert.match(content, /status\?:\s*string/, 'Must define optional status?: string prop')
    assert.match(content, /icon\?:\s*string/, 'Must define optional icon?: string prop')
    assert.match(content, /tabs\?:/, 'Must define optional tabs?: InspectorTab[] prop')
    assert.match(content, /activeTab\?:\s*string/, 'Must define optional activeTab?: string prop')
    assert.match(content, /width\?:\s*string/, 'Must define optional width?: string prop')
    assert.match(content, /closeOnBackdrop\?:\s*boolean/, 'Must define optional closeOnBackdrop?: boolean prop')
    assert.match(content, /closeOnEsc\?:\s*boolean/, 'Must define optional closeOnEsc?: boolean prop')

    // Defaults check
    assert.match(content, /580px/, 'Must default width to 580px')
  })

  it('3. Emits contract defines close, update:isOpen, update:activeTab, and action', () => {
    const content = fs.readFileSync(inspectorPath, 'utf-8')

    assert.match(content, /['"]close['"]/, 'Must emit close')
    assert.match(content, /['"]update:isOpen['"]/, 'Must emit update:isOpen')
    assert.match(content, /['"]update:activeTab['"]/, 'Must emit update:activeTab')
    assert.match(content, /['"]action['"]/, 'Must emit action')
  })

  it('4. Uses Teleport to body for zero table re-renders and smooth slide animation', () => {
    const content = fs.readFileSync(inspectorPath, 'utf-8')

    assert.match(content, /<Teleport\s+to=["']body["']>/, 'Must teleport drawer to body')
    assert.match(content, /transform:\s*translateX\(100%\)/, 'Must slide from right using translateX(100%)')
    assert.match(content, /transform:\s*translateX\(0\)/, 'Must slide to translateX(0) when open')
    assert.match(content, /backdrop-filter:\s*blur\(4px\)|blur\(8px\)/, 'Must provide backdrop blur')
  })

  it('5. Mechanical sympathy: 580px fixed width on desktop and 100vw on mobile (< 640px)', () => {
    const content = fs.readFileSync(inspectorPath, 'utf-8')

    // Desktop width 580px
    assert.match(content, /580px/, 'Desktop width must be 580px')
    // Fixed 48px header
    assert.match(content, /height:\s*48px|min-height:\s*48px/, 'Header must be fixed 48px height')
    // Mobile breakpoint
    assert.match(content, /@media\s*\(\s*max-width:\s*640px\s*\)/, 'Must have @media (max-width: 640px) rule')
    assert.match(content, /100vw/, 'Mobile width must be 100vw')
  })

  it('6. Integrates StatusBadge and BaseIcon cleanly with zero placeholder mocks', () => {
    const content = fs.readFileSync(inspectorPath, 'utf-8')

    assert.match(content, /import\s+StatusBadge\s+from/, 'Must import StatusBadge')
    assert.match(content, /<StatusBadge/, 'Must render <StatusBadge /> for status indicator')
    assert.match(content, /import\s+BaseIcon\s+from/, 'Must import BaseIcon')
    assert.match(content, /<BaseIcon/, 'Must render <BaseIcon /> for icons')
    assert.match(content, /x-circle|['"]x['"]/, 'Must use close icon in header')
  })

  it('7. Tab navigation strip supports default Overview, YAML, Logs, Actions and custom tabs', () => {
    const content = fs.readFileSync(inspectorPath, 'utf-8')

    // Default tabs
    assert.match(content, /overview/i, 'Must support overview tab')
    assert.match(content, /yaml/i, 'Must support yaml tab')
    assert.match(content, /logs/i, 'Must support logs tab')
    assert.match(content, /actions/i, 'Must support actions tab')

    // Tab slots
    assert.match(content, /<slot\s+:name=["']currentTabId["']|<slot\s+:name=["']currentTab["']|<slot\s+:name=["']activeTab["']|<slot\s+:name=/, 'Must support dynamic named tab slots')
    assert.match(content, /<slot\s*\/?>|<slot>/, 'Must provide default slot fallback')

    // ARIA roles
    assert.match(content, /role=["']tablist["']/, 'Must define role="tablist" on tab container')
    assert.match(content, /role=["']tab["']/, 'Must define role="tab" on tab buttons')
    assert.match(content, /aria-selected/, 'Must define aria-selected on tab buttons')
  })

  it('8. Keyboard and accessibility handling with Escape dismiss, scroll lock, and cleanup', () => {
    const content = fs.readFileSync(inspectorPath, 'utf-8')

    assert.match(content, /['"]Escape['"]|key\s*===\s*['"]Escape['"]/, 'Must listen for Escape key')
    assert.match(content, /document\.body\.style\.overflow/, 'Must lock body scroll when open')
    assert.match(content, /onUnmounted/, 'Must clean up listeners and styles in onUnmounted')
    assert.match(content, /role=["']dialog["']/, 'Must define role="dialog"')
    assert.match(content, /aria-modal=["']true["']/, 'Must define aria-modal="true"')
  })
})
