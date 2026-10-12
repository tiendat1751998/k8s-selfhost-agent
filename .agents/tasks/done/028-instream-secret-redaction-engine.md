# Task 028 — In-Stream Secret Redaction & Log Masking Engine on `k8s-agent` (P1)
# Checkpoint: 2026-10-12T10:35:00+07:00

> **Severity**: P1 (Credential Leakage Prevention, DevSecOps Compliance & High-Throughput Log Sanitization)  
> **Target Agent**: `backend-coder`  
> **Workspace**: Git Worktree Isolated (`Workspace: "branch"`)  
> **Design Rules**: Zero-Allocation In-Place Replacement, Regex Automata Stream Masking, Zero Memory Leak  

---

## 1. BỐI CẢNH KỸ THUẬT & PHÂN TÍCH LỖI THỰC TẾ (ROOT CAUSE ANALYSIS)

### Lỗi Rò Rỉ Bí Mật (Credentials Leak) Tràn Lan Trong Nhật Ký Hệ Thống (Logs)
* **Hiện tượng thực tế**:
  1. Khi ứng dụng gặp panic hoặc lỗi kết nối, stack trace thường in ra toàn bộ biến môi trường hoặc chuỗi kết nối:
     `FATAL: connection to "postgres://admin:SuperSecretP@ss123@db.internal:5432/production" failed`.
  2. Các framework log request HTTP thường in toàn bộ header `Authorization: Bearer eyJhbGciOi...` hoặc query param `?apiKey=sk_live_938471928374`.
* **Hậu quả thảm khốc**:
  - `k8s-agent` tail log từ file container trên đĩa (`/var/lib/docker/containers/*`), nén ZSTD và ship về Central Server.
  - Toàn bộ secret, token ngân hàng và mật khẩu quản trị bị lưu trữ vĩnh viễn trong kho lưu trữ log. Bất kỳ lập trình viên hoặc bên thứ ba nào có quyền xem log đều có thể trích xuất toàn bộ credentials của production!

---

## 2. ĐẶC TẢ STRUCTS & INTERFACES (CONTRACT DEFINITION)

Trong `internal/pkg/logengine/redact.go`:
```go
package logengine

import "regexp"

type SecretPatternType string

const (
    PatternConnectionString SecretPatternType = "CONNECTION_STRING"
    PatternBearerToken      SecretPatternType = "BEARER_TOKEN"
    PatternAPIKey           SecretPatternType = "API_KEY"
    PatternPasswordKeyValue SecretPatternType = "PASSWORD_KEY_VALUE"
    PatternPrivateKey       SecretPatternType = "PRIVATE_KEY"
)

type RedactionRule struct {
    Type        SecretPatternType
    Regex       *regexp.Regexp
    Replacement []byte
}

type InStreamRedactor struct {
    rules []RedactionRule
}

func NewInStreamRedactor() *InStreamRedactor {
    return &InStreamRedactor{
        rules: []RedactionRule{
            {
                Type:        PatternConnectionString,
                Regex:       regexp.MustCompile(`(?i)(postgres|mysql|mongodb|redis|amqp):\/\/[^:\s]+:[^@\s]+@`),
                Replacement: []byte(`$1://[REDACTED_USER]:[REDACTED_SECRET]@`),
            },
            {
                Type:        PatternBearerToken,
                Regex:       regexp.MustCompile(`(?i)(bearer\s+)[a-zA-Z0-9_\-\.]{20,}`),
                Replacement: []byte(`$1[REDACTED_BEARER_TOKEN]`),
            },
            {
                Type:        PatternAPIKey,
                Regex:       regexp.MustCompile(`(?i)(sk_live_|ghp_|glpat-|xoxb-)[a-zA-Z0-9_\-]{16,}`),
                Replacement: []byte(`$1[REDACTED_API_KEY]`),
            },
            {
                Type:        PatternPasswordKeyValue,
                Regex:       regexp.MustCompile(`(?i)("(?:password|secret|token|api_key|access_key)"\s*:\s*")[^"]+(")`),
                Replacement: []byte(`$1[REDACTED_SECRET]$2`),
            },
            {
                Type:        PatternPrivateKey,
                Regex:       regexp.MustCompile(`(?s)-----BEGIN [A-Z ]+PRIVATE KEY-----.*?-----END [A-Z ]+PRIVATE KEY-----`),
                Replacement: []byte(`[REDACTED_PRIVATE_KEY]`),
            },
        },
    }
}
```

---

## 3. THUẬT TOÁN XỬ LÝ TỪNG BƯỚC (STEP-BY-STEP WORKFLOW)

### Quy trình Lọc Secret Tại Nguồn (Edge Stream Redaction):
1. **Tiếp nhận Chunk Log**: `container_tailer.go` đọc một dòng log thô từ file log của container (`[]byte`).
2. **Kiểm Tra Nhanh (Fast Path Heuristic)**:
   - Trước khi chạy regex, kiểm tra xem dòng log có chứa các từ khóa nhạy cảm hay không (`bytes.ContainsAny` hoặc substring check: `password`, `bearer`, `://`, `token`, `sk_live`).
   - Nếu không chứa bất kỳ từ khóa nào $\implies$ Bỏ qua bước regex (Zero CPU overhead cho 95% dòng log bình thường).
3. **Thực Thi Redaction (Slow Path)**:
   - Nếu có từ khóa, chạy qua các rules đã pre-compile.
   - Thay thế các chuỗi bí mật bằng mảng byte `[REDACTED_SECRET]`.
4. **Nén & Đóng Block ZSTD**:
   - Dòng log đã được làm sạch 100% được đẩy vào `Columnar LogEngine` để nén ZSTD và ghi đĩa/truyền mạng.
   - **Bảo đảm tuyệt đối**: Không có bất kỳ credential dạng plaintext nào lọt vào file ZSTD hay stream SSE.

---

## 4. CHUẨN HÓA MÃ LỖI (RFC 7807 ERROR SPECIFICATION)

* `ERR_LOG_REDACTION_FAILED`: `HTTP 500 Internal Server Error` — "Log stream processing halted: Redaction engine failed to process log chunk safely."

---

## 5. MA TRẬN TEST CASE (VERIFICATION SUITE)

1. `TestRedactor_PostgresConnectionString_Masked`: Feed chuỗi `postgres://root:123456@db:5432/app`. Assert kết quả trả về `postgres://[REDACTED_USER]:[REDACTED_SECRET]@db:5432/app`.
2. `TestRedactor_BearerToken_Masked`: Feed `Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI...`. Assert token bị che giấu hoàn toàn.
3. `TestRedactor_JSONPayload_PasswordMasked`: Feed `{"username":"admin","password":"MySecretPassword123!"}`. Assert password bị biến thành `[REDACTED_SECRET]`.
4. `TestRedactor_Benchmark_Throughput`: Chạy benchmark với 100,000 dòng log. Đo lường throughput: Phải đạt tối thiểu $> 500,000$ lines/second trên 1 CPU core.

---

## 6. KẾ HOẠCH ROLLBACK & GUARDRAILS AN TOÀN

* **Guardrail**: Các regex rule bắt buộc phải là `MustCompile` ở `init()` hoặc `NewInStreamRedactor()`, tuyệt đối không compile regex trong hot path của vòng lặp đọc log.
