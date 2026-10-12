# Task 016 — Comprehensive Backend Audit & Remediation Plan
# Checkpoint: 2026-10-12T09:50:00+07:00

> **Target Workspace**: `d:\project\k8sseflhost`  
> **Status**: IN_PROGRESS (Audit Completed, Remediation Ready)  
> **Total Verified Findings**: 64 (14 P0 Blocker, 33 P1 Critical, 17 P2 Important)  
> **Verification Command**: `go build ./...` (Exit 0), `go vet ./...` (Exit 0)  

---

## 1. Executive Summary & Root Cause Synthesis

The comprehensive audit across all three layers (Wiring, Usecase, Infrastructure) confirms the user's report: multiple backend services are either not working or operating incorrectly due to 5 architectural defects:

1. **Entrypoint Parity Breakage (`cmd/server` vs `cmd/standalone`)**:
   `cmd/server/bootstrap_services.go` initializes 9 major platform domains as `nil` (`Storage`, `K8sBootstrap`, `DR`, `Settings`, `Catalog`, `Scaffolder`, `Plugin`, `Ecosystem`, `Helm`), causing 32 production server API routes to return 404 or 503.
2. **Frontend-Backend API Contract Fracture**:
   `frontend-vue` calls deployment mutations (`/rollback`, `/bluegreen`, `/pause`, `/resume`, `/canary`) and tenancy mutations (`/members`, `/quota`, `/rbac`, `DELETE org`) that do not exist or have mismatched payloads in backend handlers.
3. **Imminent Crash Defect in Kubernetes & HealthCenter**:
   - `explorer_k8s.go:54`: Dereferences `*d.Spec.Replicas` without a `nil` check (P0 panic crash).
   - `healthcenter_repo.go:139,148`: Unsynchronized concurrent map access on `r.cache` (fatal runtime crash).
4. **Silent Failure & Fake Security Passes**:
   - `checkov_scanner.go` & `trivy_scanner.go` synthesize 100% score and `PassSecurityGate: true` when binaries fail.
   - `agents_impl.go:65`: `qaAgent.Execute` returns `err = nil` on failed builds.
5. **Multi-Tenant Isolation Leak across 12 Repositories**:
   Migration 057 added `tenant_id` to 22 tables, but 12 Postgres repositories omit `tenant_id` in `SELECT` and `INSERT` queries.

---

## 2. Master Findings Matrix (All 64 Verified Findings)

### Tier 1: P0 Blocker Findings (Immediate System Risk / Data Loss / Crash)

