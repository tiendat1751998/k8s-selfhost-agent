# Task: Backend Go Ports and Dependency Flow Refactoring
ID: TASK-REFACTOR-B01
Status: success

## Objective
Define domain ports (interfaces) for NATS, Docker, and LLM clients. Refactor the usecase layer to reference only these ports, eliminating reverse dependency imports.

## Requirements
- Identify reverse dependency violations where usecase files import `internal/adapter/event`, `internal/infrastructure/llm`, or `internal/infrastructure/nats`.
- Define port interfaces inside the domain layer (e.g. `internal/domain/ports/` or appropriate domain/usecase packages) for:
  - NATS publisher / broker client.
  - Docker builder / sandbox environment wrapper.
  - LLM Completion client.
  - Incident event collector.
- Refactor the usecases (`pipeline.go`, `worker.go`, `agents_impl.go`, `orchestrator.go`, and `health_poller.go`) to reference these port interfaces instead of concrete packages.
- Ensure main cmd packages wire the concrete adapters to the usecases at startup.

## Verification
- Code must compile with `go build ./...`
- Go unit and integration tests must pass.
