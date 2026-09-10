# 🎯 MASTER ALL-SCREENS CRAFTSMANSHIP AUDIT & RE-ENGINEERING PLAN

> **Location**: `.agents/tasks/inprocess/003-master-all-screens-craftsmanship-audit.md`  
> **Status**: IN_PROGRESS (Sequential Execution)  
> **Standard**: Production Enterprise Grade (Craftsmanship > Speed, Zero Toy Projects)  
> **Viewports Tested per Screen**: Desktop (1440x900), Tablet (768x1024), Mobile PWA (375x812)  
> **Pipeline per Screen**: `AUDIT / PLAN -> DISPATCH CODER (branch) -> REVIEWER -> QA AUDIT (MCP Screenshots) -> MERGE`

---

## 📐 BINDING CRAFTSMANSHIP CRITERIA (THE 5 NON-NEGOTIABLES)

1. **Zero Desktop Table Leakage on Mobile**:
   - Every `<table>` or `.desktop-table-view` MUST have `display: none !important;` on screens `<768px`.
   - Never allow background borders, text, or horizontal scrollbars to leak through on touch devices.

2. **Mobile Header Height < 50px**:
   - Single clean command bar + search / filter toggle.
   - Prohibit stacking 5-6 header bars (Search + Filter + Status + Telemetry + Badges).

3. **High-Density Mobile Card Streams (~60-75px/item)**:
   - High information density displaying 4-5 workloads/nodes on the initial viewport without scrolling.
   - No 6-button card monsters. Quick actions limited to 1-2 essential buttons (`📜 Logs`, `⋯ Actions`). Tapping the card or `⋯` opens the full Inspector Drawer / Action Sheet.

4. **Table-First on Desktop (1440x900)**:
   - 100% available container width with zero cramped double sidebars and zero unnecessary horizontal scrollbars.

5. **Visual Sanity & Zero Clutter**:
   - Prohibit neon badge storms (no 7-pill color explosions). Use subtle slate/neutral badges.
   - Sanitize all typos (`k8smater` -> `k8smaster`).

---

## 📋 34-SCREEN AUDIT & EXECUTION MATRIX

