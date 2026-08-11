# Task: Resolve Backend Code Audits & Redundancies

Refactor the Go backend codebase to resolve 10 identified duplication, redundancy, and mock structure violations.

## Scope of Work

### 1. Fix Nil LLM Client Mock Payload
- **File**: `internal/usecase/agent/agents_impl.go` (lines 27-29)
- **Fix**: Propagate a proper initialization error (`fmt.Errorf`) instead of returning a mock success string with a `nil` error.

### 2. Configure Build/Test Directory Paths Dynamically
- **File**: `internal/usecase/agent/agents_impl.go` (lines 60, 67)
- **Fix**: Replace the hardcoded `"d:\\project\\k8sseflhost"` directory path with a configurable value or default to the relative `.` path.

### 3. Remove/Implement Explorer GetByID Method
- **File**: `internal/infrastructure/kubernetes/explorer_repo.go` (lines 321-325)
- **Fix**: Parse resource details from the ID (kind, namespace, name) and query the live Kubernetes client-go API directly to return the resource.

### 4. Implement SyncResource Endpoint
- **File**: `internal/infrastructure/kubernetes/explorer_repo.go` (lines 327-330)
- **Fix**: Replace the no-op stub with live resource reconciliation using client-go to apply/patch the resource YAML dynamically in the cluster.

### 5. Remove Unused detectNodeIncident Helper
- **File**: `internal/adapter/event/watcher.go` (lines 251-269)
- **Fix**: Delete the unused helper from `watcher.go`.

### 6. Remove Unused IsIncidentType Helper
- **File**: `internal/adapter/event/watcher.go` (lines 279-295)
- **Fix**: Delete the unused helper and its unit tests in `watcher_test.go`.

### 7. Calculate Project Quality Metrics Dynamically
- **File**: `internal/usecase/agent/orchestrator.go` (lines 125-128)
- **Fix**: Replace hardcoded `100.0` values. Retrieve and calculate actual quality metrics dynamically from test coverages and analysis results.

### 8. Consolidate String Truncation Helpers
- **Files**: `internal/usecase/gitops/controller.go` (lines 185-190) and `internal/usecase/rca/pipeline.go` (lines 267-272)
- **Fix**: Move the duplicate helper into a single shared function under `pkg/stringutil` or `internal/pkg/stringutil`.

### 9. Share wsBridge Struct
- **Files**: `cmd/server/main.go` (lines 376-382) and `cmd/standalone/main.go` (lines 443-449)
- **Fix**: Declare `wsBridge` in `internal/adapter/http/websocket.go` and reuse it in both commands.

### 10. Delete Unused Operation Simulation Loops
- **File**: `cmd/standalone/main.go` (lines 233-324, 327-414)
- **Fix**: Remove the dead `generateOperationalData` and `generateEnterpriseData` simulation loops.
