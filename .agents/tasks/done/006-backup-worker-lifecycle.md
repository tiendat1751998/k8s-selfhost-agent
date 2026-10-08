# 006 — Backup Worker: Context Inheritance, Prompt Shutdown, Stale Job Reconcile

> Generated: 2026-10-08 | Revised after independent audit | Branch: fix/comprehensive-audit | Source: ADR-007
> Priority: **P0** | Owner: `backend-coder` | Wave: 2 | Depends on: —
> Audit verdict: **FIX-NOW**
> Status: OPEN

## Problem (orchestrator + auditor verified)
- `internal/infrastructure/backup/worker.go:54,63` — job/restore ctx = `context.WithTimeout(context.Background(), 2*time.Hour)`, detached from pool lifecycle.
- `worker.go:93-96` — `Stop()` closes `stopChan` then `wg.Wait()`; an in-flight job ignores it → shutdown hangs up to 2h. Server registers `backupPool.Stop` (`cmd/server/bootstrap_services.go:203`) so the hang is real on SIGTERM.
- `internal/infrastructure/backup/engine.go:181-195` (auditor) — on cancel, `e.repo.UpdateJob(ctx, job)` runs with the **already-cancelled ctx** → status write fails → row stuck `running`.
- Correction to earlier draft: standalone does **not** instantiate a backup pool at all (no `NewWorkerPool` in `cmd/standalone`). Not in this task's scope — see Open question.

## Required change
1. `WorkerPool` owns `ctx, cancel`; job ctx derived from it. `Stop()` cancels, then waits.
2. Terminal status updates use a short detached ctx (`context.WithoutCancel(ctx)` + timeout ~5s), never the cancelled job ctx.
3. On cancel/error: status `failed` with reason; delete partial artifact via storage provider.
4. `ReconcileStaleJobs` on engine start = one parameterized `UPDATE ... SET status='failed', error_message='interrupted by restart' WHERE status='running'` (no background goroutine).

## Files
`internal/infrastructure/backup/worker.go`, `engine.go`, tests; `internal/domain/backup/` repository port (if a method is added); `internal/infrastructure/postgres/backup_repo.go`.
No changes to `cmd/server/bootstrap_services.go` expected (Stop already registered) — if needed, coordinate with 014.

## Acceptance criteria
- [ ] Test: blocking fake driver; `Stop()` returns < 1s; job ctx cancelled.
- [ ] Test: cancelled job ends `failed` in repo (proves update not using cancelled ctx); partial artifact deleted.
- [ ] Test: reconcile marks pre-existing `running` rows `failed`.
- [ ] All files < 500 lines.

## Verify
```
go test -race -count=1 -v ./internal/infrastructure/backup/... ./internal/infrastructure/postgres/...
go build ./cmd/server ./cmd/standalone && go vet ./...
```

## Open question (user decision)
Standalone mode has a backup usecase but no worker pool → enqueued backup jobs are never executed in standalone. Wire it, or disable backup endpoints in standalone?

## Not provable in CI → 013
Network cut mid large dump/upload.
