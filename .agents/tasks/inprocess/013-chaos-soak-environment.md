# 013 — Out-of-Process Chaos, Fault Injection & Soak

> Generated: 2026-10-08 | Branch: fix/comprehensive-audit | Source: architect §6.2
> Priority: **P2 (environment)** | Owner: `devops` + `qa-test-engineer` | Wave: 4 | Depends on: 005–012, 014
> Audit verdict: **FIX-LATER** — does not block code delivery of 005–014.
> Status: OPEN — **requires Linux/WSL2 with Docker, kind/k3d, toxiproxy; not part of `go test ./...`**

## Why
Unit tests with fake clientsets / in-memory drivers cannot reproduce: unix-socket stalls, K8s APF 429 storms, TCP cuts mid-transfer, long-horizon heap/goroutine drift.

## Scenarios (each = script + pass/fail criteria)
| # | Scenario | Pass criteria |
|---|----------|---------------|
| S1 | kind cluster, Deployment 2 replicas + PDB minAvailable=1, drain node via API | Eviction respected, never <1 ready replica, drain returns only after node empty |
| S2 | APF tuned low + parallel drains/scales | 429s retried with backoff, no request storm, operations eventually succeed or fail bounded |
| S3 | toxiproxy between worker and Postgres/MinIO, cut at ~50% of a large dump | job → `failed`, no orphan partial artifact, server shutdown < 10s |
| S4 | `kill -STOP dockerd` 60s then `kill -CONT` | agent retry rate follows backoff (not 5/s/container), recovers streaming |
| S5 | ClickHouse restart during sustained ingest | zero loss within buffer cap; `droppedLogs` accurate beyond cap |
| S6 | 30-min bounded soak (nightly), 24–48h on staging | goroutines flat, heap stable (pprof diff at t0/t30 attached as artifact) |

## Files (proposed)
`deployments/chaos/docker-compose.chaos.yml`, `tests/chaos/*.sh`, `.github/workflows/resilience-soak.yml` (only if `.github/workflows` exists — check first).

## Acceptance
- [ ] Each scenario has a reproducible script and recorded output/artifacts (no fabricated results).
- [ ] Scenarios that cannot run in the available environment are reported as NOT RUN with reason.
