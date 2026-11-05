import { describe, it, beforeEach } from 'node:test'
import assert from 'node:assert/strict'
import { register } from 'node:module'
import { createPinia, setActivePinia } from 'pinia'
import fs from 'node:fs'
import path from 'node:path'
import { h, defineComponent } from 'vue'
import { renderToString } from 'vue/server-renderer'

register('data:text/javascript,' + encodeURIComponent(`
export async function resolve(specifier, context, nextResolve) {
  try {
    return await nextResolve(specifier, context);
  } catch (err) {
    if (specifier.startsWith('.')) {
      try {
        return await nextResolve(specifier + '.ts', context);
      } catch {}
    }
    throw err;
  }
}
`))

const { useAlertStore } = await import('../src/stores/alertStore')

describe('Frontend UI Polish & Responsive Defects - Behavioral Tests', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  describe('Defect 1: /audit Mobile 375px Sub-Tab Pill Overflow & Bottom Nav Clearance', () => {
    it('audit.css defines horizontal scroll on .audit-tab-switcher and nowrap on tab buttons', () => {
      const cssPath = path.resolve(process.cwd(), 'src/assets/styles/views/audit.css')
      const content = fs.readFileSync(cssPath, 'utf8')

      // .audit-tab-switcher must scroll horizontally on narrow viewports
      assert.ok(content.includes('overflow-x: auto'), 'Tab switcher must have overflow-x: auto')
      assert.ok(content.includes('scrollbar-width: none'), 'Tab switcher must hide native scrollbar')
      assert.ok(content.includes('.audit-tab-switcher::-webkit-scrollbar'), 'Webkit scrollbar must be hidden')

      // Tab button and its inner span must never wrap
      assert.ok(
        content.includes('button.audit-tab-btn, .audit-tab-btn') || content.includes('.audit-tab-btn'),
        'Tab btn style rule must exist'
      )
      assert.ok(content.includes('white-space: nowrap'), 'Tab btn must enforce white-space: nowrap')
      assert.ok(content.includes('flex-shrink: 0'), 'Tab btn must enforce flex-shrink: 0')

      // Mobile padding-bottom 84px for bottom nav clearance
      assert.ok(
        content.includes('padding-bottom: 84px'),
        '.audit-view must have padding-bottom: 84px on mobile for bottom nav clearance'
      )
    })
  })

  describe('Defect 2A & 2B: /audit Tablet 768px Breakpoint & Topbar Truncation Relaxation', () => {
    it('audit.css aligns desktop vs mobile view on DEC-257 1024px paired breakpoint (zero dead zone)', () => {
      const cssPath = path.resolve(process.cwd(), 'src/assets/styles/views/audit.css')
      const content = fs.readFileSync(cssPath, 'utf8')

      assert.ok(content.includes('@media (max-width: 1023.98px)'), 'Must pair with max-width: 1023.98px')
      assert.ok(content.includes('@media (min-width: 1024px)'), 'Must pair with min-width: 1024px')
    })

    it('AuditView.vue provides both desktop-view and mobile-view for Container CVEs sub-tab', () => {
      const auditViewPath = path.resolve(process.cwd(), 'src/views/AuditView.vue')
      const content = fs.readFileSync(auditViewPath, 'utf8')

      // Check sub-view 2 has both .audit-desktop-view and .audit-mobile-view
      const findingsSectionMatch = content.match(/<section[^>]*activeTab === 'findings'[\s\S]*?<\/section>/)
      assert.ok(findingsSectionMatch, 'activeTab === "findings" section must exist')
      const sectionContent = findingsSectionMatch[0]
      assert.ok(sectionContent.includes('audit-desktop-view'), 'Findings sub-tab must include .audit-desktop-view')
      assert.ok(sectionContent.includes('audit-mobile-view'), 'Findings sub-tab must include .audit-mobile-view')
      assert.ok(sectionContent.includes('VulnerabilityMobileCards'), 'Findings sub-tab must mount VulnerabilityMobileCards')
    })

    it('vulnerabilityCardsUtils accurately resolves severity badges and fallback display codes', async () => {
      const {
        resolveFindingSeverityBadge,
        formatFindingDisplayCode,
        formatFindingPackage,
      } = await import('../src/components/audit/vulnerabilityCardsUtils')

      assert.strictEqual(resolveFindingSeverityBadge('critical'), 'badge-rose')
      assert.strictEqual(resolveFindingSeverityBadge('CRITICAL'), 'badge-rose')
      assert.strictEqual(resolveFindingSeverityBadge('high'), 'badge-amber')
      assert.strictEqual(resolveFindingSeverityBadge('medium'), 'badge-cyan')
      assert.strictEqual(resolveFindingSeverityBadge('low'), 'badge-emerald')
      assert.strictEqual(resolveFindingSeverityBadge(undefined), 'badge-emerald')

      assert.strictEqual(
        formatFindingDisplayCode({ id: 'raw-1', cve_id: 'CVE-2026-9999' }),
        'CVE-2026-9999'
      )
      assert.strictEqual(
        formatFindingDisplayCode({ id: 'raw-2', rule_id: 'SEC-001' }),
        'SEC-001'
      )
      assert.strictEqual(
        formatFindingDisplayCode({ id: 'raw-3' }),
        'raw-3'
      )

      assert.strictEqual(
        formatFindingPackage({ package_name: 'libssl' }),
        'libssl'
      )
      assert.strictEqual(
        formatFindingPackage({ resource: 'deployment/api' }),
        'deployment/api'
      )
    })

    it('global-context-selector.css and app-shell.css relax pill-text clamp up to 135px on tablet', () => {
      const gcsCssPath = path.resolve(process.cwd(), 'src/assets/styles/components/global-context-selector.css')
      const gcsContent = fs.readFileSync(gcsCssPath, 'utf8')
      assert.ok(
        gcsContent.includes('max-width: 135px'),
        'global-context-selector.css must relax pill-text up to 135px on tablet'
      )

      const appShellCssPath = path.resolve(process.cwd(), 'src/assets/styles/layout/app-shell.css')
      const appShellContent = fs.readFileSync(appShellCssPath, 'utf8')
      assert.ok(
        appShellContent.includes('max-width: 135px'),
        'app-shell.css must relax pill-text up to 135px on tablet'
      )
    })
  })

  describe('Defect 3: Top HUD Alert Bell & alertStore Telemetry Occlusion Prevention', () => {
    it('syncAlerts updates rawAlerts and activeAlerts WITHOUT auto-dropping toast (no unsolicited popover)', () => {
      const store = useAlertStore()
      assert.strictEqual(store.isToastDropped, false)

      const sampleAlerts = [
        {
          node_id: 'node-worker-1',
          node_name: 'k8snode-worker-1',
          type: 'node_down',
          level: 'critical',
          message: 'Node unreachable',
          timestamp: new Date().toISOString(),
        },
      ]

      // Background telemetry polling triggers syncAlerts
      store.syncAlerts(sampleAlerts)

      // Active alert count is 1 and critical
      assert.strictEqual(store.activeAlerts.length, 1)
      assert.strictEqual(store.hasCriticalAlerts, true)

      // Popover toast MUST NOT drop automatically
      assert.strictEqual(store.isToastDropped, false)
    })

    it('dismissToast resets isToastDropped to false', () => {
      const store = useAlertStore()
      store.syncAlerts([
        {
          node_id: 'node-1',
          node_name: 'k8snode',
          type: 'high_cpu',
          level: 'warning',
          message: 'CPU 95%',
          timestamp: new Date().toISOString(),
        },
      ])

      // User explicitly opened the popover
      store.isToastDropped = true
      assert.strictEqual(store.isToastDropped, true)

      // User dismisses
      store.dismissToast()
      assert.strictEqual(store.isToastDropped, false)
    })

    it('muteAll silences alerts and resets isToastDropped to false', () => {
      const store = useAlertStore()
      store.syncAlerts([
        {
          node_id: 'node-1',
          node_name: 'k8snode',
          type: 'node_down',
          level: 'critical',
          message: 'Node unreachable',
          timestamp: new Date().toISOString(),
        },
      ])

      store.isToastDropped = true
      store.muteAll('restart')

      assert.strictEqual(store.isToastDropped, false)
      assert.strictEqual(store.activeAlerts.length, 0)
      assert.strictEqual(store.mutedAlertsCount, 1)
    })
  })

  describe('Defect 4A: /overview Infrastructure Hosts Left-Edge Clipping & Light Mode Track Contrast', () => {
    it('overview-hosts.css and overview.css add horizontal breathing room', () => {
      const hostsCssPath = path.resolve(process.cwd(), 'src/assets/styles/components/overview-hosts.css')
      const hostsContent = fs.readFileSync(hostsCssPath, 'utf8')
      assert.ok(
        hostsContent.includes('padding: 0 6px'),
        '.hosts-header-row.topology-header-row must have padding: 0 6px so pulse dot and title are not clipped'
      )

      const overviewCssPath = path.resolve(process.cwd(), 'src/assets/styles/views/overview.css')
      const overviewContent = fs.readFileSync(overviewCssPath, 'utf8')
      assert.ok(
        overviewContent.includes('padding: 6px 6px 32px'),
        '.overview-dashboard must have safe horizontal padding'
      )
    })

    it('PercentageBar track uses var(--color-surface-inset) and border for dual-theme contrast', () => {
      const filePath = path.resolve(process.cwd(), 'src/components/ui/PercentageBar.vue')
      const content = fs.readFileSync(filePath, 'utf8')
      assert.ok(
        !content.includes('rgba(255, 255, 255, 0.08)'),
        'PercentageBar track must not use rgba(255, 255, 255, 0.08) which is invisible in Light Mode'
      )
      assert.ok(
        content.includes('--color-surface-inset'),
        'PercentageBar track must use var(--color-surface-inset) for dual-theme contrast'
      )
    })
  })

  describe('Defect 4B: /logs Toolbar btn-toggle-targets Specificity Collision & Crisp SVGs', () => {
    it('logstream.css provides padding: 0 on btn-toggle-targets and flex-shrink: 0 on toolbar svgs', () => {
      const logstreamCssPath = path.resolve(process.cwd(), 'src/assets/styles/views/logstream.css')
      const content = fs.readFileSync(logstreamCssPath, 'utf8')

      assert.ok(
        content.includes('button.toolbar-btn.btn-toggle-targets'),
        'button.toolbar-btn.btn-toggle-targets selector must exist to override .btn-secondary padding'
      )
      assert.ok(
        content.includes('padding: 0 !important'),
        'btn-toggle-targets must have padding: 0 !important'
      )
      assert.ok(
        content.includes('.toolbar-btn svg') && content.includes('flex-shrink: 0'),
        '.toolbar-btn svg must have flex-shrink: 0'
      )
    })
  })

  describe('File Size Invariants (<= 350 views, < 300 components, < 500 styles)', () => {
    it('TopHudAlertBell.vue is strictly < 300 lines', () => {
      const filePath = path.resolve(process.cwd(), 'src/components/layout/TopHudAlertBell.vue')
      const content = fs.readFileSync(filePath, 'utf8')
      const lines = content.split('\n').length
      assert.ok(lines < 300, `TopHudAlertBell.vue must be < 300 lines (current: ${lines})`)
    })

    it('AuditView.vue is strictly <= 350 lines', () => {
      const filePath = path.resolve(process.cwd(), 'src/views/AuditView.vue')
      const content = fs.readFileSync(filePath, 'utf8')
      const lines = content.split('\n').length
      assert.ok(lines <= 350, `AuditView.vue must be <= 350 lines (current: ${lines})`)
    })

    it('VulnerabilityMobileCards.vue is strictly < 300 lines', () => {
      const filePath = path.resolve(process.cwd(), 'src/components/audit/VulnerabilityMobileCards.vue')
      const content = fs.readFileSync(filePath, 'utf8')
      const lines = content.split('\n').length
      assert.ok(lines < 300, `VulnerabilityMobileCards.vue must be < 300 lines (current: ${lines})`)
    })

    it('PercentageBar.vue is strictly < 300 lines', () => {
      const filePath = path.resolve(process.cwd(), 'src/components/ui/PercentageBar.vue')
      const content = fs.readFileSync(filePath, 'utf8')
      const lines = content.split('\n').length
      assert.ok(lines < 300, `PercentageBar.vue must be < 300 lines (current: ${lines})`)
    })

    it('audit.css, app-shell.css, and logstream.css are strictly < 500 lines', () => {
      const files = [
        'src/assets/styles/views/audit.css',
        'src/assets/styles/layout/app-shell.css',
        'src/assets/styles/views/logstream.css',
        'src/assets/styles/views/overview.css',
        'src/assets/styles/components/overview-hosts.css',
        'src/assets/styles/components/global-context-selector.css',
        'src/assets/styles/components/floating-alert-toast.css',
      ]
      for (const rel of files) {
        const full = path.resolve(process.cwd(), rel)
        const content = fs.readFileSync(full, 'utf8')
        const lines = content.split('\n').length
        assert.ok(lines < 500, `${rel} must be < 500 lines (current: ${lines})`)
      }
    })
  })
})
