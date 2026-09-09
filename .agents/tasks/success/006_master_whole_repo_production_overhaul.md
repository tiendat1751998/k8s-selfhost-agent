# Master Whole-Repository Production Overhaul & Architecture Plan
**Task ID**: `TASK-006`  
**Standard**: DDD, SOLID, ACID, Ponytail YAGNI, Zero-Trust Multi-Tenancy, Pixel-Perfect RWD  
**Execution Strategy**: Multi-Agent Assembly Line via Subagent-Driven Development (`Workspace: "branch"`)

---

## 1. Multi-Agent Assembly Line Roles & Responsibilities

| Role | Agent Type | Mode | Primary Responsibility |
| :--- | :--- | :---: | :--- |
| **Chief Orchestrator** | `self` (Main Thread) | Coordination | Granular WBS decomposition, prompt composition, subagent dispatch, independent diff verification, and final aggregation. |
| **Principal Architect** | `architect` | Read-only | Domain boundary governance, port extraction, DIP/SRP refactoring review. |
| **Uncompromising Reviewer** | `reviewer` | Read-only | Ponytail complexity, YAGNI dead code audit, Go stdlib replacement gate. |
| **Data Integrity Custodian** | `database-engineer` | Worktree Branch | Physical SQL migrations, `DBTX` transaction boundaries, FK index tuning. |
| **Principal Backend Engineer** | `backend-coder` | Worktree Branch | Port implementations, God-object deconstruction, RBAC middleware, Zero-Trust query scoping. |
| **Principal Frontend Engineer** | `frontend-coder` | Worktree Branch | Vue 3 SPA code splitting, component deduplication, @vueuse integration, pixel-perfect CSS. |
| **Cyber Threat Analyst** | `security-engineer` | Read-only | STRIDE threat re-verification, secret hygiene audit, tenant boundary validation. |
| **QA Test Engineer** | `qa-test-engineer` | Test-only | Automated test execution, race condition checks, headless Chrome visual audit. |

---

## 2. Granular Phased Work Breakdown Structure (WBS)

### 🌊 Wave 1: Ponytail YAGNI Dead-Code Purge & Root Cleanup
- [ ] **Task 1.1**: Delete dead legacy Vanilla JS tree `frontend/` (137 files, ~18,500 lines) and update `findFrontendDir()` in `internal/adapter/http/router.go`.
- [ ] **Task 1.2**: Remove obsolete browser test script `scripts/browser_test/main.go` and clean unused dependencies (`chromedp`, `cdproto`, `sysutil`, `gobwas/ws`) from `go.mod`.
- [ ] **Task 1.3**: Delete root scratch file `fix.py`, abandoned store `frontend-vue/src/stores/backupStore.ts`, and unreferenced `*-responsive` CSS classes in `base.css`.
- [ ] **Task 1.4**: Delete residual build binaries (`server.exe`, `standalone.exe`, `k8s-agent-linux*`, `debug.log`) from repository root.
- **Verification Gate**: `go test ./...` and `npm run build` must pass with 0 errors; Ponytail scorecard verified.

---

### 🌊 Wave 2: DDD Purity, Dependency Inversion (DIP) & Hexagonal Refactoring
- [ ] **Task 2.1**: Define pure domain port `internal/domain/ports/tx.go` for `TransactionManager`.
- [ ] **Task 2.2**: Refactor `internal/usecase/agent/orchestrator.go` and `internal/usecase/gitops/controller.go` to depend on `ports.TransactionManager` instead of `infrastructure/postgres`.
- [ ] **Task 2.3**: Refactor `internal/usecase/agent/agents_impl.go` to use `ports.LLMClient` domain port instead of `infrastructure/llm.Client`.
- [ ] **Task 2.4**: Move shared telemetry aggregate models (`SystemOverview`, `AgentMetrics`) to `internal/domain/nodemetrics` to eliminate horizontal usecase coupling.
- **Verification Gate**: `go test -v ./internal/domain/... ./internal/usecase/...` passes 100%; zero infrastructure imports in domain/usecase.

---

### 🌊 Wave 3: SOLID Deconstruction of God Objects & SRP Enforcement
- [ ] **Task 3.1**: Split `internal/usecase/metrics/collector.go` (1,535 lines) into:
  - `internal/usecase/metrics/scraper.go` (Docker & HTTP scraping)
  - `internal/usecase/metrics/aggregator.go` (Rate and math calculations)
  - `internal/usecase/metrics/evaluator.go` (Alert threshold evaluation)
  - `internal/usecase/metrics/broadcaster.go` (WebSocket event broadcasting)
