# Task: Cryptographic JWT Signature Generation & Verification
ID: TASK-REFACTOR-B03
Status: success

## Objective
Implement proper signature signing and verification for JWT tokens using standard HMAC-SHA256, replacing mock/fake base64-only JWT tokens.

## Requirements
- Edit `internal/adapter/http/auth_handler.go`: sign the generated token using HMAC-SHA256 with a secret key (loaded from config/env). Signature must be appended as the third segment of the JWT.
- Edit `internal/adapter/http/middleware/auth.go`: verify the JWT token signature cryptographically using HMAC-SHA256. Deny requests with invalid or altered signatures.
- Ensure standard library `crypto/hmac` and `crypto/sha256` are utilized to remain zero-dependency.

## Verification
- Code must compile with `go build ./...`
- Go unit and integration tests must pass.
- Verify request authentication fails when signature is invalid or altered.
