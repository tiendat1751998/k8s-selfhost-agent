# Master Remediation Plan: Enterprise Hardening & Mechanical Sympathy
**Checkpoint**: 2026-10-12T11:15:00+07:00  
**Baseline**: Git branch `fix/comprehensive-audit` (HEAD `99ae1ed`)  
**Binding Constitution**: `.agents/rules/anti-toy-and-zero-complacency.md` & `AGENTS.md` §00, §0, §1, §5  
**Audit Findings Register**: `.agents/audit/2026-10-12-findings.md` (>300 verified defects across 16 audit reports)  

---

## 1. Executive Summary & Root Cause Synthesis

The line-by-line audit across all 16 slices of the Go backend and Vue frontend uncovered systematic toy-project patterns, fake operational telemetry, security bypasses, and hardware-oblivious designs. 

### Core Root Causes:
1. **Pervasive Telemetry Deception & Fake Operational States**:
   - `useDeployments.ts` renders `FALLBACK_SAMPLE_WORKLOADS` (5 fake healthy services) when the cluster has zero workloads or when the API 500s. Corrupted test suites (`mobileTelemetryAndMenuCompaction.test.ts`, `mobileUiAndWorkloadsRemediation.test.ts`) assert using regex that this fake data exists.
   - `dr/dr_usecase.go` returns fake in-memory 10MB snapshots and logs "restored" without writing to etcd.
   - `storage/volume_usecase.go` returns fake volume snapshots without invoking CSI snapshotters.
   - `ZeroTrustDrawer.vue` simulates KMS key rotation with `setTimeout` and fake SHA-256 strings.
   - `useInfraHosts.ts` sleeps 600ms and returns "Reachable" without sending any network probe.
2. **Multi-Tenancy Collusion & Repository Whitelist Bypasses**:
   - `tenant_verification_test.go:102-119` explicitly whitelists 15 Postgres repositories to bypass tenant isolation tests.
   - IDOR vulnerabilities in `catalog_handler.go:209` and `compute_host_handler.go:170,201` allow cross-tenant inspection and modification of private keys and catalog items.
   - `WSHub.Broadcast` broadcasts all container and incident events globally without tenant filtering.
3. **Hardware Contention & High-Throughput Log Bottlenecks**:
   - `internal/pkg/logengine/writer.go:128-175`: Inverted Mutex Anti-Pattern. `Write()` holds `w.mu.Lock()` across disk I/O, zstd compression, and dictionary saves, blocking all concurrent container loggers.
   - Inability to ingest Kubernetes pod logs: DaemonSet agent only communicates with Docker socket (`cmd/agent/container_tailer.go:125`), finding 0 containers on containerd / CRI-O clusters.
   - 64KB scanner buffer overflow (`container_tailer.go:315`, `journal_tailer.go:126`): lines >64KB terminate log streams in an infinite 200ms reconnect loop.
4. **Destructive Operations Masquerading as Safe Simulations**:
   - `internal/infrastructure/backup/drivers/postgres.go:126` & `backup/engine.go:403-404`: Dry-run restore ignores the `DryRun` flag and passes `--clean`, dropping live production database tables during dry-run simulation!
   - `sre/node_remediation.go:84-93`: Remote cluster client resolution failure falls back to local production cluster, force-deleting pods (`GracePeriodSeconds: 0`) and killing CNI DaemonSets.
5. **Frontend Asset Explosion & Dead Subsystems**:
   - Cascading scoped `@import` in `OverviewView.vue` duplicates `overview.css` 12 times into production CSS, inflating the stylesheet to **643.76 kB**.
   - 22 composables (~7,100 lines), 33 CSS files (10,822 lines), and entire component domains (`src/components/backup/` 1,348 lines, `src/components/platform/`) are orphaned dead code with 0 imports.

---

## 2. Hardware Architecture & Mechanical Sympathy Specifications

Per the binding rule `.agents/rules/anti-toy-and-zero-complacency.md`, every high-throughput component must declare its mechanical trade-offs:

