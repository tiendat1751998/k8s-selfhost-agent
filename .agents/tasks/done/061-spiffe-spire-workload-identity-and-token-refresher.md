# Task 061 — SPIFFE/SPIRE Workload Identity & Token Rotation Safeguard (P1)
# Checkpoint: 2026-10-12T13:06:00+07:00

> **Severity**: P1 (Zero-Trust Identity, SVID Thundering Herd, TokenRequest Expiration & OIDC Clock Skew)  
> **Target Agent**: `backend-coder` / `security-engineer`  
> **Workspace**: Git Worktree Isolated (`Workspace: "branch"`)  
> **Design Rules**: Jittered SVID Renewal, Inotify Token Watcher, Monotonic Generation Fencing, Leeway Clock Defense  

---

## 1. BỐI CẢNH KỸ THUẬT & PHÂN TÍCH LỖI THỰC TẾ (ROOT CAUSE ANALYSIS)

### Lỗi 1: SPIFFE/SPIRE SVID Thundering Herd Khi Node Phục Hồi (Scenarios 171)
* **Hiện tượng thực tế**: Sau khi node bị khởi động lại, 200 Pods đồng thời gửi yêu cầu cấp phát X.509 SVID qua `/tmp/spire-agent/public/api.sock`. Socket backlog bị tràn, SPIRE Agent cạn kiệt CPU, và các kết nối mTLS giữa các microservices bị từ chối với lỗi `Bad Certificate`.
* **Căn nguyên**: SPIFFE Workload API thiếu cơ chế jitter khi quay vòng chứng chỉ và thiếu local cache trên client SDK.

### Lỗi 2: Kubernetes Projected Token Stale Mount & 401 Unauthorized Loop (Scenario 172)
* **Hiện tượng thực tế**: Kubelet không kịp refresh projected service account token file tại `/var/run/secrets/kubernetes.io/serviceaccount/token` do API server nghẽn. Ứng dụng đọc file một lần khi start và giữ token đã hết hạn trong RAM, nhận lỗi 401 vĩnh viễn tới AWS/Vault/GCP.
* **Căn nguyên**: Thiếu filesystem watcher giám sát mtime của file token và thiếu logic kiểm tra claim `exp` để proactive refresh.

### Lỗi 3: OIDC Clock Skew Gây Ra Lockout Toàn Cụm (Scenario 173)
* **Hiện tượng thực tế**: Ingress node đồng hồ chạy nhanh hơn OIDC Provider 2 giây (`iat` ở tương lai). Mọi request đăng nhập bị OAuth2-Proxy / Envoy JWT reject với lỗi `token used before issued`.

---

## 2. ĐẶC TẢ STRUCTS & INTERFACES (CONTRACT DEFINITION)

Trong `internal/domain/security/workload_identity.go`:
```go
package security

import (
    "context"
    "crypto/x509"
    "time"
)

type SVIDStatus struct {
    SPIFFEID       string        `json:"spiffe_id"`
    ExpiresAt      time.Time     `json:"expires_at"`
    TimeToLive     time.Duration `json:"time_to_live"`
    IsNearExpiry   bool          `json:"is_near_expiry"` // TTL < 50%
    CertSerial     string        `json:"cert_serial"`
}

type TokenWatchStatus struct {
    TokenPath      string    `json:"token_path"`
    LastRefreshed  time.Time `json:"last_refreshed"`
    ExpiresAt      time.Time `json:"expires_at"`
    ClockSkewSecs  float64   `json:"clock_skew_secs"`
    IsStale        bool      `json:"is_stale"`
}

type WorkloadIdentityManager interface {
    InspectSVID(ctx context.Context, socketPath string) (*SVIDStatus, error)
    RenewSVIDWithJitter(ctx context.Context, socketPath string, maxJitter time.Duration) (*SVIDStatus, error)
    WatchProjectedToken(ctx context.Context, tokenPath string, leeway time.Duration) (*TokenWatchStatus, error)
    ValidateTokenClockSkew(ctx context.Context, tokenIAT time.Time, allowedSkew time.Duration) error
}
```

---

## 3. THUẬT TOÁN XỬ LÝ TỪNG BƯỚC (STEP-BY-STEP WORKFLOW)

### Quy trình 1: Tự Động Xoay Vòng SVID Kèm Random Jitter
1. Client kiểm tra thời gian sống của X.509 SVID định kỳ 30s.
2. Nếu thời gian sống còn lại $\le 50\%$:
   - Tính toán Jitter ngẫu nhiên: $J = \text{rand}(0, 0.2 \times \text{TTL})$.
   - Đặt lịch renew tại thời điểm $\text{Now}() + J$.
   - Khi chạm mốc, gửi CSR tới SPIRE Agent qua domain socket với timeout 5s.
3. Nếu SPIRE Agent trả về HTTP 429 hoặc socket busy:
   - Áp dụng Exponential Backoff with Jitter: $2^n \times 100\text{ms} + \text{rand}(50\text{ms})$.
   - Tiếp tục phục vụ request bằng SVID hiện tại (vẫn còn 30-50% hạn).

### Quy trình 2: Giám Sát Projected Token Qua Inotify
1. Khởi tạo `fsnotify.Watcher` trên file `/var/run/secrets/kubernetes.io/serviceaccount/token`.
2. Lắng nghe sự kiện `Write` hoặc `Create`:
   - Đọc nội dung token mới vào atomic pointer trong RAM.
   - Giải mã unverified payload để trích xuất claim `exp`.
3. Nếu không nhận được sự kiện inotify và thời gian còn lại trước khi hết hạn $< 5\text{ phút}$:
   - Kích hoạt cơ chế Fallback: Gửi lệnh `TokenRequest` khẩn cấp tới Kubernetes API Server để cấp token mới.

---

## 4. CHUẨN HÓA MÃ LỖI (RFC 7807 ERROR SPECIFICATION)

* `ERR_SVID_BACKLOG_SATURATED`: `HTTP 503 Service Unavailable` — "SPIRE Agent socket backlog saturated; renewal deferred with jitter."
* `ERR_TOKEN_CLOCK_SKEW_EXCEEDED`: `HTTP 401 Unauthorized` — "JWT token iat is in the future beyond acceptable clock leeway (skew: %s, leeway: %s)."

---

## 5. MA TRẬN TEST CASE (VERIFICATION SUITE)

1. `TestSVID_RenewWithJitter_CalculatesStaggeredTime`: Khởi tạo 100 client SVID, assert thời gian renew phân bổ đều trong khoảng jitter, không có 2 request cùng miligiây.
2. `TestTokenWatcher_InotifyReload_AtomicUpdate`: Ghi file token mới, assert atomic pointer trong RAM trỏ ngay tới token mới mà không cần restart service.
3. `TestToken_ClockSkew_WithinLeeway_Accepts`: Feed token có `iat` 5 giây ở tương lai, assert chấp nhận thành công với leeway 60s.

---

## 6. KẾ HOẠCH ROLLBACK & GUARDRAILS AN TOÀN

* **Guardrail**: Khi cấp token khẩn cấp, chỉ cấp scoped token với thời hạn tối đa 1 giờ; tuyệt đối không cấp cluster-admin token.
