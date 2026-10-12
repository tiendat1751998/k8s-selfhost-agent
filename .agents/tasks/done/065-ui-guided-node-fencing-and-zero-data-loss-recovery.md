# Task 065: UI-Guided Zero-Data-Loss Dead Node Recovery & Storage Fencing Engine

## 1. Executive Summary & Problem Context
When a Kubernetes worker or master node enters `NotReady` / `Unknown` due to hardware crash, power outage, kernel panic, or network partition:
1. **The Multi-Attach Trap**: Pods with ReadWriteOnce (RWO) PersistentVolumes (Postgres, Redis, Kafka, Ceph, EBS) get stuck in `Terminating` on the dead node. K8s will NOT schedule or mount replacement pods on healthy nodes because the CSI `VolumeAttachment` is still locked to the dead node (`Multi-Attach error for volume ... Volume is already exclusively attached`).
2. **The Split-Brain Threat**: If an operator or script blindly force-deletes pods (`kubectl delete pod --force`) without verifying that the dead node is truly powered off, the old node may recover and write concurrently to the same storage block, corrupting the entire database.
3. **The 5-Minute Default SLA Breach**: Default K8s `pod-eviction-timeout` delays pod relocation by 300 seconds (5 minutes).

This task delivers an **Enterprise UI-Guided Zero-Data-Loss Dead Node Recovery & Storage Fencing Engine**:
- An interactive, 5-stage fail-safe Wizard in the UI (`NodeStorageFencingModal.vue` / `NodeRemediationModal.vue`).
- A backend orchestration API integrating K8s 1.28+ Non-Graceful Node Shutdown (`node.kubernetes.io/out-of-service=nodeshutdown:NoExecute`) with `CSIFencingHelper` and `RemediationController`.
- Zero data corruption, zero multi-attach deadlocks, and an RTO < 30 seconds.

---

## 2. Technical Architecture & Component Design

### 2.1 Backend Architecture (Go Clean Architecture)

```
[ HTTP REST / SSE ] 
       │
       ▼
[ internal/adapter/http/node_fencing_handler.go ]
       │
       ▼
[ internal/usecase/sre/node_remediation.go ] ◄──► [ internal/usecase/storage/fencing.go ]
       │
       ├── CoreV1: Nodes, Pods, Taints
       └── StorageV1: VolumeAttachments, PV, PVC
```

#### New/Enhanced Backend Endpoints:
1. `GET /api/v1/clusters/:clusterId/nodes/:nodeName/fencing/preflight`
   - Scans target node and returns:
     - Node health, lease status, and condition.
     - Workload inventory: Stateful pods (with attached PVCs) vs Stateless pods.
     - VolumeAttachment inventory: PV name, PVC name, StorageClass, and attachment status.
     - Hardware isolation status (STONITH / IPMI if registered).
2. `POST /api/v1/clusters/:clusterId/nodes/:nodeName/fencing/execute`
   - Payload:
     ```json
     {
       "confirm_power_isolated": true,
       "auto_fence_storage": true,
       "evict_stateless_first": true,
       "evict_stateful_after_fencing": true,
       "timeout_seconds": 30
     }
     ```
   - Execution Pipeline:
     1. **Stage 1 (Cordon)**: Patch `spec.unschedulable = true`.
     2. **Stage 2 (Storage Fencing)**: Apply `node.kubernetes.io/out-of-service=nodeshutdown:NoExecute` taint via `CSIFencingHelper.ApplyOutOfServiceTaint`.
     3. **Stage 3 (Volume Release)**: Call `CSIFencingHelper.WaitForVolumeAttachmentsDetached` to ensure CSI driver releases locks. If volume is locked past deadline, force delete `VolumeAttachment` only after confirmed power isolation.
     4. **Stage 4 (Controlled Eviction)**: Force delete stateless pods immediately (<2s), then force delete stateful pods (<5s) so replica controllers spawn replacement pods.
     5. **Stage 5 (Verification)**: Polls K8s to verify replacement pods transition to `Running` without `Multi-Attach error`.

