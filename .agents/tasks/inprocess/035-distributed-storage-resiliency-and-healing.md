# Task 035 — Distributed Storage Resiliency & Healing Controller (Ceph / MinIO) (P1)
# Checkpoint: 2026-10-12T10:45:00+07:00

> **Severity**: P1 (Distributed Storage Health, RocksDB WAL Recovery & Adaptive Bit-Rot Healing)  
> **Target Agent**: `devops` / `backend-coder`  
> **Workspace**: Git Worktree Isolated (`Workspace: "branch"`)  
> **Design Rules**: Automated Storage Repair, Non-Disruptive Crawler Tuning, Anti-Wipe Safeguards  

---

## 1. BỐI CẢNH KỸ THUẬT & PHÂN TÍCH LỖI THỰC TẾ (ROOT CAUSE ANALYSIS)

### Lỗi 1: Ceph OSD Crashlooping do RocksDB WAL Corruption Sau Mất Điện Đột Ngột
* **Hiện tượng**: Khi server mất nguồn điện đột ngột hoặc kernel panic trong lúc ghi nặng, metadata block trong RocksDB WAL của Ceph BlueStore bị hỏng. Khi OSD bật lại, nó crash ngay tại `DB::Open()` với `Corruption: bad entry in WAL`. Thiếu quorum OSD khiến Ceph kẹt vĩnh viễn ở trạng thái `peering`, làm tê liệt toàn bộ Persistent Volumes của cụm K8s.
* **Cách xử lý triệt để**:
  1. Tự động cấu hình `bluestore_rocksdb_options` kích hoạt `wal_recovery_mode = 2` (PointInTimeRecovery): Khi gặp corrupted tail entry, RocksDB tự động bỏ qua block hỏng và khôi phục về điểm nhất quán liền trước thay vì panic crash.
  2. Xây dựng SRE Repair Job tự động: Khi phát hiện OSD crashlooping với log RocksDB corruption, tự động chạy container cứu hộ thực thi `ceph-bluestore-tool fsck` và `ceph-bluestore-tool repair` trực tiếp trên disk phân vùng.

### Lỗi 2: MinIO Erasure Coding Bit-Rot Healing Bị Stall Dưới Tải Ghi Nặng
* **Hiện tượng**: Trình quét bit-rot crawler của MinIO chạy ngầm để phát hiện và phục hồi data hỏng bit. Dưới áp lực ghi lớn từ client, scanner cạnh tranh I/O lock khiến cả luồng đọc ghi của client lẫn crawler đều bị nghẽn (stall), IOPS tụt về 0.
* **Cách xử lý triệt để**:
  - Triển khai **Adaptive Scanner Controller**:
    - Khi `minio_s3_requests_waiting_total > 50` hoặc client write throughput $> 80\%$ baseline: Tự động điều chỉnh biến môi trường runtime `MINIO_DRIVE_SCANNER_SPEED = slowest`.
    - Khi hệ thống bước vào khung giờ thấp điểm (off-peak 01:00 - 05:00 sáng): Tự động nâng `MINIO_DRIVE_SCANNER_SPEED = fastest` để dồn lực quét và vá dữ liệu bit-rot nhanh nhất.

### Lỗi 3: GitLab 2017 Anti-Wipe Safeguard (Chống Xóa Nhầm Primary Database)
* **Cách xử lý triệt để**:
  - Tuyệt đối cấm mọi script hoặc operator chạy `rm -rf` trên thư mục dữ liệu database.
  - Mọi thao tác resync replication bắt buộc phải chạy qua hàm kiểm định:
    ```sql
    SELECT pg_is_in_recovery();
    ```
    Nếu trả về `false` (đây là node Primary đang phục vụ dữ liệu thật) $\implies$ Khóa cứng tiến trình và từ chối 100% mọi lệnh ghi đè/xóa thư mục.

---

## 2. ĐẶC TẢ STRUCTS & INTERFACES (CONTRACT DEFINITION)