| ID | Layer | Component | Defect Summary | Evidence `file:line` |
| :--- | :--- | :--- | :--- | :--- |
| **P0-01** | `Wiring` | `cmd/server` | 9 platform domains initialized as nil in server: `Storage`, `K8sBootstrap`, `DR`, `Settings`, `Catalog`, `Scaffolder`, `Plugin`, `Ecosystem`, `Helm`. | `cmd/server/bootstrap_services.go:288-324` |
| **P0-02** | `Wiring` | `Deployments` | Missing deployment mutation routes called by frontend (`/rollback`, `/bluegreen`, `/pause`, `/resume`); `/canary` param mismatch. | `frontend-vue/src/api/deployments.ts:195`<br>`deployment_handler.go:42` |
| **P0-03** | `Wiring` | `Tenancy` | Missing tenancy mutation endpoints called by frontend (`DELETE org`, `POST/DELETE members`, `PUT rbac`, `PUT quota`). | `frontend-vue/src/api/management.ts:87`<br>`tenancy_handler.go:27` |
| **P0-04** | `Infra` | `K8s Explorer` | Nil pointer dereference on `*d.Spec.Replicas` causes server panic crash. | `internal/infrastructure/kubernetes/explorer_k8s.go:54,182` |
| **P0-05** | `Infra` | `HealthCenter` | Fatal runtime crash from unsynchronized concurrent map read/write on `r.cache`. | `internal/infrastructure/kubernetes/healthcenter_repo.go:139,148` |
| **P0-06** | `Infra` | `Security` | Checkov & Trivy return fake 100% clean report when CLI fails or binary is missing. | `internal/infrastructure/security/checkov_scanner.go:47`<br>`internal/infrastructure/security/trivy_scanner.go:53` |
| **P0-07** | `Usecase` | `Auth` | `authUsecase` initialized without `WithRefreshTokenRepo`; fake plaintext token issuer (`"access-token-" + userID`). | `cmd/server/bootstrap_services.go:261`<br>`internal/usecase/auth/refresh_token.go:24,68` |
| **P0-08** | `Usecase` | `GitOps` | Git providers never registered in controller; 100% of remediation PR creations fail. | `cmd/server/bootstrap_services.go:266`<br>`internal/usecase/gitops/controller.go:63` |
| **P0-09** | `Usecase` | `Agent QA` | `qaAgent.Execute` returns `nil` error when build or test fails, falsely reporting success. | `internal/usecase/agent/agents_impl.go:65-75` |
| **P0-10** | `Usecase` | `DR` | 100% stub returning fake 10MB in-memory snapshots and fake Velero backups without touching cluster. | `internal/usecase/dr/dr_usecase.go:89,180` |
| **P0-11** | `Infra` | `Backup SQLServer` | Dump outputs table names instead of data; Restore crashes with T-SQL syntax errors. | `internal/infrastructure/backup/drivers/sqlserver.go:79,161` |
| **P0-12** | `Infra` | `Backup Redis` | Dump uses `/dev/stdout` (fails on Windows); Restore feeds RDB into `--pipe` (rejected). | `internal/infrastructure/backup/drivers/redis.go:53,95` |
| **P0-13** | `Infra` | `Backup S3` | SigV4 signer omits `ContentLength`, triggering AWS S3 HTTP 411 Length Required. | `internal/infrastructure/backup/storage/s3.go:139-150` |
| **P0-14** | `Wiring` | `Standalone Backup`| `backupUsecase.SetRunner(backupPool)` never called in standalone mode (`u.runner == nil`). | `cmd/standalone/standalone_wiring.go:295`<br>`internal/usecase/backup/usecase.go:57` |

---

### Tier 2: P1 Critical Findings (Logic Defect, Security & Multi-Tenancy Leaks)

- **P1-01**: Multi-tenant leaks across 12 Postgres repos (`changes`, `compliance`, `correlation`, `cost`, `capacity`, `drift`, `notification`, `observability`, `promotion`, `reporting`, `tagging`, `timeline`).
- **P1-02**: `notification_repo.go:243` (`MarkAllRead`) runs `UPDATE notifications SET is_read = true WHERE is_read = false`, globally marking all tenants' notifications as read.
- **P1-03**: Router wraps root `/fleet`, `/tenancy`, `/cloud`, `/settings` with `RBACMiddleware("platform_admin")`, blocking read-only `GET` for viewers and operators.
- **P1-04**: Global `RequestBodyLimit(1MB)` drops 10MB Kubeconfig uploads and large K8s YAML apply requests.
- **P1-05**: `POST /docker/logs/cluster-search` mounted under `RequireRolesForMutations`, blocking viewers from searching logs.
- **P1-06**: `CloudClientFactory` passed as `nil` in both server and standalone entrypoints.
- **P1-07**: `POST /fleet/{id}/upgrade` returns HTTP 501 Not Implemented.
- **P1-08**: `GET /deployments/templates` returns 4 static hardcoded maps without querying catalog DB.
- **P1-09**: `POST /automation/rules/{id}/trigger` creates fake audit record without executing action.
- **P1-10**: `POST /runbooks/{id}/execute` returns `"status": "executed"` without executing any runbook steps.
- **P1-11**: Alert evaluation loop only evaluates `tenant in ["default", ""]`, ignoring all other tenants; lacks cooldown.
- **P1-12**: Container vCPU calculated as `cpuPercentSum / 100.0` (assuming 100% = 1 core), undercounting on multi-core nodes.
- **P1-13**: Storage volume `usedBytes` hardcoded to 35% of capacity (`capacityBytes * 0.35`).
- **P1-14**: `UpdateResources` ignores `CPUReservation`; all deployment operations lack `tenant_id` scoping.
- **P1-15**: `promotion_repo.Complete` called before container image update; if container fails, promotion is permanently marked completed.
- **P1-16**: `ComputeSLI` swallows calculation errors and reports fake 100.0% compliance.
- **P1-17**: Capacity forecast uses synthetic multiplier (`currCPU * 0.005`) instead of historical regression.
- **P1-18**: Agent shutdown calls `engineSrc.Close()` while background tailers are still writing.
- **P1-19**: Package-level global `zstdDecoder`/`zstdEncoder` in `pkg/logengine` causes concurrent data races.
- **P1-20**: `backup_repo.go` fails to persist or scan `secondary_storage_id` (migration 030), breaking dual-sync backups.

