# 005 — ClickHouse Batch Writer: No Silent Loss on Flush Failure

> Generated: 2026-10-08 | Branch: fix/comprehensive-audit | Source: architect ADR-004 (simplified by planner)
> Priority: **P0** | Owner: `backend-coder` | Wave: 1 | Depends on: —
> Status: OPEN

## Problem (verified by orchestrator)
`internal/infrastructure/clickhouse/batch_writer.go:243-253` — after `w.executeFlush(ctx, buf)` the code runs `buf = buf[:0]` **regardless of error**. Any transient ClickHouse outage (restart, DDL, network blip) permanently drops the whole batch (up to BatchSize entries). Same pattern on the shutdown path around `:225-237`.

## Required change
1. On flush error keep the entries; retry with exponential backoff + jitter (`github.com/cenkalti/backoff/v5`, already in go.mod) up to 3 attempts per batch.
2. Bound pending memory (existing channel/buffer capacity config). On overflow drop **oldest**, log at Error, increment an atomic `droppedLogs` counter exposed via existing status/metrics.
3. On ctx cancel / `Close()`: final flush with bounded timeout (5s); return the error if it fails (no silent success).

## Acceptance criteria
- [ ] Flush failure does not clear the buffer; entries are re-sent on next attempt (test with flushFn failing N times then succeeding → zero loss).
- [ ] Persistent failure: memory stays bounded at configured cap; `droppedLogs` equals the exact number of evicted entries.
- [ ] Shutdown attempts final flush; failure is surfaced, not swallowed.
- [ ] `batch_writer.go` < 500 lines (currently 340).

## Verify
```
go test -race -count=1 -v ./internal/infrastructure/clickhouse/...
go vet ./internal/infrastructure/clickhouse/...
```

## Not provable in CI → Task 013
ClickHouse restart under sustained ingest load.

## Do NOT
Disk spool / WAL / zstd replay worker (rejected as over-engineering by planner).
