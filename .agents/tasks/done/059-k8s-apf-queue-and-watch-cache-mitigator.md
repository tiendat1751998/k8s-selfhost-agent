# Task 059: Kubernetes APF Queue Optimizer & Watch Cache Thundering Herd Guard
**Status**: `READY_FOR_IMPLEMENTATION`  
**Priority**: `P1_HIGH`  
**FMEA Scenarios Covered**: Scenario 161, 162, 163, 164, 165 (Tầng 36)  
**Target Package Scope**: `internal/infrastructure/k8scontrol/` & `internal/usecase/k8scontrol/`

---

## 1. Root Cause Analysis (Verbatim Production Failure Mode)

### Problematic Scenario
Khi Kubernetes Control Plane chịu tải đột biến:
1. **Unbounded List-Watches**: Quá nhiều client gọi `List` không paging làm `kube-apiserver` tốn 32GB RAM, rớt probe `/livez` và crash liên tục.
2. **Watch Cache Compaction Thundering Herd**: Mã lỗi `HTTP 410 Gone` khiến hàng trăm controller đồng loạt fallback re-list toàn cụm, làm nghẽn 100% CPU API server.
3. **APF FlowSchema Starvation**: Request gia hạn lease của `kube-controller-manager` bị xếp chung hàng đợi `workload-high`, mất quyền leader election và restart hàng loạt.
4. **Metrics-Server Crash HPA Freeze**: Extension APIService rớt khiến HPA không đọc được CPU/RAM, đóng băng số lượng replica trong đợt cao điểm.

---

## 2. Go Structs & Interface Contracts

```go
package k8scontrol

import (
	"context"
	"time"
)

type APFQueueStatus struct {
	PriorityLevel   string        `json:"priority_level"`
	CurrentInQueue  int           `json:"current_in_queue"`
	CurrentExecuting int          `json:"current_executing"`
	ConcurrencyLimit int          `json:"concurrency_limit"`
	DropCount       int64         `json:"drop_count"`
	IsStarved       bool          `json:"is_starved"`
}

type WatchCacheCompactionPolicy struct {
	MaxJitterWindow time.Duration `json:"max_jitter_window"`
	BatchSize       int           `json:"batch_size"`
	CompactionAge   time.Duration `json:"compaction_age"`
}

type HPAAutoscaleGuardReport struct {
	MetricsAPIAvailable bool      `json:"metrics_api_available"`
	LastSampleTime      time.Time `json:"last_sample_time"`
	FallbackTriggered   bool      `json:"fallback_triggered"`
	TargetReplicas      int32     `json:"target_replicas"`
}

type K8sControlPlaneResilienceEngine interface {
	AuditAPFQueueStatus(ctx context.Context) ([]APFQueueStatus, error)
	MitigateWatchCacheCompactionThunderingHerd(ctx context.Context, policy WatchCacheCompactionPolicy) error
	CheckHPAHealthAndApplyFallback(ctx context.Context, namespace, deployment string) (*HPAAutoscaleGuardReport, error)
}
```

---

## 3. Step-by-Step State Machine Algorithm

```
State: MONITORING
  │
  ├─► [Event: APF_AUDIT_TICK (every 15s)]
  │     ├─ Scrape apiserver_flowcontrol_current_inqueue_requests
  │     ├─ If system-leader-election in_queue > 0:
  │     │     ├─ Alert P0: LEADER_ELECTION_QUEUE_STARVED
  │     │     └─ Auto-scale assuredConcurrencyShares for exempt flow
  │
  ├─► [Event: WATCH_CACHE_410_DETECTED]
  │     ├─ Intercept HTTP 410 Gone error
  │     ├─ Inject random exponential jitter (5s - 60s)
  │     ├─ Paginate re-list by namespace with limit=500
  │     └─ Verify API server memory remains < 75%
  │
  └─► [Event: METRICS_SERVER_FAILURE_PROBE]
        ├─ Probe GET /apis/metrics.k8s.io/v1beta1
        ├─ If 503 / timeout > 2m:
        │     ├─ Trigger Fallback Scaling: Replicas = Max(current, minReplicas * 1.5)
        │     └─ Alert P1: HPA_FROZEN_METRICS_SERVER_DOWN
```

---

## 4. RFC 7807 Problem Details Error Codes

- `ERROR_APF_QUEUE_SATURATED`: Hàng đợi API Priority and Fairness bị nghẽn.
- `ERROR_WATCH_CACHE_COMPACTED`: ResourceVersion quá cũ bị xóa khỏi etcd cache.
- `ERROR_METRICS_SERVER_UNAVAILABLE`: Không thể truy vấn Metrics API cho HPA.

---

## 5. Verification Test Matrix

1. `TestAPFQueueOptimizer_DetectsStarvation`: Inject high queue depth, verify alert triggered.
2. `TestWatchCache_AppliesJitteredBackoff`: Mock 410 Gone, verify sleep duration within jitter window.
3. `TestHPAAutoscaleGuard_AppliesFallbackReplicas`: Mock 503 on metrics API, verify fallback scale applied.

---

## 6. Rollback Guardrails

- Không nâng concurrency shares của unprivileged flows lên quá 80% tổng dung lượng API server để tránh làm sập control plane.
