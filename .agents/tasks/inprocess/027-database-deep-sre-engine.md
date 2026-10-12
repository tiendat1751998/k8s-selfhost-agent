# Task 027 — Database Deep Operations & Anti-Bloat SRE Engine (P1)
# Checkpoint: 2026-10-12T10:35:00+07:00

> **Severity**: P1 (Database Production Performance, Connection Pool Starvation, Table Bloat & Memory Defrag)  
> **Target Agent**: `database-engineer` / `backend-coder`  
> **Workspace**: Git Worktree Isolated (`Workspace: "branch"`)  
> **Design Rules**: Non-Blocking DDL, Autonomous Deadlock Interception, Table Bloat Early Warning  

---

## 1. BỐI CẢNH KỸ THUẬT & PHÂN TÍCH LỖI THỰC TẾ (ROOT CAUSE ANALYSIS)

### Lỗi 1: `idle_in_transaction` Cạn Kiệt Connection & Chặn Đứng `VACUUM` (Gây Phình To Bảng)
* **Hiện tượng thực tế**: Developer viết code gọi `BEGIN`, thực thi một vài câu SELECT/UPDATE, sau đó thực hiện một tác vụ HTTP API mất 15-30 giây trước khi `COMMIT`.
* **Hậu quả thảm khốc**:
  1. Connection bị giữ ở trạng thái `idle in transaction`, chiếm dụng slot trong connection pool (HikariCP, pgxpool). Khi traffic tăng, pool chạm ngưỡng `max_connections` (500/500) $\implies$ Toàn bộ ứng dụng bị tê liệt với lỗi `FATAL: remaining connection slots are reserved for non-replication superuser connections`.
  2. Transaction này giữ `xmin horizon` của PostgreSQL, **chặn đứng hoàn toàn tiến trình `VACUUM` thu dọn các dead tuples**. Hàng triệu dòng dữ liệu cũ bị cập nhật không thể dọn dẹp, khiến bảng từ 500MB phình to thành 20GB (Table Bloat), tàn phá I/O đĩa và làm chậm toàn bộ hệ thống.

### Lỗi 2: Read Replica Replication Lag Bị Hủy Truy Vấn Do Xung Đột (Conflict Cancellation)
* **Hiện tượng thực tế**: Standby replica đang chạy các câu truy vấn báo cáo hoặc phân tích dữ liệu kéo dài. Master gửi bản tin WAL dọn dẹp các tuple cũ mà query trên replica đang đọc.
* **Hậu quả**: Replica buộc phải hủy ngang truy vấn của người dùng:
  ```
  ERROR: canceling statement due to conflict with recovery
  DETAIL: User query might have needed to see row versions that must be removed.
  ```

### Lỗi 3: Redis Memory Fragmentation Ratio Tăng Đột Biến (> 1.5) Gây OOM Crash
* **Hiện tượng thực tế**: Redis ghi và xóa liên tục các key có kích thước biến động lớn. Trình phân bổ bộ nhớ `jemalloc` bị phân mảnh nặng. Dữ liệu thực tế chỉ 1.5GB nhưng RSS Memory của tiến trình chiếm tới 5.8GB.
* **Hậu quả**: Linux Kernel OOM Killer thẳng tay gửi SIGKILL tiêu diệt tiến trình Redis của cụm cache/session.

---

## 2. ĐẶC TẢ STRUCTS & INTERFACES (CONTRACT DEFINITION)

Trong `internal/domain/database/sre_entity.go`:
```go
package database

import (
    "context"
    "time"
)

type IdleTransactionRecord struct {
    PID             int           `json:"pid"`
    Database        string        `json:"database"`
    Username        string        `json:"username"`
    ClientIP        string        `json:"client_ip"`
    ApplicationName string        `json:"application_name"`
    Duration        time.Duration `json:"duration"`
    StateChangeAge  time.Duration `json:"state_change_age"`
    CurrentQuery    string        `json:"current_query"`
}

type TableBloatReport struct {
    TableName       string  `json:"table_name"`
    TotalSizeBytes  int64   `json:"total_size_bytes"`
    BloatSizeBytes  int64   `json:"bloat_size_bytes"`
    BloatPercentage float64 `json:"bloat_percentage"`
    DeadTuples      int64   `json:"dead_tuples"`
    LiveTuples      int64   `json:"live_tuples"`
    RecommendedFix  string  `json:"recommended_fix"` // VACUUM FULL (locking), pg_repack (online)
}

type RedisFragmentationStatus struct {
    UsedMemoryBytes      int64   `json:"used_memory_bytes"`
    RSSMemoryBytes       int64   `json:"rss_memory_bytes"`
    FragmentationRatio   float64 `json:"fragmentation_ratio"` // RSS / Used
    ActiveDefragEnabled  bool    `json:"active_defrag_enabled"`
    DefragRunning        bool    `json:"defrag_running"`
}

type DatabaseSREManager interface {
    InspectIdleTransactions(ctx context.Context, clusterID, dbName string) ([]IdleTransactionRecord, error)
    TerminateIdleTransaction(ctx context.Context, clusterID, dbName string, pid int) error
    AnalyzeTableBloat(ctx context.Context, clusterID, dbName string) ([]TableBloatReport, error)
    GetRedisFragmentation(ctx context.Context, clusterID string) (*RedisFragmentationStatus, error)
    ToggleRedisActiveDefrag(ctx context.Context, clusterID string, enable bool) error
}
```

