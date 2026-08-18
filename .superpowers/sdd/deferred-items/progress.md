# SDD Ledger — Deferred Items Remediation

## Domains (all disjoint — parallel dispatch valid)

### Domain 1: Backup System Hardening
Files: `internal/infrastructure/backup/drivers/`, `internal/infrastructure/backup/engine.go`, `internal/infrastructure/backup/storage/s3.go`
Items:
- Fake DB driver fallback stubs (postgres/redis/oracle/sqlserver/mariadb/mongodb dump 2-line comments)
- S3 storage missing AWS SigV4 request signing
- Credential leaks in CLI args (oracle/sqlserver/mongodb passwords visible in `ps aux`)
- Backup engine OOM (bytes.Buffer in-memory → io.Pipe streaming)
- Hardcoded `VerificationStatus = "verified"` without actual checksum verification

### Domain 2: Security & Auth Hardening
Files: `internal/infrastructure/telegram/bot.go`, `internal/adapter/http/change_handler.go`, `internal/adapter/http/promotion_handler.go`, `internal/usecase/cluster/import_usecase.go`, `internal/usecase/cluster/health_checker.go`
Items:
- Telegram bot open auth when `adminMap` empty (anyone can restart/rollback)
- Fake rollback action (just restarts pod, ignores errors)
- Approver identity spoofing via URL query params (`?approver=...`)
- Plaintext kubeconfig stored as `EncryptedToken`

### Domain 3: Infrastructure Quality
Files: `internal/infrastructure/logging/aggregator.go`, `internal/infrastructure/kubernetes/capacity_repo.go`, `cmd/agent-runner/main.go`, `internal/adapter/http/fleet_handler.go`, `internal/adapter/http/audit_handler.go`
Items:
- Unbounded memory growth in log aggregator (no TTL eviction)
- Hardcoded capacity metrics (25%, 35% storage, formula-based CPU)
- agent-runner missing `/healthz` endpoint
- Simulated cluster upgrade in `fleet_handler.go:187-193`
- Fake audit run in `audit_handler.go:56-77`

## Progress
- Domain 1 (Backup): complete — Agent + USER manual fixes (postgres, redis, mongodb, oracle, sqlserver drivers hardened; LookPath checks; env var credentials; fallback stubs removed)
- Domain 2 (Security): complete — Agent + USER manual fixes (telegram bot deny-all on empty adminMap; rollback = not-implemented; approver from JWT context; kubeconfig encrypted)
- Domain 3 (Infrastructure): complete — Agent fixes (log aggregator TTL+LRU eviction; capacity_repo returns ErrMetricsUnavailable; agent-runner /healthz+/readyz; fleet upgrade 501; audit run pending status)
