# Task 068: 3-Wave UX De-Cloning — Domain-Specific SRE Interfaces

> **Status**: `COMPLETED` (All 3 Waves Verified Live E2E)
> **Created**: 2026-10-26
> **Completed**: 2026-10-26
> **Branch**: `feat/declone-sre-ux`
> **Design Blueprint**: [`.agents/audit/2026-10-26-ux-decloning-ia-blueprint.md`](../audit/2026-10-26-ux-decloning-ia-blueprint.md)
> **Hard Constraints**:
> 1. **CodeGraph First (`LES-005`)**: Every subagent MUST call `codegraph_explore` (`projectPath: "d:\\project\\k8sseflhost"`) before searching/editing files.
> 2. **Zero Toy/Fake Data (`LES-002`)**: 100% real backend telemetry (`NodeMetrics`, `Application`, `SLOSnapshot`, `Framework`/`Violation`, `AutomationRule`, `Runbook`). Zero `Math.random()`, zero synthetic arrays.
> 3. **Strict Modularity**: Every `.vue`, `.ts`, and `.css` file MUST stay strictly `< 500` lines.
> 4. **Design Tokens & Dual Theme (`LES-003`)**: Use `tokens.css` semantic variables (`--bg-*`, `--border-*`, `--text-*`, `--color-status-*`) so both Dark and Light modes maintain WCAG AA contrast.

---

## Wave 1: Infrastructure Fabric (`/hosts`) & Workload Topology (`/deployments`)
- [x] **T068-1.1 (`/hosts` Hardware Blade Rack — `useInfraHosts.ts` + `InfraHostsView.vue` + `src/components/hosts/`, `1510ef79` & `d822d28d`)**:
  - Enriched `useInfraHosts.ts` by fetching `GET /api/v1/overview/nodes` (`NodeMetrics[]`) alongside `hostsApi.list()` and joining live hardware metrics (`cpu_percent`, `load_average`, `memory_used`, `memory_total`, `memory_percent`, `disk_used`, `disk_total`, `disk_percent`, `disk_avg_await_ms`, `os_distro`, `kernel_version`, `arch`, `uptime_seconds`, `running_count`, `processes`, `top_processes`) onto each `ComputeHost`.
  - Added **Fleet Hardware Capacity Summary Bar** (`FleetCapacityStrip.vue`, `2x2` micro-grid on mobile) and defaulted `/hosts` to **Hardware Blade Rack Cards** (`HostBladeRackGrid.vue`, `< 300` lines) with `Blades | Table` view switcher.
- [x] **T068-1.2 (`/deployments` Namespace & Pod Replica Topology — `DeploymentsView.vue` + `src/components/deployments/`, `5c816803` & `7db42458`)**:
  - Built `WorkloadNamespaceTopology.vue` (`278` lines) grouping workloads by `namespace` (`kube-system`, `ingress`, `database`, `observability`) into collapsible Namespace Lanes.
  - Rendered **Visual Pod Replica Health Matrix** (`■` per replica + `N/N Pods Ready`), image/port/network tags, `CPU` & `MEM` quota pills, and `<= 2` inline actions (`Logs`, `Scale`) + `[⋯]` menu, defaulting `/deployments` to `Topology` mode (`Topology | Table` preserved).
- [x] **T068-1.3 (Wave 1 Gates)**: `npm.cmd run build` + `npm.cmd test` + `reviewer` (`APPROVED`) + `qa-test-engineer` live `chrome-devtools-mcp` audit + contrast/mobile polish (`d822d28d`).

---

## Wave 2: Error Budget Burn-Down (`/slo`) & Security Posture Cockpit (`/audit`)
- [x] **T068-2.1 (`/slo` Burn-Down & Velocity Board — `SLOView.vue` + `src/components/slo/`, `3032f064` & `905acb4d`)**:
  - Replaced PromQL CRUD table default with `SloBurnDownBoard.vue` (`258` lines): Top Fleet Reliability Pulse Strip (`2x2` `minmax(0, 1fr)` on mobile) + Service Error Budget Cards showing Current SLI vs Target, Burn-Rate Multiplier badge (`0.0x Normal`), 20-segment Error Budget Remaining bar (`0–100%`), and collapsible PromQL/Inspect drawer (`Board | Table` toggle preserved).
