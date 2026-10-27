# Deep Interactive Destruction Audit — All 16 Screens, Modals, Drawers & Streams
**Date**: 2026-10-27  
**Branch**: `feat/declone-sre-ux` (`297dd0d8`)  
**Auditors**: `qa-test-engineer` (70 interactive Chrome DevTools screenshots across `1440x900`, `768x1024`, `375x812` & Dark/Light) + `research` (full static Vue/Go + 7,495-line `task-1976.log` forensic scan)  
**Evidence Folder**: `docs/qa-evidence/2026-10-27/deep-destruction-audit/` (`01_overview_landing.png` .. `70_settings_light_mode.png`)

---

## Executive Summary
While Task 070 (`297dd0d8`) closed 13 security findings and top-level landing bugs, exercising deeper secondary workflows (tabs, modals, drawers, switches, WebSocket/SSE streams, and cross-screen navigation) uncovered **16 concrete functional, auth-regression, backend-contract, and UI defects (`DEF-01` .. `DEF-16`)**:
- **3 P0 Auth & Stream Regressions (`DEF-01` .. `DEF-03`)** caused by lingering `localStorage.getItem('token' | 'k8s_token')` calls after Task 070 Wave 1B moved JWTs strictly into in-memory `authStore.ts` — including a **chain-reaction token wipe** whenever the user visits `/tenancy`.
- **5 P0 Backend/Action Failures (`DEF-04` .. `DEF-08`)** returning `400 Bad Request` or `500 Internal Server Error` on interactive buttons (`Rolling Restart`, `Install Chart`, `Invite Member`, `Toggle Alert Rule`, dead `Save Quota` no-op, and a 4,800-request self-hammering Traefik loop on `:8080`).
- **8 P1/P2 Functional, Data-Honesty & Form/Responsive Defects (`DEF-09` .. `DEF-16`)** across `/deployments`, `/overview`, `/audit`, `ZeroTrustDrawer`, `/explorer`, `/helm`, `/automation`, and modal numeric spinbuttons.

---

## Complete Defect Catalog (`DEF-01` .. `DEF-16`)

### Category A — P0 Auth Regressions & Stream Failures
1. **`DEF-01` (P0 Chain-Reaction Token Wipe on `/tenancy`)**:
   - **Location**: `frontend-vue/src/api/management.ts:42,50,58,66,74`
   - **Root Cause**: `tenancyApi` calls `api.setToken(localStorage.getItem('token') || localStorage.getItem('k8s_token'))` before every `/tenancy` request. Because `localStorage` no longer stores tokens (`SEC-MED-02`), visiting `/tenancy` overwrites the in-memory `api.token` with `null`, breaking authenticated API requests on subsequent screens until silent refresh or reload.
2. **`DEF-02` (P0 `/logs` Real-Time WebSocket Stream `401` Failure)**:
   - **Location**: `frontend-vue/src/stores/logStore.ts:176-194` (`11_logs_landing.png`)
   - **Repro**: `WebSocket connection to 'ws://127.0.0.1:3000/api/v1/logs/stream?node=k8smasterdeb' failed: HTTP Authentication failed; no valid credentials available`
   - **Root Cause**: Reads `localStorage.getItem('k8s_token')` (`null`) instead of `api.getToken() || useAuthStore().token` and fails to pass `['bearer', token]` WebSocket subprotocols.
3. **`DEF-03` (P0 Pod SSE Logs & Pod Exec Terminal WebSocket `401` Failure)**:
   - **Location**: `frontend-vue/src/composables/usePodLogs.ts:170-185`, `frontend-vue/src/composables/usePodTerminal.ts:76-80`, `internal/adapter/http/middleware/auth.go`
   - **Root Cause**: Both composables still read `localStorage.getItem('k8s_token')` (`null`), causing `401 Unauthorized` on SSE pod log streams and WebSocket pod exec sessions.