Trong `internal/domain/storage/resiliency.go`:
```go
package storage

import (
    "context"
    "time"
)

type CephOSDStatus struct {
    OSDID         int       `json:"osd_id"`
    Host          string    `json:"host"`
    Status        string    `json:"status"` // UP, DOWN, CRASHLOOPING
    RocksDBHealth string    `json:"rocksdb_health"` // OK, CORRUPTED, REPAIRING
    LastCrashLog  string    `json:"last_crash_log,omitempty"`
    UpdatedAt     time.Time `json:"updated_at"`
}

type MinIOScannerTuning struct {
    CurrentSpeed      string  `json:"current_speed"` // slowest, slow, default, fast, fastest
    ActiveClientIOPS  int64   `json:"active_client_iops"`
    HealQueueLength   int64   `json:"heal_queue_length"`
    RecommendedSpeed  string  `json:"recommended_speed"`
    AutoTunedAt       time.Time `json:"auto_tuned_at"`
}

type StorageResiliencyManager interface {
    AuditCephCluster(ctx context.Context, clusterID string) ([]CephOSDStatus, error)
    TriggerCephOSDRepair(ctx context.Context, clusterID string, osdID int) error
    AutoTuneMinIOScanner(ctx context.Context, clusterID string) (*MinIOScannerTuning, error)
    VerifySafeDatabaseTarget(ctx context.Context, dbConnString string) (bool, error)
}
```

---

## 3. THUẬT TOÁN XỬ LÝ TỪNG BƯỚC (STEP-BY-STEP WORKFLOW)

### Quy trình 1: Tự Động Phục Hồi Ceph OSD RocksDB Crashlooping
1. Agent giám sát Ceph cluster đọc output từ `ceph osd tree -f json`:
2. Nếu một OSD có trạng thái `down` và restart count $> 3$ trong 5 phút:
   - Đọc 100 dòng log cuối cùng trong `/var/log/ceph/ceph-osd.{id}.log`.
   - Nếu phát hiện chuỗi: `Corruption: bad entry in WAL` hoặc `RocksDB error: Corruption`:
     - Gửi cảnh báo P1 về Telegram: `🚨 OSD %d bị hỏng RocksDB WAL sau sự cố điện. Đang kích hoạt chế độ tự động sửa chữa.`
     - Khởi chạy ephemeral K8s job gắn đúng phân vùng disk của OSD:
       ```bash
       ceph-bluestore-tool repair --path /var/lib/ceph/osd/ceph-$OSD_ID
       ```
     - Sau khi repair thành công, khởi động lại OSD daemon và kiểm tra PG peering phục hồi.

### Quy trình 2: Adaptive MinIO Healing Scanner Tuner
1. Định kỳ 60 giây, kiểm tra metric Prometheus `minio_s3_requests_total` và `minio_drive_scanner_speed`:
2. Nếu lưu lượng ghi hiện tại $> 2000$ IOPS:
   - Tự động gọi API MinIO Admin: `mc admin config set myminio/ api drive_scanner_speed=slowest`.
   - `mc admin service restart myminio`.
3. Nếu lưu lượng ghi $< 100$ IOPS trong 30 phút:
   - Tự động nâng lên `fastest` để dọn sạch heal queue.

---

## 4. CHUẨN HÓA MÃ LỖI (RFC 7807 ERROR SPECIFICATION)

* `ERR_CEPH_OSD_ROCKSDB_CORRUPTED`: `HTTP 500 Internal Server Error` — "Ceph OSD %d failed with RocksDB corruption. Automated repair triggered."
* `ERR_DATABASE_PRIMARY_WIPE_BLOCKED`: `HTTP 403 Forbidden` — "Destructive command blocked by GitLab-2017 Safeguard: Target node is Primary (pg_is_in_recovery=false)."

---

## 5. MA TRẬN TEST CASE (VERIFICATION SUITE)

1. `TestCeph_DetectRocksDBLogCorruption`: Feed đoạn log chứa `RocksDB error: Corruption`. Assert hàm audit phát hiện đúng `RocksDBHealth == "CORRUPTED"`.
2. `TestMinIO_AdaptiveTuning_ThrottlesUnderLoad`: Mock tải client 5000 IOPS. Assert hàm tuner trả về `RecommendedSpeed == "slowest"`.
3. `TestGitLabSafeguard_BlocksPrimaryWipe`: Mock kết nối Postgres trả về `pg_is_in_recovery() == false`. Gọi hàm verify safe target. Assert hàm trả về lỗi từ chối và chặn đứng lệnh xóa.

---

## 6. KẾ HOẠCH ROLLBACK & GUARDRAILS AN TOÀN

* **Guardrail**: Khi chạy `ceph-bluestore-tool repair`, bắt buộc phải unmount và stop OSD daemon trước để tránh 2 tiến trình cùng mở file descriptor nhị phân của BlueStore.
