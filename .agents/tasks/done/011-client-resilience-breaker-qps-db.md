# 011 — Configurable K8s Client QPS / Burst / Timeout

> Generated: 2026-10-08 | Revised after independent audit | Branch: fix/comprehensive-audit
> Priority: **P2** | Owner: `backend-coder` | Wave: 3 | Depends on: —
> Audit verdict: **REWRITE-SCOPE** (item 1 only)
> Status: OPEN

## Problem (auditor verified)
`internal/infrastructure/kubernetes/client.go:42-44` and `internal/infrastructure/cluster/manager.go:124-126` hardcode `QPS=50`, `Burst=100`, `Timeout=30s` in two places.

## Required change
Single config source (`internal/infrastructure/config/`, env `K8S_CLIENT_QPS`, `K8S_CLIENT_BURST`, `K8S_CLIENT_TIMEOUT`), defaults unchanged, used by both call sites. < 25 lines.

## Acceptance criteria
- [ ] Defaults identical to today; override test.
- [ ] Invalid values rejected at config load (fail fast).

## Verify
```
go test -race -count=1 -v ./internal/infrastructure/config/... ./internal/infrastructure/kubernetes/... ./internal/infrastructure/cluster/...
```

## Dropped (audit rulings)
- 429 helper → moved into 008 (only real consumer).
- Promote LLM breaker to `internal/pkg/resilience` → no second consumer.
- `postgres.IsSaturated()` + 503 → no caller, TOCTOU; rely on pgxpool queueing + ctx timeouts.
