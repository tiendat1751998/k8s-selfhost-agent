# TASK-BACKEND-02-IN-MEMORY-MOCKS: Centralized Go In-Memory Mock Repository Layer

## Goal
Implement thread-safe in-memory mock repository representations under `internal/infrastructure/mock/` for all un-implemented domain data layers.

## Scope
- Create `internal/infrastructure/mock/repositories.go` with mock repository structs satisfying the domain interface definitions for:
  - `drift.Repository`
  - `correlation.Repository`
  - `tagging.Repository`
  - `compliance.Repository`
  - `runbook.Repository`
  - `observability.Repository`
  - `capacity.Repository`
  - `change.Repository`
  - `promotion.Repository`
  - `explorer.Repository`
  - `reporting.Repository`
  - `healthcenter.Repository`
  - `fleet.Repository`
  - `audit.Repository`
  - `notification.Repository`
  - `automation.Repository`
- Ensure simulated in-memory read/write state behaves deterministically.

## Success Criteria
- Mock structures compile and satisfy the interfaces cleanly.
