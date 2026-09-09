# ARCHITECTURE-STANDARDS.md — Binding Architectural Constitution

> **STATUS**: MANDATORY & UNIVERSALLY BINDING ACROSS ALL SESSIONS AND SUBAGENTS.
> **VIOLATION POLICY**: Zero tolerance. Any file exceeding 500 lines or mixing monolithic CSS/scripts is rejected immediately.

---

## 🏛️ 1. FRONTEND MODULARITY & SEPARATION OF CONCERNS (< 500 LINES PER FILE)

Every Vue 3 / TypeScript source file MUST strictly adhere to the **Tripartite Separation of Concerns (CSS - Logic - Template)**:

```text
frontend-vue/src/
├── 📂 assets/styles/                 # 1. COMPLETELY SEGREGATED CSS (Zero Fat Scoped CSS)
│   ├── base.css                      # Design tokens, colors, typography, reset
│   ├── buttons.css                   # Standard buttons: [ 📄 Logs ] [ ⚡ Scale ] [ 🗑 Delete ]
│   ├── tables.css                    # Data tables, row actions, hover states
│   ├── modals.css                    # Modals, Drawers, Backdrops, Tooltips
│   ├── responsive.css                # Centralized 4-Tier RWD (Mobile, Tablet, Desktop, 4K)
│   └── views/                        # Scoped CSS for individual views
│
├── 📂 composables/                   # 2. SEPARATED JS/TS LOGIC (Single Responsibility)
│   ├── useK8sExplorer.ts             # State, API fetch, Kind categories, Filter
│   ├── useDeployments.ts             # Workload normalization, Canary split, Scale
│   ├── useInfraHosts.ts              # Real ping probes, Add/Edit host, Health
│   └── useWebSocketStream.ts         # Live pod logs, Terminal WebSockets
│
├── 📂 components/<domain>/           # 3. SPECIALIZED MODALS & DRAWERS (< 150 - 300 lines)
│   ├── explorer/                     # ExplorerCommandBar, ApplyYamlModal, PodLogsDrawer...
│   ├── deployments/                  # DeploymentsMobileCards, CanaryStrategyModal...
│   └── hosts/                        # HostsMobileCards, AddEditHostModal, HostDetailDrawer...
│
└── 📂 views/                         # 4. LEAN ORCHESTRATOR VIEWS (150 - 350 LINES ONLY)
    ├── ExplorerView.vue              # Orchestrator layout only (< 400 lines)
    ├── DeploymentsView.vue           # Orchestrator layout only (< 350 lines)
    └── InfraHostsView.vue            # Orchestrator layout only (< 350 lines)
```

---

## 📱 2. 4-TIER RWD & MOBILE-FIRST PWA (ZERO-WASTE VIEWPORT MATRIX)

Every view MUST be fully responsive across **4 display tiers**:
1. **Tier 1: Mobile (< 640px / 390x844)**:
   - Hide oversized headers, hero banners, and 400-800px stacked KPI cards.
   - Show compact command bar (~48px) with resource name, count, status, action button.
   - Display data in **Mobile Card Stream (~65-75px/card)**, showing 4-5 workloads on first screen with **Zero horizontal scroll**.
2. **Tier 2: Tablet (768px - 1024px / 820x1180)**:
   - Auto-collapsing 64px icon-only sidebar, symmetrical 2x2 KPI grid.
3. **Tier 3: Desktop Full HD (1440x900 / 1920x1080)**:
   - Single 240px left sidebar (no double sidebars), 4 horizontal KPI cards, 100% width data table with distinct action buttons (Delete: #f43f5e).
4. **Tier 4: Ultra-Wide & 4K (2560x1440 / 3840x2160)**:
   - Container `max-width: 1920px; margin: 0 auto;`, standard rem typography scale.

---

## ⚙️ 3. BACKEND CLEAN & HEXAGONAL ARCHITECTURE, SOLID & ACID (GO)

Backend Go code MUST adhere to 4 independent layers:
1. **internal/domain/ (Inward Core)**: Pure business entities, value objects, domain errors, zero framework/db dependencies.
2. **internal/usecase/ (Application Layer)**: Business logic orchestration, transactional boundaries (TxManager).
3. **internal/adapter/http/ (Primary Adapters)**: REST handlers, HTTP middleware (RBAC, JWT, Audit), JSON serialization.
4. **internal/infrastructure/ (Secondary Adapters / Ports Implementation)**: PostgreSQL repositories, K8s client-go, Docker SDK, real port 9100 probes.
5. **Zero Mock / Zero Fake Telemetry**: Strict ban on `Math.random()` or hardcoded mock latency. Measure real telemetry.
