---
title: "Phase 4: Frontend UI Wiring & Event Binding"
status: "completed"
priority: "high"
assignee: "frontend-agent"
---

# Objective
Ensure that 100% of the buttons and forms on the web interface are fully functional and correctly call the real backend APIs. Remove all client-side mock logic (`console.log`, fake `alert()`).

# Scope
Audit and update all JavaScript modules in `frontend/modules/`:
1. `audit-mode.js`
2. `explorer-view.js`
3. `kubernetes.js` (Verify Test/Remove actions)
4. `fleet-view.js`
5. `enterprise-tenancy.js`

# Tasks
- [x] Replace all mock button event handlers (e.g., "Run Automation", "Promote Version") with `fetch()` calls to the corresponding `/api/v1/...` endpoints.
- [x] Add robust error handling (try/catch) and loading spinners for all async operations.
- [x] Ensure payload structures sent via `POST/PUT` strictly match the Go backend's JSON expectations.
- [x] Remove any hardcoded mock data structures generated directly in the JS layer.
- [x] Test the full end-to-end flow from the browser to the Postgres database/Kubernetes API.

# Acceptance Criteria
- Clicking any actionable button triggers a real network request.
- The UI gracefully handles API errors (e.g., 500 Internal Server Error, 400 Bad Request) and displays appropriate toast notifications.
- No buttons remain as "placeholder" or "mock".
