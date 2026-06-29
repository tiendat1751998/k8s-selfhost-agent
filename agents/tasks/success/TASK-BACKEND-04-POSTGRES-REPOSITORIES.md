# TASK-BACKEND-04-POSTGRES-REPOSITORIES: Postgres DB Repository Mapping

## Goal
Implement PostgreSQL-backed repositories for key domain entities in `internal/infrastructure/postgres/` mapping to their SQL migrations schema.

## Scope
- Write real SQL repository implementations under `internal/infrastructure/postgres/` using `pgxpool` for:
  - `drift.Repository`
  - `correlation.Repository`
  - `compliance.Repository`
  - `tagging.Repository`
- Map Go structures to tables defined in migrations `005_drift_detection.sql`, `006_event_correlation.sql`, etc.

## Success Criteria
- Postgres repositories compile cleanly.
- Database CRUD queries behave correctly in tests.
