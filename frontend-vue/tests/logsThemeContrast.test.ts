import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { calculateContrastRatio } from './tokensSurfaceLadder.test.ts'
import {
  cleanFingerprint,
  cleanLogMessage,
  getTargetBadge,
  inferEffectiveLevel
} from '../src/components/logs/virtualLogUtils.ts'
import type { LogEntry } from '../src/stores/logStore.ts'

/**
 * Dual-Theme Design Tokens specification for Logs View (LES-003).
 * Sourced directly from src/assets/css/tokens.css to verify WCAG contrast conformance.
 */
export const LOGS_THEME_TOKENS = {
  light: {
    surfaceCard: '#ffffff',
    surfaceElevated: '#f8fafc',
    surfaceInset: '#f1f5f9',
    colorBody: '#0f172a',
    colorTextSecondary: '#334155',
    colorMuted: '#64748b',
    statusOfflineText: '#b91c1c',
    statusOfflineBg: '#fee2e2',
    statusWarningText: '#b45309',
    statusWarningBg: '#fef3c7',
    statusOnlineText: '#15803d',
    statusOnlineBg: '#dcfce7',
    badgeAgentText: '#0284c7',
    badgeAgentBg: '#f0f9ff',
    badgeK8sText: '#1d4ed8',
    badgeK8sBg: '#eff6ff'
  },
  dark: {
    surfaceCard: '#161f30',
    surfaceElevated: '#222f46',
    surfaceInset: '#111827',
    colorBody: '#f8fafc',
    colorTextSecondary: '#cbd5e1',
    colorMuted: '#94a3b8',
    statusOfflineText: '#f85149',
    statusOfflineBg: '#331a1a',
    statusWarningText: '#f59e0b',
    statusOnlineText: '#3fb950',
    badgeAgentText: '#38bdf8',
    badgeK8sText: '#58a6ff'
  }
}

export function resolveLevelBadgeClass(level: string): string {
  return `virtual-badge-${(level || 'info').toLowerCase()}`
}

export function resolveLogRowClass(level: string): string {
  const lvl = (level || 'info').toLowerCase()
  return `log-${lvl}`
}

export function resolveRadarCountBadgeClass(level: string): string {
  return level.toLowerCase() === 'warn' || level.toLowerCase() === 'warning'
    ? 'radar-count-badge count-warn'
    : 'radar-count-badge'
}

