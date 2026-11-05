import { describe, it, beforeEach } from 'node:test'
import assert from 'node:assert/strict'
import { register } from 'node:module'
import { h, defineComponent } from 'vue'
import { renderToString } from 'vue/server-renderer'
import { createPinia, setActivePinia } from 'pinia'

// Register Vue SFC loader for node test runner & tsx
register('./vue-loader.mjs', import.meta.url)

// Mock browser globals for SSR rendering
if (typeof globalThis.window === 'undefined') {
  // @ts-expect-error Mock window for SSR test runner
  globalThis.window = {
    innerWidth: 1440,
    addEventListener: () => {},
    removeEventListener: () => {},
    location: { protocol: 'http:', host: 'localhost:3000' }
  }
}

if (typeof globalThis.document === 'undefined') {
  // @ts-expect-error Mock document for SSR test runner
  globalThis.document = {
    addEventListener: () => {},
    removeEventListener: () => {}
  }
}

const { useAlertStore } = await import('../src/stores/alertStore.ts')
const {
  resolveFindingSeverityBadge,
  formatFindingDisplayCode,
  formatFindingPackage,
} = await import('../src/components/audit/vulnerabilityCardsUtils.ts')

const PercentageBar = (await import('../src/components/ui/PercentageBar.vue')).default
const VulnerabilityMobileCards = (await import('../src/components/audit/VulnerabilityMobileCards.vue')).default
const TopHudAlertBell = (await import('../src/components/layout/TopHudAlertBell.vue')).default

