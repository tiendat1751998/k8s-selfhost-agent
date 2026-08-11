# Phase 2 Critical Code Quality Issues Resolution Plan

## Overview
Survey completed by Explorers 1, 2, and 3. The implementation is decomposed into 4 milestones:

- **Milestone 1 (M1: R1 + R5)**: Data Isolation & Down Migrations
  - Migration 026 to add `tenant_id` (NOT NULL, FK to `organizations(id)`) to `incidents`, `rca_reports`, `gitops_prs`.
  - Add missing `.down.sql` files for migrations 001 through 020, plus 026.down.sql.
  - Refactor all SELECT queries in `internal/infrastructure/postgres/` repositories to use `BuildTenantQuery`.
- **Milestone 2 (M2: R2)**: Transaction Support for Repositories
  - Define `DBTX` interface implemented by `*pgxpool.Pool` and `pgx.Tx`.
  - Refactor repository constructors to accept `DBTX`.
  - Implement `TransactionManager` with `RunInTx(ctx, fn)`.
- **Milestone 3 (M3: R3)**: HTTP Input Validation
  - Implement input validation on request body structs across 26 HTTP handler endpoints in `internal/adapter/http/`.
  - Return HTTP 400 Bad Request with JSON error response when validation fails.
- **Milestone 4 (M4: R4)**: Kubernetes Resource Limits
  - Create/update K8s workload manifests in `deployments/k8s/*.yaml` with CPU (100m/500m) and Memory (128Mi/256Mi) requests/limits.

## Execution Strategy
Each milestone will be executed using the Project Pattern iteration loop:
1. **Explorer**: Technical investigation & concrete fix plan for the milestone.
2. **Worker**: Implement code/config changes, run build (`go build ./...`) and tests.
3. **Reviewer**: Code review & correctness check.
4. **Challenger**: Verification & stress testing.
5. **Forensic Auditor**: Integrity verification (binary veto).
6. **Gate**: Evaluate pass criteria.