| # | Route | Screen Name | Component | Status | Desktop 1440 | Mobile 375 | Notes / Defects |
|---|-------|-------------|-----------|--------|--------------|------------|-----------------|
| **G1** | Global | Command Palette & Nav | `AppCommandPalette.vue` | **VERIFIED & MERGED** ✅ | ✅ Clean | ✅ BottomSheet | Commits `84ba699`, `18f6aa1` |
| **S1** | `/` | Cluster Overview | `OverviewView.vue` | **VERIFIED & MERGED** ✅ | ✅ 100% Desktop | ✅ High-Density Stream | Commits `3358de8`, `17141a1` |
| **S2** | `/explorer` | K8s Resource Explorer | `ExplorerView.vue` | **VERIFIED & MERGED** ✅ | ✅ 100% Table | ✅ Clean Bar | Commits `1258069`, `3111818` |
| **S3** | `/hosts` | Infrastructure Hosts | `InfraHostsView.vue` | **VERIFIED & MERGED** ✅ | ✅ 100% Table | ✅ Cards Stream | Commits `96fedb0`, `d048e9b` |
| **S4** | `/incidents` | Incident Center | `IncidentsView.vue` | **VERIFIED & MERGED** ✅ | ✅ Compact | ✅ Full Header | Commit `2e954b4` |
| **S5** | `/logs` | Real-Time Logs Explorer | `LogStreamView.vue` | **VERIFIED & MERGED** ✅ | ✅ RingBuffer | ✅ Truncate Safe | Commit `d8edd56` |
| **S6** | `/fleet` | Fleet Manager | `FleetView.vue` | **VERIFIED & MERGED** ✅ | ✅ Toggle Mode | ✅ Card Stream | Commit `b92538c` |
| **S7** | `/deployments` | Workload Deployments | `DeploymentsView.vue` | **VERIFIED & MERGED** ✅ | ✅ 100% Table | ✅ High-Density Stream | Commits `e5f9b81`, `e7cfb3f` |
| **S8** | `/slo` | SLOs & Error Budgets | `SLOView.vue` | **VERIFIED & MERGED** ✅ | ✅ 100% Table | ✅ Touch Stream | Commit `9e6b2e0` |
| **S9** | `/promotions` | GitOps Promotions | `PromotionsView.vue` | **VERIFIED & MERGED** ✅ | ✅ 100% Table | ✅ High-Density Stream | Commits `51a6dc1`, `dadeffb` |
| **S10** | `/docker` | Docker Swarm | `DockerSwarmView.vue` | **VERIFIED & MERGED** ✅ | ✅ 100% Table (Zero Scroll) | ✅ 68px Card Stream | Commits `16745f7`, `008a30c`, `e364be4` |
| **S11** | `/helm` | Helm Apps & Charts | `HelmCatalogView.vue` | **VERIFIED & MERGED** ✅ | ✅ 100% Table | ✅ 88px Card Stream | Commit `8866b55` |
| **S12** | `/audit` | Security Audit Findings | `AuditView.vue` | **VERIFIED & MERGED** ✅ | ✅ 100% Table (Live API) | ✅ 77px Card Stream | Commits `918f762`, `e1dde34`, `879e9d9` |
| **S13** | `/security` | DevSecOps Pipeline | `DevSecOpsView.vue` | **VERIFIED & MERGED** ✅ | ✅ 100% Table & Matrix | ✅ 65px Card Stream | Commit `1bbc543` |
| **S14** | `/compliance` | Compliance Governance | `ComplianceView.vue` | **QUEUED** ⏳ | Pending | Pending | |
| **S15** | `/drift` | Configuration Drift | `DriftView.vue` | **QUEUED** ⏳ | Pending | Pending | |
| **S16** | `/backup` | Backup & Disaster Recovery | `BackupRestoreView.vue` | **QUEUED** ⏳ | Pending | Pending | |
| **S17** | `/automation` | Remediation Automation | `AutomationView.vue` | **QUEUED** ⏳ | Pending | Pending | |
| **S18** | `/runbooks` | Interactive Runbooks | `RunbooksView.vue` | **QUEUED** ⏳ | Pending | Pending | |
| **S19** | `/cost` | FinOps & Cost Intelligence | `CostFinOpsView.vue` | **QUEUED** ⏳ | Pending | Pending | |
| **S20** | `/capacity` | Capacity & Scaling | `CapacityView.vue` | **QUEUED** ⏳ | Pending | Pending | |
| **S21** | `/tenancy` | Multi-Tenancy & RBAC | `TenancyRbacView.vue` | **QUEUED** ⏳ | Pending | Pending | |
| **S22** | `/agents` | Autonomous Agent Swarm | `AgentsView.vue` | **QUEUED** ⏳ | Pending | Pending | |
| **S23** | `/ai-hub` | AI Provider Hub | `AIProviderHubView.vue` | **QUEUED** ⏳ | Pending | Pending | |
| **S24** | `/changes` | Change Management | `ChangesView.vue` | **QUEUED** ⏳ | Pending | Pending | |
| **S25** | `/alerts` | Alert Notification Center | `AlertsView.vue` | **QUEUED** ⏳ | Pending | Pending | |
| **S26** | `/reports` | Reports & Intelligence | `ReportsView.vue` | **QUEUED** ⏳ | Pending | Pending | |
| **S27** | `/catalog` | Service Catalog | `ServiceCatalogView.vue` | **QUEUED** ⏳ | Pending | Pending | |
| **S28** | `/scaffolder` | Developer Scaffolder | `ScaffolderView.vue` | **QUEUED** ⏳ | Pending | Pending | |
| **S29** | `/ecosystem` | Cloud Native Ecosystem | `EcosystemView.vue` | **QUEUED** ⏳ | Pending | Pending | |
| **S30** | `/plugins` | Plugin Marketplace | `PluginsView.vue` | **QUEUED** ⏳ | Pending | Pending | |
| **S31** | `/settings` | System Settings | `SettingsView.vue` | **QUEUED** ⏳ | Pending | Pending | |
| **S32** | `/settings/2fa` | TOTP & Security Setup | `TOTPSetupView.vue` | **QUEUED** ⏳ | Pending | Pending | |
| **S33** | `/login` | Enterprise Authentication | `LoginView.vue` | **QUEUED** ⏳ | Pending | Pending | |
| **S34** | `/workloads` | Workload Alias Redirect | `DeploymentsView.vue` | **QUEUED** ⏳ | Pending | Pending | |

