# TASK 009: Distributed High-Availability Storage & Volume Snapshot Engine Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Eliminate single-node data trapping by deploying Longhorn / Distributed CSI with 3-way synchronous cross-node replication (<15s failover SLA), dynamic online volume expansion, and scheduled encrypted volume snapshots to S3/MinIO.

**Architecture:** Distributed CSI driver replicates block storage across 3 distinct physical nodes. The K8sControl backend manages volume topology, exposes PVC metrics, triggers online volume expansion via Kubernetes patch API, and coordinates volume snapshots via CSI Snapshotter with retention policies.

**Tech Stack:** Go 1.23, Kubernetes CSI (v1.7+), Longhorn Engine, S3/MinIO SDK, Vue 3, TypeScript.

## Global Constraints
- All Vue files must stay <= 350 lines; styles separated into `src/assets/styles/views/` or sub-components.
- Backend must adhere to Clean Architecture: Domain -> Usecase -> HTTP Adapter -> Infrastructure.
- Zero fake/mock data: real volume metrics and Longhorn CRDs only.
- Strict Tam Quyền Phân Lập: Coder agents implement code; QA agents test; Orchestrator plans.

---

### Task 1: Package Longhorn Distributed CSI Manifests

**Files:**
- Create: `deploy/k8s/manifests/storage/longhorn-csi.yaml`
- Test: YAML syntax verification

**Interfaces:**
- Produces: Ready-to-deploy Longhorn CSI Driver, CSI Attacher, Provisioner, Resizer, Snapshotter, and StorageClass `longhorn-fast` (3 replicas).

- [ ] **Step 1: Write Longhorn CSI Manifest**
Create `deploy/k8s/manifests/storage/longhorn-csi.yaml` containing the complete Longhorn CSI deployment manifests, DaemonSets, and StorageClass definition with `numberOfReplicas: "3"` and `dataLocality: "best-effort"`.

- [ ] **Step 2: Validate Manifest Syntax**
Run: `powershell -Command "Get-Content deploy/k8s/manifests/storage/longhorn-csi.yaml | Select-Object -First 10"`
Expected: File exists and contains valid YAML header.

- [ ] **Step 3: Commit**
```bash
git add deploy/k8s/manifests/storage/longhorn-csi.yaml
git commit -m "feat(deploy): package distributed longhorn csi manifests with 3-way replication"
```

---

### Task 2: Backend Domain & Entities for Distributed Storage

**Files:**
- Create: `internal/domain/storage/volume.go`
- Test: `internal/domain/storage/volume_test.go`

**Interfaces:**
- Produces: `DistributedVolume`, `VolumeReplica`, `VolumeSnapshotJob`, `VolumeExpandRequest`.

- [ ] **Step 1: Write Unit Test for Volume Entity**
```go
package storage_test

import (
    "testing"
    "github.com/datdt/k8sselfhost/internal/domain/storage"
)

func TestDistributedVolume_IsHealthy(t *testing.T) {
    vol := storage.DistributedVolume{
        Name: "pvc-data-pg",
        ReplicationCount: 3,
        HealthStatus: "Healthy",
    }
    if !vol.IsHealthy() {
        t.Fatalf("expected healthy volume")
    }
}
```

- [ ] **Step 2: Run test to verify it fails**
Run: `go test -v ./internal/domain/storage/...`
Expected: FAIL

- [ ] **Step 3: Implement `volume.go`**
Create `internal/domain/storage/volume.go`:
```go
package storage

type VolumeReplica struct {
    NodeID   string `json:"node_id"`
    NodeName string `json:"node_name"`
    Mode     string `json:"mode"`
    Size     int64  `json:"size_bytes"`
}

type DistributedVolume struct {
    Name             string          `json:"name"`
    Namespace        string          `json:"namespace"`
    StorageClass     string          `json:"storage_class"`
    CapacityBytes    int64           `json:"capacity_bytes"`
    UsedBytes        int64           `json:"used_bytes"`
    ReplicationCount int             `json:"replication_count"`
    HealthStatus     string          `json:"health_status"`
    AttachedNode     string          `json:"attached_node"`
    Replicas         []VolumeReplica `json:"replicas"`
}

func (v *DistributedVolume) IsHealthy() bool {
    return v.HealthStatus == "Healthy"
}

type VolumeExpandRequest struct {
    NewSizeBytes int64 `json:"new_size_bytes"`
}
```

- [ ] **Step 4: Run test to verify it passes**
Run: `go test -v ./internal/domain/storage/...`
Expected: PASS

