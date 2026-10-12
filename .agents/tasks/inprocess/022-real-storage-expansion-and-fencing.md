# Task 022 — Real Storage Management, PVC Expansion & CSI Fencing (P1)
# Checkpoint: 2026-10-12T10:30:00+07:00

> **Severity**: P1 (Storage Reliability, Zero-Toy Metrics, Safe PVC Expansion & CSI Multi-Attach Fencing)  
> **Target Agent**: `backend-coder`  
> **Workspace**: Git Worktree Isolated (`Workspace: "branch"`)  
> **Design Rules**: Zero-Fake Telemetry, K8s StorageClass Capability Inspection, Non-Graceful Node Fencing  

---

## 1. PHÂN TÍCH LỖI & BẰNG CHỨNG MÃ NGUỒN (ROOT CAUSE ANALYSIS)

### Lỗi 1: Gán Cứng Dung Lượng Đĩa 35% & Bịa Ra 3 Node Ảo
* **Vị trí**: `internal/usecase/storage/volume_usecase.go:121, 148-163`
* **Đoạn code bị lỗi**:
  ```go
  // CODE BỊP BỢM (TOY IMPLEMENTATION):
  usedBytes := int64(float64(vol.CapacityBytes) * 0.35) // Bịa số liệu 35%
  
  // Bịa ra 3 replica node ảo cho Longhorn:
  Replicas: []domain.VolumeReplica{
      {NodeName: "node-1", Mode: "RW", Status: "Healthy"},
      {NodeName: "node-2", Mode: "RW", Status: "Healthy"},
      {NodeName: "node-3", Mode: "RW", Status: "Healthy"},
  }
  ```
* **Căn nguyên**: Thay vì truy vấn Kubelet summary metrics hoặc Prometheus CSI metrics, code gán cứng disk usage là 35%. Khi ổ đĩa thực tế của khách hàng đã đầy 99%, dashboard vẫn thản nhiên báo 35%, dẫn đến thảm họa cơ sở dữ liệu sập đột ngột do hết disk!

### Lỗi 2: Patch Mở Rộng PVC Mù Quáng Gây Treo Disk
* **Vị trí**: `internal/usecase/storage/volume_usecase.go:228-240`
* **Căn nguyên**: Hàm `ExpandVolume` gọi trực tiếp patch K8s PVC mà không kiểm tra xem `StorageClass` có bật `allowVolumeExpansion: true` hay không. Đồng thời không kiểm tra CSI plugin có hỗ trợ online expansion hay không, khiến PVC kẹt vĩnh viễn ở trạng thái `FileSystemResizePending`.

### Lỗi 3: Deadlock Multi-Attach Khi Worker Node Crash Vật Lý
* **Vị trí**: `internal/usecase/sre/node_remediation.go:98-200`
* **Căn nguyên**: Khi worker node chết đột ngột, hàm remediation force-delete pod (`gracePeriodSeconds = 0`). Tuy nhiên K8s CSI controller vẫn coi volume đang được mount trên node cũ. Khi pod StatefulSet DB được khởi động lại trên node mới, nó bị kẹt vĩnh viễn ở `ContainerCreating` với lỗi:
  `Multi-Attach error for volume "pvc-xxx" Volume is already exclusively attached to one node`.

---

## 2. ĐẶC TẢ STRUCTS & INTERFACES (CONTRACT DEFINITION)

Trong `internal/domain/storage/volume.go`:
```go
package storage

import "time"

type VolumeUsageSource string

const (
    UsageSourceKubeletMetrics VolumeUsageSource = "kubelet_metrics"
    UsageSourceCSIDriver      VolumeUsageSource = "csi_driver"
    UsageSourceUnknown        VolumeUsageSource = "unknown"
)

type VolumeDetails struct {
    VolumeID         string            `json:"volume_id"`
    PersistentVolume string            `json:"persistent_volume"`
    ClaimName        string            `json:"claim_name"`
    Namespace        string            `json:"namespace"`
    StorageClassName string            `json:"storage_class_name"`
    CapacityBytes    int64             `json:"capacity_bytes"`
    UsedBytes        *int64            `json:"used_bytes"`          // nil nếu metrics không khả dụng
    UsagePercentage  *float64          `json:"usage_percentage"`    // nil nếu UsedBytes == nil
    UsageSource      VolumeUsageSource `json:"usage_source"`
    AllowExpansion   bool              `json:"allow_expansion"`
    VolumeMode       string            `json:"volume_mode"`
    AccessModes      []string          `json:"access_modes"`
    Phase            string            `json:"phase"`
    AttachedNode     string            `json:"attached_node,omitempty"`
    AttachmentStatus string            `json:"attachment_status,omitempty"`
    UpdatedAt        time.Time         `json:"updated_at"`
}
```

---

## 3. THUẬT TOÁN XỬ LÝ TỪNG BƯỚC (STEP-BY-STEP WORKFLOW)

