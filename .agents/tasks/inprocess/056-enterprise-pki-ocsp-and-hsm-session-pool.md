# Task 056: Enterprise PKI OCSP Enforcer & HSM Session Pool Guard
**Status**: `READY_FOR_IMPLEMENTATION`  
**Priority**: `P1_HIGH`  
**FMEA Scenarios Covered**: Scenario 146, 147, 148, 149, 150 (Tầng 33)  
**Target Package Scope**: `internal/infrastructure/crypto/` & `internal/usecase/crypto/`

---

## 1. Root Cause Analysis (Verbatim Production Failure Mode)

### Problematic Scenario
Trong hệ thống mã hóa và xác thực bảo mật doanh nghiệp:
1. **OCSP Responder Blackout**: CA OCSP server bị ngắt kết nối khiến client từ chối handshake TLS (Fail-Closed), làm sập 100% kết nối ngân hàng/thanh toán toàn cầu do thiếu OCSP Stapling.
2. **HSM Session Pool Saturation**: PKCS#11 sessions không được giải phóng khi ứng dụng gặp exception, chạm giới hạn 256 sessions làm đóng băng mọi thao tác ký số JWT.
3. **OpenSSL FIPS Mode Abort**: Thư viện legacy gọi hàm MD5/SHA1 gây crash ngay lập tức `digital envelope routines::disabled for fips`.
4. **KMS Outage DEK Eviction**: LRU cache xóa Data Encryption Key (DEK) đúng lúc Cloud KMS bị gián đoạn 15 phút, khiến ứng dụng không thể giải mã dữ liệu dù DB hoàn toàn bình thường.

---

## 2. Go Structs & Interface Contracts

```go
package crypto

import (
	"context"
	"time"
)

type OCSPStapleStatus struct {
	Domain          string    `json:"domain"`
	HasStaple       bool      `json:"has_staple"`
	NextUpdate      time.Time `json:"next_update"`
	IsExpired       bool      `json:"is_expired"`
	ResponderURL    string    `json:"responder_url"`
}

type HSMSessionPoolStatus struct {
	MaxSessions       int           `json:"max_sessions"`
	ActiveSessions    int           `json:"active_sessions"`
	AvailableSessions int           `json:"available_sessions"`
	AcquireWaitTime   time.Duration `json:"acquire_wait_time"`
	IsSaturated       bool          `json:"is_saturated"`
}

type EnvelopeCacheEntry struct {
	KeyID         string    `json:"key_id"`
	PlaintextKey  []byte    `json:"-"`
	CachedAt      time.Time `json:"cached_at"`
	TTL           time.Duration `json:"ttl"`
	GracePeriod   time.Duration `json:"grace_period"`
	IsStaleValid  bool      `json:"is_stale_valid"`
}

type EnterpriseCryptoShield interface {
	VerifyAndPrefetchOCSPStaple(ctx context.Context, certChainPEM []byte) (*OCSPStapleStatus, error)
	AcquireHSMSession(ctx context.Context, timeout time.Duration) (sessionID uint64, release func(), err error)
	GetOrExtendEnvelopeDEK(ctx context.Context, keyID string, kmsFallbackFunc func() ([]byte, error)) ([]byte, error)
	ScanSourceForNonFIPSAlgorithms(ctx context.Context, rootDir string) (violations []string, err error)
}
```

---

## 3. Step-by-Step State Machine Algorithm

```
State: ACTIVE
  │
  ├─► [Event: OCSP_STAPLE_REFRESH_TICK (every 1h)]
  │     ├─ Inspect cert NextUpdate timestamp
  │     ├─ If Now > NextUpdate - 4h:
  │     │     ├─ Fetch signed OCSP response from CA
  │     │     ├─ Verify signature against Issuer Cert
  │     │     └─ Cache staple in Envoy / Ingress Nginx -> State: ACTIVE
  │     └─ If CA unreachable, retain last valid staple up to 24h grace period
  │
  ├─► [Event: HSM_SESSION_REQUEST]
  │     ├─ Attempt non-blocking acquire from bounded semaphore channel
  │     ├─ If timeout 500ms exceeded -> Return ERROR_HSM_POOL_SATURATED (Fast Fail)
  │     └─ On successful work, deferred release returns token to channel
  │
  └─► [Event: ENVELOPE_DECRYPT_REQUEST]
        ├─ Lookup DEK in LRU cache
        ├─ If Miss -> Call Cloud KMS Decrypt
        ├─ If Cloud KMS returns 503 / Network Error:
        │     ├─ Check if stale key exists within 4-hour Grace Window
        │     ├─ If Yes: Use stale key and log SEC_AUDIT_STALE_DEK_USED
        │     └─ If No: Return ERROR_KMS_UNREACHABLE_NO_GRACE
```

---

## 4. RFC 7807 Problem Details Error Codes

- `ERROR_OCSP_STAPLE_FETCH_FAILED`: Không thể prefetch OCSP staple từ CA.
- `ERROR_HSM_POOL_SATURATED`: Vượt quá số lượng session đồng thời tới HSM.
- `ERROR_FIPS_NON_COMPLIANT_HASH`: Phát hiện MD5/SHA1 khi FIPS mode đang bật.
- `ERROR_KMS_DEK_GRACE_EXPIRED`: Hết hạn grace period của Data Encryption Key.

---

## 5. Verification Test Matrix

1. `TestOCSPEnforcer_PrefetchesAndCachesStaple`: Verify prefetch logic updates before expiry.
2. `TestHSMSessionPool_RejectsOnSaturationWithinTimeout`: Verify bounded pool fast-fails in 500ms.
3. `TestEnvelopeCache_UsesStaleKeyOnKMSFailure`: Mock KMS 503, verify decryption succeeds within grace window.

---

## 6. Rollback Guardrails

- Không kéo dài stale DEK quá 4 giờ để tuân thủ PCI-DSS và tránh rủi ro bảo mật khóa đã bị thu hồi.
