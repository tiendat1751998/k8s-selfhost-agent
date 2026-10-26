# UX & Information Architecture Blueprint: De-Cloning the SRE Command Center

> **Date**: 2026-10-26
> **Branch**: `feat/declone-sre-ux`
> **Task**: [`.agents/tasks/068-ux-decloning-3-waves.md`](../tasks/068-ux-decloning-3-waves.md)
> **Core Principle**: Every screen must be purpose-built around its **SRE Job-to-be-Done** and backed by **100% real backend API telemetry** (zero `Math.random()`, zero synthetic arrays, zero generic Admin CRUD table clones).

---

## 1. Root-Cause Diagnosis

Across 8 of 14 operational views, the UI previously defaulted to the same interchangeable Admin CRUD skeleton:
`[Search Input + Filter Pills + "+ + Create" Button] -> [Rounded Card with Generic 7-8 Column <table>]`

Additional cross-cutting defects identified via `codegraph_explore` and visual audit:
1. **`/hosts` (`InfraHostsView.vue`)**: `useInfraHosts.ts` only fetched `GET /api/v1/hosts` (DB registry rows) and ignored `GET /api/v1/overview/nodes` (`NodeMetrics[]` scraped from the 3 live Linux `k8s-agent` servers `.60`, `.133`, `.80`), hiding real CPU %, Load Avg, RAM used/total, Disk used/total, OS/Kernel, Uptime, and Container/Process counts.
2. **`/deployments` (`DeploymentsView.vue`)**: Rendered workloads as a flat 6-column CRUD table instead of grouping by Namespace with visual Pod Replica health blocks and resource quota badges.
3. **`/slo` (`SLOView.vue`)**: Rendered `SLODefinition` + `SLOSnapshot` as a table of raw PromQL text instead of an Error Budget Burn-Down & Burn-Rate Velocity Board.
4. **`/audit` (`AuditView.vue`)**: Left `useCompliance.ts` and `/api/v1/compliance/frameworks` + `/api/v1/compliance/violations` completely orphaned while rendering two empty CRUD tables.
5. **`/automation` (`AutomationView.vue`) & `/runbooks` (`RunbooksView.vue`)**: Flattened event-driven self-healing rules into two stacked CRUD tables and hid runbook steps behind drawers instead of a Split-Pane Playbook Console.
6. **Global Toolbar & Token Bugs**:
   - 8 components rendered duplicate `+ +` icons (`<BaseIcon name="plus" />` next to `+ Create...`).
   - `base.css:415-421` (`[class*="-toolbar-sleek"] { max-height: 48px !important; }`) clipped right-hand action buttons (`Refres...`) on 1024px–1440px viewports.
   - `--color-primary-fg` was referenced in 4 CSS files without being defined in `tokens.css`.

---

## 2. 3-Wave Domain UX Architecture & Data Contracts

### Wave 1: Infrastructure Fabric (`/hosts`) & Workload Topology (`/deployments`)

#### A. `/hosts` — Physical Hardware Blade Rack (`InfraHostsView.vue`)
- **Job-to-be-Done**: Inspect physical server capacity, OS/kernel health, saturation, and live processes across `.60`, `.133`, `.80` in < 2 seconds.
- **Data Contract (100% Real)**:
  - Join `hostsApi.list()` (`ComputeHost`) with `GET /api/v1/overview/nodes` (`NodeMetrics` from `internal/usecase/metrics/collector.go:61-95`) by `node_id === host.id` or `node_name === host.name`.
  - Fields used: `cpu_percent`, `load_average`, `memory_used`, `memory_total`, `memory_percent`, `disk_used`, `disk_total`, `disk_percent`, `disk_avg_await_ms`, `os_distro`, `kernel_version`, `arch`, `uptime_seconds`, `running_count`, `processes`, `top_processes`.
- **Visual Paradigm**:
  - **Fleet Capacity Strip**: Compact summary of Aggregate Fleet Cores/CPU %, Total RAM Allocated/Used, Total Storage Used/Total, and Online Blades (`3/3 Connected`).
  - **Default View = Hardware Blade Rack Cards** (`Table` kept only as secondary compact toggle):
    - Blade Header: Hostname, Connection Dot + Ping Latency, Role/Env tags, Endpoint, OS Distro + Kernel + Arch + Uptime (`Debian · 6.1.x · amd64 · up Xd`).
    - 3 Horizontal Telemetry Meters: `CPU %` (with `Load 1m/5m/15m`), `Memory %` (`used / total GiB`), `Storage %` (`used / total GiB` + disk await ms).
    - Bottom Process Strip: Active containers / OS processes + hottest process preview (`top_processes[0]`) + `[Inspect]` / `[Probe]` / `[⋯]`.

#### B. `/deployments` — Namespace & Pod Replica Topology (`DeploymentsView.vue`)
- **Job-to-be-Done**: Scan Kubernetes/Container workloads grouped by Namespace, immediately spot unhealthy pod replicas, and trigger Logs / Scale / Rollout actions.
- **Data Contract (100% Real)**:
  - `GET /api/v1/deployments` (`deployment.Application`: `name`, `namespace`, `target`, `type`, `image`, `replicas`, `status`, `cpu`, `memory`, `cpu_limit`, `memory_limit`, `port`, `netType`).
