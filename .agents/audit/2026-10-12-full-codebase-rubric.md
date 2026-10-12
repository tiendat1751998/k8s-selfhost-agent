# Full-Codebase Audit Rubric — 2026-10-12

Baseline: branch `fix/comprehensive-audit`, HEAD `99ae1ed`.
Mindset: **assume the code is a toy project until proven otherwise.** Previous "production ready" claims in
`.agents/memory/project_state.md` and `decision_log.jsonl` are UNVERIFIED. Do not trust comments or commit messages; read the code.

## Rules for auditors (read-only)
1. Read every file in your slice, including `_test.go` / `*.test.ts`. You may not edit anything.
2. Every finding MUST cite `path:line` and quote the offending line(s) (≤ 3 lines).
3. No speculation. If unsure, mark `NEEDS-VERIFY` and say what command/test would prove it.
4. Coverage manifest is mandatory: list every file in the slice with line count and `READ` / `SKIMMED` / `SKIPPED (reason)`.
   Never say "no issues" for a file you did not READ.
5. Do not recommend new dependencies unless you show measured evidence (benchmark/pprof) that stdlib is the bottleneck.

## Categories (tag each finding with one)
| Tag | What to look for |
|---|---|
| FAKE | Hardcoded sample/demo/fallback data, Math.random, fake latency/percentages, stub handlers returning success, TODO bodies, data shown to users without a real source |
| ERR | Swallowed errors (`_ =`, `, _ :=`, empty `if err != nil {}`, `catch {}`), errors logged but success returned, lost error context |
| SEC | Missing authz/tenant checks, trusting client tenant IDs, cross-tenant fallbacks (e.g. `ListAll` when tenant query fails), secrets in code/logs, SQL built with fmt/concat, path traversal, SSRF, missing input bounds |
| CONC | Data races, unsynchronized maps, goroutines without exit path, missing `ctx.Done()`, `context.Background()` in request/job paths, unbounded goroutine fan-out |
| BOUND | Unbounded slices/maps/channels/caches, `io.ReadAll` on untrusted bodies, no pagination/limit, no timeout on HTTP/DB/K8s calls |
| PERF | Allocations in hot loops, N+1 queries, repeated full-list API calls (e.g. listing all pods to find one), lock contention, O(n²) on request path, polling where watch/informer is standard |
| OPS | Missing graceful shutdown, no retries/backoff where needed, non-idempotent mutations, missing health/readiness semantics, no metrics for queues/drops, config not validated at startup |
| DOMAIN | Logic that is wrong for Kubernetes/Docker/DB-ops domain semantics (e.g. treating Deployment name as pod name, ignoring owner references, wrong restart/scale semantics, backup without restore verification) |
| TEST | Tests that assert nothing, assert on mocks only, regex-on-source-file "tests", tests that would pass with the feature deleted |
| ARCH | File > 500 lines, layer violations (adapter importing infrastructure directly, domain importing infra), dead code, duplicate implementations |

## Severity
- **P0**: data loss, security/tenant breach, operator shown false state (fake healthy), crash/leak under normal load.
- **P1**: wrong behavior under realistic failure (timeouts, partial outage), missing bounds, swallowed errors on important paths.
- **P2**: performance, maintainability, test quality, style with real cost.

## High-throughput / go-disruptor question
For any hot path in your slice (log ingest, metrics collection, WebSocket broadcast, event bus):
- Identify the queue mechanism (channel, mutex+slice, ring buffer) and its capacity.
- State whether there is *evidence* (benchmark, pprof, measured rate) that the queue is a bottleneck. If not, say "no evidence".
- Do NOT recommend LMAX/go-disruptor unless contention on the queue itself is demonstrated; note that go-disruptor's
  low-latency mode busy-spins and needs dedicated cores, which conflicts with a lightweight self-hosted agent.

## Output format (send via send_message to the orchestrator)
1. Coverage manifest (table).
2. Findings table: `ID | Sev | Tag | path:line | Quoted code | Real-world failure scenario | Fix direction (1 line)`.
3. Hot-path / queue analysis (if any in slice).
4. Top 5 findings for your slice, ranked.
