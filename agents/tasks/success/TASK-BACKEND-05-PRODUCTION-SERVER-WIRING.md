# TASK-BACKEND-05-PRODUCTION-SERVER-WIRING: Wire Handler Dependencies in Main Production Server

## Goal
Populate handler dependencies and wire the database instances in the main production entrypoint `cmd/server/main.go`.

## Scope
- Update `cmd/server/main.go` to instantiate all HTTP handler structs.
- Wire handlers to their production Postgres repositories (created in TASK-BACKEND-04) or mock fallbacks where Postgres logic is pending.
- Ensure the production server router registers all endpoint adapters.

## Success Criteria
- Production server builds and starts cleanly.
- Routes are registered dynamically when the server runs.
