import { test, describe } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'

describe('Task 014: Enterprise StatusBadge Standardization Specifications', () => {
  const rootDir = path.resolve(import.meta.dirname, '..')
  const statusBadgePath = path.join(rootDir, 'src/components/ui/StatusBadge.vue')

  test('1. StatusBadge.vue exists and strictly obeys line count constraints (< 250 lines)', () => {
    assert.ok(fs.existsSync(statusBadgePath), 'StatusBadge.vue must exist')
    const content = fs.readFileSync(statusBadgePath, 'utf-8')
    const lineCount = content.split('\n').length
    assert.ok(lineCount < 250, `StatusBadge.vue must be strictly < 250 lines (currently ${lineCount})`)
  })

  test('2. Props definition supports status?, label?, pulse?, size?, class? with proper defaults', () => {
    const content = fs.readFileSync(statusBadgePath, 'utf-8')

    // Props contract
    assert.match(content, /status\?:?\s*string/, 'Must define optional status?: string')
    assert.match(content, /label\?:?\s*string/, 'Must define optional label?: string')
    assert.match(content, /pulse\?:?\s*boolean/, 'Must define optional pulse?: boolean')
    assert.match(content, /size\?:?\s*['"]sm['"]\s*\|\s*['"]md['"]/, "Must define optional size?: 'sm' | 'md'")
    assert.match(content, /class\?:?\s*string/, 'Must define optional class?: string')

    // Default size is sm
    assert.match(content, /size:\s*['"]sm['"]|size\s*===\s*['"]md['"]\s*\?\s*['"]badge-md['"]\s*:\s*['"]badge-sm['"]/, 'Default size must be sm')
  })

  test('3. State mapping adheres to Kubernetes state categories', () => {
    const content = fs.readFileSync(statusBadgePath, 'utf-8')

    // Emerald states: Ready, Healthy, Active, Running
    assert.match(content, /['"]ready['"]/, 'Emerald must include ready')
    assert.match(content, /['"]healthy['"]/, 'Emerald must include healthy')
    assert.match(content, /['"]active['"]/, 'Emerald must include active')
    assert.match(content, /['"]running['"]/, 'Emerald must include running')

    // Amber states: Pending, Warning, Degraded, Elevated
    assert.match(content, /['"]pending['"]/, 'Amber must include pending')
    assert.match(content, /['"]warning['"]/, 'Amber must include warning')
    assert.match(content, /['"]degraded['"]/, 'Amber must include degraded')
    assert.match(content, /['"]elevated['"]/, 'Amber must include elevated')

    // Rose states: Failed, Critical, Offline, Error
    assert.match(content, /['"]failed['"]/, 'Rose must include failed')
    assert.match(content, /['"]critical['"]/, 'Rose must include critical')
    assert.match(content, /['"]offline['"]/, 'Rose must include offline')
    assert.match(content, /['"]error['"]/, 'Rose must include error')

    // Slate states: Cordoned, Draining, Terminating, Unknown
    assert.match(content, /['"]cordoned['"]/, 'Slate must include cordoned')
    assert.match(content, /['"]draining['"]/, 'Slate must include draining')
    assert.match(content, /['"]terminating['"]/, 'Slate must include terminating')
    assert.match(content, /['"]unknown['"]/, 'Slate must include unknown')
  })

  test('4. Dynamic pulse logic activates for active/running/ready states or explicit pulse prop', () => {
    const content = fs.readFileSync(statusBadgePath, 'utf-8')

    // Check pulse logic
    assert.match(content, /pulse/i, 'Must contain pulse logic')
    assert.match(
      content,
      /ACTIVE_PULSE_STATUSES|['"]ready['"]|['"]active['"]|['"]running['"]/,
      'Pulse logic must account for active/running/ready states'
    )
    assert.match(
      content,
      /props\.pulse|typeof\s+props\.pulse|props\.pulse\s*!==\s*undefined/,
      'Must respect explicit pulse boolean prop override'
    )
  })

  test('5. Label defaults to capitalized status when label prop is omitted', () => {
    const content = fs.readFileSync(statusBadgePath, 'utf-8')

    // Capitalization of status for displayLabel
    assert.match(
      content,
      /toUpperCase\(\)|charAt\(0\)\.toUpperCase\(\)/,
      'Must capitalize status string for display text when label is not provided'
    )
  })

  test('6. Rancher/Headlamp/Datadog styling standards: dot, pulse, typography, and translucency', () => {
    const content = fs.readFileSync(statusBadgePath, 'utf-8')

    // 6px circular dot indicator
    assert.match(content, /width:\s*6px/, 'Dot width must be 6px')
    assert.match(content, /height:\s*6px/, 'Dot height must be 6px')
    assert.match(content, /border-radius:\s*(?:50%|9999px)/, 'Dot must be circular')

    // Pulse keyframes
    assert.match(content, /@keyframes\s+status-pulse/, 'Must declare @keyframes status-pulse')
    assert.match(content, /\.dot-pulse/, 'Must define .dot-pulse animation class')

    // Strict typography: Inter font, 11px uppercase, letter-spacing: 0.04em, font-weight: 600
    assert.match(content, /font-family:[^;]*Inter/i, 'Must specify Inter font-family')
    assert.match(content, /letter-spacing:\s*0\.04em/i, 'Must specify letter-spacing: 0.04em')
    assert.match(content, /font-weight:\s*600/i, 'Must specify font-weight: 600')
    assert.match(content, /text-transform:\s*uppercase/i, 'Must specify text-transform: uppercase')

    // Translucent background (10-14% opacity) and 1px subtle border:
    // Emerald: rgba(16, 185, 129, 0.12), border rgba(16, 185, 129, 0.25)
    assert.match(content, /rgba\(16,\s*185,\s*129,\s*0\.12\)/, 'Emerald background must be rgba(16, 185, 129, 0.12)')
    assert.match(content, /rgba\(16,\s*185,\s*129,\s*0\.25\)/, 'Emerald border must be rgba(16, 185, 129, 0.25)')

    // Amber: rgba(245, 158, 11, 0.12), border rgba(245, 158, 11, 0.25)
    assert.match(content, /rgba\(245,\s*158,\s*11,\s*0\.12\)/, 'Amber background must be rgba(245, 158, 11, 0.12)')
    assert.match(content, /rgba\(245,\s*158,\s*11,\s*0\.25\)/, 'Amber border must be rgba(245, 158, 11, 0.25)')

    // Rose: rgba(244, 63, 94, 0.12), border rgba(244, 63, 94, 0.25)
    assert.match(content, /rgba\(244,\s*63,\s*94,\s*0\.12\)/, 'Rose background must be rgba(244, 63, 94, 0.12)')
    assert.match(content, /rgba\(244,\s*63,\s*94,\s*0\.25\)/, 'Rose border must be rgba(244, 63, 94, 0.25)')

    // Slate: rgba(148, 163, 184, 0.12), border rgba(148, 163, 184, 0.25)
    assert.match(content, /rgba\(148,\s*163,\s*184,\s*0\.12\)/, 'Slate background must be rgba(148, 163, 184, 0.12)')
    assert.match(content, /rgba\(148,\s*163,\s*184,\s*0\.25\)/, 'Slate border must be rgba(148, 163, 184, 0.25)')
  })
})
