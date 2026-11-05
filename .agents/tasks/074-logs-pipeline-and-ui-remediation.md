# Task 074 — Logs Pipeline, Rate-Limit Bypass, `klog` Severity & UI Overflow Remediation

> **Status**: `IN_PROGRESS`
> **Branch**: `feat/declone-sre-ux`
> **Origin**: Systematic Phase-1 Audit (`docs/qa-evidence/2026-11-05/logs-investigation/01..08`)

## 1. Verified Root Causes
1. **RC-1 (`github_rate_limit.go` & `agent_auth.go` Regression from `1ccf0647`)**:
   - `DefaultGitHubRateLimit` exempts `/api/v1/logs` from the secondary 80 req/min mutation cap, but **does NOT exempt `POST /api/v1/logs/ingest` or streaming WebSocket `/api/v1/logs/stream` from the 5,000 req/hr primary core quota** (nor exempt `/api/v1/logs/ingest` from `MaxConcurrent: 100`), causing `HTTP 429 Too Many Requests` under continuous log ingestion/streaming, and writes `X-RateLimit-*` headers onto WebSocket responses before `Hijack()`.
   - `agent_auth.go` (`matchAgentAuthToken`) only accepts `AGENT_AUTH_TOKEN` on `/agents/register`, rejecting `X-Agent-Token` on `POST /api/v1/logs/ingest` with `401 Unauthorized`.
2. **RC-2 (`klog` Severity Misclassification & Multi-Node Fan-Out Timeout)**:
   - Kubernetes daemons (`kubelet.service`, `kube-apiserver`, `kube-scheduler`, etc.) write Google `klog` lines (`E1105 ...`, `W1105 ...`, `F1105 ...`) to stdout/stderr where `journald` assigns default priority `6` (`info`). Every `kubelet` error (`E1105 ... "Unable to register node with API server"`) is misclassified as `INFO`, breaking `ERR` filter pills, `Volume` error bars, and `Blind Error Radar`.
   - Offline node (`10.10.10.60` `k8smasterdeb`) causes slow dial timeouts on distributed log / workload log queries unless bounded by fast-fail circuit/connect timeout (`<= 1.5s`), and `All Nodes` initial search can be monopolized by a single noisy service (`traefik.service`) instead of balanced multi-node recent logs.
3. **RC-3 (Stale `frontend-vue/dist` Served by `:8080` & `/logs` Layout Clipping)**:
   - `frontend-vue/dist` on disk predates `6b1c3e85` (`73E`), so `:8080` (`standalone.exe`) serves an outdated UI missing `<ClickHouseEngineBadge />`, sidebar-docked `PwaInstallBanner`, and `1024px` `/alerts` cards.
   - Switching `/logs` to `History` mode overflows `.log-stream-toolbar` horizontally (`scrollWidth > clientWidth`), shifting the entire page sideways underneath the left sidebar (`08-logs-history-mode.png`).
   - `VirtualLogTerminal` clips the top half of row `0` at `scrollTop = 0` (`04`, `05`, `06`), and the leftmost target-drawer toggle button renders a tiny clipped glyph instead of a clean `BaseIcon`.

## 2. Disjoint Parallel Slices (`<= 2` Builders, `Workspace: "branch"`, `Model: "pro"`)

### Slice 74A (`backend-coder`) — Rate-Limit & Agent-Auth Exemption, `klog` Severity Normalization & Fast-Fail Offline Node Fan-Out
- `internal/adapter/http/middleware/github_rate_limit.go` (+ `_test.go`): Exempt `IsStreamingRequest(r)` (no primary decrement, no pre-hijack `X-RateLimit-*` header writes) and `POST /api/v1/logs/ingest` (exempt from primary 5,000/hr user quota & concurrency cap, or dedicated high-throughput agent ingest exemption).
- `internal/adapter/http/middleware/agent_auth.go` (+ `_test.go`): Allow `AGENT_AUTH_TOKEN` (`X-Agent-Token`) on `/api/v1/logs/ingest` in `matchAgentAuthToken`.
- `internal/infrastructure/logengine/` & `internal/infrastructure/observability/` (`distributed_log_repo.go` / parser + `_test.go`): Normalize `klog` prefixes (`E\d{4}\s` -> `error`, `F\d{4}\s` -> `fatal`/`error`, `W\d{4}\s` -> `warn`, `I\d{4}\s` -> `info`) and structured `level=error`/`"level":"error"` when raw level is empty or default `info` so `Search`, `Stream`, `Histogram`, and `Errors` (`Error Radar`) accurately report `ERR`/`WARN`; ensure fast TCP connect timeout (`<= 1.2s`) on unreachable edge nodes (`10.10.10.60:9100`) and balanced per-node merge on `All Nodes` queries.

### Slice 74B (`frontend-coder`) — `History` Toolbar Zero-Overflow, `VirtualLogTerminal` Top-Row Padding, Toggle Icon & Production `dist` Sync
- `frontend-vue/src/components/logs/LogStreamToolbar.vue`, `VirtualLogTerminal.vue`, `ErrorRadarPanel.vue`, `RealTimeLogsView.vue`, and `frontend-vue/src/assets/styles/views/logs*.css`:
  - Fix `History` mode horizontal overflow (`flex-wrap: wrap`, `min-width: 0`, `max-width: 100%`, `overflow-x: hidden`) so `scrollWidth <= clientWidth` across `1440px`, `768px`, and `375px` in both `Live` and `History` modes.
  - Fix top-row clipping at `scrollTop = 0` in `VirtualLogTerminal.vue` (including when `Error Radar` and `Log Volume` panels are open).
  - Replace the tiny/clipped target-panel collapse character on the far left of `LogStreamToolbar.vue` with a crisp `BaseIcon` (`sidebar` / `chevron-left`) with proper `aria-label` and contrast in Dark & Light modes.
  - Normalize `klog` client-side fallback in `logStore.ts` if raw streamed frame arrives with `level: 'info'` + `^[EF]\d{4}\s`, and display clear offline-node status + 1-click `"Switch to History"` button when viewing an offline node in `Live` mode.
  - Behavioral tests (`tests/task074LogsFix.test.ts` — zero `fs.readFileSync`) + `npm run build` verification.
