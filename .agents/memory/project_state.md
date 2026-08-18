# Project State — k8s-selfhost-agent

> Last updated: 2026-08-14T15:39:00+07:00
> Updated by: Orchestrator session `2aba5d84-0be1-4e98-817c-874156f54dc8`

## Architecture Overview

Self-hosted Kubernetes cluster management platform with AI-assisted incident RCA.

### Stack
- **Backend**: Go 1.23, Chi router, PostgreSQL, Redis, NATS JetStream
- **Frontend**: Vue 3 + TypeScript + Vite, Pinia stores
- **Infrastructure**: Docker (distroless), Kubernetes, Helm, RBAC
- **Observability**: Zap structured logging, OpenTelemetry, Prometheus metrics

## Completed Enterprise Hardening (Session 2aba5d84)

### Phase 1: Security Critical ✅
- Removed `test-token-for-unit-tests` auth backdoor from `auth.go`
- JWT middleware now validates `exp`, `iat`, `iss` claims
- `JWT_SECRET` env var required at startup (fail-fast)
- CORS switched from wildcard `*` to configurable `CORS_ALLOW_ORIGIN`
- `crypto.go` uses HKDF key derivation (not raw byte slicing)
- Prometheus metric cardinality fixed (normalized paths)

### Phase 2: Backend Quality ✅
- Removed `initializeWelcomeMessage` seed data injection
- Fake health broadcasts replaced with real `Ping()` calls
- Trivy/Checkov scanners return explicit errors when binaries missing (not fake clean)
- Terraform/Ansible runners error on missing binaries (not fake success)
- `stringutil.go` uses `[]rune()` for UTF-8 safety
- Standalone server reads `DOCKER_HOST` from env (no hardcoded IP)

### Phase 3: Frontend ✅
- Removed `demo-admin-token` fallback, added 401 interceptor
- Created `LoginView.vue` with real POST auth flow
- Router has `beforeEach` auth guards + 404 catch-all
- Removed all mock arrays: `defaultDrivers`, `baselineScans`, `initialLogs`, `defaultJobs`
- Fixed deceptive `catch` blocks (no more fake success alerts)
- All metric cards/badges wired to real Pinia store state
- Design system consolidated to dark glassmorphic (`style.css`)
- TypeScript strict mode enabled, `any` types removed
- Deleted boilerplate: `HelloWorld.vue`, `useApi.ts`, old `useAuth.ts`

### Phase 4: Infrastructure ✅
- Dockerfile: multi-stage (Node.js + Go + distroless), static healthcheck binary
- docker-compose: fixed healthcheck, added ENCRYPTION_KEY/JWT_SECRET, fixed migration mount
- K8s: serviceAccountName, RBAC (ClusterRole + Binding), resource limits
- Created `Makefile`, `service.yaml`, `rbac.yaml`
- `.gitignore` updated (*.exe, fix.py)
- `config.yaml` complete with all sections, consistent K8S_ prefix
- `go.mod` aligned to Go 1.23

## Deferred Items — NOW COMPLETE (Session 2aba5d84, Phase 2)

### Domain 1: Backup System ✅
- All DB drivers (postgres, redis, oracle, sqlserver, mongodb) use `exec.LookPath()` pre-checks
- Fake fallback stubs removed (no more 2-line comment headers pretending to be backups)
- Credentials moved from CLI args to env vars (PGPASSWORD, ORACLE_PWD, SQLCMDPASSWORD, MONGODB_URI)
- Redis driver rewritten to use redis-cli binary instead of go-redis in-process

### Domain 2: Security & Auth ✅
- Telegram bot denies all commands when adminMap empty (was: allow all)
- Rollback action returns "not implemented" error (was: fake restart)
- Approver identity from JWT context, not query params (was: spoofable)
- Kubeconfig encrypted at rest via crypto.Encrypt()

### Domain 3: Infrastructure Quality ✅
- Log aggregator: TTL pruning (1hr) + LRU eviction (max 1000 buffers) + cleanup goroutine
- Capacity repo: returns ErrMetricsUnavailable instead of hardcoded percentages
- Agent-runner: /healthz + /readyz endpoints on :8081
- Fleet upgrade: 501 Not Implemented (was: fake DB update)
- Audit run: creates "pending" status (was: fake "completed")

### Remaining Known Issues — NOW FIXED
- ✅ Backup engine uses `io.Pipe()` streaming (was: `bytes.Buffer` OOM risk)
- ✅ S3 storage has AWS SigV4 signing — stdlib-only impl (HMAC-SHA256, no aws-sdk-go-v2 dep)
- ✅ Backup verification: SHA-256 checksum re-download comparison (was: hardcoded "verified")

## Verification Evidence
- `go vet ./...` — PASS (exit 0)
- `go test ./internal/adapter/http/middleware/...` — PASS
- `npm run build` (frontend-vue) — PASS (0 errors, 66 modules, 523ms)
