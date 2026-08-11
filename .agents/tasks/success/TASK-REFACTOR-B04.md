# Task: Fix database connection leaks in search_handler.go and clean junior patterns
ID: TASK-REFACTOR-B04
Status: success

## Objective
Fix concurrent database cursors connection leak in `search_handler.go` by scoping row closures, deduplicate LLM initialization logic, and clean up junior patterns.

## Requirements
- Edit `internal/adapter/http/search_handler.go`: refactor database query blocks so that pgx rows are closed immediately using localized scopes or helper functions, rather than sequential deferred closures at the function end.
- Extract identical LLM provider registry parsing / builder code in `cmd/server/main.go` and `cmd/agent-runner/main.go` into a shared builder helper function in `internal/infrastructure/llm/`.
- Clean junior patterns: replace `interface{}` with Go's `any` in updated areas, and address error check in `MarkFailed()` if necessary.

## Verification
- Code must compile with `go build ./...`
- Go unit and integration tests must pass.
