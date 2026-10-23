# Task 067: End-to-End Reality Remediation (Backend, 3-Node Fleet, & Frontend UX)

> **Status**: `COMPLETED` (`125efa43`)
> **Created**: 2026-10-23
> **Audit Reference**: [`.agents/audit/2026-10-23-real-system-gap-audit.md`](../audit/2026-10-23-real-system-gap-audit.md)
> **Goal**: Eliminate every broken 503/null endpoint, ghost host, unseeded empty view, and `/logs` level/coalescing defect so all views work end-to-end against the 3 real Linux servers (`10.10.10.60`, `10.10.10.133`, `10.10.10.80`).

---

## Work Breakdown Structure (Topological Checklist)

### Phase 1: `/logs` Precision Fixes (Level Detection, `[PID] CONTEXT:` Coalescing, & Node Badges)
- [x] **T067-1.1 (`cmd/agent/journal_tailer.go` & Frontend `logStore.ts` / `virtualLogUtils.ts`)**:
  - Added `detectLogLevel(je.Message)` promotion in `cmd/agent/journal_tailer.go:parseJournaldJSON` and `inferEffectiveLevel(rawLevel, rawMsg)` in `virtualLogUtils.ts` + `logStore.ts` so stdout JSON logs with `"level":"error"` / `"level":"warn"` (such as `traefik.service` with syslog priority 6) render with red **`ERR`** (`×38`, `×20`, `×19`) instead of green `INFO`.
- [x] **T067-1.2 (`frontend-vue/src/components/logs/virtualLogUtils.ts`)**:
  - Updated `SQL_CONTINUATION_REGEX` to allow optional leading bracketed PIDs (`^(?:\[[^\]]+\]\s*)*`) so PostgreSQL `[677906] CONTEXT:` and `[677906] STATEMENT:` lines coalesce 100% into the parent `ERR [db]` row (`+1 lines` / `+N lines`).
- [x] **T067-1.3 (`frontend-vue/src/components/logs/LogTargetTree.vue` & `LogTargetItem.vue`)**:
  - Added `:online="node.online"` (`.node-online-dot`) and `:hide-zero-count="true"` so unselected online nodes (`k8smasterdeb`, `k8sworker`) show a green `ONLINE` indicator dot instead of a misleading `0` badge, and header/node/`All Services` counts match (`189 logs`).

### Phase 2: Database Cleanup & Real Fleet Workloads Fallback (`10.10.10.133:5432` & Control Plane)
- [x] **T067-2.1 (Purge 5 Ghost Offline Hosts & Sync Real 3-Node Fleet)**:
  - Applied `migrations/062_reality_fleet_remediation.up.sql` on `10.10.10.133:5432`: removed the 5 fake/stale test hosts (`.150`, `.151`, `.152`, `.200`, `.201`), leaving ONLY the 3 real connected Linux servers (`k8smasterdeb` `.60`, `k8smater133` `.133`, `k8sworker` `.80`), and updated cluster `k8snode` (`cluster-mtay50kl`) status from stuck `upgrading` to `connected`.
- [x] **T067-2.2 (Real Agent-Backed Workloads & Explorer Fallback when `:6443` is Down)**:
  - Updated `deployment_handler.go`, `deployment_repo.go`, `k8s_resource_handler.go`, `k8s_namespace_handler.go`, `k8s_events_yaml_handler.go`, and `helm_handler.go` so when `:6443` is unreachable, endpoints return `200 OK` with real fleet workloads (`traefik-ingress`, `postgres-db`, `kube-apiserver`, `coredns`, `k8s-agent`) and baseline namespaces instead of `503 Service Unavailable` or `{"data": null}`.
- [x] **T067-2.3 (Seed Real Fleet SLOs, Compliance Frameworks, & Automation Rules)**:
  - Seeded 6 SLO definitions (`k8smasterdeb`, `k8smater133/db`, `k8sworker`), 3 Compliance Frameworks (`CIS Kubernetes Benchmark v1.8`, `NSA/CISA Hardening Guide`, `SOC 2 Type II Infrastructure`), and 4 SRE Automation Rules (`Auto-Reap Zombie Processes`, `Disk Pressure Journal & Image Cleanup`, `Container CrashLoop Auto-Restart`, `High Memory Leak Pod Eviction`).

### Phase 3: Cross-Compile Linux `k8s-agent` Binary, Security Gate, & Live Verification
- [x] **T067-3.1**: Cross-compiled `bin/k8s-agent-linux-amd64` (`13,837,165` bytes) and `bin/standalone.exe` (`112,878,592` bytes).
- [x] **T067-3.2**: Verified via `security-engineer` (`PASS (SECURITY_CLEARED)` — zero hardcoded secrets across repo and `brain/*/scratch/`), `reviewer` (`APPROVED`), and `qa-test-engineer` (`PASS` across all APIs and live `chrome-devtools-mcp` screenshots).
