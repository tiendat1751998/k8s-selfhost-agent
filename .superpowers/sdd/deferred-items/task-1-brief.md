# Task 1: Backup System Hardening

## Goal
Fix all fake/stub implementations in the backup subsystem to make them production-ready.

## Files to Modify (ONLY these)
- `internal/infrastructure/backup/drivers/postgres.go`
- `internal/infrastructure/backup/drivers/mysql.go`
- `internal/infrastructure/backup/drivers/mariadb.go`
- `internal/infrastructure/backup/drivers/redis.go`
- `internal/infrastructure/backup/drivers/oracle.go`
- `internal/infrastructure/backup/drivers/sqlserver.go`
- `internal/infrastructure/backup/drivers/mongodb.go`
- `internal/infrastructure/backup/drivers/sqlite.go`
- `internal/infrastructure/backup/engine.go`
- `internal/infrastructure/backup/storage/s3.go`

## Specific Fixes Required

### 1. Driver Fallback Stubs → Real Errors
Each driver has a fallback function that writes a 2-line SQL comment header and returns success when the dump binary (pg_dump, mysqldump, mongodump, etc.) is missing.

**Pattern to find in each driver:**
```go
// When binary not found, writes "-- K8S-SELFHOST AGENT BACKUP..." header and returns nil
```

**Required fix:** Use `exec.LookPath("binary_name")` to pre-check binary availability. If missing, return a typed error `ErrBinaryNotFound` with the binary name. DO NOT write fake comment headers.

### 2. Redis Driver — Fake Dump/Restore
- `redis.go:50-64`: `Dump()` triggers `BgSave()` then writes only a comment string. Must actually download the RDB file via `DEBUG OBJECT` or copy from the configured RDB path.
- `redis.go:67-79`: `Restore()` only pings, ignores `reader`. Must use `RESTORE` command or pipe RDB data.

### 3. S3 Storage — Missing AWS SigV4
- `storage/s3.go:48-135`: Raw HTTP requests without AWS Signature V4. Credentials stored in fields but never used for signing.
- **Fix:** Use `aws-sdk-go-v2` (check go.mod first) or implement minimal SigV4 signing. If adding a dependency, state why.

### 4. Credential Leaks in CLI Args
- `oracle.go:34,42,100`: Password in `sqlplus` CLI arg string `%s/%s@%s`
- `sqlserver.go:38,64,113`: Password in `-P opts.Password` CLI arg
- `mongodb.go:29,42,81`: Password in connection URI via CLI args
- **Fix:** Use environment variables (`PGPASSWORD`, `MYSQL_PWD`, `ORACLE_PWD`) or pipe credentials via stdin where supported.

### 5. Backup Engine OOM
- `engine.go:94-102`: Streams dumps into `bytes.Buffer` (in-memory). Large databases cause OOM.
- **Fix:** Use `io.Pipe()` for streaming, or temp files with cleanup.

### 6. Fake Verification
- `engine.go:176-177`: Hardcodes `VerificationStatus = "verified"` without checking.
- `engine.go:283`: Hardcodes fake restore verification log.
- **Fix:** Compute SHA-256 checksum of the backup data and compare on restore. Set `VerificationStatus` based on actual checksum match.

## Acceptance Criteria
1. No driver returns `nil` error when its binary is missing
2. No driver writes fake comment headers as "backup data"
3. S3 operations use signed requests (or return clear error if credentials missing)
4. No passwords visible in `exec.Command` arguments
5. `engine.go` streams via `io.Pipe()` or temp files, not `bytes.Buffer`
6. Verification status reflects actual checksum comparison
7. `go vet ./internal/infrastructure/backup/...` passes
8. `go test ./internal/infrastructure/backup/...` passes

## Verify Command
```
go vet ./internal/infrastructure/backup/...
go test ./internal/infrastructure/backup/... -v -count=1
```
