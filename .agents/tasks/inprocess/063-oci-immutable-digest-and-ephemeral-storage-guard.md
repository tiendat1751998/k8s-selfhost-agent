# Task 063 — OCI Immutable Digest Enforcement & Ephemeral Storage Guard (P1)
# Checkpoint: 2026-10-12T13:06:00+07:00

> **Severity**: P1 (Container Tag Mutability, Ephemeral Storage Eviction Cascade & Host PID Exhaustion)  
> **Target Agent**: `backend-coder` / `devops`  
> **Workspace**: Git Worktree Isolated (`Workspace: "branch"`)  
> **Design Rules**: Sha256 Digest Mutation, Ephemeral Storage Cap Webhook, PID Cgroup Limiter  

---

## 1. BỐI CẢNH KỸ THUẬT & PHÂN TÍCH LỖI THỰC TẾ (ROOT CAUSE ANALYSIS)

### Lỗi 1: OCI Container Tag Mutability Gây Lệch Phiên Bản Chạy (Scenario 181)
* **Hiện tượng thực tế**: Developer push lại cùng một tag `v1.0.0` với commit khác. Nửa số node trong cụm K8s chạy cache cũ, nửa còn lại pull cache mới. Cụm chạy 2 phiên bản code cùng lúc dưới một Service.
* **Căn nguyên**: Kubernetes manifest chỉ chỉ định tag dạng string thay vì immutable sha256 digest (`@sha256:...`).

### Lỗi 2: Ephemeral Storage Eviction Cascade Do Log Tràn `/tmp` (Scenario 182)
* **Hiện tượng thực tế**: Pod ghi log unrotated vào `/tmp` làm đầy 85% root disk của node. Kubelet evict pod, pod chuyển sang node khác và lại ghi đầy tiếp, đánh sập cả cụm.
* **Căn nguyên**: Pod không khai báo `resources.limits.ephemeral-storage` và `/tmp` mount trực tiếp từ root filesystem của host.

### Lỗi 3: Linux Host PID Exhaustion Do Rogue Fork Bomb (Scenario 184)
* **Hiện tượng thực tế**: Pod bị fork-bomb hoặc thread-leak làm cạn kiệt `kernel.pid_max` (32768) của host. Host không thể spawn process mới, SSH bị từ chối, Kubelet health check fail.
* **Căn nguyên**: Thiếu giới hạn `podPidsLimit` trong Kubelet config và thiếu cgroup PID admission limiter.

---

## 2. ĐẶC TẢ STRUCTS & INTERFACES (CONTRACT DEFINITION)

Trong `internal/domain/workload/artifact_guard.go`:
```go
package workload

import (
    "context"
)

type ImageDigestResolution struct {
    OriginalImage string `json:"original_image"`
    ResolvedDigest string `json:"resolved_digest"` // e.g. registry/repo@sha256:abc...
    IsImmutable   bool   `json:"is_immutable"`
}

type EphemeralStorageCheck struct {
    HasLimit      bool   `json:"has_limit"`
    LimitBytes    int64  `json:"limit_bytes"`
    TmpMountedAsEmptyDir bool `json:"tmp_mounted_as_empty_dir"`
    IsPassed      bool   `json:"is_passed"`
    RejectReason  string `json:"reject_reason,omitempty"`
}

type WorkloadArtifactGuard interface {
    ResolveImageToDigest(ctx context.Context, image string) (*ImageDigestResolution, error)
    ValidateEphemeralStoragePolicy(ctx context.Context, podSpecJSON []byte) (*EphemeralStorageCheck, error)
    EnforcePodPIDLimit(ctx context.Context, podSpecJSON []byte, defaultMaxPIDs int64) ([]byte, error)
}
```

---

## 3. THUẬT TOÁN XỬ LÝ TỪNG BƯỚC (STEP-BY-STEP WORKFLOW)

### Quy trình 1: Mutating Webhook Chuyển Đổi Tag Sang Immutable SHA256 Digest
1. Khi tiếp nhận request tạo/cập nhật Pod hoặc Deployment:
2. Với mỗi container trong `spec.containers`:
   - Nếu `image` sử dụng tag trôi nổi (`latest`, `dev`, `staging`), từ chối admission ngay lập tức.
   - Nếu `image` đã chứa `@sha256:`, bỏ qua (đã đạt chuẩn bất biến).
   - Nếu `image` dùng tag cố định (ví dụ `app:v1.2.3`), truy vấn Container Registry để lấy HTTP header `Docker-Content-Digest`.
   - Thay thế trường `image` bằng chuỗi immutable digest đầy đủ.

### Quy trình 2: Kiểm Soát Ephemeral Storage & Mount `/tmp`
1. Kiểm tra `resources.limits.ephemeral-storage` trong từng container:
   - Nếu không có: Tự động inject default limit `2Gi`.
2. Kiểm tra volumeMounts của container:
   - Nếu mount `/tmp`: Bắt buộc volume tương ứng phải là `emptyDir` có `sizeLimit`.
   - Nếu không mount: Tự động inject volume `emptyDir` có `sizeLimit: 1Gi` mount vào `/tmp`.

---

## 4. CHUẨN HÓA MÃ LỖI (RFC 7807 ERROR SPECIFICATION)

* `ERR_MUTABLE_IMAGE_TAG_REJECTED`: `HTTP 422 Unprocessable Entity` — "Deployment rejected: Floating image tags (latest, staging) forbidden; digest pinning required."
* `ERR_EPHEMERAL_STORAGE_LIMIT_MISSING`: `HTTP 400 Bad Request` — "Pod rejected: ephemeral-storage limit is required to prevent node disk pressure eviction cascades."

---

## 5. MA TRẬN TEST CASE (VERIFICATION SUITE)

1. `TestImageDigest_ResolveTagToSHA256`: Mock registry response, assert tag `myrepo:v1.0.0` được mutate thành `myrepo@sha256:...`.
2. `TestEphemeralStorage_InjectDefaultLimit`: Feed pod spec thiếu limit, assert webhook inject đúng `limits.ephemeral-storage: 2Gi`.
3. `TestPIDLimit_EnforceMaxPIDs`: Feed pod spec không giới hạn PID, assert webhook cấu hình `podPidsLimit: 1024`.

---

## 6. KẾ HOẠCH ROLLBACK & GUARDRAILS AN TOÀN

* **Guardrail**: Webhook phải có `failurePolicy: Ignore` hoặc circuit breaker ngắn hạn trong trường hợp Container Registry tạm thời không truy cập được để tránh chặn đứng toàn bộ việc triển khai cụm.
