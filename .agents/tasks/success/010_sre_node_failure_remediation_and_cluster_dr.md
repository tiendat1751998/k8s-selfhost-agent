# TASK 010: SRE Node Failure Auto-Remediation (<30s Failover) & Cluster Disaster Recovery (etcd & Velero) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Eliminate the default 5-minute Kubernetes pod eviction delay by implementing a fast-failover controller (<30s recovery) upon physical node crashes, and engineer full cluster disaster recovery via an automated etcd snapshot driver and Velero integration.

**Architecture:** The SRE Auto-Remediation Controller listens to `node_watcher.go` and `collector.go` heartbeats; when a node drops offline for 15s, it auto-cordons the node and force-deletes stuck `Terminating` pods, triggering immediate rescheduling on healthy nodes. The etcd backup driver integrates directly into the existing `internal/infrastructure/backup/engine.go` dualsync pipeline (zstd + AES-256 to S3/MinIO), paired with Velero for full-cluster manifest restores.

**Tech Stack:** Go 1.23, etcd clientv3, Velero SDK, Chi router, Vue 3, TypeScript, Telegram SRE Bot.

## Global Constraints
- All Vue files must stay <= 350 lines; styles separated into `src/assets/styles/views/` or sub-components.
- Backend must adhere to Clean Architecture: Domain -> Usecase -> HTTP Adapter -> Infrastructure.
- Zero fake/mock data: real etcd snapshots and Kubernetes node status events only.
- Strict Tam Quyền Phân Lập: Coder agents implement code; QA agents test; Orchestrator plans.

---

### Task 1: SRE Node Auto-Remediation & Fast-Failover Controller

**Files:**
- Create: `internal/usecase/sre/node_remediation.go`
- Test: `internal/usecase/sre/node_remediation_test.go`

**Interfaces:**
- Consumes: `internal/adapter/event/node_watcher.go`, `internal/infrastructure/kubernetes/resource_repo.go`
- Produces: `RemediationController` with `HandleNodeOffline(ctx, nodeName)`.

- [ ] **Step 1: Write Unit Test for Node Remediation Logic**
```go
package sre_test

import (
    "context"
    "testing"
    "github.com/datdt/k8sselfhost/internal/usecase/sre"
)

func TestRemediationController_HandleNodeOffline(t *testing.T) {
    // Mock ResourceRepo and verify Cordon and Pod Eviction called within 15s
    ctrl := sre.NewRemediationController(nil, nil, nil)
    err := ctrl.HandleNodeOffline(context.Background(), "test-node-01")
    if err == nil {
        t.Log("remediation triggered successfully")
    }
}
```

- [ ] **Step 2: Run test to verify it fails**
Run: `go test -v ./internal/usecase/sre/...`
Expected: FAIL

- [ ] **Step 3: Implement `node_remediation.go`**
Implement the fast-failover algorithm:
1. When node heartbeat fails 3 consecutive times (15s):
2. Call `resourceRepo.CordonNode(ctx, clusterID, nodeName)`.
3. List pods on node; filter Deployment/ReplicaSet pods stuck in `Terminating`.
4. Call `client.CoreV1().Pods(ns).Delete(ctx, podName, metav1.DeleteOptions{GracePeriodSeconds: new(int64)})` (gracePeriod=0).
5. Emit `incident.TypeNodeNotReady` with impact report and trigger Telegram notification.

- [ ] **Step 4: Run test to verify it passes**
Run: `go test -v ./internal/usecase/sre/...`
Expected: PASS

- [ ] **Step 5: Commit**
```bash
git add internal/usecase/sre/
git commit -m "feat(sre): implement node fast-failover and auto-remediation controller"
```

---

### Task 2: Backend etcd Disaster Recovery Snapshot Driver

**Files:**
- Create: `internal/infrastructure/backup/drivers/etcd.go`
- Test: `internal/infrastructure/backup/drivers/etcd_test.go`

**Interfaces:**
- Consumes: `internal/infrastructure/backup/engine.go` (`Driver` interface)
- Produces: Registered `etcd` driver in `drivers/registry.go`.

- [ ] **Step 1: Write Unit Test for etcd Driver**
Write test verifying etcd driver satisfies `backup.Driver` interface (`Name()`, `Backup()`, `Restore()`, `ValidateConfig()`).

- [ ] **Step 2: Run test to verify it fails**
Run: `go test -v ./internal/infrastructure/backup/drivers/ -run TestEtcd`
Expected: FAIL

- [ ] **Step 3: Implement `etcd.go`**
Create `internal/infrastructure/backup/drivers/etcd.go`:
Connect to `https://127.0.0.1:2379` using clientv3 and TLS certs from `/etc/kubernetes/pki/etcd/`. Stream snapshot bytes through `dualsync` pipeline (zstd level 3 + AES-256-GCM) into S3/Local targets. Implement `Restore` method.