### Category B — P0 HTTP 400 / 500 Interactive Action Failures & Log Storm
4. **`DEF-04` (P0 `/deployments` Rolling Restart `500` on Host Daemons)**:
   - **Location**: `internal/infrastructure/kubernetes/deployment_repo.go:210-228`, `internal/adapter/http/deployment_handler.go`, `frontend-vue/src/composables/useDeployments.ts:263-278,422-435` (`25_deployments_workload_drawer.png`)
   - **Repro**: Clicking `Rolling Restart` on discovered host daemons (`type: "daemon"`, e.g. `redis-server`, `dockerd`, `k8s-agent`) sends `POST /api/v1/deployments/restart` -> `500 {"error":"failed to restart deployment"}` (`unsupported target type: daemon`).
   - **Required Behavior**: Dispatch safe `POST <node>:9100/remediate` (`restart_service`) for registered host daemons (protecting `k8s-agent` from synchronous self-kill or handling async restart cleanly), and return `422 Unprocessable Entity` (and disable K8s-only Scale/Delete buttons in UI) for systemd daemons.
5. **`DEF-05` (P0 `/helm` Chart Install & Upgrade `400 Bad Request`)**:
   - **Location**: `internal/infrastructure/helm/release_operations.go:56,69`, `internal/adapter/http/helm_handler.go` (`35_helm_install_wizard.png`)
   - **Repro**: Submitting Step 3 of `Install Chart` wizard sends `"values": "# Default values...\n"` (YAML string) while Go expects `map[string]interface{}` -> `400 {"error":"invalid request body"}`.
   - **Required Behavior**: Support both raw YAML `string` and JSON `map[string]interface{}` for `values` on the backend, and surface an honest `503`/`422` diagnostic when K8s `:6443` is unreachable instead of opaque `400`.
6. **`DEF-06` (P0 `/tenancy` Invite Member `500` Invalid UUID + Dead Quota Save No-Op)**:
   - **Location**: `internal/infrastructure/postgres/tenancy_extended_repo.go:50`, `internal/adapter/http/tenancy_handler.go`, `frontend-vue/src/views/TenancyRbacView.vue:285` (`51_tenancy_quotas_modal.png`, `52_tenancy_invite_member_modal.png`)
   - **Repro**:
     - Invite Member sends `"id": "mem-1793095309421"` into PostgreSQL `UUID PRIMARY KEY` -> `500 Internal Server Error`.
     - `TenantQuotaModal` `@save` only fires `showFeedback('Quota updated successfully')` without calling `handleUpdateQuota` (`PUT /api/v1/tenancy/organizations/:id/quota`)!
7. **`DEF-07` (P0 `/alerts` Toggle Alert Rule Switch `400 Validation Failed`)**:
   - **Location**: `frontend-vue/src/composables/useAlertManager.ts:166`, `internal/adapter/http/alert_handler.go:208`
   - **Repro**: Toggling `ARMED` switch sends `PUT /api/v1/alerts/rules/{id}` with `{"channel_ids":[],"enabled":false}` -> `400 {"details":{"condition":"condition is required","metric_name":"metric_name is required","name":"name is required"}}`.
8. **`DEF-08` (P0 Backend Self-Hammering Loop Storm — 4,800+ `404`/`401` Errors in `task-1976.log`)**:
   - **Location**: `cmd/standalone/standalone_wiring.go:134-140`, `internal/usecase/metrics/tps_traefik.go:146`
   - **Root Cause**: When `cfg.DockerHost == ""` and `TRAEFIK_API_URL == ""`, `DeriveTraefikURL("")` returns `"http://localhost:8080"`, causing `:8080` to poll **itself** every few seconds on `/api/http/services` (`404`), `/api/overview` (`404`), and `/metrics` (`401`).

### Category C — P1/P2 Functional, Data-Honesty & Form/Responsive Defects
9. **`DEF-09` (`/deployments` Container Logs Tab Blank on Host Daemons)**:
   - **Location**: `frontend-vue/src/components/deployments/WorkloadLogsTab.vue:36-38,77-104` (`25_deployments_workload_drawer.png`)
   - **Root Cause**: Only queries K8s or Docker logs; sets `logsRawContent.value = ''` for `type === 'daemon'` instead of querying `/api/v1/logs` (`logengine` on `:9100`) by node + daemon service name.