- [x] **T068-2.2 (`/audit` Compliance Scorecards & Posture Cockpit — `AuditView.vue` + `src/components/compliance/`, `e74be199` & `905acb4d`)**:
  - Connected `useCompliance.ts` (`GET /api/v1/compliance/frameworks` & `/violations`) into `AuditView.vue` (`336` lines) so the 3 frameworks (`CIS Kubernetes v1.8`, `NSA-CISA`, `SOC 2`) render as **SVG Radial Compliance Scorecards** + **Posture Risk Distribution Bar**, followed by tabbed `Compliance & Controls | Container CVEs | Forensic Audit Timeline` (deduplicated secondary severity bar in `ComplianceControlsTable.vue`).
- [x] **T068-2.3 (Wave 2 Gates)**: `npm.cmd run build` + `npm.cmd test` + `reviewer` (`APPROVED`) + `qa-test-engineer` live `chrome-devtools-mcp` audit (`1440x900` + `375x812`).

---

## Wave 3: Event Pipeline (`/automation`), Split Playbook Console (`/runbooks`) & Global Polish
- [x] **T068-3.1 (`/automation` Visual `WHEN -> IF -> THEN` Pipeline + Execution Timeline — `AutomationView.vue` + `src/components/automation/` + `automation_repo.go`, `b4015758`, `905acb4d`, `4ddf9737`)**:
  - Replaced stacked CRUD tables with a 2-column layout: left **3-Stage Event Pipeline Cards** (`AutomationPipelineBoard.vue`, `234` lines, unique `#000001..#000004` short IDs, live toggle, run count, `Test Fire`) + right **Self-Healing Execution Timeline** (`AutomationTimelineFeed.vue`, `230` lines).
  - Fixed backend `CreateExecution` (`internal/infrastructure/postgres/automation_repo.go`, `315` lines) for live PostgreSQL `VARCHAR(255) NOT NULL` `tenant_id` and `TEXT NOT NULL` `error_detail` constraints; verified live `POST /api/v1/automation/rules/{id}/trigger` -> `200 OK`.
- [x] **T068-3.2 (`/runbooks` Split-Pane Interactive Playbook Console — `RunbooksView.vue` + `src/components/runbooks/`, `7db42458`)**:
  - Transformed `/runbooks` into a Master-Detail Split Workspace (`RunbookPlaybookConsole.vue`, `250` lines: `SOP NAVIGATOR` on left + interactive step-by-step `SOP Procedure Progress`, copyable `kubectl` snippets, and `Run Step` execution on right; responsive stack on mobile).
- [x] **T068-3.3 (Global `+ +` Double-Plus Fix, Toolbar Clipping Fix, & Token Hygiene, `7db42458`)**:
  - Fixed all 8 `+ +` button text bugs (`AlertsHeaderBar.vue`, `AutomationHeaderBar.vue`, `ExplorerToolbar.vue`, `ReportsFilterToolbar.vue`, `FacetedFilterBar.vue`, `RunbooksView.vue`, `SLOView.vue`).
  - Fixed `[class*="-toolbar-sleek"]` right-edge clipping in `base.css` (`min-height: 48px; height: auto; flex-wrap: wrap`).
  - Defined `--color-primary-fg: var(--color-on-primary)` in `tokens.css` and replaced legacy `.cyber-*` classes in `HelmCatalogView.vue` & `helm.css`.
- [x] **T068-3.4 (Final Full-Suite Gates)**:
  - `npm.cmd run build`: `Exit 0` (`746 modules transformed`).
  - `npm.cmd test`: `304/304 PASS` across `70` test suites.
  - `go test -count=1 ./internal/infrastructure/postgres/... ./internal/adapter/http/...`: `Exit 0` (including live DB `TestAutomationRepo_LiveDB_CreateAndListExecution`).
  - `reviewer`: `APPROVED` across all 3 waves & backend fix (`4ddf9737`).
  - `qa-test-engineer`: `PASS` across all 6 screens (`final_01`..`final_09`) in Desktop (`1440x900`), Mobile (`375x812`), and Dark/Light modes via `chrome-devtools-mcp`.