- [ ] **Step 4: Register `etcd` in `drivers/registry.go`**
Add `etcd` to the available database/system drivers registry.

- [ ] **Step 5: Run test to verify it passes**
Run: `go test -v ./internal/infrastructure/backup/drivers/ -run TestEtcd`
Expected: PASS

- [ ] **Step 6: Commit**
```bash
git add internal/infrastructure/backup/drivers/
git commit -m "feat(backup): add etcd snapshot driver with zstd compression and aes-256 encryption"
```

---

### Task 3: Velero Full-Cluster State Backup Manifests & Bridge

**Files:**
- Create: `deploy/k8s/manifests/dr/velero.yaml`
- Create: `internal/usecase/dr/velero_usecase.go`
- Test: `internal/usecase/dr/velero_usecase_test.go`

**Interfaces:**
- Produces: `VeleroService` with `TriggerBackup`, `ListBackups`, `TriggerRestore`.

- [ ] **Step 1: Write Velero Manifests**
Create `deploy/k8s/manifests/dr/velero.yaml` configuring Velero deployment, plugin for AWS/S3, and BackupStorageLocation pointing to MinIO/S3.

- [ ] **Step 2: Implement `velero_usecase.go`**
Interact with Velero CRDs (`backups.velero.io`, `restores.velero.io`) via dynamic client.

- [ ] **Step 3: Run test to verify it passes**
Run: `go test -v ./internal/usecase/dr/...`
Expected: PASS

- [ ] **Step 4: Commit**
```bash
git add deploy/k8s/manifests/dr/ internal/usecase/dr/
git commit -m "feat(dr): integrate velero full cluster state backup and restore engine"
```

---

### Task 4: HTTP Handlers & Router Integration

**Files:**
- Create: `internal/adapter/http/dr_handler.go`
- Modify: `internal/adapter/http/router.go`
- Test: `internal/adapter/http/dr_handler_test.go`

**Interfaces:**
- Produces:
  - `POST /api/v1/k8s/{cluster}/dr/etcd/snapshot`
  - `POST /api/v1/k8s/{cluster}/dr/etcd/restore`
  - `GET  /api/v1/k8s/{cluster}/dr/backups`
  - `POST /api/v1/k8s/{cluster}/dr/backups`

- [ ] **Step 1: Write DR Handler Unit Test**
Write test asserting `POST /etcd/snapshot` returns 200 OK.

- [ ] **Step 2: Implement `dr_handler.go` and Wire in `router.go`**
Mount routes under `/k8s/{cluster}/dr` in `router.go`.

- [ ] **Step 3: Run test to verify it passes**
Run: `go test -v ./internal/adapter/http/ -run TestDR`
Expected: PASS

- [ ] **Step 4: Commit**
```bash
git add internal/adapter/http/
git commit -m "feat(api): expose disaster recovery etcd and velero endpoints"
```

---

### Task 5: Frontend Disaster Recovery Tab & SRE Settings

**Files:**
- Create: `frontend-vue/src/components/backup/ClusterDisasterRecoveryTab.vue` (< 250 lines)
- Create: `frontend-vue/src/components/settings/NodeRemediationSettings.vue` (< 200 lines)
- Modify: `frontend-vue/src/views/BackupRestoreView.vue` (< 350 lines)

**Interfaces:**
- Consumes: DR APIs and Settings APIs
- Produces: SRE Auto-Remediation toggle controls and 1-Click Cluster Restore UI.

- [ ] **Step 1: Create `ClusterDisasterRecoveryTab.vue`**
Tab listing hourly etcd snapshots and Velero cluster backups with `[ ⚡ Restore etcd ]` and `[ 🚀 1-Click Disaster Recovery ]`.

- [ ] **Step 2: Create `NodeRemediationSettings.vue`**
Settings toggle for `[x] Automated Fast-Failover (< 30s)` and heartbeat threshold slider.

- [ ] **Step 3: Integrate into `BackupRestoreView.vue`**
Embed tab cleanly without exceeding 350 lines.

- [ ] **Step 4: Verify Frontend Build**
Run: `powershell -Command "cd frontend-vue; npm run build"`
Expected: 0 errors, 0 warnings.

- [ ] **Step 5: Commit**
```bash
git add frontend-vue/src/
git commit -m "feat(ui): add cluster disaster recovery tab and sre auto-remediation settings"
```

---

### Task 6: Chaos Verification: Node Hard Crash & Full Cluster Recovery

**Files:**
- Test: `tests/e2e_disaster_recovery_test.go`

- [ ] **Step 1: Run SRE Fast Failover Chaos Test**
Simulate node network isolation; verify pods failover to surviving nodes in < 30 seconds.
Expected: PASS

- [ ] **Step 2: Run etcd Snapshot & Restore Validation**
Trigger snapshot, corrupt test key in etcd, restore from S3 snapshot, verify cluster state is 100% recovered.
Expected: PASS
