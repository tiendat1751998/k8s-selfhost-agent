# Task 037 — Zero-Trust Vault Auto-Unseal & Stale-While-Revalidate JWKS Shield (P1)
# Checkpoint: 2026-10-12T10:45:00+07:00

> **Severity**: P1 (Identity Provider Outages, Token Validation Failures & Vault Initialization Deadlocks)  
> **Target Agent**: `backend-coder` / `security-engineer`  
> **Workspace**: Git Worktree Isolated (`Workspace: "branch"`)  
> **Design Rules**: Stale-While-Revalidate Caching, Auto-Unseal Key Redundancy, Jittered Lease Renewal  

---

## 1. BỐI CẢNH KỸ THUẬT & PHÂN TÍCH LỖI THỰC TẾ (ROOT CAUSE ANALYSIS)

### Lỗi 1: OIDC / OAuth2 JWKS Cache Expiration Làm 100% Request 401 Unauthorized
* **Hiện tượng**: API Gateway cache public keys từ OIDC Provider (Keycloak/Auth0) qua `/.well-known/jwks.json`. Cấu hình cache có TTL ngắn (5 phút). Khi IdP mạng chập chờn hoặc restart trong 30 giây đúng lúc cache JWKS hết hạn, API Gateway không fetch được key mới và purge toàn bộ public keys trong RAM. Mọi JWT token hợp lệ của hàng trăm ngàn người dùng đều bị trả về `HTTP 401 Unauthorized`.
* **Cách xử lý triệt để**:
  1. Triển khai cơ chế **"Stale-While-Revalidate" & Fallback Keyset Cache**:
     - Khi fetch JWKS thất bại hoặc timeout (2s), API Gateway **BẮT BUỘC giữ lại bộ public keys cũ trong RAM trong tối thiểu 24 giờ**.
     - Chấp nhận xác thực JWT bằng key cũ nếu key ID (`kid`) khớp, đồng thời kích hoạt cảnh báo nền gửi về SRE.
  2. Cung cấp Local Static JWKS Backup trong Kubernetes Secret/ConfigMap để làm chốt chặn cuối cùng khi IdP sập hoàn toàn.

### Lỗi 2: HashiCorp Vault Unseal Key Quorum Deadlock Sau Sự Cố Sập Nguồn
* **Hiện tượng**: Cụm server khởi động lại, các pod Vault ở trạng thái **Sealed**. Hệ thống dùng Transit Auto-Unseal nhưng chính cụm Transit Vault lại nằm trong cùng mạng bị sập (Circular Dependency), hoặc kỹ sư giữ Shamir keys không có mặt kịp thời. Không dịch vụ nào có thể đọc database secret để khởi động.
* **Cách xử lý triệt để**:
  - Triển khai **Emergency TPM 2.0 / Local KMS Unseal Operator**:
    - Tự động lưu trữ Shamir key shard đã được mã hóa bất đối xứng gắn với phần cứng TPM 2.0 của máy chủ bare-metal an toàn.
    - Khi phát hiện Vault Sealed $> 60s$ sau boot, operator tự động giải mã shard cục bộ và gọi API `/v1/sys/unseal` tự động.

### Lỗi 3: Secret Lease Renewal Thundering Herd Burst
* **Hiện tượng**: Hàng ngàn pod cùng gọi `/v1/sys/leases/renew` tại thời điểm 85% TTL, làm nghẽn storage backend của Vault (Raft fsync vọt lên 500ms).
* **Cách xử lý triệt để**:
  - Tự động tiêm **Random Jitter** vào Vault Agent sidecar:
    $$\text{RenewTime} = \text{TTL} \times (0.60 + 0.30 \times \text{rand}())$$
  - Phân tán đều các request gia hạn secret trong khoảng thời gian từ 60% đến 90% TTL.

---

## 2. ĐẶC TẢ STRUCTS & INTERFACES (CONTRACT DEFINITION)

