# Task 025 — Real Disaster Recovery (etcd/Velero) & Restore Drills (P2)
# Checkpoint: 2026-10-12T10:30:00+07:00

> **Severity**: P2 (Cluster DR, Real etcd Snapshot, Velero CRDs & Automated Restore Drills)  
> **Target Agent**: `devops` / `backend-coder`  
> **Workspace**: Git Worktree Isolated (`Workspace: "branch"`)  
> **Design Rules**: Zero-Fake DR, Ephemeral Sandbox Testing, Verifiable RTO/RPO Metrics  

---

## 1. PHÂN TÍCH LỖI & BẰNG CHỨNG MÃ NGUỒN (ROOT CAUSE ANALYSIS)

### Lỗi 1: etcd Snapshot Giả Lập Bằng Struct In-Memory
* **Vị trí**: `internal/usecase/dr/dr_usecase.go:94-100, 139-144`
* **Đoạn code bị lỗi**:
  ```go
  // CODE BỊP BỢM (MOCK IN-MEMORY TRONG PRODUCTION USECASE):
  func (u *drUsecase) TriggerEtcdSnapshot(ctx context.Context, clusterID string) (*domain.EtcdSnapshot, error) {
      snapshot := &domain.EtcdSnapshot{
          ID:        uuid.New().String(),
          ClusterID: clusterID,
          Size:      10485760, // 10MB giả lập cố định!
          Status:    "Completed",
          CreatedAt: time.Now(),
      }
      u.snapshots[snapshot.ID] = snapshot // Nhét vào map in-memory!
      return snapshot, nil
  }
  ```
* **Căn nguyên**: Người dùng bấm nút "Tạo etcd snapshot", hệ thống trả về `10MB Completed`, nhưng trên thực tế **không có bất kỳ file snapshot nào được tạo ra trên máy chủ**! Khi cụm K8s gặp thảm họa mất Raft Quorum hoặc etcd sập, không có dữ liệu nào để khôi phục.

### Lỗi 2: Đếm Số Pod Rồi Báo Tạo Backup Cụm Thành Công (Fake Velero)
* **Vị trí**: `internal/usecase/dr/dr_usecase.go:184-206`
* **Đoạn code bị lỗi**:
  ```go
  // CODE BỊP BỢM:
  pods, _ := u.k8sClient.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
  // Đếm len(pods.Items) rồi in log: "Cluster backup completed with N pods"
  ```
* **Căn nguyên**: Thay vì tương tác với Velero API hoặc tạo CRD `backups.velero.io`, code chỉ đếm số pod đang chạy rồi báo hoàn tất.

---

## 2. ĐẶC TẢ STRUCTS & INTERFACES (CONTRACT DEFINITION)

Trong `internal/domain/dr/entity.go`:
```go
package dr

import "time"

type EtcdSnapshotResult struct {
    SnapshotID   string    `json:"snapshot_id"`
    ClusterID    string    `json:"cluster_id"`
    FilePath     string    `json:"file_path"`
    SizeBytes    int64     `json:"size_bytes"`
    SHA256Hash   string    `json:"sha256_hash"`
    TotalKeys    int64     `json:"total_keys"`
    RaftTerm     uint64    `json:"raft_term"`
    RaftIndex    uint64    `json:"raft_index"`
    CreatedAt    time.Time `json:"created_at"`
}

type RestoreDrillReport struct {
    DrillID         string        `json:"drill_id"`
    SnapshotID      string        `json:"snapshot_id"`
    TargetClusterID string        `json:"target_cluster_id"`
    Status          string        `json:"status"` // PASSED, FAILED, TIMED_OUT
    RealRTODuration time.Duration `json:"real_rto_duration"` // Thời gian khôi phục thực tế
    RealRPOLag      time.Duration `json:"real_rpo_lag"`      // Độ lệch dữ liệu
    TablesRestored  int           `json:"tables_restored"`
    RowsRestored    int64         `json:"rows_restored"`
    ExecutedAt      time.Time     `json:"executed_at"`
    LogOutput       string        `json:"log_output"`
}

type EtcdClientDriver interface {
    SaveSnapshot(ctx context.Context, endpoint, caPath, certPath, keyPath string) (*EtcdSnapshotResult, error)
    VerifySnapshot(ctx context.Context, snapshotPath string) (bool, error)
}
```

