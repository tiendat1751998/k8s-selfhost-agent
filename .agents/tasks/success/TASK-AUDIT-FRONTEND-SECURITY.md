# Task: Consolidate Fetch Interceptors & Resolve Security Bypasses

Refactor authentication and network logic to enforce strict token handling and centralized interceptors.

## Scope of Work

### 1. Consolidate window.fetch Interceptors
- **Files**: `frontend/modules/auth/auth.js` (lines 4–20) and `frontend/core/services/api-client.js` (lines 10–66)
- **Fix**: Centralize all fetch hook logic (caching, request coalescing, navigation abort, JWT token injections, 401 intercepting) into a single module-scoped system inside `frontend/core/services/api-client.js`.

### 2. Remove Authentication Bypass Fallback Token
- **File**: `frontend/modules/auth/auth.js` (lines 91–95)
- **Fix**: Remove the logic that automatically registers a mock `'k8s-enterprise-demo-token'` if credentials are absent. Enforce authentic validation routes to safeguard the API endpoints.
