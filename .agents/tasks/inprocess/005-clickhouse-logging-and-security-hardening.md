# 🎯 005: CLICKHOUSE HIGH-THROUGHPUT LOGGING ENGINE & COMPREHENSIVE SECURITY HARDENING

> **Location**: `.agents/tasks/inprocess/005-clickhouse-logging-and-security-hardening.md`  
> **Status**: IN PROCESS  
> **Standard**: Production Enterprise Grade (Craftsmanship > Speed, Zero Toy Projects)  
> **Active Branch**: `fix/comprehensive-audit`  
> **Memory Invariant**: Strict < 500MB RAM for ClickHouse, < 500 lines per file across all Go/Vue/TS.

---

## 🛡️ PHASE 1: HARNESS LAYER (Pre-Flight, Baselines & Vulnerability Models)
- [ ] **1.1 ClickHouse Infrastructure & Memory Constraint Harness**
  - File: `deployments/docker/docker-compose.clickhouse.yaml`
  - Hard constraints:
    - `mem_limit: 512m`, `mem_reservation: 256m`
    - `max_server_memory_usage: 450000000` (450MB hard limit)
    - `mark_cache_size: 67108864` (64MB sparse index cache)
    - Ports: Native TCP `9000`, HTTP `8123`
  - File: `migrations/clickhouse/001_cluster_logs.sql`
    - Table: `cluster_logs` with MergeTree engine
    - Column codecs: DoubleDelta + ZSTD(1) on `timestamp`, ZSTD(3) on `message`, ZSTD(1) on `attributes`
    - LowCardinality dictionary encoding: `tenant_id`, `cluster_id`, `namespace`, `pod_name`, `container_name`, `stream`, `log_level`
    - Token Bloom Filter: `INDEX idx_msg message TYPE tokenbf_v1(30720, 2, 0) GRANULARITY 1`
    - Partitioning: `PARTITION BY toYYYYMMDD(timestamp)`
    - Order: `ORDER BY (tenant_id, cluster_id, namespace, log_level, timestamp)`
    - Auto TTL: `TTL timestamp + INTERVAL 30 DAY DELETE`
- [ ] **1.2 Monolith Baseline Audit**
  - Record line count: `internal/domain/scaffold/builtin.go` (780 lines)
  - Record line count: `internal/usecase/ecosystem/detector_k8s.go` (502 lines)
  - Target: Shred all files to strictly < 400 lines each
- [ ] **1.3 Security Vulnerability Harness & Threat Modeling**
  - Model P0: VULN-001 (tenant leak in `tenant_query.go`), VULN-002 (K8s exec RCE in `router.go`), VULN-004 (audit log leak)
  - Model P1: VULN-005 (crypto keys in `crypto.go`), VULN-006 (path traversal in `storage_local.go`), VULN-003 & 011 (SSRF defense)
  - Model P2: VULN-007 (trace token leak), VULN-008 (DoS log allocation limit), VULN-009/010 (rate limiting)

---

## 🗺️ PHASE 2: GRAPH LAYER (Topological WBS & Dependency Ordering)
```
[Harness Verification]
         │
         ▼
[Step 1: ClickHouse DDL & Infra Config]
         │
         ▼
[Step 2: Go ClickHouse Native Driver & Domain Entity]
         │
         ▼
[Step 3: Zero-Allocation Batch Ingestion Buffer (5s / 5k logs)]
         │
         ▼
[Step 4: REST & WebSocket Log APIs (RBAC + Tenant Middleware)]
         │
         ▼
[Step 5: Frontend LogStreamView Modernization (Live Tail, Sparkline)]
         │
         ▼
[Step 6: Monolith Shredding (builtin.go & detector_k8s.go < 400 lines)]
         │
         ▼
[Step 7: P0 Security Remediation (VULN-001, VULN-002, VULN-004)]
         │
         ▼
[Step 8: P1 Security Remediation (VULN-005, VULN-006, VULN-003/011)]
         │
         ▼
[Step 9: P2 Security Remediation (VULN-007, VULN-008, VULN-009/010)]
         │
         ▼
[Phase 4: Release & Fleet Verification]
```