### Quy trình 1: Truy Vấn Metrics Ổ Đĩa Thực Tế (Real Disk Metrics)
1. Lấy thông tin `PersistentVolumeClaim` và `PersistentVolume`.
2. Gọi Kubelet Summary API `/stats/summary` trên node đang gắn pod hoặc truy vấn Prometheus metric `kubelet_volume_stats_used_bytes`.
3. Nếu lấy được số liệu thực: Gán `UsedBytes = &realUsed`, tính `UsagePercentage`, gán `UsageSource = UsageSourceKubeletMetrics`.
4. Nếu metrics server hoặc kubelet không phản hồi: Gán `UsedBytes = nil`, `UsagePercentage = nil`, `UsageSource = UsageSourceUnknown`.
5. **Tuyệt đối cấm tự động tính toán `capacityBytes * 0.35`.**

### Quy trình 2: Mở Rộng PVC An Toàn (Safe PVC Expansion)
1. Kiểm tra kích thước mới: `newSizeBytes > currentSizeBytes` (K8s không hỗ trợ thu nhỏ PVC; nếu nhỏ hơn, từ chối ngay).
2. Lấy thông tin `StorageClass` từ Kubernetes Client:
   - Nếu `sc.AllowVolumeExpansion == nil || *sc.AllowVolumeExpansion == false`:
     $\implies$ Từ chối thao tác, trả về lỗi `ERR_STORAGE_CLASS_NO_EXPANSION`.
3. Thực hiện Patch PVC manifest: `spec.resources.requests.storage = newSize`.
4. Theo dõi K8s PVC condition:
   - Nếu xuất hiện condition `Resizing == True`: Trả về tiến trình đang thực thi.
   - Nếu xuất hiện condition `FileSystemResizePending == True`: Báo cho người dùng biết cần khởi động lại Pod để filesystem unmount/mount nhận dung lượng mới.

### Quy trình 3: Cô Lập Lưu Trữ Khi Node Chết (Storage Fencing Drill)
1. Khi phát hiện node $N_1$ bị `NotReady` quá 60 giây và có pod StatefulSet đang chạy:
2. Đánh dấu taint cô lập phần cứng: `node.kubernetes.io/out-of-service=nodeshutdown:NoExecute` lên node $N_1$ (K8s 1.28+).
3. Đợi cho controller K8s xác nhận giải phóng `VolumeAttachment` của PV trên node $N_1$.
4. Chỉ sau khi VolumeAttachment đã sạch sẽ, mới cho phép scheduler gán pod DB lên node mới $N_2$ (triệt tiêu 100% lỗi Multi-Attach error).

---

## 4. CHUẨN HÓA MÃ LỖI (RFC 7807 ERROR SPECIFICATION)

* `ERR_STORAGE_SHRINK_UNSUPPORTED`: `HTTP 400 Bad Request` — "Kubernetes does not support shrinking persistent volume claims."
* `ERR_STORAGE_CLASS_NO_EXPANSION`: `HTTP 422 Unprocessable Entity` — "StorageClass '%s' has allowVolumeExpansion=false. Expansion rejected."
* `ERR_VOLUME_ATTACHMENT_LOCKED`: `HTTP 409 Conflict` — "Volume '%s' is locked by offline node '%s'. Storage fencing required."

---

## 5. MA TRẬN TEST CASE (VERIFICATION SUITE)

1. `TestVolumeDetails_RealMetrics_Success`: Mock metrics API trả về 40GB/100GB. Assert `UsagePercentage == 40.0` và `UsageSource == UsageSourceKubeletMetrics`.
2. `TestVolumeDetails_MetricsUnavailable_NoFake`: Mock metrics trả về lỗi. Assert `UsedBytes == nil`, `UsagePercentage == nil`, `UsageSource == UsageSourceUnknown`. Không có bất kỳ con số 35% nào.
3. `TestExpandVolume_StorageClassNotAllowed_Rejects`: Fake StorageClass có `AllowVolumeExpansion: false`. Gọi `ExpandVolume`. Assert trả về lỗi `ERR_STORAGE_CLASS_NO_EXPANSION`.
4. `TestExpandVolume_ShrinkAttempt_Rejects`: Thử giảm size từ 100Gi xuống 50Gi. Assert lỗi `ERR_STORAGE_SHRINK_UNSUPPORTED`.
5. `TestNodeFencing_VolumeAttachmentWait`: Giả lập node offline. Kiểm tra hàm đợi VolumeAttachment được detach trước khi cho phép pod schedule lại.

---

## 6. KẾ HOẠCH ROLLBACK & GUARDRAILS AN TOÀN

* **Guardrail**: Trước khi patch PVC, lưu lại manifest gốc trong audit log. Nếu thao tác patch làm PVC rơi vào trạng thái `Lost`, cung cấp cơ chế khôi phục từ snapshot lưu trữ.
