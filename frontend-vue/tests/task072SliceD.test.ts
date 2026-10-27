import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const SRC_DIR = path.resolve(__dirname, '../src')

function readFile(relPath: string): string {
  return fs.readFileSync(path.resolve(SRC_DIR, relPath), 'utf-8')
}

describe('Task 072 — Subtask 72D Acceptance & Regression Suite', () => {
  describe('Criterion 1: SloCreateModal.vue Full-Viewport Overlay & Unclipped Header (t072_12)', () => {
    it('SloCreateModal.vue uses Teleport to body and sets role="dialog" and aria-modal="true"', () => {
      const modal = readFile('components/slo/SloCreateModal.vue')
      assert.match(modal, /<Teleport\s+to="body">/)
      assert.match(modal, /role="dialog"/)
      assert.match(modal, /aria-modal="true"/)
    })

    it('SloCreateModal.vue keeps header and footer unclipped outside or fixed relative to scrollable modal-body', () => {
      const modal = readFile('components/slo/SloCreateModal.vue')
      assert.match(modal, /modal-header/)
      assert.match(modal, /modal-footer/)
      assert.match(modal, /modal-body/)
    })

    it('slo-drawers.css defines full-viewport overlay with z-modal, blur, and 24px padding', () => {
      const css = readFile('assets/styles/components/slo-drawers.css')
      assert.match(css, /\.modal-overlay\s*\{[^}]*position:\s*fixed;/)
      assert.match(css, /\.modal-overlay\s*\{[^}]*inset:\s*0;/)
      assert.match(css, /\.modal-overlay\s*\{[^}]*z-index:\s*var\(--z-modal,\s*50\);/)
      assert.match(css, /\.modal-overlay\s*\{[^}]*display:\s*flex;/)
      assert.match(css, /\.modal-overlay\s*\{[^}]*align-items:\s*center;/)
      assert.match(css, /\.modal-overlay\s*\{[^}]*justify-content:\s*center;/)
      assert.match(css, /\.modal-overlay\s*\{[^}]*padding:\s*24px;/)
      assert.match(css, /\.modal-overlay\s*\{[^}]*background:\s*rgba\(15,\s*23,\s*42,\s*0\.55\);/)
      assert.match(css, /\.modal-overlay\s*\{[^}]*backdrop-filter:\s*blur\(4px\);/)
    })

    it('slo-drawers.css constrains modal-card to max-height calc(100vh - 48px) with flex column and scrollable body', () => {
      const css = readFile('assets/styles/components/slo-drawers.css')
      assert.match(css, /\.modal-card\s*\{[^}]*max-height:\s*calc\(100vh\s*-\s*48px\);/)
      assert.match(css, /\.modal-card\s*\{[^}]*display:\s*flex;/)
      assert.match(css, /\.modal-card\s*\{[^}]*flex-direction:\s*column;/)
      assert.match(css, /\.modal-body\s*\{[^}]*overflow-y:\s*auto;/)
    })
  })

  describe('Criterion 2: Global Pure-CSS Auto-Hide of PwaInstallBanner During Active Modals/Drawers (t072_12)', () => {
    it('pwa-install-banner.css includes comprehensive :has rule for dialogs, modals, and drawers', () => {
      const css = readFile('assets/styles/components/pwa-install-banner.css')
      assert.match(
        css,
        /body:has\([^)]*\[role=["']dialog["']\][^)]*\[aria-modal=["']true["']\][^)]*\.modal-overlay[^)]*\.modal-backdrop[^)]*\.drawer-overlay[^)]*\.inspector-backdrop\)[^{]*\.pwa-install-banner\s*\{[^}]*display:\s*none\s*!important;/
      )
    })
  })

  describe('Criterion 3: AlertRulesTable.vue & alerts.css Zero Horizontal Clipping at 1440x900 (t072_06)', () => {
    it('AlertRulesTable.vue allocates at least 18% width to Management actions column', () => {
      const table = readFile('components/alerts/AlertRulesTable.vue')
      const match = table.match(/key:\s*'actions'[^}]*width:\s*'([^']+)'/)
      assert.ok(match, 'Actions column must have a width definition')
      const widthVal = match[1]
      if (widthVal.endsWith('%')) {
        const pct = parseFloat(widthVal)
        assert.ok(pct >= 18, `Actions column width (${pct}%) must be >= 18%`)
      } else if (widthVal.endsWith('px')) {
        const px = parseFloat(widthVal)
        assert.ok(px >= 170, `Actions column width (${px}px) must be >= 170px`)
      }
    })

    it('AlertRulesTable.vue enforces nowrap and minimum width on rule-actions-cell', () => {
      const table = readFile('components/alerts/AlertRulesTable.vue')
      assert.match(table, /\.rule-actions-cell\s*\{[^}]*white-space:\s*nowrap;/)
      assert.match(table, /\.rule-actions-cell\s*\{[^}]*min-width:\s*(?:150|160|170|180)px;/)
    })

    it('AlertRulesTable.vue and alerts.css truncate PromQL cell cleanly', () => {
      const table = readFile('components/alerts/AlertRulesTable.vue')
      assert.match(table, /promql-cell|truncate/)
      const css = readFile('assets/styles/views/alerts.css')
      assert.match(css, /\.desktop-table-view\s*table|data-table\s*\{[^}]*table-layout:\s*fixed;/)
    })
  })

  describe('Criterion 4: WorkloadInspectorDrawer.vue Truthful Daemon Labels (t072_10)', () => {
    it('WorkloadInspectorDrawer.vue dynamically changes tab labels for daemon workloads', () => {
      const drawer = readFile('components/deployments/WorkloadInspectorDrawer.vue')
      assert.match(drawer, /isDaemon\s*\?\s*['"]Daemon Logs['"]\s*:\s*['"]Container Logs['"]/)
      assert.match(drawer, /isDaemon\s*\?\s*['"]Systemd Unit['"]\s*:\s*['"]YAML Manifest['"]/)
    })

    it('WorkloadInspectorDrawer.vue hides or customizes Strategy & Traffic tab for daemon workloads', () => {
      const drawer = readFile('components/deployments/WorkloadInspectorDrawer.vue')
      assert.match(drawer, /v-if="!isDaemon"/)
    })

    it('WorkloadInspectorDrawer.vue dynamically updates YAML/Unit spec header and copy button for daemon workloads', () => {
      const drawer = readFile('components/deployments/WorkloadInspectorDrawer.vue')
      assert.match(drawer, /isDaemon\s*\?\s*['"]Systemd Unit Spec['"]\s*:\s*['"]Live Kubernetes Spec['"]/)
      assert.match(drawer, /isDaemon\s*\?\s*['"]Copy Unit['"]\s*:\s*['"]Copy Manifest['"]/)
    })
  })

  describe('Criterion 5: Primary Modal CTA Button Contrast in Light & Dark Modes (t072_08, t072_09, t072_11, t072_12)', () => {
    it('tokens.css defines --accent-primary in both dark (:root) and light ([data-theme="light"]) themes', () => {
      const tokens = readFile('assets/css/tokens.css')
      assert.match(tokens, /:root[\s\S]*?--accent-primary:\s*(?:#2563eb|var\(--color-primary\));/)
      assert.match(tokens, /\[data-theme=["']light["']\][\s\S]*?--accent-primary:\s*(?:#2563eb|var\(--color-primary\));/)
    })

    it('buttons.css enforces primary CTA styling with high specificity and crisp hover state', () => {
      const buttons = readFile('assets/css/buttons.css')
      assert.match(buttons, /\.btn-primary[^}]*background:\s*var\(--accent-primary,\s*#2563eb\);/)
      assert.match(buttons, /\.btn-primary[^}]*color:\s*#ffffff;/)
      assert.match(buttons, /\.btn-primary[^}]*border-color:\s*var\(--accent-primary,\s*#2563eb\);/)
    })

    it('FleetImportModal, HostAddEditModal, HelmInstallDrawer, and SloCreateModal use btn-primary for submit and btn-secondary for cancel/dry-run', () => {
      const fleet = readFile('components/fleet/FleetImportModal.vue')
      assert.match(fleet, /class="[^"]*btn-primary[^"]*"[^>]*>[\s\S]*?Import Cluster/)
      assert.match(fleet, /class="[^"]*btn-secondary[^"]*"[^>]*>[\s\S]*?Validate Connectivity/)
      assert.match(fleet, /class="[^"]*btn-secondary[^"]*"[^>]*>[\s\S]*?Cancel/)

      const host = readFile('components/hosts/HostAddEditModal.vue')
      assert.match(host, /class="[^"]*btn-primary[^"]*"[^>]*>[\s\S]*?(?:Register Host|Update Host)/)
      assert.match(host, /class="[^"]*btn-secondary[^"]*"[^>]*>[\s\S]*?Test Connection/)
      assert.match(host, /class="[^"]*btn-secondary[^"]*"[^>]*>[\s\S]*?Cancel/)

      const helm = readFile('components/helm/HelmInstallDrawer.vue')
      assert.match(helm, /class="[^"]*btn-primary[^"]*"[^>]*>[\s\S]*?Confirm & Deploy Release/)
      assert.match(helm, /class="[^"]*btn-secondary[^"]*"[^>]*>[\s\S]*?Back/)

      const slo = readFile('components/slo/SloCreateModal.vue')
      assert.match(slo, /class="[^"]*btn-primary[^"]*"[^>]*>[\s\S]*?(?:Save Changes|Save & Arm SLO Target)/)
      assert.match(slo, /class="[^"]*btn-secondary[^"]*"[^>]*>[\s\S]*?Cancel/)
    })
  })
})
