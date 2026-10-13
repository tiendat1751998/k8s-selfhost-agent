// Package apfguard provides domain contracts, resilience models, and cache-line padded
// telemetry trackers for Kubernetes API Priority and Fairness (APF) queue optimization,
// watch cache 410 Gone compaction thundering herd mitigation, and HPA fallback scaling.
package apfguard

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

/*
ENGINEERING NOTE: PHYSICAL HARDWARE BOTTLENECKS & KUBERNETES CONTROL PLANE FAILURE MODES

1. PHYSICAL HARDWARE BOTTLENECKS:
- Kube-apiserver etcd Watch Serialization Overhead:
  The kube-apiserver acts as a state gateway between etcd and all cluster consumers (kubelets,
  controllers, operators, kube-proxy, daemonsets). Every watch event received from etcd's bbolt
  Raft log stream is deserialized and fanned out to thousands of open watch connections. Under high
  cluster mutation rates, JSON and Protocol Buffers serialization across multi-threaded HTTP/2
  streams saturates CPU caches and memory bandwidth. When watch client buffers fill, apiserver
  threads stall waiting on TCP socket write buffers, introducing CPU pipeline stalls and high
  kernel network stack contention.

- Memory Bloat on Unbounded List Calls (32GB RAM Spikes & OOM Killer):
  When controllers or operators issue unindexed, cluster-wide LIST requests (e.g., `LIST pods` or
  `LIST endpoints` across all namespaces without pagination or chunking), the kube-apiserver must
  fetch every matching item from its in-memory watch cache or query etcd directly. Deserializing tens
  of thousands of complete Kubernetes API object graphs causes multi-gigabyte heap allocation spikes.
  Because Go's concurrent garbage collector (GC) requires up to 2x current live heap memory (GOGC=100)
  before sweeping, apiserver memory rapidly surges past 32GB, tripping Linux cgroup memory limits
  (oom-killer kills `kube-apiserver` with SIGKILL).

- APF Fair-Queueing CPU Overhead:
  API Priority and Fairness (APF) replaces legacy max-requests-inflight semaphores with a multi-queue
  fair queueing algorithm (Fair Queuing with Deficit Round Robin / FQ-CoDel). Every incoming HTTP
  request is evaluated against FlowSchemas, mapped into a PriorityLevelConfiguration, hashed into
  one of multiple FIFO queues, and dispatched based on assured concurrency shares. Under request bursts
  (>10,000 req/s), queue locks, atomic virtual time calculations, and shuffle sharding hash evaluations
  induce substantial CPU cache bouncing (L1/L2 miss rates) across NUMA nodes.

- Network Saturation on Mass Controller Reconnects:
  When an apiserver replica restarts or transient network partitions heal, thousands of kubelets,
  CSI/CNI plugins, and custom controllers reconnect simultaneously. Each client issues initialization
  LIST queries followed by WATCH streams. This SYN flood and TLS handshake storm saturates network
  interfaces (10GbE/25GbE NIC drops, nf_conntrack exhaustion, epoll wakeup storms), cascading into
  HTTP request timeouts and connection resets across the entire control plane.

2. REAL-WORLD FAILURE MODES:
- HTTP 410 Gone Compaction Thundering Herd Crashing API Server:
  etcd periodically compacts historical revisions to reclaim disk space. If a slow watcher (or a
  watcher disconnected during network blips) attempts to resume watching from a compacted resourceVersion,
  kube-apiserver responds with HTTP 410 Gone ("too old resource version"). Standard client-go Informers
  fall back to a full LIST to re-establish the baseline resourceVersion. When hundreds of controllers
  receive 410 Gone simultaneously, they unleash an unpaginated LIST thundering herd against apiserver,
  triggering memory exhaustion and cascading restarts. Mitigation requires randomized exponential jitter
  (5s to 60s) and paginated chunking (limit=500).

- APF FlowSchema Starvation Starving kube-controller-manager Leader Election:
  `kube-controller-manager` and `kube-scheduler` maintain active-standby leadership via Lease API
  objects (`coordination.k8s.io/leases`) updated every 2s (renewDeadline 10s, leaseDuration 15s).
  If APF flow schemas misclassify leader election requests or if the `system-leader-election` priority
  level experiences queue depth saturation (`CurrentInQueue > 0`), lease renew requests queue and timeout.
  The leader controller assumes it has lost leadership, terminates active reconcilers, or panics,
  causing split-brain controller behavior or continuous restart loops.

- Extension APIService Crash Freezing HPA Scaling During Peak Traffic:
  Horizontal Pod Autoscalers (HPA) rely on aggregated metrics APIs (`/apis/metrics.k8s.io/v1beta1`)
  served by metrics-server or Prometheus adapter. If the extension APIService crashes or enters 503
  unavailability during a viral traffic surge, HPA reconcilers fail to retrieve CPU/memory metrics.
  By default, HPA freezes scaling at the current replica count, preventing workloads from autoscaling
  to handle the load spike, leading to severe end-user 5xx outages. Fallback scaling proactively scales
  target replicas based on historical minimums.
*/

