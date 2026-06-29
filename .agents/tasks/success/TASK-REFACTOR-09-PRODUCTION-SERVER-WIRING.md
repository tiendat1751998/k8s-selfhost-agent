# TASK-REFACTOR-09-PRODUCTION-SERVER-WIRING: Wire Handlers in Production Server

## Goal
Populate handler dependencies in the main production entry point to ensure API endpoints are available on the production server.

## Scope
- Update `cmd/server/main.go` to instantiate all HTTP handlers.
- Wire repositories (using Postgres repositories where available, or mock instances for currently unimplemented domain stores).
- Assign all handlers to the `PlatformHandlers` struct in the server start routine.

## Success Criteria
- Production server compiles cleanly.
- Routes are registered dynamically when the server runs.
