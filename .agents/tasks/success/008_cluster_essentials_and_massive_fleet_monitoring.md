# TASK 008: Tri-Runtime Adaptive Hybrid Fleet Monitoring (Bare-Metal, Docker & K8s DaemonSet) & Cluster Essentials

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement a tri-runtime auto-adaptive fleet monitoring architecture where `k8s-agent` runs flexibly across all 3 deployment topologies:
1. **Local Bare-Metal (No K8s)**: Native systemd binary for dedicated DB/Linux hosts (e.g. `masterdb`, `workerdb1`).
2. **Docker / Docker Swarm**: Containerized agent (`--net=host --pid=host`) for Docker-managed hosts.
3. **Kubernetes Cluster**: Auto-scaling DaemonSet (`k8s-control-agent`) for multi-node K8s clusters.
Combined with 1-click bootstrap of Kubernetes cluster essentials (Metrics Server, Local StorageClass, Control-Plane Untaint) from K8sControl UI.

**Architecture:** 
- The single Go binary (`cmd/agent`) auto-detects its runtime environment (`runtime_environment: "bare-metal" | "docker" | "kubernetes"`):
  - If `KUBERNETES_SERVICE_HOST` is set $\rightarrow$ `kubernetes` (DaemonSet pod, downward API node name).
  - If `/.dockerenv` exists $\rightarrow$ `docker` (containerized host-monitoring).
  - Otherwise $\rightarrow$ `bare-metal` (native systemd binary directly on host OS).
- Regardless of runtime, it reads host `/proc` with zero virtualization lag, exposes port 9100, detects database processes (Postgres, MySQL, Mongo, Redis), and reports into K8sControl's Host Registry with the correct badge (`[ 🖥️ BARE-METAL ]`, `[ 🐳 DOCKER ]`, `[ ☸️ K8S ]`).

**Tech Stack:** Go 1.23, Linux /proc telemetry, systemd service, Docker container (`--net=host`), Kubernetes DaemonSet, Chi router, client-go dynamic client, Vue 3, TypeScript.

## Global Constraints
- All Vue files must stay <= 350 lines; styles separated into `src/assets/styles/views/` or sub-components.
- Backend must adhere to Clean Architecture: Domain -> Usecase -> HTTP Adapter -> Infrastructure.
- Zero fake/mock data: real metrics from Linux `/proc` and K8s `/apis/metrics.k8s.io/v1beta1` only.
- Strict Tam Quyền Phân Lập: Coder agents implement code; QA agents test; Orchestrator plans.
- Standalone bare-metal DB hosts MUST NEVER require Docker or Kubernetes to be monitored.

---

### Task 1: Auto-Adaptive Runtime Detection & DB Process Discovery in `k8s-agent`

**Files:**
- Modify: `cmd/agent/collector.go`
- Modify: `deploy-agent.sh`
- Test: `cmd/agent/collector_test.go`

**Interfaces:**
- Consumes: Linux `/proc`, OS environment variables (`KUBERNETES_SERVICE_HOST`, `/.dockerenv`)
- Produces: `RuntimeEnvironment` ("bare-metal" | "docker" | "kubernetes"), `HostRole` ("database" | "compute" | "k8s"), `DetectedServices` ("postgres", "redis", etc.) in `/metrics` JSON.