---

## 3. THUẬT TOÁN XỬ LÝ TỪNG BƯỚC (STEP-BY-STEP WORKFLOW)

### Quy trình 1: Tự Động Diệt Kết Nối `idle_in_transaction` Treo
1. Định kỳ 15 giây, engine thực hiện truy vấn trên PostgreSQL:
   ```sql
   SELECT pid, datname, usename, client_addr, application_name,
          NOW() - state_change AS idle_duration, query
   FROM pg_stat_activity
   WHERE state = 'idle in transaction'
     AND (NOW() - state_change) > INTERVAL '10 seconds';
   ```
2. Nếu phát hiện transaction bị treo $> 15s$:
   - Ghi nhận thông tin chi tiết vào audit log.
   - Tự động gọi `SELECT pg_terminate_backend(pid)` để ngắt kết nối an toàn, giải phóng lock và hạ nhiệt pool connection.
   - Gửi cảnh báo tức thời về Telegram:
     `🚨 Tự động ngắt kết nối [PID: %d] treo idle in transaction %s. App: %s, Client: %s.`

### Quy trình 2: Phân Tích & Cảnh Báo Phình To Bảng (Table Bloat Analyzer)
1. Hàng ngày vào lúc 02:00 sáng, chạy thuật toán tính bloat qua `pg_stat_user_tables`:
   ```sql
   SELECT relname, n_live_tup, n_dead_tup,
          ROUND(n_dead_tup * 100.0 / NULLIF(n_live_tup + n_dead_tup, 0), 2) AS bloat_ratio
   FROM pg_stat_user_tables
   WHERE n_dead_tup > 50000 AND (n_dead_tup * 100.0 / NULLIF(n_live_tup + n_dead_tup, 0)) > 20.0
   ORDER BY n_dead_tup DESC;
   ```
2. Nếu bảng có `bloat_ratio > 30%` và dung lượng $> 1GB$:
   - Cảnh báo SRE.
   - Đề xuất câu lệnh chạy `pg_repack` (Online Reorganization) không khóa bảng thay vì dùng `VACUUM FULL` (gây exclusive lock).

### Quy trình 3: Tự Động Kích Hoạt Active Defragmentation Trên Redis
1. Định kỳ 60 giây, kiểm tra `INFO memory` trên Redis.
2. Tính toán `FragmentationRatio = used_memory_rss / used_memory`.
3. Nếu `FragmentationRatio > 1.4` VÀ `used_memory_rss > 1GB`:
   - Nếu `active_defrag_enabled == false`:
     - Tự động gửi lệnh `CONFIG SET activedefrag yes`.
     - Cấu hình `active-defrag-ignore-bytes 100mb`, `active-defrag-threshold-lower 10`.
     - Log thông báo: `Kích hoạt tự động Redis active defragmentation (Ratio: %.2f)`.
4. Khi `FragmentationRatio` giảm về $< 1.15$:
   - Tự động gửi lệnh `CONFIG SET activedefrag no` để giải phóng CPU chu kỳ.

---

## 4. CHUẨN HÓA MÃ LỖI (RFC 7807 ERROR SPECIFICATION)

* `ERR_DB_IDLE_TRANSACTION_TERMINATED`: `HTTP 409 Conflict` — "Transaction terminated by SRE Watchdog: Idle in transaction exceeded threshold (10s)."
* `ERR_DB_TABLE_BLOAT_CRITICAL`: `HTTP 422 Unprocessable Entity` — "Table '%s' bloat critical (Bloat: %.2f%%, Dead Tuples: %d). Vacuum or pg_repack required."

---

## 5. MA TRẬN TEST CASE (VERIFICATION SUITE)

1. `TestPostgres_TerminateIdleTransaction`: Mở kết nối test, chạy `BEGIN` rồi sleep. Assert engine phát hiện sau 10s, gọi `pg_terminate_backend`, và kết nối bị ngắt với thông báo rõ ràng.
2. `TestPostgres_AnalyzeTableBloat_CalculatesCorrectly`: Tạo 100,000 rows và thực hiện UPDATE 80,000 rows mà không vacuum. Assert hàm tính bloat phát hiện `bloat_ratio > 40%`.
3. `TestRedis_ActiveDefrag_AutoToggle`: Giả lập Redis có fragmentation 1.6. Assert engine gửi lệnh `CONFIG SET activedefrag yes`. Khi fragmentation hạ xuống 1.1, assert gửi `CONFIG SET activedefrag no`.

---

## 6. KẾ HOẠCH ROLLBACK & GUARDRAILS AN TOÀN

* **Guardrail**: Khi kill transaction, chỉ kill các transaction có state `idle in transaction`. Tuyệt đối không kill các transaction đang có state `active` (đang xử lý dữ liệu thật) trừ khi có lệnh can thiệp khẩn cấp từ SRE.
