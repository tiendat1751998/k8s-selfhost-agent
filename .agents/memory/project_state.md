# Project State — Checkpoint 2026-10-27 (Task 069: 100% Live Fleet Telemetry Verified)

## Branch: `feat/declone-sre-ux` (`9eff0322`)
## Completed Tasks:
- [`.agents/tasks/069-backend-real-telemetry-purge-fake-data.md`](../tasks/069-backend-real-telemetry-purge-fake-data.md) (`COMPLETED`)
- [`.agents/tasks/068-ux-decloning-3-waves.md`](../tasks/068-ux-decloning-3-waves.md) (`COMPLETED`)

---

## 0. Runtime & Deployment Reality
- **No Automated CI/CD**: Remote Linux nodes require explicit rollout via [`./deploy-agent.sh`](../../deploy-agent.sh).
- **Active Local Ports**: Central Server (`bin/standalone.exe`, commit `9eff0322`) running on `:8080`, Frontend Vite Dev running on `:3000`, Remote PostgreSQL on `10.10.10.133:5432`, Remote Node Agents (`10.10.10.60`, `10.10.10.133`, `10.10.10.80`) on `:9100`.

---

## 1. Mandatory Tooling & Guardrails
- **CodeGraph Index Available (`.codegraph/`)**: Call `codegraph_explore` (`projectPath: "d:\\project\\k8sseflhost"`) FIRST before grep/find/reading files (`LES-005`).
- **Zero Secrets in Scratch/Repo (`LES-004`)**: Load `.env` via `os.Getenv` only; never hardcode credentials.
- **PostgreSQL Schema & Precision Grounding (`LES-007`, `LES-008`)**: Inspect live `information_schema.columns` (`data_type, is_nullable, numeric_precision, numeric_scale`) before repository changes; clamp values to column bounds (`NUMERIC(5,2)` max `999.99`) and never return `100.0%` when `totalCount == 0`.

---

## 2. Live Telemetry Status Across All Operational Screens
- **`/hosts`**: 100% live 5s scrape from `.60:9100`, `.133:9100`, `.80:9100` (`HostBladeRackGrid.vue`).
- **`/logs`**: 100% live ClickHouse `MergeTree` `logengine` on `:9100`.
- **`/slo` (`939cbdbc`, `9eff0322`, `7470fdc6`)**: Real 30s multi-target prober (`collector_probes.go`). Live state: 3 UP (`k8s-agent-fleet`, `postgres-primary`, `workload-runtime` at `100.00%`) + 3 DOWN (`kube-apiserver :6443`, `coredns :53`, `traefik-ingress :80` at `0.00%` RED `Exhausted`, `AVG ERROR BUDGET = 50.0%`). Zero-sample targets report `unmeasured` (`-1.0`).
- **`/deployments` (`939cbdbc`)**: `fallbackFleetWorkloads()` deleted; `deployment_fleet_discovery.go` discovers 8 live daemons across `k8smasterdeb`, `k8smater133`, `k8sworker` with real CPU/RAM/ports.
- **`/audit` (`66a65ad0`, `7470fdc6`)**: `FleetComplianceScanner` probes live `.60`, `.133`, `.80` on boot & `POST /api/v1/compliance/scan` (`CIS 77%`, `NSA-CISA 75%`, `SOC 2 100%`, 10 real active findings).
- **`/automation` (`66a65ad0`)**: `automation_executor.go` inspects live `:9100/metrics` & `:9100/k8s/health` and dispatches `POST :9100/remediate` (`reap_zombies`, `disk_pressure_cleanup`, `restart_service`).
- **Known Unrelated Pre-Existing Test Issue (Out of Task 069 Scope)**: `promotion_repo.go:125` passes `"default-tenant"` into UUID `tenant_id` (flagged by QA for separate follow-up).
