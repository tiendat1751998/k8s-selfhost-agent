# TASK-REFACTOR-04-REST-API-BOILERPLATE: Backend REST API Cleanup

## Goal
Audit backend Go HTTP adapter packages and centralize JSON serialization, response parsing, and standard HTTP error formatting into shared utilities.

## Scope
- Refactor private helper functions (`writeJSON`, `writeError`, `parseIntParam`) currently duplicated within `internal/adapter/http/handler.go` and various handlers (such as `drift_handler.go` or `correlation_handler.go`) to use unified public utilities.
- Clean up any REST boilerplate code in Go packages.

## Success Criteria
- Backend compiles cleanly and `go test ./...` passes.
- REST responses are structured and consistent across all API routes.
