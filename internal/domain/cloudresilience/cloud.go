// Package cloudresilience provides domain contracts, telemetry data structures,
// and hardware-aware resilience models for AWS/cloud burst credit exhaustion,
// spot instance 120s interruption drainage, and NAT Gateway SNAT port saturation shields.
package cloudresilience

import (
	"context"
	"errors"
	"strings"
	"time"
)

/*
ENGINEERING NOTE: PHYSICAL HARDWARE BOTTLENECKS & CLOUD RESILIENCE FAILURE MODES

1. PHYSICAL HARDWARE & INFRASTRUCTURE BOTTLENECKS:
- AWS EBS gp2 I/O Burst Credit Token Bucket Exhaustion:
  Legacy General Purpose SSD (gp2) volumes utilize an I/O credit bucket architecture. Each volume
  begins with an initial credit balance of 5.4 million I/O credits (allowing up to 3,000 IOPS for
  30 minutes). Volumes under 1,000 GiB accumulate credits at a baseline rate of 3 IOPS per GiB
  (minimum 100 IOPS baseline for a small volume). When sustained database write workloads (WAL flushes,
  PostgreSQL checkpoints, Kafka log appends) exceed baseline accumulation, the credit bucket drains.
  Upon reaching 0% burst balance, volume throughput immediately collapses from 3,000 IOPS to the
  bare baseline (e.g., 300 IOPS for a 100 GiB volume), causing disk latency to spike from sub-millisecond
  to several seconds (>5,000ms).
  gp3 volumes decouple IOPS and throughput from volume size, providing an architectural baseline of
  3,000 IOPS and 125 MB/s provisioned throughput out of the box without any burst bucket depletion hazard.

- Cloud Metadata Polling Overhead (IMDSv2 / IMDSv1):
  Querying instance metadata (e.g., http://169.254.169.254/latest/meta-data/spot/instance-action)
  requires link-local IPv4 HTTP round-trips. With IMDSv2 session token acquisition (`PUT /api/token`
  with X-aws-ec2-metadata-token-ttl-seconds) and polling frequencies faster than 5 seconds across hundreds
  of cluster nodes or microservice containers, the link-local network bridge and kernel socket table
  incur serialization latency and TCP connection setup overhead. Telemetry polling loops must balance
  detection responsiveness against host kernel socket churn.

- Linux TCP SNAT Socket Table Limits (64,512 Ports per NAT Gateway IP):
  A standard AWS VPC NAT Gateway allocates ephemeral TCP/UDP ports from 1024 to 65535, yielding
  exactly 64,512 concurrent source ports per assigned Elastic IP. Outbound TCP connections to identical
  destination IP and port combinations (such as external APIs, payment gateways, or third-party SaaS)
  must each acquire a unique source port. When active connections to the same target endpoint approach
  64,512, the NAT Gateway experiences port allocation exhaustion.

- Cloud API DescribeInstances / CloudWatch API Rate-Limit Throttling:
  AWS control plane APIs enforce token-bucket rate limits per region/account (e.g., EC2 DescribeInstances,
  ModifyVolume, and CloudWatch GetMetricData). High-concurrency cluster-wide status polling storms
  trigger `RequestLimitExceeded` (HTTP 400/503) errors. Exponential backoff, jitter, and localized
  cache-line telemetry tracking are mandatory to isolate control-plane polling from provider API saturation.

2. REAL-WORLD FAILURE MODES:
- EBS Volume Lockup Crashing Databases:
  When an EBS gp2 volume's burst balance hits 0%, storage operations queue in the host I/O scheduler.
  File system write queues (ext4/xfs journal transactions) stall. Linux kernel watchdogs fire hung
  task panics (`INFO: task kworker/... blocked for more than 120 seconds`), and database engines
  (PostgreSQL, MySQL InnoDB) trigger suicide crashes or enter read-only mode to prevent write corruption.
  Proactive detection flags volumes when burst balance drops below 20%, recommending non-disruptive
  online Elastic Volumes migration to gp3.

- Spot Instance Abrupt Termination Without Graceful Node Drain:
  Cloud providers issue a 120-second termination notice via instance metadata before reclaiming spot
  nodes. If unhandled, the hypervisor abruptly pulls physical power after 120s, severing open TCP
  connections, terminating long-running batch jobs, and corrupting in-flight distributed transactions.
  The resilience engine immediately captures the notice, cordons the Kubernetes node to halt new pods,
  and initiates an aggressive 90-second pod eviction cycle (leaving a 30s safety buffer), respecting
  PodDisruptionBudgets (PDB) while bypassing non-terminating DaemonSets.

- NAT Gateway Silent SYN Packet Drops & Cascading Timeouts:
  When NAT Gateway SNAT port allocation is exhausted, outbound TCP SYN packets are silently discarded
  at the gateway without sending TCP RST back to the client. Workloads experience hanging connections,
  default 60-second connection timeout exceptions, cascading retry storms, and thread pool exhaustion
  across backend microservices. Shielding requires monitoring active connection limits and port allocation
  errors, alerting for additional secondary private/public IPs or Route 53 DNS sharding before saturation.
*/