### 2.1 Lock-Free Ingest Ring Buffer (Go Disruptor Pattern)
- **Target Component**: `internal/pkg/logengine/ringbuffer.go`
- **Memory Allocation Profile**: Pre-allocated array of power-of-2 capacity (e.g., $2^{16} = 65,536$ slots). Zero runtime heap allocations (`0 B/op`, `0 allocs/op`) during log streaming.
- **Cache-Line Alignment (Mechanical Sympathy)**: Each sequence barrier and slot header is padded with 64 bytes (`[56]byte` or `[7]uint64` padding) to prevent **False Sharing** between CPU L1/L2 caches when multiple tailer goroutines produce concurrently across different CPU cores.
- **Wait Strategy & CPU Boundary**:
  - *Failure Mode*: A classic LMAX `BusySpinWaitStrategy` pins 100% of a CPU core in a tight loop. On Kubernetes nodes or lightweight VPS hosts where the DaemonSet is budgeted at 50m–100m CPU limits, busy-spinning causes immediate CPU throttling and kills the node.
  - *Mandatory Fix*: Implement a **Hybrid Yielding Wait Strategy**:
    1. Spin for 100 iterations on active burst.
    2. Call `runtime.Gosched()` for 50 iterations to yield to other goroutines.
    3. Sleep via `sync.Cond` / micro-timer when queue remains empty.
- **Backpressure & Ring Wrap-Around**:
  - If disk I/O flusher is slower than ingress rate: ring buffer detects sequence wrap-around `(producerSeq - consumerSeq >= capacity)`. It must increment an atomic `dropped_logs_counter` and trigger batch backpressure rather than corrupting memory or blocking producers indefinitely.

### 2.2 Direct Kubernetes CRI Pod Log Tailer
- **Target Component**: `cmd/agent/pod_tailer.go`
- **Mechanism**: Reads directly from host path `/var/log/pods/<namespace>_<pod>_<uid>/<container>/*.log`.
- **CRI Format Parsing**: Parses standard CRI timestamp, stream (`stdout`/`stderr`), flag (`F`/`P`), and log content. Reconstructs partial lines (`P`) into complete log entries before writing to ring buffer.
- **Buffer Expansion**: Replaces 64KB `bufio.Scanner` with a 1MB dynamic buffer or chunked `bufio.Reader` using `ReadLine` to safely handle oversized JSON lines and stack traces without stream termination.

---

## 3. Phased Execution Roadmap

```
PHASE 0: Test Sanity & Dead Code Purge
   ↓
PHASE 1: Emergency P0 Security & Multi-Tenancy Hardening
   ↓
PHASE 2: High-Throughput Log Engine & Mechanical Sympathy
   ↓
PHASE 3: DaemonSet K8s Permissions & Server Bootstrap Parity
   ↓
PHASE 4: Authentic Disaster Recovery, GitOps & SRE Orchestration
   ↓
PHASE 5: 100% Fake Data Eradication & Frontend Refactoring
   ↓
PHASE 6: CSS Deduplication, 4-Tier RWD & Vite Bundler Optimization
   ↓
PHASE 7: Empirical Benchmarks, FMEA Verification & Quality Gate
```

---

### Phase 0: Test Sanity & Dead Code Purge
**Goal**: Eradicate test-suite collusion that blocks fixes, and purge 100% dead code.

1. **Purge Test Suite Collusion**:
   - `frontend-vue/tests/mobileTelemetryAndMenuCompaction.test.ts:101-104`: Delete regex assertion mandating `FALLBACK_SAMPLE_WORKLOADS`.
   - `frontend-vue/tests/mobileUiAndWorkloadsRemediation.test.ts:79-83`: Delete regex assertion requiring fake fallback workloads.
   - `internal/infrastructure/postgres/tenant_verification_test.go:102-119`: Delete hardcoded whitelist of 15 repositories that bypass `tenant_id` validation.
2. **Purge Orphaned Composables (~7,100 LOC)**:
   - Delete 22 unused composables identified in Report F1 (`useAgentMesh.ts`, `useAIProviderHub.ts`, `useBackupRestore.ts`, `useCapacityForecast.ts`, `useChangesTimeline.ts`, `useCompliance.ts`, `useCostFinOps.ts`, `useDevSecOps.ts`, `useDockerSwarm.ts`, `useDriftDetection.ts`, `useEcosystem.ts`, `useEventsTimeline.ts`, `useFleetManagement.ts`, `useGenericPlatform.ts`, `usePlugins.ts`, `usePodLogs.ts`, `usePodTerminal.ts`, `usePromotions.ts`, `useReports.ts`, `useScaffolder.ts`, `useServiceCatalog.ts`, `useTimeSync.ts`).
