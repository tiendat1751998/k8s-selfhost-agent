# Task 071 — Deep Interactive Defects Remediation (`DEF-01` .. `DEF-16`)

- **Status**: `COMPLETED` (`6d2a8602` — `reviewer` `APPROVED`, Live Chrome E2E QA `16/16 PASS`)
- **Branch**: `feat/declone-sre-ux`
- **Source Audit**: [`.agents/audit/2026-10-27-deep-interactive-destruction-audit.md`](../audit/2026-10-27-deep-interactive-destruction-audit.md)
- **Tier**: Tier 2 Full-Stack Interactive Remediation (3 Parallel File-Disjoint Coders + 2 Delta Polish Coders → `reviewer` → `qa-test-engineer` Live Chrome DevTools Verification)

---

## Parallel Work Breakdown Structure (100% Disjoint File Scopes)

### Subtask 71A — `backend-coder` (`*.go` Backend Contracts, Daemon RPC, YAML Values, UUIDs, Partial Alert Update, Deduplicated Scanner & Zero Log Storm)
- **Scope (`*.go` ONLY)**:
  1. **`DEF-08` (Stop `:8080` Self-Hammering Loop)**: `cmd/standalone/standalone_wiring.go`, `internal/usecase/metrics/tps_traefik.go` — return `""` from `DeriveTraefikURL` when `dockerHost == ""` and `TRAEFIK_API_URL == ""` so `:8080` never polls itself (`0` self-inflicted `404`/`401` spam).
  2. **`DEF-10` (Node Role Classification)**: `internal/usecase/metrics/collector_aggregator.go` — derive `Role` (`"control-plane"` vs `"worker"`) from host labels, K8s control-plane probe, or master/control hostname instead of hardcoded `"agent"`.
  3. **`DEF-04` (Host Daemon Restart & Honest Scale/Delete)**: `internal/infrastructure/kubernetes/deployment_repo.go`, `internal/infrastructure/kubernetes/deployment_fleet_discovery.go`, `internal/adapter/http/deployment_handler.go` — support `type: "daemon"` / `"host-daemon"` on `Restart` by dispatching `POST http://<host>:9100/remediate` (`action: "restart_service"`, `target: name`) using `AGENT_AUTH_TOKEN` (handling `k8s-agent` safely or returning explicit safety guard if needed, and returning HTTP `422 Unprocessable Entity` with descriptive message on `Scale`/`Delete` of systemd host daemons instead of `500`).
  4. **`DEF-05` (Helm Raw YAML + JSON `values`)**: `internal/infrastructure/helm/release_operations.go`, `internal/adapter/http/helm_handler.go` — accept both raw YAML `string` and JSON `map[string]interface{}` in `values` (unmarshaling YAML via `sigs.k8s.io/yaml`), and return clean HTTP `503`/`422` with actionable `:6443 unreachable` detail when K8s API is offline instead of `400 invalid request body`.
  5. **`DEF-06` (Tenancy Member UUID & Live Schema Grounding)**: `internal/infrastructure/postgres/tenancy_extended_repo.go`, `internal/adapter/http/tenancy_handler.go` — inspect live `information_schema.columns` for `tenant_members` & `tenant_quotas` first (`LES-007`), parse `id` with `uuid.Parse` and generate `uuid.NewString()` when empty or non-UUID (`"mem-..."`), ensuring `POST /api/v1/tenancy/members` and `PUT /api/v1/tenancy/organizations/:id/quota` succeed `200/201` on `10.10.10.133:5432`.
  6. **`DEF-07` (Partial Alert Rule Update)**: `internal/adapter/http/alert_handler.go` — support partial `PUT`/`PATCH` on `/api/v1/alerts/rules/{id}` by loading the existing rule first and merging omitted fields so `{ "enabled": false }` succeeds `200 OK`.
  7. **`DEF-11` & `DEF-03` (Deduplicated Compliance Violations & Stream Auth)**: `internal/infrastructure/postgres/compliance_scanner_store.go` — replace active open scanner violations deterministically by `(tenant_id)` / deterministic ID without the 1-hour duplicate accumulation bug, cleaning existing duplicate rows; verify `internal/adapter/http/middleware/auth.go` supports WebSocket `Sec-WebSocket-Protocol: bearer, <token>` and SSE auth cleanly.

### Subtask 71B — `frontend-coder` (Stream Auth Recovery, `/logs`, `/deployments`, `/helm`, `/overview`, `/explorer`, `/slo`)
- **Exclusive Frontend Files (Disjoint from 71C)**:
  - `frontend-vue/src/stores/logStore.ts`
  - `frontend-vue/src/composables/usePodLogs.ts`
  - `frontend-vue/src/composables/usePodTerminal.ts`
  - `frontend-vue/src/components/deployments/**`, `frontend-vue/src/composables/useDeployments.ts`
  - `frontend-vue/src/composables/useHelmCatalog.ts`, `frontend-vue/src/components/helm/**`, `frontend-vue/src/assets/helm.css`
  - `frontend-vue/src/composables/useOverviewDashboard.ts`
  - `frontend-vue/src/views/ExplorerView.vue`, `frontend-vue/src/components/explorer/**`
  - `frontend-vue/src/components/slo/SloAddModal.vue`
