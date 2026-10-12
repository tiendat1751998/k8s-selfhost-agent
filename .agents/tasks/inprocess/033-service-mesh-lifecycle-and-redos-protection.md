# Task 033 — Service Mesh Lifecycle & Ingress ReDoS Protection Engine (P1)
# Checkpoint: 2026-10-12T10:45:00+07:00

> **Severity**: P1 (Service Mesh Startup Races, mTLS Connection Drops & ReDoS CPU Spikes)  
> **Target Agent**: `backend-coder` / `devops`  
> **Workspace**: Git Worktree Isolated (`Workspace: "branch"`)  
> **Design Rules**: Native Sidecar Injection, Linear DFA Regex Verification, Graceful TLS Drainage  

---

## 1. BỐI CẢNH KỸ THUẬT & PHÂN TÍCH LỖI THỰC TẾ (ROOT CAUSE ANALYSIS)

### Lỗi 1: Istio Sidecar Startup Race Condition Gây Lỗi Kết Nối Khi Khởi Động
* **Vị trí**: K8s deployments sử dụng Istio / Envoy sidecar.
* **Hiện tượng**: Pod ứng dụng khởi động nhanh hơn container `istio-proxy`. Khi app vừa chạy xong, nó gọi ngay DB migration hoặc remote RPC. Tuy nhiên `iptables` đã redirect traffic ra port 15001 nhưng Envoy chưa nạp xong cấu hình từ Istiod $\implies$ Lỗi `ECONNREFUSED` hoặc TCP Reset `RST`.
* **Cách xử lý triệt để**:
  1. Mutating Admission Webhook tự động kiểm tra và chèn cấu hình `holdApplicationUntilProxyStarts: true` vào pod metadata annotation `proxy.istio.io/config`.
  2. Đối với cụm K8s 1.28+, tự động chuyển đổi cấu hình sidecar sang **Native Sidecar Containers** (`initContainers` có `restartPolicy: Always`). Kubelet bảo đảm sidecar đạt `Ready` trước khi khởi chạy container ứng dụng chính.

### Lỗi 2: Thảm Họa ReDoS (Regular Expression Denial of Service) Khóa Cứng CPU Cores
* **Vị trí**: Ingress Controller / WAF rules / Custom request filters.
* **Hiện tượng**: Một rule WAF mới được thêm vào chứa biểu thức regex lồng nhau dạng `(?:.*=.*)+`. Gặp payload bất thường, engine NFA đệ quy cấp số nhân $O(2^n)$, khóa cứng CPU 100%, Ingress pod đơ và bị restart làm sập cổng vào.
* **Cách xử lý triệt để**:
  1. Thay thế toàn bộ engine PCRE bằng engine DFA tuyến tính $O(n)$ như **Google RE2** hoặc Rust `regex` trong các middleware WAF.
  2. Triển khai Static Regex Linter phân tích các rule mới; từ chối mọi regex có độ phức tạp phi tuyến tính.
  3. Bổ sung hard execution timeout: mỗi lần đánh giá regex tối đa 5ms; quá 5ms tự động ngắt và trả về `StatusWAFTimeout`.

---

## 2. ĐẶC TẢ STRUCTS & INTERFACES (CONTRACT DEFINITION)

Trong `internal/domain/mesh/lifecycle.go`:
```go
package mesh

import (
    "context"
    "time"
)

type SidecarStartupPolicy string

const (
    SidecarPolicyHoldApp       SidecarStartupPolicy = "HOLD_APP_UNTIL_PROXY_READY"
    SidecarPolicyNativeSidecar SidecarStartupPolicy = "K8S_NATIVE_SIDECAR"
)

type RegexVerificationReport struct {
    Pattern       string        `json:"pattern"`
    IsLinearDFA   bool          `json:"is_linear_dfa"`
    MaxEvalTime   time.Duration `json:"max_eval_time"`
    HasReDoSBacktrack bool      `json:"has_redos_backtrack"`
    Verdict       string        `json:"verdict"` // APPROVED, REJECTED
}

type MeshLifecycleManager interface {
    InjectSidecarStartupGuard(ctx context.Context, podSpecBytes []byte) ([]byte, error)
    VerifyWAFRegexSafety(pattern string) (*RegexVerificationReport, error)
    ConfigureGracefulTLSDrain(ctx context.Context, namespace, deployment string, drainDuration time.Duration) error
}
```

---

## 3. THUẬT TOÁN XỬ LÝ TỪNG BƯỚC (STEP-BY-STEP WORKFLOW)

### Bước 1: Mutating Webhook Bảo Đảm Proxy Sẵn Sàng Trước Khi App Chạy
1. Tiếp nhận `AdmissionReview` của Pod:
2. Kiểm tra xem namespace có bật mesh injection hay không (`istio-injection=enabled`).
3. Nếu K8s version $\ge 1.28$:
   - Di chuyển container `istio-proxy` vào mục `initContainers` kèm cờ `restartPolicy: Always`.
4. Nếu K8s version $< 1.28$:
   - Chèn annotation: `proxy.istio.io/config: '{ "holdApplicationUntilProxyStarts": true }'`.
5. Trả về JSON Patch cập nhật Pod manifest an toàn.

### Bước 2: Bảo Vệ Cổng WAF Khỏi Lỗi ReDoS Bằng Google RE2
1. Mọi rule regex WAF khi người dùng cấu hình qua UI/API bắt buộc phải đi qua hàm `VerifyWAFRegexSafety`:
   - Sử dụng thư viện `regexp` chuẩn của Go (được bảo đảm chạy trên thuật toán RE2 DFA tuyến tính $O(n)$).
   - Chạy synthetic benchmark với payload 10,000 ký tự lặp để đo worst-case latency.
   - Nếu phát hiện cú pháp PCRE backreference không tương thích DFA: Trả về lỗi từ chối `ERR_WAF_NON_LINEAR_REGEX`.

---

## 4. CHUẨN HÓA MÃ LỖI (RFC 7807 ERROR SPECIFICATION)

* `ERR_WAF_NON_LINEAR_REGEX`: `HTTP 422 Unprocessable Entity` — "WAF rule rejected: Regular expression contains non-linear backtracking constructs vulnerable to ReDoS."
* `ERR_MESH_PROXY_STARTUP_TIMEOUT`: `HTTP 504 Gateway Timeout` — "Proxy sidecar failed to reach ready state within 60s."

---

## 5. MA TRẬN TEST CASE (VERIFICATION SUITE)

1. `TestSidecar_InjectHoldApplication`: Gửi Pod spec chưa có annotation. Assert patch chèn đúng `holdApplicationUntilProxyStarts: true`.
2. `TestWAF_RejectCatastrophicRegex`: Thử nạp regex `(a+)+$`. Assert hàm `VerifyWAFRegexSafety` phát hiện nguy cơ ReDoS và trả về `REJECTED`.
3. `TestWAF_AcceptLinearRegex`: Thử nạp regex tuyến tính `^[a-zA-Z0-9_\-\.]+@[a-zA-Z0-9\-]+\.[a-zA-Z0-9\-\.]+$`. Assert trả về `APPROVED`.

---

## 6. KẾ HOẠCH ROLLBACK & GUARDRAILS AN TOÀN

* **Guardrail**: Không bao giờ mutate các Pod thuộc control plane hoặc các DaemonSet của CNI/Storage.
