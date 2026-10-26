# Project State — Checkpoint 2026-10-26 (Task 068: 3-Wave UX De-Cloning Completed)

## Branch: `feat/declone-sre-ux` (`4ddf9737`)
## Completed Task: [`.agents/tasks/068-ux-decloning-3-waves.md`](../tasks/068-ux-decloning-3-waves.md)
## UX Architecture Blueprint: [`.agents/audit/2026-10-26-ux-decloning-ia-blueprint.md`](../audit/2026-10-26-ux-decloning-ia-blueprint.md)

---

## 0. Runtime & Deployment Reality
- **No Automated CI/CD**: Remote Linux nodes require explicit rollout via [`./deploy-agent.sh`](../../deploy-agent.sh).
- **Canonical Agent Binary (`fd6155e5`)**: Static stripped `linux/amd64` `./k8s-agent` with embedded ClickHouse `MergeTree` `logengine` (hot in-memory buffer snapshot + compactor + 4-gram skip index + K8s LowCardinality metadata).
- **Active Local Ports**: Central Server (`bin/standalone.exe`) running on `:8080`, Frontend Vite Dev running on `:3000`, Remote Node Agents (`10.10.10.60`, `10.10.10.133`, `10.10.10.80`) on `:9100`.

---

## 1. Mandatory Tooling & Guardrails
- **CodeGraph Index Available (`.codegraph/` — `1,570 files, 25,799 nodes, 71,047 edges`)**:
  - Every subagent MUST call `codegraph_explore` (`ServerName: "codegraph"`, `projectPath: "d:\\project\\k8sseflhost"`) FIRST before grep/find/reading files (`LES-005`).
- **Zero Secrets in Scratch/Repo (`LES-004`)**:
  - Never hardcode passwords, tokens, or DSNs anywhere; load `.env` via `os.Getenv`.
- **Live PostgreSQL Schema Grounding (`LES-007`)**:
  - Always inspect verbatim backend error logs and live `information_schema.columns` before modifying DB repositories; never bind `(*string)(nil)` into `TEXT NOT NULL DEFAULT ''` columns.
- **Zero Cookie-Cutter CRUD Clones (`LES-005`)**:
  - Every operational screen implements its domain Job-to-be-Done visual paradigm rather than repeating `[Filter Bar] + [Generic <table>]`.

---

## 2. Completed Deliverables (Task 068 — All 3 Waves Verified Live)
- **Wave 1 (`COMPLETED`)**:
  - `/hosts` (`InfraHostsView.vue` + `HostBladeRackGrid.vue` + `FleetCapacityStrip.vue`): Physical Hardware Blade Rack Cards joined with live `GET /api/v1/overview/nodes` (`NodeMetrics` for `.60`, `.133`, `.80`).
  - `/deployments` (`DeploymentsView.vue` + `WorkloadNamespaceTopology.vue`): Namespace Topology Lanes + Visual Pod Replica Health Matrix (`■` per replica).
- **Wave 2 (`COMPLETED`)**:
  - `/slo` (`SLOView.vue` + `SloBurnDownBoard.vue`): Error Budget Burn-Down & Burn-Rate Velocity Board (20-segment meters + `2x2` mobile pulse strip).
  - `/audit` (`AuditView.vue` + `CompliancePostureHeader.vue`): CIS / NSA-CISA / SOC 2 SVG Radial Compliance Scorecards + Posture Risk Distribution Bar + 3 Cockpit Tabs.
- **Wave 3 (`COMPLETED`)**:
  - `/automation` (`AutomationView.vue` + `AutomationPipelineBoard.vue` + `AutomationTimelineFeed.vue` + `automation_repo.go` `4ddf9737`): 2-Column `WHEN -> IF -> THEN` Event Pipeline Cards + Live Self-Healing Execution Timeline (`Test Fire` verified `200 OK` end-to-end).
  - `/runbooks` (`RunbooksView.vue` + `RunbookPlaybookConsole.vue`): Master-Detail Split-Pane Interactive Playbook Console (`SOP NAVIGATOR` + runnable `kubectl` steps).
  - Global Hygiene: Removed all 8 `+ +` button bugs, fixed toolbar right-edge clipping in `base.css`, added `--color-primary-fg` in `tokens.css`, removed `.cyber-*` in `HelmCatalogView.vue`.
- **Verification Status**: `304/304` frontend tests PASS, Go repo/handler tests PASS (including live DB integration test), `reviewer` `APPROVED`, `qa-test-engineer` `PASS` (`docs/qa-evidence/2026-10-26/final-6-screens/final_01..final_09.png`).
