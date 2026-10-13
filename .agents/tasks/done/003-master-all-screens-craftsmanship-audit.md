# 🎯 MASTER ALL-SCREENS CRAFTSMANSHIP AUDIT & RE-ENGINEERING PLAN

> **Location**: `.agents/tasks/inprocess/003-master-all-screens-craftsmanship-audit.md`  
> **Status**: COMPLETED (100% Verified in Loop 1 & Loop 2)  
> **Standard**: Production Enterprise Grade (Craftsmanship > Speed, Zero Toy Projects)  
> **Viewports Tested per Screen**: Desktop (1440x900), Tablet (768x1024), Mobile PWA (375x812)  
> **Pipeline per Screen**: `AUDIT / PLAN -> DISPATCH CODER (branch) -> REVIEWER -> QA AUDIT (MCP Screenshots) -> MERGE`

---

## 👑 SUPREME CRAFTSMANSHIP CONSTITUTION (BINDING PRODUCTION DIRECTIVE — ZERO COMPROMISES, ZERO SHORTCUTS)

> **MANDATORY USER DIRECTIVE**: *"Work with extreme care, granular attention to detail, and uncompromising quality. Do not rush or provide superficial 'check-the-box' patches to finish fast. Token consumption is irrelevant; enterprise production excellence is the sole metric of success."*  
> **MANDATORY LOOP 2 DIRECTIVE**: *"Once all 34 screens complete their first-pass overhaul and merge, immediately initiate Loop 2: a complete ground-up regression & stress verification across every single screen (S1–S34) to guarantee zero regressions, cross-screen link validity, and absolute production perfection."*

### 1. Eradicate "Hide instead of Adapt" (Zero Lazy Feature Hiding):
- Any screen featuring data visualizations (line charts, saturation splines, trend graphs, breakdown bars, gauges) on Desktop **MUST have a bespoke, responsive Mobile-First visualization** (e.g. compact SVG sparkline card ~120-140px, live pulse dot, compact time axis).
- **ABSOLUTE PROHIBITION** on setting `display: none !important;` on entire charts to artificially force mobile tests to pass. Any PR violating this = **INSTANT REJECT AND ROLLBACK**.

### 2. Mandatory 3-Tier Viewport Parity (Desktop, Tablet & Mobile):
- **Desktop (1440x900 / 1920x1080)**:
  - Data table occupies 100% available container width, `hasScroll: false` (zero horizontal scrollbar).
  - The rightmost Actions column must NEVER be cut off at screen edge.
  - Every action button (`Logs`, `Details`, `Scale`, `Restart`, `YAML`, `Delete`) must invoke real APIs, open real modals/drawers, and produce **0 console errors and 0 HTTP 500 errors**.
- **Tablet (768x1024 - iPad Standard)**:
  - HUD metric cards MUST auto-reorganize into a **balanced 2x2 grid** (`grid-template-columns: repeat(2, 1fr)`). Never squish 4 cards into 1 row.
  - Data tables must scale cleanly via `table-layout: fixed; width: 100%;` without overflowing horizontally (+300px blowout prohibited).
  - Sidebar transitions to off-canvas drawer to grant 100% screen real estate to active content.
- **Mobile (375x812 - iPhone Standard)**:
  - Command bar $\le 44\text{px}$ + Micro-telemetry strip $\le 20\text{px}$.
  - **Live SVG Saturation Line Chart** displaying CPU, RAM, and RPS telemetry over time.
  - High-density mobile card stream ~60–75px/item, displaying 4–5 workloads on the initial fold, touch targets $\ge 32\text{px}$.
  - Zero horizontal page overflow (`document.documentElement.scrollWidth === 375px`).

### 3. Eradicate "Button Suite Clutter" (Max 2 Inline Actions):
- Prohibit repeating 5–6 bulky text buttons across every row in tables (`Logs`, `Scale`, `Restart`, `YAML`, `Details`, `Delete`).
- Standardize enterprise action architecture:
  - **Maximum 2 primary inline buttons** (e.g. `📄 Logs`, `🔍 Details`).
  - All secondary operations MUST reside inside a sleek `[ ⋯ ]` dropdown menu (`.btn-more-actions`) or 30x30px compact icon buttons with tooltips.

