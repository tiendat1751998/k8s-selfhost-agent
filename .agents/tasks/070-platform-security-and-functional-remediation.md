# Task 070 — Full-Platform Security Hardening & Remaining-Screens Functional Remediation

- **Status**: `COMPLETED (Verified 13/13 Security + Reviewer APPROVED + 8/8 Live Chrome E2E QA PASS)`
- **Source Audit**: [`.agents/audit/2026-10-27-platform-security-and-functional-audit.md`](../audit/2026-10-27-platform-security-and-functional-audit.md)
- **Target Branch**: `feat/declone-sre-ux` (or dedicated sub-branches merged cleanly into `feat/declone-sre-ux` before final PR to `master`)
- **Hard Guardrails**:
  - Strict `< 500 lines/file` (`<= 350` for `.vue` views, `< 300` for `.vue` components).
  - CodeGraph FIRST (`LES-005`), Zero Secrets (`LES-004`), Live PostgreSQL `information_schema` inspection (`LES-007`, `LES-008`), Zero Fake Fallbacks (`LES-002`).
  - Max 3 parallel implementation subagents per wave with strictly disjoint file sets (`Workspace: "branch"`, `Model: "pro"`).

---

## Wave 1 — Critical/High Security Hardening & PostgreSQL Tenant Schema Unification

### Subtask 1A (`backend-coder`): SSRF, IaC RCE, Agent Register RBAC & Workload/Explorer Tenant Isolation
- **Fixes**: `SEC-CRIT-01`, `SEC-CRIT-02`, `SEC-CRIT-03`, `SEC-HIGH-01`, `SEC-HIGH-05`, `SEC-MED-01`
- **Exact Scope**:
  - `internal/usecase/slo/collector_probes.go`, `internal/adapter/http/observability_handler.go` (validate custom `SLO.Query` URLs with `netguard.ValidateURL` + use `netguard.New().NewTransport()` / safe dialer for custom HTTP/TCP probes).
  - `internal/adapter/http/platform_routes.go`, `internal/adapter/http/iac_handler.go`, `internal/infrastructure/iac/terraform.go`, `internal/infrastructure/iac/ansible.go` (`platform_admin` RBAC on `/iac/*` & `/agents/register`; mandatory non-empty sandbox `baseDir`; reject leading `-` and out-of-sandbox paths in `opts.Inventory`).
  - `internal/adapter/http/deployment_handler.go`, `internal/adapter/http/explorer_handler.go`, `internal/adapter/http/docker_host_handler.go` (require `platform_admin` for unnamespaced Docker mutations; enforce `isTenantNamespaceAllowed` on `GET /api/v1/explorer`; mask `auth_token` / secret labels on host GET responses).
  - `cmd/agent/main.go` (optional `ListenAndServeTLS` when `AGENT_TLS_CERT` & `AGENT_TLS_KEY` are set).

### Subtask 1B (`backend-coder` + `frontend-coder` auth slice): Auth Hardening, Trusted Proxy Rate-Limit, Logout Denylist & Real `k8s_live_` API Keys
- **Fixes**: `SEC-HIGH-02`, `SEC-HIGH-03`, `SEC-MED-02`, `SEC-MED-03`, `SEC-LOW-01`, `SEC-LOW-02`
- **Exact Scope**:
  - `internal/adapter/http/router.go`, `internal/adapter/http/middleware.go`, `internal/adapter/http/middleware/rate_limit.go`, `internal/adapter/http/middleware/jwt_auth.go` (replace unconditional `chimw.RealIP` with `TRUSTED_PROXIES` CIDR check; add `X-Tenant-ID` to CORS; add bounded TTL logout token denylist; authenticate `Bearer k8s_live_...` via SHA-256 hash lookup).
  - `internal/adapter/http/auth_handler.go`, `internal/adapter/http/settings_handler.go`, `internal/infrastructure/postgres/settings_repo.go` (surface `requires_password_change` + `POST /api/v1/auth/change-password`; persist `k8s_live_` SHA-256 hash & prefix in PostgreSQL and return raw key once on creation).
  - `frontend-vue/src/stores/authStore.ts`, `frontend-vue/src/api/client.ts` (keep access token in memory `ref`, bootstrap/refresh via `HttpOnly` cookie `/api/v1/auth/refresh`, purge `localStorage` JWT storage, handle `requires_password_change` + one-time API key copy modal).