---

## 🔄 PHASE 3: LOOP LAYER (Implementation -> Adversarial Review -> Verification)

### Loop 1: ClickHouse Service & Storage Schema (< 500MB RAM)
- [ ] Task 1.1: Author `deployments/docker/docker-compose.clickhouse.yaml`
- [ ] Task 1.2: Author `migrations/clickhouse/001_cluster_logs.sql`
- [ ] Verification: Dry-run manifest, validate SQL syntax and partition/index rules

### Loop 2: Go Backend ClickHouse Infrastructure Driver
- [ ] Task 2.1: Add `github.com/ClickHouse/clickhouse-go/v2` to `go.mod`
- [ ] Task 2.2: Create `internal/domain/logging/entity.go` (< 250 lines)
- [ ] Task 2.3: Create `internal/infrastructure/clickhouse/client.go` (< 250 lines)
- [ ] Task 2.4: Create `internal/infrastructure/clickhouse/batch_writer.go` (< 300 lines)
- [ ] Task 2.5: Create `internal/infrastructure/clickhouse/log_repository.go` (< 350 lines)
- [ ] Task 2.6: Create `internal/usecase/logging/service.go` (< 300 lines)
- [ ] Verification: `go vet` and unit tests pass

### Loop 3: REST & WebSocket Log APIs
- [ ] Task 3.1: Create `internal/adapter/http/log_handler.go` (< 350 lines)
- [ ] Task 3.2: Register routes in `internal/adapter/http/router.go` with tenant context & RBAC
- [ ] Verification: Unit tests for handlers, ensure RBAC denies unauthenticated calls

### Loop 4: Frontend LogStreamView Modernization
- [ ] Task 4.1: Create `frontend-vue/src/composables/useLogStream.ts` (< 250 lines)
- [ ] Task 4.2: Refactor `frontend-vue/src/views/LogStreamView.vue` (< 450 lines)
- [ ] Verification: `npm.cmd run build` passes with zero errors

### Loop 5: Monolith Shredding (< 500 lines per file)
- [ ] Task 5.1: Shred `internal/domain/scaffold/builtin.go` (780 lines -> modular files < 400 lines)
- [ ] Task 5.2: Shred `internal/usecase/ecosystem/detector_k8s.go` (502 lines -> modular files < 400 lines)
- [ ] Verification: `powershell scripts\verify_quality_gate.ps1 -SkipBuild` confirms zero files > 500 lines

### Loop 6: Security Vulnerability Remediation (11 Findings)
- [ ] Task 6.1 (Batch 1 - P0 Critical):
  - VULN-001: Fix cross-tenant leak in `internal/infrastructure/postgres/tenant_query.go`
  - VULN-002: Wrap `/k8s/{cluster}/exec` in `internal/adapter/http/router.go` with RBAC
  - VULN-004: Apply mandatory tenant isolation filter in `internal/infrastructure/postgres/audit_query.go`
- [ ] Task 6.2 (Batch 2 - P1 High):
  - VULN-005: Eliminate fallback dev cryptographic keys in `pkg/crypto/crypto.go`
  - VULN-006: Sanitize local backup storage paths in `internal/infrastructure/storage/storage_local.go`
  - VULN-003 & VULN-011: SafeHTTPClient SSRF defense
- [ ] Task 6.3 (Batch 3 - P2 Medium):
  - VULN-007: Trace token redaction in audit logs
  - VULN-008: Payload size limit on log ingest (prevent DoS)
  - VULN-009 & VULN-010: Rate limiting & auth hardening
- [ ] Verification: Security regression suite passes

---

## 🚀 PHASE 4: RELEASE & FLEET
- [ ] Run full mechanical quality gate: `powershell -ExecutionPolicy Bypass -File scripts\verify_quality_gate.ps1 -SkipBuild`
- [ ] Run Go test suite: `go test ./...`
- [ ] Run Frontend production build: `npm.cmd run build`
- [ ] Verify ClickHouse runtime memory footprint: strictly < 500MB RAM
- [ ] Commit with Conventional Commits and sync remote branch: `git push origin fix/comprehensive-audit`
