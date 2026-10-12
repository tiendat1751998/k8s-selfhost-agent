# 015 — Workload Log Retrieval & Multi-Pod Aggregation

> Generated: 2026-10-12 | Branch: `fix/comprehensive-audit` | Source: User Incident Report & Architectural Audit
> Priority: **P0** | Owners: `backend-coder`, `frontend-coder` | Wave: 5 | Depends on: 005, 010, 014
> Status: INPROCESS

## Problem (verified by orchestrator & researcher)
Users cannot view logs by application/workload (e.g. `ingress-traefik`, `api-gateway`, `auth-service`):
1. **Backend Disconnect**: `internal/adapter/http/k8s_logs_handler.go:132` calls `client.CoreV1().Pods(ns).GetLogs(pod, opts)`. K8s CoreV1 requires an exact Pod name (e.g. `api-gateway-7f9d1-xk92m`). Passing a Deployment name returns `404 Not Found`. There is no workload-level endpoint or resolver from `Deployment Name` -> `matchLabels` -> `Pods`.
2. **Drawer Fragility**: `frontend-vue/src/components/deployments/WorkloadLogsTab.vue:86-114` attempts client-side pod resolution by listing all pods in the namespace, then matching prefix. When pods are missing, pending, or in preview mode, it falls back to printing mock string `[INFO] Kubernetes Workload: ... Direct pod log streaming: Open Live Logs Explorer`.
3. **Log Stream URL Query Blindness**: `frontend-vue/src/views/LogStreamView.vue:128-142` only reads `route.query.node`. It ignores `?app=`, `?pod=`, and `?namespace=`, immediately resetting the view to default `postgres_db`. Furthermore, `LogTargetTree.vue` lacks a "Kubernetes Workloads" category.
4. **Mock Server Gap**: `frontend-vue/scripts/mock-preview-server.js` lacks mock endpoints for workload/pod logs, causing dev/preview environments to show empty or fake text.

## Required Changes

### Subtask 1: Backend Go — Workload Log Resolver & Aggregator (`backend-coder`)
1. Implement `HandleWorkloadLogs(w http.ResponseWriter, r *http.Request)` in `internal/adapter/http/k8s_logs_handler.go`:
   - URL parameters: `cluster`, `kind` (default `deployments`), `name` (workload/deployment name).
   - Query parameters: `namespace`, `pod` (optional specific pod), `container` (optional), `tailLines` (default 100, max 1000), `follow` (SSE stream).
   - Logic:
     - Check namespace RBAC access via `checkNamespaceAccess(r, namespace)`.
     - If `?pod=` provided: stream/fetch log directly for that pod.
     - If `?pod=` omitted:
       - Query Deployment `Spec.Selector.MatchLabels` (or StatefulSet/DaemonSet).
       - List active pods matching the label selector via `client.CoreV1().Pods(ns).List(...)`.
       - Return JSON containing `pods: ["pod-1", "pod-2"]` and aggregated logs with prefix `[pod-name] [time] message` (or recent active pod log).
2. Register routes in `internal/adapter/http/router.go`:
   - `sub.Get("/workloads/{kind}/{name}/logs", platform.K8sLogs.HandleWorkloadLogs)`
   - `sub.Get("/deployments/{name}/logs", platform.K8sLogs.HandleWorkloadLogs)`
3. Unit tests in `internal/adapter/http/k8s_logs_handler_test.go` verifying pod resolution, label matching, and fake clientset log extraction.

### Subtask 2: Frontend — Workload Logs Drawer & Pod Selector (`frontend-coder`)
1. In `frontend-vue/src/api/k8s.ts`:
   - Add `getWorkloadLogs(cluster, name, namespace, options)` querying `/k8s/{cluster}/workloads/{kind}/{name}/logs`.
2. In `frontend-vue/src/components/deployments/WorkloadLogsTab.vue`:
   - Replace fragile client-side `listResources('pods')` + regex with direct `k8sApi.getWorkloadLogs`.
   - Add Pod Selector dropdown/pills when workload has multiple pods: `[ All Replicas (Aggregated) ]` or specific pod.
   - Add Container Selector dropdown if pod has multiple containers (sidecars).
   - Eradicate hardcoded fake fallback strings `[INFO] Kubernetes Workload...`. Render clean real terminal output or actionable technical error state.

### Subtask 3: Frontend — LogStreamView Sync & Target Tree (`frontend-coder`)
1. In `frontend-vue/src/views/LogStreamView.vue`:
   - Watch and read `route.query.app`, `route.query.pod`, `route.query.namespace` on mount and route change, setting `selectedTarget` to the requested workload/pod instead of defaulting to `postgres_db`.
2. In `frontend-vue/src/components/logs/LogTargetTree.vue`:
   - Add "Kubernetes Workloads" category tree node displaying active deployments with `box` icon, allowing direct 1-click selection from the log sidebar.

### Subtask 4: Preview Server — Mock Workload & Pod Logs (`frontend-coder`)
1. In `frontend-vue/scripts/mock-preview-server.js`:
   - Add `GET /api/v1/k8s/:cluster/workloads/:kind/:name/logs` and `/api/v1/k8s/:cluster/pods/:pod/logs`.
   - Provide realistic application log streams for `ingress-traefik`, `api-gateway`, `auth-service`, `redis-master`, and `postgres-operator`.

## Acceptance Criteria
- [ ] Backend test: `GET /api/v1/k8s/{cluster}/deployments/{name}/logs` resolves deployment pods and returns logs.
- [ ] Drawer test: Opening logs on a multi-replica workload renders pod selector and streams real workload logs.
- [ ] Navigation test: Clicking "Full Stream" from Deployments opens `/logs?app=...` with the app pre-selected, not `postgres_db`.
- [ ] Sidebar test: LogTargetTree lists Kubernetes workloads for the active cluster.
- [ ] Preview test: Mock server returns application-specific logs for all sample workloads.
- [ ] Full Gate passes: `go build ./...`, `go vet ./...`, `go test -count=1 ./...`, `npm run build`, all touched files strictly < 500 lines.

## Exact File Scope
- `internal/adapter/http/k8s_logs_handler.go` (< 500 lines)
- `internal/adapter/http/k8s_logs_handler_test.go`
- `internal/adapter/http/router.go` (< 500 lines)
- `frontend-vue/src/api/k8s.ts` (< 500 lines)
- `frontend-vue/src/components/deployments/WorkloadLogsTab.vue` (< 500 lines)
- `frontend-vue/src/views/LogStreamView.vue` (< 500 lines)
- `frontend-vue/src/components/logs/LogTargetTree.vue` (< 500 lines)
- `frontend-vue/scripts/mock-preview-server.js` (< 500 lines)

## Verify Commands
```bash
go test -count=1 -v ./internal/adapter/http -run "TestWorkloadLogs|TestPodLogs"
go build ./...
go vet ./...
npm.cmd --prefix frontend-vue test
npm.cmd --prefix frontend-vue run build
```
