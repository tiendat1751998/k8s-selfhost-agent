# 🎯 MASTER ALL-SCREENS CRAFTSMANSHIP AUDIT & RE-ENGINEERING PLAN

> **Location**: `.agents/tasks/inprocess/003-master-all-screens-craftsmanship-audit.md`  
> **Status**: IN_PROGRESS (Sequential Execution)  
> **Standard**: Production Enterprise Grade (Craftsmanship > Speed, Zero Toy Projects)  
> **Viewports Tested per Screen**: Desktop (1440x900), Tablet (768x1024), Mobile PWA (375x812)  
> **Pipeline per Screen**: `AUDIT / PLAN -> DISPATCH CODER (branch) -> REVIEWER -> QA AUDIT (MCP Screenshots) -> MERGE`

---

## 👑 SUPREME CRAFTSMANSHIP CONSTITUTION (CHỈ THỊ THI CÔNG TUYỆT ĐỐI — KHÔNG ĐỐI PHÓ, KHÔNG LÀM TẮT)

> **MỆNH LỆNH TỪ USER**: *"Làm việc cẩn thận, chi tiết, chất lượng cao, không làm cho có hay làm nhanh để đối phó. Không quan tâm tốn token, chỉ quan tâm sản phẩm chuẩn Enterprise thực sự."*

### 1. Triệt tiêu tư duy "Hide instead of Adapt" (Ẩn tính năng để né việc):
- Bất kỳ màn nào có biểu đồ (line chart, trend graphs, saturation splines, breakdown bars) trên Desktop thì **BẮT BUỘC phải thiết kế phiên bản Mobile-First tương ứng** (SVG sparkline card ~120-140px, live pulse dot, compact time axis).
- **CẤM TUYỆT ĐỐI** hành vi đặt `display: none !important;` lên toàn bộ component biểu đồ để né overflow trên mobile. Mọi PR vi phạm = **LẬP TỨC REJECT VÀ ROLLBACK**.

### 2. Tiêu chuẩn 3-Tier Viewports Bắt buộc (Không bỏ quên Tablet & Desktop):
- **Desktop (1440x900 / 1920x1080)**:
  - Bảng dữ liệu chiếm 100% độ rộng khả dụng, `hasScroll: false` (zero horizontal scrollbar).
  - Không được cắt cụt cột Actions ở mép phải màn hình.
  - Mọi nút bấm (`Logs`, `Details`, `Scale`, `Restart`, `YAML`, `Delete`) phải gọi API thật, mở drawer/modal thật, **0 lỗi console, 0 lỗi HTTP 500**.
- **Tablet (768x1024 - iPad Standard)**:
  - Cụm HUD Cards tự động chia thành **lưới 2x2 cân xứng** (`grid-template-columns: repeat(2, 1fr)`), không được ép dồn 4 cột làm vỡ layout.
  - Bảng dữ liệu tự động co giãn (`table-layout: fixed; width: 100%;`), không tràn viền ngang +300px.
  - Sidebar chuyển thành off-canvas drawer để dành 100% diện tích cho nội dung.
- **Mobile (375x812 - iPhone Standard)**:
  - Command bar $\le 44\text{px}$ + Micro-telemetry $\le 20\text{px}$.
  - **Live SVG Line Chart / Saturation Spline** sắc nét, hiển thị CPU, RAM, RPS theo thời gian.
  - Mobile Card Stream ~60–75px/item, hiển thị 4–5 workloads ngay trên màn hình đầu tiên, touch targets $\ge 32\text{px}$.
  - Zero horizontal overflow (`document.documentElement.scrollWidth === 375px`).

### 3. Xóa bỏ triệt để "Hội chứng Spam Nút Bấm" (Button Suite Clutter):
- Cấm lặp lại 5–6 nút text to đùng trên mỗi hàng (`Logs`, `Scale`, `Restart`, `YAML`, `Details`, `Delete`).
- Chuẩn hóa kiến trúc nút bấm Enterprise:
  - **Tối đa 2 nút chính inline** (ví dụ: `📄 Logs`, `🔍 Details`).
  - Toàn bộ hành động phụ gom vào nút menu `[ ⋯ ]` (`.btn-more-actions`) tinh tế hoặc bộ icon 30px có tooltip.

### 4. Kiểm toán Thật 100% Trước Khi Báo Cáo (Evidence Over Claims):
- Coder phải tự chạy `npm run type-check && npm run build` và kiểm tra layout trước khi handoff.
- Reviewer duyệt theo **8-Point Adversarial Checklist** (đặc biệt điểm số 8: Anti-Lazy & Feature Parity).
- QA bắt buộc dùng `chrome-devtools-mcp` chụp ảnh màn hình và đo đạc kích thước thực tế trên **cả 3 viewports: Desktop, Tablet, Mobile** trước khi xác nhận PASS.

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
- 2026-09-10T14:24:00+07:00: S14 Compliance Governance verified and merged (commits `ceb3db1`, `adf3a99`).
- 2026-09-10T15:05:00+07:00: S15 Configuration Drift & Reconciliation verified and merged (commit `4aee370`).
- 2026-09-10T15:27:00+07:00: S16 Backup & Disaster Recovery verified and merged (commit `9f1a315`).
- 2026-09-10T15:52:00+07:00: S17 Remediation Automation verified and merged (commit `98a57fc`).
- 2026-09-10T16:17:00+07:00: S18 Interactive Runbooks active.