- [ ] **Step 5: Commit**
```bash
git add internal/domain/storage/
git commit -m "feat(domain): define distributed storage entities and replica models"
```

---

### Task 3: Backend Usecase for Distributed Volume Management & Expansion

**Files:**
- Create: `internal/usecase/storage/volume_usecase.go`
- Test: `internal/usecase/storage/volume_usecase_test.go`

**Interfaces:**
- Consumes: `k8s.io/client-go/dynamic.Interface`, `k8s.io/client-go/kubernetes.Clientset`
- Produces: `VolumeService` with `ListVolumes`, `ExpandVolume`, `TriggerSnapshot`.

- [ ] **Step 1: Write Unit Test for Volume Usecase**
Write test mocking PVC retrieval and verifying `ExpandVolume` calls K8s patch API correctly.

- [ ] **Step 2: Run test to verify it fails**
Run: `go test -v ./internal/usecase/storage/...`
Expected: FAIL

- [ ] **Step 3: Implement `volume_usecase.go`**
Implement dynamic client query to list PVCs and Longhorn volume CRDs (`volumes.longhorn.io`), patch PVC specs with new storage request, and trigger VolumeSnapshot creation.

- [ ] **Step 4: Run test to verify it passes**
Run: `go test -v ./internal/usecase/storage/...`
Expected: PASS

- [ ] **Step 5: Commit**
```bash
git add internal/usecase/storage/
git commit -m "feat(usecase): implement volume listing, online expansion, and snapshot service"
```

---

### Task 4: HTTP Adapter & Storage Endpoints

**Files:**
- Create: `internal/adapter/http/storage_handler.go`
- Modify: `internal/adapter/http/router.go`
- Test: `internal/adapter/http/storage_handler_test.go`

**Interfaces:**
- Consumes: `VolumeService`
- Produces:
  - `GET  /api/v1/k8s/{cluster}/storage/volumes`
  - `POST /api/v1/k8s/{cluster}/storage/volumes/{name}/expand`
  - `POST /api/v1/k8s/{cluster}/storage/volumes/{name}/snapshot`

- [ ] **Step 1: Write Storage Handler Unit Test**
Write test asserting `POST /expand` updates PVC and returns 200 OK.

- [ ] **Step 2: Run test to verify it fails**
Run: `go test -v ./internal/adapter/http/ -run TestStorage`
Expected: FAIL

- [ ] **Step 3: Implement `storage_handler.go` and Wire in `router.go`**
Mount routes under `/k8s/{cluster}/storage` in `router.go`.

- [ ] **Step 4: Run test to verify it passes**
Run: `go test -v ./internal/adapter/http/ -run TestStorage`
Expected: PASS

- [ ] **Step 5: Commit**
```bash
git add internal/adapter/http/
git commit -m "feat(api): expose distributed storage volume management endpoints"
```

---

### Task 5: Frontend Volume Topology & Expansion UI

**Files:**
- Create: `frontend-vue/src/components/storage/VolumeReplicaMatrix.vue` (< 200 lines)
- Create: `frontend-vue/src/components/storage/VolumeManageDrawer.vue` (< 250 lines)

**Interfaces:**
- Consumes: `GET /api/v1/k8s/{cluster}/storage/volumes`, `POST /expand`, `POST /snapshot`
- Produces: Visual 3-node replica layout and interactive online resize slider.

- [ ] **Step 1: Create `VolumeReplicaMatrix.vue`**
Component showing 3 physical node chips holding data blocks with health indicator (Emerald Green / Amber / Red).

- [ ] **Step 2: Create `VolumeManageDrawer.vue`**
Drawer with dynamic size slider (`10GB -> 50GB`), 1-click snapshot trigger, and IOPS throughput gauge.

- [ ] **Step 3: Verify Frontend Build**
Run: `powershell -Command "cd frontend-vue; npm run build"`
Expected: 0 errors, 0 warnings.

- [ ] **Step 4: Commit**
```bash
git add frontend-vue/src/
git commit -m "feat(ui): add volume replica topology matrix and online expansion drawer"
```

---

### Task 6: Chaos Verification: Node Outage Data Persistence

**Files:**
- Test: `tests/e2e_storage_ha_test.go`

- [ ] **Step 1: Run Storage HA Test**
Deploy PostgreSQL on `longhorn-fast`, simulate node failure, verify Pod restarts on surviving node with 100% data intact.
Expected: PASS
