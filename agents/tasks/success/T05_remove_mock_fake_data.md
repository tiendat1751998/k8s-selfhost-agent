# TASK: Remove All Mock/Fake Data Remnants

## Priority: 🟠 MEDIUM — Fake data displayed to users
## Status: PENDING
## Estimated Effort: 60 minutes

---

## Problem Description

Multiple frontend modules still generate hardcoded mock data instead of calling real APIs. Multiple backend files contain dummy/placeholder logic.

## Sub-Tasks — Frontend

### 5.1 — `action-center.js`: Remove mock data generators
- **File**: `frontend/modules/actions/action-center.js` (L60-179)
- **Functions to remove**: `loadMockData()`, `generateMockPods()`, `generateMockDeployments()`, `generateMockStatefulSets()`, `generateMockNodes()`
- **Replace with**: API calls to `/api/v1/explorer/pods`, `/api/v1/explorer/deployments`, `/api/v1/explorer/statefulsets`, `/api/v1/explorer/nodes`
- **Fallback**: Show empty state with "No data available" message on API error

### 5.2 — `capacity-planning.js`: Remove mock charts and mock metrics
- **File**: `frontend/modules/capacity/capacity-planning.js` (L41-65, L110-132)
- **Remove**: `<div class="mock-chart-line">` elements, `renderMockMetrics()` function
- **Replace with**: Real data from `/api/v1/capacity` endpoint
- **Charts**: Use canvas-based mini charts or SVG bars instead of CSS div hacks

### 5.3 — `change-management.js`: Remove mock table
- **File**: `frontend/modules/change/change-management.js` (L108-158)
- **Remove**: `renderMockTable()` function and hardcoded mock rows
- **Replace with**: Empty state UI on API error: "Unable to load changes. Check connection."

### 5.4 — `command-palette.js`: Remove mock search results
- **File**: `frontend/modules/core/command-palette.js` (L70-83)
- **Remove**: `renderMockResults()` function with hardcoded payment-svc items
- **Replace with**: Show "No results found" or "Search requires API connection"

### 5.5 — `auth.js`: Remove hardcoded fake JWT token
- **File**: `frontend/modules/auth/auth.js` (L48)
- **Current**: `const fakeToken = "eyJhbGciOiJIUzI1NiIsInR5cCI...fake"`
- **Replace with**: A demo token format like `k8s-enterprise-demo-{timestamp}` that makes it clear it's not a real JWT
- **Long-term**: Implement real `POST /api/v1/auth/login` flow

## Sub-Tasks — Backend

### 5.6 — `auth.go`: Remove mock user/role assignment
- **File**: `internal/adapter/http/middleware/auth.go` (L42-44)
- **Current**: 
  ```go
  userID := "admin-user-id"
  userRole := "platform_admin"
  ```
- **Fix**: Parse the JWT token properly (at minimum decode the base64 payload). Use `golang-jwt/jwt` library or `crypto/hmac` for HMAC verification.

### 5.7 — `capacity_repo.go`: Remove hardcoded forecast values
- **File**: `internal/infrastructure/kubernetes/capacity_repo.go` (L57-102)
- **Current**: `CurrentUsage: 45.0, Forecast7d: 48.0` — all hardcoded numbers
- **Fix**: Parse `metricsResp.Items` and compute real CPU/Memory percentages. Calculate forecast trend from historical data (or at minimum use actual current values).

### 5.8 — `audit_handler.go`: Remove dummy trigger
- **File**: `internal/adapter/http/audit_handler.go` (L56)
- **Current**: `// Dummy trigger` comment with potentially fake logic
- **Fix**: Implement real audit trigger that creates proper audit entries

### 5.9 — `fleet_handler.go`: Remove dummy upgrade action
- **File**: `internal/adapter/http/fleet_handler.go` (L69)
- **Current**: `// Set status to upgrading as a dummy action`
- **Fix**: Implement real cluster upgrade workflow or return "not implemented" status

### 5.10 — `crypto.go`: Remove hardcoded fallback encryption key
- **File**: `internal/pkg/crypto/crypto.go` (L17)
- **Current**: `key = "12345678901234567890123456789012"` (insecure default)
- **Fix**: Panic or return error if `ENCRYPTION_KEY` env var is missing. Add startup validation in `cmd/server/main.go`.

## Files Involved
- Frontend: `action-center.js`, `capacity-planning.js`, `change-management.js`, `command-palette.js`, `auth.js`
- Backend: `auth.go`, `capacity_repo.go`, `audit_handler.go`, `fleet_handler.go`, `crypto.go`

## Verification
- No "mock", "fake", "dummy", "hardcoded" strings in non-test code
- Run: `grep -ri "mock\|fake\|dummy" --include="*.js" frontend/modules/ --include="*.go" internal/` → zero results (excluding test files)
