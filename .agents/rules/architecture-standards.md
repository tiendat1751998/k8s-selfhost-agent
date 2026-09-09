# ARCHITECTURE-STANDARDS.md — Binding Architectural Constitution

> **STATUS**: MANDATORY & UNIVERSALLY BINDING ACROSS ALL SESSIONS AND SUBAGENTS.
> **VIOLATION POLICY**: Zero tolerance. Any file exceeding 500 lines or mixing monolithic CSS/scripts is rejected immediately.

---

## 🏛️ 1. FRONTEND MODULARITY & TAM QUYỀN PHÂN LẬP (< 500 LINES PER FILE)

Mọi tệp nguồn Frontend Vue 3 / TypeScript bắt buộc tuân thủ nguyên tắc **Tam Quyền Phân Lập (Triệt để tách CSS - JS/TS Logic - UI Template)**:

`
frontend-vue/src/
├── 📂 assets/styles/                 # 1. CSS TÁCH BIỆT HOÀN TOÀN (Zero Fat Scoped CSS)
│   ├── base.css                      # Design tokens, colors, typography, reset
│   ├── buttons.css                   # Chuẩn nút [ 📄 Logs ] [ ⚡ Scale ] [ 🗑 Delete ]
│   ├── tables.css                    # Data table, row actions, hover states
│   ├── modals.css                    # Modals, Drawers, Backdrop, Tooltips
│   ├── responsive.css                # 4-Tier RWD tập trung (Mobile, Tablet, Desktop, 4K)
│   └── views/                        # Scoped CSS riêng của từng màn hình
│
├── 📂 composables/                   # 2. LOGIC JS/TS TÁCH BIỆT (Single Responsibility)
│   ├── useK8sExplorer.ts             # State, API fetch, Kind categories, Filter
│   ├── useDeployments.ts             # Workload normalization, Canary split, Scale
│   ├── useInfraHosts.ts              # Real ping probes, Add/Edit host, Health
│   └── useWebSocketStream.ts         # Live pod logs, Terminal WebSockets
│
├── 📂 components/<domain>/           # 3. MODALS / DRAWERS CHUYÊN BIỆT (< 150 - 300 dòng)
│   ├── explorer/                     # (ExplorerCommandBar, ApplyYamlModal, PodLogsDrawer...)
│   ├── deployments/                  # (DeploymentsMobileCards, CanaryStrategyModal...)
│   └── hosts/                        # (HostsMobileCards, AddEditHostModal, HostDetailDrawer...)
│
└── 📂 views/                         # 4. VIEW CHÍNH SIÊU GỌN GÀNG (150 - 350 DÒNG)
    ├── ExplorerView.vue              # Orchestrator layout only (< 400 dòng)
    ├── DeploymentsView.vue           # Orchestrator layout only (< 350 dòng)
    └── InfraHostsView.vue            # Orchestrator layout only (< 350 dòng)
`

---

## 📱 2. CHUẨN 4-TIER RWD & MOBILE-FIRST PWA (ZERO-WASTE VIEWPORT MATRIX)

Mọi view bắt buộc tương thích hoàn hảo trên **4 tầng màn hình**:
1. **Tier 1: Mobile (< 640px / 390x844)**:
   - Ẩn 100% Header to, banner cồng kềnh, và các thẻ KPI xếp chồng chiếm 400-800px.
   - Command Bar siêu gọn (~48px) chứa tên tài nguyên + số lượng + trạng thái + nút hành động.
   - Dữ liệu hiển thị dạng **Mobile Card Stream (~65-75px/card)**, hiển thị 4-5 workloads trên màn hình đầu tiên, **Zero horizontal overflow scroll**.
2. **Tier 2: Tablet (768px - 1024px / 820x1180)**:
   - Sidebar tự động thu gọn 64px icon-only, KPI grid 2x2 cân xứng.
3. **Tier 3: Desktop Full HD (1440x900 / 1920x1080)**:
   - Single left sidebar 240px (không double sidebar), KPI 4 thẻ hàng ngang, bảng dữ liệu 100% kèm action buttons [ Icon + Nhãn Chữ + Màu Sắc Độc Bản ] (Delete đỏ #f43f5e).
4. **Tier 4: Ultra-Wide & 4K (2560x1440 / 3840x2160)**:
   - Khung chứa max-width: 1920px; margin: 0 auto;, typography scale chuẩn rem.

---

## ⚙️ 3. BACKEND CLEAN & HEXAGONAL ARCHITECTURE, SOLID & ACID (GO)

Backend Go bắt buộc tuân thủ 4 lớp độc lập:
1. **internal/domain/ (Inward Core)**: Pure business entities, value objects, domain errors, zero framework/db dependencies.
2. **internal/usecase/ (Application Layer)**: Business logic orchestration, transactional boundaries (TxManager).
3. **internal/adapter/http/ (Primary Adapters)**: REST Handlers, HTTP middleware (RBAC, JWT, Audit), JSON serialization.
4. **internal/infrastructure/ (Secondary Adapters / Ports Implementation)**: PostgreSQL repositories, K8s client-go, Docker SDK, real probes on port 9100.
5. **Zero Mock / Zero Fake Telemetry**: Cấm tuyệt đối fake Math.random() hoặc hardcoded latency. Đo đúng telemetry thực tế.
