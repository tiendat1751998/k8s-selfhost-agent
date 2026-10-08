# 014 — Server Docker Log Streamer Leak, Standalone Postgres Pool, Event Watcher Retry

> Generated: 2026-10-08 | Source: independent audit "missed risks" | Branch: fix/comprehensive-audit
> Priority: **P1** | Owner: `backend-coder` | Wave: 2 | Depends on: 007 (shares cmd wiring files)
> Status: OPEN

## Problems
1. **P1 goroutine leak / shutdown hang** — `cmd/server/bootstrap_services.go:370-399` (`ContainerLogs` at :371, orchestrator verified): per-container goroutine blocks in `stdcopy.StdCopy` on the Docker HTTP body; `reader` is never closed on `ctx.Done()`, plus 2 `pipeStream` goroutines each. Standalone already handles this correctly in `cmd/standalone/docker_log_streamer.go:79-88`.
2. **P1 unconfigured pool** — `cmd/standalone/standalone_wiring.go:69` uses raw `pgxpool.New(ctx, dsn)` (orchestrator verified), bypassing `internal/infrastructure/postgres/client.go` `NewClient` (pool limits, health check, lifetimes, connect timeout, startup retry).
3. **P2 silent death** — `cmd/standalone/docker_log_streamer.go:171-178`: first Docker events error returns from the watcher permanently; new containers are never tailed until restart.

## Required change
1. Close the reader on ctx cancel (prefer reusing the standalone streamer logic instead of duplicating; net line reduction in `bootstrap_services.go`).
2. Standalone uses `postgres.NewClient(ctx, cfg.Postgres)`; adapt call sites to its pool accessor.
3. Re-subscribe events with backoff (`cenkalti/backoff/v5`) until ctx done; log at Warn, not Debug.

## Files
`cmd/server/bootstrap_services.go` (440), `cmd/standalone/standalone_wiring.go` (436), `cmd/standalone/docker_log_streamer.go`, tests.

## Acceptance criteria
- [ ] Test (fake Docker client with blocking body): cancel ctx → streamer goroutines exit within deadline (goroutine count back to baseline).
- [ ] Standalone pool settings come from config (test or build + code evidence).
- [ ] Test: events channel errors once → watcher re-subscribes.
- [ ] Both wiring files stay < 500 lines.

## Verify
```
go test -race -count=1 -v ./cmd/server/... ./cmd/standalone/...
go build ./cmd/server ./cmd/standalone && go vet ./...
```
