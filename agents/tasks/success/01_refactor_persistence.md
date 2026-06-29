---
title: "Phase 1: Persistence Layer Refactoring (PostgreSQL)"
status: "completed"
priority: "high"
assignee: "backend-agent"
---

# Objective
Replace all remaining mock repositories with real PostgreSQL-backed repositories to ensure data persistence across restarts. The system must completely eliminate in-memory mock state for CRUD operations.

# Scope
Refactor the following domains from `mock.Repo` to `postgres.Repo`:
1. **Audit Logs & Timeline**: `internal/domain/audit`, `internal/domain/timeline`
2. **Runbook & Automation**: `internal/domain/runbook`, `internal/domain/automation`
3. **Changes & Promotion**: `internal/domain/changes`, `internal/domain/promotion`
4. **Notification**: `internal/domain/notification`
5. **Explorer, Reporting, HealthCenter, Capacity, Observability**: (Note: some of these will be handled in Phase 2/3 via live APIs, but any configuration data for them must be persisted).

# Tasks
- [x] Create missing database migrations in `migrations/` folder for `audit_logs`, `timeline_events`, `runbooks`, `automations`, `changes`, `promotions`, and `notification_configs`.
- [x] Implement `postgres.AuditRepo`, `postgres.TimelineRepo`, etc. in `internal/infrastructure/postgres/`.
- [x] Ensure all Postgres repositories implement their respective Domain interfaces correctly.
- [x] Update `cmd/server/main.go` and `cmd/standalone/main.go` to inject the new Postgres repos instead of `mockRepos.New...()`.
- [x] Verify that HTTP handlers correctly process and return the persisted data.

# Acceptance Criteria
- No `mockRepos` are used for the domains listed in the scope.
- Data created via API persists after restarting `server.exe` or `standalone.exe`.