10. **`DEF-10` (`/overview` `Control-Plane (0)` Hardcoded Role & Throughput Rounding Jitter)**:
    - **Location**: `internal/usecase/metrics/collector_aggregator.go:132`, `frontend-vue/src/composables/useOverviewDashboard.ts:188-220,343` (`01_overview_landing.png`)
    - **Root Cause**: Backend hardcodes `Role: "agent"` on all nodes (`Control-Plane (0)` despite `k8smaster` and `k8smasterdeb`); `Math.round(throughput)` jitters `0 <-> 1`.
11. **`DEF-11` (`/audit` Duplicate Violation Accumulation `35 -> 70 -> 105` & Stale Tab Badge)**:
    - **Location**: `internal/infrastructure/postgres/compliance_scanner_store.go:128-157`, `frontend-vue/src/views/AuditView.vue`, `frontend-vue/src/composables/useCompliance.ts` (`37_audit_landing.png`, `38_audit_remediation_drawer.png`)
    - **Root Cause**: `ReplaceViolations` only deletes rows older than `1 hour`, multiplying 5 violations per scan into 35+ duplicate rows; remediating an item does not decrement the active tab count pill.
12. **`DEF-12` (`ZeroTrustDrawer.vue` Dead Sidebar Event + Remaining Hardcoded Fake Raft/S3/SPIFFE Strings)**:
    - **Location**: `frontend-vue/src/components/layout/ZeroTrustDrawer.vue:162,171,186,188`, `frontend-vue/src/components/layout/zeroTrustUtils.ts:64-78`, `frontend-vue/src/components/layout/AppSidebar.vue` (`69_zerotrust_drawer.png`)
    - **Root Cause**: Still renders hardcoded `Commit Index #4,892,104 · 1.14ms write`, `Secondary S3 (Air-Gapped Sync) TLS 1.3 Immutable WORM · 0 byte lag`, `spiffe://k8scontrol.prod`, and `2027-03-15 (543d valid)`.
13. **`DEF-13` (`/explorer` Apply YAML Modal Invalid `namespace: all`)**:
    - **Location**: `frontend-vue/src/views/ExplorerView.vue` / `frontend-vue/src/components/explorer/*` (`29_explorer_apply_yaml_modal.png`)
    - **Root Cause**: Injects `namespace: all` into sample manifests when global namespace selector is `"All Namespaces"`.
14. **`DEF-14` (`/helm` Landing `Chart Catalog (0)` Badge & Mobile `375px` Header Overlap)**:
    - **Location**: `frontend-vue/src/composables/useHelmCatalog.ts:311-315`, `frontend-vue/src/components/helm/HelmCatalogToolbar.vue`, `frontend-vue/src/assets/helm.css:449-455` (`31_helm_landing.png`, `36_helm_mobile_375x812.png`)
    - **Root Cause**: Skips catalog count fetch when landing on `Releases` tab; header buttons overlap on `375x812`.
15. **`DEF-15` (`/automation` Edit Rule Modal Blank Action Combobox)**:
    - **Location**: `frontend-vue/src/components/automation/*` (`43_automation_edit_rule_modal.png`)
    - **Root Cause**: Existing rule action `prune_logs_and_containers` is missing from `<select>` options.
16. **`DEF-16` (Form Numeric Spinbutton Range Bugs & Float Precision Artifacts)**:
    - **Location**: `AlertRuleModal.vue`, `RunbookExecuteModal.vue`, `TenancyRbacView.vue` / Create Project modal, `SloAddModal.vue`, `SettingsView` Notifications (`08_slo_add_modal.png`, `47_runbooks_execute_modal.png`, `53_tenancy_create_project_modal.png`, `56_alerts_new_rule_modal.png`, `63_settings_notifications.png`)
    - **Root Cause**: `min="0" max="0"` (`valuemin=0 valuemax=0`), Create Project `min="1"` with default `0`, and `SloAddModal` float `99.9000015258789`.