- [ ] **Task 3.2**: Refactor `internal/usecase/ecosystem/detector.go` to isolate live Docker inspection, HTTP probing, and cache TTL management.
- [ ] **Task 3.3**: Segregate `internal/domain/provider/docker/entity.go` 19-method repository interface into `ContainerRepository`, `SwarmServiceManager`, and `SwarmClusterManager`.
- **Verification Gate**: All metrics, ecosystem, and docker provider tests pass 100%.

---

### 🌊 Wave 4: ACID Database Integrity, DBTX Upgrade & Index Optimization
- [ ] **Task 4.1**: Upgrade `internal/infrastructure/postgres/alert_repo.go` to `DBTX` interface and route queries through `ExtractTx(ctx, r.db)`.
- [ ] **Task 4.2**: Author migration `053_foreign_key_indexes.up.sql` to add indexes on `notifications.channel_id`, `agent_subtasks.task_id`, `agent_executions.task_id`, `projects.org_id`, `tenant_members.org_id`, `backup_policies.storage_id`, `backup_jobs.policy_id`, `restore_jobs.backup_job_id`.
- [ ] **Task 4.3**: Author symmetric rollback `053_foreign_key_indexes.down.sql` and verify migration symmetry.
- **Verification Gate**: `go test -v ./internal/infrastructure/postgres/...` passes 100% with empirical transaction isolation assertions.

---

### 🌊 Wave 5: Zero-Trust Security, Multi-Tenant Scoping & RBAC Gating
- [ ] **Task 5.1**: Add `WHERE tenant_id = $X` to `node_metrics_repo.go` (`QueryHistory`, `GetSummary`) and `observability_repo.go` (`GetHealthSamples`, `ComputeSLI`).
- [ ] **Task 5.2**: Author migration `054_tenant_isolation_legacy_tables.up.sql` adding `tenant_id` column to 16 remaining non-tenant tables and update `tenant_query.go`.
- [ ] **Task 5.3**: Mount `mw.RequireRolesForMutations` on `/audit` routes and dashboard mutation handlers in `internal/adapter/http/router.go`.
- [ ] **Task 5.4**: Configure dynamic `Secure: true` cookie flag for refresh tokens under HTTPS.
- **Verification Gate**: Run adversary tests in `internal/adapter/http/middleware/...` and verify tenant isolation across all endpoints.

---

### 🌊 Wave 6: Pixel-Perfect Frontend RWD, PWA & Code Splitting
- [ ] **Task 6.1**: Convert all static route imports in `frontend-vue/src/router/index.ts` to dynamic code-splitting imports `() => import(...)`.
- [ ] **Task 6.2**: Consolidate copy-pasted `formatBytes` and `formatDate` utilities into `frontend-vue/src/utils/format.ts`.
- [ ] **Task 6.3**: Refactor component polling loops across 6+ views to `@vueuse/core` (`useIntervalFn`, `useDebounceFn`).
- [ ] **Task 6.4**: Consolidate `useExplorerState` and `useExplorerOperations` in `frontend-vue/src/domain/explorer/`.
- **Verification Gate**: `vue-tsc -b && vite build` passes with 0 errors; bundle size optimized under 500kB chunks.

---

## 3. Execution Ledger & Progress Tracker

| Wave | Task Description | Assigned Agent | Status | Verification Command | Commit / Evidence |
| :--- | :--- | :--- | :---: | :--- | :--- |
| **W1** | Ponytail YAGNI Dead-Code Purge | `backend-coder` / `reviewer` | ✅ COMPLETED | `go test ./... && npm run build` | 104 files changed, -20,773 lines, 4 deps removed |
| **W2** | DDD & DIP Port Refactoring | `architect` / `backend-coder` | ✅ COMPLETED | `go test ./internal/domain/...` | `ports.TransactionManager` extracted, 0 leaks |
| **W3** | SOLID God-Object Deconstruction | `backend-coder` | ✅ COMPLETED | `go test ./internal/usecase/...` | `collector.go` split into 4 SRP modules |
| **W4** | ACID DBTX & Index Migrations | `database-engineer` | ✅ COMPLETED | `go test -v ./internal/infrastructure/postgres/...` | `alert_repo` DBTX upgrade + Migration 053 (13 FK indexes) |
| **W-PERF** | Latency SLA & Memory Allocation Audit | `performance-engineer` | ✅ COMPLETED | `go test -bench=. -benchmem ./...` | Benchmarks across crypto, tenancy, postgres, metrics, http |
| **W5** | Zero-Trust & Tenant Isolation | `security-engineer` / `backend-coder` | ✅ COMPLETED | `go test -v ./internal/infrastructure/postgres/...` | Migration 054 + Tenant Query Scoping on legacy tables + RBAC |
| **W6** | Frontend Code Splitting & RWD | `frontend-coder` / `qa-test-engineer` | ✅ COMPLETED | `npm run build && vue-tsc -b` | Dynamic imports on 25+ routes, -95% initial JS bundle (62 kB) |