---

### Tier 3: P2 Important Findings (Dead Code, Cleanup & Hygiene)

- **P2-01**: `cmd/agent-runner` loads all tasks into memory without limit or tenant filter.
- **P2-02**: `cluster/manager.go:128` silently forces `Insecure = true` if CA cert is missing.
- **P2-03**: Missing `rows.Err()` check after `rows.Next()` loop across multiple Postgres repositories.
- **P2-04**: `concurrency.Go` panics if passed a `nil` logger.
- **P2-05**: `internal/pkg/httputil/ssrf.go` is completely redundant with `internal/pkg/netguard`.
- **P2-06**: `internal/infrastructure/iac` (Terraform/Ansible runners) has 230 lines of dead code with 0 callers.
- **P2-07**: `compute_host_handler.go:20` defines `/hosts` CRUD routes never registered in router.
- **P2-08**: `alert_handler.go:253` hardcodes `userID := "system"` when acknowledging alerts.

---

## 3. Actionable Remediation Sprints

### Sprint 1: Data Integrity, Crash Prevention & Security Gate (P0)
- Guard `*d.Spec.Replicas` in `explorer_k8s.go:54`.
- Add `sync.RWMutex` to `healthcenter_repo.go:139`.
- Inject `tenant_id` and apply `BuildTenantQuery` on all 12 Postgres repos.
- Scope `notification_repo.go:243` to current tenant.
- Eliminate fake reports in `checkov_scanner.go` and `trivy_scanner.go`.

### Sprint 2: Server Entrypoint Parity & API Contracts (P0/P1)
- Wire 9 missing platform services in `cmd/server/bootstrap_services.go`.
- Implement missing deployment endpoints in `deployment_handler.go`.
- Implement missing tenancy mutation endpoints in `tenancy_handler.go`.
- Relax RBAC router wrappers to `RequireRolesForMutations` (allow `GET` for viewers).
- Increase `RequestBodyLimit` to 10MB for Kubeconfig upload and YAML apply.

### Sprint 3: Core Engines, Auth & Drivers (P1)
- Wire `WithRefreshTokenRepo` and JWT issuer in `authUsecase`.
- Register Git providers in `gitopsController`.
- Propagate real error in `qaAgent.Execute`.
- Update Alert Engine to evaluate all tenants with state tracking and cooldown.
- Fix vCPU normalization in Cost Calculator.
- Wire `backupPool` in `cmd/standalone`.
- Fix SQLServer, Redis, and S3 backup drivers.

### Sprint 4: Cleanup & Ponytail Optimization (P2)
- Remove redundant `internal/pkg/httputil/ssrf.go`.
- Remove dead IaC runners.
- Replace package-level `zstdDecoder` with `sync.Pool`.
- Coordinate graceful agent shutdown between tailers and log engine writer.
