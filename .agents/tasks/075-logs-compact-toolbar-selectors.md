# Task 075 — Replace Cluttered `LOG TARGETS` Sidebar with Compact Toolbar `[Node ▾]` & Searchable `[App ▾]` Combobox

> **Status**: `COMPLETED` (`86c7ef65`)
> **Branch**: `feat/declone-sre-ux`
> **User Approved Design**: Option 1 — Remove left `LOG TARGETS` column entirely; place compact `[Node ▾]` + searchable `[App / Service ▾]` (with inline `×` clear) directly on the top toolbar, giving 100% screen width to the log terminal (`Grafana / Datadog` pattern).
> **Verified Evidence**: `docs/qa-evidence/2026-11-05/task-075-verified/01..06` (`reviewer` APPROVED + `5/5` Gates `LES-012` PASS)

## 1. Root Causes Addressed
1. **Severe Visual Clutter (`LogTargetTree.vue` + `LogTargetServicesSection.vue`)**: 10+ stacked UI layers (`LOG TARGETS` header, `[50 logs]` pill, search box, `Nodes` accordion, `+ 1 offline` dashed box, `Services` accordion, `All | Apps | System` 3-tab bar, `All Services` row, per-row `[50]` count pills, `+ idle services` dashed box) wasted ~250px of horizontal screen width just to choose 1 Node and 1 App.
2. **Service List Wipe-Out Bug (`LogTargetServicesSection.vue:49-70`)**: Computing `availableServices` strictly from filtered in-memory `logStore.logs` caused selecting `cron.service` to purge all other services from the list (`SERVICES [1] -> cron.service`), while offline nodes fell back to a flat un-scoped global service list from other nodes.

## 2. Remediated Slices (`reviewer` APPROVED & Live Chrome E2E PASS)

### Slice 75A (`714a5011` / `f4e28070`) — Node-Scoped Service Catalog (`GET /api/v1/logs/services?node=...` & `by_node` Map)
- `internal/adapter/http/log_handler.go`, `internal/infrastructure/agent/distributed_log_repo.go`, `internal/usecase/logging/service.go` (+ unit tests):
  - Added `ListServicesCatalog(ctx)` querying online hosts concurrently (`1s` timeout, mutex-guarded `byNode` map, `HostBackoffManager` fast-fail `< 10ms` returning non-nil `[]` for offline hosts).
  - `GET /api/v1/logs/services` & `/api/v1/logs/targets` now accept optional `?node=<host>` and return `{"services": [...], "targets": [...], "by_node": {"k8smater133": [...], "k8sworker": [...], "k8smasterdeb": []}}`.

### Slice 75B (`86c7ef65` / `3ae88528`) — Full-Width Terminal & Compact Toolbar `LogNodeDropdown` + `LogServiceCombobox`
- Deleted obsolete `LogTargetTree.vue`, `LogTargetServicesSection.vue`, `LogTargetItem.vue`, and `logstream-tree.css` (reducing `LogStreamView` CSS bundle from `128.88 kB` to `84.36 kB`).
- Added `LogNodeDropdown.vue` (`199` lines) and `LogServiceCombobox.vue` (`232` lines) directly inside `LogStreamToolbar.vue` (`278` lines) + `LogStreamView.vue` (`330` lines), reclaiming 100% viewport width for `VirtualLogTerminal.vue`.
- Fixed service wipe-out permanently in `useLogTargets.ts` via persistent per-node discovered service sets (`by_node` + additive stream discovery) and verified with `463/463` passing behavioral tests (`logToolbarSelectors.test.ts`).