3. **Purge Orphaned Components & Dead CSS (~12,000 LOC)**:
   - Delete `frontend-vue/src/components/backup/` (12 files, 1,348 LOC).
   - Delete `frontend-vue/src/components/platform/` (3 files, 365 LOC).
   - Delete 6 duplicate modals in `frontend-vue/src/components/explorer/` (588 LOC).
   - Delete `IncidentSimulateModal.vue` (exact duplicate of `IncidentSimulationModal.vue`).
   - Delete 33 dead CSS files identified in Report F4 (10,822 LOC).
   - Delete unused `@vueuse/core` from `frontend-vue/package.json`.
4. **Verification Gate 0**:
   - Run `npm.cmd run build` to verify 0 missing import errors.
   - Run `go test ./internal/infrastructure/postgres/...` to verify test harness runs cleanly.

---

### Phase 1: Emergency P0 Security & Multi-Tenancy Hardening
**Goal**: Eliminate catastrophic data loss risks, IDOR vulnerabilities, and security bypasses.

1. **Fix Catastrophic Dry-Run Table Drop**:
   - `internal/infrastructure/backup/drivers/postgres.go:126`: Respect `opts.DryRun`. Never execute `pg_restore --clean` or SQL drop statements during dry run.
   - `internal/infrastructure/backup/engine.go:403-404`: Ensure dry-run verification creates a temporary isolated database rather than targeting production tables.
2. **Fix Multi-Tenancy IDOR Vulnerabilities**:
   - `internal/adapter/http/catalog_handler.go:209`: Require `tenantID` filter in `GetByID` and `Update`.
   - `internal/adapter/http/compute_host_handler.go:170, 201`: Check tenant ownership before exposing private SSH keys or endpoints.
   - `internal/domain/tenancy/repository.go:6-8`: Add `tenantID` to `GetOrganizations` and `GetProjects`.
   - `internal/domain/automation/repository.go`: Add `TenantID` to `Rule` and `Execution` entities and repository queries.
   - `internal/domain/cloud/repository.go`: Add `tenantID` to `GetByID` and `Delete`.
   - `internal/domain/gitops/repository.go`: Add `tenantID` to `PullRequest` queries.
   - `internal/domain/promotion/repository.go`: Add `TenantID` to `Promotion` entity and mutations.
   - `internal/adapter/http/websocket.go:137-145`: Filter broadcast events strictly by `client.TenantID`.
3. **Eradicate Fake Security Passes & Insecure Fallbacks**:
   - `internal/infrastructure/security/trivy_scanner.go:53-56`: When Trivy binary is missing or fails, return `PassSecurityGate: false` and return error.
   - `internal/infrastructure/security/checkov_scanner.go:46-50`: Treat exit code 1 as security policy failure, not 100% clean pass.
   - `internal/pkg/crypto/crypto.go:143-147`: Remove hardcoded fallback encryption keys (`0123456789abcdef...`). Fail fast with error if key is missing.
   - `internal/domain/cluster/manager.go:128-130`: Reject insecure TLS connections; do not silently set `Insecure = true` when CA is absent.
   - `frontend-vue/src/router/index.ts:182-186`: Validate JWT expiration, signature, and user roles against `/api/v1/auth/me` on protected admin routes.
   - `frontend-vue/src/services/pluginLoader.ts:85-99`: Remove direct DOM `<script>` injection. Load plugins inside sandboxed `<iframe>` with CSP.
   - `frontend-vue/src/stores/logStore.ts:166-170`: Remove token from WebSocket GET query parameters; pass via `Sec-WebSocket-Protocol` header.
4. **Verification Gate 1**:
   - Run `go test -v ./internal/infrastructure/backup/...`
   - Run `go test -v ./internal/infrastructure/security/...`
   - Run `go test -v ./internal/adapter/http/...`

---

### Phase 2: High-Throughput Log Engine & Mechanical Sympathy
**Goal**: Implement production-grade, zero-allocation log ingestion with hardware sympathy.

1. **Lock-Free MPSC Ingest Ring Buffer**:
   - Create `internal/pkg/logengine/ringbuffer.go`:
     - Capacity: 65,536 slots (power of 2 for bitwise masking `seq & mask`).
     - Cache padding: 64-byte padding (`_ [56]byte`) on cursor, sequence, and slot structs.
     - Mechanics: Atomic `CompareAndSwapUint64` sequence claims for multiple producers; single consumer thread draining batches.
     - Wait strategy: Hybrid spin (100x) -> `runtime.Gosched()` (50x) -> `sync.Cond` wait.
     - Allocation target: `0 B/op`, `0 allocs/op` on hot path.
