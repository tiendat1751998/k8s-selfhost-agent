# Task 075 — Replace Cluttered `LOG TARGETS` Sidebar with Compact Toolbar `[Node ▾]` & Searchable `[App ▾]` Combobox

> **Status**: `IN_PROGRESS`
> **Branch**: `feat/declone-sre-ux`
> **User Approved Design**: Option 1 — Remove left `LOG TARGETS` column entirely; place compact `[Node ▾]` + searchable `[App / Service ▾]` (with inline `×` clear) directly on the top toolbar, giving 100% screen width to the log terminal (`Grafana / Datadog` pattern).

## 1. Root Causes Addressed
1. **Severe Visual Clutter (`LogTargetTree.vue` + `LogTargetServicesSection.vue`)**: 10+ stacked UI layers (`LOG TARGETS` header, `[50 logs]` pill, search box, `Nodes` accordion, `+ 1 offline` dashed box, `Services` accordion, `All | Apps | System` 3-tab bar, `All Services` row, per-row `[50]` count pills, `+ idle services` dashed box) wasted ~250px of horizontal screen width just to choose 1 Node and 1 App.
2. **Service List Wipe-Out Bug (`LogTargetServicesSection.vue:49-70`)**: Computing `availableServices` strictly from filtered in-memory `logStore.logs` caused selecting `cron.service` to purge all other services from the list (`SERVICES [1] -> cron.service`), while offline nodes fell back to a flat un-scoped global service list from other nodes.

## 2. Disjoint Parallel Slices (`2` Builders, `Workspace: "branch"`, `Model: "pro"`)

### Slice 75A (`backend-coder`) — Node-Scoped Service Catalog (`GET /api/v1/logs/services?node=...` & `by_node` Map)
- Scope:
  - `internal/domain/logging/` (if interface signature extended safely or via optional node-aware method `ListServicesByNode(ctx, node string)`)
  - `internal/infrastructure/agent/distributed_log_repo.go` (+ `_test.go`)
  - `internal/adapter/http/log_handler.go` (+ `_test.go`)
- Contract:
  - `GET /api/v1/logs/services` (and `/api/v1/logs/targets`) accepts optional `?node=<nodeName>`.
  - Response JSON preserves backward-compatible `"services": string[]` and `"targets": string[]`, and adds `"by_node": Record<string, string[]>` mapping each online host name (e.g. `"k8smater133"`, `"k8sworker"`) to its sorted deduplicated service list.
  - If `?node=<nodeName>` is passed for an online node, `"services"` / `"targets"` contain only that node's services; if passed for an offline node (`k8smasterdeb`), fast-fail (`HostBackoffManager`) returns `"services": []` in `< 10ms` instead of leaking another node's services.

### Slice 75B (`frontend-coder`) — Remove Left `LOG TARGETS` Sidebar & Add Toolbar `LogNodeDropdown` + `LogServiceCombobox`
- Scope:
  - `frontend-vue/src/views/LogStreamView.vue` (`<= 350` lines)
  - `frontend-vue/src/components/logs/LogStreamToolbar.vue` (`< 300` lines)
  - `frontend-vue/src/components/logs/LogNodeDropdown.vue` (new, `< 200` lines)
  - `frontend-vue/src/components/logs/LogServiceCombobox.vue` (new, `< 250` lines)
  - `frontend-vue/src/composables/useLogTargets.ts` & `frontend-vue/src/api/logs.ts` (consume `by_node` + `?node=` and retain cached per-node catalog so selecting `cron.service` NEVER wipes out sibling apps!)
  - `frontend-vue/src/assets/styles/views/logstream.css` (`< 500` lines)
  - Remove obsolete `LogTargetTree.vue`, `LogTargetServicesSection.vue`, `LogTargetItem.vue`, `log-target-tree.css` (verify zero remaining imports) and update behavioral tests (`0` `fs.readFileSync` regex assertions).