### 4. 100% Verifiable Evidence Before Handoff:
- Coders must execute project verification (`npm run type-check && npm run build`) and self-inspect layout prior to handoff.
- Reviewers evaluate PRs against the **8-Point Adversarial Reviewer Checklist** (strict rejection for lazy feature suppression).
- QA Engineers MUST execute `chrome-devtools-mcp` across **ALL THREE VIEWPORTS (Desktop 1440x900, Tablet 768x1024, Mobile 375x812)**, click test real action buttons, and provide verbatim screenshot paths before declaring PASS.

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
| **S14** | `/compliance` | Compliance Governance | `ComplianceView.vue` | **VERIFIED & MERGED** ✅ | ✅ 100% Table & Matrix | ✅ 68px Card Stream | Commits `ceb3db1`, `adf3a99` |
| **S15** | `/drift` | Configuration Drift | `DriftView.vue` | **VERIFIED & MERGED** ✅ | ✅ 100% Table | ✅ Filter Sheet | Commit `4aee370` |
| **S16** | `/backup` | Backup & Disaster Recovery | `BackupRestoreView.vue` | **VERIFIED & MERGED** ✅ | ✅ 100% Table | ✅ Compact Cards | Commit `9f1a315` |
| **S17** | `/automation` | Remediation Automation | `AutomationView.vue` | **VERIFIED & MERGED** ✅ | ✅ 100% Table | ✅ Stream & Actions | Commit `98a57fc` |
| **S18** | `/runbooks` | Interactive Runbooks | `RunbooksView.vue` | **VERIFIED & MERGED** ✅ | ✅ 100% Table (Zero Overflow) | ✅ 85px Stream & 32px Buttons | Commits `12bf0b6`, `ee39393` |
| **S19** | `/cost` | FinOps & Cost Intelligence | `CostFinOpsView.vue` | **VERIFIED & MERGED** ✅ | ✅ 100% Table (Fixed Cols) | ✅ 68px Stream & Tabs | Commits `4b91f49`, `8acbb10` |
| **S20** | `/capacity` | Capacity & Scaling | `CapacityView.vue` | **VERIFIED & MERGED** ✅ | ✅ 100% Table (Zero Overflow) | ✅ Mobile SVG Trend & Drawer | Commits `934e78f` |
| **S21** | `/tenancy` | Multi-Tenancy & RBAC | `TenancyRbacView.vue` | **VERIFIED & MERGED** ✅ | ✅ 100% Table (Fixed Cols) | ✅ 72px Stream & Modals | Commits `9037f77`, `879db5a` |
| **S22** | `/agents` | Autonomous Agent Swarm | `AgentsView.vue` | **VERIFIED & MERGED** ✅ | ✅ 100% Table (Zero Overflow) | ✅ 44px Bar & DAG Carousel | Commits `f4515e3`, `5ca3ee9` |
| **S23** | `/ai-hub` | AI Provider Hub | `AIProviderHubView.vue` | **VERIFIED & MERGED** ✅ | ✅ 100% Table (Zero Overflow) | ✅ 42px Bar & Prompt Console | Commits `bd0ed5d`, `8750027` |
| **S24** | `/changes` | Change Management | `ChangesView.vue` | **VERIFIED & MERGED** ✅ | ✅ 100% Table | ✅ Stream & Actions | Commit `5b7fc67` |
| **S25** | `/alerts` | Alert Notification Center | `AlertsView.vue` | **VERIFIED & MERGED** ✅ | ✅ 100% Table | ✅ High-Density Stream | Commit `a5917b1` |
| **S26** | `/reports` | Reports & Intelligence | `ReportsView.vue` | **VERIFIED & MERGED** ✅ | ✅ 100% Table | ✅ High-Density Stream | Commit `12cf3ea` |
| **S27** | `/catalog` | Service Catalog | `ServiceCatalogView.vue` | **VERIFIED & MERGED** ✅ | ✅ 100% Table | ✅ 72px Card Stream | Commit `36fd625` |
| **S28** | `/scaffolder` | Developer Scaffolder | `ScaffolderView.vue` | **VERIFIED & MERGED** ✅ | ✅ 100% Grid | ✅ Wizard Stream | Commit `f291e29` |
| **S29** | `/ecosystem` | Cloud Native Ecosystem | `EcosystemView.vue` | **VERIFIED & MERGED** ✅ | ✅ 100% Tools Grid | ✅ 65px Tool Stream | Commit `0d80320` |
| **S30** | `/plugins` | Plugin Marketplace | `PluginsView.vue` | **VERIFIED & MERGED** ✅ | ✅ 100% Registry Grid | ✅ 68px Plugin Stream | Commit `219ef0d` |
| **S31** | `/settings` | System Settings | `SettingsView.vue` | **VERIFIED & MERGED** ✅ | ✅ 100% Config Cards | ✅ Responsive Tabs | Commit `b0fa6d1` |
| **S32** | `/settings/2fa` | TOTP & Security Setup | `TOTPSetupView.vue` | **VERIFIED & MERGED** ✅ | ✅ 220px QR & 48px PIN | ✅ 180px QR & 4-Step Stream | Commit `a49fd17` |
| **S33** | `/login` | Enterprise Authentication | `LoginView.vue` | **VERIFIED & MERGED** ✅ | ✅ Dual Hero Layout | ✅ Responsive Form Card | Commit `0b8b10a` |
| **S34** | `/workloads` | Workload Alias Redirect | `DeploymentsView.vue` | **VERIFIED & MERGED** ✅ | ✅ 100% Table | ✅ High-Density Stream | Commit `e7cfb3f` |

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
- 2026-09-10T14:24:00+07:00: S14 Compliance Governance verified and merged (commits `ceb3db1`, `adf3a99`).
- 2026-09-10T15:05:00+07:00: S15 Configuration Drift & Reconciliation verified and merged (commit `4aee370`).
- 2026-09-10T15:27:00+07:00: S16 Backup & Disaster Recovery verified and merged (commit `9f1a315`).
- 2026-09-10T15:52:00+07:00: S17 Remediation Automation verified and merged (commit `98a57fc`).
- 2026-09-10T16:17:00+07:00: S18 Interactive Runbooks active.
