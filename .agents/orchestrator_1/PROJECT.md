# Project: k8sseflhost Phase 2 Code Quality Resolution

## Architecture & Survey Findings
- **Backend Architecture**: Go 1.26 Clean Architecture (`cmd/`, `internal/domain`, `internal/usecase`, `internal/adapter/http/`, `internal/infrastructure/postgres/`).
- **Database Layer**: PostgreSQL via `pgxpool.Pool`, migrations managed under `migrations/`.
- **HTTP Layer**: standard `net/http` / JSON decoding in `internal/adapter/http/`.
- **Deployment**: `deployments/k8s/` (and Helm chart in `deployments/helm/`).

## Feature Inventory
| # | Requirement | Description | Target Files/Packages | Milestone |
|---|-------------|-------------|-----------------------|-----------|
| 1 | R1 Multi-Tenant Data Isolation | Add `tenant_id` column + FK + NOT NULL constraint to `incidents`, `rca_reports`, `gitops_prs` in migration `026_tenant_isolation.up.sql`. Refactor repository SELECT queries to enforce `BuildTenantQuery`. | `migrations/026_tenant_isolation.*`, `internal/infrastructure/postgres/*.go` | M1 |
| 2 | R5 Missing Down Migrations | Create matching `.down.sql` files for migrations 001 through 020 in `migrations/`. | `migrations/001_*.down.sql` to `migrations/020_*.down.sql` | M1 |
| 3 | R2 Transaction Support | Define `DBTX` interface (`*pgxpool.Pool` & `pgx.Tx`), update repository constructors to accept `DBTX`, and implement `TransactionManager.RunInTx`. | `internal/infrastructure/postgres/dbtx.go`, `tx_manager.go`, `*_repo.go` | M2 |
| 4 | R3 HTTP Input Validation | Add input validation to request structs in `internal/adapter/http/`, return HTTP 400 with descriptive JSON error body on validation failure. | `internal/adapter/http/*.go` | M3 |
| 5 | R4 K8s Resource Limits | Add `resources.requests` and `resources.limits` (100m-500m CPU, 128Mi-256Mi RAM) to container specs in `deployments/k8s/*.yaml`. | `deployments/k8s/*.yaml` | M4 |

## Milestones
| # | Name | Scope | Dependencies | Status |
|---|------|-------|-------------|--------|
| M1 | Data Isolation & Down Migrations (R1, R5) | Migration 026 + 20 down migrations (001-020) + repo SELECT query tenant filtering | None | **DONE** |
| M2 | Transaction Support for Repositories (R2) | `DBTX` interface, repo constructor refactoring, `TransactionManager` | M1 | **DONE** |
| M3 | HTTP Input Validation (R3) | HTTP request struct validation + 400 Bad Request error handling | None | **DONE** |
| M4 | K8s Resource Limits (R4) | Deployment manifests resource requests & limits | None | **DONE** |

## Interface Contracts & Guidelines
- **DBTX Interface**:
  ```go
  type DBTX interface {
      Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
      Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
      QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
  }
  ```
- **TransactionManager**:
  ```go
  type TransactionManager interface {
      RunInTx(ctx context.Context, fn func(ctx context.Context) error) error
  }
  ```
- **HTTP 400 Error Format**:
  Uses `writeError(w, http.StatusBadRequest, "validation error message", err)` yielding `{"error": "...", "detail": "..."}`.

## Code Layout
- `migrations/`: SQL migration files (`001_*.up.sql` / `001_*.down.sql`, ... `026_tenant_isolation.up.sql` / `.down.sql`)
- `internal/infrastructure/postgres/`: Database abstraction, repositories, tenant_query.go, transaction manager
- `internal/adapter/http/`: HTTP handlers and request structs
- `deployments/k8s/`: Kubernetes manifest YAML files
