# Task 018 — Crash Elimination & Multi-Tenancy Data Isolation (P0)
# Checkpoint: 2026-10-12T10:30:00+07:00

> **Severity**: P0 (Production Crash, Concurrency Data Race & Tenant Data Isolation)  
> **Target Agent**: `backend-coder` / `database-engineer`  
> **Workspace**: Git Worktree Isolated (`Workspace: "branch"`)  
> **Design Rules**: Zero-Nil Dereference, Lock Contention Elimination, Strict Tenancy Boundary  

---

## 1. PHÂN TÍCH LỖI & BẰNG CHỨNG MÃ NGUỒN (ROOT CAUSE ANALYSIS)

### Lỗi 1: Nil Pointer Panic trong K8s Workload Explorer
* **Vị trí**: `internal/adapter/http/explorer_k8s.go:54`
* **Đoạn code bị lỗi**:
  ```go
  // CODE HIỆN TẠI (CRASH NGUY HIỂM):
  Replicas: *d.Spec.Replicas,
  ```
* **Căn nguyên**: Trong Kubernetes API (`k8s.io/api/apps/v1`), trường `Spec.Replicas` của `Deployment` là một con trỏ `*int32`. Khi một deployment bị tạm dừng (`spec.paused: true`), hoặc được quản lý bởi HPA bên ngoài mà chưa gán replicas tĩnh, hoặc manifest rỗng, con trỏ này là `nil`. Việc dereference trực tiếp `*d.Spec.Replicas` khiến tiến trình Go panic và crash HTTP worker ngay lập tức:
  ```
  panic: runtime error: invalid memory address or nil pointer dereference
  [signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x...]
  ```

### Lỗi 2: Data Race Gây Crash Go Runtime trong HealthCenter Cache
* **Vị trí**: `internal/infrastructure/kubernetes/healthcenter_repo.go:18, 139, 148`
* **Đoạn code bị lỗi**:
  ```go
  // CODE HIỆN TẠI (KHÔNG CÓ KHÓA):
  type HealthCenterRepo struct {
      client *kubernetes.Clientset
      cache  map[string]healthcenter.ComponentStatus // Concurrent read/write crash
  }
  func (r *HealthCenterRepo) GetComponentStatus(name string) ... {
      return r.cache[name] // Concurrent Read
  }
  func (r *HealthCenterRepo) UpdateComponentStatus(...) {
      r.cache[name] = status // Concurrent Write
  }
  ```
* **Căn nguyên**: Map trong Go không phải là cấu trúc an toàn luồng (thread-safe). Khi có nhiều HTTP request đồng thời gọi health check hoặc background poller cập nhật trạng thái, Go Runtime phát hiện xung đột và dừng chương trình khẩn cấp:
  `fatal error: concurrent map read and map write`.

### Lỗi 3: Rò Rỉ Dữ Liệu Multi-Tenancy Trên 12 PostgreSQL Repositories
* **Vị trí**: 12 repositories trong `internal/infrastructure/postgres/`:
  `agent_repo.go`, `alert_repo.go`, `audit_repo.go`, `backup_repo.go`, `cluster_repo.go`, `cost_repo.go`, `deployment_repo.go`, `incident_repo.go`, `logging_repo.go`, `metric_repo.go`, `plugin_repo.go`, `user_repo.go`.
* **Căn nguyên**: Các câu lệnh SQL SELECT/UPDATE/DELETE chỉ filter theo `id = $1` mà bỏ quên điều kiện `AND tenant_id = $2`. Người dùng Tenant A có thể đoán ID hoặc sửa param URL để xem và thao tác dữ liệu của Tenant B, vi phạm nghiêm trọng tính cô lập đa người thuê (Multi-Tenant Isolation).

---

## 2. ĐẶC TẢ STRUCTS & INTERFACES (CONTRACT DEFINITION)

### HealthCenter Thread-Safe Contract:
```go
package kubernetes

import (
    "sync"
    "k8s-selfhost/internal/domain/healthcenter"
)

type HealthCenterRepo struct {
    client    kubernetes.Interface
    mu        sync.RWMutex
    cache     map[string]healthcenter.ComponentStatus
    updatedAt map[string]time.Time
}

func NewHealthCenterRepo(client kubernetes.Interface) *HealthCenterRepo {
    return &HealthCenterRepo{
        client:    client,
        cache:     make(map[string]healthcenter.ComponentStatus),
        updatedAt: make(map[string]time.Time),
    }
}
```