- [ ] **Step 1: Write Unit Test for Runtime & Role Detection**
```go
package main

import (
    "os"
    "testing"
)

func TestDetectRuntimeEnvironment(t *testing.T) {
    os.Unsetenv("KUBERNETES_SERVICE_HOST")
    runtime := detectRuntimeEnvironment()
    if runtime != "bare-metal" && runtime != "docker" {
        t.Fatalf("unexpected runtime: %s", runtime)
    }
}

func TestDetectHostRole_Database(t *testing.T) {
    processes := []ProcessInfo{
        {Name: "postgres", Cmdline: "/usr/lib/postgresql/16/bin/postgres"},
    }
    role := detectHostRole(processes)
    if role != "database" {
        t.Fatalf("expected role database, got %s", role)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**
Run: `go test -v ./cmd/agent/ -run TestDetect`
Expected: FAIL (functions not defined)

- [ ] **Step 3: Implement Runtime & Role Detection in `collector.go`**
1. Add `detectRuntimeEnvironment()`:
   - If `os.Getenv("KUBERNETES_SERVICE_HOST") != ""` $\rightarrow$ return `"kubernetes"`.
   - If file `/.dockerenv` exists $\rightarrow$ return `"docker"`.
   - Else $\rightarrow$ return `"bare-metal"`.
2. Add `detectHostRole(processes []ProcessInfo)`:
   - Match `postgres`, `mysqld`, `mariadbd`, `mongod`, `redis-server`, `clickhouse` $\rightarrow$ return `"database"`.
   - Else match docker/containerd $\rightarrow$ return `"compute"` or `"k8s"`.
3. Enrich `SystemMetrics` struct with `RuntimeEnvironment`, `HostRole`, `DetectedServices`.

- [ ] **Step 4: Update `deploy-agent.sh` for Local Bare-Metal DB Hosts**
Ensure `./deploy-agent.sh user@ip` compiles the native binary, deploys to `~/k8s-agent`, configures `systemd --user` with linger, and checks port 9100.

- [ ] **Step 5: Run test to verify it passes**
Run: `go test -v ./cmd/agent/ -run TestDetect`
Expected: PASS

- [ ] **Step 6: Commit**
```bash
git add cmd/agent/ deploy-agent.sh
git commit -m "feat(agent): add auto-adaptive runtime detection and db process discovery"
```

---

### Task 2: Package Docker & Kubernetes Deployment Manifests

**Files:**
- Create: `deploy/docker/docker-compose.agent.yaml`
- Create: `deploy/k8s/agent-daemonset.yaml`
- Test: Syntax verification

**Interfaces:**
- Produces: 
  - Docker Compose manifest for Docker hosts (`--net=host --pid=host`).
  - Kubernetes DaemonSet manifest for K8s clusters.

- [ ] **Step 1: Write Docker Compose Manifest**
Create `deploy/docker/docker-compose.agent.yaml`:
```yaml
version: '3.8'
services:
  k8s-agent:
    image: k8s-control-agent:latest
    container_name: k8s-control-agent
    restart: always
    network_mode: host
    pid: host
    volumes:
      - /proc:/proc:ro
      - /sys:/sys:ro
      - /var/log:/var/log:ro
    environment:
      - AGENT_PORT=9100
