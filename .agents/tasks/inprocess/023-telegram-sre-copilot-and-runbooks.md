# Task 023 — Telegram SRE Copilot & Interactive Runbooks (P1/P2)
# Checkpoint: 2026-10-12T10:30:00+07:00

> **Severity**: P1/P2 (Incident Response, 1-Click Operational Remediation & SRE Automation)  
> **Target Agent**: `backend-coder`  
> **Workspace**: Git Worktree Isolated (`Workspace: "branch"`)  
> **Design Rules**: Idempotent Incident Callbacks, Secure Telemetry Inlining, Automated Rollback Execution  

---

## 1. PHÂN TÍCH LỖI & BẰNG CHỨNG MÃ NGUỒN (ROOT CAUSE ANALYSIS)

### Lỗi 1: Nút Bấm Rollback Bị "Bỏ Ngỏ" (Stub Implementation)
* **Vị trí**: `internal/infrastructure/telegram/bot.go:277`
* **Đoạn code bị lỗi**:
  ```go
  // CODE BỊP BỢM TRẢ LỜI NGƯỜI DÙNG:
  case "rollback":
      return b.sendMessage(chatID, "rollback is not yet implemented")
  ```
* **Căn nguyên**: Nửa đêm hệ thống gặp sự cố OOMKilled hoặc CrashLoop, bot Telegram gửi cảnh báo kèm nút bấm "Rollback". Kỹ sư SRE bấm nút với hy vọng cứu vãn tình thế thì bot thản nhiên trả lời "tính năng chưa được làm"! Đây là một thiếu sót chí mạng trong vận hành khẩn cấp.

### Lỗi 2: Thiếu Nút Xem Log Cấp Tốc (Fast Diagnostic Log)
* **Vị trí**: `internal/infrastructure/telegram/bot.go:BuildInlineMarkup`
* **Căn nguyên**: Khi có cảnh báo sự cố, kỹ sư cần nhìn ngay 50 dòng log lỗi cuối cùng của container để biết lỗi là do thiếu RAM, sai config hay lỗi code. Hiện tại tin nhắn chỉ có nút Restart và nút Rollback bị lỗi, bắt kỹ sư phải mở laptop gõ `kubectl logs`.

---

## 2. ĐẶC TẢ STRUCTS & INTERFACES (CONTRACT DEFINITION)

Trong `internal/domain/sre/runbook.go`:
```go
package sre

import (
    "context"
    "time"
)

type RunbookID string

const (
    RunbookRollbackDeployment RunbookID = "RUNBOOK-01-ROLLBACK"
    RunbookFencingNode        RunbookID = "RUNBOOK-02-NODE-FENCING"
    RunbookRotateSecret       RunbookID = "RUNBOOK-03-ROTATE-SECRET"
    RunbookRestoreDrill       RunbookID = "RUNBOOK-04-RESTORE-DRILL"
    RunbookRenewCertificates  RunbookID = "RUNBOOK-05-RENEW-CERTS"
)

type RunbookExecutionRequest struct {
    RunbookID   RunbookID         `json:"runbook_id"`
    TriggeredBy string            `json:"triggered_by"` // Telegram:@user, WebUI:admin
    Parameters  map[string]string `json:"parameters"`
    IncidentID  string            `json:"incident_id,omitempty"`
}

type RunbookExecutionResult struct {
    ExecutionID string        `json:"execution_id"`
    Status      string        `json:"status"` // SUCCESS, FAILED, TIMEOUT
    Output      string        `json:"output"`
    Duration    time.Duration `json:"duration"`
    ExecutedAt  time.Time     `json:"executed_at"`
}

type RunbookExecutor interface {
    Execute(ctx context.Context, req RunbookExecutionRequest) (*RunbookExecutionResult, error)
}
```

---

## 3. THUẬT TOÁN XỬ LÝ TỪNG BƯỚC (STEP-BY-STEP WORKFLOW)

### Bước 1: Thực Thi 1-Click Rollback Chuẩn Kubernetes
1. Khi nhận callback query `action:rollback:cluster:ns:deployment`:
2. Tìm `Deployment` trong Kubernetes API.
3. Lấy `DeploymentRollback` hoặc tìm `ReplicaSet` có revision liền trước ($N-1$).
4. Patch Deployment pod template spec với revision cũ (tương đương `kubectl rollout undo`).
5. Watch trạng thái rollout trong tối đa 30 giây:
   - Nếu pod mới đạt `Ready == True`: Gửi tin nhắn cập nhật về Telegram:
     `✅ Rollback thành công về Revision N-1. Dịch vụ đã hoạt động ổn định.`
   - Nếu rollback timeout: Báo lỗi và gợi ý tạo Remediation PR.

### Bước 2: Bổ Sung Nút & Bộ Xử Lý "Xem 50 Dòng Log"
1. Trong hàm tạo bàn phím inline `BuildInlineMarkup`:
   ```go
   inlineKeyboard := [][]InlineKeyboardButton{
       {
           {Text: "🔄 Restart Pod", CallbackData: fmt.Sprintf("restart:%s:%s", ns, pod)},
           {Text: "⏪ 1-Click Rollback", CallbackData: fmt.Sprintf("rollback:%s:%s", ns, deploy)},
       },
       {
           {Text: "📄 Xem 50 Dòng Log", CallbackData: fmt.Sprintf("logs:%s:%s", ns, pod)},
       },
   }
   ```
2. Khi nhận callback `logs:ns:pod`:
   - Gọi trực tiếp `LogEngine` hoặc Kubernetes Pod Logs API với `TailLines = 50`.
   - Lọc bỏ các ký tự điều khiển ANSI.
   - Định dạng code block Markdown gửi lại vào chat Telegram (giới hạn tối đa 4000 ký tự).

---

## 4. CHUẨN HÓA MÃ LỖI (RFC 7807 ERROR SPECIFICATION)

* `ERR_RUNBOOK_EXECUTION_TIMEOUT`: `HTTP 504 Gateway Timeout` — "Runbook '%s' execution timed out after %s."
* `ERR_ROLLBACK_NO_PREVIOUS_REVISION`: `HTTP 422 Unprocessable Entity` — "Cannot rollback deployment '%s': No previous revision found in ReplicaSet history."

---

## 5. MA TRẬN TEST CASE (VERIFICATION SUITE)

1. `TestTelegramBot_RollbackCallback_ExecutesRollback`: Mock K8s client có Deployment revision 1 và 2. Giả lập bấm nút callback rollback. Assert K8s client nhận được lệnh patch revision 1 và bot gửi tin nhắn xác nhận.
2. `TestTelegramBot_ViewLogsCallback_ReturnsFormattedLogs`: Mock log engine trả về 50 dòng log. Assert tin nhắn trả về định dạng code block `<pre>...</pre>` và không vượt quá 4096 ký tự của Telegram API.
3. `TestRunbookExecutor_Dispatch5Runbooks`: Kiểm tra bộ dispatcher hỗ trợ đầy đủ 5 mã runbooks (`RUNBOOK-01` đến `RUNBOOK-05`) mà không báo unhandled action.

---

## 6. KẾ HOẠCH ROLLBACK & GUARDRAILS AN TOÀN

* **Guardrail**: Nút bấm trên Telegram bắt buộc phải kiểm tra quyền của người bấm (`TelegramUserID` có nằm trong whitelist `TELEGRAM_ALLOWED_ADMIN_IDS` hay không). Người lạ trong nhóm chat không được phép bấm nút xóa pod hoặc rollback.