### Tenancy Guard Query Contract:
Trong `internal/pkg/tenancy/query.go`:
```go
package tenancy

import "fmt"

type TenantContext interface {
    TenantID() string
    IsPlatformAdmin() bool
}

// EnforceTenantQuery tự động gắn tenant_id vào mệnh đề WHERE
func EnforceTenantQuery(baseQuery string, tenantID string, argIndex int) (string, []interface{}) {
    // Luôn đảm bảo query có AND tenant_id = $N
}
```

---

## 3. THUẬT TOÁN XỬ LÝ TỪNG BƯỚC (STEP-BY-STEP WORKFLOW)

### Bước 1: Sửa Triệt Để Nil Replicas Trong `explorer_k8s.go`
1. Kiểm tra an toàn:
   ```go
   var replicas int32 = 1
   if d.Spec.Replicas != nil {
       replicas = *d.Spec.Replicas
   }
   ```
2. Áp dụng tương tự cho `d.Status.ReadyReplicas`, `d.Status.UpdatedReplicas`, `d.Status.AvailableReplicas`.

### Bước 2: Bảo Vệ Map Cache Bằng `sync.RWMutex`
1. Mọi thao tác đọc map (`GetComponentStatus`, `GetAllComponentStatuses`):
   ```go
   r.mu.RLock()
   defer r.mu.RUnlock()
   status, exists := r.cache[name]
   ```
2. Mọi thao tác ghi map (`UpdateComponentStatus`, `InvalidateCache`):
   ```go
   r.mu.Lock()
   defer r.mu.Unlock()
   r.cache[name] = status
   r.updatedAt[name] = time.Now()
   ```

### Bước 3: Rà Soát & Cập Nhật 12 Postgres Repositories
1. Mở từng file repository.
2. Tìm kiếm tất cả các hàm `GetByID`, `List`, `Update`, `Delete`.
3. Bổ sung tham số `tenantID string` vào domain repository interface và implementation.
4. Chèn `AND tenant_id = $N` vào câu truy vấn parameterized SQL:
   ```sql
   -- TRƯỚC (LỖI RÒ RỈ):
   SELECT id, name, status FROM clusters WHERE id = $1;
   
   -- SAU (CÔ LẬP CHUẨN MỰC):
   SELECT id, name, status FROM clusters WHERE id = $1 AND tenant_id = $2;
   ```

---

## 4. CHUẨN HÓA MÃ LỖI (RFC 7807 ERROR SPECIFICATION)

* `ERR_UNAUTHORIZED_TENANT_ACCESS`: `HTTP 403 Forbidden` — "Access denied: Entity does not belong to the requesting tenant context."
* `ERR_WORKLOAD_NOT_FOUND`: `HTTP 404 Not Found` — "Workload '%s' not found in namespace '%s' for tenant '%s'."

---

## 5. MA TRẬN TEST CASE (VERIFICATION SUITE)

1. `TestExplorer_NilReplicas_NoPanic`: Tạo `appsv1.Deployment` với `Spec.Replicas = nil`. Gọi `formatDeploymentDTO`. Assert không panic, `Replicas == 1` hoặc `0` tùy logic mặc định.
2. `TestHealthCenter_ConcurrentReadWrite_RaceFree`: Khởi chạy 50 goroutines đọc và 50 goroutines ghi đồng thời vào `HealthCenterRepo`. Chạy lệnh `go test -race -count=1 ./internal/infrastructure/kubernetes/...`. Assert **Zero Data Race Warnings**.
3. `TestPostgresRepo_TenantIsolation`:
   - Tạo Tenant A và Tenant B.
   - Tenant A tạo Cluster $C_A$.
   - Dùng ngữ cảnh Tenant B gọi `GetByID(ctx, $C_A)`.
   - Assert trả về lỗi `sql.ErrNoRows` hoặc `ErrUnauthorizedTenantAccess`. Tuyệt đối không trả về dữ liệu của Tenant A.

---

## 6. KẾ HOẠCH ROLLBACK & GUARDRAILS AN TOÀN

* **Guardrail**: Biên dịch và chạy vet trên toàn bộ codebase: `go build ./... && go vet ./...`.
* **Rollback Plan**: Nếu việc thay đổi repository interface làm gãy các usecase caller, revert git commit và cập nhật đồng bộ các caller trước khi merge.
