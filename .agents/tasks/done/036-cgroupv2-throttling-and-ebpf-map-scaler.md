# Task 036 — Kernel cgroup v2 Throttling Interceptor & eBPF Map Auto-Scaler (P1)
# Checkpoint: 2026-10-12T10:45:00+07:00

> **Severity**: P1 (Silent Kernel Space Stalls, eBPF Packet Drops & Slab Memory Recovery)  
> **Target Agent**: `backend-coder` / `devops`  
> **Workspace**: Git Worktree Isolated (`Workspace: "branch"`)  
> **Design Rules**: cgroup v2 Direct Memory Event Trapping, eBPF Dynamic Map Scaling, VFS Cache Reclamation  

---

## 1. BỐI CẢNH KỸ THUẬT & PHÂN TÍCH LỖI THỰC TẾ (ROOT CAUSE ANALYSIS)

### Lỗi 1: Linux cgroup v2 `memory.high` Throttling Làm App Đứng Hình Bí Ẩn
* **Hiện tượng**: Trên K8s v1.25+ dùng cgroup v2, khi container chạm ngưỡng mềm `memory.high`, kernel ép các thread vào Direct Memory Reclaim và sleep phạt (Proportional Sleep Penalty). Không có bất kỳ cảnh báo OOM nào trong dmesg, nhưng ứng dụng bị delay 5 - 30 giây trong kernel space.
* **Cách xử lý triệt để**:
  1. `k8s-agent` định kỳ đọc file `/sys/fs/cgroup/system.slice/.../memory.events` trên từng container cgroup.
  2. Bắt các sự kiện `high` count tăng vọt. Khi `high > 10` trong 1 phút, tự động thông báo SRE: "Workload đang bị Kernel phạt sleep do chạm memory.high" và đề xuất tăng memory limit.
  3. Cung cấp cờ cấu hình Kubelet `memory.high.factor = 0` cho các cụm yêu cầu SLA độ trễ siêu thấp để tắt hoàn toàn cơ chế throttle mềm này (thay bằng OOM hoặc proactive autoscaling).

### Lỗi 2: eBPF Hash Map Exhaustion Âm Thầm Drop Socket Packets
* **Hiện tượng**: Bảng theo dõi kết nối eBPF của Cilium/Calico (`BPF_MAP_TYPE_HASH`) bị đầy do microservice mở hàng triệu TCP connection ngắn hạn. Kernel trả về `-E2BIG`, các gói tin mới bị drop âm thầm ở driver card mạng (`XDP_DROP`).
* **Cách xử lý triệt để**:
  1. Giám sát metric `cilium_bpf_map_pressure{map_name="cilium_ct4_global"}`.
  2. Khi áp lực $> 75\%$, tự động trigger cơ chế **eBPF Map Dynamic Resizing**:
     - Cập nhật Cilium ConfigMap tăng `bpf-ct-global-any-max` từ 262,144 lên 524,288 hoặc 1,048,576.
     - Giảm timeout của kết nối TCP đã hoàn tất (`bpf-ct-timeout-regular = 60s`).
     - Tự động nạp lại bản đồ eBPF mà không làm rớt các kết nối đang active.

### Lỗi 3: Linux Kernel Slab Dentry & Inode Cache Leak
* **Hiện tượng**: Hàng triệu file tạm trong `/tmp` bị xóa nhưng dentry/inode cache trong kernel slab memory chiếm sạch 80% RAM, khiến Kubelet hiểu nhầm là memory pressure và evict nhầm pod quan trọng.
* **Cách xử lý triệt để**:
  - Áp dụng profile sysctl `vm.vfs_cache_pressure = 150` qua Ansible Day-1.
  - Tự động mount `tmpfs` (RAM disk có giới hạn dung lượng) cho thư mục `/tmp` của container thay vì dùng chung với rootfs của node.

---

## 2. ĐẶC TẢ STRUCTS & INTERFACES (CONTRACT DEFINITION)