// Standard sentinel errors for cloud infrastructure resilience operations.
var (
	// ErrCloudSpotTerminationDraining indicates a spot termination notice is actively draining node workloads.
	ErrCloudSpotTerminationDraining = errors.New("spot instance interruption notice active: node draining in progress")

	// ErrCloudEBSBurstExhausted indicates an EBS volume burst balance has completely depleted (0%).
	ErrCloudEBSBurstExhausted = errors.New("ebs burst balance depleted: critical volume i/o lockup risk")

	// ErrNATGatewaySaturated indicates NAT Gateway active connections or port errors exceed safe capacity.
	ErrNATGatewaySaturated = errors.New("nat gateway snat port allocation saturated: active connections exceed ip capacity")

	// ErrInvalidVolumeID indicates an empty or invalid volume identifier.
	ErrInvalidVolumeID = errors.New("volume id must not be empty")

	// ErrInvalidBurstPercentage indicates burst percentage is out of range [0.0, 100.0].
	ErrInvalidBurstPercentage = errors.New("burst balance percentage must be between 0.0 and 100.0")

	// ErrInvalidInstanceID indicates an empty or invalid cloud instance identifier.
	ErrInvalidInstanceID = errors.New("instance id must not be empty")

	// ErrInvalidGatewayID indicates an empty or invalid NAT Gateway identifier.
	ErrInvalidGatewayID = errors.New("gateway id must not be empty")

	// ErrInvalidAllocatedIPs indicates allocated IPs is less than 1.
	ErrInvalidAllocatedIPs = errors.New("allocated ips must be at least 1")
)

// PortsPerNATIP defines the theoretical max ephemeral port count per NAT IP (1024-65535).
const PortsPerNATIP = 64512

// PaddedBurstTracker holds high-frequency EBS volume burst balance and IOPS metrics.
// Padded to 64 bytes (the L1/L2/L3 cache line boundary on modern x86-64 and ARM64 CPUs)
// to eliminate false sharing (MESI/MOESI cache invalidation storms) across concurrent polling goroutines.
type PaddedBurstTracker struct {
	BalancePermille uint64 // Burst balance stored as permille (e.g. 850 = 85.0%)
	CurrentIOPS     uint64
	AuditCount      uint64
	DepletionAlerts uint64
	_               [32]byte // 4 * 8 bytes = 32 bytes; 32 + 32 = 64 bytes.
}

// PaddedSpotTimer holds spot termination timestamps and eviction counters.
// Padded to 64 bytes to prevent cache-line contention between metadata pollers and drain watchers.
type PaddedSpotTimer struct {
	NoticeReceivedUnixSec uint64
	DrainDeadlineUnixSec  uint64
	TotalNotices          uint64
	SuccessfulDrains      uint64
	_                     [32]byte // 4 * 8 bytes = 32 bytes; 32 + 32 = 64 bytes.
}

// PaddedSNATPortCounter holds high-frequency NAT Gateway connection and allocation counters.
// Padded to 64 bytes to eliminate false sharing between gateway telemetry collectors.
type PaddedSNATPortCounter struct {
	ActiveConnections    uint64
	PortAllocationErrors uint64
	SaturationAlerts     uint64
	TotalQueries         uint64
	_                    [32]byte // 4 * 8 bytes = 32 bytes; 32 + 32 = 64 bytes.
}

// StorageBurstStatus represents the point-in-time I/O burst credit telemetry of a cloud block storage volume.
type StorageBurstStatus struct {
	VolumeID        string  `json:"volume_id"`
	VolumeType      string  `json:"volume_type"`
	BurstBalancePct float64 `json:"burst_balance_pct"`
	CurrentIOPS     int64   `json:"current_iops"`
	NeedsUpgrade    bool    `json:"needs_upgrade"`
}

