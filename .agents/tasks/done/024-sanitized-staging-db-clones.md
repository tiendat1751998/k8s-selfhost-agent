# Task 024 — Sanitized Staging Database Clones & Masking Engine (P2)
# Checkpoint: 2026-10-12T10:30:00+07:00

> **Severity**: P2 (Data Privacy, Compliance, PII Protection & Staging Sandbox Isolation)  
> **Target Agent**: `database-engineer` / `backend-coder`  
> **Workspace**: Git Worktree Isolated (`Workspace: "branch"`)  
> **Design Rules**: Zero-PII Leakage, Stream-Based Transformation, Deterministic Anonymization  

---

## 1. PHÂN TÍCH LỖI & BẰNG CHỨNG MÃ NGUỒN (ROOT CAUSE ANALYSIS)

### Thực Trạng Thiếu Hụt Trong Codebase:
* **Vị trí**: Hiện tại trong `internal/domain/` và `internal/usecase/` **hoàn toàn chưa có bất kỳ dòng code nào** phục vụ việc che giấu dữ liệu nhạy cảm (Sanitization / Anonymization).
* **Căn nguyên & Rủi ro**: Khi developer cần dữ liệu thực tế để debug lỗi trên môi trường Staging, quy trình thông thường là lấy trực tiếp bản dump Production nạp vào Staging. Hàng triệu số điện thoại, email, địa chỉ, số CCCD, mã thẻ và mật khẩu khách hàng bị phơi bày trên môi trường Staging/Dev (nơi có độ bảo mật thấp hơn nhiều so với Prod). Điều này vi phạm nghiêm trọng Luật An toàn Thông tin mạng và các tiêu chuẩn bảo mật quốc tế (GDPR, PCI-DSS, ISO 27001).

---

## 2. ĐẶC TẢ STRUCTS & INTERFACES (CONTRACT DEFINITION)

Trong `internal/domain/datasanitizer/entity.go`:
```go
package datasanitizer

import "time"

type RuleType string

const (
    RuleTypeSaltedHash       RuleType = "SALTED_HASH"
    RuleTypeFixedPlaceholder RuleType = "FIXED_PLACEHOLDER"
    RuleTypeFakeGenerator    RuleType = "FAKE_GENERATOR"
    RuleTypeRedact           RuleType = "REDACT"
)

type MaskingColumnRule struct {
    TableName    string   `json:"table_name"`
    ColumnName   string   `json:"column_name"`
    RuleType     RuleType `json:"rule_type"`
    FormatHint   string   `json:"format_hint"` // email, phone, cccd, password
    Salt         string   `json:"salt,omitempty"`
    DefaultValue string   `json:"default_value,omitempty"`
}

type SanitizationPolicy struct {
    PolicyID    string              `json:"policy_id"`
    Database    string              `json:"database"`
    Rules       []MaskingColumnRule `json:"rules"`
    CreatedAt   time.Time           `json:"created_at"`
}

type CloneExecutionStatus string

const (
    StatusDumping    CloneExecutionStatus = "DUMPING"
    StatusMasking    CloneExecutionStatus = "MASKING"
    StatusRestoring  CloneExecutionStatus = "RESTORING"
    StatusCompleted  CloneExecutionStatus = "COMPLETED"
    StatusFailed     CloneExecutionStatus = "FAILED"
)

type CloneExecution struct {
    ExecutionID     string               `json:"execution_id"`
    SourceClusterID string               `json:"source_cluster_id"`
    SourceDB        string               `json:"source_db"`
    TargetClusterID string               `json:"target_cluster_id"`
    TargetDB        string               `json:"target_db"`
    Status          CloneExecutionStatus `json:"status"`
    RowsMasked      int64                `json:"rows_masked"`
    StartedAt       time.Time            `json:"started_at"`
    FinishedAt      *time.Time           `json:"finished_at,omitempty"`
    ErrorMessage    string               `json:"error_message,omitempty"`
}
```

---

## 3. THUẬT TOÁN XỬ LÝ TỪNG BƯỚC (STEP-BY-STEP WORKFLOW)

### Quy trình Nhân Bản Dữ Liệu Ẩn Danh (Sanitization Pipeline):
1. **Khởi tạo Pipeline**: Nhận yêu cầu nhân bản DB từ Production sang Sandbox Staging.
2. **Nạp Chính Sách Ẩn Danh (Policy Loading)**:
   - Các trường mặc định:
     - `email` $\implies$ `sha256(email + salt)[0:10] + "@staging.internal"`
     - `phone` $\implies$ `0900000000` hoặc số random format `09XXXXXXXX`
     - `password_hash` $\implies$ Mã hash bcrypt cố định của mật khẩu `StagingDefaultPass123!`
     - `api_key` / `token` $\implies$ `sk_test_staging_placeholder`
     - `credit_card` $\implies$ `4111-XXXX-XXXX-1111`
3. **Thực thi Stream Pipeline (Không Ghi Đĩa Trung Gian)**:
   ```
   [pg_dump Production] ---> [In-Stream SQL Masker / Transformer] ---> [pg_restore Staging DB]
   ```
4. **Đối Soát Sau Khi Nạp**:
   - Chạy query kiểm tra trên Staging DB: Assert không còn bất kỳ email nào chứa domain thực tế của khách hàng.
5. **Bàn giao**: Cấp connection string của Staging DB cho developer.

---

## 4. CHUẨN HÓA MÃ LỖI (RFC 7807 ERROR SPECIFICATION)

* `ERR_SANITIZATION_POLICY_INVALID`: `HTTP 400 Bad Request` — "Masking rule invalid for column '%s': Unsupported rule type."
* `ERR_CLONE_PII_LEAK_DETECTED`: `HTTP 500 Internal Server Error` — "Post-sanitization audit failed: Unmasked PII detected in target database. Aborting clone."

---

## 5. MA TRẬN TEST CASE (VERIFICATION SUITE)

1. `TestMasker_EmailTransformation`: Feed chuỗi SQL `INSERT INTO users (email) VALUES ('real_customer@gmail.com');`. Assert chuỗi sau khi stream biến thành `INSERT INTO users (email) VALUES ('[hash]@staging.internal');`.
2. `TestMasker_PasswordReset`: Feed mật khẩu hash thật của production. Assert sau khi stream biến thành hash chuẩn staging.
3. `TestSanitizer_EndToEnd_Postgres`: Chạy pipeline trên DB test mẫu. Query kiểm tra bảng kết quả: 100% bản ghi PII được thay thế thành công.

---

## 6. KẾ HOẠCH ROLLBACK & GUARDRAILS AN TOÀN

* **Bảo vệ Tối Cao (Safety Guardrail)**: Pipeline bắt buộc kiểm tra `TargetDB` và `TargetClusterID`. Tuyệt đối cấm trỏ target vào Production cluster (chống vô tình ghi đè làm mất dữ liệu production).
