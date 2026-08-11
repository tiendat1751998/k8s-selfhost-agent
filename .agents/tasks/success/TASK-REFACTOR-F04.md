# Task: Migrate frontend JS modules to use unified APIClient
ID: TASK-REFACTOR-F04
Status: success

## Objective
Replace direct ad-hoc `fetch` calls across all JavaScript modules in `frontend/modules/` with the unified `APIClient` methods.

## Requirements
- Scan for all direct fetch calls inside `frontend/modules/` (e.g. `action-center.js`, `agents-dashboard.js`, `ai-providers.js`, `audit-mode.js`, `auth.js`, `capacity-planning.js`, `change-management.js`, `kubernetes.js`, `nodes.js`, `pods.js`, `scaling.js`, `command-palette.js`, `event-correlation.js`, `cost-management.js`, `deployment-catalog.js`, `deployment-center.js`, `deployment-wizard.js`, `rollouts.js`, `drift-detection.js`, `resource-explorer.js`, `fleet-view.js`, `gitops.js`, `health-center.js`, `incidents-page.js`, `observability.js`, etc.)
- Replace calls like `fetch('/api/v1/incidents')` with `APIClient.get('/incidents')`.
- Replace POST/PUT/DELETE requests with `APIClient.post`, `APIClient.put`, `APIClient.delete`.
- Ensure all returned data parses JSON correctly and handles response objects properly.

## Verification
- No JS errors should exist in browser console.
- Dashboard views must load and render data correctly.
