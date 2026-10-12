# Task 029 — Bare-Metal Air-Gapped Hardware & Time Drift Watchdog (P1)
# Checkpoint: 2026-10-12T10:35:00+07:00

> **Severity**: P1 (Air-Gapped Time Synchronization, Hardware SMART Early Warning & OOM Daemon Protection)  
> **Target Agent**: `devops` / `backend-coder`  
> **Workspace**: Git Worktree Isolated (`Workspace: "branch"`)  
> **Design Rules**: Low-Level Linux Syscall Inspection, Disk Health Predictive Failure, Kernel OOM Shielding  

---

## 1. BỐI CẢNH KỸ THUẬT & PHÂN TÍCH LỖI THỰC TẾ (ROOT CAUSE ANALYSIS)

### Lỗi 1: Lệch Pha Đồng Hồ Hệ Thống (NTP Time Drift) Trong Mạng Cách Ly (Air-Gapped)
* **Hiện tượng thực tế**: Môi trường On-Premises Air-Gapped không có kết nối ra Internet để sync với `pool.ntp.org`. Các server vật lý sử dụng bộ dao động tinh thể phần cứng (hardware crystal clock) bị trôi giờ theo nhiệt độ. Sau vài tuần, đồng hồ giữa Master 1 và Worker 2 lệch nhau 1.5 giây.
* **Hậu quả thảm khốc**:
  1. **etcd rớt Quorum liên tục**: etcd leader election phụ thuộc vào heartbeat timestamp. Lệch đồng hồ khiến các node nghi ngờ leader đã chết, kích hoạt bầu cử liên miên làm cụm K8s tê liệt.
  2. **Gãy toàn bộ TLS Certificates**: TLS cert mới tạo trên Master có `NotBefore` trong tương lai của Worker node $\implies$ Lỗi `certificate is not yet valid`.
  3. **Xác thực JWT & TOTP thất bại 100%**: Mọi token đăng nhập bị từ chối do lệch quá window cho phép.

### Lỗi 2: Ổ Cứng NVMe/SATA Suy Hao Ẩn (Silent Degradation & Bad Sectors)
* **Hiện tượng thực tế**: Ổ cứng SSD/NVMe máy chủ bị hỏng chip flash cục bộ. Write latency từ 0.5ms nhảy vọt lên 900ms do hardware controller liên tục retry. Tuy nhiên hệ điều hành chưa nhận được lỗi disk chết hẳn (drive crash) nên vẫn ghi dữ liệu vào.
* **Hậu quả**: Toàn bộ database write thread bị treo I/O wait, kéo sập toàn bộ các dịch vụ phụ thuộc mà không có cảnh báo rõ ràng.

### Lỗi 3: Kernel OOM Killer Giết Nhầm Kubelet / Containerd
* **Căn nguyên**: Worker node thiếu cấu hình cgroup isolation giữa `system.slice` và `kubepods.slice`. Điểm `oom_score_adj` của Kubelet để mặc định (0). Khi RAM chạm đáy, Linux Kernel SIGKILL ngay `kubelet` hoặc `containerd`.

---

## 2. ĐẶC TẢ STRUCTS & INTERFACES (CONTRACT DEFINITION)

Trong `internal/domain/nodemetrics/hardware.go`:
```go
package nodemetrics

import (
    "context"
    "time"
)

type TimeDriftStatus struct {
    NodeName       string        `json:"node_name"`
    OffsetDuration time.Duration `json:"offset_duration"` // Độ lệch so với Master/NTP source
    IsSynchronized bool          `json:"is_synchronized"`
    NTPServer      string        `json:"ntp_server"`
    Status         string        `json:"status"`          // HEALTHY, WARNING, CRITICAL
    UpdatedAt      time.Time     `json:"updated_at"`
}

type DiskHealthReport struct {
    DevicePath      string    `json:"device_path"`      // /dev/nvme0n1, /dev/sda
    Model           string    `json:"model"`
    Serial          string    `json:"serial"`
    TemperatureC    int       `json:"temperature_c"`
    BadSectorsCount int64     `json:"bad_sectors_count"` // Reallocated sectors
    MediaErrors     int64     `json:"media_errors"`      // NVMe Media & Data Integrity Errors
    PowerOnHours    int64     `json:"power_on_hours"`
    HealthStatus    string    `json:"health_status"`     // PASSED, FAILING, DEGRADED
}

type HardwareWatchdog interface {
    GetClusterTimeDrift(ctx context.Context, clusterID string) ([]TimeDriftStatus, error)
    InspectNodeDisks(ctx context.Context, nodeName string) ([]DiskHealthReport, error)
    VerifyOOMProtection(ctx context.Context, nodeName string) (bool, error)
}
```

