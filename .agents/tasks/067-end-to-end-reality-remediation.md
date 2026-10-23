# Task 067: End-to-End Reality Remediation (Backend, 3-Node Fleet, & Frontend UX)

> **Status**: `IN_PROGRESS`
> **Created**: 2026-10-23
> **Audit Reference**: [`.agents/audit/2026-10-23-real-system-gap-audit.md`](../audit/2026-10-23-real-system-gap-audit.md)
> **Goal**: Eliminate every broken 503/null endpoint, ghost host, unseeded empty view, and `/logs` level/coalescing defect so all views work end-to-end against the 3 real Linux servers (`10.10.10.60`, `10.10.10.133`, `10.10.10.80`).

---

## Work Breakdown Structure (Topological Checklist)

### Phase 1: `/logs` Precision Fixes (Level Detection, `[PID] CONTEXT:` Coalescing, & Node Badges)
- [ ] **T067-1.1 (`cmd/agent/journal_tailer.go` & Frontend `logStore.ts` / `virtualLogUtils.ts`)**:
  - In `cmd/agent/journal_tailer.go:parseJournaldJSON`, when `je.Priority == "6"` (or default info), inspect `je.Message` via `detectLogLevel(je.Message)` (including JSON `"level":"error"` / `"level":"warn"` / `"level":"fatal"`) so systemd services like `traefik.service` are indexed with `Level: "error"` instead of `"info"`.
  - In `frontend-vue/src/components/logs/virtualLogUtils.ts` (and `logStore.ts`), infer effective log level from structured JSON `"level":"error"` / `"level":"warn"` when `entry.level === 'INFO'`, so existing historical Traefik errors on disk immediately render with red **`ERR`** badges instead of green `INFO` badges!
- [ ] **T067-1.2 (`frontend-vue/src/components/logs/virtualLogUtils.ts`)**:
  - Update `SQL_CONTINUATION_REGEX` to allow optional leading bracketed PID `^(?:\[\d+\]\s+)?(?:CONTEXT:|DETAIL:|HINT:|STATEMENT:|QUERY:|SELECT\b|...)` so PostgreSQL `[677906] CONTEXT:` and `[677906] STATEMENT:` lines coalesce 100% into the parent `ERR [db]` row (`+6 lines`) instead of rendering as 2 extra `INFO [db]` rows.
- [ ] **T067-1.3 (`frontend-vue/src/components/logs/LogTargetTree.vue`)**:
  - For online nodes in `NODES` that are not the currently active node (so their logs aren't in the active buffer), display a green `ONLINE` indicator dot instead of a misleading `0` count badge.

### Phase 2: Database Cleanup & Real Fleet Workloads Fallback (`10.10.10.133:5432` & Control Plane)
- [ ] **T067-2.1 (Purge 5 Ghost Offline Hosts & Sync Real 3-Node Fleet)**:
  - Remove the 5 fake/stale test hosts (`10.10.10.150`, `.151`, `.152`, `.200`, `.201`) and their stale unreachable incidents from PostgreSQL (`10.10.10.133:5432`), leaving only the 3 real production servers (`k8smasterdeb` `10.10.10.60`, `k8smater133` `10.10.10.133`, `k8sworker` `10.10.10.80`).
  - Update cluster `k8snode` (`cluster-mtay50kl`) status from stuck `upgrading` to reflect real health/agent state.
- [ ] **T067-2.2 (Real Agent-Backed Workloads & Explorer Fallback when `:6443` is Down)**:
  - When `10.10.10.60:6443` K8s API is unreachable or local Docker is empty, `GET /api/v1/deployments` and `GET /api/v1/k8s/{cluster}/resources/...` must not return `{"data": null}` or `503 Service Unavailable`; instead, aggregate real running services/containers/processes from the connected `k8s-agent` nodes (`10.10.10.60`, `10.10.10.133`, `10.10.10.80`) and return graceful structured responses (`200 OK`) so `Deployments & Workloads`, `Cluster Explorer`, and `Helm Catalog` never crash or render blank `null` screens.
- [ ] **T067-2.3 (Seed Real Fleet SLOs, Compliance Frameworks, & Automation Rules)**:
  - Ensure PostgreSQL has real, non-empty SLO definitions (for `k8smasterdeb`, `k8smater133/db`, `k8sworker`), Compliance Frameworks (`CIS Kubernetes Benchmark`, `NSA/CISA Hardening`), and SRE Automation Rules (`Auto-Reap Zombies`, `Disk Pressure Cleanup`, `Container CrashLoop Restart`) so `/slo`, `/audit`, and `/automation` work out of the box with real data.

### Phase 3: Cross-Compile Linux `k8s-agent` Binary & Live Verification
- [ ] **T067-3.1**: Cross-compile `GOOS=linux GOARCH=amd64 go build -o bin/k8s-agent-linux-amd64 ./cmd/agent` so the binary includes `/logs/errors` (Blind Error Radar), the `journal_tailer.go` JSON level fix, and the 10GB–100GB `logengine` optimizations (and deploy to `.60`, `.133`, `.80` if SSH keys/access are available, or provide single-command deploy).
- [ ] **T067-3.2**: Verify all 12 screens via `qa-test-engineer` (`chrome-devtools-mcp`) + `reviewer` and update `.agents/memory/project_state.md`.
