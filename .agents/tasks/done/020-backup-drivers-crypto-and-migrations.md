# Task 020 — Backup Drivers, Cryptographic AEAD & Migration Hardening (P1)
# Checkpoint: 2026-10-12T10:30:00+07:00

> **Severity**: P1 (Data Integrity, Backup Cryptography, Cross-Platform Driver Reliability & Zero-Downtime Migration)  
> **Target Agent**: `backend-coder` / `database-engineer`  
> **Workspace**: Git Worktree Isolated (`Workspace: "branch"`)  
> **Design Rules**: Authenticated Encryption (AEAD), Cross-Platform File I/O, Safe DDL Locking  

---

## 1. PHÂN TÍCH LỖI & BẰNG CHỨNG MÃ NGUỒN (ROOT CAUSE ANALYSIS)

### Lỗi 1: Sử Dụng Raw AES-CTR Thiếu Xác Thực Toàn Vẹn Trong Backup Stream
* **Vị trí**: `internal/infrastructure/backup/dualsync/pipe.go:61`
* **Đoạn code bị lỗi**:
  ```go
  // CODE HIỆN TẠI (MÃ HÓA KHÔNG AN TOÀN):
  stream := cipher.NewCTR(block, iv)
  writer := &cipher.StreamWriter{S: stream, W: targetWriter}
  ```
* **Căn nguyên**: AES-CTR là stream cipher thuần túy, không có tính năng MAC (Message Authentication Code). Kẻ tấn công hoặc lỗi lật bit trên đĩa (bit-rot) có thể thay đổi dữ liệu nhị phân mà quá trình giải mã không hề phát hiện, dẫn đến việc khôi phục cơ sở dữ liệu bị hỏng hoàn toàn (Data Corruption). Đặc tả cam kết chuẩn **AES-256-GCM** (AEAD) nhưng code thực tế lại dùng raw CTR.

### Lỗi 2: Redis Driver Chết Trên Windows & Dùng Lệnh `--pipe` Sai Trầm Trọng
* **Vị trí**: `internal/infrastructure/backup/drivers/redis.go:53, 95`
* **Đoạn code bị lỗi**:
  ```go
  // Backup: Lỗi đường dẫn Linux cứng trên Windows:
  cmd := exec.Command("redis-cli", "--rdb", "/dev/stdout")
  
  // Restore: Dùng --pipe trên file nhị phân RDB (LỖI 100%):
  cmd := exec.Command("redis-cli", "--pipe") // --pipe chỉ nhận giao thức RESP văn bản!
  ```
* **Căn nguyên**: Trên Windows, `/dev/stdout` không tồn tại, khiến lệnh backup sập ngay lập tức. Khi restore, `redis-cli --pipe` được thiết kế chỉ để nhận lệnh text theo chuẩn Redis Protocol (RESP); nếu bơm file nhị phân `dump.rdb` vào, Redis trả về hàng ngàn lỗi protocol parsing và từ chối 100% dữ liệu.

### Lỗi 3: S3 Driver Gặp Lỗi HTTP 411 Length Required Trên AWS S3
* **Vị trí**: `internal/infrastructure/backup/storage/s3.go:139`
* **Căn nguyên**: Khi thực hiện stream upload bằng `http.NewRequestWithContext("PUT", url, reader)`, driver không thiết lập trường `ContentLength` hoặc `GetBody`. AWS S3 và Cloudflare R2 bắt buộc phải có `Content-Length` cho các request PUT trực tiếp, nếu không sẽ lập tức từ chối với lỗi `HTTP 411 Length Required`.

### Lỗi 4: Migration Thiếu Tracking Table, Advisory Lock & Lock Timeout
* **Vị trí**: `scripts/migrate/main.go:101`
* **Căn nguyên**: Khi nhiều container server cùng khởi động, chúng chạy migration đồng thời mà không có khóa phân tán (Distributed Lock) hoặc Advisory Lock, dẫn đến lỗi xung đột DDL (`relation already exists` hoặc `deadlock detected`). Đồng thời không có `SET lock_timeout = '2s'`, khiến lệnh DDL có thể khóa chết bảng production.

---

## 2. ĐẶC TẢ STRUCTS & INTERFACES (CONTRACT DEFINITION)

### Chunked AEAD Encryption Pipeline:
Trong `internal/infrastructure/backup/dualsync/crypto_gcm.go`:
```go
package dualsync

import (
    "crypto/aes"
    "crypto/cipher"
    "io"
)

// ChunkSize 64KB cho AEAD chunked streaming
const GCMChunkSize = 64 * 1024

type GCMStreamWriter struct {
    w     io.Writer
    aead  cipher.AEAD
    nonce []byte
    buf   []byte
}

func NewGCMStreamWriter(w io.Writer, key []byte) (*GCMStreamWriter, error) {
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, err
    }
    aead, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }
    // Ghi Salt và Nonce Header trước khi stream dữ liệu
    ...
}
```