### Subtask 1C (`database-engineer`): Unify `tenant_id` (`UUID` vs `VARCHAR(255)`) Across All 21 Tables & 12 Repositories
- **Fixes**: `SEC-HIGH-04` (`SQLSTATE 22P02` in `TestPromotion_CreateAndGetByID` and 11 other repos)
- **Exact Scope**:
  - Inspect live `information_schema.columns` & `table_constraints` on `10.10.10.133:5432` first (`LES-007`).
  - Create reversible migration `migrations/063_unify_tenant_id_varchar.up.sql` & `.down.sql` safely converting the `057` `tenant_id UUID` columns (handling any FK constraints cleanly) to `VARCHAR(255) NOT NULL DEFAULT 'default-tenant'`, matching `clusters`, `incidents`, `compute_hosts`, and `automation_rules`.
  - Verify & align the 12 Postgres repositories (`promotion_repo.go`, `reporting_repo.go`, `report_repo.go`, `capacity_repo.go`, `changes_repo.go`, `compliance_repo.go`, `correlation_repo.go`, `drift_repo.go`, `tagging_repo.go`, `tenancy_repo.go`, `timeline_repo.go`, `tenant_helper.go`) so live DB integration tests (`TestPromotion_CreateAndGetByID`, etc.) pass 100%.

- **Wave 1 Gate**: `security-engineer` re-audit + `reviewer` audit + Go unit/integration tests + Vitest.

---

## Wave 2 — Broken Core Features & Real Backend Execution (`/helm`, `/clusters`, `/runbooks`, `/audit`)

### Subtask 2A (`frontend-coder` + `backend-coder` Swarm/Cluster health): Fix `/helm` 120 Blank Cards & `/clusters` + Top-Bar Real Health
- **Exact Scope**:
  - `frontend-vue/src/composables/useHelm.ts`, `frontend-vue/src/components/helm/HelmChartsGrid.vue`, `HelmInstallDrawer.vue`, `HelmCatalogView.vue` (normalize Go Helm SDK payload `Name` / `Chart.Metadata.name`, `version`, `appVersion`, `description`, `icon` so all 120 charts render real names, versions, descriptions, search filtering, and working Install/Dry-Run drawer).
  - `internal/adapter/http/docker_swarm_handler.go` (return `200 { "active": false, "message": "Docker Swarm inactive on host" }` instead of `500` when Swarm/Docker daemon is inactive).
  - `internal/usecase/cluster/health_checker.go`, `internal/infrastructure/postgres/cluster_repo.go`, `frontend-vue/src/components/common/GlobalContextSelector.vue`, `frontend-vue/src/views/ClustersView.vue` (probe all registered clusters regardless of `import_method` using `:6443` / registered `:9100/k8s/health` telemetry; mark `k8snode` honestly as `degraded` when `:6443` is unreachable while `3/3` host agents are online, updating both `/clusters` and the Top-Bar dot to amber `degraded` instead of fake green `healthy`).

### Subtask 2B (`backend-coder` + `frontend-coder`): Real `/runbooks` Step & Playbook Execution + `/audit` Remediate & Runtime Image Posture
- **Exact Scope**:
  - `internal/adapter/http/runbook_handler.go`, `internal/adapter/http/runbook_executor.go`, `frontend-vue/src/composables/useRunbooks.ts`, `frontend-vue/src/components/runbooks/*` (implement `POST /api/v1/runbooks/{id}/steps/{stepIndex}/execute` and real `POST /api/v1/runbooks/{id}/execute` inspecting live fleet endpoints/metrics or executing safe whitelisted `:9100/remediate` diagnostics, returning verbatim output, duration, and step status; remove fake client-side timer).
  - `internal/adapter/http/compliance_handler.go`, `internal/usecase/compliance/scanner.go`, `frontend-vue/src/composables/useAuditCompliance.ts`, `frontend-vue/src/views/AuditComplianceView.vue` (implement `POST /api/v1/compliance/violations/{id}/remediate` executing safe automated remediation where supported or returning structured operator runbook steps + audit log entry instead of `404`; populate `Container CVEs` / Runtime Image Security tab from live `:9100` container/daemon inventory with honest scanner status).

