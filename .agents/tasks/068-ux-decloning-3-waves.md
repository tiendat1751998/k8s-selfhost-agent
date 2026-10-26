# Task 068: 3-Wave UX De-Cloning — Domain-Specific SRE Interfaces

> **Status**: `IN_PROGRESS` (Wave 1 Active)
> **Created**: 2026-10-26
> **Branch**: `feat/declone-sre-ux`
> **Design Blueprint**: [`.agents/audit/2026-10-26-ux-decloning-ia-blueprint.md`](../audit/2026-10-26-ux-decloning-ia-blueprint.md)
> **Hard Constraints**:
> 1. **CodeGraph First (`LES-005`)**: Every subagent MUST call `codegraph_explore` (`projectPath: "d:\\project\\k8sseflhost"`) before searching/editing files.
> 2. **Zero Toy/Fake Data (`LES-002`)**: 100% real backend telemetry (`NodeMetrics`, `Application`, `SLOSnapshot`, `Framework`/`Violation`, `AutomationRule`, `Runbook`). Zero `Math.random()`, zero synthetic arrays.
> 3. **Strict Modularity**: Every `.vue`, `.ts`, and `.css` file MUST stay strictly `< 500` lines.
> 4. **Design Tokens & Dual Theme (`LES-003`)**: Use `tokens.css` semantic variables (`--bg-*`, `--border-*`, `--text-*`, `--color-status-*`) so both Dark and Light modes maintain WCAG AA contrast.

---

## Wave 1: Infrastructure Fabric (`/hosts`) & Workload Topology (`/deployments`)
- [ ] **T068-1.1 (`/hosts` Hardware Blade Rack — `useInfraHosts.ts` + `InfraHostsView.vue` + `src/components/hosts/`)**:
  - Enrich `useInfraHosts.ts` by fetching `GET /api/v1/overview/nodes` (`NodeMetrics[]`) alongside `hostsApi.list()` and joining live hardware metrics (`cpu_percent`, `load_average`, `memory_used`, `memory_total`, `memory_percent`, `disk_used`, `disk_total`, `disk_percent`, `disk_avg_await_ms`, `os_distro`, `kernel_version`, `arch`, `uptime_seconds`, `running_count`, `processes`, `top_processes`) onto each `ComputeHost`.
  - Add a compact **Fleet Hardware Capacity Summary Bar** (Aggregate CPU %, RAM Used/Total GiB, Disk Used/Total GiB, `3/3 Blades Connected`) and default `/hosts` to **Hardware Blade Rack Cards** (`HostBladeRackGrid.vue`, `< 300` lines) with `Blades | Table` view switcher so `/hosts` looks like Proxmox / Datadog Host Fabric instead of an employee table.
- [ ] **T068-1.2 (`/deployments` Namespace & Pod Replica Topology — `DeploymentsView.vue` + `src/components/deployments/`)**:
  - Build `WorkloadNamespaceTopology.vue` (`< 300` lines) grouping workloads by `namespace` (`kube-system`, `ingress`, `database`, `observability`, etc.) into collapsible Namespace Lanes.
  - Inside each tile, render a **Visual Pod Replica Matrix** (individual status squares per replica + `N/N Ready`), image/port/network tags, `CPU` & `Memory` quota pills, and 2 clean inline actions (`Logs`, `Scale`) + `[⋯]` menu, defaulting `/deployments` to `Topology` mode (`Topology | Table` toggle preserved).
- [ ] **T068-1.3 (Wave 1 Gates)**: `npm.cmd run build` + `npx vitest run` + `reviewer` audit + `qa-test-engineer` live `chrome-devtools-mcp` audit (`1440x900` + `375x812`).

---

## Wave 2: Error Budget Burn-Down (`/slo`) & Security Posture Cockpit (`/audit`)
- [ ] **T068-2.1 (`/slo` Burn-Down & Velocity Board — `SLOView.vue` + `src/components/slo/`)**:
  - Replace the PromQL CRUD table default with `SloBurnDownBoard.vue` (`< 300` lines): Top Fleet Reliability Pulse Strip + Service Error Budget Cards showing Current SLI vs Target, Burn-Rate Multiplier badge (`0.4x` vs `14.4x`), segmented Error Budget Remaining bar (`0–100%`), and collapsible PromQL/Inspect drawer (`Board | Table` toggle preserved).
- [ ] **T068-2.2 (`/audit` Compliance Scorecards & Posture Cockpit — `AuditView.vue` + `src/components/audit/` & `compliance/`)**:
  - Connect `useCompliance.ts` (`GET /api/v1/compliance/frameworks` & `/violations`) directly into `AuditView.vue` so the 3 seeded frameworks (`CIS Kubernetes v1.8`, `NSA/CISA`, `SOC 2`) render at top as **Radial Compliance Scorecards** + **Severity Distribution Bar**, followed by tabbed `Policy Violations | Container CVEs | Forensic Audit Timeline`.
- [ ] **T068-2.3 (Wave 2 Gates)**: `npm.cmd run build` + `npx vitest run` + `reviewer` audit + `qa-test-engineer` live `chrome-devtools-mcp` audit (`1440x900` + `375x812`).

---

## Wave 3: Event Pipeline (`/automation`), Split Playbook Console (`/runbooks`) & Global Polish
- [ ] **T068-3.1 (`/automation` Visual `WHEN -> IF -> THEN` Pipeline + Execution Timeline — `AutomationView.vue` + `src/components/automation/`)**:
  - Replace the two stacked CRUD tables with a 2-column layout: left **3-Stage Event Pipeline Cards** (`Trigger -> Threshold Gate -> Action` with live toggle, run count, and `Test Run` button) + right **Chronological Self-Healing Execution Feed**.
- [ ] **T068-3.2 (`/runbooks` Split-Pane Interactive Playbook Console — `RunbooksView.vue` + `src/components/runbooks/`)**:
  - Transform `/runbooks` into a Master-Detail Split Workspace (`320px` categorized SOP list on left + live Markdown step-by-step interactive checklist, copyable CLI snippets, and inline execution console on right; responsive drawer/stack on mobile).
- [ ] **T068-3.3 (Global `+ +` Double-Plus Fix, Toolbar Clipping Fix, & Token Hygiene)**:
  - Fix all 8 `+ +` button text bugs (`AlertsHeaderBar.vue`, `AutomationHeaderBar.vue`, `ExplorerToolbar.vue`, `ReportsFilterToolbar.vue`, `FacetedFilterBar.vue`, `RunbooksView.vue`, `SLOView.vue`).
  - Fix `[class*="-toolbar-sleek"]` right-edge clipping in `base.css:415-421` (`min-height: 48px; height: auto; flex-wrap: wrap`).
  - Define `--color-primary-fg: var(--color-on-primary)` in `tokens.css` and replace legacy `.cyber-*` classes in `HelmCatalogView.vue`.
- [ ] **T068-3.4 (Final Full-Suite Gates)**: `npm.cmd run build`, `npx vitest run`, `reviewer` audit, and `qa-test-engineer` full 6-screen `chrome-devtools-mcp` verification across Desktop (`1440x900`), Tablet (`768x1024`), Mobile (`375x812`), and Dark/Light themes.
