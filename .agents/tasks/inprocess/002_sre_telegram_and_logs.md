# TASK 002: SRE Telegram Copilot & Enterprise Log Engine (ELK/Vector/Loki)

> **Module**: 02 & 03 - Telegram SRE Bot & Enterprise Log Aggregator  
> **Status**: READY_FOR_DISPATCH  
> **Assigned Subagents**:  
>   - `devops`: Vector/Fluent Bit DaemonSets & Logging Pipeline Configs  
>   - `backend-coder`: Telegram SRE Bot with Inline Actions & WebSocket Log Tail  
> **Model**: `flash` (Gemini 3.7 Flash High)  
> **Workspace Mode**: `Workspace: "branch"` (Git worktree isolation)  

---

## 1. Goal
1. Xây dựng SRE Telegram Bot Copilot với khả năng lọc nhiễu sự cố chùm, tích hợp AI RCA tóm tắt nguyên nhân lỗi, và các nút bấm Inline Actions tương tác 2 chiều (`/restart <app>`, `/rollback <app>`, `/restore_db <job_id>`).
2. Xây dựng phân hệ Logging Pipeline (Vector/Fluent Bit DaemonSet -> OpenSearch/Loki -> WebSocket Live Tail Server) hỗ trợ phân tích log thời gian thực <50ms.

---

## 2. File Scope & Subagent Responsibilities
- **Subagent A (`devops`)**:
  - `deploy/logging/vector-daemonset.yaml`
  - `deploy/logging/fluentbit-config.yaml`
  - `deploy/logging/loki-stack.yaml`
- **Subagent B (`backend-coder`)**:
  - `internal/infrastructure/telegram/bot.go`
  - `internal/infrastructure/telegram/handler.go`
  - `internal/infrastructure/logging/aggregator.go`
  - `internal/adapter/http/log_stream_handler.go`
  - `internal/usecase/incident/copilot.go`

---

## 3. Acceptance Criteria
- [ ] Bot Telegram nhận webhook/polling, gửi cảnh báo có format Markdown kèm nút bấm Inline (`🔄 Khởi động lại`, `⏮️ Rollback`, `📦 Khôi phục DB`).
- [ ] Xử lý callback query an toàn: xác thực quyền Admin trước khi thực thi action.
- [ ] WebSocket Live Tail stream log liên tục theo pod/container với latency <50ms.
- [ ] Toàn bộ unit test và build phải PASS (`go test ./internal/infrastructure/telegram/...`, `go test ./internal/infrastructure/logging/...`).