2. **Decouple Disk Flusher from Ingestion Mutex**:
   - Refactor `internal/pkg/logengine/writer.go`:
     - Remove synchronous `flushLocked()` from inside `w.mu.Lock()`.
     - Ingestion pushes to ring buffer; independent background flusher worker drains batches, compresses via zstd, and writes blocks to disk.
3. **Implement Direct Kubernetes CRI Pod Log Tailer**:
   - Create `cmd/agent/pod_tailer.go`:
     - Scan `/var/log/pods/*/*/*.log` directly to support containerd, CRI-O, and Docker runtimes.
     - Parse CRI log line format (`<timestamp> <stream> <flag> <content>`).
     - Buffer handling: Dynamic 1MB line buffer handling stack traces without stream abortion.
4. **Fix Agent Memory Buffer Bloat**:
   - `cmd/agent/log_reader.go:163`: Replace unbounded `bytes.Buffer` with streaming chunked reader.
   - `cmd/agent/main.go:385`: Remove 10-second `WriteTimeout` on SSE `/logs/tail` endpoint.
   - `cmd/agent/main.go:404-406`: Fix shutdown sequence: drain tailers and await waitgroups before calling `engineSrc.Close()`.
5. **Verification Gate 2**:
   - Run `go test -bench=BenchmarkRingBuffer -benchmem ./internal/pkg/logengine/...`
   - Verify `0 B/op` and `0 allocs/op` on ingest path.
   - Run `go test -v ./cmd/agent/...`

---

### Phase 3: DaemonSet K8s Permissions & Server Bootstrap Parity
**Goal**: Fix Kubernetes deployment crashes and unify server/standalone bootstrap wiring.

1. **Fix DaemonSet Manifest & Binary Path**:
   - `deploy/k8s/agent-daemonset.yaml:75`: Change command from `["/app/k8s-agent"]` to `["/agent"]`.
   - `Dockerfile.agent:39` & `deploy/k8s/agent-daemonset.yaml:151`: Grant UID 65532 write permissions to `/var/lib/k8s-agent` via initContainer `chown` or run with proper security context.
   - Mount `/var/log/pods` as hostPath read-only into agent container.
2. **Restore 9 Missing Subsystems in Server Bootstrap**:
   - `cmd/server/bootstrap_services.go:288-324`: Wire all 9 missing domains matching `standalone_wiring.go`:
     - `K8sBootstrap`
     - `Storage`
     - `DR`
     - `Settings`
     - `Catalog`
     - `Scaffolder`
     - `Plugin`
     - `Ecosystem`
     - `Helm`
3. **Wire Distributed Log Repo in Server**:
   - `cmd/server/bootstrap_logging.go:66-67`: Wire `DistributedAgentLogRepo` with `ComputeHostRepo` instead of falling back to in-memory only.
4. **Fix Backup Entity Validation**:
   - `internal/domain/backup/entity.go:138-140`: Enforce `if result.Passed && result.ChecksumOK`.
5. **Fix Cluster Bootstrap Taint Check**:
   - `internal/domain/cluster/bootstrap.go:84-88`: Ignore standard `control-plane:NoSchedule` taints on master nodes when computing worker readiness.
6. **Verification Gate 3**:
   - Run `go build ./cmd/server` and `go build ./cmd/agent`
   - Validate manifests with `kubectl dry-run`: `kubectl apply --dry-run=client -f deploy/k8s/agent-daemonset.yaml`

---

### Phase 4: Authentic Disaster Recovery, GitOps & SRE Orchestration
**Goal**: Replace 100% simulated DR and mock GitOps with genuine infrastructure operations.

1. **Real Etcd Disaster Recovery**:
   - `internal/usecase/dr/dr_usecase.go:93-104, 139-143`:
     - Integrate real `etcd/client/v3` snapshot APIs or invoke `etcdctl snapshot save / restore`.
     - Output genuine snapshot file to disk/S3; return actual error if etcd is unreachable.
2. **Real Kubernetes Volume Snapshots**:
   - `internal/usecase/storage/volume_usecase.go:281-286`:
     - Create authentic Kubernetes `VolumeSnapshot` CRD objects via dynamic client.
     - Poll CSI snapshotter until `readyToUse: true` or timeout.
3. **Real GitOps Pull Request Lifecycle**:
   - `internal/usecase/gitops/controller.go:274-335`:
     - Implement real provider API calls (`githubClient.MergePullRequest()`, `gitlabClient.AcceptMergeRequest()`).
     - Update local database records only after successful remote API completion.