---

## 3. THUẬT TOÁN XỬ LÝ TỪNG BƯỚC (STEP-BY-STEP WORKFLOW)

### Bước 1: Triển Khai Driver `etcdctl` Thật
1. Kết nối với etcd endpoint qua TLS chứng chỉ:
   ```bash
   ETCDCTL_API=3 etcdctl --endpoints=$ENDPOINTS --cacert=$CA --cert=$CERT --key=$KEY snapshot save $DEST
   ```
2. Đọc file snapshot vừa tạo, tính mã băm `SHA256(snapshot)` và kiểm tra tính toàn vẹn:
   ```bash
   ETCDCTL_API=3 etcdctl snapshot status $DEST --write-out=json
   ```
3. Lưu bản ghi snapshot vào cơ sở dữ liệu `etcd_snapshots` (lưu SHA-256 hash và dung lượng thực tế).
4. Đẩy snapshot lên Secondary Storage (S3 / MinIO WORM storage).

### Bước 2: Tích Hợp Velero CRDs Thực Sự
1. Định nghĩa dynamic client cho Group `velero.io`, Version `v1`, Resource `backups`.
2. Tạo Custom Resource `Backup`:
   ```yaml
   apiVersion: velero.io/v1
   kind: Backup
   metadata:
     name: cluster-backup-{timestamp}
     namespace: velero
   spec:
     includedNamespaces: ["*"]
     storageLocation: default
     volumeSnapshotLocations: ["default"]
   ```
3. Theo dõi status `.status.phase == "Completed"`.

### Bước 3: Diễn Tập Khôi Phục Định Kỳ Trong Sandbox Tạm (Restore Drill)
1. Hàng tuần (Cronjob), hệ thống tự động:
   - Khởi tạo namespace tạm thời: `restore-test-ephemeral-{id}`.
   - Kéo bản backup database mới nhất từ S3 về.
   - Khởi chạy một container DB tạm thời và restore dữ liệu.
   - Đo thời gian hoàn thành $\implies$ Ghi nhận **RTO Thực Tế (Recovery Time Objective)**.
   - Chạy các query đối soát dữ liệu (so sánh max timestamp với thời điểm backup) $\implies$ Ghi nhận **RPO Thực Tế (Recovery Point Objective)**.
   - Đánh dấu `StatusVerified` vào lịch sử backup.
   - Xóa bỏ namespace tạm thời để giải phóng tài nguyên.

---

## 4. CHUẨN HÓA MÃ LỖI (RFC 7807 ERROR SPECIFICATION)

* `ERR_ETCD_SNAPSHOT_FAILED`: `HTTP 500 Internal Server Error` — "etcdctl snapshot save failed: Connection refused or invalid certificates."
* `ERR_RESTORE_DRILL_INTEGRITY_MISMATCH`: `HTTP 422 Unprocessable Entity` — "Restore drill failed: Snapshot checksum mismatch or missing tables during test restore."

---

## 5. MA TRẬN TEST CASE (VERIFICATION SUITE)

1. `TestEtcd_RealSnapshot_Execution`: Giả lập etcd mock server. Thực thi snapshot save. Assert file được tạo trên đĩa, dung lượng $> 0$, và SHA-256 hash được tính đúng.
2. `TestVelero_CRD_Creation`: Gọi `TriggerClusterBackup`. Assert K8s dynamic client nhận được đối tượng Velero Backup CRD với đầy đủ spec.
3. `TestRestoreDrill_MeasuresRTO`: Chạy quy trình restore drill. Assert kết quả trả về `RealRTODuration > 0` và status `PASSED`.

---

## 6. KẾ HOẠCH ROLLBACK & GUARDRAILS AN TOÀN

* **Guardrail**: File snapshot etcd và cluster backup bắt buộc phải được mã hóa AES-256-GCM trước khi truyền qua mạng công cộng.
