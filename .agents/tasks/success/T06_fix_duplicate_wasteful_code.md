# TASK: Fix Duplicate & Wasteful Code

## Priority: 🔵 MEDIUM — Maintainability & performance
## Status: PENDING
## Estimated Effort: 30 minutes

---

## Problem Description

Multiple instances of duplicate code, redundant object creation, and wasteful patterns exist across the codebase.

## Sub-Tasks

### 6.1 — Remove duplicate `ObservabilityRepo` in `main.go`
- **File**: `cmd/server/main.go`
- **Problem**: Two separate repo instances created:
  - L154: `obsRepo := postgres.NewObservabilityRepo(pgClient.Pool())`
  - L266: `observabilityRepo := postgres.NewObservabilityRepo(pgClient.Pool())`
- **Fix**: Delete L266. Change L277 to use `obsRepo`:
  ```go
  // Before:
  Observability: adapthttp.NewObservabilityHandler(observabilityRepo),
  // After:
  Observability: adapthttp.NewObservabilityHandler(obsRepo),
  ```

### 6.2 — Fix inline `FleetRepo` creation in `main.go`
- **File**: `cmd/server/main.go` (L284)
- **Current**: `Fleet: adapthttp.NewFleetHandler(postgres.NewFleetRepo(pgClient.Pool()))`
- **Fix**: Create `fleetRepo` variable alongside other repos (around L254), pass to handler:
  ```go
  fleetRepo := postgres.NewFleetRepo(pgClient.Pool())
  // ...
  Fleet: adapthttp.NewFleetHandler(fleetRepo),
  ```

### 6.3 — Extract 391 inline styles from `index.html` into CSS
- **File**: `frontend/index.html` (391 occurrences of `style="..."`)
- **High-frequency patterns to extract**:
  - `display:flex;gap:var(--space-xs);` → `.flex-row-xs { display:flex;gap:var(--space-xs); }`
  - `display:flex;gap:var(--space-md);` → `.flex-row-md { display:flex;gap:var(--space-md); }`
  - `display:grid;grid-template-columns:repeat(N,1fr);gap:var(--space-md);` → `.grid-N-col { ... }`
  - `padding:var(--space-md);` → already exists in panel classes
  - `font-size:13px;color:var(--color-muted);` → `.text-muted-sm { ... }`
  - `margin-bottom:var(--space-md);` → `.mb-md { ... }`
- **Approach**: 
  1. Add utility classes to `frontend/css/enterprise.css`
  2. Replace inline styles in HTML with class names
  3. Target top 20 most-repeated patterns first
- **Note**: Don't remove ALL inline styles at once — some are truly one-off (like specific widths for dropdowns)

### 6.4 — Merge duplicate enterprise sections in HTML
- **File**: `frontend/index.html`
- **Two sections**: `section-enterprise` (L1270) and `section-enterprise-console` (L1347)
- **Cross-reference**: See Task T04 for detailed merge plan
- **This task**: Just tracks the code deduplication aspect

### 6.5 — Verify and clean up orphan `modules/core/websocket.js`
- **Problem**: `<script src="/modules/core/websocket.js">` was removed from script tags (correctly — `core/services/ws-client.js` handles WebSocket)
- **Check**: Does the file `frontend/modules/core/websocket.js` still exist? If so, verify no code references it and delete it.
- **Verification**: `grep -r "websocket.js" frontend/` returns zero results

## Files Involved
- `cmd/server/main.go` — duplicate repo cleanup
- `frontend/index.html` — inline style extraction
- `frontend/css/enterprise.css` — new utility classes
- `frontend/modules/core/websocket.js` — potentially delete

## Verification
- `go build ./cmd/server/` succeeds
- No duplicate repo variables in main.go
- Inline style count drops significantly (target: < 100)
