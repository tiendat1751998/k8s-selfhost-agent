# Project State — Checkpoint 2026-10-23 (Honest Gap Audit & Active Remediation)

## Branch: `fix/comprehensive-audit`
## Active Task: [`.agents/tasks/067-end-to-end-reality-remediation.md`](../tasks/067-end-to-end-reality-remediation.md)
## Active Audit Report: [`.agents/audit/2026-10-23-real-system-gap-audit.md`](../audit/2026-10-23-real-system-gap-audit.md)

---

## 1. Current Ground-Truth Reality (2026-10-23)

### What Is Completed & Verified:
1. **High-Scale ClickHouse-Style Log Engine (`internal/pkg/logengine`)**:
   - Verified on a **100% real `12.16 GB` (`67,461,120` rows / `32,940` blocks)** physical disk dataset (`TestReal10GB`):
     - Ingestion: `143.84 MB/s` (`779,389 rows/sec`), `26.96 : 1` compression (`495.51 MB` on disk), `2.81 MB` peak Go heap.
     - Latest Tail (`Limit: 50`): `1.49 ms`.
     - 5-Min Time Window: `3.97 ms`.
     - Full-Scan Needle (`DEADLOCK_TX_999999`): `105.79 ms` (`ExactMatch=false`).
     - **Blind Error Radar (`SummarizeErrors` with zero keywords)**: `37.70 ms` across `12.16 GB`.
2. **PostgreSQL `slo_definitions.tenant_id` Fix (`observability_repo.go`)**:
   - Column `tenant_id UUID` added on `10.10.10.133:5432` and `withValidTenantUUID(ctx)` implemented so background workers no longer trigger `22P02: invalid input syntax for type uuid: ""`.
3. **`/logs` 2-Zone Target Menu & Log Coalescing (`commit efd47ecc`)**:
   - Redesigned `LogTargetTree.vue` into 2 clean zones (`NODES` + `SERVICES` segmented tabs `All | 📦 Apps | ⚙️ System`), removed duplicate `LogNodeServiceBar`, and added `×N` duplicate badge + `+N lines` SQL coalescing.

### Active Defects Being Remediated in Task 067 (DO NOT FORGET ACROSS SESSIONS):
1. **`GAP-01` & `GAP-02` — K8s API (`10.10.10.60:6443`) Down Cascades into `503` / `null` on `/deployments`, `/explorer`, `/helm`**:
   - Cluster `k8snode` (`cluster-mtay50kl`) is stuck in `status = "upgrading"` and `10.10.10.60:6443` refuses TCP connections.
   - `GET /api/v1/deployments` returns `{"data": null}`, and `/explorer` + `/helm/.../releases` return `503 Service Unavailable`.
2. **`GAP-03` — 5 Ghost Disconnected Hosts in PostgreSQL (`10.10.10.150`, `.151`, `.152`, `.200`, `.201`)**:
   - Pollutes `/hosts`, `/overview`, `/incidents`, and `/logs (+ 5 offline)`. Only `.60`, `.133`, and `.80` are real servers.
3. **`GAP-04` — Empty Database Tables for `/slo`, `/audit`, and `/automation`**:
   - `slo_definitions`, `compliance_frameworks`/`violations`, and `automation_rules` return `[]`, making those 3 views look empty/unfinished.
4. **`GAP-05`, `GAP-06`, `GAP-07` — Residual `/logs` Bugs**:
   - `traefik.service` JSON logs (`{"level":"error",...}`) are tagged `INFO` (green) by `journal_tailer.go` (`PRIORITY=6`) and rendered as `INFO` in History mode instead of `ERR` (red).
   - Postgres `[677906] CONTEXT:` and `[677906] STATEMENT:` lines have a leading `[PID]` that prevented `SQL_CONTINUATION_REGEX` from merging them into the parent `ERR [db]` row.
   - Unselected online nodes (`k8smasterdeb`, `k8sworker`) show a misleading `0` badge in `NODES`.
5. **`GAP-08` — Outdated `k8s-agent` Binaries on `.60`, `.133`, `.80`**:
   - Remote agents return `404` on `/logs/errors` because the latest `cmd/agent` binary has not been re-deployed to the 3 Linux hosts yet.
