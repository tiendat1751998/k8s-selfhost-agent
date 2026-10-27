# Task 069: Purge All 4 Backend Fake/Seeded Fallbacks & Wire 100% Live Fleet Telemetry

> **Status**: `IN_PROGRESS`
> **Created**: 2026-10-27
> **Branch**: `feat/declone-sre-ux`
> **Trigger**: User caught `/slo` displaying 6 seeded services at `100.00%` / `0.0x Normal` (`observability_repo.go:464` returning `100.0` when `slo_health_samples` had `0` rows). Forensic audit exposed 4 backend layers relying on static SQL seeds or hardcoded Go fallbacks.
> **Hard Invariants**:
> 1. **Zero Fake Success / Zero Toy Fallbacks (`Rule 2`, `LES-002`)**: Never return `100.0%` on `0` samples, never return hardcoded `fallbackFleetWorkloads()`, never insert hardcoded `"Result: success"` without probing real infrastructure, and never leave static compliance scores un-evaluated.
> 2. **100% Physical Reality (`10.10.10.60`, `10.10.10.133`, `10.10.10.80`)**: Services that are genuinely UP (`k8s-agent :9100`, `PostgreSQL :5432`, `Redis :6379`, `containerd`, `kubelet :10250`) report UP with real measured latency/metrics; services whose ports are genuinely CLOSED (`kube-apiserver :6443`, `coredns :53`, `traefik-ingress :80`) MUST report **DOWN / `0.00%` RED (Depleted Error Budget)**.
> 3. **Strict `< 500` Lines/File & Zero Hardcoded Secrets (`LES-004`, `LES-007`)**: Every `.go`, `.vue`, `.ts`, `.css` file strictly `< 500` lines (`< 300` for Vue components, `<= 350` for Vue views). Load `.env` via `os.Getenv` only.

---

## Work Breakdown Structure

- [ ] **T069-1 (`/slo` Real Multi-Target Fleet Prober & Zero-Sample Honesty)**:
  - Remove `if totalCount == 0 { return 100.0, nil }` in `internal/infrastructure/postgres/observability_repo.go:464`; return an explicit unmeasured indicator when `totalCount == 0` so unmeasured targets never masquerade as `100.00%`.
  - Upgrade `SLOCollector` (`internal/usecase/slo/collector.go`) to probe all registered `slo_definitions` against live `compute_hosts` (`:9100/health`, `:9100/k8s/health`), PostgreSQL (`db.Ping`), and real TCP/HTTP service endpoints (`:6443`, `:53`, `:80`, `:10250`, custom URLs) with 2s bounded timeouts, recording real `slo_health_samples` immediately on boot and every 30s.
  - Ensure `SLOSnapshotUpdater` runs AFTER initial `SLOCollector.CollectOnce(ctx)` on startup so live `slo_snapshots` immediately reflect real UP (`100.00%` for `k8s-agent-fleet`, `postgres-primary`, `workload-runtime`) vs real DOWN (`0.00%` RED depleted budget for `kube-apiserver`, `coredns`, `traefik-ingress`).

- [ ] **T069-2 (`/deployments` Purge `fallbackFleetWorkloads()` & Live Fleet Daemon Discovery)**:
  - Delete `fallbackFleetWorkloads()` completely from `internal/infrastructure/kubernetes/deployment_repo.go`.
  - When `KUBECONFIG` is absent/unreachable, discover real running daemons/services directly from live `compute_hosts` telemetry (`:9100/metrics` & `:9100/k8s/health`: `postgres`, `redis-server`, `proxysql`, `dockerd`, `containerd`, `kubelet`, `kube-scheduler`, `k8s-agent`) with real CPU/Memory usage, real ports, real host namespaces, and real online/offline status.

- [ ] **T069-3 (`/audit` Live Fleet Security & Hardening Scanner)**:
  - Implement `FleetComplianceScanner` (`internal/usecase/compliance/`) evaluating real checks across `10.10.10.60`, `10.10.10.133`, `10.10.10.80` (`:9100/metrics`, `:9100/k8s/health`, unauthenticated `:9100/metrics` 401 enforcement check, LAN exposure probes on `:10250`, `:6379`, `:5432`, `:6443` control-plane status, rootfs/memory/zombie thresholds, SSL/audit/SLO operational controls).
  - Replace static migration-062 fake K8s violations on startup and wire `POST /api/v1/compliance/scan` + `POST /api/v1/audit/run` (`Scan Now`) to execute the live scanner, update `compliance_frameworks` + `compliance_violations`, and append a real `audit_logs` record.

- [ ] **T069-4 (`/automation` Real `:9100` Agent Telemetry Check & `/remediate` RPC)**:
  - Upgrade `TriggerRule` (`internal/adapter/http/automation_handler.go` / usecase) so `Test Fire` queries live `:9100/metrics` & `:9100/k8s/health` across active `compute_hosts` and invokes `POST http://<node>:9100/remediate` (`reap_zombies`, `disk_pressure_cleanup`, `restart_service`) when applicable, recording exact real node telemetry & latency in `automation_executions` (and `result = "failed"` with `error_detail` if any node fails).

- [ ] **T069-5 (Frontend Honest States & Live Scan Wiring)**:
  - Update `useSLOMonitor.ts` & `SloBurnDownBoard.vue` to render `UNMEASURED` / `--` when `budget_status === 'unmeasured'` (`actual < 0`) and crisp `0.00%` `CRITICAL / EXHAUSTED` burn-rate badges when down targets deplete their error budget.
  - Wire `Scan Now` in `useCompliance.ts` / `AuditView.vue` to trigger live compliance scanning and immediately refresh framework rings, violations, and forensic audit logs.

- [ ] **T069-6 (Quality, Security & Live MCP E2E Gates)**:
  - Unit & live DB integration tests (`go test ./...` on modified packages + `npm.cmd test` + `npm.cmd run build`).
  - `reviewer` adversarial code audit (`< 500` lines/file, zero toy fallbacks, `LES-004` zero secrets).
  - `qa-test-engineer` live `chrome-devtools-mcp` verification across `/slo`, `/deployments`, `/audit`, `/automation` against restarted `bin/standalone.exe` (`:8080`).
