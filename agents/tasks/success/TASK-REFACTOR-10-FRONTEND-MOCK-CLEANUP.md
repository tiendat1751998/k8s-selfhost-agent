# TASK-REFACTOR-10-FRONTEND-MOCK-CLEANUP: Clean Up Frontend Mock Data Fallbacks

## Goal
Remove local mock data generators and fallbacks from the frontend module scripts, enforcing a strict client-server communication contract.

## Scope
- Scan and modify all client-side module files (e.g. `drift-detection.js`, `event-correlation.js`, `capacity-planning.js`, `reporting-center.js`, `audit-mode.js`, etc.).
- Delete hardcoded mock list structures.
- Simplify logic to fetch from `/api/v1/...` and handle loading, empty, and HTTP error states cleanly in the DOM.

## Success Criteria
- Frontend scripts do not contain copy-pasted layout mock data.
- UI relies entirely on backend REST payloads.
