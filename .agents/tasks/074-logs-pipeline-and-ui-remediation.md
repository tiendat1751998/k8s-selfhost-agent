# Task 074 — Logs Pipeline, Rate-Limit Bypass, `klog` Severity, Error Radar & UI Overflow Remediation

> **Status**: `COMPLETED` (`71054489`)
> **Branch**: `feat/declone-sre-ux`
> **Origin**: Systematic Phase-1 Audit (`docs/qa-evidence/2026-11-05/logs-investigation/01..08`)
> **Verified Evidence**: `docs/qa-evidence/2026-11-05/task-074-verified/01..08` (`5/5` Gates `LES-012` PASS)

## 1. Verified Root Causes
1. **RC-1 (`github_rate_limit.go` & `agent_auth.go` Regression from `1ccf0647`)**:
   - `DefaultGitHubRateLimit` exempted `/api/v1/logs` from the secondary 80 req/min mutation cap, but **did NOT exempt `POST /api/v1/logs/ingest` or streaming WebSocket `/api/v1/logs/stream` from the 5,000 req/hr primary core quota** (nor exempt `/api/v1/logs/ingest` from `MaxConcurrent: 100`), causing `HTTP 429 Too Many Requests` under continuous log ingestion/streaming, and wrote `X-RateLimit-*` headers onto WebSocket responses before `Hijack()`.
   - `agent_auth.go` (`matchAgentAuthToken`) only accepted `AGENT_AUTH_TOKEN` on `/agents/register`, rejecting `X-Agent-Token` on `POST /api/v1/logs/ingest` with `401 Unauthorized`.
2. **RC-2 (`klog` Severity Misclassification, `Blind Error Radar` Zero-Pattern Contradiction & Offline Node Timeout)**:
   - Kubernetes daemons (`kubelet.service`, `kube-apiserver`, `kube-scheduler`, etc.) write Google `klog` lines (`E1105 ...`, `W1105 ...`, `F1105 ...`) to stdout/stderr where `journald` assigns default priority `6` (`info`). Every `kubelet` error (`E1105 ... "Unable to register node with API server"`) was misclassified as `INFO`, and `SummarizeErrors` (`Blind Error Radar`) exited early on `GET /logs/errors` (`groups: []`) because edge MergeTree stored those rows as `level="info"`.
   - Offline node (`10.10.10.60` `k8smasterdeb`, physically powered off) caused slow dial timeouts on distributed log and `/deployments` K8s `:6443` queries unless bounded by fast-fail connect timeout (`800ms`) + `15s` `HostBackoffManager` cooldown, while `All Nodes` search could be monopolized by a single noisy service without fair per-node merging.
3. **RC-3 (Stale `frontend-vue/dist` Served by `:8080`, `History` Toolbar Overflow, Row-0 Clipping & Scoped CTA Leak)**:
   - `frontend-vue/dist` on disk predated `6b1c3e85` (`73E`), making `:8080` (`standalone.exe`) serve an outdated bundle.
   - Switching `/logs` to `History` mode overflowed `.log-stream-toolbar` horizontally (`scrollWidth > clientWidth`), clipping the page left edge under the sidebar.
   - `VirtualLogTerminal` clipped the top half of row `0` at `scrollTop = 0`, the leftmost target-drawer toggle button rendered a broken glyph, and extracting `LogTerminalEmptyState.vue` (`< 300` lines) left `.btn-switch-history` trapped in parent `<style scoped>`.

## 2. Remediated Slices (`reviewer` APPROVED & `qa-test-engineer` 5-Gate E2E PASS)

### Slice 74A (`a688949d`) — Rate-Limit Spoof-Safe Exemption, Agent Auth, `klog` Normalization & Fast-Fail Backoff
- `internal/adapter/http/middleware/streaming.go`, `github_rate_limit.go`, `github_rate_limit_spoof_test.go`: Added strict route-aware `isVerifiedStreamingEndpoint(r)` (`GET` + verified streaming endpoints), exempted `POST /api/v1/logs/ingest` & `/api/v1/metrics/ingest` by exact path match, suppressed `X-RateLimit-*` before `Hijack()`, and blocked `Upgrade: websocket` / `Accept: text/event-stream` spoofing on normal REST endpoints (`429` verified).
- `internal/adapter/http/middleware/agent_auth.go` (+ `agent_auth_test.go`): Exact path equality allowing `AGENT_AUTH_TOKEN` on `/api/v1/logs/ingest` and `/api/v1/agents/register`.
- `internal/domain/logging/entity.go`, `internal/infrastructure/agent/host_backoff.go`, `log_client.go`, `distributed_log_repo.go`, `internal/infrastructure/kubernetes/deployment_repo.go`: Normalized `klog` prefixes (`E/F` -> `error`, `W` -> `warn`, `I` -> `info`), added `800ms` TCP dialer + `15s` `HostBackoffManager` cooldown + `1200ms` K8s `:6443` list timeout + `fairMergeLogEntries`.

### Slice 74B (`9a11f598`) — Zero-Overflow `History` Toolbar, Unclipped Row 0, `<300`-Line Split & Dual-Theme Parity
- `LogStreamToolbar.vue`, `VirtualLogTerminal.vue` (`290` lines), `LogTerminalEmptyState.vue` (`98` lines), `LogTargetTree.vue` (`220` lines), `LogTargetServicesSection.vue` (`258` lines), `useVirtualLogScroll.ts` (`140` lines), `LogStreamView.vue` (`326` lines): Eliminated horizontal overflow across `1440px`/`768px`/`375px`, fixed row-0 clipping at `scrollTop = 0`, replaced broken toggle glyph with `<BaseIcon name="sidebar" />`, and purged 100% of `rgba(15, 23, 42)` & raw neon hexes.

### Slice 74C (`46f9bc35` + `71054489`) — `Blind Error Radar` `klog` Fingerprinting & Offline CTA Token Grounding
- `internal/infrastructure/agent/distributed_error_radar.go`, `distributed_log_repo.go`, `internal/pkg/logengine/error_radar.go`, `writer.go`, `cmd/agent/log_search.go`, `pod_tailer.go`: Parallel `level=error` + unfiltered recent sample (`Limit: 2000`) normalized via `NormalizeLogLevel`, deduplicated by `timestamp|node|service|message`, and fingerprinted with pre-compiled `klogHeaderRegex` (`9 unique error patterns / 1,572 occurrences` discovered in `73ms` on live `k8sworker`).
- `LogTerminalEmptyState.vue`, `tokens.css`, `virtual-log-terminal.css`, `log-error-radar.css`, `task074LogsCtaAndTokens.test.ts`: Grounded `.btn-switch-history` (`#2563eb`, `gap: 8px`) inside child scoped styles, added responsive Error Radar wrapping, rebuilt `frontend-vue/dist` + `bin/standalone.exe`, and verified `450/450` genuine behavioral frontend tests (`0` `fs.readFileSync` regex assertions).