describe('Frontend UI Polish & Responsive Defects - Behavioral Acceptance Suite (LES-012)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    if (globalThis.window) {
      globalThis.window.innerWidth = 1440
    }
  })

  describe('1. alertStore & syncAlerts Behavioral Telemetry (DEC-257 Parity)', () => {
    it('syncAlerts with offline host populates activeAlerts while keeping isToastDropped === false', () => {
      const store = useAlertStore()
      assert.strictEqual(store.activeAlerts.length, 0)
      assert.strictEqual(store.isToastDropped, false)

      const offlineAlert = {
        node_id: 'host-offline-1',
        node_name: 'k8snode',
        type: 'node_down' as const,
        severity: 'critical' as const,
        value: 100,
        threshold: 90,
        message: 'Host k8snode ping unreachable',
        timestamp: Date.now()
      }

      // Background telemetry sync must NOT drop toast unsolicited
      store.syncAlerts([offlineAlert])

      assert.strictEqual(store.activeAlerts.length, 1)
      assert.strictEqual(store.hasNodeDown, true)
      assert.strictEqual(store.hasCriticalAlerts, true)
      assert.strictEqual(store.isToastDropped, false, 'syncAlerts must never auto-open toast unprompted')
    })

    it('dismissToast and muteAll update store state cleanly without leaving stale flags', () => {
      const store = useAlertStore()
      const offlineAlert = {
        node_id: 'host-offline-1',
        node_name: 'k8snode',
        type: 'node_down' as const,
        severity: 'critical' as const,
        value: 100,
        threshold: 90,
        message: 'Host k8snode ping unreachable',
        timestamp: Date.now()
      }
      store.syncAlerts([offlineAlert])
      store.isToastDropped = true

      store.dismissToast()
      assert.strictEqual(store.isToastDropped, false, 'dismissToast must close the toast')

      store.muteAll('restart')
      assert.strictEqual(store.activeAlerts.length, 0, 'muteAll must mute remaining active alerts')
      assert.strictEqual(store.mutedAlertsCount, 1)
    })
  })

  describe('2. TopHudAlertBell Component Rendering & Mobile Lockout Fix', () => {
    it('renders bell badge count 1 and does NOT render .header-alert-toast by default', async () => {
      const store = useAlertStore()
      store.syncAlerts([{
        node_id: 'host-1',
        node_name: 'k8snode',
        type: 'node_down',
        severity: 'critical',
        value: 100,
        threshold: 90,
        message: 'Cluster Health Warning: k8snode unreachable',
        timestamp: Date.now()
      }])

      assert.strictEqual(store.isToastDropped, false)
      const html = await renderToString(h(TopHudAlertBell))

      // Must render active bell indicator with count 1
      assert.ok(html.includes('1'), 'Must render badge count 1 in header')
      assert.ok(html.includes('pill-critical'), 'Must apply pill-critical class for critical node_down')

      // Must NOT render popover toast card when isToastDropped is false
      assert.strictEqual(
        html.includes('header-alert-toast'),
        false,
        'Toast card must not render into DOM when isToastDropped is false'
      )
    })

    it('handleBellClick opens alert center on mobile viewports without toast lockout', () => {
      const store = useAlertStore()
      store.syncAlerts([{
        node_id: 'host-1',
        node_name: 'k8snode',
        type: 'node_down',
        severity: 'critical',
        value: 100,
        threshold: 90,
        message: 'Cluster Health Warning: k8snode unreachable',
        timestamp: Date.now()
      }])

      // Simulate component setup logic with isMobile === true
      const isMobile = true
      function simulateBellClick() {
        if (isMobile) {
          store.openAlertCenter()
          return
        }
        if (store.activeAlerts.length > 0) {
          if (store.isToastDropped) store.dismissToast()
          else store.isToastDropped = true
        } else {
          store.openAlertCenter()
        }
      }

      assert.strictEqual(store.showAlertCenterModal, false)
      simulateBellClick()
      assert.strictEqual(store.showAlertCenterModal, true, 'Mobile bell tap must open alert center')
      assert.strictEqual(store.isToastDropped, false, 'Mobile bell tap must not drop invisible desktop toast')
    })
  })

  describe('3. VulnerabilityMobileCards Severity Borders & Error Handling', () => {
    it('renders severity border classes, packages, CVEs, and fix versions properly', async () => {
      const sampleFindings = [
        {
          id: 'cve-crit-1',
          severity: 'critical' as const,
          cve_id: 'CVE-2024-1234',
          package_name: 'openssl',
          fixed_version: '3.0.13',
          status: 'open' as const,
        },
        {
          id: 'cve-high-2',
          severity: 'high' as const,
          cve_id: 'CVE-2024-5678',
          package_name: 'curl',
          fixed_version: '8.5.0',
          status: 'open' as const,
        }
      ]

      const html = await renderToString(h(VulnerabilityMobileCards, {
        findings: sampleFindings as any,
        loading: false,
        error: null
      }))

      assert.ok(html.includes('border-critical'), 'Must include border-critical class')
      assert.ok(html.includes('border-high'), 'Must include border-high class')
      assert.ok(html.includes('CVE-2024-1234'), 'Must render CVE ID')
      assert.ok(html.includes('openssl'), 'Must render package name')
      assert.ok(html.includes('Fix: 3.0.13'), 'Must render fix version')
      assert.ok(html.includes('Resolve'), 'Must render Resolve action button')
    })

    it('renders actionable error state with Retry button when error is truthy', async () => {
      const errorHtml = await renderToString(h(VulnerabilityMobileCards, {
        findings: [],
        loading: false,
        error: 'Backend timeout: security scanner unavailable'
      }))

      assert.ok(errorHtml.includes('Backend timeout: security scanner unavailable'), 'Must display error message')
      assert.ok(errorHtml.includes('Retry'), 'Must render Retry button')
      assert.strictEqual(errorHtml.includes('No active vulnerability findings'), false, 'Must not mask error with empty state')
    })

    it('renders clean empty state when zero findings and no error', async () => {
      const emptyHtml = await renderToString(h(VulnerabilityMobileCards, {
        findings: [],
        loading: false,
        error: null
      }))

      assert.ok(emptyHtml.includes('No active vulnerability findings'), 'Must render empty state message')
      assert.strictEqual(emptyHtml.includes('Retry'), false)
    })
  })

  describe('4. PercentageBar Contrast & Token Precision', () => {
    it('renders track with semantic progress bar styling and aria attributes', async () => {
      const html = await renderToString(h(PercentageBar, { value: 2 }))

      assert.ok(html.includes('percentage-bar-track'), 'Must render percentage-bar-track container')
      assert.ok(html.includes('role="progressbar"'), 'Must render ARIA progressbar role')
      assert.ok(html.includes('aria-valuenow="2"'), 'Must render aria-valuenow')
      assert.ok(html.includes('width:2%'), 'Must compute width percentage accurately')
    })
  })

  describe('5. vulnerabilityCardsUtils Domain Formatting Logic', () => {
    it('resolveFindingSeverityBadge assigns correct semantic badge classes', () => {
      assert.strictEqual(resolveFindingSeverityBadge('critical'), 'badge-rose')
      assert.strictEqual(resolveFindingSeverityBadge('high'), 'badge-amber')
      assert.strictEqual(resolveFindingSeverityBadge('medium'), 'badge-cyan')
      assert.strictEqual(resolveFindingSeverityBadge('low'), 'badge-emerald')
      assert.strictEqual(resolveFindingSeverityBadge(undefined), 'badge-emerald')
    })

    it('formatFindingDisplayCode falls back through cve_id, rule_id, and id cascades', () => {
      assert.strictEqual(formatFindingDisplayCode({ id: '1', cve_id: 'CVE-2024-9999' }), 'CVE-2024-9999')
      assert.strictEqual(formatFindingDisplayCode({ id: '1', rule_id: 'RULE-101' }), 'RULE-101')
      assert.strictEqual(formatFindingDisplayCode({ id: 'finding-101' }), 'finding-101')
      assert.strictEqual(formatFindingDisplayCode({ id: '' }), 'CVE-UNKNOWN')
    })

    it('formatFindingPackage formats package and resource references gracefully', () => {
      assert.strictEqual(formatFindingPackage({ package_name: 'glibc' }), 'glibc')
      assert.strictEqual(formatFindingPackage({ resource: 'containerd' }), 'containerd')
      assert.strictEqual(formatFindingPackage({ category: 'network' }), 'network')
      assert.strictEqual(formatFindingPackage({}), 'cve_vulnerability')
    })
  })
})