```

- [ ] **Step 2: Write Kubernetes DaemonSet Manifest**
Create `deploy/k8s/agent-daemonset.yaml` with `hostNetwork: true`, `hostPID: true`, tolerations for all taints, downward API for NodeName, exposing port 9100.

- [ ] **Step 3: Validate Manifests**
Run: `powershell -Command "Test-Path deploy/docker/docker-compose.agent.yaml; Test-Path deploy/k8s/agent-daemonset.yaml"`
Expected: True, True

- [ ] **Step 4: Commit**
```bash
git add deploy/docker/ deploy/k8s/
git commit -m "feat(deploy): provide docker compose and k8s daemonset agent manifests"
```

---

### Task 3: Standardize Self-Host Essentials Manifests (Metrics Server & Storage)

**Files:**
- Create: `deploy/k8s/manifests/essentials/01-metrics-server.yaml`
- Create: `deploy/k8s/manifests/essentials/02-local-path-storage.yaml`

**Interfaces:**
- Produces: Ready-to-apply manifests for `metrics-server` (`--kubelet-insecure-tls`) and `local-path-provisioner` (default StorageClass).

- [ ] **Step 1: Create Metrics Server Manifest with Insecure TLS**
Create `deploy/k8s/manifests/essentials/01-metrics-server.yaml` with `--kubelet-insecure-tls` flag and control-plane tolerations.

- [ ] **Step 2: Create Local-Path StorageClass Manifest**
Create `deploy/k8s/manifests/essentials/02-local-path-storage.yaml` with `storageclass.kubernetes.io/is-default-class: "true"`.

- [ ] **Step 3: Commit**
```bash
git add deploy/k8s/manifests/essentials/
git commit -m "feat(deploy): add standardized metrics-server and local-path storage manifests"
```

---

### Task 4: Backend Domain & Entities for Hybrid Fleet & Cluster Essentials

**Files:**
- Create: `internal/domain/cluster/bootstrap.go`
- Test: `internal/domain/cluster/bootstrap_test.go`

**Interfaces:**
- Produces: `ClusterEssentialsStatus`, `BootstrapRequest`, `EssentialComponent` constants.

- [ ] **Step 1: Write Unit Test for Bootstrap Request Validation**
- [ ] **Step 2: Implement `bootstrap.go`**
- [ ] **Step 3: Run test to verify it passes**
Run: `go test -v ./internal/domain/cluster/...`
Expected: PASS

- [ ] **Step 4: Commit**
```bash
git add internal/domain/cluster/
git commit -m "feat(domain): define cluster essentials bootstrap entities"
```

---

### Task 5: Backend Usecase for Hybrid Host Auto-Sync & Cluster Essentials

**Files:**
- Create: `internal/usecase/cluster/bootstrap_usecase.go`
- Test: `internal/usecase/cluster/bootstrap_usecase_test.go`

**Interfaces:**
- Consumes: `ResourceRepo` (`ApplyYAML`, `UpdateNodeTaints`), `compute_hosts` repository
- Produces: `BootstrapService` with:
  - `GetEssentialsStatus(ctx, clusterID)`
  - `ExecuteBootstrap(ctx, clusterID, req)`
  - `SyncDiscoveredNodesToHostRegistry(ctx, clusterID)`: Scans all K8s Node IPs and upserts them into `compute_hosts` (`/hosts`) as `k8s` node type, while preserving standalone `bare-metal` and `database` hosts.
  - `GetPodMetrics(ctx, clusterID)`: Reads `/apis/metrics.k8s.io/v1beta1/pods`.

- [ ] **Step 1: Write Unit Test for Bootstrap Usecase**
- [ ] **Step 2: Implement `bootstrap_usecase.go`**
- [ ] **Step 3: Run test to verify it passes**
Run: `go test -v ./internal/usecase/cluster/...`
Expected: PASS

- [ ] **Step 4: Commit**
```bash
git add internal/usecase/cluster/
git commit -m "feat(usecase): implement cluster bootstrap and hybrid host sync service"
```

---

### Task 6: HTTP Handlers & Router Integration

**Files:**
- Create: `internal/adapter/http/k8s_bootstrap_handler.go`
- Modify: `internal/adapter/http/router.go`
- Test: `internal/adapter/http/k8s_bootstrap_handler_test.go`

**Interfaces:**
- Produces:
  - `GET  /api/v1/k8s/{cluster}/essentials`
  - `POST /api/v1/k8s/{cluster}/bootstrap`
  - `GET  /api/v1/k8s/{cluster}/metrics/pods`

- [ ] **Step 1: Write Handler Unit Test**
- [ ] **Step 2: Implement `k8s_bootstrap_handler.go` and Wire in `router.go`**
- [ ] **Step 3: Run test to verify it passes**
Run: `go test -v ./internal/adapter/http/ -run TestBootstrap`
Expected: PASS

- [ ] **Step 4: Commit**
```bash
git add internal/adapter/http/
git commit -m "feat(api): expose cluster essentials, hybrid sync, and pod metrics endpoints"
```

---

### Task 7: Frontend UI: Fleet Essentials Matrix & Tri-Runtime Badges

**Files:**
- Create: `frontend-vue/src/components/fleet/ClusterEssentialsMatrix.vue` (< 200 lines)
- Create: `frontend-vue/src/components/fleet/BootstrapClusterModal.vue` (< 250 lines)
- Create: `frontend-vue/src/components/explorer/PodMetricsSparkline.vue` (< 150 lines)
- Modify: `frontend-vue/src/components/fleet/ClusterDetailsDrawer.vue` (< 300 lines)
- Modify: `frontend-vue/src/components/hosts/HostCard.vue` (or table view): Show runtime badges (`[ 🖥️ BARE-METAL ]`, `[ 🐳 DOCKER ]`, `[ ☸️ K8S ]`) and role badges (`[ 💽 DATABASE ]`).

**Interfaces:**
- Consumes: Cluster bootstrap APIs and host APIs
- Produces: Visual distinction between Bare-Metal DB Hosts, Docker Hosts, and K8s Nodes.

- [ ] **Step 1: Create `ClusterEssentialsMatrix.vue`**
- [ ] **Step 2: Create `BootstrapClusterModal.vue`**
- [ ] **Step 3: Create `PodMetricsSparkline.vue`**
- [ ] **Step 4: Update Host UI with Tri-Runtime Badges**
- [ ] **Step 5: Verify Frontend Production Build**
Run: `powershell -Command "cd frontend-vue; npm run build"`
Expected: 0 errors, 0 warnings.

- [ ] **Step 6: Commit**
```bash
git add frontend-vue/src/
git commit -m "feat(ui): add cluster essentials matrix, bootstrap modal, and tri-runtime badges"
```

---

### Task 8: Verification on Live Environment (Bare-Metal DB, Docker & K8s Node)

**Files:**
- Test: `tests/e2e_cluster_bootstrap_test.go`

- [ ] **Step 1: Verify Bare-Metal DB Host Telemetry**
Verify `k8s-agent` on `masterdb` (10.10.10.200) reports active Postgres metrics with `runtime_environment: "bare-metal"` and `host_role: "database"`.
- [ ] **Step 2: Verify Docker Host Telemetry**
Verify `k8s-agent` running under Docker container reports `runtime_environment: "docker"`.
- [ ] **Step 3: Verify K8s Cluster Node Bootstrap**
Verify `k8smasterdeb` (10.10.10.60) runs DaemonSet reporting `runtime_environment: "kubernetes"`, and live pod sparklines render on UI.
