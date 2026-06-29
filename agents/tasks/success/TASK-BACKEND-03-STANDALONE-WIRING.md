# TASK-BACKEND-03-STANDALONE-WIRING: Wire Mock Handlers in Standalone Entrypoint

## Goal
Wire all missing HTTP platform handlers to their mock repositories inside `cmd/standalone/main.go` to activate all REST routes.

## Scope
- Instantiate all HTTP handlers inside `cmd/standalone/main.go` using the in-memory mock repositories created in TASK-BACKEND-02.
- Wire these handler instances to the `PlatformHandlers` struct in the standalone bootstrap loop.

## Success Criteria
- Standalone mode starts without panic.
- REST paths `/api/v1/drift`, `/api/v1/correlation`, `/api/v1/compliance`, etc., return JSON payloads instead of HTTP 404.
