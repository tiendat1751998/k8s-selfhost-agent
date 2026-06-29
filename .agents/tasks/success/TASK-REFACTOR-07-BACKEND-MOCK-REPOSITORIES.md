# TASK-REFACTOR-07-BACKEND-MOCK-REPOSITORIES: Backend Mock Repository Layer

## Goal
Create thread-safe, in-memory repository implementations inside a centralized mock package for all domain repositories that are currently un-implemented in the Postgres directory.

## Scope
- Create `internal/infrastructure/mock/repositories.go` containing mock repository structs implementing domain repository interfaces for:
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
- Ensure mock datasets match the structures expected by the frontend.

## Success Criteria
- Mock repositories compile cleanly and satisfy their respective domain interfaces.
