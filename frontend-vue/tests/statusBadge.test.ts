import { describe, it } from 'node:test'
import assert from 'node:assert/strict'

type BadgeColor = 'emerald' | 'amber' | 'rose' | 'slate'

function mapStatusToColor(status?: string): BadgeColor {
  const s = (status || 'unknown').toLowerCase().trim()
  if (['ready', 'healthy', 'active', 'running', 'connected', 'synced'].includes(s)) {
    return 'emerald'
  }
  if (['pending', 'warning', 'degraded', 'elevated', 'progressing'].includes(s)) {
    return 'amber'
  }
  if (['failed', 'critical', 'offline', 'error', 'crashed'].includes(s)) {
    return 'rose'
  }
  return 'slate' // cordoned, draining, terminating, unknown, etc.
}

function resolveBadgePulse(status?: string, explicitPulse?: boolean): boolean {
  if (explicitPulse !== undefined) return explicitPulse
  const color = mapStatusToColor(status)
  return color === 'emerald' || color === 'rose'
}

function formatBadgeLabel(status?: string, customLabel?: string): string {
  if (customLabel && customLabel.trim()) return customLabel.trim()
  if (!status || !status.trim()) return 'Unknown'
  const clean = status.trim().replace(/[_-]/g, ' ')
  return clean.charAt(0).toUpperCase() + clean.slice(1)
}

describe('Task 014: StatusBadge Kubernetes State Mapping & Label Tests', () => {
  it('mapStatusToColor correctly maps Kubernetes active states to emerald', () => {
    assert.equal(mapStatusToColor('ready'), 'emerald')
    assert.equal(mapStatusToColor('healthy'), 'emerald')
    assert.equal(mapStatusToColor('active'), 'emerald')
    assert.equal(mapStatusToColor('running'), 'emerald')
    assert.equal(mapStatusToColor('RUNNING'), 'emerald')
  })

  it('mapStatusToColor correctly maps warning states to amber', () => {
    assert.equal(mapStatusToColor('pending'), 'amber')
    assert.equal(mapStatusToColor('warning'), 'amber')
    assert.equal(mapStatusToColor('degraded'), 'amber')
    assert.equal(mapStatusToColor('elevated'), 'amber')
  })

  it('mapStatusToColor correctly maps error and critical states to rose', () => {
    assert.equal(mapStatusToColor('failed'), 'rose')
    assert.equal(mapStatusToColor('critical'), 'rose')
    assert.equal(mapStatusToColor('offline'), 'rose')
    assert.equal(mapStatusToColor('error'), 'rose')
    assert.equal(mapStatusToColor('crashed'), 'rose')
  })

  it('mapStatusToColor defaults cordoned, draining, terminating, and unknown to slate', () => {
    assert.equal(mapStatusToColor('cordoned'), 'slate')
    assert.equal(mapStatusToColor('draining'), 'slate')
    assert.equal(mapStatusToColor('terminating'), 'slate')
    assert.equal(mapStatusToColor('unknown'), 'slate')
    assert.equal(mapStatusToColor(''), 'slate')
    assert.equal(mapStatusToColor(undefined), 'slate')
  })

  it('resolveBadgePulse activates pulse for active/critical states or explicit pulse override', () => {
    assert.equal(resolveBadgePulse('running'), true)
    assert.equal(resolveBadgePulse('critical'), true)
    assert.equal(resolveBadgePulse('pending'), false)
    assert.equal(resolveBadgePulse('pending', true), true, 'Explicit override takes precedence')
    assert.equal(resolveBadgePulse('running', false), false, 'Explicit override can disable pulse')
  })

  it('formatBadgeLabel formats status with proper capitalization when label is omitted', () => {
    assert.equal(formatBadgeLabel('running'), 'Running')
    assert.equal(formatBadgeLabel('crash_loop_back_off'), 'Crash loop back off')
    assert.equal(formatBadgeLabel(undefined), 'Unknown')
    assert.equal(formatBadgeLabel('active', 'Custom Override'), 'Custom Override')
  })
})