---

### 2.2 Frontend UI/UX Architecture (Vue 3 + TypeScript + 4-Tier RWD)

#### Component: `frontend-vue/src/components/overview/drawer/NodeStorageFencingModal.vue`
Interactive 5-Stage SRE Recovery Stepper:

```
[ Step 1: Impact Scan ] ➔ [ Step 2: STONITH Gate ] ➔ [ Step 3: CSI Fencing ] ➔ [ Step 4: Migration ] ➔ [ Step 5: Verification ]
```

1. **Step 1: Impact Assessment & Workload Inventory**:
   - Visual breakdown pills:
     - 🔴 **Stateful Workloads**: e.g. `postgres-0 (pvc-db-data, 50Gi)`, `redis-master-0`.
     - 🟢 **Stateless Workloads**: e.g. `api-gateway (3 replicas)`, `frontend (2 replicas)`.
     - ⚪ **DaemonSets**: Skipped.
   - Highlighting RWO volumes that will deadlock if not fenced.
2. **Step 2: STONITH & Power Isolation Gate**:
   - Operator safety check:
     - Radio/Toggle: "Node is powered off via IPMI/iLO/Cloud API" OR "Manual physical power switch off confirmed".
     - Mandatory checkbox: *"I certify that this node cannot write to shared/SAN/CSI storage, preventing split-brain corruption."*
3. **Step 3: Storage Fencing Execution**:
   - Live visual status:
     - Applying `node.kubernetes.io/out-of-service` taint: [DONE]
     - Releasing VolumeAttachments (`pvc-db-data`): [DETACHED]
4. **Step 4: Pod Relocation Pipeline**:
   - Evicting pods with real-time progress bar.
   - Replacement pods scheduled on surviving healthy nodes (e.g. `worker-2`, `worker-3`).
5. **Step 5: Post-Recovery Audit**:
   - Confirms all replaced pods are `Running` and healthy.
   - Direct button: "Drain & Decommission Node" or "Re-check Node Hardware".

---

## 3. Strict Safety Guardrails & Rollback Mechanism
1. **Anti-Split-Brain Barrier**: The execute API **strictly rejects** requests if `confirm_power_isolated` is false while RWO PVCs exist on the node.
2. **Automatic Rollback / Abort**:
   - If the operator cancels before Step 3, the node is simply uncordoned (`spec.unschedulable = false`).
   - If the node suddenly reports `Ready=True` before Step 3, the engine auto-aborts the fencing procedure and logs a warning.
3. **Audit Trail**: Every fencing action is logged to the Postgres Audit Table (`internal/infrastructure/postgres/audit_repo.go`) with operator identity, IP, target node, and affected volumes.

---

## 4. Work Breakdown & Scope of Files

### Backend Scope:
- `internal/usecase/sre/node_remediation.go`: Enhance with preflight scan and integrate `CSIFencingHelper`.
- `internal/adapter/http/dr_handler.go`: Expose `GET /fencing/preflight` and enhanced `POST /fencing/execute`.
- `internal/usecase/sre/node_remediation_test.go`: Unit tests for preflight scan, storage fencing, and error handling.

### Frontend Scope:
- `frontend-vue/src/api/dr.ts`: Define TypeScript types for PreflightScan, VolumeAttachmentItem, and FencingPayload.
- `frontend-vue/src/components/overview/drawer/NodeStorageFencingModal.vue`: The 5-stage interactive SRE modal (< 450 lines).
- `frontend-vue/src/assets/styles/components/node-fencing.css`: Scoped OLED styling and responsive progress bars.
- `frontend-vue/src/views/InfraHostsView.vue` & `frontend-vue/src/views/OverviewView.vue`: Wire "Safe Remediation / Fence" trigger for `NotReady` nodes.
