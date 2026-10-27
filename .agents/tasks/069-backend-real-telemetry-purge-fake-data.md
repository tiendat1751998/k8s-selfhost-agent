# Task 069: Purge All 4 Backend Fake/Seeded Fallbacks & Wire 100% Live Fleet Telemetry

> **Status**: `COMPLETED` (All 4 Modules Verified Live E2E)
> **Created**: 2026-10-27
> **Completed**: 2026-10-27
> **Branch**: `feat/declone-sre-ux` (`9eff0322`)
> **Trigger**: User caught `/slo` displaying 6 seeded services at `100.00%` / `0.0x Normal` (`observability_repo.go:464` returning `100.0` when `slo_health_samples` had `0` rows). Forensic audit exposed 4 backend layers relying on static SQL seeds or hardcoded Go fallbacks.
> **Hard Invariants**:
> 1. **Zero Fake Success / Zero Toy Fallbacks (`Rule 2`, `LES-002`)**: Never return `100.0%` on `0` samples, never return hardcoded `fallbackFleetWorkloads()`, never insert hardcoded `"Result: success"` without probing real infrastructure, and never leave static compliance scores un-evaluated.
> 2. **100% Physical Reality (`10.10.10.60`, `10.10.10.133`, `10.10.10.80`)**: Services that are genuinely UP (`k8s-agent :9100`, `PostgreSQL :5432`, `Redis :6379`, `containerd`, `kubelet :10250`) report UP with real measured latency/metrics; services whose ports are genuinely CLOSED (`kube-apiserver :6443`, `coredns :53`, `traefik-ingress :80`) report **DOWN / `0.00%` RED (Depleted Error Budget)**.
> 3. **Strict `< 500` Lines/File & Zero Hardcoded Secrets (`LES-004`, `LES-007`)**: Every `.go`, `.vue`, `.ts`, `.css` file strictly `< 500` lines (`< 300` for Vue components, `<= 350` for Vue views). Load `.env` via `os.Getenv` only.

---

## Work Breakdown Structure

- [x] **T069-1 (`/slo` Real Multi-Target Fleet Prober & Zero-Sample Honesty — `939cbdbc` & `9eff0322`)**:
  - Removed `if totalCount == 0 { return 100.0, nil }` in `internal/infrastructure/postgres/observability_repo.go`; returns `-1.0` sentinel (`budget_status = "unmeasured"`) when `totalCount == 0`.
  - Upgraded `SLOCollector` (`collector.go` + `collector_probes.go`) to probe all registered `slo_definitions` every 30s (and synchronously on boot before `SLOSnapshotUpdater`) against live `compute_hosts` (`:9100/health`, `:9100/k8s/health`), PostgreSQL (`dbPool.Ping`), and real TCP/HTTP ports (`:6443`, `:53`, `:80`).
  - Clamped `burn_rate` to `[0, 999.99]` matching PostgreSQL `NUMERIC(5,2)` (`9eff0322`), verified by `TestObservabilityRepo_LiveDB_SaveDownAndUnmeasuredSnapshot` on `10.10.10.133:5432`.

- [x] **T069-2 (`/deployments` Purge `fallbackFleetWorkloads()` & Live Fleet Daemon Discovery — `939cbdbc`)**:
  - Deleted `fallbackFleetWorkloads()` completely from `internal/infrastructure/kubernetes/deployment_repo.go`.
  - Implemented `deployment_fleet_discovery.go` querying live `:9100/metrics` & `:9100/k8s/health` across registered `compute_hosts` concurrently (`2s` timeout, Bearer token auth), discovering the 8 real live daemons (`kube-scheduler`, `dockerd`, `postgres`, `redis-server`, `proxysql`, `k8s-agent`, `containerd`, `kubelet`) with real CPU/RAM/ports.

- [x] **T069-3 (`/audit` Live Fleet Security & Hardening Scanner — `66a65ad0`)**:
  - Built `FleetComplianceScanner` (`internal/usecase/compliance/scanner.go`) + `compliance_scanner_store.go` running live checks across `.60`, `.133`, `.80` (rootfs/RAM/zombie checks, Kubelet `:10250` LAN exposure, `:6443` unreachable, Redis `:6379` LAN exposure, Postgres `:5432` `sslmode=disable`, unauthenticated `:9100/metrics` 401 zero-trust check, SOC 2 operational controls).
  - Replaced static migration-062 fake K8s violations on boot and wired `POST /api/v1/compliance/scan` + `POST /api/v1/audit/run` synchronously.

- [x] **T069-4 (`/automation` Real `:9100` Agent Telemetry Check & `/remediate` RPC — `66a65ad0`)**:
  - Built `automation_executor.go` querying live `:9100/metrics` & `:9100/k8s/health` and invoking `POST <host>:9100/remediate` (`reap_zombies`, `disk_pressure_cleanup`, `restart_service`), recording real node metrics & RPC latency (or `Result = "failed"` with `ErrorDetail` when unreachable).

- [x] **T069-5 (Frontend Honest States & Live Scan Wiring — `7470fdc6`)**:
  - Updated `useSLOMonitor.ts`, `SloBurnDownBoard.vue`, and `SLOView.vue` to render `Unmeasured` (`--`) on `actual < 0` and `0.00%` `1000x Burn` / `0.0% (Exhausted)` on down services without layout overflow.
  - Wired `Scan Now` in `api/compliance.ts`, `useCompliance.ts`, and `AuditView.vue` to `POST /api/v1/compliance/scan` with parallel refresh of frameworks, violations, and audit logs (`311/311` tests PASS).

- [x] **T069-6 (Quality, Security & Live MCP E2E Gates)**:
  - `reviewer`: **APPROVED** across all 36 files (`ab02f4cc..9eff0322`, every file `< 500` lines, zero hardcoded secrets).
  - `qa-test-engineer`: **PASS** across `/slo`, `/deployments`, `/audit`, `/automation` in Desktop (`1440x900`), Mobile (`375x812`), and Dark/Light modes (`docs/qa-evidence/2026-10-27/task-069-real-telemetry/t069_01..06.png`).