// Standard sentinel errors for APF and watch cache operations.
var (
	// ErrAPFQueueSaturated is returned when an APF priority level queue reaches capacity and drops requests.
	ErrAPFQueueSaturated = errors.New("apf priority level queue saturated: requests being dropped or queuing")

	// ErrWatchCacheCompacted is returned when an etcd resourceVersion is compacted (HTTP 410 Gone).
	ErrWatchCacheCompacted = errors.New("watch cache compacted: resource version too old (HTTP 410 Gone)")

	// ErrMetricsServerUnavailable is returned when the metrics-server APIService is unreachable.
	ErrMetricsServerUnavailable = errors.New("metrics-server /apis/metrics.k8s.io/v1beta1 unavailable")

	// ErrLeaderElectionStarved is returned when system-leader-election priority level experiences queue delay.
	ErrLeaderElectionStarved = errors.New("critical: system-leader-election priority level starved in APF")

	// ErrUnprivilegedCapExceeded is returned when unprivileged concurrency shares exceed 80% guardrail limit.
	ErrUnprivilegedCapExceeded = errors.New("guardrail violation: unprivileged flow concurrency shares cannot exceed 80% of total capacity")
)

// PaddedAPFMetrics holds high-frequency APF queue state and drop counters.
// Padded to 64 bytes (the L1/L2/L3 cache line boundary on modern x86-64 and ARM64 CPUs)
// to eliminate false sharing (MESI/MOESI cache invalidation storms) across concurrent polling goroutines.
type PaddedAPFMetrics struct {
	CurrentInQueue   uint64   // 8 bytes
	CurrentExecuting uint64   // 8 bytes
	ConcurrencyLimit uint64   // 8 bytes
	DropCount        uint64   // 8 bytes
	StarvationAlerts uint64   // 8 bytes
	_                [24]byte // 5 * 8 bytes = 40 bytes; 40 + 24 = 64 bytes.
}

// PaddedCompactionMetrics holds high-frequency watch cache compaction and relist metrics.
// Padded to 64 bytes to eliminate cache-line bouncing under concurrent watch reconnect storms.
type PaddedCompactionMetrics struct {
	Compacted410Errors uint64   // 8 bytes
	PaginatedChunks    uint64   // 8 bytes
	JitterWaitsTotal   uint64   // 8 bytes
	LastRelistMicros   uint64   // 8 bytes
	_                  [32]byte // 4 * 8 bytes = 32 bytes; 32 + 32 = 64 bytes.
}

// APFQueueStatus captures real-time queue depth and concurrency saturation for an APF priority level.
type APFQueueStatus struct {
	PriorityLevel    string `json:"priority_level"`
	CurrentInQueue   int    `json:"current_in_queue"`
	CurrentExecuting int    `json:"current_executing"`
	ConcurrencyLimit int    `json:"concurrency_limit"`
	DropCount        uint64 `json:"drop_count"`
	IsStarved        bool   `json:"is_starved"`
}

