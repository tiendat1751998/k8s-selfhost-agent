# Task 021 — DevSecOps Fail-Close Security Gates & Supply Chain (P1)
# Checkpoint: 2026-10-12T10:30:00+07:00

> **Severity**: P1 (Security Gate Integrity, Zero Fake-Pass, Supply Chain Verification & Fail-Close Invariant)  
> **Target Agent**: `backend-coder` / `security-engineer`  
> **Workspace**: Git Worktree Isolated (`Workspace: "branch"`)  
> **Design Rules**: Strict Zero-Trust, Fail-Close Enforcement, Cryptographic Image Verification  

---

## 1. PHÂN TÍCH LỖI & BẰNG CHỨNG MÃ NGUỒN (ROOT CAUSE ANALYSIS)

### Lỗi 1: Checkov Scanner Bịa Báo Cáo Pass 100% Khi Gặp Lỗi
* **Vị trí**: `internal/infrastructure/security/checkov_scanner.go:47-50, 128-137`
* **Đoạn code bị lỗi**:
  ```go
  // CODE BỊP BỢM (NGUY HIỂM BẢO MẬT TỐI CAO):
  if err != nil {
      // Khi Checkov binary không tìm thấy hoặc crash, trả về 100% Pass giả lập:
      return fallbackReport(PassedChecks: 10, FailedChecks: 0, ScorePercentage: 100.0), nil
  }
  ```
* **Căn nguyên**: Đoạn mã này được viết kiểu đối phó để "cho qua" trong môi trường test/dev. Khi triển khai ra Production, nếu hacker xóa file binary checkov hoặc làm crash tiến trình scan, hệ thống tự động báo cáo "Hạ tầng an toàn 100%" và cho phép triển khai manifest chứa quyền `privileged: true` hoặc rò rỉ secret ra Internet!

### Lỗi 2: Trivy Scanner Tự Động Xóa Sạch CVE Khi Thất Bại
* **Vị trí**: `internal/infrastructure/security/trivy_scanner.go:53-56, 107-119`
* **Đoạn code bị lỗi**:
  ```go
  // CODE BỊP BỢM TƯƠNG TỰ:
  if err != nil {
      return fallbackScanReport(TotalCVEs: 0, PassSecurityGate: true), nil
  }
  ```
* **Căn nguyên**: Image chứa lỗ hổng RCE nghiêm trọng (CVSS 10.0), nhưng chỉ cần Trivy timeout hoặc hết dung lượng RAM, hệ thống lập tức cấp giấy phép "0 CVE - Pass" để đẩy thẳng image lên production!

---

## 2. ĐẶC TẢ STRUCTS & INTERFACES (CONTRACT DEFINITION)

### Security Gate Result Contract:
Trong `internal/domain/security/gate.go`:
```go
package security

type GateStatus string

const (
    GateStatusPassed             GateStatus = "PASSED"
    GateStatusFailed             GateStatus = "FAILED"
    GateStatusScannerUnavailable GateStatus = "SCANNER_UNAVAILABLE"
    GateStatusPolicyViolation    GateStatus = "POLICY_VIOLATION"
)

type SecurityGateReport struct {
    ScanID              string            `json:"scan_id"`
    Target              string            `json:"target"` // Image tag hoặc Manifest path
    Status              GateStatus        `json:"status"`
    Passed              bool              `json:"passed"`
    CriticalCount       int               `json:"critical_count"`
    HighCount           int               `json:"high_count"`
    MediumCount         int               `json:"medium_count"`
    LowCount            int               `json:"low_count"`
    Vulnerabilities     []Vulnerability   `json:"vulnerabilities,omitempty"`
    FailedChecks        []IaCFailedCheck  `json:"failed_checks,omitempty"`
    CosignVerified      bool              `json:"cosign_verified"`
    ErrorMessage        string            `json:"error_message,omitempty"`
    ScannedAt           time.Time         `json:"scanned_at"`
}
```

---

## 3. THUẬT TOÁN XỬ LÝ TỪNG BƯỚC (STEP-BY-STEP WORKFLOW)

### Nguyên Tắc Bất Biến: The Fail-Close Invariant
$$\text{Gate Policy} = \text{STRICT} \implies \begin{cases} \text{Critical CVE} > 0 \implies \mathbf{REJECT} \\ \text{Scanner Error / Missing} \implies \mathbf{REJECT (ScannerUnavailable)} \\ \text{Cosign Invalid / Unsigned} \implies \mathbf{REJECT} \end{cases}$$

### Quy trình Kiểm Soát An Ninh (Security Gate Pipeline):
1. **Tiếp nhận Artifact**: Nhận Container Image digest hoặc Kubernetes YAML manifest.
2. **Quét IaC (Checkov)**:
   - Thực thi `checkov -f manifest.yaml --output json`.
   - Nếu binary không tìm thấy hoặc exit code $\neq 0$:
     - Ghi nhận `Status = GateStatusScannerUnavailable`, `Passed = false`.
     - Lập tức chặn tiến trình rollout, ghi log cảnh báo SRE.
     - **Tuyệt đối cấm trả về fallback report 100%.**
3. **Quét Lỗ Hổng Container (Trivy)**:
   - Thực thi `trivy image --severity CRITICAL,HIGH --format json <image>`.
   - Nếu có bất kỳ `CRITICAL` CVE nào:
     - Ghi nhận `Status = GateStatusPolicyViolation`, `Passed = false`.
     - Chặn deployment và gửi thông báo Telegram kèm danh sách CVEs.
4. **Xác Thực Chữ Ký Số (Cosign)**:
   - Kiểm tra chữ ký số công khai của image qua `cosign verify --key cosign.pub <image>`.
   - Nếu image chưa được ký hoặc chữ ký không hợp lệ: Từ chối khởi chạy pod.

---

## 4. CHUẨN HÓA MÃ LỖI (RFC 7807 ERROR SPECIFICATION)

* `ERR_SECURITY_SCANNER_UNAVAILABLE`: `HTTP 503 Service Unavailable` — "Security gate execution failed: Scanner engine '%s' is unavailable. Deployment rejected under Fail-Close policy."
* `ERR_SECURITY_GATE_REJECTED`: `HTTP 422 Unprocessable Entity` — "Deployment rejected: Artifact violates security gate policy. Critical CVEs: %d, Failed IaC Checks: %d."

---

## 5. MA TRẬN TEST CASE (VERIFICATION SUITE)

1. `TestCheckov_BinaryMissing_FailClose`: Giả lập biến môi trường trỏ vào binary không tồn tại. Gọi `ScanManifest`. Assert kết quả trả về `Passed == false` và `Status == GateStatusScannerUnavailable`. Không có bất kỳ con số 100% giả mạo nào.
2. `TestTrivy_ExecutionError_FailClose`: Giả lập Trivy trả về exit code 1. Assert kết quả trả về `Passed == false`.
3. `TestSecurityGate_CriticalCVE_BlocksDeployment`: Feed một scan report giả lập có 1 Critical CVE. Assert hàm `EvaluateGate` trả về `false` và kích hoạt alert P1.
4. `TestCosign_UnsignedImage_Rejects`: Thử nạp image không có signature. Assert cổng từ chối triển khai.

---

## 6. KẾ HOẠCH ROLLBACK & GUARDRAILS AN TOÀN

* **Guardrail**: Khi ở môi trường Dev local (`ENV=development`), cho phép cấu hình cờ `SECURITY_GATE_MODE=permissive` (chỉ log warning, không chặn). Tuy nhiên trên Production (`ENV=production`), bắt buộc `SECURITY_GATE_MODE=strict` (Fail-Close cưỡng chế, không thể bypass).
