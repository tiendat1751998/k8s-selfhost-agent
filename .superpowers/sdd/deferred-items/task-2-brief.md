# Task 2: Security & Auth Hardening

## Goal
Fix security bypasses and data protection issues in Telegram bot, change approval, and cluster import.

## Files to Modify (ONLY these)
- `internal/infrastructure/telegram/bot.go`
- `internal/adapter/http/change_handler.go`
- `internal/adapter/http/promotion_handler.go`
- `internal/usecase/cluster/import_usecase.go`
- `internal/usecase/cluster/health_checker.go`

## Specific Fixes Required

### 1. Telegram Bot Open Auth (bot.go:139-142)
```go
if len(b.adminMap) == 0 { return true }
```
When no admin IDs configured, ANY Telegram user can invoke cluster restarts, rollbacks, and database restores.

**Fix:** Invert the logic — if `adminMap` is empty, deny all commands and log a warning. Require explicit admin configuration.

### 2. Fake Rollback Action (bot.go:275-286)
Under `case "rollback"`, line 282 calls `RestartDeployment` (not rollback) and ignores errors with `_ = ...`. The response says "✅ Đã kích hoạt Rollback" but it only restarts the pod.

**Fix:** 
- Use actual `RollbackDeployment` if it exists, or return an honest error/message saying rollback is not yet implemented.
- Do NOT ignore errors with `_ =`. Capture and report them.

### 3. Approver Identity Spoofing (change_handler.go:90-92, promotion_handler.go:86-88)
Approver/rejecter identity comes from URL query param `?approver=...` or defaults to `"system"`. Anyone can approve changes by setting this query param.

**Fix:** Read the approver identity from the authenticated JWT context (`middleware.UserIDKey`), never from query params. The authenticated user IS the approver.

### 4. Plaintext Kubeconfig as "EncryptedToken" (import_usecase.go:59)
```go
EncryptedToken: string(kubeconfig)
```
Stores raw plaintext kubeconfig in a DB column named `EncryptedToken`.

**Fix:** Use the project's `crypto.Encrypt()` function (from `internal/pkg/crypto/crypto.go`) to actually encrypt the kubeconfig before storing. Decrypt in `health_checker.go:59` when reading.

## Acceptance Criteria
1. Empty `adminMap` → ALL commands denied (not allowed)
2. Rollback action either performs real rollback or returns explicit "not implemented" error
3. Approver identity comes from JWT context, not query params
4. Kubeconfig encrypted at rest using `crypto.Encrypt()`
5. `go vet` passes on all modified packages
6. `go test` passes on all modified packages

## Verify Command
```
go vet ./internal/infrastructure/telegram/... ./internal/adapter/http/... ./internal/usecase/cluster/...
go test ./internal/infrastructure/telegram/... ./internal/adapter/http/... ./internal/usecase/cluster/... -v -count=1
```
