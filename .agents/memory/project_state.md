# Project State — Checkpoint 2026-10-26 (Task 068: 3-Wave UX De-Cloning Active)

## Branch: `feat/declone-sre-ux` (branched from `fix/comprehensive-audit` `4564482b`)
## Active Task: [`.agents/tasks/068-ux-decloning-3-waves.md`](../tasks/068-ux-decloning-3-waves.md)
## Active UX Blueprint: [`.agents/audit/2026-10-26-ux-decloning-ia-blueprint.md`](../audit/2026-10-26-ux-decloning-ia-blueprint.md)

---

## 0. Runtime & Deployment Reality
- **No Automated CI/CD**: Remote Linux nodes require explicit rollout via [`./deploy-agent.sh`](../../deploy-agent.sh).
- **Canonical Agent Binary (`fd6155e5`)**: Static stripped `linux/amd64` `./k8s-agent` with embedded ClickHouse `MergeTree` `logengine` (hot in-memory buffer snapshot + compactor + 4-gram skip index + K8s LowCardinality metadata).
- **Active Ports**: Central Server on `:8080`, Frontend Vite Dev on `:3000`, Node Agent on `:9100`.

---

## 1. Mandatory Tooling & Guardrails
- **CodeGraph Index Available (`.codegraph/` — `1,554 files, 25,609 nodes, 70,311 edges`)**:
  - Every subagent MUST call `codegraph_explore` (`ServerName: "codegraph"`, `projectPath: "d:\\project\\k8sseflhost"`) FIRST before grep/find/reading files (`LES-005`).
- **Zero Secrets in Scratch/Repo (`LES-004`)**:
  - Never hardcode passwords, tokens, or DSNs anywhere; load `.env` via `os.Getenv`.
- **Zero Cookie-Cutter CRUD Clones (`LES-005`)**:
  - Every operational screen must express its domain Job-to-be-Done visual paradigm rather than repeating `[Filter Bar] + [Generic <table>]`.

## 2. Active Execution Roadmap (Task 068)
- **Wave 1 (`IN_PROGRESS`)**:
  - `/hosts` (`InfraHostsView.vue`): Hardware Blade Rack Cards joined with live `GET /api/v1/overview/nodes` (`NodeMetrics` for `.60`, `.133`, `.80`).
  - `/deployments` (`DeploymentsView.vue`): Namespace Topology Lanes + Visual Pod Replica Matrix (`■` per replica).
- **Wave 2 (`PENDING`)**:
  - `/slo` (`SLOView.vue`): Error Budget Burn-Down & Burn-Rate Velocity Board.
  - `/audit` (`AuditView.vue`): CIS / NSA-CISA / SOC 2 Radial Compliance Scorecards + Severity Bar + Violations/CVEs/Forensic Trail.
- **Wave 3 (`PENDING`)**:
  - `/automation` (`AutomationView.vue`): Visual `WHEN -> IF -> THEN` Event Pipeline Cards + Live Execution Timeline.
  - `/runbooks` (`RunbooksView.vue`): Split-Pane Interactive Playbook Reader & Step Runner.
  - Global Polish: Fix all 8 `+ +` button bugs, toolbar `Refres...` right-edge clipping in `base.css`, `--color-primary-fg` token in `tokens.css`, and `.cyber-*` classes in `HelmCatalogView.vue`.
