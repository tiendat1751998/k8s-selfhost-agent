---
title: "Phase 11c: Clean Architecture & Security Enforcement (Enterprise Polish)"
status: "pending"
priority: "medium"
assignee: "backend-agent"
---

# Objective
Implement automated checking systems or testing scripts in the backend to ensure zero architectural boundary violations and zero SQL injection queries.

# Tasks
- [ ] **Architecture Boundary Validation**:
  - Add a unit test `TestArchitecture_Imports` (e.g. in `internal/domain/domain_test.go`) that scans all Go files under `internal/domain` and fails if any of them imports packages from `internal/infrastructure` or `internal/adapter`.
- [ ] **SQL Query Security Guard**:
  - Add a unit test `TestSecurity_SQLQueries` (e.g. in `internal/infrastructure/postgres/postgres_test.go`) that parses/analyzes postgres repository files to detect if query methods format SQL dynamically using format variables (`Sprintf`) or string concatenation (`+`). Ensure all database mutations are query parameter bound.

# Acceptance Criteria
- Running `go test ./...` automatically verifies compliance with clean imports and secure parameter-bound SQL statements.