- **Deliverables**:
  - `DEF-02`, `DEF-03`: Replace `localStorage.getItem('k8s_token')` with `api.getToken() || useAuthStore().token` and `['bearer', token]` WebSocket subprotocol in `logStore.ts`, `usePodLogs.ts`, and `usePodTerminal.ts`.
  - `DEF-04`, `DEF-09`: Fetch live logs from `/api/v1/logs` (`node` + `query/service`) in `WorkloadLogsTab.vue` for `daemon` / `host-daemon` workloads; disable K8s-only Scale/Delete actions on systemd daemons while keeping `Inspect Logs` & `Rolling Restart` functional.
  - `DEF-05`, `DEF-14`: Fetch chart catalog count on `/helm` mount so `Chart Catalog (120)` renders immediately on `Releases` tab; surface backend `:6443` error cleanly in `HelmInstallDrawer.vue`; fix `375x812` mobile header squeeze/overlap in `HelmCatalogToolbar.vue` / `helm.css`.
  - `DEF-10`, `DEF-13`, `DEF-16`: Match `"control-plane"` role in `useOverviewDashboard.ts` + format throughput without `0 <-> 1` integer jitter; map `'all'` namespace to `'default'` in Explorer `Apply YAML` templates; round `SloAddModal.vue` float target (`99.9` instead of `99.9000015258789`).

### Subtask 71C — `frontend-coder` (`/tenancy` Token Wipe & Modals, `/alerts` Switch, `/audit` Badges, `/automation` Dropdown, `ZeroTrustDrawer` Purge & Spinbuttons)
- **Exclusive Frontend Files (Disjoint from 71B)**:
  - `frontend-vue/src/api/management.ts`
  - `frontend-vue/src/views/TenancyRbacView.vue`, `frontend-vue/src/composables/useTenancyRbac.ts`, `frontend-vue/src/components/tenancy/**`
  - `frontend-vue/src/composables/useAlertManager.ts`, `frontend-vue/src/components/alerts/**`
  - `frontend-vue/src/views/AuditView.vue`, `frontend-vue/src/composables/useCompliance.ts`
  - `frontend-vue/src/components/automation/**`
  - `frontend-vue/src/components/runbooks/**`
  - `frontend-vue/src/components/settings/**`
  - `frontend-vue/src/components/layout/ZeroTrustDrawer.vue`, `frontend-vue/src/components/layout/zeroTrustUtils.ts`, `frontend-vue/src/components/layout/AppSidebar.vue`
- **Deliverables**:
  - `DEF-01`: Delete ALL `api.setToken(localStorage...)` overrides in `management.ts` so visiting `/tenancy` NEVER wipes the in-memory JWT!
  - `DEF-06`: Wire `TenantQuotaModal` `@save` in `TenancyRbacView.vue` to actually call `handleUpdateQuota` (`PUT /api/v1/tenancy/organizations/:id/quota`); generate valid UUID (`crypto.randomUUID()`) for new members; fix Create Project modal default numeric value (`>= 1`).
  - `DEF-07`: Send complete rule fields / clean toggle update in `useAlertManager.ts` and fix `AlertRuleModal.vue` numeric `min`/`max` constraints.
  - `DEF-11`: Compute reactive open/active counts on `/audit` tab badges (`Compliance & Controls (N)` & `Container CVEs (N)`) so remediating/resolving items updates the pill immediately.
  - `DEF-15`, `DEF-16`: Add `prune_logs_and_containers` + all backend actions + dynamic fallback `<option>` in Automation Rule modal; fix invalid `max="0"` / spinbutton constraints in `RunbookExecuteModal.vue` & Settings Notifications.
  - `DEF-12`: Purge ALL hardcoded fake Raft `#4,892,104` / Air-Gapped S3 / `spiffe://k8scontrol.prod` / `2027-03-15` text from `ZeroTrustDrawer.vue` & `zeroTrustUtils.ts`, binding strictly to real `/api/v1/security/kms/status` & live PostgreSQL/Agent posture, and wire/clean unused `@open-zerotrust` binding.

---

## Verification & Acceptance Gates
1. `go build ./cmd/standalone ./cmd/server ./cmd/agent` + `go test ./...` on touched Go packages (`0` errors, `0` Traefik self-404/401 lines in fresh server log).
2. `npm run type-check` / `npx vue-tsc --noEmit` + `npm run build` in `frontend-vue` (`0` errors, `0` occurrences of `localStorage.getItem('k8s_token')` or `localStorage.getItem('token')` in `frontend-vue/src/`).
3. `reviewer` gate: `< 500` lines/file, `<= 350` lines/view, `< 300` lines/component, zero fake strings, zero secrets.
4. `qa-test-engineer` gate: Interactive `chrome-devtools-mcp` verification of all 16 fixed defects against live `:8080` & `10.10.10.133:5432` across Desktop (`1440x900`), Tablet (`768x1024`), and Mobile (`375x812`) in Dark & Light modes, resetting viewport to `1440x900` (`isMobile: false`) Dark Mode (`LES-001`).
