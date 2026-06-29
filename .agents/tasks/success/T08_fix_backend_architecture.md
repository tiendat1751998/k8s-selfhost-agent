# TASK: Fix Backend Architecture Issues

## Priority: 🟤 MEDIUM — Logic bugs and missing features
## Status: PENDING
## Estimated Effort: 45 minutes

---

## Problem Description

Several backend components have logic bugs, missing endpoints, or incomplete implementations that affect the reliability of the system.

## Sub-Tasks

### 8.1 — Fix HealthPoller exponential backoff never resets
- **File**: `internal/usecase/ai/health_poller.go` (L53-80)
- **Problem**: The backoff algorithm increases `currentInterval` from base to 10x max, but **never resets** when a healthy response comes back. After a few poll cycles, the interval permanently stays at max (10 minutes if base is 1 minute).
- **Current code**:
  ```go
  currentInterval = time.Duration(float64(currentInterval) * 1.5)
  if currentInterval > maxInterval {
      currentInterval = maxInterval
  }
  timer.Reset(currentInterval)
  ```
- **Fix**: Reset interval after a successful poll:
  ```go
  hp.pollAll(ctx)
  
  // Reset backoff on successful poll (all providers healthy)
  results := hp.registry.HealthCheckAll(ctx)
  allHealthy := true
  for _, r := range results {
      if r.Status != "healthy" { allHealthy = false; break }
  }
  if allHealthy {
      currentInterval = hp.interval // Reset to base
  } else {
      currentInterval = time.Duration(float64(currentInterval) * 1.5)
      if currentInterval > maxInterval { currentInterval = maxInterval }
  }
  timer.Reset(currentInterval)
  ```

### 8.2 — Fix circuit breaker timeout (5s too short for LLM)
- **File**: `internal/infrastructure/llm/circuit_breaker.go`
- **Problem**: 5-second `context.WithTimeout` is too short for LLM completion requests which can take 10-30+ seconds for complex RCA prompts.
- **Fix**: Make timeout configurable via `CircuitBreakerConfig`:
  ```go
  type CircuitBreakerConfig struct {
      // ... existing fields
      RequestTimeout time.Duration // NEW: default 30s
  }
  ```
  Use `cfg.RequestTimeout` instead of hardcoded `5*time.Second`.

### 8.3 — Fix `capacity_repo.go` — parsed metrics never used
- **File**: `internal/infrastructure/kubernetes/capacity_repo.go` (L45-50)
- **Problem**: `json.Unmarshal(rawMetrics, &metricsResp)` parses the metrics-server response, but the actual `metricsResp.Items` data is **never used**. The code falls through to hardcoded values: `CurrentUsage: 45.0`.
- **Fix**: 
  1. Parse CPU from `metricsResp.Items[].Usage.CPU` (e.g., "250m" → 0.25 cores)
  2. Parse Memory from `metricsResp.Items[].Usage.Memory` (e.g., "1Gi" → 1073741824 bytes)
  3. Calculate actual percentages against allocatable resources
  4. Use real values in `Forecast` structs

### 8.4 — Create `POST /api/v1/auth/login` endpoint
- **New file**: `internal/adapter/http/auth_handler.go`
- **What**: Authentication endpoint that validates credentials against the `users` table (from `020_auth_rbac.sql`), issues a JWT token.
- **Request body**: `{ "email": "...", "password": "..." }`
- **Response**: `{ "token": "jwt...", "user": { "id": "...", "role": "..." } }`
- **Register**: Add to router at `/api/v1/auth/login` (OUTSIDE the JWT middleware group)
- **Dependencies**: `golang-jwt/jwt` package or manual HMAC signing

### 8.5 — Create `GET /api/v1/search` endpoint
- **New file**: `internal/adapter/http/search_handler.go`
- **What**: Global search endpoint that command palette calls
- **Query params**: `q` (search term), `type` (filter: all, kubernetes, log, git, incident)
- **Implementation**: Search across:
  - Incidents (by message, pod name)
  - Fleet clusters (by name)
  - Audit logs (by action, target)
- **Register**: Add to router at `/api/v1/search`

### 8.6 — Fix `scripts/` dual `main` packages
- **Files**: `scripts/check_db.go`, `scripts/migrate.go`
- **Problem**: Both declare `package main` → `go test ./...` fails with `main redeclared`
- **Fix**: Move to separate directories:
  ```
  scripts/check_db/main.go  (was scripts/check_db.go)
  scripts/migrate/main.go   (was scripts/migrate.go)
  ```

## Files Involved
- `internal/usecase/ai/health_poller.go`
- `internal/infrastructure/llm/circuit_breaker.go`
- `internal/infrastructure/kubernetes/capacity_repo.go`
- `internal/adapter/http/auth_handler.go` (NEW)
- `internal/adapter/http/search_handler.go` (NEW)
- `internal/adapter/http/router.go` — register new routes
- `scripts/check_db.go` → `scripts/check_db/main.go`
- `scripts/migrate.go` → `scripts/migrate/main.go`

## Verification
- `go test ./...` passes with zero failures
- `go build ./cmd/server/` succeeds
- Health poller resets interval after all providers report healthy
- LLM RCA requests don't timeout at 5s for complex prompts
- `POST /api/v1/auth/login` returns a valid JWT
- `GET /api/v1/search?q=test` returns search results