---

## 3. THUẬT TOÁN XỬ LÝ TỪNG BƯỚC (STEP-BY-STEP WORKFLOW)

### Quy trình 1: Giám Sát Time Drift Bằng Linux Syscall `adjtimex`
1. `k8s-agent` chạy trực tiếp trên từng worker node:
   - Gọi Linux system call `unix.Adjtimex(&buf)` hoặc đọc file `/sys/devices/system/clocksource/`.
   - Tính toán `offset = time.Duration(buf.Offset) * time.Microsecond`.
2. Đánh giá ngưỡng:
   - Nếu $|offset| > 100ms$: Cảnh báo mức `WARNING`.
   - Nếu $|offset| > 500ms$: Cảnh báo mức `CRITICAL-P1` qua Telegram:
     `🚨 Node %s bị lệch giờ %s so với chuẩn! Nguy cơ rớt etcd và lỗi TLS.`
   - Tự động gọi Ansible playbook resync Chrony daemon: `systemctl restart chronyd`.

### Quy trình 2: Quét Chỉ Số S.M.A.R.T Của Ổ Cứng
1. Định kỳ 6 giờ một lần, agent thực thi lệnh kiểm tra:
   ```bash
   smartctl -H -A -j /dev/nvme0n1
   ```
2. Phân tích JSON output:
   - Nếu `smart_status.passed == false`: Báo động đỏ thay thế ổ đĩa ngay lập tức.
   - Đối với NVMe: Kiểm tra `nvme_smart_health_information_log.media_errors > 0`.
   - Đối với SATA: Kiểm tra `Reallocated_Sector_Ct > 10` hoặc `Current_Pending_Sector > 0`.
3. Khi phát hiện ổ đĩa suy hao: Cảnh báo SRE để chủ động dời pod (Evict & Drain node) TRƯỚC KHI ổ đĩa chết hẳn.

### Quy trình 3: Khóa Điểm Bảo Vệ OOM Shielding Cho Kubelet
1. Agent kiểm tra file `/proc/$(pidof kubelet)/oom_score_adj`.
2. Nếu giá trị $\neq -999$:
   - Tự động ghi `-999` vào file (báo cho Linux Kernel biết: **Tuyệt đối không bao giờ được kill tiến trình này dù hệ thống có cạn kiệt RAM 100%**).
   - Tương tự gán `-999` cho `containerd` và `dockerd`.

---

## 4. CHUẨN HÓA MÃ LỖI (RFC 7807 ERROR SPECIFICATION)

* `ERR_TIME_DRIFT_EXCEEDED`: `HTTP 500 Internal Server Error` — "Node '%s' time drift exceeded critical threshold (%s > 500ms). Resync required."
* `ERR_SMART_DEGRADATION_DETECTED`: `HTTP 422 Unprocessable Entity` — "Disk '%s' on node '%s' is degraded (Bad sectors: %d, Media errors: %d)."

---

## 5. MA TRẬN TEST CASE (VERIFICATION SUITE)

1. `TestTimeDrift_DetectsHighOffset`: Mock trả về offset 650ms. Assert hàm đánh giá gán status `CRITICAL` và kích hoạt alert.
2. `TestSMART_DetectsFailingDisk`: Feed file JSON output của smartctl chứa 45 reallocated sectors. Assert hàm trả về `HealthStatus == "DEGRADED"`.
3. `TestOOM_VerifyScore`: Kiểm tra hàm kiểm tra OOM score trả về đúng `-999` cho protected daemons.

---

## 6. KẾ HOẠCH ROLLBACK & GUARDRAILS AN TOÀN

* **Guardrail**: Khi chạy `smartctl`, luôn chạy với quyền read-only, tuyệt đối không chạy các lệnh self-test nặng (`-t long`) trong giờ cao điểm làm nghẽn I/O của ứng dụng.
