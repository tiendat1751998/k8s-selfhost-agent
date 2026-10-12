# Workload Log Retrieval & Multi-Pod Aggregation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable end-to-end log retrieval and streaming by Application/Workload across Backend Go API, Workload Inspector Drawer, and Log Stream Explorer.

**Architecture:** A unified 4-layer log pipeline where the Backend Go adapter resolves Workload (Deployment/StatefulSet/DaemonSet) label selectors to active Pods and aggregates their log streams, the Frontend Drawer renders a Pod/Container selector with live terminal output, and LogStreamView seamlessly synchronizes URL query params and sidebar target trees.

**Tech Stack:** Go (client-go v0.36.2, Chi router, CoreV1 PodLogs), Vue 3 + TypeScript, Vite, CSS variables.

## Global Constraints
- Every touched and created file MUST be strictly < 500 lines.
- Zero stubs, zero mocks in production code, no fake progress, no `_ = err`.
- Concurrency and goroutine safety: contexts bounded by deadlines, streams cleanly closed with `defer stream.Close()`.
- Standard library and existing dependencies only (no new dependencies).
- Full verification: Backend tests `go test`, Frontend tests `npm test`, Build `npm run build`, and live Chrome DevTools MCP audit.

---

### Task 1: Backend Go — Workload Log Resolver & Pod Aggregation API

**Files:**
- Modify: `internal/adapter/http/k8s_logs_handler.go:54-188`
- Modify: `internal/adapter/http/router.go:250-256`
- Test: `internal/adapter/http/k8s_logs_handler_test.go`

**Interfaces:**
- Consumes: `client.AppsV1().Deployments(ns).Get`, `client.CoreV1().Pods(ns).List`, `client.CoreV1().Pods(ns).GetLogs`
- Produces: `GET /api/v1/k8s/{cluster}/workloads/{kind}/{name}/logs`, `GET /api/v1/k8s/{cluster}/deployments/{name}/logs` returning `{"workload": string, "namespace": string, "pods": []string, "logs": string, "total_lines": int}`

- [ ] **Step 1: Write unit tests in `k8s_logs_handler_test.go`**
  - Test resolving deployment `ingress-traefik` with 2 active pods in fake clientset.
  - Test querying with specific `?pod=` parameter returns only that pod's log.
  - Test querying non-existent deployment returns clean 404/empty.
- [ ] **Step 2: Run test to verify it fails**
  Run: `go test -v ./internal/adapter/http -run "TestWorkloadLogs"`
  Expected: FAIL with undefined route / method
- [ ] **Step 3: Implement `HandleWorkloadLogs` in `k8s_logs_handler.go` and register routes in `router.go`**
  - Parse `cluster`, `kind`, `name`, `namespace`, `pod`, `container`, `tailLines`, `follow`.
  - Lookup Deployment / StatefulSet label selector and list matching pods.
  - Stream/read logs from matching pods with `[pod-name]` prefix.
- [ ] **Step 4: Run backend tests to verify they pass**
  Run: `go test -count=1 -v ./internal/adapter/http -run "TestWorkloadLogs|TestPodLogs"`
  Expected: PASS
- [ ] **Step 5: Run `go vet` and check line counts**
  Assert `k8s_logs_handler.go` < 500 lines and `router.go` < 500 lines.

---

### Task 2: Frontend Client & Workload Drawer (`WorkloadLogsTab.vue`)

**Files:**
- Modify: `frontend-vue/src/api/k8s.ts:417-444`
- Modify: `frontend-vue/src/components/deployments/WorkloadLogsTab.vue:86-135`
- Test: `frontend-vue/tests/workloadLogsTab.test.ts`

**Interfaces:**
- Consumes: Backend `/api/v1/k8s/{cluster}/workloads/{kind}/{name}/logs`
- Produces: `k8sApi.getWorkloadLogs(cluster, name, namespace, options)` with Pod selector & Container selector UI