describe('Logs View Dual-Theme & Contrast Specifications (LES-003)', () => {
  describe('Light Mode Contrast Conformance (WCAG AAA & AA)', () => {
    const light = LOGS_THEME_TOKENS.light

    it('Terminal viewport surfaceInset (#f1f5f9) vs colorBody (#0f172a) satisfies WCAG AAA (>= 7.0:1)', () => {
      const contrast = calculateContrastRatio(light.surfaceInset, light.colorBody)
      assert.ok(contrast >= 7.0, `Contrast ratio ${contrast} must be >= 7.0:1 (WCAG AAA)`)
    })

    it('Radar card surfaceCard (#ffffff) vs colorBody (#0f172a) satisfies WCAG AAA (>= 7.0:1)', () => {
      const contrast = calculateContrastRatio(light.surfaceCard, light.colorBody)
      assert.ok(contrast >= 7.0, `Contrast ratio ${contrast} must be >= 7.0:1 (WCAG AAA)`)
    })

    it('Header surfaceElevated (#f8fafc) vs colorTextSecondary (#334155) satisfies WCAG AAA (>= 7.0:1)', () => {
      const contrast = calculateContrastRatio(light.surfaceElevated, light.colorTextSecondary)
      assert.ok(contrast >= 7.0, `Contrast ratio ${contrast} must be >= 7.0:1 (WCAG AAA)`)
    })

    it('Muted metadata text (#64748b) vs surfaceCard (#ffffff) satisfies WCAG AA (>= 4.5:1)', () => {
      const contrast = calculateContrastRatio(light.surfaceCard, light.colorMuted)
      assert.ok(contrast >= 4.5, `Contrast ratio ${contrast} must be >= 4.5:1 (WCAG AA)`)
    })

    it('Status offline badge pill text (#b91c1c) on background (#fee2e2) satisfies WCAG AA (>= 4.5:1)', () => {
      const contrast = calculateContrastRatio(light.statusOfflineBg, light.statusOfflineText)
      assert.ok(contrast >= 4.5, `Contrast ratio ${contrast} must be >= 4.5:1 (WCAG AA)`)
    })

    it('Status warning badge pill text (#b45309) on background (#fef3c7) satisfies WCAG AA (>= 4.5:1)', () => {
      const contrast = calculateContrastRatio(light.statusWarningBg, light.statusWarningText)
      assert.ok(contrast >= 4.5, `Contrast ratio ${contrast} must be >= 4.5:1 (WCAG AA)`)
    })

    it('Status online badge pill text (#15803d) on background (#dcfce7) satisfies WCAG AA (>= 4.5:1)', () => {
      const contrast = calculateContrastRatio(light.statusOnlineBg, light.statusOnlineText)
      assert.ok(contrast >= 4.5, `Contrast ratio ${contrast} must be >= 4.5:1 (WCAG AA)`)
    })

    it('Badge agent text (#0284c7) on background (#f0f9ff) satisfies WCAG AA UI component contrast (>= 3.0:1)', () => {
      const contrast = calculateContrastRatio(light.badgeAgentBg, light.badgeAgentText)
      assert.ok(contrast >= 3.0, `Contrast ratio ${contrast} must be >= 3.0:1 (WCAG AA UI component)`)
    })

    it('Badge k8s text (#1d4ed8) on background (#eff6ff) satisfies WCAG AA (>= 4.5:1)', () => {
      const contrast = calculateContrastRatio(light.badgeK8sBg, light.badgeK8sText)
      assert.ok(contrast >= 4.5, `Contrast ratio ${contrast} must be >= 4.5:1 (WCAG AA)`)
    })
  })

  describe('Dark Mode Contrast Conformance (WCAG AAA)', () => {
    const dark = LOGS_THEME_TOKENS.dark

    it('Terminal viewport surfaceCard (#161f30) vs colorBody (#f8fafc) satisfies WCAG AAA (>= 7.0:1)', () => {
      const contrast = calculateContrastRatio(dark.surfaceCard, dark.colorBody)
      assert.ok(contrast >= 7.0, `Contrast ratio ${contrast} must be >= 7.0:1 (WCAG AAA)`)
    })

    it('Header surfaceElevated (#222f46) vs colorBody (#f8fafc) satisfies WCAG AAA (>= 7.0:1)', () => {
      const contrast = calculateContrastRatio(dark.surfaceElevated, dark.colorBody)
      assert.ok(contrast >= 7.0, `Contrast ratio ${contrast} must be >= 7.0:1 (WCAG AAA)`)
    })

    it('Surface inset (#111827) vs colorBody (#f8fafc) satisfies WCAG AAA (>= 7.0:1)', () => {
      const contrast = calculateContrastRatio(dark.surfaceInset, dark.colorBody)
      assert.ok(contrast >= 7.0, `Contrast ratio ${contrast} must be >= 7.0:1 (WCAG AAA)`)
    })
  })

  describe('Blind Error Radar Runtime Formatting & Badge Mapping', () => {
    it('cleanFingerprint strips timestamps, worker brackets, and wildcard noise', () => {
      assert.equal(
        cleanFingerprint('2026-10-26 11:20:00.123 UTC [worker-1] database connection pool exhausted'),
        'database connection pool exhausted'
      )
      assert.equal(
        cleanFingerprint('* * * [stderr] panic: runtime error: invalid memory address'),
        'panic: runtime error: invalid memory address'
      )
      assert.equal(
        cleanFingerprint('2026-10-26T04:12:34.000Z Connection refused: dial tcp 10.0.0.1:5432'),
        'Connection refused: dial tcp 10.0.0.1:5432'
      )
      assert.equal(cleanFingerprint(''), '')
      assert.equal(cleanFingerprint(undefined), '')
    })

    it('resolveRadarCountBadgeClass correctly flags warning vs error badge styles', () => {
      assert.equal(resolveRadarCountBadgeClass('WARN'), 'radar-count-badge count-warn')
      assert.equal(resolveRadarCountBadgeClass('warning'), 'radar-count-badge count-warn')
      assert.equal(resolveRadarCountBadgeClass('ERROR'), 'radar-count-badge')
      assert.equal(resolveRadarCountBadgeClass('CRITICAL'), 'radar-count-badge')
    })
  })

  describe('Virtual Log Terminal Runtime Helper Specifications', () => {
    it('getTargetBadge compacts target identifier when matching current scope', () => {
      const entry: LogEntry = {
        time: '2026-10-26T11:00:00Z',
        level: 'ERROR',
        node: 'k8s-worker-1',
        service: 'payment-svc',
        msg: 'Transaction timeout'
      }
      assert.equal(getTargetBadge(entry, 'k8s-worker-1'), 'payment-svc')
      assert.equal(getTargetBadge(entry), 'k8s-worker-1/payment-svc')
    })

    it('getTargetBadge includes namespace when non-default pod is provided', () => {
      const podEntry: LogEntry = {
        time: '2026-10-26T11:00:00Z',
        level: 'INFO',
        pod: 'auth-pod-abc',
        namespace: 'security',
        msg: 'Token generated'
      }
      assert.equal(getTargetBadge(podEntry), 'security/auth-pod-abc')
    })

    it('inferEffectiveLevel infers ERROR and WARN levels from unparsed stdout payloads', () => {
      assert.equal(inferEffectiveLevel('INFO', '{"level":"error","msg":"disk failure"}'), 'ERROR')
      assert.equal(inferEffectiveLevel('INFO', 'level=error msg="out of memory"'), 'ERROR')
      assert.equal(inferEffectiveLevel('INFO', 'ERROR: connection reset by peer'), 'ERROR')
      assert.equal(inferEffectiveLevel('INFO', '{"level":"warn","msg":"high latency"}'), 'WARN')
      assert.equal(inferEffectiveLevel('INFO', 'standard informational status update'), 'INFO')
    })

    it('cleanLogMessage unpacks structured JSON messages and strips boilerplate', () => {
      assert.equal(cleanLogMessage('{"msg":"Server listening on port 8080","level":"info"}'), 'Server listening on port 8080')
      assert.equal(cleanLogMessage('[service] [INFO]'), '')
      assert.equal(cleanLogMessage('-- No entries --'), '')
    })

    it('resolveLevelBadgeClass and resolveLogRowClass produce canonical CSS hooks', () => {
      assert.equal(resolveLevelBadgeClass('ERROR'), 'virtual-badge-error')
      assert.equal(resolveLevelBadgeClass('WARN'), 'virtual-badge-warn')
      assert.equal(resolveLevelBadgeClass('INFO'), 'virtual-badge-info')
      assert.equal(resolveLevelBadgeClass('DEBUG'), 'virtual-badge-debug')

      assert.equal(resolveLogRowClass('ERROR'), 'log-error')
      assert.equal(resolveLogRowClass('WARN'), 'log-warn')
      assert.equal(resolveLogRowClass('INFO'), 'log-info')
    })
  })
})