// NewStorageBurstStatus constructs a StorageBurstStatus with invariant validation and upgrade recommendations.
func NewStorageBurstStatus(volumeID, volumeType string, burstPct float64, currentIOPS int64) (*StorageBurstStatus, error) {
	if strings.TrimSpace(volumeID) == "" {
		return nil, ErrInvalidVolumeID
	}
	if burstPct < 0.0 || burstPct > 100.0 {
		return nil, ErrInvalidBurstPercentage
	}

	needsUpgrade := false
	if strings.EqualFold(volumeType, "gp2") && burstPct < 20.0 {
		needsUpgrade = true
	}

	status := &StorageBurstStatus{
		VolumeID:        strings.TrimSpace(volumeID),
		VolumeType:      strings.ToLower(strings.TrimSpace(volumeType)),
		BurstBalancePct: burstPct,
		CurrentIOPS:     currentIOPS,
		NeedsUpgrade:    needsUpgrade,
	}

	if strings.EqualFold(volumeType, "gp2") && burstPct <= 0.0 {
		return status, ErrCloudEBSBurstExhausted
	}

	return status, nil
}

// SpotTerminationEvent records a spot instance interruption notice received from cloud metadata.
type SpotTerminationEvent struct {
	InstanceID   string    `json:"instance_id"`
	ActionTime   time.Time `json:"action_time"`
	DrainStarted bool      `json:"drain_started"`
	DrainSuccess bool      `json:"drain_success"`
}

// NewSpotTerminationEvent initializes a new spot interruption event.
func NewSpotTerminationEvent(instanceID string, actionTime time.Time) (*SpotTerminationEvent, error) {
	if strings.TrimSpace(instanceID) == "" {
		return nil, ErrInvalidInstanceID
	}
	if actionTime.IsZero() {
		actionTime = time.Now().UTC()
	}
	return &SpotTerminationEvent{
		InstanceID:   strings.TrimSpace(instanceID),
		ActionTime:   actionTime,
		DrainStarted: false,
		DrainSuccess: false,
	}, nil
}

// NATGatewayMetrics represents real-time SNAT port utilization and allocation health.
type NATGatewayMetrics struct {
	GatewayID            string `json:"gateway_id"`
	AllocatedIPs         int    `json:"allocated_ips"`
	ActiveConnections    int64  `json:"active_connections"`
	PortAllocationErrors int64  `json:"port_allocation_errors"`
	IsSaturated          bool   `json:"is_saturated"`
}

// NewNATGatewayMetrics constructs NATGatewayMetrics and evaluates port exhaustion thresholds.
func NewNATGatewayMetrics(gatewayID string, allocatedIPs int, activeConnections, portAllocationErrors int64) (*NATGatewayMetrics, error) {
	if strings.TrimSpace(gatewayID) == "" {
		return nil, ErrInvalidGatewayID
	}
	if allocatedIPs < 1 {
		return nil, ErrInvalidAllocatedIPs
	}

	totalCapacity := int64(allocatedIPs * PortsPerNATIP)
	isSaturated := activeConnections >= totalCapacity || portAllocationErrors > 0

	metrics := &NATGatewayMetrics{
		GatewayID:            strings.TrimSpace(gatewayID),
		AllocatedIPs:         allocatedIPs,
		ActiveConnections:    activeConnections,
		PortAllocationErrors: portAllocationErrors,
		IsSaturated:          isSaturated,
	}

	if isSaturated {
		return metrics, ErrNATGatewaySaturated
	}

	return metrics, nil
}

// CloudResilienceEngine defines the domain interface for cloud burst, spot, and NAT Gateway resilience.
type CloudResilienceEngine interface {
	CheckEBSBurstBalance(ctx context.Context, volumeID string) (*StorageBurstStatus, error)
	UpgradeVolumeToGP3(ctx context.Context, volumeID string) error
	ListenSpotTerminationNotices(ctx context.Context, handler func(event SpotTerminationEvent)) error
	DrainNodeGracefully(ctx context.Context, nodeName string, gracePeriod time.Duration) error
	MonitorNATGatewaySNAT(ctx context.Context, gatewayID string) (*NATGatewayMetrics, error)
}