- **Visual Paradigm**:
  - **Default View = Namespace Topology Lanes** (`Table` kept as secondary compact toggle):
    - Collapsible **Namespace Sections** (`kube-system`, `ingress`, `database`, `observability`, etc.) with namespace health badge and workload count.
    - **Workload Topology Tiles** inside each namespace:
      - Workload title + runtime badge + container image tag + network port pill (`:80`, `:5432`, `:6443`).
      - **Visual Pod Replica Matrix**: Individual status squares (`■` per replica, green = healthy, amber = degraded, red = down) alongside `N/N Ready`.
      - Resource Quota Strip (`CPU` / `Memory` reservation & limit) + max 2 primary actions (`Logs`, `Scale`) and `[⋯]` menu.

---

### Wave 2: Reliability Burn-Down (`/slo`) & Security Posture Cockpit (`/audit`)

#### A. `/slo` — Error Budget Burn-Down & Velocity Board (`SLOView.vue`)
- **Job-to-be-Done**: Detect fast-burning error budgets before SLA breach without reading raw PromQL code.
- **Data Contract (100% Real)**:
  - `GET /api/v1/observability/slo/definitions` (`SLODefinition`) + `GET /api/v1/observability/slo` (`SLOSnapshot`: `actual`, `target`, `error_budget`, `burn_rate`, `budget_status`, `window`, `alert_threshold`).
- **Visual Paradigm**:
  - **Top Reliability Pulse Strip**: Healthy SLOs, Fast-Burning Budgets (`burn_rate >= alert_threshold`), Exhausted Budgets, Average Error Budget Remaining %.
  - **SLO Burn-Down Cards Grid** (replacing the PromQL CRUD table as primary view):
    - Service name + Indicator pill (`Availability` / `Latency` / `Error Rate`) + Window (`30d`).
    - Large **Current SLI vs Objective** (`99.96%` vs `Target 99.90%`) + **Burn-Rate Velocity Pill** (`0.4x Normal` / `14.4x Fast Burn`).
    - Segmented **Error Budget Remaining Meter** (`0%–100%`) with threshold marker; raw PromQL moved into click-to-inspect drawer/collapsible footer.

#### B. `/audit` — Security Posture, Compliance Benchmarks & Forensic Cockpit (`AuditView.vue`)
- **Job-to-be-Done**: Measure compliance posture against CIS / NSA-CISA / SOC 2 benchmarks, triage open policy violations & CVEs by severity, and inspect forensic audit events.
- **Data Contract (100% Real)**:
  - Wire existing `useCompliance.ts` (`GET /api/v1/compliance/frameworks` + `GET /api/v1/compliance/violations`) alongside `auditApi.getFindings` and `useAuditLogs()`.
- **Visual Paradigm**:
  - **Top Compliance Posture Scorecards**: 3 interactive framework cards (`CIS Kubernetes Benchmark`, `NSA/CISA Hardening`, `SOC 2 Type II`) with radial SVG score rings (`score%`), `passed / failed` check bars, and click-to-filter framework selection.
  - **Severity Distribution Strip**: Proportional horizontal bar (`Critical`, `High`, `Medium`, `Low`) filtering active findings.
  - **3 Purpose-Built Sub-Views**: `Policy Violations` (grouped/filtered by benchmark & severity) | `Container CVEs` | `Forensic Audit Timeline` (chronological actor-action-resource stream with expandable diff payload).

---

### Wave 3: Event Pipeline Automation (`/automation`), Split Playbook Runner (`/runbooks`) & Global UI Polish

#### A. `/automation` — Self-Healing Event Pipeline (`AutomationView.vue`)
- **Job-to-be-Done**: Understand `WHEN [Telemetry Trigger] ➔ IF [Condition] ➔ THEN [Automated Remediation]` flows at a glance and monitor live execution history.
- **Visual Paradigm**:
  - **2-Column Asymmetric Layout** (`65%` Visual Rule Pipelines on left, `35%` Live Self-Healing Execution Feed on right on desktop; stacked cleanly on mobile/tablet):
    - Each rule renders as a **3-Stage Connected Pipeline Card**: `[Trigger Node] ──▶ [Threshold/Scope Gate] ──▶ [Remediation Action Node]`, with live enable switch, run counter, and `[Test Trigger]` button.
    - Right panel renders a chronological **Execution Event Timeline** (not a 7-column empty CRUD table) with clean empty state CTA to test-fire a rule.

#### B. `/runbooks` — Interactive Split-Pane Playbook Console (`RunbooksView.vue`)
- **Job-to-be-Done**: Read, step through, and execute operational SOPs without losing context in modal popups.
- **Visual Paradigm**:
  - **Master-Detail Split Workspace** (`320px` categorized Runbook Navigator on left + **Interactive Runbook Playbook Reader & Step Runner** on right):
    - Left pane: Search + category pills + compact SOP items with step count badge.
    - Right pane: Structured SOP header, tags, parsed step-by-step interactive checklist with code copy buttons, and inline `[Execute Runbook]` terminal output panel.

#### C. Global UI Hygiene & Token Sweep
- Remove duplicate `+ ` text inside buttons that already render `<BaseIcon name="plus" />` across all 8 components.
- Fix `[class*="-toolbar-sleek"]` in `base.css` so toolbars wrap cleanly instead of clipping right-edge buttons (`Refres...`).
- Define `--color-primary-fg: var(--color-on-primary)` in `tokens.css` and remove legacy `.cyber-*` classes in `HelmCatalogView.vue`.
- Strictly enforce `< 500` lines per `.vue` / `.ts` / `.css` file and 3-tier RWD (`375x812`, `768x1024`, `1440x900`) in both Dark & Light themes.
