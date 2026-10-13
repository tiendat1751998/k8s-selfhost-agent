// Package k8sresilience provides domain contracts and resilience models for Kubernetes
// control plane self-healing, admission webhook deadlock breaking, PLEG watchdogging,
// and terminating namespace orphaned finalizer reclamation.
package k8sresilience

import (
	"context"
	"errors"
	"strings"
	"time"
)

/*
ENGINEERING NOTE: PHYSICAL HARDWARE BOTTLENECKS & KUBERNETES FAILURE MODES

1. PHYSICAL HARDWARE BOTTLENECKS:
- Kube-Apiserver etcd Serialization Overhead:
  The kube-apiserver serializes and deserializes every resource definition (JSON/Protocol Buffers)
  into etcd's bbolt B+ tree database. Under severe cluster mutation storms or mass node reboot cycles,
  etcd raft log syncs to non-volatile block storage (fsync latency spikes >10ms) saturate the
  apiserver admission chain. Each validating or mutating admission webhook injects sequential or
  parallel HTTPS round-trips over the pod overlay network. When admission latency exceeds client
  timeouts, kube-apiserver worker threads block, exhausting the max-requests-inflight semaphore
  and freezing all control plane write paths.

- Containerd UNIX Domain Socket CRI Latency:
  The Kubelet interacts with the container runtime (containerd) via CRI (Container Runtime Interface)
  over local UNIX domain sockets (/run/containerd/containerd.sock). Heavy local I/O churn, rapid
  cgroup teardowns, or bursty container initialization cause socket queue depth saturation and
  gRPC connection queue backpressure. Serialized cri-service calls block Kubelet runtime workers,
  preventing state observation and status posting.

- VFS Lockup During ListContainers & Inode Thrashing:
  During high-churn workload scheduling or memory exhaustion (OOM cascades), the kernel Virtual File
  System (VFS) dcache lock contention and overlayfs inode lookups during `ListContainers` or
  `ContainerStatus` CRI calls cause containerd shims and task daemons to enter uninterruptible sleep
  (TASK_UNINTERRUPTIBLE / 'D' state). Because D-state processes cannot be interrupted or signaled,
  container runtime calls hang indefinitely, halting Kubelet status reconciliation loops.

- Network Partition Delays Between Apiserver and Webhook Pods:
  In self-hosted or bare-metal Kubernetes architectures, webhook pods typically run on worker nodes
  routed via kube-proxy iptables/IPVS or eBPF tunnels (VXLAN, Geneve, or WireGuard). When CNI nodes
  experience network partitions, MTU blackholing, or packet drops between control-plane masters and
  worker nodes, admission webhook calls timeout (default 10s-30s).

2. REAL-WORLD FAILURE MODES:
- Apiserver Admission Webhook Deadlock (Chicken-and-Egg Cold Start):
  When webhooks are configured with `failurePolicy: Fail` (e.g., Calico, cert-manager, Cilium, or
  security enforcers) and the cluster undergoes a rolling restart, power outage, or node crash,
  webhook server pods are offline. When the scheduler attempts to launch the webhook pod, the
  kube-apiserver invokes the admission webhook for the new pod creation. Because the webhook endpoint
  is unreachable and failurePolicy is Fail, the apiserver rejects the pod creation request! The webhook
  pods can never start because they are waiting on themselves to admit themselves.
  Remediation requires temporarily breaking the deadlock by patching `failurePolicy: Ignore` until
  pods achieve Ready state, then restoring `failurePolicy: Fail`.

- Namespaces Stuck in "Terminating" Due to Orphaned Finalizers:
  When namespaces are marked for deletion, the Kubernetes namespace lifecycle controller waits for
  all custom resources and finalizers to be cleared before finalizing the namespace. If a Custom
  Resource Definition (CRD) or an external APIService (e.g., aggregated API) is deleted before its
  custom resources are purged, the respective operator no longer exists to execute its finalizer hook.
  The namespace remains trapped in `Terminating` for days or weeks. Safe emergency reclamation requires
  discovering orphaned CRD finalizers and patching the namespace metadata to release the resource.

- Kubelet PLEG Relist Timeouts (>3m) Triggering Cascading Eviction Storms:
  The Pod Lifecycle Event Generator (PLEG) within Kubelet periodically relists container states
  (`GenericPLEG.relist()`) every 1s by querying the runtime over the CRI socket. If containerd hangs
  due to VFS lockup or socket starvation and the relist cycle exceeds 3 minutes (180s), Kubelet
  reports `PLEG is not healthy: pleg was last seen active ...`. The Node Controller marks the node
  `NotReady` after node-monitor-grace-period (40s), and the Pod Eviction Controller forcibly terminates
  workloads and reschedules them to remaining nodes. This concentrates load on the surviving nodes,
  causing them to experience CRI timeouts as well, triggering a cascading cluster-wide outage.
  Proactive PLEG watchdogging detects containerd stalling before node eviction occurs and executes
  controlled CRI daemon recycling.
*/

