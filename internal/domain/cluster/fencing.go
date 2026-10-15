package cluster

import (
	"context"
	"errors"
	"sync/atomic"
	"time"
)

/*
ENGINEERING NOTE: PHYSICAL HARDWARE BOTTLENECKS & MULTI-DC FAILURE MODES

1. PHYSICAL HARDWARE BOTTLENECKS:
- Cross-Datacenter Fiber Optic WAN Latency & BGP Convergence:
  Light propagation in single-mode silica optical fiber (refractive index n ≈ 1.468)
  operates at ~204,000 km/s (~4.9 µs/km). Trans-continental and inter-region round-trips
  exhibit baseline physical propagation delays of 15ms to 85ms RTT. Furthermore, BGP
  route flap dampening, upstream transit provider reconvergence, and MPLS traffic
  engineering path reconvergence during link cuts frequently introduce 30s to 90s
  of transient asymmetric routing or packet drops.
- IPMI/BMC Controller Serial-Over-LAN & Out-of-Band Bus Timeouts:
  Baseboard Management Controllers (e.g., Aspeed AST2500/AST2600 running OpenBMC or AMI MegaRAC)
  rely on low-power ARM SoCs communicating over internal I2C/SMBus/IPMB system buses.
  Under heavy chassis power events, thermal throttling, or aux-rail load spikes, BMC watchdog
  delays and ACPI power-cycle calls can block for 15s to 30s. Fencing engines must enforce
  strict multi-channel verification and out-of-band timeout bounds rather than single-path polls.
- Kernel Panic & D-State Process Hang Latency:
  Processes entering uninterruptible sleep (TASK_UNINTERRUPTIBLE / 'D' state) due to deadlocked
  NFS/Ceph mounts, broken NVMe-oF controllers, or saturated kernel locks cannot be terminated
  via POSIX signals (SIGKILL / kill -9). Software-level daemon termination is ineffective; only
  hardware-enforced STONITH (physical IPMI power-cut or hypervisor force-stop) guarantees
  absolute write isolation before replica promotion.

2. REAL-WORLD FAILURE MODES:
- GitHub October 2018 Database Split-Brain Incident:
  A 43-second optical network glitch between US-East and US-West triggered automatic database
  failover while the original primary database was still accepting writes. When replicas in the
  secondary DC were promoted prematurely, dual-master replication divergence created colliding
  auto-increment primary keys, corrupting thousands of production tables across multi-tenant shards.
- Dual-Master Replication Divergence:
  Partitioned clusters where write locks are not fenced allow concurrent transactions on both sides,
  causing silent divergence that violates ACID transaction consistency and requires manual recovery.
- Premature Failover on Transient Hiccups:
  Triggering failover during transient BGP routing reconvergences (typically 30-45s) causes
  unnecessary cluster churn, split-brain hazards, and degraded availability. A strict 60-120s
  grace period watcher is required to distinguish transient blips from hard node failures.
- IMDSv1 SSRF Cloud Credential Theft:
  Workloads on cloud-hosted Kubernetes clusters can be exploited via Server-Side Request Forgery
  (SSRF) to query the unauthenticated AWS/GCP instance metadata endpoint (169.254.169.254).
  Mandatory enforcement of IMDSv2 (session tokens via X-aws-ec2-metadata-token, hop limit = 1)
  and zero-trust Kubernetes egress NetworkPolicies prevents metadata exfiltration.
*/

// Standard sentinel errors for multi-datacenter fencing and failover safety.
var (
	// ErrFailoverPrimaryNotFenced is returned when replica promotion is requested but old primary is not fenced.
	ErrFailoverPrimaryNotFenced = errors.New("failover rejected: old primary is not confirmed fenced")
	// ErrIMDSAccessBlocked is returned when insecure IMDSv1 metadata access is detected or blocked.
	ErrIMDSAccessBlocked = errors.New("imds access blocked: insecure metadata access detected")
	// ErrGracePeriodActive is returned when failover is held during transient heartbeat recovery window.
	ErrGracePeriodActive = errors.New("failover postponed: transient grace period active")
)

// FencingMethod describes the physical or hypervisor mechanism used for STONITH.
type FencingMethod string

const (
	// IPMI_POWER_OFF physically cuts power to the chassis via out-of-band BMC IPMI/Redfish.
	IPMI_POWER_OFF FencingMethod = "IPMI_POWER_OFF"
	// NETWORK_ISOLATION isolates the node at the top-of-rack switch or hypervisor vSwitch port.
	NETWORK_ISOLATION FencingMethod = "NETWORK_ISOLATION"
	// CLOUD_INSTANCE_STOP initiates an authoritative StopInstances API call on cloud infrastructure.
	CLOUD_INSTANCE_STOP FencingMethod = "CLOUD_INSTANCE_STOP"
)

// NodeFencingReport summarizes the execution and verification of a STONITH operation.
type NodeFencingReport struct {
	TargetNodeID string        `json:"target_node_id"`
	Datacenter   string        `json:"datacenter"`
	Method       FencingMethod `json:"method"`
	IsFenced     bool          `json:"is_fenced"`
	ConfirmedAt  time.Time     `json:"confirmed_at"`
	Duration     time.Duration `json:"duration"`
	OutputLog    []string      `json:"output_log"`
}

// NewNodeFencingReport constructs a new NodeFencingReport with defensive slice copying.
func NewNodeFencingReport(targetNodeID, datacenter string, method FencingMethod, isFenced bool, confirmedAt time.Time, duration time.Duration, outputLog []string) *NodeFencingReport {
	logs := make([]string, len(outputLog))
	copy(logs, outputLog)
	return &NodeFencingReport{
		TargetNodeID: targetNodeID,
		Datacenter:   datacenter,
		Method:       method,
		IsFenced:     isFenced,
		ConfirmedAt:  confirmedAt,
		Duration:     duration,
		OutputLog:    logs,
	}
}

