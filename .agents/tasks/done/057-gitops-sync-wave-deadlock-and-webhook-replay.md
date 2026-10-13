# Task 057: GitOps Sync-Wave DAG Verifier & Webhook Monotonic Ordering Guard
**Status**: `READY_FOR_IMPLEMENTATION`  
**Priority**: `P1_HIGH`  
**FMEA Scenarios Covered**: Scenario 151, 152, 153, 154, 155 (Tầng 34)  
**Target Package Scope**: `internal/infrastructure/gitops/` & `internal/usecase/gitops/`

---

## 1. Root Cause Analysis (Verbatim Production Failure Mode)

### Problematic Scenario
Trong hệ thống GitOps và CI/CD phức tạp:
1. **Sync-Wave Deadlock**: Manifests có quan hệ phụ thuộc vòng tròn giữa sync-waves và mutating webhooks khiến ArgoCD/Flux bị treo vĩnh viễn ở `Progressing`.
2. **Webhook Out-of-Order Rollback**: Webhook commit cũ đến muộn do mạng chập chờn, GitOps checkout đè commit cũ lên production làm mất sạch hotfix mới.
3. **Monorepo Clone OOM**: CI runner đọc toàn bộ packfile repository 50GB làm cạn kiệt RAM, bị Linux OOM killer tiêu diệt (`exit code 137`).
4. **Three-Way Merge Conflict**: Thao tác thủ công `kubectl edit` làm lệch `managedFields`, chặn đứng hoàn toàn các lần Helm upgrade tiếp theo.

---

## 2. Go Structs & Interface Contracts

```go
package gitops

import (
	"context"
	"time"
)

type WebhookEventPayload struct {
	DeliveryID      string    `json:"delivery_id"`
	RepositoryURL   string    `json:"repository_url"`
	Branch          string    `json:"branch"`
	CommitSHA       string    `json:"commit_sha"`
	CommitTimestamp time.Time `json:"commit_timestamp"`
	Author          string    `json:"author"`
}

type SyncWaveGraphNode struct {
	ResourceName string   `json:"resource_name"`
	Kind         string   `json:"kind"`
	Wave         int      `json:"wave"`
	DependsOn    []string `json:"depends_on"`
}

type DriftReconciliationResult struct {
	HasDrift        bool     `json:"has_drift"`
	DriftedFields   []string `json:"drifted_fields"`
	ConflictDetected bool    `json:"conflict_detected"`
	AutoResolved    bool     `json:"auto_resolved"`
}

type GitOpsResilienceEngine interface {
	VerifyCommitMonotonicOrdering(ctx context.Context, repoPath string, incomingSHA, currentSHA string) (isNewer bool, err error)
	AuditSyncWaveCycle(ctx context.Context, manifests [][]byte) (hasCycle bool, cycleDetails string, err error)
	DetectAndReconcileDrift(ctx context.Context, releaseName, namespace string, newChartManifest []byte) (*DriftReconciliationResult, error)
}
```

---

## 3. Step-by-Step State Machine Algorithm

```
State: IDLE
  │
  ├─► [Event: WEBHOOK_RECEIVED]
  │     ├─ Extract incoming CommitSHA & CommitTimestamp
  │     ├─ Query git merge-base --is-ancestor incoming current:
  │     │     ├─ If incoming is ancestor of current -> DROP (Obsolete Webhook)
  │     │     └─ If current is ancestor of incoming -> ACCEPT & TRIGGER_ROLLOUT
  │     └─ Log audit trail: GITOPS_MONOTONIC_VERIFIED
  │
  ├─► [Event: SYNC_WAVE_VALIDATE_PREFLIGHT]
  │     ├─ Parse all manifests in target release
  │     ├─ Build Directed Acyclic Graph (DAG) by sync-wave annotations
  │     ├─ Run Tarjan's Strongly Connected Components algorithm
  │     ├─ If Cycle detected:
  │     │     ├─ Reject PR / Release
  │     │     └─ Return ERROR_GITOPS_CIRCULAR_SYNC_WAVE
  │     └─ If Valid: Proceed to staging rollout
```

---

## 4. RFC 7807 Problem Details Error Codes

- `ERROR_GITOPS_OBSOLETE_WEBHOOK`: Webhook mang commit cũ hơn phiên bản đang chạy.
- `ERROR_GITOPS_CIRCULAR_SYNC_WAVE`: Phát hiện chu trình deadlock giữa các sync-waves.
- `ERROR_GITOPS_DRIFT_CONFLICT`: Xung đột three-way merge do thao tác ngoài luồng.

---

## 5. Verification Test Matrix

1. `TestMonotonicOrdering_RejectsOlderCommitWebhook`: Verify obsolete commit webhook dropped.
2. `TestSyncWaveDAG_DetectsCircularDependency`: Verify cyclic sync-wave manifests rejected.
3. `TestDriftReconciliation_DetectsManualEdits`: Verify out-of-band edit flagged before upgrade.

---

## 6. Rollback Guardrails

- Nếu phát hiện drift nghiêm trọng không thể tự reconcile, tạm dừng auto-sync và gửi cảnh báo khẩn cấp lên dashboard để kỹ sư phê duyệt can thiệp.
