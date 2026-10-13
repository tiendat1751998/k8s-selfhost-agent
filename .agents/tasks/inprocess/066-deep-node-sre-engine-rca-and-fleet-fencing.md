# Task 066: Deep Node SRE Engine — Dynamic Scoring Matrix, Host Service Probing, 1M+1W Capacity-Aware Admission & Fleet Blast-Radius Fencing

## 1. Executive Summary & Production Problem Statement
Task 065 delivered the baseline UI modal and K8s 1.28+ `out-of-service` taint integration. However, in enterprise multi-cluster environments and resource-constrained edge topologies (e.g., 1 Master + 1 Worker), real production requires eliminating all heuristic toy artifacts and addressing 4 critical failure modes:
1. **Toy RCA Elimination**: Replace static hardcoded confidence numbers (`0.94`, `0.89`) with a rigorous, mathematically grounded **Weighted Evidence Scoring Matrix** combining K8s conditions, kernel dmesg, and systemd journal logs.
2. **Pre-Fencing Service Probing (Soft vs Hard Death)**: In ~80% of node failures, the underlying Linux OS and hardware are healthy; only `kubelet` or `containerd` crashed or deadlocked. Blindly fencing storage and killing nodes in this scenario causes unnecessary downtime. The engine must first attempt an automated host service rescue (`systemctl restart containerd kubelet`) within a 30s grace window.
3. **1M+1W Topology Cascading OOM Defense**: In a 1 Master + 1 Worker cluster, blindly evacuating all worker workloads to the master will cause the master to exhaust its RAM/CPU, killing `etcd` and the API server (complete cluster destruction). The engine must enforce **Capacity-Aware Admission & Priority-Based Workload Shedding** (admitting P0 DB/Ingress, suspending P2/P3 batch/cron/observability).
4. **100-Cluster Fleet Blast-Radius Protection**: When scaling to 100 clusters, simultaneous multi-node failures indicate correlated incidents (kernel CVE, cloud AZ failure, expired CA root). The engine must enforce a **Fleet Blast-Radius Circuit Breaker** (>15% node loss triggers an immediate eviction freeze) and integrate an extensible `NodeProvisionerDriver` (AWS ASG, Proxmox QEMU, PXE).

---

## 2. Technical Architecture & Component Specification

### 2.1 Pillar 1: Dynamic Multi-Signal RCA Scoring Matrix Engine

#### Data Model (`internal/domain/sre/rca_scoring.go`)
```go
type IndicatorSignal struct {
    ID          string  `json:"id"`
    Description string  `json:"description"`
    Weight      float64 `json:"weight"`      // 1.0 to 5.0
    Detected    bool    `json:"detected"`
    Evidence    string  `json:"evidence"`    // Verbatim metric value or log snippet
}

type FailureHypothesis struct {
    Category       FailureCategory   `json:"category"`
    ProbableCause  string            `json:"probable_cause"`
    Indicators     []IndicatorSignal `json:"indicators"`
    Recommendation string            `json:"recommendation"`
}

type DynamicRCAReport struct {
    NodeName       string              `json:"node_name"`
    ClusterID      string              `json:"cluster_id"`
    Timestamp      string              `json:"timestamp"`
    PrimaryCause   string              `json:"primary_cause"`
    Confidence     float64             `json:"confidence"` // Dynamically calculated
    Hypotheses     []FailureHypothesis `json:"hypotheses"`
    ActionPlan     string              `json:"action_plan"`
}
```

#### Dynamic Scoring Formula:
$$\text{Score}(F_k) = \frac{\sum_{i \in \text{Detected}} w_i}{\sum_{i \in \text{All}} w_i}$$
$$\text{Confidence}(F_k) = \min\left(0.99, \frac{\text{Score}(F_k)}{1.0}\right)$$
Zero hardcoded float literals. Every confidence score is dynamically derived from factual evidence.

---

### 2.2 Pillar 2: Pre-Fencing Host Service Probing & Soft-Death Rescue

#### Protocol:
1. **Phase 1: Liveness Probing**:
   - Probe 1: ICMP Ping to Node IP.
   - Probe 2: TCP connect to Node SSH / Agent port.
   - Probe 3: TCP connect to Kubelet port `10250`.
   - Probe 4: Container runtime socket `/run/containerd/containerd.sock`.
