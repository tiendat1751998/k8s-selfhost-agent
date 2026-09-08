# Task 014: K8s Explorer Mobile PWA Overhaul, Monolith Shredding & Button Text Fix

**Created**: 2026-09-08T13:20:00+07:00  
**Completed**: 2026-09-08T13:38:00+07:00  
**Status**: COMPLETED  
**Owner**: Orchestrator (tiendat1751998/k8s-selfhost-agent)  
**Supreme Directive**: Craftsmanship > Speed (AGENTS.md §00). Proactive MCP visual audits across all breakpoints. Zero user manual QA.

## Objective & User Bug Reproduction
User reported mobile screen on `/explorer` (`media_1788848064611.png`):
1. **Truncated Button Bug**: Desktop header `.header-actions` displays 3 verbose buttons on mobile, causing `+ Create Deployment` to be clipped to `+ Create Deploym`.
2. **Screen Real Estate Cannibalization**: 3 giant `MetricCard`s (`Total Deployments`, `Active Namespaces`, `Cluster Target`) stack vertically on mobile (<640px), eating up 350px+ of height. Workloads (`cilium-operator`, etc.) are buried off the bottom of the screen.
3. **Monolithic Invariant Violation**: `ExplorerView.vue` was **4,375 lines** (AGENTS.md invariant strictly mandates `<500 lines` per file, views 150-350 lines).

## Work Breakdown Structure (WBS)
- [x] Wave 1: Shred `ExplorerView.vue` Monolith (< 350 lines)
  - [x] Reduced `ExplorerView.vue` from 4,375 lines down to 333 lines.
  - [x] Connected `useK8sExplorer.ts` composable for cluster, namespace, kind, and resource telemetry.
  - [x] Wired extracted subcomponents: `ExplorerSidebar`, `ExplorerResourceTable`, `ExplorerMobileCards`, `ExplorerDetailDrawer`, `ExplorerCreateModal`, `ExplorerScaleModal`, `ExplorerRestartModal`, `ExplorerDeleteModal`, `ExplorerDrainModal`, `ApplyYamlModal`, `ExplorerImportModal`, `ExplorerCreateNsModal`, `PodLogsDrawer`, `PodTerminalDrawer`.
- [x] Wave 2: Mobile-First PWA 4-Tier RWD Ergonomics (<640px)
  - [x] Hide bulky desktop `.desktop-header-wrap` containing `.view-header` and 3 stacked `.metrics-grid` cards on `<640px`.
  - [x] Added 44px Mobile Command Bar: `☸️ {{ currentKindLabel }} ({{ totalInKind }})` + normalized 32px icon buttons (`[ 🔍 ] [ 🔄 ] [ 📄 ] [ ➕ ] [ ☰ ]`) with zero text clipping.
  - [x] Added 20px micro-telemetry strip: `🌐 {{ selectedCluster }} · 📁 {{ selectedNamespace }} · 📦 {{ totalInKind }} {{ currentKindLabel }}`.
  - [x] Added 36px horizontal kind pill scroller for quick 1-touch switching between Pods, Deployments, Services, ConfigMaps, Nodes.
  - [x] Mounted `ExplorerMobileCards.vue` directly on Screen 1 so all 4 workloads (`cilium-operator`, `coredns`, `hubble-relay`, `hubble-ui`) are immediately visible on Screen 1 without scrolling!
- [x] Wave 3: Desktop & Tablet Harmony (>= 640px)
  - [x] Tablet (768px-1024px): 3 KPI cards configured in single horizontal row (`repeat(3, 1fr)`) with all 4 workload cards fitting on Screen 1.
  - [x] Desktop (1440x900): Full 290px main navigation sidebar, 270px resource categories tree, full breadcrumbs, and interactive `DataTable`.
- [x] Wave 4: Autonomous Proactive Verification
  - [x] `cmd.exe /c "npm run build"`: Exit code 0, 0 TS errors, built in 3.25s.
  - [x] Chrome DevTools MCP Mobile (375x812): Zero button clipping, all 4 workloads on Screen 1.
  - [x] Chrome DevTools MCP Tablet (768x1024): 3 KPI cards in single row + 4 workload cards visible.
  - [x] Chrome DevTools MCP Desktop (1440x900): Full HD Enterprise layout verified.