Trong `internal/domain/nodemetrics/cgroup_ebpf.go`:
```go
package nodemetrics

import (
    "context"
    "time"
)

type CGroupV2MemoryEvent struct {
    ContainerID     string    `json:"container_id"`
    PodName         string    `json:"pod_name"`
    Namespace       string    `json:"namespace"`
    LowEvents       int64     `json:"low_events"`
    HighEvents      int64     `json:"high_events"`      // Số lần bị kernel phạt sleep
    MaxEvents       int64     `json:"max_events"`       // Số lần chạm trần OOM
    OOMKillEvents   int64     `json:"oom_kill_events"`
    ThrottledDuration time.Duration `json:"throttled_duration"`
    UpdatedAt       time.Time `json:"updated_at"`
}

type EBPFMapUtilization struct {
    MapName         string  `json:"map_name"`
    CurrentEntries  int64   `json:"current_entries"`
    MaxEntries      int64   `json:"max_entries"`
    PressureRatio   float64 `json:"pressure_ratio"` // Current / Max
    Status          string  `json:"status"`         // NORMAL, WARNING, CRITICAL
    RecommendedSize int64   `json:"recommended_size"`
}

type KernelInterceptorManager interface {
    AuditCGroupV2Events(ctx context.Context, nodeName string) ([]CGroupV2MemoryEvent, error)
    InspectEBPFMaps(ctx context.Context, clusterID string) ([]EBPFMapUtilization, error)
    AutoResizeEBPFConntrackMap(ctx context.Context, clusterID string, newCapacity int64) error
    EnforceNodeSlabTuning(ctx context.Context, nodeName string) error
}
```

---

## 3. THUẬT TOÁN XỬ LÝ TỪNG BƯỚC (STEP-BY-STEP WORKFLOW)

### Quy trình 1: Bắt Sự Kiện `memory.high` Trên cgroup v2
1. `k8s-agent` gắn inotify watcher vào file `/sys/fs/cgroup/kubepods.slice/.../memory.events`:
2. Khi file có sự thay đổi, đọc nội dung:
   ```
   high 142
   max 0
   oom 0
   oom_kill 0
   ```
3. Nếu giá trị `high` tăng đột biến $> 20$ events trong 30 giây:
   - Kích hoạt cảnh báo SRE Telegram:
     `⚠️ Pod %s/%s đang bị Kernel cgroup v2 throttle (High Events: %d). Latency đang bị kéo dài!`
   - Đưa ra đề xuất tăng memory limit thêm 25% qua 1-click button.

### Quy trình 2: Tự Động Scale Dung Lượng Bảng eBPF Conntrack
1. Agent đọc metric eBPF từ Cilium metrics endpoint:
2. Tính `PressureRatio = current_entries / max_entries`.
3. Nếu `PressureRatio > 0.80`:
   - Gửi cảnh báo P1 `ALERT_EBPF_MAP_SATURATION`.
   - Tính toán dung lượng mới: `newCapacity = max_entries * 2` (tối đa 2,097,152 entries).
   - Tự động patch Cilium ConfigMap và gửi tín hiệu hot-reload eBPF map.

---

## 4. CHUẨN HÓA MÃ LỖI (RFC 7807 ERROR SPECIFICATION)

* `ERR_CGROUPV2_MEMORY_HIGH_THROTTLED`: `HTTP 429 Too Many Requests` — "Container '%s' is experiencing kernel-level memory.high throttling."
* `ERR_EBPF_MAP_SATURATED`: `HTTP 503 Service Unavailable` — "eBPF map '%s' utilization exceeded critical threshold (%.2f%%). Silent packet drop imminent."

---

## 5. MA TRẬN TEST CASE (VERIFICATION SUITE)

1. `TestCGroupV2_ParseMemoryEvents`: Feed file text chứa `high 50`. Assert parser đọc đúng `HighEvents == 50`.
2. `TestEBPF_AutoResizeCalculation`: Feed map có 220,000 / 262,144 entries (pressure 83.9%). Assert hàm tính kích thước đề xuất đúng 524,288.
3. `TestSlabTuning_AppliesSysctl`: Kiểm tra hàm tuning áp dụng đúng `vm.vfs_cache_pressure = 150`.

---

## 6. KẾ HOẠCH ROLLBACK & GUARDRAILS AN TOÀN

* **Guardrail**: Khi resize eBPF map, kích thước mới không được vượt quá 10% tổng dung lượng RAM của máy chủ để tránh kernel memory exhaustion.
