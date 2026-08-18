# Task 3: Infrastructure Quality

## Goal
Fix fake implementations, unbounded memory, and missing health endpoints in infrastructure modules.

## Files to Modify (ONLY these)
- `internal/infrastructure/logging/aggregator.go`
- `internal/infrastructure/kubernetes/capacity_repo.go`
- `cmd/agent-runner/main.go`
- `internal/adapter/http/fleet_handler.go`
- `internal/adapter/http/audit_handler.go`

## Specific Fixes Required

### 1. Log Aggregator Unbounded Memory (aggregator.go:20-63, 100-148)
Logs stored in in-memory `RingBuffer` per namespace/pod with no eviction. `a.buffers[namespace/pod]` grows unboundedly.

**Fix:**
- Add TTL-based eviction: entries older than configurable TTL (default 1 hour) are pruned.
- Add max buffer count: limit total number of tracked namespace/pod combinations (e.g., 1000). Evict LRU when exceeded.
- Add a background goroutine for periodic cleanup (every 30 seconds).

### 2. Hardcoded Capacity Metrics (capacity_repo.go:50-58, 103-113, 203-242)
- CPU metrics: `cpuPercent := (float64(activeContainers) * 0.15 / float64(info.NCPU)) * 100.0` — hardcoded formula, not real metrics.
- Storage usage: hardcoded `25.0%` or `35.0%` with arbitrary offsets.
- `capacity_repo.go:246-249`: `Record(ctx, f)` is an empty stub returning `nil`.

**Fix:**
- CPU/Memory: Use Docker API `container.Stats()` for real per-container metrics, or Kubernetes metrics API (`metrics.k8s.io/v1beta1`).
- Storage: Use `disk.Usage()` or `os.StatFS()` for real disk stats.
- If real metrics source unavailable, return a clear error or `ErrMetricsUnavailable`, NOT hardcoded percentages.
- Implement `Record()` to actually persist metrics or remove it.

### 3. Agent-Runner Missing Health Endpoint (cmd/agent-runner/main.go:50-106)
The agent-runner runs as an infinite worker loop with no HTTP health endpoints. Kubernetes has no way to probe liveness/readiness.

**Fix:**
- Start a minimal HTTP server on a configurable port (default `:8081`) with:
  - `GET /healthz` → 200 OK (liveness)
  - `GET /readyz` → 200 OK when connected to NATS, 503 otherwise (readiness)
- Run the HTTP server in a separate goroutine alongside the worker loop.

### 4. Simulated Cluster Upgrade (fleet_handler.go:187-193)
`UpgradeCluster` launches a 10-second goroutine that just updates the DB status to `"active"` and increments the version string. No actual Kubernetes upgrade.

**Fix:** Either:
- (a) Remove the endpoint and return `501 Not Implemented` with a clear message, OR
- (b) Make it trigger a real Kubernetes control plane upgrade via `kubeadm upgrade` or provider API. Since we can't assume provider access, option (a) is safer.

### 5. Fake Audit Run (audit_handler.go:56-77)
`TriggerAuditRun` hardcodes `Status: "completed"`, `FindingsCount: 0`, `StartTime: now.Add(-5*time.Second)`.

**Fix:** 
- Create the audit run with `Status: "pending"` and real `StartTime: now`.
- If no background audit worker exists, return the pending run and document that a worker is needed.
- Do NOT fake completion — it's worse than honest "pending".

## Acceptance Criteria
1. Log aggregator has TTL eviction + max buffer count limit
2. Capacity metrics return real data or explicit `ErrMetricsUnavailable`
3. Agent-runner exposes `/healthz` and `/readyz` HTTP endpoints
4. `UpgradeCluster` returns 501 or triggers real upgrade (not fake DB update)
5. `TriggerAuditRun` creates `"pending"` status (not fake `"completed"`)
6. `go vet` passes on all modified packages
7. `go test` passes on all modified packages

## Verify Command
```
go vet ./internal/infrastructure/logging/... ./internal/infrastructure/kubernetes/... ./cmd/agent-runner/... ./internal/adapter/http/...
go test ./internal/infrastructure/logging/... ./internal/infrastructure/kubernetes/... ./internal/adapter/http/... -v -count=1
```
