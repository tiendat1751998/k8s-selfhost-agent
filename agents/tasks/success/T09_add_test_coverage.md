# TASK: Add Test Coverage for Critical Packages

## Priority: ⚪ LOW — Long-term reliability
## Status: PENDING
## Estimated Effort: 90 minutes

---

## Problem Description

Of 40+ Go packages, only 8 have test files. Critical security and infrastructure packages have zero tests.

## Current Test Coverage

| Package | Has Tests? | Priority |
|---|---|---|
| `internal/adapter/event` | ✅ watcher_test.go | — |
| `internal/adapter/http` | ✅ handler_test.go, docker_handler_test.go | — |
| `internal/adapter/http/middleware` | ✅ middleware_test.go | — |
| `internal/domain/gitops` | ✅ entity_test.go | — |
| `internal/domain/incident` | ✅ entity_test.go, status_test.go | — |
| `internal/domain/report` | ✅ entity_test.go | — |
| `internal/infrastructure/gitprovider/github` | ✅ provider_test.go | — |
| `internal/usecase/rca` | ✅ pipeline_test.go, worker_test.go | — |
| `internal/pkg/crypto` | ❌ **NO TESTS** | 🔴 HIGH |
| `internal/adapter/http/middleware/auth.go` | ❌ Only middleware_test.go (base middleware) | 🔴 HIGH |
| `internal/infrastructure/llm/circuit_breaker.go` | ❌ **NO TESTS** | 🟡 MED |
| `internal/infrastructure/postgres/*` | ❌ **NO TESTS** (18 files!) | 🟡 MED |
| `internal/usecase/ai/health_poller.go` | ❌ **NO TESTS** | 🟡 MED |

## Sub-Tasks

### 9.1 — Add tests for `crypto.go` (Encrypt/Decrypt)
- **New file**: `internal/pkg/crypto/crypto_test.go`
- **Test cases**:
  - `TestEncryptDecrypt` — round-trip: encrypt → decrypt → matches original
  - `TestEncryptEmpty` — empty string returns empty
  - `TestDecryptInvalid` — invalid hex string returns error
  - `TestDecryptTooShort` — ciphertext shorter than nonce returns error
  - `TestKeyConsistency` — same key encrypts/decrypts consistently

### 9.2 — Add tests for `auth.go` middleware
- **New file**: `internal/adapter/http/middleware/auth_test.go`
- **Test cases**:
  - `TestJWTAuth_MissingHeader` → 401
  - `TestJWTAuth_InvalidFormat` → 401 (not "Bearer xxx")
  - `TestJWTAuth_EmptyToken` → 401
  - `TestJWTAuth_ValidToken` → passes, context has UserID + UserRole
  - `TestRBAC_RequiredRole_HasRole` → passes
  - `TestRBAC_RequiredRole_MissingRole` → 403

### 9.3 — Add tests for `circuit_breaker.go`
- **New file**: `internal/infrastructure/llm/circuit_breaker_test.go`
- **Test cases**:
  - `TestCircuitBreaker_Closed` — requests pass through
  - `TestCircuitBreaker_OpensOnFailures` — after N failures, requests are rejected
  - `TestCircuitBreaker_HalfOpenAfterTimeout` — after cooldown, allows one test request
  - `TestCircuitBreaker_ClosesOnSuccess` — successful request in half-open → closes

### 9.4 — Add tests for `health_poller.go`
- **New file**: `internal/usecase/ai/health_poller_test.go`
- **Test cases**:
  - `TestHealthPoller_Start` — poller starts and runs initial poll
  - `TestHealthPoller_Stop` — clean shutdown
  - `TestHealthPoller_CallsOnStatusChange` — callback is invoked
  - `TestHealthPoller_BackoffResets` — interval resets after healthy poll (after Task 8.1 fix)

## Files to Create
- `internal/pkg/crypto/crypto_test.go`
- `internal/adapter/http/middleware/auth_test.go`
- `internal/infrastructure/llm/circuit_breaker_test.go`
- `internal/usecase/ai/health_poller_test.go`

## Verification
- `go test ./...` passes with all new tests
- `go test -cover ./internal/pkg/crypto/` shows > 80% coverage
- `go test -cover ./internal/adapter/http/middleware/` shows > 80% coverage
