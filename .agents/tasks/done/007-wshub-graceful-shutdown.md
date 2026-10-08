# 007 — WebSocket Hub: Graceful Shutdown & Unregister Block

> Generated: 2026-10-08 | Revised after independent audit | Branch: fix/comprehensive-audit | Source: ADR-001
> Priority: **P1** | Owner: `backend-coder` | Wave: 1 | Depends on: —
> Audit verdict: **FIX-NOW**
> Status: OPEN

## Problem (orchestrator + auditor verified)
- `internal/adapter/http/websocket.go:87` — `Run()` infinite loop, no ctx/stop.
- `cmd/server/bootstrap_services.go:74` — `wsHub.Run()` inside `eg.Go` ⇒ `eg.Wait()` never returns on SIGTERM.
- `cmd/standalone/standalone_wiring.go:63` — `go wsHub.Run()` (must be updated too or the build breaks).
- `websocket.go:187` — `readPump` defer blocking `c.hub.unregister <- c`.

## Required change
1. `Run(ctx context.Context)`: on `ctx.Done()` close all clients (close frame + close `send`), exit.
2. Unregister must not block after hub exit: `select { case h.unregister <- c: case <-h.done: }`. Do not use `default:` while hub is alive (would leak clients in the map).
3. Update both callers (server :74, standalone :63).

## Files
`internal/adapter/http/websocket.go`, its test, `cmd/server/bootstrap_services.go` (440 lines — surgical), `cmd/standalone/standalone_wiring.go` (436 lines — surgical).

## Acceptance criteria
- [ ] Test: cancel ctx → `Run` returns < 1s; connected test clients receive close.
- [ ] Test: 100 connect/disconnect cycles → `runtime.NumGoroutine()` returns to baseline within deadline.
- [ ] `go test -race` clean; both binaries build.

## Verify
```
go test -race -count=1 -v -run "WebSocket|WSHub" ./internal/adapter/http/...
go build ./cmd/server ./cmd/standalone
```
