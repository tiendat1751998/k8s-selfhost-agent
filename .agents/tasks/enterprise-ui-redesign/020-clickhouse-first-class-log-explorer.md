# TASK 020: ClickHouse First-Class Log Explorer Integration

> **Location**: .agents/tasks/enterprise-ui-redesign/020-clickhouse-first-class-log-explorer.md  
> **Benchmark Source**: Datadog Log Explorer, SigNoz, Grafana Loki, Rancher Logging  
> **Status**: READY_FOR_DISPATCH  

## 1. Problem Statement
The current `/logs` screen functions primarily as a basic terminal stream. While the Go backend already possesses a robust, native ClickHouse pipeline (`internal/infrastructure/clickhouse/` with `BatchWriter`, `/api/v1/logs/query`, and `/api/v1/logs/histogram`), the standalone runner (`cmd/standalone/standalone_wiring.go`) leaves `platformHandlers.CentralizedLogs = nil`, returning 404 on log search and histogram endpoints.
Furthermore, the frontend lacks:
1. Visual log volume histogram over time (bars grouped by log level `ERROR`, `WARN`, `INFO`, `DEBUG`).
2. Faceted query chips (filter by `service`, `namespace`, `pod`, `severity`, `cluster`).
3. Explicit ClickHouse Engine Status badge (cluster connectivity, ingestion rate, retention window).
4. Clean vector icons (currently contaminated with 16 emojis in `LogTargetTree.vue`).

## 2. Industry Standard Pattern (Datadog & SigNoz)
- **Top Bar**: Search input with Lucide search icon, faceted filter chips, time range selector (`Last 15m`, `1h`, `24h`, `Custom`), and Engine Status Badge (`ClickHouse MergeTree · Connected · 1.4ms`).
- **Telemetry Histogram**: 64px–80px interactive stacked SVG/Canvas histogram above logs showing message volume per bucket with level-based color breakdown (Rose for ERROR, Amber for WARN, Cyan for INFO, Slate for DEBUG).
- **Log Stream Grid**: Virtualized log line list with tabular monospace timestamps (`YYYY-MM-DD HH:mm:ss.SSS`), level pill badges, and structured JSON context toggle.
- **Resilient Fallback**: Standalone and local environments without a running ClickHouse instance automatically route requests through an in-memory ring buffer with zero 404s.

## 3. Action Items

### Backend (`backend-coder`):
1. **Wire ClickHouse with Resilient Fallback in `cmd/standalone/standalone_wiring.go` & `cmd/server/bootstrap_services.go`**:
   - Add ClickHouse configuration in `internal/infrastructure/config/config.go`.
   - Attempt ClickHouse client connection with 3-second timeout.
   - If ClickHouse is reachable: wire `BatchWriter`, `LogRepository`, and mount `platformHandlers.CentralizedLogs`.
   - If ClickHouse is offline: wrap the in-memory `logAggregator` with a fallback `LoggingService` so `/api/v1/logs/search`, `/histogram`, and `/status` succeed reliably without 404.
2. **Add `/api/v1/logs/status` Endpoint**:
   - Expose engine health, active engine (`ClickHouse MergeTree` vs `In-Memory RingBuffer`), connection latency, row count, and retention TTL.
3. **Faceted Query Parser**:
   - Parse `level:`, `container:`, `pod:`, `node:` tokens from query string.

### Frontend (`frontend-coder`):
1. **ClickHouse Engine Status Badge (`frontend-vue/src/components/logs/ClickHouseEngineBadge.vue`)**:
   - Display real-time engine health badge with popover metadata.
2. **Interactive Stacked Log Volume Histogram (`frontend-vue/src/components/logs/LogVolumeHistogram.vue`)**:
   - 80px stacked bar chart displaying log volume grouped by severity level.
   - Click-to-filter time range.
3. **Purge Emojis in `LogTargetTree.vue`**:
   - Replace all 16 emojis (`🌲`, `🔍`, `🌐`, `👑`, `🖥️`, `🗄️`, `🚦`, `🐘`, `⚡`, `🛰️`, `🐳`, `🔒`, `📊`, `⚙️`) with clean vector SVGs (`BaseIcon.vue`).
4. **Faceted Query Chips (`LogFilterToolbar.vue`)**:
   - Support structured search tags.

## 4. Verification:
- `go test -v ./internal/infrastructure/clickhouse/...`
- `go test -v ./internal/adapter/http/... -run TestLogHandler`
- `npm.cmd run build` inside `frontend-vue` passes with 0 errors.
- Chrome DevTools MCP audit verifying live histogram and engine badge rendering.
