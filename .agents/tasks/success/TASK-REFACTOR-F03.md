# Task: Implement unified APIClient and remove fetch interceptors
ID: TASK-REFACTOR-F03
Status: success

## Objective
Remove the global `window.fetch` overrides in `auth.js` and `api-client.js`. Build the unified `APIClient` in `api-client.js`.

## Requirements
- Edit `frontend/modules/auth/auth.js` to remove the global `fetch` interceptor (lines 4-20). Keep standard auth logic and ensure `global.Auth` supports standard `logout` and redirecting to login.
- Edit `frontend/core/services/api-client.js` to remove the caching/cancellation fetch interceptor (lines 10-66).
- Build the unified `APIClient` structure in `api-client.js` with:
  - Base URL `/api/v1`
  - Token injection: automatically retrieves `k8s_token` from `localStorage` and adds as `Authorization: Bearer <token>`
  - Request caching: 3 seconds default cache for GET requests
  - Coalescing: coalesce duplicate concurrent active GET requests
  - Central 401 response handling: redirects/shows login modal when unauthorized error is returned.
  - Expose generic `APIClient.get`, `APIClient.post`, `APIClient.put`, `APIClient.delete`.
  - Maintain specific helper methods (`loadIncidents`, `loadReports`, `loadPRs`, `loadMetrics`) by wrapping them around `APIClient.get(...)`.

## Verification
- Code must load successfully without syntax errors in the browser.
