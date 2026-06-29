---
title: "Mock Data Removal, Live Client Integration, and E2E Testing"
status: "completed"
priority: "critical"
assignee: "backend-agent"
---

# Objective
Remove all fallback mock arrays and mock rendering scripts from both the backend Go code and frontend JavaScript client. Connect the control plane dynamically to real Kubernetes and Docker Swarm clusters via `ClientManager` decryption keys, and run validation tests.

# Scope
- `internal/infrastructure/kubernetes/explorer_repo.go` (Remove static fallback mock JSON lists)
- `internal/infrastructure/kubernetes/capacity_repo.go` (Remove fallback CPU/Memory estimation cards)
- `internal/infrastructure/kubernetes/healthcenter_repo.go` (Query live postgres, docker, and k8s node readiness states)
- `frontend/modules/drift/drift-detection.js` (Remove frontend renderMockTable backup, query live drift changes)
- `frontend/modules/healthcenter/health-center.js` (Remove client-side renderMockComponents fallback)
- `frontend/modules/provider/docker-swarm.js` (Remove legacy fallback template lists)

# Tasks
- [x] Refactor explorer_repo.go to throw errors when no live provider connection is reachable instead of silently mock listing.
- [x] Update capacity_repo.go to return live metrics API/Docker info calculations only, removing fallbackForecast entirely.
- [x] Upgrade healthcenter_repo.go to read dynamic database, swarm socket, and k8s API server status states.
- [x] Clean up health-center.js and drift-detection.js frontends to only render raw API responses.
- [x] Build and verify correct compilation of standalone and production server commands.
- [x] Test real-world operations on the UI using browser subagents to ensure zero fake data banners appear.

# Acceptance Criteria
- No hardcoded mock JSON arrays are returned for explorer list queries.
- Capacity dashboard values react directly to scaling operations.
- The UI handles down connection states gracefully by presenting clean connection alerts rather than generating mock components.
