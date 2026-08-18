# TASK 001: Core Database Backup & Dual-Target Restore Worker Engine

> **Module**: 01 - Database Backup & Disaster Recovery  
> **Status**: IN_PROGRESS  
> **Assigned Subagent**: `backend-coder` / `database-engineer`  
> **Model**: `flash` (Gemini 3.7 Flash High)  
> **Workspace Mode**: `Workspace: "branch"` (Git worktree isolation)  

---

## 1. Goal
Xây dựng Engine thực thi sao lưu & khôi phục Database thực tế (PostgreSQL & MySQL & MongoDB) với cơ chế Dual-Target (đẩy đồng thời 1 bản Local Storage + 1 bản Cloud S3/MinIO), nén stream `zstd`, mã hóa, và Worker xử lý nền.

---

## 2. Context & File Scope
- Domain Entity: [`internal/domain/backup/entity.go`](file:///d:/project/k8sseflhost/internal/domain/backup/entity.go)
- Repository: [`internal/domain/backup/repository.go`](file:///d:/project/k8sseflhost/internal/domain/backup/repository.go)
- Usecase: [`internal/usecase/backup/usecase.go`](file:///d:/project/k8sseflhost/internal/usecase/backup/usecase.go)
- Postgres Storage: [`internal/infrastructure/postgres/backup_repo.go`](file:///d:/project/k8sseflhost/internal/infrastructure/postgres/backup_repo.go)
- New Package Scope: `internal/infrastructure/backup/` (Drivers & Storage clients)

---

## 3. Acceptance Criteria
- [ ] **Driver Interface**: Định nghĩa interface `DatabaseDriver` với `Dump(ctx, config) (io.ReadCloser, error)` và `Restore(ctx, config, stream io.Reader) error`.
- [ ] **PostgreSQL Driver**: Hiện thực hóa dump/restore cho PostgreSQL qua `pg_dump` stream / protocol với hỗ trợ nén `zstd`.
- [ ] **Dual-Target Streamer**: Cho phép stream dữ liệu đồng thời qua `io.MultiWriter` tới:
  1. Local Storage target (filesystem / NFS / onprem MinIO).
  2. Cloud S3 target (AWS S3, GCS, Cloudflare R2, Azure Blob).
- [ ] **Backup Worker**: Worker chạy nền xử lý `BackupJob` và `RestoreJob`, cập nhật trạng thái `running`, `completed`, `failed`, tính toán `size_bytes`, `duration_ms`, `checksum`.
- [ ] **Verification**: Toàn bộ unit tests trong `internal/infrastructure/backup/` phải PASS bằng lệnh `go test -v ./internal/infrastructure/backup/...`.

---

## 4. Constraints
- Tuân thủ nghiêm ngặt Clean Architecture (Không import `infrastructure` vào `domain`).
- Không sinh mã stub, mock giả tạo hoặc `TODO` rỗng.
- Tất cả lỗi phải được xử lý và ghi log chi tiết, không nuốt lỗi (No error swallowing).
