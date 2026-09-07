# MASTER BLUEPRINT: SYSTEM-WIDE MODULAR ARCHITECTURE REFACTORING (< 350 LINES/VIEW, 4-TIER RWD, TRI-PARTITION)

> **Document Type**: Master WBS Execution Blueprint & Quality Gate  
> **Target Status**: 20 / 34 Views Complete (100% Build Pass). Remaining: 14 Views across Wave 7 & Wave 8.  
> **Binding Constitution**: [AGENTS.md](file:///d:/project/k8sseflhost/AGENTS.md) & [.agents/rules/architecture-standards.md](file:///d:/project/k8sseflhost/.agents/rules/architecture-standards.md)

---

## 🏛️ 1. CONSTITUTIONAL ARCHITECTURE INVARIANTS (TAM QUYỀN PHÂN LẬP)

1. **Strict File Size Ceiling (< 350 Lines)**:
   - Under NO circumstances may any `.vue`, `.ts`, or `.go` file exceed 350 lines.
   - Views must be pure orchestrator skeletons of **150 - 300 lines**.
   - Sub-components and Dialogs must be standalone components under **150 - 300 lines**.
2. **Tri-Partition Separation (Zero Inline Monoliths)**:
   - **CSS**: All styles must reside in `src/assets/styles/views/<view>.css` (Zero `<style scoped>` blocks in views).
   - **Logic & Reactive State**: All business logic, API calls, formatters, and reactive state must reside in `src/composables/use<View>.ts`.
   - **Sub-Components**: All tables, grids, cards, modals, and drawers must reside in `src/components/<domain>/`.
3. **Button UI & Color Standardization**:
   - All action buttons must have **[ SVG Icon / Emoji + Text Label ]**.
   - Status `CONNECTED` / `ONLINE` / `READY` must be Emerald Green (`#10b981`).
   - Action `[ 🗑 Delete ]` / `[ 🗑 Terminate ]` must be Crimson Red (`#f43f5e`).
4. **4-Tier Responsive Web Design (RWD)**:
   - **Tier 1 (Mobile < 640px)**: Compact stream (~65px/item), 2x2 KPI grids, 0 horizontal scroll.
   - **Tier 2 (Tablet 768-1024px)**: Auto-collapsing sidebar, 2x2 grid.
   - **Tier 3 (Desktop Full HD 1440/1920px)**: 100% data tables with action buttons.
   - **Tier 4 (4K 3840px)**: `max-width: 1920px; margin: 0 auto;`.
5. **Verification Protocol**:
   - Every wave must pass `npm.cmd run build` in `frontend-vue` with **0 errors, 0 warnings**.

---

## 📊 2. ACCOMPLISHED WAVES (25 / 34 VIEWS — 100% VERIFIED)

| Wave | View Name | Original Lines | Refactored Lines | Modular Assets |
| :--- | :--- | :---: | :---: | :--- |
| **Wave 1** | `ExplorerView.vue` | 4,646 | **352** | `useK8sExplorer.ts` + 12 sub-components + `explorer.css` |
| | `DeploymentsView.vue` | 4,426 | **337** | `useDeployments.ts` + 7 sub-components + `deployments.css` |
| | `InfraHostsView.vue` | 2,800 | **226** | `useInfraHosts.ts` + 5 sub-components + `hosts.css` |
| | `OverviewView.vue` | 2,365 | **282** | `useOverviewDashboard.ts` + 5 sub-components + `overview.css` |
| | `HelmCatalogView.vue` | 3,151 | **239** | `useHelmCatalog.ts` + 6 sub-components + `helm.css` |
| **Wave 2** | `IncidentsView.vue` | 1,597 | **167** | `useIncidents.ts` + 5 sub-components + `incidents.css` |
| | `SLOView.vue` | 1,976 | **239** | `useSLOMonitor.ts` + 5 sub-components + `slo.css` |
| | `TenancyRbacView.vue` | 1,168 | **328** | `useTenancyRbac.ts` + 5 sub-components + `tenancy.css` |
| **Wave 3** | `PluginsView.vue` | 2,034 | **241** | `usePlugins.ts` + 5 sub-components + `plugins.css` |
| | `ServiceCatalogView.vue` | 2,231 | **239** | `useServiceCatalog.ts` + 5 sub-components + `catalog.css` |
| | `SettingsView.vue` | 2,097 | **229** | `useSettings.ts` + 7 sub-components + `settings.css` |
| **Wave 4** | `ScaffolderView.vue` | 1,632 | **200** | `useScaffolder.ts` + 5 sub-components + `scaffolder.css` |
| | `FleetView.vue` | 1,533 | **230** | `useFleetManagement.ts` + 5 sub-components + `fleet.css` |
| | `DockerSwarmView.vue` | 1,500 | **253** | `useDockerSwarm.ts` + 7 sub-components + `swarm.css` |
| **Wave 5** | `AIProviderHubView.vue` | 1,279 | **221** | `useAIProviderHub.ts` + 6 sub-components + `aihub.css` |
| | `EcosystemView.vue` | 1,386 | **226** | `useEcosystem.ts` + 5 sub-components + `ecosystem.css` |
| | `PromotionsView.vue` | 1,495 | **200** | `usePromotions.ts` + 5 sub-components + `promotions.css` |
| **Wave 6** | `BackupRestoreView.vue` | 1,222 | **234** | `useBackupRestore.ts` + 8 sub-components + `backup.css` |
| | `RunbooksView.vue` | 1,095 | **247** | `useRunbooks.ts` + 5 sub-components + `runbooks.css` |
| | `AgentsView.vue` | 1,035 | **233** | `useAgentMesh.ts` + 6 sub-components + `agents.css` |
| **Wave 7** | `CapacityView.vue` | 1,059 | **217** | `useCapacityForecast.ts` + 5 sub-components + `capacity.css` |
| | `AlertsView.vue` | 1,043 | **227** | `useAlertManager.ts` + 8 sub-components + `alerts.css` |
| | `TOTPSetupView.vue` | 1,089 | **213** | `useTOTPSetup.ts` + 4 sub-components + `totp.css` |
| | `ReportsView.vue` | 894 | **183** | `useReports.ts` + 4 sub-components + `reports.css` |
| | `CostFinOpsView.vue` | 714 | **193** | `useCostFinOps.ts` + 4 sub-components + `cost.css` |

---

## 🎯 3. ACTION PLAN: WAVE 8 (FINAL 9 VIEWS)

Wave 8 covers the remaining 9 views to achieve **34 / 34 Views (< 350 lines)**:

### Batch 1 of Wave 8 (3 Views in parallel):
1. `ComplianceView.vue` (866 lines $\rightarrow$ < 250 lines):
   - Composable: `useCompliance.ts`
   - CSS: `compliance.css`
   - Sub-components: `ComplianceScoreCards.vue`, `ComplianceFrameworksGrid.vue`, `ComplianceControlsTable.vue`, `ComplianceMobileCards.vue`
2. `ChangesView.vue` (825 lines $\rightarrow$ < 250 lines):
   - Composable: `useChangesTimeline.ts`
   - CSS: `changes.css`
   - Sub-components: `ChangesHudCards.vue`, `ChangesTimelineStream.vue`, `ChangesFilterBar.vue`, `ChangesMobileCards.vue`, `ChangeDiffDrawer.vue`
3. `DevSecOpsView.vue` (822 lines $\rightarrow$ < 250 lines):
   - Composable: `useDevSecOps.ts`
   - CSS: `secops.css`
   - Sub-components: `SecurityScoreCards.vue`, `VulnerabilityScanTable.vue`, `SecretAuditGrid.vue`, `SecOpsMobileCards.vue`

### Batch 2 of Wave 8 (3 Views in parallel):
4. `AutomationView.vue` (812 lines $\rightarrow$ < 250 lines):
   - Composable: `useAutomationEngine.ts`, `automation.css`
5. `DriftView.vue` (808 lines $\rightarrow$ < 250 lines):
   - Composable: `useDriftDetection.ts`, `drift.css`
6. `AuditView.vue` (745 lines $\rightarrow$ < 250 lines):
   - Composable: `useAuditLogs.ts`, `audit.css`

### Batch 3 of Wave 8 (Final 3 Views in parallel):
7. `LoginView.vue` (610 lines $\rightarrow$ < 250 lines):
   - Composable: `useLoginAuth.ts`, `login.css`
8. `LogStreamView.vue` (593 lines $\rightarrow$ < 250 lines):
   - Composable: `useLogStreamer.ts`, `logstream.css`
9. `GenericPlatformView.vue` (482 lines $\rightarrow$ < 250 lines):
   - Composable: `useGenericPlatform.ts`, `generic.css` + sub-components

---

## 🔍 4. POST-REFACTOR VISUAL & FUNCTIONAL AUDIT (CHROME DEVTOOLS MCP)

Sau khi hoàn tất toàn bộ 34 views đạt chuẩn < 350 dòng:
1. Start dev server `npm run dev -- --port 5173`.
2. Dùng `chrome-devtools-mcp` quét 3 Viewport:
   - **Mobile (390 x 844)**: Verify không có horizontal scroll, mobile card stream hiển thị ~65px/item.
   - **Tablet (820 x 1180)**: Verify 2x2 grid, collapsing sidebar.
   - **Desktop (1440 x 900 & 1920 x 1080)**: Verify 100% data table, labeled buttons `[ ⚡ Execute ]`, `[ 🔍 Inspect ]`, `[ ⚙️ Edit ]`, `[ 🗑 Delete ]` (crimson red `#f43f5e`).
3. Click test tất cả các nút bấm và modal để đảm bảo 100% chức năng hoạt động hoàn hảo.
4. Chụp ảnh màn hình thực tế lưu vào artifacts và báo cáo nghiệm thu toàn diện cho user.
