# Task Plan: Nâng cấp `k8s-agent` Columnar LogEngine (ClickHouse Architecture, 1000 Nodes, <= 50MB RAM) & Xử lý triệt để 7 lỗi Live UI/Backend

> **Created**: 2026-10-15
> **Branch**: `fix/comprehensive-audit`
> **Status**: COMPLETED & VERIFIED VIA CHROME DEVTOOLS MCP

---

## EPIC 1: Nâng cấp `internal/pkg/logengine` & `cmd/agent` theo kiến trúc ClickHouse ($\le$ 50MB RAM / Node, Scale 1.000 Nodes)

### 1.1. Ép cứng Trần RAM $\le$ 45MB (Strict Memory Budget & Zero-Leak) — DONE (`481177f7`)
- [x] **`internal/pkg/logengine/arena.go`**:
  - Hạ `debug.SetMemoryLimit(38 * 1024 * 1024)` + `debug.SetGCPercent(25)`.
  - Benchmark thực tế 20.000 dòng log: `Alloc = 2.89 MB, Sys = 18.06 MB` (nén `28.75x`).
- [x] **`internal/pkg/logengine/block.go`**:
  - Giảm ZSTD encoder/decoder window xuống `128 * 1024` (`128KB`).
- [x] **`internal/pkg/logengine/ringbuffer.go` & `writer.go` & `reader.go`**:
  - Giảm `DefaultRingBufferCapacity` xuống `4096` slots (~384KB tĩnh).
  - Xóa tham chiếu `batch[i] = Entry{}` ngay sau khi copy vào `BlockBuilder`.
  - Tái sử dụng pooled buffer trong `reader.go` thay vì cấp phát Heap trên mỗi block.

### 1.2. Kiến trúc ClickHouse Multi-Block Segment Part + TTL Compaction (Xóa bỏ "Bom Inode") — DONE (`481177f7`)
- [x] **`internal/pkg/logengine/writer.go` & `index.go`**:
  - Mở **Active Segment Part** (`part_XXXXXXXXXXXXXXXX`) ghi nối tiếp (append) tối đa `256 blocks` (~524.288 dòng log hoặc `16MB` nén) vào chung 1 thư mục part.
  - Tách `Level` khỏi `LabelDictionary` (`codec.go`), đảm bảo `dict.Export()` chỉ trả về tên `Service` thuần túy.
- [x] **Retention & Disk Quota Pruner (`internal/pkg/logengine/retention.go`)**:
  - Tự động dọn các `part_*` cũ nhất khi vượt quá `MaxRetention` (72h) hoặc `MaxDiskBytes` (1GB).

### 1.3. ClickHouse Granule Bitmask Pruning & Late Materialization — DONE (`481177f7`)
- [x] **Granule Bitmask trong `BlockMark` (`internal/pkg/logengine/index.go`)**:
  - Bổ sung `LevelMask uint16` và `ServiceBloom uint64` vào `BlockMark` (`marks.bin`), skip 100% block không khớp trước khi đọc đĩa hay giải nén.
- [x] **Late Materialization (`internal/pkg/logengine/block.go` & `reader.go`)**:
  - Tách nén cột Header (`Timestamps + ServiceIDs + LevelIDs + MsgLengths`) và cột `Messages`; chỉ giải nén `Messages` khi có dòng khớp bộ lọc.

### 1.4. Sửa lỗi OOM Fallback trong `cmd/agent/log_search.go` & Bổ sung năng lực cho `k8s-agent` — DONE (`481177f7`)
- [x] **`cmd/agent/log_search.go`**:
  - Sửa lỗi OOM fallback khi truy vấn Columnar Engine trả về 0 dòng khớp.
- [x] **Checkpointing Offset cho `PodTailer` (`cmd/agent/pod_tailer.go`)**:
  - Lưu trạng thái offset đọc file (`tail_offsets.json` trong `engineDir`) để restart `k8s-agent` không đọc trùng log cũ.
- [x] **Bổ sung Telemetry Chuyên sâu cho K8s Node (`cmd/agent/collector*.go`, `types.go`, `disk_linux.go`, `collector_k8s_psi.go`)**:
  - Bổ sung **Inode Metrics thật** (`InodesTotal`, `InodesUsed`, `InodesFree`, `InodesUsagePercent`) từ `syscall.Statfs`.
  - Bổ sung **Linux PSI** (`/proc/pressure/{cpu,memory,io}`).
  - Bổ sung **cgroup v2 & K8s Counters** (`PodCount`, `OOMKills`, `CPUThrottledPeriods`) + **Network Errors/Drops & Conntrack** (`nf_conntrack_count`, `nf_conntrack_max`).
- [x] **Auto-Registration & Heartbeat Push (`cmd/agent/registrar.go`, `cmd/agent/main.go`)**:
  - Thêm `--control-plane-url` (`CONTROL_PLANE_URL`) và `--node-ip` tự động đăng ký & gửi heartbeat định kỳ về `POST /api/v1/agents/register`.

### 1.5. Distributed Scatter-Gather Query Coordinator cho 1.000 Nodes (Control Plane) — DONE (`620681a5`)
- [x] **`internal/infrastructure/logging/distributed_search.go` & `internal/adapter/http/agent_register_handler.go`**:
  - Bounded Scatter-Gather Coordinator (64 workers, HTTP Keep-Alive pool, timeout 1.5s/node) + Streaming Min-Heap Top-K Merge.

---

## EPIC 2: Xử lý triệt để 7 lỗi Runtime / UI / Seed Data từ ảnh chụp Mobile 375px

- [x] **Lỗi 1 (`DESKTOP-GKK38T1` giả làm Worker Node & sinh Incident `NodeNotReady`)** — DONE (`620681a5`)
- [x] **Lỗi 2 (Dữ liệu mẫu/giả trong SQL Migrations & Backend Seed)** — DONE (`620681a5`, `8c2d9d42`)
- [x] **Lỗi 3 (WebSocket `/logs` bị kẹt `RECONNECTING` + chữ `60 FPS` & `Live stream is active` mâu thuẫn)** — DONE (`620681a5`, `b1c68ad7`, `8c2d9d42`)
- [x] **Lỗi 4 (Màn hình `/slo` lặp chữ `1h (1h)`, tiêu đề `Touch-Optimized SLO Workload Stream`)** — DONE (`b1c68ad7`)
- [x] **Lỗi 5 (Dải chữ Neon Micro-Telemetry trùng lặp & Toast `Cluster Health Warning` đè lên nút bấm)** — DONE (`b1c68ad7`, `5625c8b5`)
- [x] **Lỗi 6 (Màn hình `/settings` chữ quá to/dài trên mobile & Banner `Install App [PWA]` chiếm diện tích)** — DONE (`b1c68ad7`)
- [x] **Lỗi 7 (Kiểm tra lại bằng `chrome-devtools-mcp` & Orchestrator trực tiếp đọc từng file `.png`)** — DONE (`docs/qa-evidence/2026-10-15/forensic-audit-e2e-round3/`)
