# Task: Move SQL and coordination logic from HTTP handlers to Usecase
ID: TASK-REFACTOR-B02
Status: success

## Objective
Extract database queries and orchestrations from presentation adapters (`auth_handler.go`, `search_handler.go`, `handler.go`) into new usecase services.

## Requirements
- Edit `internal/adapter/http/auth_handler.go`: move database querying and bcrypt credential matching into a new `AuthUsecase` or service in `internal/usecase/auth/`. The handler must only handle request/response JSON encoding and call the usecase.
- Edit `internal/adapter/http/search_handler.go`: move multi-source searching database query blocks into a new `SearchUsecase` or service in `internal/usecase/search/`. The handler must only invoke the usecase.
- Edit `internal/adapter/http/handler.go`: move incident creation fallback and PR/GitOps orchestration from handlers like `CreatePR` and `MergePR` to a new usecase service in `internal/usecase/`.

## Verification
- Code must compile with `go build ./...`
- Go unit and integration tests must pass.
