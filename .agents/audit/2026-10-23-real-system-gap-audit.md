# Brutally Honest Full-System Audit Report (2026-10-23)

> **Date**: 2026-10-23
> **Branch**: `fix/comprehensive-audit`
> **Scope**: Live Backend (`http://localhost:8080`), PostgreSQL (`10.10.10.133:5432`), Live Linux Agents (`10.10.10.60:9100`, `10.10.10.133:9100`, `10.10.10.80:9100`), and Vue 3 Frontend (`http://localhost:3000`).

---

## 1. Executive Summary of Real Problems (Zero Sugarcoating)

Previous `project_state.md` checkpoints claimed "100% Complete / Zero Tech Debt", which masked severe end-to-end operational gaps when the user actually clicked through the application. A live probe of all backend APIs, database tables, and the 3 Linux servers revealed **5 root-cause categories** explaining why the backend, frontend, and features feel broken/incomplete:

### Category A — Broken K8s Control Plane Link & Empty Workloads (`503` / `null` Screens)
1. **Cluster `k8snode` (`cluster-mtay50kl`) Stuck & Port `6443` Unreachable**:
   - In PostgreSQL `clusters` table, `k8snode` has `status = "upgrading"` and points to `https://10.10.10.60:6443`.
   - Live TCP check to `10.10.10.60:6443` fails (`TcpTestSucceeded: False`), while the `k8s-agent` on `10.10.10.60:9100` IS online and healthy (`kubelet_healthy: true`, `containerd_healthy: true`).
   - **Cascade Impact**:
     - **Cluster Explorer** (`GET /api/v1/k8s/k8snode/namespaces`, `/resources/pods`) fails with `503 Service Unavailable (K8S_UNAVAILABLE)`.
     - **Helm Catalog Releases** (`GET /api/v1/helm/k8snode/releases`) fails with `503 Service Unavailable`.
     - **Deployments & Workloads** (`GET /api/v1/deployments`) returns `{"data": null}` (blank screen) because it only queries the unreachable K8s API / local Windows Docker instead of falling back to live workloads/containers discovered from the 3 connected Linux agents (`.60`, `.133`, `.80`).

### Category B — Outdated `k8s-agent` Binaries on All 3 Linux Servers (`.60`, `.133`, `.80`)
1. **Live Agent Endpoint Probe**:
   - `GET http://10.10.10.60:9100/logs/errors` -> `404 Not Found`
   - `GET http://10.10.10.133:9100/logs/errors` -> `404 Not Found`
   - `GET http://10.10.10.80:9100/logs/errors` -> `404 Not Found`
   - The 3 Linux machines are still running an older `k8s-agent` binary compiled before the `logengine` high-scale optimizations and `/logs/errors` (Blind Error Radar) endpoint were added.
2. **`journal_tailer.go` Level Misclassification (`INFO` instead of `ERROR`)**:
   - In `cmd/agent/journal_tailer.go:197-224`, systemd services writing structured JSON to stdout (like `traefik.service` logging `{"level":"error","error":"...","msg":"Command error"}`) get syslog `PRIORITY=6` (`info`) from journald.
   - `parseJournaldJSON` blindly uses `priorityToLevel(je.Priority)` (`"info"`) without checking `detectLogLevel(je.Message)` or JSON `"level":"error"`, causing all `traefik.service` errors in ClickHouse/logengine storage to be stored and rendered as green `INFO` rows!

### Category C — Stale / Ghost Data & Empty Database Tables in PostgreSQL (`10.10.10.133:5432`)
1. **5 Ghost Offline Hosts Polluting the Fleet**:
   - The `hosts` table contains 8 rows: 3 real servers (`10.10.10.60`, `10.10.10.133`, `10.10.10.80`) and **5 fake/stale test IPs** (`10.10.10.150`, `.151`, `.152`, `.200`, `.201`) that are permanently `disconnected`.
   - These 5 ghost hosts pollute `Infrastructure Hosts`, `Overview`, `Incidents & RCA`, and `Real-Time Logs (+ 5 offline)`.
2. **Empty Feature Tables (`SLOs`, `Compliance / Audit`, `Automation Rules`)**:
   - `GET /api/v1/observability/slo/definitions` -> `{"data": []}`
   - `GET /api/v1/compliance/frameworks` & `/violations` -> `{"data": []}`
   - `GET /api/v1/automation/rules` -> `{"data": []}`
   - Clicking into **SLOs & Budgets**, **Audit & CVEs**, or **Automation Rules** presents empty screens because real baseline definitions/rules for the 3-node fleet were never seeded or synchronized with live agent telemetry.

### Category D — `/logs` UI & Log Parsing Residual Defects
1. **JSON `"level":"error"` Override in Frontend/Backend Log Mapper**:
   - Even before re-deploying `k8s-agent`, historical logs from `traefik.service` already stored in `logengine` have `level: "INFO"` in metadata while their JSON body explicitly states `"level":"error"`. The frontend/backend log normalizer does not promote `level` to `ERROR` when the JSON payload has `"level":"error"` or `"level":"fatal"`.
2. **Postgres `[PID] CONTEXT:` and `[PID] STATEMENT:` Coalescing Bug**:
   - In `frontend-vue/src/components/logs/virtualLogUtils.ts:133`, `SQL_CONTINUATION_REGEX` anchors at `^(?:CONTEXT:|DETAIL:|STATEMENT:...)` without allowing optional leading `(?:\[\d+\]\s+)?`.
   - Because `cleanLogMessage` preserves the Postgres PID `[677906]` on `[677906] CONTEXT:` and `[677906] STATEMENT:`, those 2 lines fail to match `SQL_CONTINUATION_REGEX` and still render as 2 separate `INFO [db]` rows underneath the `ERR [db]` row instead of collapsing into a single `ERR [db] (+7 lines)` row!
3. **Node Log Count Badges on Unselected Online Nodes**:
   - In `LogTargetTree.vue`, when `k8smater133` is selected, `k8smasterdeb` and `k8sworker` display `0` in their badge because `logStore.logs` only holds the currently selected node's logs. Displaying `0` next to an online node misleads the user into thinking those nodes have 0 logs on disk.

---

## 2. View-by-View Status Matrix (Baseline Before Task 067)

| Screen / Route | Current State | Key Defect IDs |
| :--- | :--- | :--- |
| `/overview` | Partial (shows 3 connected + 5 dead hosts, 0 workloads) | `GAP-01`, `GAP-03` |
| `/incidents` | Shows stale unreachable-host incidents from 5 ghost hosts | `GAP-03` |
| `/slo` | Empty (`data: []`) | `GAP-04` |
| `/logs` | Traefik errors shown as green `INFO`; `[PID] CONTEXT:` not merged; `.60`/`.80` show `0` badge | `GAP-05`, `GAP-06`, `GAP-07` |
| `/clusters` | `k8snode` stuck in `upgrading`, port `6443` unreachable | `GAP-01` |
| `/hosts` | Polluted by 5 ghost offline hosts (`.150`–`.201`) | `GAP-03` |
| `/deployments` | Returns `{"data": null}` (blank screen) | `GAP-02` |
| `/explorer` | Returns `503 K8S_UNAVAILABLE` | `GAP-01`, `GAP-02` |
| `/helm` | Repos work, Releases return `503 K8S_UNAVAILABLE` | `GAP-01` |
| `/audit` | Frameworks & violations empty (`data: []`) | `GAP-04` |
| `/automation` | Rules empty (`data: []`) | `GAP-04` |
| `/runbooks` | Working | — |
