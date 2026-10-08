# 012 — Diagnostics: Agent pprof + goleak Leak Gate

> Generated: 2026-10-08 | Branch: fix/comprehensive-audit | Source: architect ADR-008
> Priority: **P2** | Owner: `backend-coder` (+ `qa-test-engineer` review) | Wave: 3 | Depends on: 005, 006, 007 (leak gate should run on fixed code)
> Audit verdict: **FIX-NOW** — goleak needs user approval; until approved use stdlib `runtime.NumGoroutine()` baseline polling with deadline.
> Status: OPEN

## Problem (architect evidence, re-verify before coding)
- Server already has gated pprof: `internal/adapter/http/router.go:101-118` (`K8S_ENABLE_PPROF` + JWT + `platform_admin`).
- Agent has none: `cmd/agent/main.go:87-99`.
- No automated goroutine-leak detection in tests.

## Required change
1. Agent: mount `net/http/pprof` handlers only when `AGENT_ENABLE_PPROF=true`, behind the existing agent bearer-token auth. Default off.
2. Add `go.uber.org/goleak` (**new test-only dependency** — confirm with user before adding; stdlib fallback is `runtime.NumGoroutine` polling).
3. `TestMain` with `goleak.VerifyTestMain` in packages touched by 005/006/007 (clickhouse, backup, adapter/http websocket tests) and `cmd/agent`; ignore only documented third-party goroutines.

## Acceptance criteria
- [ ] pprof disabled by default; enabled + no token → 401; enabled + token → 200.
- [ ] goleak passes in listed packages with `-race`.
- [ ] `cmd/agent/main.go` < 500 lines (currently 391).

## Verify
```
go test -race -count=1 -v ./cmd/agent/... ./internal/infrastructure/clickhouse/... ./internal/infrastructure/backup/...
go test -race -count=1 ./internal/adapter/http/...
```