2. **Phase 2: Soft-Death Triage**:
   - If Host IP is pingable and Agent responds, but Kubelet is `NotReady`:
     - Mark state as `SOFT_DEATH_SERVICE_FAULT`.
     - Attempt service recovery: `ExecuteServiceRescue(ctx, nodeName, []string{"containerd", "kubelet"})`.
     - Poll Kubelet lease renewal for 30s.
     - If Node recovers to `Ready=True`:
       - **RESCUE_SUCCESS**: Cancel fencing, zero pod evictions, zero volume detachments.
     - If service restart fails:
       - Escalate to `HARD_DEATH_FENCING_REQUIRED`.

---

### 2.3 Pillar 3: 1 Master + 1 Worker Capacity-Aware Admission & Priority Shedding

#### Algorithm:
1. **Compute Master Available Headroom**:
   $$RAM_{\text{avail}} = RAM_{\text{allocatable}} - \sum RAM_{\text{req}}(\text{MasterExistingPods})$$
   $$CPU_{\text{avail}} = CPU_{\text{allocatable}} - \sum CPU_{\text{req}}(\text{MasterExistingPods})$$
2. **Sort Worker Pods by Priority Tier**:
   - **Tier 0 (Critical P0)**: Stateful databases (PVC attached), Ingress controller.
   - **Tier 1 (Core P1)**: Web APIs, core microservices.
   - **Tier 2 (Batch P2)**: Background workers, report processors.
   - **Tier 3 (Auxiliary P3)**: Observability collectors, promtail, test runners.
3. **Admission Decision**:
   - For each pod in sorted priority order:
     - If $RAM_{\text{req}}(p) \le RAM_{\text{avail}}$ and $CPU_{\text{req}}(p) \le CPU_{\text{avail}}$:
       - Admit pod to Master, decrement available headroom.
     - Else:
       - **Shed Pod**: Suspend Deployment (scale to 0) or defer scheduling with label `sre.k8s.local/capacity-deferred=true`.
       - Record in Execution Report: `SheddedPods: [pod-name] (Preserving Master Stability)`.

---

### 2.4 Pillar 4: Multi-Cluster (100 Clusters) Fleet Architecture & Blast-Radius Circuit Breaker

#### Fleet Circuit Breaker Rules:
1. **Cluster-Level Rule**: If $\frac{\text{UnreadyNodes}}{\text{TotalNodes}} > 0.15$ in a single cluster, trip cluster circuit breaker (`CLUSTER_BLAST_RADIUS_TRIPPED`). Freeze automated mass evictions to prevent cascading control plane collapse.
2. **Fleet-Level Rule**: If $> 5$ clusters experience simultaneous node loss within a 3-minute sliding window, trip fleet circuit breaker (`FLEET_CORRELATED_DISASTER_TRIPPED`). Dispatch P1 emergency notification and pause autonomous remediation across all clusters.
3. **Node Provisioner Interface**:
   ```go
   type NodeProvisionerDriver interface {
       ProvisionReplacementNode(ctx context.Context, clusterID string, failedNode *NodeDetails) (*ProvisionResult, error)
       DeprovisionFailedNode(ctx context.Context, clusterID string, failedNode *NodeDetails) error
   }
   ```

---

## 3. Work Breakdown & Subagent Scope

### Phase 1: Backend Domain & SRE Engine
- `internal/domain/sre/rca_scoring.go`: Scoring matrix, hypothesis models, dynamic confidence math.
- `internal/usecase/sre/node_service_rescue.go`: Host probing, soft-death detection, systemd restart probe.
- `internal/usecase/sre/capacity_admission.go`: 1M+1W Master headroom calculation, priority-based pod shedding.
- `internal/usecase/sre/fleet_circuit_breaker.go`: Cluster and Fleet-wide blast-radius circuit breaker.
- `internal/usecase/sre/node_remediation.go`: Integrate all 4 pillars into unified execution pipeline.
- All files strictly `< 500 lines`.

### Phase 2: Frontend UI & Telemetry Visualization
- `frontend-vue/src/components/overview/drawer/NodeStorageFencingModal.vue`:
  - Enhance with Soft-Death Rescue action ("Try Service Restart first").
  - Dynamic RCA evidence viewer with breakdown scores.
  - 1M+1W Master Capacity Headroom Gauge (visualizing admitted vs shedded pods).
  - Fleet Blast-Radius Warning Banner.
- `frontend-vue/src/api/dr.ts`: Add types and endpoints.

### Phase 3: Independent Review & Verification
- `reviewer`: Code diff audit.
- `qa-test-engineer`: Chrome DevTools MCP live audit across 375x812 and 1440x900.