4. **Safe SRE Node Remediation**:
   - `internal/usecase/sre/node_remediation.go:84-93, 157-199`:
     - Fail fast if remote cluster client resolution fails; never fall back to local cluster.
     - Replace force-delete (`GracePeriodSeconds: 0`) with Kubernetes Eviction API (`policy/v1.Eviction`), respecting PodDisruptionBudgets and skipping DaemonSet pods.
5. **Real Capacity & SLO Forecasts**:
   - `internal/usecase/capacity/forecaster.go:117-132`: Replace hardcoded 0.5% growth with linear regression over Prometheus historical data.
   - `internal/usecase/slo/snapshot_updater.go:107-112`: Return error state when `ComputeSLI` fails; fix inverted error budget math.
6. **Verification Gate 4**:
   - Run `go test -v ./internal/usecase/dr/...`
   - Run `go test -v ./internal/usecase/gitops/...`
   - Run `go test -v ./internal/usecase/sre/...`

---

### Phase 5: 100% Fake Data Eradication & Frontend Refactoring
**Goal**: Purge all mock data, simulated delays, and fake badges from the frontend.

1. **Purge Fake Workloads & Mock Networks**:
   - `frontend-vue/src/composables/useDeployments.ts`: Delete `FALLBACK_SAMPLE_WORKLOADS`. Render authentic empty state when cluster has 0 workloads.
   - `frontend-vue/src/composables/useInfraHosts.ts:234-242`: Remove 600ms `setTimeout` connection test. Connect to real `hostsApi.testEndpoint`.
   - `frontend-vue/src/composables/useAlertManager.ts:233-235`: Connect "Test Channel", "Silence", and "Resolve" to real Alertmanager REST APIs.
   - `frontend-vue/src/composables/useSLOMonitor.ts:204-214`: Remove synthetic percentile multipliers (`* 0.25`, `* 0.6`). Query Prometheus histogram quantiles or render `--`.
   - `frontend-vue/src/composables/useEcosystem.ts:304-307`: Remove `setTimeout(500)` latency measurement and fake webhook logs (`wh-1`, `wh-2`).
2. **Purge UI Simulation Theater & Developer Leaks**:
   - `frontend-vue/src/components/layout/ZeroTrustDrawer.vue`: Connect KMS rotation to real backend APIs; remove client-side `setTimeout` and fake SHA-256 hashes.
   - `frontend-vue/src/components/logs/LogTargetTree.vue:60-81`: Eradicate developer homelab hostnames (`k8smasterdeb`, `k8sworker`, `10.10.10.60`, `10.10.10.80`, `tiki_redis`). Fetch dynamic targets from `/api/v1/logs/targets`.
   - `frontend-vue/src/components/overview/modals/deepdive/TrafficEndpointsTable.vue:45`: Remove hardcoded fallback `'k8smaster'`.
   - `frontend-vue/src/components/overview/drawer/tabs/useNodeProcesses.ts:119`: Remove hardcoded `tiki_` regex prefix.
   - `frontend-vue/src/components/deployments/CanaryTrafficSection.vue:65, 70`: Bind passing probes and error budget to live rollout metrics; render `--` when unmeasured.
   - `frontend-vue/src/components/deployments/WorkloadLogsTab.vue:141`: Remove fabricated log line `Container PID is active and operational` on error.
   - `frontend-vue/src/components/deployments/ScaleVerticalSection.vue:75, 93`: Fix CPU syntax from `"2 Cores"` to `"2000m"`.
   - `frontend-vue/src/components/cost/NamespaceCostTable.vue:171-185`: Remove hardcoded 45%/35%/20% cost multipliers; bind to real OpenCost metrics.
   - `frontend-vue/src/components/promotions/PromotionDiffDrawer.vue:58-61`: Bind gate status to live CI/CD and vulnerability results.
   - `frontend-vue/src/components/settings/SettingsApiKeysTab.vue:21-25`: Connect API key creation and revocation to backend REST endpoints.
3. **Fix Mojibake & Encoding Artifacts**:
   - `frontend-vue/src/components/k8s/SecretViewer.vue` & `YamlEditorModal.vue`: Re-encode files to clean UTF-8; fix `atob` crash on binary TLS certificates using `TextDecoder`.
4. **Fix Hot-Loops & Virtualize Mobile Card Streams**:
   - `frontend-vue/src/components/logs/LogTargetTree.vue:247-267`: Pre-aggregate log counts into a reactive `Map<string, number>`.
   - Virtualize or paginate mobile card streams (`LogMobileStream.vue`, `HelmMobileCards.vue`, `HostsMobileCards.vue`, `IncidentsMobileCards.vue`, `OverviewMobileStream.vue`).