// NewAPFQueueStatus initializes an APFQueueStatus and evaluates starvation invariants.
func NewAPFQueueStatus(priorityLevel string, inQueue, executing, limit int, dropCount uint64) APFQueueStatus {
	// system-leader-election must NEVER queue. Any queue depth > 0 is considered starved.
	isLeaderLevel := strings.EqualFold(priorityLevel, "system-leader-election")
	isStarved := false

	if isLeaderLevel {
		isStarved = inQueue > 0 || dropCount > 0
	} else {
		// Workload and general priority levels are starved when queue fills to capacity or drops occur.
		isStarved = (limit > 0 && inQueue >= limit) || dropCount > 0
	}

	return APFQueueStatus{
		PriorityLevel:    priorityLevel,
		CurrentInQueue:   inQueue,
		CurrentExecuting: executing,
		ConcurrencyLimit: limit,
		DropCount:        dropCount,
		IsStarved:        isStarved,
	}
}

// WatchCacheCompactionPolicy defines jitter and chunking parameters for mitigating 410 Gone thundering herds.
type WatchCacheCompactionPolicy struct {
	MaxJitterWindow time.Duration `json:"max_jitter_window"`
	BatchSize       int           `json:"batch_size"`
	CompactionAge   time.Duration `json:"compaction_age"`
}

// DefaultWatchCacheCompactionPolicy returns safe production defaults.
func DefaultWatchCacheCompactionPolicy() WatchCacheCompactionPolicy {
	return WatchCacheCompactionPolicy{
		MaxJitterWindow: 60 * time.Second,
		BatchSize:       500,
		CompactionAge:   5 * time.Minute,
	}
}

// Validate ensures compaction policy parameters meet safety invariants.
func (p WatchCacheCompactionPolicy) Validate() error {
	if p.MaxJitterWindow <= 0 {
		return errors.New("max_jitter_window must be greater than zero")
	}
	if p.BatchSize <= 0 {
		return errors.New("batch_size must be greater than zero")
	}
	if p.CompactionAge <= 0 {
		return errors.New("compaction_age must be greater than zero")
	}
	return nil
}

// HPAAutoscaleGuardReport summarizes the health of the metrics API and any fallback scaling decisions.
type HPAAutoscaleGuardReport struct {
	MetricsAPIAvailable bool      `json:"metrics_api_available"`
	LastSampleTime      time.Time `json:"last_sample_time"`
	FallbackTriggered   bool      `json:"fallback_triggered"`
	TargetReplicas      int32     `json:"target_replicas"`
	CurrentReplicas     int32     `json:"current_replicas"`
	Message             string    `json:"message,omitempty"`
}

// String provides a human-readable representation of the autoscale guard report.
func (r HPAAutoscaleGuardReport) String() string {
	return fmt.Sprintf("HPA Report: Available=%t, Fallback=%t, Target=%d, Current=%d, Msg=%s",
		r.MetricsAPIAvailable, r.FallbackTriggered, r.TargetReplicas, r.CurrentReplicas, r.Message)
}

// K8sControlPlaneResilienceEngine defines the core resilience interface for APF queue optimization,
// watch cache 410 thundering herd mitigation, and HPA fallback scaling.
type K8sControlPlaneResilienceEngine interface {
	// AuditAPFQueueStatus audits current queue depths, drop counters, and starvation across all priority levels.
	AuditAPFQueueStatus(ctx context.Context) ([]APFQueueStatus, error)

	// MitigateWatchCacheCompactionThunderingHerd applies jittered backoff and paginated relisting to recover from 410 Gone.
	MitigateWatchCacheCompactionThunderingHerd(ctx context.Context, policy WatchCacheCompactionPolicy) error

	// CheckHPAHealthAndApplyFallback probes metrics-server health and executes fallback replica scaling if down >2m.
	CheckHPAHealthAndApplyFallback(ctx context.Context, namespace, deployment string) (*HPAAutoscaleGuardReport, error)
}