---

## 3. THUẬT TOÁN XỬ LÝ TỪNG BƯỚC (STEP-BY-STEP WORKFLOW)

### Bước 1: Chuẩn Hóa Chuỗi Mã Hóa AES-256-GCM
1. Dữ liệu từ stream nén ZSTD đi vào `GCMStreamWriter`.
2. Đóng gói dữ liệu thành các block 64KB.
3. Mỗi block được tính toán Authentication Tag bằng `aead.Seal(nil, nonce, chunk, nil)`.
4. Khi giải mã (`GCMStreamReader`), mỗi block bắt buộc phải xác thực Tag bằng `aead.Open(...)`. Nếu phát hiện bit-rot hoặc bị can thiệp, dừng stream ngay lập tức và báo lỗi `ErrCorruptedBackupData`.

### Bước 2: Viết Lại Quy Trình Backup/Restore Redis Chuẩn Kỹ Thuật
1. **Backup**:
   - Tạo thư mục tạm an toàn độc lập hệ điều hành: `tempFile := filepath.Join(os.TempDir(), fmt.Sprintf("redis-backup-%d.rdb", time.Now().UnixNano()))`.
   - Chạy lệnh: `redis-cli --rdb tempFile`.
   - Đọc stream từ file tạm này đẩy vào pipeline DualSync. Sau khi hoàn tất, xóa file tạm bằng `os.Remove(tempFile)`.
2. **Restore**:
   - Dừng Redis instance tạm thời hoặc sử dụng lệnh cứu hộ: copy file RDB đã giải mã vào thư mục data của Redis (`/data/dump.rdb`).
   - Khởi động lại Redis instance để nạp toàn vẹn memory từ file `dump.rdb`.

### Bước 3: Khắc Phục Lỗi S3 HTTP 411
1. Chuyển đổi sang `aws-sdk-go-v2/service/s3` hoặc `s3manager.Uploader` (tự động phân đoạn Multipart Upload có Content-Length chuẩn xác cho stream).

### Bước 4: Khóa An Toàn Cho Migration
1. Trước khi thực thi migration, bắt buộc gọi:
   ```sql
   SELECT pg_advisory_lock(7421893);
   ```
2. Tạo bảng quản lý phiên bản nếu chưa có:
   ```sql
   CREATE TABLE IF NOT EXISTS schema_migrations (
       version BIGINT PRIMARY KEY,
       applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
       dirty BOOLEAN NOT NULL DEFAULT FALSE
   );
   ```
3. Chèn lệnh timeout vào đầu mỗi session migration:
   ```sql
   SET lock_timeout = '2000ms';
   SET statement_timeout = '30000ms';
   ```
4. Giải phóng khóa sau khi hoàn tất: `SELECT pg_advisory_unlock(7421893)`.

---

## 4. CHUẨN HÓA MÃ LỖI (RFC 7807 ERROR SPECIFICATION)

* `ERR_BACKUP_INTEGRITY_COMPROMISED`: `HTTP 422 Unprocessable Entity` — "Backup data integrity check failed: GCM authentication tag mismatch."
* `ERR_MIGRATION_LOCK_TIMEOUT`: `HTTP 500 Internal Server Error` — "Migration aborted: Lock timeout exceeded after 2000ms. Concurrent transaction holding lock."

---

## 5. MA TRẬN TEST CASE (VERIFICATION SUITE)

1. `TestGCM_EncryptionDecryption_Integrity`: Stream 10MB dữ liệu qua GCM Writer rồi đọc lại qua GCM Reader. Assert dữ liệu giải mã khớp 100% SHA-256 ban đầu.
2. `TestGCM_BitRotDetection`: Lật 1 bit ngẫu nhiên trong ciphertext. Assert GCM Reader trả về lỗi `cipher: message authentication failed`.
3. `TestRedis_CrossPlatform_BackupRestore`: Chạy test backup và restore Redis trên môi trường local. Assert Redis khôi phục đầy đủ 1,000 keys đã seed.
4. `TestMigration_AdvisoryLock_ConcurrentInstances`: Chạy 5 goroutine migration cùng lúc. Assert chỉ có 1 goroutine chạy tại một thời điểm, không có lỗi race condition hoặc dirty state.

---

## 6. KẾ HOẠCH ROLLBACK & GUARDRAILS AN TOÀN

* **Guardrail**: Không bao giờ áp dụng migration mới nếu cờ `dirty == true`. Bắt buộc SRE can thiệp thủ công nếu migration trước đó bị đứt gánh.