// Standard sentinel errors for control plane resilience operations.
var (
	// ErrK8sWebhookDeadlock is returned when admission webhooks reject pod scheduling while webhook pods are unready.
	ErrK8sWebhookDeadlock = errors.New("k8s admission webhook deadlock detected: failurePolicy is Fail while webhook pods are unready")

	// ErrK8sPLEGTimeout is returned when Kubelet PLEG relist cycle exceeds the critical 180s threshold.
	ErrK8sPLEGTimeout = errors.New("kubelet pleg relist timeout exceeded 180s threshold: containerd unresponsive")

	// ErrFinalizerReapFailed is returned when an orphaned finalizer cannot be safely cleared.
	ErrFinalizerReapFailed = errors.New("failed to reap orphaned finalizer from terminating namespace")

	// ErrWebhookNotFound is returned when the target webhook configuration is not found.
	ErrWebhookNotFound = errors.New("webhook configuration not found")

	// ErrNodeNotFound is returned when the target node cannot be located in the cluster.
	ErrNodeNotFound = errors.New("node not found")

	// ErrNamespaceNotStuck is returned when finalizer reaping is requested on a namespace that is not stuck.
	ErrNamespaceNotStuck = errors.New("namespace is not stuck in terminating state")
)

// PaddedWebhookMetrics holds high-frequency admission webhook audit and failure metrics.
// Padded to 64 bytes (the L1/L2/L3 cache line boundary on modern x86-64 and ARM64 CPUs)
// to eliminate false sharing (MESI/MOESI cache invalidation storms) across concurrent polling goroutines.
type PaddedWebhookMetrics struct {
	ConsecutiveFailures uint64
	TotalAudits         uint64
	BypassCount         uint64
	RestoreCount        uint64
	LastLatencyMicros   uint64
	_                   [24]byte // 5 * 8 bytes = 40 bytes; 40 + 24 = 64 bytes.
}

// PaddedPLEGMetrics holds high-frequency PLEG timing metrics and node health monitors.
// Padded to 64 bytes to eliminate false sharing between node status polling threads.
type PaddedPLEGMetrics struct {
	RelistDurationMicros uint64
	TimeoutCount         uint64
	RestartCount         uint64
	HealthyChecks        uint64
	_                    [32]byte // 4 * 8 bytes = 32 bytes; 32 + 32 = 64 bytes.
}

// WebhookHealthStatus captures the real-time operational and admission state of a webhook.
type WebhookHealthStatus struct {
	Name                string        `json:"name"`
	Type                string        `json:"type"`           // "mutating" or "validating"
	FailurePolicy       string        `json:"failure_policy"` // "Fail" or "Ignore"
	EndpointURL         string        `json:"endpoint_url"`
	ConsecutiveFailures uint64        `json:"consecutive_failures"`
	LastLatency         time.Duration `json:"last_latency"`
	IsDeadlocked        bool          `json:"is_deadlocked"`
	PodsReady           bool          `json:"pods_ready"`
}

