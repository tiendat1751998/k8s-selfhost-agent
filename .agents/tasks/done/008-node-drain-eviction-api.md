# 008 — Node Drain via Eviction API + 429 Backoff + Termination Wait

> Generated: 2026-10-08 | Revised after independent audit | Branch: fix/comprehensive-audit | Source: ADR-006 (+ 429 helper moved here from 011)
> Priority: **P1** | Owner: `backend-coder` | Wave: 1 | Depends on: —
> Audit verdict: **FIX-NOW**
> Status: OPEN

## Problem (auditor verified)
`internal/infrastructure/kubernetes/resource_operations_node.go:100` raw `CoreV1().Pods(ns).Delete` (bypasses PDB); `:105` returns `nil` without waiting.

## API facts (client-go v0.36.2, per audit)
- Use `client.CoreV1().Pods(ns).EvictV1(ctx, &policyv1.Eviction{...})`. `PolicyV1().Evictions(ns).Evict` does **not** exist.
- Fake clientset: assert via `client.Actions()` with `GetSubresource() == "eviction"`; simulate 429 with `PrependReactor("create", "pods", ...)` filtering on subresource.

## Required change
1. Evict via `EvictV1`; keep DaemonSet/mirror-pod skip rules.
2. 429 (PDB) → retry with `cenkalti/backoff/v5` bounded by ctx; on give-up return error naming the blocking pod.
3. Poll until no evictable pods remain on the node or ctx deadline.

## Acceptance criteria
- [ ] Test asserts eviction subresource action, zero `delete pods` actions.
- [ ] 429×N then success → ok; persistent 429 → bounded error.
- [ ] Never returns success while evictable pods remain.
- [ ] File < 500 lines (317 now).

## Verify
```
go test -race -count=1 -v ./internal/infrastructure/kubernetes/...
go test -race -count=1 ./internal/usecase/sre/...
```

## Not provable in CI → 013
Real PDB enforcement on kind/k3d.