// FailoverReadinessStatus describes whether a cluster replica can be safely promoted to primary.
type FailoverReadinessStatus struct {
	CanPromote       bool          `json:"can_promote"`
	OldPrimaryFenced bool          `json:"old_primary_fenced"`
	ReplicationLag   time.Duration `json:"replication_lag"`
	GraceDelayPassed bool          `json:"grace_delay_passed"`
	RejectReason     string        `json:"reject_reason,omitempty"`
}

// NewFailoverReadinessStatus constructs a FailoverReadinessStatus struct.
func NewFailoverReadinessStatus(canPromote, oldPrimaryFenced bool, lag time.Duration, gracePassed bool, rejectReason string) *FailoverReadinessStatus {
	return &FailoverReadinessStatus{
		CanPromote:       canPromote,
		OldPrimaryFenced: oldPrimaryFenced,
		ReplicationLag:   lag,
		GraceDelayPassed: gracePassed,
		RejectReason:     rejectReason,
	}
}

// PaddedTelemetryCounter holds high-frequency failover telemetry counters.
type PaddedTelemetryCounter struct {
	HeartbeatChecks  uint64
	HeartbeatMisses  uint64
	FencingAttempts  uint64
	FencingSuccesses uint64
	FencingFailures  uint64
}

// HeartbeatStatus represents the health state of a node's heartbeat.
type HeartbeatStatus uint32

const (
	HeartbeatStatusHealthy HeartbeatStatus = iota
	HeartbeatStatusSuspect
	HeartbeatStatusLost
	HeartbeatStatusFenced
)

// HeartbeatStateTracker tracks node heartbeat.
type HeartbeatStateTracker struct {
	LastHeartbeatNano int64
	MissedBeats       uint64
	State             uint32
}

// NewHeartbeatStateTracker constructs an initialized HeartbeatStateTracker.
func NewHeartbeatStateTracker() *HeartbeatStateTracker {
	return &HeartbeatStateTracker{
		LastHeartbeatNano: time.Now().UnixNano(),
		MissedBeats:       0,
		State:             uint32(HeartbeatStatusHealthy),
	}
}

// GetState returns the current heartbeat status atomically.
func (h *HeartbeatStateTracker) GetState() HeartbeatStatus {
	return HeartbeatStatus(atomic.LoadUint32(&h.State))
}

// SetState updates the heartbeat status atomically.
func (h *HeartbeatStateTracker) SetState(status HeartbeatStatus) {
	atomic.StoreUint32(&h.State, uint32(status))
}

// RecordMissedBeat increments missed beat count atomically.
func (h *HeartbeatStateTracker) RecordMissedBeat() uint64 {
	return atomic.AddUint64(&h.MissedBeats, 1)
}

// GetMissedBeats retrieves the missed beats count atomically.
func (h *HeartbeatStateTracker) GetMissedBeats() uint64 {
	return atomic.LoadUint64(&h.MissedBeats)
}

// RecordHeartbeatSuccess records a successful heartbeat and resets suspect counters.
func (h *HeartbeatStateTracker) RecordHeartbeatSuccess(t time.Time) {
	atomic.StoreInt64(&h.LastHeartbeatNano, t.UnixNano())
	atomic.StoreUint64(&h.MissedBeats, 0)
	atomic.StoreUint32(&h.State, uint32(HeartbeatStatusHealthy))
}

// GetLastHeartbeatTime returns the timestamp of the last successful heartbeat.
func (h *HeartbeatStateTracker) GetLastHeartbeatTime() time.Time {
	nano := atomic.LoadInt64(&h.LastHeartbeatNano)
	if nano == 0 {
		return time.Time{}
	}
	return time.Unix(0, nano)
}

// FencingExecutionTracker tracks STONITH executions per node.
type FencingExecutionTracker struct {
	ExecutionsCount uint64
	LastDurationNs  int64
	SuccessFlag     uint32
}

// NewFencingExecutionTracker constructs an initialized FencingExecutionTracker.
func NewFencingExecutionTracker() *FencingExecutionTracker {
	return &FencingExecutionTracker{}
}

// RecordExecution stores execution metrics atomically.
func (f *FencingExecutionTracker) RecordExecution(duration time.Duration, success bool) {
	atomic.AddUint64(&f.ExecutionsCount, 1)
	atomic.StoreInt64(&f.LastDurationNs, duration.Nanoseconds())
	val := uint32(0)
	if success {
		val = 1
	}
	atomic.StoreUint32(&f.SuccessFlag, val)
}

// MultiDCFencingManager defines the business contract for multi-datacenter STONITH fencing,
// anti-split-brain replica promotion validation, and cloud IMDSv2 zero-trust security.
type MultiDCFencingManager interface {
	// ExecuteSTONITH executes Shoot-The-Other-Node-In-The-Head storage fencing on target node.
	ExecuteSTONITH(ctx context.Context, targetNodeID, dc string) (*NodeFencingReport, error)
	// ValidateFailoverReadiness guards replica promotion against split-brain scenarios.
	ValidateFailoverReadiness(ctx context.Context, clusterID string) (*FailoverReadinessStatus, error)
	// EnforceIMDSv2Security hardens cluster nodes against SSRF attacks on instance metadata (169.254.169.254).
	EnforceIMDSv2Security(ctx context.Context, clusterID string) error
}
