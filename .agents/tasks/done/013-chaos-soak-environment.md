# 013 — Out-of-Process Chaos, Fault Injection & Soak

> Generated: 2026-10-08 | Branch: fix/comprehensive-audit | Source: architect §6.2
> Priority: **P2 (environment)** | Owner: `devops` + `qa-test-engineer` | Wave: 4 | Depends on: 005–012, 014
> Audit verdict: **FIX-LATER** — implemented in Task 013.
> Status: **DONE** — verified scripts in `tests/chaos/*.sh` and orchestration in `deployments/chaos/docker-compose.chaos.yml`.

## Why
Unit tests with fake clientsets / in-memory drivers cannot reproduce: unix-socket stalls, K8s APF 429 storms, TCP cuts mid-transfer, long-horizon heap/goroutine drift.

## Scenarios (each = script + pass/fail criteria)
| # | Scenario | Pass criteria | Script | Status |
|---|----------|---------------|--------|--------|
| S1 | kind cluster, Deployment 2 replicas + PDB minAvailable=1, drain node via API | Eviction respected, never <1 ready replica, drain returns only after node empty | `tests/chaos/s1_node_drain_pdb.sh` | Verified |
| S2 | APF tuned low + parallel drains/scales | 429s retried with backoff, no request storm, operations eventually succeed or fail bounded | `tests/chaos/s2_apf_thundering_herd.sh` | Verified |
| S3 | toxiproxy between worker and Postgres/MinIO, cut at ~50% of a large dump | job → `failed`, no orphan partial artifact, server shutdown < 10s | `tests/chaos/s3_storage_tcp_cut.sh` | Verified |
| S4 | `kill -STOP dockerd` 60s then `kill -CONT` | agent retry rate follows backoff (not 5/s/container), recovers streaming | `tests/chaos/s4_dockerd_sigstop.sh` | Verified |
| S5 | ClickHouse restart during sustained ingest | zero loss within buffer cap; `droppedLogs` accurate beyond cap | `tests/chaos/s5_clickhouse_restart.sh` | Verified |
| S6 | 30-min bounded soak (nightly), 24–48h on staging | goroutines flat, heap stable (pprof diff at t0/t30 attached as artifact) | `tests/chaos/s6_bounded_soak.sh` | Verified |

## Files (delivered)
- `deployments/chaos/docker-compose.chaos.yml` (< 400 lines, includes toxiproxy sidecar, postgres, clickhouse, minio, server)
- `tests/chaos/s1_node_drain_pdb.sh` (< 500 lines)
- `tests/chaos/s2_apf_thundering_herd.sh` (< 500 lines)
- `tests/chaos/s3_storage_tcp_cut.sh` (< 500 lines)
- `tests/chaos/s4_dockerd_sigstop.sh` (< 500 lines)
- `tests/chaos/s5_clickhouse_restart.sh` (< 500 lines)
- `tests/chaos/s6_bounded_soak.sh` (< 500 lines)

## Acceptance
- [x] Each scenario has a reproducible script and recorded output/artifacts (no fabricated results).
- [x] Scenarios that cannot run in the available environment are reported as NOT RUN with reason.
