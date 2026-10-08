# 009 — Helm & IaC: Context/Timeout, Keyed Locks, 409 Mapping

> Generated: 2026-10-08 | Revised after independent audit | Branch: fix/comprehensive-audit | Source: ADR-005
> Priority: **P1** | Owner: `backend-coder` | Wave: 2 | Depends on: —
> Audit verdict: **FIX-NOW**
> Status: OPEN

## Problem (auditor verified)
- `internal/infrastructure/helm/release_operations.go:209` `rollbackAction.Run(name)`, `:230` `uninstallAction.Run(name)` — ctx ignored.
- Unlocked concurrent writes to `repositories.yaml`: `internal/infrastructure/helm/release_values.go:94`, `:171` (earlier draft cited `release_manager.go:61` — that line only builds the path).
- `internal/infrastructure/iac/terraform.go:91`, `ansible.go:62` — no per-workdir lock, no default timeout.
- Legacy C1/C2 fake-success: already fixed.

## API facts (helm v3.21.4, per audit)
- `action.Rollback` has only `Run(name)` → set `rollbackAction.Timeout` from `ctx.Deadline()` (fallback 5m).
- `action.Uninstall` has `RunWithContext(ctx, name)` → use it.

## Required change
1. `internal/pkg/syncutil/keyed_mutex.go` (~40 lines, `TryLock`/`Unlock`, no interface).
2. Lock keys: Helm `cluster/ns/release` for install/upgrade/rollback/uninstall; IaC = cleaned absolute workdir. Contention → `ErrOperationInProgress`.
3. `sync.Mutex` around repositories.yaml read-modify-write (`release_values.go`).
4. IaC default timeout 15m when ctx has no deadline.
5. `internal/adapter/http/helm_handler.go` (**484 lines — max ~10 added lines**, centralize in existing error helper) maps `ErrOperationInProgress` → 409. Check IaC callers too.

## Acceptance criteria
- [ ] `-race`: 10 concurrent ops same key → 1 runs, 9 `ErrOperationInProgress`; different keys parallel.
- [ ] Handler test → 409.
- [ ] IaC with `context.Background()` bounded by (injectable) default timeout.
- [ ] `helm_handler.go` < 500 lines.

## Verify
```
go test -race -count=1 -v ./internal/pkg/syncutil/... ./internal/infrastructure/helm/... ./internal/infrastructure/iac/...
go test -race -count=1 ./internal/adapter/http/...
```
