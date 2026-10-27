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

describe('Task 072 — Subtask 72B Regression Suite', () => {
  describe('Criterion 1: tokens.css & buttons.css Light Mode Rescue (UI-01)', () => {
    it('tokens.css defines --bg-card-hover, surface hover/active, focus-ring, and canonical overlay scale in both dark and light modes', () => {
      const tokens = readFile('assets/css/tokens.css')

      // Canonical overlay scale
      assert.match(tokens, /--z-sticky:\s*10;/)
      assert.match(tokens, /--z-nav:\s*20;/)
      assert.match(tokens, /--z-banner:\s*25;/)
      assert.match(tokens, /--z-dropdown:\s*30;/)
      assert.match(tokens, /--z-drawer:\s*40;/)
      assert.match(tokens, /--z-modal:\s*50;/)
      assert.match(tokens, /--z-toast:\s*60;/)

      // Both dark (:root) and light ([data-theme="light"]) define --bg-card-hover
      assert.match(tokens, /--bg-card-hover:/)
      assert.match(tokens, /\[data-theme="light"\][\s\S]*?--bg-card-hover:\s*#f1f5f9;/)
      assert.match(tokens, /\[data-theme="light"\][\s\S]*?--color-surface-hover:\s*#f1f5f9;/)
      assert.match(tokens, /\[data-theme="light"\][\s\S]*?--color-surface-active:/)
      assert.match(tokens, /\[data-theme="light"\][\s\S]*?--color-focus-ring:/)
    })

    it('buttons.css does not hardcode dark surfaces for hover in light mode', () => {
      const buttons = readFile('assets/css/buttons.css')
      assert.doesNotMatch(buttons, /\.btn-secondary:hover\s*\{\s*background:\s*var\(--bg-card,\s*#161f30\);/)
    })
  })

  describe('Criterion 2: Purge Hardcoded Dark Surfaces & Neon Hexes (UI-02)', () => {
    const targetFiles = [
      'assets/styles/views/logstream-tree.css',
      'assets/css/host-remediation.css',
      'assets/styles/views/infra-hosts-controls.css',
      'assets/styles/components/pod-log-viewer.css',
      'assets/styles/components/pod-log-stream.css',
      'assets/styles/components/pod-terminal.css',
      'assets/styles/components/overview-mesh.css',
      'assets/styles/components/floating-alert-toast.css',
      'assets/styles/components/pwa-install-banner.css',
      'assets/styles/components/events-timeline.css',
      'styles/views/explorer/table.css',
      'assets/styles/views/login.css',
      'assets/styles/components/storage.css',
      'components/deployments/WorkloadLogToolbar.vue',
      'components/telemetry/CanvasTimeSeries.vue',
      'components/overview/matrix/ClusterDensityMatrix.vue',
      'components/logs/ClickHouseEngineBadge.vue',
    ]

    for (const f of targetFiles) {
      it(`${f} contains 0 hardcoded rgba(15, 23, 42) surfaces`, () => {
        const content = readFile(f)
        assert.doesNotMatch(content, /rgba\(15,\s*23,\s*42/)
      })

      it(`${f} contains 0 raw neon hexes (#00e5ff, #00f0ff, #06b6d4)`, () => {
        const content = readFile(f)
        assert.doesNotMatch(content, /#00e5ff/i)
        assert.doesNotMatch(content, /#00f0ff/i)
        assert.doesNotMatch(content, /#06b6d4/i)
      })
    }
  })

  describe('Criterion 3: PWA Banner, Overlay Z-Index & Mobile Chart Visibility (UI-03, UI-05, AGENTS.md)', () => {
    it('pwa-install-banner.css uses canonical --z-banner and hides during active dialogs', () => {
      const bannerCss = readFile('assets/styles/components/pwa-install-banner.css')
      assert.match(bannerCss, /z-index:\s*var\(--z-banner,\s*25\);/)
      assert.match(bannerCss, /modal-backdrop/)
    })

    it('floating-alert-toast.css and alert-center.css normalized to canonical z-indices', () => {
      const toastCss = readFile('assets/styles/components/floating-alert-toast.css')
      assert.doesNotMatch(toastCss, /z-index:\s*99999;/)
      assert.match(toastCss, /--z-toast/)

      const alertCenterCss = readFile('assets/styles/components/alert-center.css')
      assert.doesNotMatch(alertCenterCss, /z-index:\s*100000;/)
      assert.match(alertCenterCss, /--z-modal/)
    })

    it('BaseModal.vue listens to Escape key to dismiss modal', () => {
      const modal = readFile('components/base/BaseModal.vue')
      assert.match(modal, /Escape/)
    })

    it('overview.css does NOT hide topology-section or desktop-overview-sections on mobile', () => {
      const overviewCss = readFile('assets/styles/views/overview.css')
      assert.doesNotMatch(overviewCss, /\.topology-section\s*\{\s*display:\s*none\s*!important;/)
      assert.doesNotMatch(overviewCss, /\.desktop-overview-sections\s*\{\s*display:\s*none\s*!important;/)
    })
  })

  describe('Criterion 4: Auth Pre-Login Truth-in-UI (UI-08)', () => {
    it('LoginBrandingHero.vue uses honest capability badges instead of fake sync claims', () => {
      const hero = readFile('components/auth/LoginBrandingHero.vue')
      assert.doesNotMatch(hero, /Control Plane[\s\S]*?SYNCHRONIZED/)
      assert.doesNotMatch(hero, /Cluster Mesh[\s\S]*?ONLINE/)
      assert.match(hero, /Multi-Tenant RBAC & Audit/)
      assert.match(hero, /Edge Agent Telemetry \(:9100\)/)
    })

    it('LoginView.vue uses honest Authenticated Control Session banner', () => {
      const login = readFile('views/LoginView.vue')
      assert.doesNotMatch(login, /System Online • TLS v1\.3/)
      assert.match(login, /Authenticated Control Session/)
    })
  })

  describe('Criterion 5: Explorer, Deployments, Hosts, Fleet, Helm & Logs Functional Fixes', () => {
    it('ExplorerDetailDrawer.vue passes :name and :kind to EventsTimeline', () => {
      const drawer = readFile('components/explorer/ExplorerDetailDrawer.vue')
      assert.match(drawer, /:name="resource\.metadata\?\.name"/)
      assert.match(drawer, /:kind="resource\.kind"/)
    })

    it('api/k8s.ts uses /k8s/${cluster}/events and appends ?ns= to apply', () => {
      const k8s = readFile('api/k8s.ts')
      assert.match(k8s, /\/k8s\/'\s*\+\s*encodeURIComponent\(cluster\)\s*\+\s*'\/events'/)
      assert.match(k8s, /\/k8s\/\$\{[^}]*\}\/apply\$\{q\}/)
    })

    it('workloadInspectorYaml.ts generates genuine systemd unit for daemon workloads', async () => {
      const { generateYamlManifest } = await import('../src/components/deployments/workloadInspectorYaml.js').catch(async () => {
        return await import('../src/components/deployments/workloadInspectorYaml.ts')
      })
      const daemonApp = {
        name: 'k8s-agent',
        type: 'daemon' as const,
        namespace: 'edge-tokyo-01',
        port: 9100,
        env: 'production',
        image: 'local-binary',
        replicas: 1,
        target: 'local',
        cpu: '100m',
        memory: '128Mi'
      }
      const manifest = generateYamlManifest(daemonApp as any)
      assert.match(manifest, /\[Unit\]/)
      assert.match(manifest, /\[Service\]/)
      assert.match(manifest, /ExecStart=.*k8s-agent/)
      assert.doesNotMatch(manifest, /apiVersion:\s*apps\/v1/)
    })

    it('WorkloadInspectorDrawer.vue handles daemon workload host endpoints and hides canary controls', () => {
      const drawer = readFile('components/deployments/WorkloadInspectorDrawer.vue')
      assert.match(drawer, /systemd/)
      assert.match(drawer, /\$\{app\.namespace\}:\$\{app\.port \|\| 9100\}/)
    })

    it('HostAddEditModal.vue contains copyable canonical ./deploy-agent.sh rollout command (LES-006)', () => {
      const modal = readFile('components/hosts/HostAddEditModal.vue')
      assert.match(modal, /\.\/deploy-agent\.sh/)
    })

    it('FleetImportModal.vue contains Validate Connectivity dry-run button', () => {
      const modal = readFile('components/fleet/FleetImportModal.vue')
      assert.match(modal, /Validate Connectivity/)
    })

    it('useLogStreamer / useLogStream passes since timestamp on resume', () => {
      const streamer = readFile('composables/useLogStreamer.ts')
      assert.match(streamer, /since/)
    })
  })

  describe('Criterion 6: Chrome QA Audit Catches & Finish Polish (DEF-001 - DEF-007, Emojis, SRE Strip)', () => {
    it('DEF-001: pwa-install-banner.css offsets bottom on mobile by 68px to avoid covering mobile nav', () => {
      const css = readFile('assets/styles/components/pwa-install-banner.css')
      assert.match(css, /@media\s*\(max-width:\s*767\.98px\)/)
      assert.match(css, /bottom:\s*calc\(68px\s*\+\s*env\(safe-area-inset-bottom,\s*0px\)\)/)
    })

    it('DEF-004: login.css ensures high contrast primary submit button in light mode', () => {
      const css = readFile('assets/styles/views/login.css')
      assert.match(css, /\[data-theme=['"]light['"]\]\s*\.login-btn/)
      assert.match(css, /var\(--color-primary/)
    })

    it('DEF-006: fleet-drawers.css prevents YAML textarea squishing in paste mode', () => {
      const css = readFile('assets/styles/components/fleet-drawers.css')
      assert.match(css, /\.text-input-wrap/)
      assert.match(css, /\.text-area-input\s*\{[^}]*width:\s*100%/)
    })

    it('DEF-007: infra-hosts-modal.css binds input-glass to theme variables', () => {
      const css = readFile('assets/styles/views/infra-hosts-modal.css')
      assert.match(css, /\.input-glass\s*\{[^}]*var\(--color-surface-inset\)/)
      assert.match(css, /\.input-glass\s*\{[^}]*var\(--text-primary\)/)
    })

    it('DEF-003: ActionDropdown.vue manages single active dropdown coordinator and uses canonical z-index', () => {
      const dropdown = readFile('components/ui/ActionDropdown.vue')
      assert.match(dropdown, /activeDropdownClose/)
      assert.match(dropdown, /var\(--z-dropdown,\s*30\)/)
    })

    it('ExplorerResourceTable.vue has scale action for deployments and statefulsets', () => {
      const table = readFile('components/explorer/ExplorerResourceTable.vue')
      assert.match(table, /props\.selectedKind === 'deployments'\s*\|\|\s*props\.selectedKind === 'statefulsets'/)
      assert.match(table, /id:\s*'scale'/)
    })

    it('ExplorerView.vue renders SRE resource summary strip', () => {
      const explorer = readFile('views/ExplorerView.vue')
      assert.match(explorer, /class="sre-summary-strip/)
      assert.match(explorer, /healthyCount/)
      assert.match(explorer, /degradedCount/)
    })

    it('HelmCatalogView.vue provides properly labeled single-row sync action', () => {
      const helm = readFile('views/HelmCatalogView.vue')
      assert.match(helm, /title="Sync repository index"/)
      assert.match(helm, /:disabled="updatingAllRepos"/)
    })

    it('Purged raw emojis from LogTargetTree, PodLogViewer, PodLogToolbar, EventsTimeline, and CyberDateTimePicker', () => {
      const targetTree = readFile('components/logs/LogTargetTree.vue')
      assert.doesNotMatch(targetTree, /[📦⚙️🔇]/)

      const podLogViewer = readFile('components/k8s/PodLogViewer.vue')
      assert.doesNotMatch(podLogViewer, /[⏸️⚠️🔄🛡️⬇️]/)

      const podLogToolbar = readFile('components/k8s/PodLogToolbar.vue')
      assert.doesNotMatch(podLogToolbar, /[📦🔴🟡🟢🔍✕⏸️⬇️↩️🔄📋💾🧹]/)

      const eventsTimeline = readFile('components/k8s/EventsTimeline.vue')
      assert.doesNotMatch(eventsTimeline, /[⚠️ℹ️🔍✕🔄📢🕒]/)

      const datePicker = readFile('components/ui/CyberDateTimePicker.vue')
      assert.doesNotMatch(datePicker, /[⏱️]/)
    })
  })
})
