# 010 — Metrics Collection Dedup & Agent Tailer Backoff

> Generated: 2026-10-08 | Revised after independent audit | Branch: fix/comprehensive-audit
> Priority: **P2** | Owner: `backend-coder` | Wave: 3 | Depends on: —
> Audit verdict: **REWRITE-SCOPE** (items 1–3 kept, `sync.Pool` dropped)
> Status: OPEN

## Problem (auditor verified)
- `internal/usecase/metrics/collector.go:303-305` and `:348-351` — both tickers call `runCollection` → double collection/broadcast.
- `cmd/agent/container_tailer.go:186-193` — fixed 200ms retry, no backoff.
- `cmd/agent/container_tailer.go:244-250` — `seenAtLastTS` unbounded while timestamps do not advance.

## Required change
1. One `runCollection` per interval (net line reduction in `collector.go`, 459 now).
2. Backoff + jitter 200ms → 5s cap (`cenkalti/backoff/v5`), reset after a successful stream.
3. Cap `seenAtLastTS` (10k); on overflow reset + log once.

## Files
`internal/usecase/metrics/collector.go`, `cmd/agent/container_tailer.go`, their tests.

## Acceptance criteria
- [ ] Test: `runCollection` once per interval.
- [ ] Test: increasing delays to cap, reset after success (inject clock/backoff).
- [ ] Test: identical-timestamp flood keeps map ≤ cap.

## Verify
```
go test -race -count=1 -v ./internal/usecase/metrics/... ./cmd/agent/...
```

## Do NOT
`sync.Pool` for log_server scanner buffer — long-lived streams, no benefit (audit).