// NewWebhookHealthStatus initializes a WebhookHealthStatus with deadlock invariant evaluation.
func NewWebhookHealthStatus(
	name string,
	webhookType string,
	failurePolicy string,
	endpointURL string,
	failures uint64,
	lastLatency time.Duration,
	podsReady bool,
) WebhookHealthStatus {
	// A webhook is in a deadlocked state when failurePolicy is "Fail",
	// webhook backing pods are NOT ready, and consecutive probe failures >= 5.
	isDeadlocked := strings.EqualFold(failurePolicy, "Fail") && !podsReady && failures >= 5

	return WebhookHealthStatus{
		Name:                name,
		Type:                strings.ToLower(webhookType),
		FailurePolicy:       failurePolicy,
		EndpointURL:         endpointURL,
		ConsecutiveFailures: failures,
		LastLatency:         lastLatency,
		IsDeadlocked:        isDeadlocked,
		PodsReady:           podsReady,
	}
}

// PLEGStatus represents Kubelet PLEG health and containerd responsiveness for a node.
type PLEGStatus struct {
	NodeName             string        `json:"node_name"`
	RelistDuration       time.Duration `json:"relist_duration"`
	IsHealthy            bool          `json:"is_healthy"`
	ContainerdResponsive bool          `json:"containerd_responsive"`
	DiagnosedAt          time.Time     `json:"diagnosed_at"`
	ActionRequired       string        `json:"action_required,omitempty"`
}

// NewPLEGStatus constructs a PLEGStatus and evaluates whether containment/recovery is needed.
func NewPLEGStatus(nodeName string, relistDuration time.Duration, containerdResponsive bool) PLEGStatus {
	// Kubelet PLEG relist threshold is critically exceeded if relist duration > 180s (3m)
	// or if containerd is completely unresponsive over the CRI UNIX socket.
	isHealthy := relistDuration <= 180*time.Second && containerdResponsive

	action := ""
	if !isHealthy {
		if !containerdResponsive {
			action = "containerd_restart_recommended"
		} else {
			action = "investigate_vfs_dcache_pressure"
		}
	}

	return PLEGStatus{
		NodeName:             nodeName,
		RelistDuration:       relistDuration,
		IsHealthy:            isHealthy,
		ContainerdResponsive: containerdResponsive,
		DiagnosedAt:          time.Now().UTC(),
		ActionRequired:       action,
	}
}

// NamespaceFinalizerReport details the diagnosis and cleanup of stuck terminating namespaces.
type NamespaceFinalizerReport struct {
	Namespace           string        `json:"namespace"`
	Phase               string        `json:"phase"`
	TerminatingDuration time.Duration `json:"terminating_duration"`
	ReapedFinalizers    []string      `json:"reaped_finalizers"`
	ReapedAt            time.Time     `json:"reaped_at"`
	Message             string        `json:"message"`
}

// ControlPlaneWatchdog defines the core resilience interface for detecting and breaking
// control plane deadlocks, re-enabling pod scheduling, and guarding against node eviction storms.
type ControlPlaneWatchdog interface {
	// AuditWebhooks inspects all Mutating and Validating webhook configurations in the cluster.
	AuditWebhooks(ctx context.Context) ([]WebhookHealthStatus, error)

	// EmergencyBypassDeadlockedWebhook patches a deadlocked webhook's failurePolicy to Ignore.
	EmergencyBypassDeadlockedWebhook(ctx context.Context, webhookName string) error

	// ReapOrphanedFinalizers purges blocking finalizers from a namespace stuck in Terminating > 10m.
	ReapOrphanedFinalizers(ctx context.Context, namespace string) error

	// CheckPLEGHealth checks whether a node's Kubelet PLEG relist cycle is healthy.
	CheckPLEGHealth(ctx context.Context, nodeName string) (*PLEGStatus, error)

	// RestartStalledContainerd executes a graceful containerd daemon recycle on the target node.
	RestartStalledContainerd(ctx context.Context, nodeName string) error
}