---

## 🚀 CURRENT WORK IN PROGRESS (IMMEDIATE DEFECT SPRINT)

### Sprint 1: S7 Workload Deployments Mobile Overhaul (`/deployments`)
- **Target Files**:
  - `frontend-vue/src/views/DeploymentsView.vue`
  - `frontend-vue/src/components/deployments/DeploymentsMobileCards.vue`
  - `frontend-vue/src/assets/styles/views/deployments.css`
- **Defects Addressed (User Image 2)**:
  1. Complete suppression of desktop table on `<768px` (`display: none !important;`).
  2. Collapsing 6-layer stacked header into a sleek 44px compact bar + slide-down filter drawer/sheet.
  3. Re-engineering mobile cards: compact 72px items with Name, Status dot, Replicas, Namespace, `[📜 Logs]` + `[⋯]` actions.
- **Verification Gate**:
  - `npm --prefix frontend-vue run type-check && npm --prefix frontend-vue run build`
  - QA inspector: Desktop 1440x900 & Mobile 375x812 screenshots.

### Sprint 2: S1 Overview Mobile Re-Architecture (`/`)
- **Target Files**:
  - `frontend-vue/src/views/OverviewView.vue`
  - `frontend-vue/src/components/overview/OverviewMobileStream.vue`
  - `frontend-vue/src/assets/styles/views/overview-saturation-trends.css`
- **Defects Addressed (User Image 1)**:
  1. Complete removal of desktop table forced on mobile; replace with responsive high-density card stream (~56-64px/node).
  2. Sanitize typo `k8smater` -> `k8smaster`.
  3. Ensure 0 horizontal overflow (`scrollWidth === 375px`).
- **Verification Gate**:
  - `npm --prefix frontend-vue run type-check && npm --prefix frontend-vue run build`
  - QA inspector: Mobile 375x812 screenshot showing high density stream.

---

## 📈 PIPELINE AUDIT LOG
- 2026-09-10T10:58:00+07:00: S7 K8s Pod Logs 500 error eliminated and merged (commit `e5f9b81`).
- 2026-09-10T11:20:00+07:00: S7 Workload Deployments Mobile Overhaul verified and merged (commit `e7cfb3f`).
- 2026-09-10T11:45:00+07:00: S1 Overview Mobile Re-Architecture verified and merged (commit `17141a1`).
- 2026-09-10T12:00:00+07:00: S8 SLOs & Error Budgets verified and merged (commit `9e6b2e0`).
- 2026-09-10T12:15:00+07:00: S9 GitOps Promotions verified and merged (commit `dadeffb`).
- 2026-09-10T12:44:00+07:00: S10 Docker Swarm Manager verified and merged (commits `16745f7`, `008a30c`, `e364be4`).
- 2026-09-10T13:12:00+07:00: S11 Helm Apps & Charts verified and merged (commit `8866b55`).
- 2026-09-10T13:58:00+07:00: S12 Security Audit Findings verified and merged (commits `918f762`, `e1dde34`, `879e9d9`).
- 2026-09-10T14:23:00+07:00: S13 DevSecOps Pipeline verified and merged (commit `1bbc543`).
- 2026-09-10T14:24:00+07:00: S14 Compliance Governance active.