- **Wave 2 Gate**: `reviewer` + `qa-test-engineer` live Chrome DevTools verification on `/helm`, `/clusters`, `/runbooks`, `/audit`.

---

## Wave 3 — Real PostgreSQL Persistence & Honest UX on Remaining Screens (`/tenancy`, `/settings`, `/explorer`, `/overview`, `/incidents`)

### Subtask 3A (`backend-coder`): PostgreSQL Persistence for `/tenancy` & Honest Envelope KMS in `/settings`
- **Exact Scope**:
  - `internal/infrastructure/postgres/tenancy_repo.go`, `internal/adapter/http/tenancy_handler.go` (delete ephemeral Go RAM maps `deletedOrgs`, `orgMembers`, `customRoles`, `quotas`; persist organization CRUD, member assignments, role bindings, and namespace quotas in PostgreSQL tables).
  - `internal/adapter/http/crypto_handler.go`, `internal/infrastructure/postgres/settings_repo.go` (persist KMS key rotation metadata in PostgreSQL instead of ephemeral RAM map; expose honest provider type `local-aes256-gcm` unless external Vault/HSM is configured).

### Subtask 3B (`frontend-coder` + `backend-coder` minor API polish): Honest `/explorer`, `/overview` IP/Workload Fixes, `/incidents` 404 Cleanup & Honest `ZeroTrustDrawer`
- **Exact Scope**:
  - `internal/adapter/http/explorer_handler.go`, `explorer_swarm.go`, `frontend-vue/src/views/ClusterExplorerView.vue` (remove Swarm-as-K8s-Pod masquerade; render explicit `Control Plane (:6443) Unreachable` diagnostic banner when `k8s_degraded: true` with deep-link to `/deployments` & `/hosts`).
  - `internal/usecase/observability/tps_collector.go`, `frontend-vue/src/views/FleetOverviewView.vue`, `frontend-vue/src/components/overview/*`, `AppSidebar.vue` (parse real node IP from `host.address` so `--` becomes `10.10.10.60 / .133 / .80`; show live discovered workload count instead of `0/0 STOPPED`; remove synthetic `traefikRx / 1200` RPS formula; fix `QDeep-Dive` button spacing and sidebar `K8S CONTROL` collapse button overlap).
  - `internal/adapter/http/incident_handler.go`, `incident_remediation.go`, `frontend-vue/src/views/IncidentsLifecycleView.vue` (return clean `200 { "available": false }` instead of `404` spam on ungenerated RCA/PR; remove fabricated dummy GitOps PR YAML when Git provider is unconfigured; gate dev `Simulate` button out of production header).
  - `frontend-vue/src/components/settings/ZeroTrustDrawer.vue` (replace fake `"HSM PKCS#11 / Vault"` claims with honest `AES-256-GCM Envelope Encryption` telemetry).

- **Wave 3 & Final Gate**:
  1. `reviewer` full diff audit (`< 500` lines/file, `< 300` lines/component, `<= 350` lines/view, zero hardcoded hex/`rgba(15,23,42,...)`).
  2. `security-engineer` final verification of all 13 `SEC-*` findings (`0` CRITICAL/HIGH/MEDIUM remaining).
  3. Rebuild `bin/standalone.exe`, restart `:8080`, run Go tests + Vitest + `npm run build`.
  4. `qa-test-engineer` full 3-viewport (`1440x900`, `768x1024`, `375x812`) + Dark/Light E2E audit across all remediated screens, saving real screenshots in `docs/qa-evidence/2026-10-27/task-070-platform-remediation/` and resetting viewport to `1440x900`.