5. **Modularize Monolithic Views (<350 Lines)**:
   - Split 9 views exceeding 350 lines into sub-components (`HelmCatalogView`, `IncidentsView`, `DeploymentsView`, `AlertsView`, `OverviewView`, `LogStreamView`, `AutomationView`, `InfraHostsView`, `ExplorerView`).
6. **Verification Gate 5**:
   - Run `npm.cmd run type-check`
   - Run `npm.cmd run build`

---

### Phase 6: CSS Deduplication, 4-Tier RWD & Vite Bundler Optimization
**Goal**: Drastically reduce bundle size, eliminate duplicate CSS, and enforce Constitution §5 4-Tier RWD.

1. **Eliminate 643.76 kB Duplicate CSS in OverviewView**:
   - `frontend-vue/src/views/OverviewView.vue:417` & child components: Remove scoped `@import '../assets/styles/views/overview.css'`. Import modular stylesheets non-scoped or bundle granular component styles.
2. **Enforce Tier 4 (4K 3840px) Constraint**:
   - `frontend-vue/src/assets/styles/layout/app-shell.css`: Add `@media (min-width: 1921px) { .app-layout { max-width: 1920px; margin: 0 auto; } }`.
3. **Fix Mobile Touch Targets (<640px)**:
   - `app-shell.css`: Enforce `min-height: 44px; min-width: 44px;` and add `position: relative` to `.mobile-menu-btn` and `.mobile-search-btn` so touch expanders do not escape to `.top-hud`.
4. **Vite Bundler Optimization & Dynamic Component Splitting**:
   - `frontend-vue/vite.config.ts`: Configure `manualChunks` to split heavy libraries (`@xterm/xterm`, `qrcode`).
   - `frontend-vue/src/views/ExplorerView.vue`: Lazily load `PodTerminalDrawer` using `defineAsyncComponent`.
5. **Self-Host Google Fonts for Air-Gapped Compliance**:
   - Download Inter and JetBrains Mono fonts to `src/assets/fonts/`.
   - Remove external Google Font links from `index.html:8` and `style.css:1`.
6. **Verification Gate 6**:
   - Run `npm.cmd run build` and verify:
     - `OverviewView.css` size drops from **643.76 kB** to **< 50 kB**.
     - `ExplorerView.js` size drops from **444.49 kB** to **< 100 kB**.

---

### Phase 7: Empirical Benchmarks, Failure Modes & Quality Gate
**Goal**: Run end-to-end mechanical benchmarks and verify adherence to the Anti-Toy Constitution.

1. **Log Ingestion Soak & Micro-Benchmark**:
   - Measure lock-free ring buffer throughput: verify sustained >50,000 logs/sec with 0 heap allocations on hot path.
   - Run pprof CPU & memory profiles on agent and server to confirm absence of mutex contention and memory leaks.
2. **End-to-End Behavioral Verification**:
   - Verify empty cluster renders authentic empty state (0 fake workloads).
   - Verify host connection testing executes real HTTP/TCP probes.
   - Verify dry-run restore does not drop production database tables.
   - Verify multi-tenant isolation across all 15 repositories.
   - Verify modern K8s CRI pod logs are collected from `/var/log/pods`.
3. **Constitutional Quality Gate Evaluation**:
   - All files strictly < 500 lines (views < 350 lines, components < 300 lines).
   - Zero hardcoded mock data, zero simulated timer delays, zero fake percentiles.
   - All tests execute actual runtime behavior (0 `fs.readFileSync` string-regex tests).

---

## 4. Verification & Rollback Protocol

- **Branch Isolation**: All implementation work executes on Git branch `fix/comprehensive-audit` or modular sub-branches (`fix/phase-1-security`, `fix/phase-2-logengine`, etc.).
- **Subagent Delegation Rule**: The orchestrator dispatches specialized coder subagents (`backend-coder`, `frontend-coder`, `database-engineer`, `devops`) with `Workspace: "branch"`.
- **Pre-Commit Gate**: Every subagent must report verbatim build/test outputs before handing off. The orchestrator independently verifies git diffs before merging.
- **Rollback Invariant**: If any phase introduces unexpected regressions, revert immediately to the baseline commit (`HEAD 99ae1ed`) before re-attempting. Never stack fixes on broken state.
