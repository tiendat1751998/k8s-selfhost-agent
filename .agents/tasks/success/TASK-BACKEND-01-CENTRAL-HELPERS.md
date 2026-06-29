# TASK-BACKEND-01-CENTRAL-HELPERS: Centralize JSON and REST Response Helpers

## Goal
Centralize duplicate package-private REST serialization and parsing helpers inside Go HTTP adapter package into unified utilities.

## Scope
- Create public/centralized utility methods (`WriteJSON`, `WriteError`, `ParseIntParam`, `ParseBoolParam`) in the HTTP adapter codebase.
- Refactor all Go HTTP handler files (e.g. `drift_handler.go`, `correlation_handler.go`, `tagging_handler.go`, `runbook_handler.go`) to use these central helper functions.
- Delete duplicated package-private helper declarations.

## Success Criteria
- Backend compiles and all tests pass with no errors.
- Duplicate helper declarations are eliminated.
