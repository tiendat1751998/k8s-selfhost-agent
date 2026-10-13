# Task 051 — DevSecOps Supply Chain Firewall, IMDSv2 & Sidecar Barrier (P0/P1)
# Checkpoint: 2026-10-12T11:45:00+07:00

> **Severity**: P0/P1 (Supply Chain Hijacking, Cloud SSRF Breaches & Plaintext Traffic Leaks)  
> **Target Agent**: `backend-coder` / `security-engineer`  
> **Workspace**: Git Worktree Isolated (`Workspace: "branch"`)  
> **Design Rules**: Zero-Trust Network Defense, Mandatory IMDSv2, Native Sidecar Ordering  

---

## 1. PHÂN TÍCH LỖI & BẰNG CHỨNG MÃ NGUỒN (ROOT CAUSE ANALYSIS)

### Lỗi 1: Tấn Công Dependency Confusion Qua Public Repositories
* **Cơ chế**: Hacker đăng ký package công khai trùng tên với thư viện nội bộ với version cao hơn (ví dụ 99.0.0). Build runner không có cấu hình scoped proxy sẽ kéo mã độc về thực thi mã từ xa.

### Lỗi 2: Rò Rỉ IAM Token Qua SSRF Với Cloud Metadata IPv6 (`[fd00:ec2::254]`)
* **Vị trí**: `internal/pkg/netguard/validator.go` nếu chỉ chặn dải IPv4 `169.254.169.254` mà bỏ sót IPv6 link-local.
* **Căn nguyên**: Hacker gửi URL IPv6 metadata, ứng dụng tải về và lộ AWS IAM Role credentials.

### Lỗi 3: Sidecar Proxy Khởi Động Chậm Khiến Traffic Lọt Ra Mạng Ngoài Không Mã Hóa
* **Cơ chế**: App container boot trước Envoy proxy trong Service Mesh, gửi dữ liệu plaintext ra ngoài trước khi mTLS được kích hoạt.

---

## 2. CẤU TRÚC DỮ LIỆU & INTERFACES (GO SPECIFICATION)

```go
package secdefense

import (
	"context"
	"net"
)

type DependencyAuditReport struct {
	TotalPackages      int      `json:"total_packages"`
	InternalNamespaces []string `json:"internal_namespaces"`
	SuspiciousPackages []string `json:"suspicious_packages"`
	IsClean            bool     `json:"is_clean"`
}

type SSRFValidationResult struct {
	TargetURL       string `json:"target_url"`
	ResolvedIP      net.IP `json:"resolved_ip"`
	IsBlocked       bool   `json:"is_blocked"`
	BlockReason     string `json:"block_reason"`
	IsCloudMetadata bool   `json:"is_cloud_metadata"`
}

type SecurityDefenseEngine interface {
	AuditDependencies(ctx context.Context, lockfilePath string, privatePrefixes []string) (*DependencyAuditReport, error)
	ValidateURLAgainstSSRF(ctx context.Context, rawURL string) (*SSRFValidationResult, error)
	EnforceIMDSv2(ctx context.Context, instanceID string) error
	ValidateSidecarStartupOrdering(ctx context.Context, podSpecYAML []byte) (bool, error)
}
```

---

## 3. LUỒNG XỬ LÝ & STATE MACHINE

```mermaid
stateDiagram-v2
    [*] --> SSRFInspection: Nhận URL từ người dùng/webhook
    SSRFInspection --> DNSResolution: Phân giải DNS kép (chống Rebinding)
    DNSResolution --> IPv4IPv6Check: Kiểm tra cả 169.254.0.0/16 và fd00::/8
    IPv4IPv6Check --> Blocked: Phát hiện Cloud Metadata / Link-Local
    IPv4IPv6Check --> Allowed: IP Public hợp lệ
    
    Blocked --> AbortRequest: Trả về HTTP 403 Forbidden
    Allowed --> SafeFetch: Thực hiện HTTP Request an toàn
```

---

## 4. CHUẨN HÓA MÃ LỖI (RFC 7807)

* `ERR_SEC_SSRF_METADATA_ATTEMPT`: `HTTP 403 Forbidden` — "Access to cloud instance metadata service (IPv4/IPv6) is strictly prohibited."
* `ERR_SEC_DEPENDENCY_CONFUSION`: `HTTP 422 Unprocessable Entity` — "Private dependency resolution routed to untrusted public registry."

---

## 5. MA TRẬN TEST CASE (VERIFICATION SUITE)

1. `TestNetGuard_IPv6MetadataBlocked`: Kiểm tra URL `http://[fd00:ec2::254]/latest/meta-data/`. Assert validator trả về `IsBlocked: true` với mã lỗi SSRF.
2. `TestNetGuard_DNSRebindingDefense`: Mock DNS trả về IP công cộng lần 1 và IP nội bộ `127.0.0.1` lần 2. Assert request bị từ chối tại thời điểm socket dial.
3. `TestSidecar_OrderingEnforced`: Kiểm tra Pod manifest có initContainer với `restartPolicy: Always`. Assert đạt chuẩn Native Sidecar K8s.

---

## 6. KẾ HOẠCH ROLLBACK & GUARDRAILS AN TOÀN

* **Guardrail**: NetGuard chạy ở tầng HTTP Transport của toàn bộ service, không thể bị bypass qua header tùy biến.
* **Fail-Close**: Mọi lỗi phân giải DNS hoặc nghi ngờ địa chỉ IP đều được xử lý mặc định là Blocked.