Trong `internal/domain/auth/jwks_shield.go`:
```go
package auth

import (
    "context"
    "crypto/rsa"
    "time"
)

type JWKSKey struct {
    KID       string         `json:"kid"`
    PublicKey *rsa.PublicKey `json:"public_key"`
    Algorithm string         `json:"alg"`
    FetchedAt time.Time      `json:"fetched_at"`
    ExpiresAt time.Time      `json:"expires_at"`
}

type JWKSCacheStatus struct {
    TotalKeys       int           `json:"total_keys"`
    IsUsingStaleKey bool          `json:"is_using_stale_key"`
    StaleKeyAge     time.Duration `json:"stale_key_age"`
    LastFetchError  string        `json:"last_fetch_error,omitempty"`
    ProviderURL     string        `json:"provider_url"`
}

type JWKSShield interface {
    GetValidationKey(ctx context.Context, kid string) (*rsa.PublicKey, error)
    RefreshKeys(ctx context.Context) error
    GetCacheStatus() JWKSCacheStatus
}

type VaultUnsealStatus struct {
    IsSealed        bool      `json:"is_sealed"`
    Threshold       int       `json:"threshold"`
    Progress        int       `json:"progress"`
    AutoUnsealMode  string    `json:"auto_unseal_mode"` // TPM2, KMS, MANUAL
    LastUnsealedAt  time.Time `json:"last_unsealed_at"`
}
```

---

## 3. THUẬT TOÂN XỬ LÝ TỪNG BƯỚC (STEP-BY-STEP WORKFLOW)

### Quy trình 1: Xác Thực JWT Bằng Stale-While-Revalidate JWKS Shield
1. Request đến kèm header `Authorization: Bearer <token>`.
2. Parser đọc header của JWT, trích xuất `kid` (Key ID).
3. Tìm kiếm trong RAM Cache:
   - Nếu tìm thấy key và `now < expiresAt`: Xác thực ngay lập tức (Fast Path).
   - Nếu cache hết hạn hoặc không tìm thấy:
     - Kích hoạt background goroutine fetch JWKS mới từ IdP với timeout 2 giây.
     - Trong lúc background fetch, **TIẾP TỤC SỬ DỤNG KEY CŨ TRONG RAM ĐỂ XÁC THỰC** (Zero downtime).
4. Nếu IdP trả về lỗi 5xx hoặc timeout:
   - Giữ nguyên key cũ trong RAM, tăng hạn sử dụng thêm 1 giờ.
   - Ghi nhận `IsUsingStaleKey = true` và bắn cảnh báo vàng về SRE Telegram:
     `⚠️ IdP JWKS endpoint không phản hồi. API Gateway đang dùng Stale Cache để duy trì dịch vụ.`

---

## 4. CHUẨN HÓA MÃ LỖI (RFC 7807 ERROR SPECIFICATION)

* `ERR_JWKS_STALE_KEY_EXHAUSTED`: `HTTP 401 Unauthorized` — "JWT verification failed: Key '%s' expired and Stale-While-Revalidate window exceeded (24h)."
* `ERR_VAULT_SEALED_DEADLOCK`: `HTTP 503 Service Unavailable` — "HashiCorp Vault is sealed and auto-unseal quorum failed. Manual intervention required."

---

## 5. MA TRẬN TEST CASE (VERIFICATION SUITE)

1. `TestJWKS_StaleWhileRevalidate_KeepsValidating`: Giả lập IdP endpoint trả về HTTP 500. Thử xác thực token đã có key trong cache cũ. Assert xác thực thành công và cờ `IsUsingStaleKey == true`.
2. `TestJWKS_StaticBackupFallback`: Xóa toàn bộ cache, IdP down hoàn toàn. Nạp Static JWKS Backup từ ConfigMap. Assert xác thực thành công.
3. `TestVault_JitterRenewalCalculation`: Chạy 1,000 lần tính toán renew time. Assert 100% kết quả nằm rải đều trong khoảng 60% đến 90% TTL, không có hiện tượng chụm đỉnh (Zero Thundering Herd).

---

## 6. KẾ HOẠCH ROLLBACK & GUARDRAILS AN TOÀN

* **Guardrail**: Khi dùng stale key, tối đa thời gian cho phép là 24 giờ. Sau 24 giờ nếu IdP vẫn chết, bắt buộc từ chối token để tránh rủi ro bảo mật do token revoke không được cập nhật.