- [ ] **Step 1: Write failing component unit test in `workloadLogsTab.test.ts`**
  - Verify `k8sApi.getWorkloadLogs` is called with workload name and namespace.
  - Verify Pod selector dropdown is rendered when `pods.length > 1`.
  - Verify selecting a specific pod fetches logs for that pod only.
  - Verify fake fallback text `[INFO] Kubernetes Workload...` is never rendered.
- [ ] **Step 2: Run test to verify it fails**
  Run: `npm.cmd --prefix frontend-vue test tests/workloadLogsTab.test.ts`
  Expected: FAIL
- [ ] **Step 3: Add `getWorkloadLogs` to `frontend-vue/src/api/k8s.ts`**
  - Call `/k8s/${cluster}/workloads/${kind}/${name}/logs` with query params.
- [ ] **Step 4: Upgrade `WorkloadLogsTab.vue`**
  - Add Pod Selector pills/dropdown: `[ All Replicas (Aggregated) ]` + individual pods.
  - Add Container Selector dropdown if pod has sidecars.
  - Render terminal log stream with line numbers and level colors.
- [ ] **Step 5: Run tests and verify they pass**
  Run: `npm.cmd --prefix frontend-vue test`
  Expected: PASS

---

### Task 3: Log Stream Explorer (`LogStreamView.vue` & `LogTargetTree.vue`) Integration

**Files:**
- Modify: `frontend-vue/src/views/LogStreamView.vue:127-142, 157-175`
- Modify: `frontend-vue/src/components/logs/LogTargetTree.vue:60-120`
- Test: `frontend-vue/tests/logStreamNavigation.test.ts`

**Interfaces:**
- Consumes: `route.query.app`, `route.query.pod`, `route.query.namespace`, `route.query.search`
- Produces: Automatic target selection matching the incoming application query; Sidebar displays "Kubernetes Workloads" node.

- [ ] **Step 1: Write failing unit test in `logStreamNavigation.test.ts`**
  - Verify navigating to `/logs?app=api-gateway&namespace=production` selects `api-gateway` as active target.
  - Verify `LogTargetTree` lists Kubernetes workloads under cluster context.
- [ ] **Step 2: Run test to verify it fails**
  Run: `npm.cmd --prefix frontend-vue test tests/logStreamNavigation.test.ts`
  Expected: FAIL
- [ ] **Step 3: Update `LogStreamView.vue` to parse `app`, `pod`, `namespace`**
  - Support query params in `onMounted` and `watch(route.query)`.
- [ ] **Step 4: Add Kubernetes Workloads group to `LogTargetTree.vue`**
  - Query cluster deployments and list them with `box` icon for quick 1-click selection.
- [ ] **Step 5: Run tests and verify they pass**
  Run: `npm.cmd --prefix frontend-vue test`
  Expected: PASS

---

### Task 4: Preview Mock Server & End-to-End Visual QA

**Files:**
- Modify: `frontend-vue/scripts/mock-preview-server.js:360-390`
- Test: Live Chrome DevTools MCP Visual QA

**Interfaces:**
- Consumes: All UI components on `http://localhost:3000/`
- Produces: Realistic logs for sample apps (`ingress-traefik`, `api-gateway`, `auth-service`, `redis-master`, `postgres-operator`).

- [ ] **Step 1: Add mock routes in `mock-preview-server.js`**
  - `GET /api/v1/k8s/:cluster/workloads/:kind/:name/logs`
  - `GET /api/v1/k8s/:cluster/pods/:pod/logs`
  - Return rich, realistic log lines for each app.
- [ ] **Step 2: Verify build and test suites**
  Run: `npm.cmd --prefix frontend-vue run build`
  Run: `go build ./... && go test -count=1 ./...`
- [ ] **Step 3: QA Audit via Chrome DevTools MCP**
  - Click `[ 📄 Logs ]` on `ingress-traefik` on mobile: verify real logs render with Pod selector.
  - Click "Full Stream": verify `/logs` opens with `ingress-traefik` selected and streaming.
  - Take screenshots and verify zero console errors.
