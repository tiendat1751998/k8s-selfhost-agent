// Package netscaler encapsulates kernel-level socket backlog, TCP TIME_WAIT recycling,
// and conntrack table scalability domain models, telemetry structures, and invariant guardrails.
//
// ENGINEERING NOTE: Physical Hardware Bottlenecks & Network Kernel Failure Modes
// --------------------------------------------------------------------------------------------------
// 1. Kernel Socket Memory Overhead & SLAB Allocation:
//    Every active TCP stream instantiates an in-kernel `struct sock` plus bidirectional socket ring buffers
//    composed of `sk_buff` structs. An established socket consumes ~2-4 KB of pinned, non-pageable kernel
//    memory. Under high connection churn (e.g. unpooled HTTP clients), socket structures exert severe pressure
//    on Linux SLAB/SLUB caches (`kmalloc-512`, `TCPv6`, `sock_inode_cache`). Uncontrolled socket proliferation
//    induces kernel Out-Of-Memory (OOM) panics well before container RSS limits trigger cgroup kills.
//
// 2. TCP SYN Queue Memory Pressure & Silent Listen Backlog Drops:
//    Connection establishment undergoes two kernel queues: the half-open SYN queue (governed by
//    `net.ipv4.tcp_max_syn_backlog`) and the completed 3-way handshake accept queue (`net.core.somaxconn`).
//    When bursts saturate either backlog, the kernel drops incoming SYN or ACK frames silently (unless
//    `tcp_abort_on_overflow=1`). Clients enter exponential backoff retransmission loops (initial RTO 1s,
//    escalating to 3s, 9s, 21s), leading to perceived cluster-wide micro-outages and P99 tail latency spikes.
//
// 3. Netfilter Conntrack Table Saturation (`nf_conntrack`):
//    Stateful NAT, iptables, and Kubernetes Service routing rely on netfilter connection tracking.
//    Each conntrack tuple entry allocates ~320 bytes within the `nf_conntrack` slab cache. When
//    `ConntrackEntries >= ConntrackMax`, netfilter logs `nf_conntrack: table full, dropping packet` and
//    indiscriminately discards ALL subsequent inbound and outbound packets. This causes instantaneous blackholing
//    of DNS resolution, inter-pod RPCs, and kubelet liveness probes.
//
// 4. Ephemeral Port Starvation in TIME_WAIT:
//    Sockets in active close remain in `TIME_WAIT` for 2 * MSL (typically 60s in Linux, or tuned via
//    `net.netfilter.nf_conntrack_tcp_timeout_time_wait`). Outbound traffic originating from nodes binds
//    ports within `net.ipv4.ip_local_port_range` (32768-60999, ~28,232 ephemeral ports). High microservice
//    turnover quickly exhausts these ports, resulting in syscall failures with `EADDRNOTAVAIL` (HTTP 503).
//    Activating `net.ipv4.tcp_tw_reuse = 1` safely allows timestamps (`tcp_timestamps = 1`) to validate
//    sequence numbers and recycle TIME_WAIT sockets for outbound connections.
//
// 5. NIC Interrupt Moderation & CPU Cache Coherency (False Sharing):
//    High packet-per-second (PPS) workloads on 10/40/100GbE interfaces generate intense softirq/NAPI activity
//    across multi-core SMP nodes. If telemetry counters share the same 64-byte L1/L2 cache line, concurrent atomic
//    increments cause bus-level cache line bouncing under the MESI/MOESI cache coherence protocols. Structuring
//    counters with 64-byte cache-line padding guarantees isolated L1/L2 caching without false sharing penalty.
// --------------------------------------------------------------------------------------------------
package netscaler

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync/atomic"
)

// Domain threshold constants and hard safety ceilings.
const (
	DefaultEphemeralPortMin         int64   = 32768
	DefaultEphemeralPortMax         int64   = 60999
	DefaultTotalEphemeralPorts      int64   = 28232 // 60999 - 32768 + 1
	EphemeralStarvationPctThreshold float64 = 80.0
	EphemeralStarvationCountThreshold int64 = 22000

	ConntrackSaturationThresholdPct float64 = 75.0
	ConntrackHardSafetyCeiling      int64   = 2097152 // ~670MB non-pageable RAM limit (2M * 320B)
	ConntrackDefaultInitialMax      int64   = 262144

	DefaultSomaxconnHighScale        int = 65535
	DefaultTCPMaxSynBacklogHighScale int = 65535
	DefaultTCPTwReuseEnabled         int = 1
	DefaultConntrackTCPTimeWaitSec   int = 30
)

// RFC 7807 problem detail error codes.
const (
	CodeNetEphemeralPortsExhausted = "ERR_NET_EPHEMERAL_PORTS_EXHAUSTED"
	CodeNetConntrackSaturation    = "ERR_NET_CONNTRACK_SATURATION"
	CodeKernelSysctlFailed        = "ERR_KERNEL_SYSCTL_FAILED"
	CodeInvalidNetConfig          = "ERR_INVALID_NET_CONFIG"
)

// Sentinel domain errors.
var (
	ErrNetEphemeralPortsExhausted = errors.New("ephemeral port pool exhausted: outbound connections failing with EADDRNOTAVAIL")
	ErrNetConntrackSaturation    = errors.New("kernel conntrack table saturated: silent packet drops occurring")
	ErrKernelSysctlFailed        = errors.New("failed to apply kernel sysctl configuration")
	ErrInvalidNetConfig          = errors.New("invalid kernel network configuration")
)

// PaddedCounter is a 64-bit integer counter padded to a 64-byte CPU cache line boundary
// (8 bytes data + 56 bytes padding) to eliminate false sharing across SMP cores under MESI/MOESI.
type PaddedCounter struct {
	val int64
	_   [56]byte
}

// Inc atomically increments the counter by 1 and returns the new value.
func (c *PaddedCounter) Inc() int64 {
	return atomic.AddInt64(&c.val, 1)
}

// Add atomically increments the counter by delta and returns the new value.
func (c *PaddedCounter) Add(delta int64) int64 {
	return atomic.AddInt64(&c.val, delta)
}

// Load atomically retrieves the current value of the counter.
func (c *PaddedCounter) Load() int64 {
	return atomic.LoadInt64(&c.val)
}

// Store atomically sets the value of the counter.
func (c *PaddedCounter) Store(v int64) {
	atomic.StoreInt64(&c.val, v)
}

// CoreSocketTelemetry aggregates high-frequency socket and netfilter counters
// with each counter aligned to an isolated cache line.
type CoreSocketTelemetry struct {
	TimeWaitCounter       PaddedCounter
	AllocatedPortCounter  PaddedCounter
	ListenDropCounter     PaddedCounter
	ConntrackEntryCounter PaddedCounter
}

// SocketMetrics represents a point-in-time snapshot of Linux kernel socket and conntrack metrics.
type SocketMetrics struct {
	TCPTimeWaitCount    int64   `json:"tcp_time_wait_count"`
	AllocatedPorts      int64   `json:"allocated_ports"`
	TotalEphemeralPorts int64   `json:"total_ephemeral_ports"`
	PortUtilizationPct  float64 `json:"port_utilization_pct"`
	ListenBacklogDrops  int64   `json:"listen_backlog_drops"`
	ConntrackEntries    int64   `json:"conntrack_entries"`
	ConntrackMax        int64   `json:"conntrack_max"`
	ConntrackUsagePct   float64 `json:"conntrack_usage_pct"`
}

// NewSocketMetrics validates inputs and constructs an immutable SocketMetrics value object.
func NewSocketMetrics(twCount, allocPorts, totalPorts, backlogDrops, ctEntries, ctMax int64) (*SocketMetrics, error) {
	if twCount < 0 {
		return nil, fmt.Errorf("%w: tcp_time_wait_count cannot be negative (%d)", ErrInvalidNetConfig, twCount)
	}
	if allocPorts < 0 {
		return nil, fmt.Errorf("%w: allocated_ports cannot be negative (%d)", ErrInvalidNetConfig, allocPorts)
	}
	if totalPorts <= 0 {
		return nil, fmt.Errorf("%w: total_ephemeral_ports must be positive (%d)", ErrInvalidNetConfig, totalPorts)
	}
	if backlogDrops < 0 {
		return nil, fmt.Errorf("%w: listen_backlog_drops cannot be negative (%d)", ErrInvalidNetConfig, backlogDrops)
	}
	if ctEntries < 0 {
		return nil, fmt.Errorf("%w: conntrack_entries cannot be negative (%d)", ErrInvalidNetConfig, ctEntries)
	}
	if ctMax <= 0 {
		return nil, fmt.Errorf("%w: conntrack_max must be positive (%d)", ErrInvalidNetConfig, ctMax)
	}

	portUtil := (float64(allocPorts) / float64(totalPorts)) * 100.0
	portUtil = math.Round(portUtil*100) / 100

	ctUsage := (float64(ctEntries) / float64(ctMax)) * 100.0
	ctUsage = math.Round(ctUsage*100) / 100

	return &SocketMetrics{
		TCPTimeWaitCount:    twCount,
		AllocatedPorts:      allocPorts,
		TotalEphemeralPorts: totalPorts,
		PortUtilizationPct:  portUtil,
		ListenBacklogDrops:  backlogDrops,
		ConntrackEntries:    ctEntries,
		ConntrackMax:        ctMax,
		ConntrackUsagePct:   ctUsage,
	}, nil
}

// IsPortStarved returns true if port utilization or TIME_WAIT count crosses critical thresholds.
func (m *SocketMetrics) IsPortStarved() bool {
	if m == nil {
		return false
	}
	return m.PortUtilizationPct >= EphemeralStarvationPctThreshold || m.TCPTimeWaitCount >= EphemeralStarvationCountThreshold
}

// IsConntrackSaturated returns true if conntrack table usage exceeds saturation threshold.
func (m *SocketMetrics) IsConntrackSaturated() bool {
	if m == nil {
		return false
	}
	return m.ConntrackUsagePct >= ConntrackSaturationThresholdPct
}

// HasListenDrops returns true if kernel listen backlog dropped packets.
func (m *SocketMetrics) HasListenDrops() bool {
	if m == nil {
		return false
	}
	return m.ListenBacklogDrops > 0
}

// KernelNetConfig specifies tuning parameters for sysctl network stack knobs.
type KernelNetConfig struct {
	Somaxconn            int   `json:"somaxconn"`
	TCPMaxSynBacklog     int   `json:"tcp_max_syn_backlog"`
	TCPTwReuse           int   `json:"tcp_tw_reuse"`
	ConntrackMax         int64 `json:"conntrack_max"`
	ConntrackTCPTimeWait int   `json:"conntrack_tcp_time_wait"`
}

// Validate checks configuration invariants and safety bounds.
func (cfg KernelNetConfig) Validate() error {
	if cfg.Somaxconn <= 0 {
		return fmt.Errorf("%w: somaxconn must be positive (%d)", ErrInvalidNetConfig, cfg.Somaxconn)
	}
	if cfg.TCPMaxSynBacklog <= 0 {
		return fmt.Errorf("%w: tcp_max_syn_backlog must be positive (%d)", ErrInvalidNetConfig, cfg.TCPMaxSynBacklog)
	}
	if cfg.TCPTwReuse < 0 || cfg.TCPTwReuse > 2 {
		return fmt.Errorf("%w: tcp_tw_reuse must be 0, 1, or 2 (%d)", ErrInvalidNetConfig, cfg.TCPTwReuse)
	}
	if cfg.ConntrackMax <= 0 {
		return fmt.Errorf("%w: conntrack_max must be positive (%d)", ErrInvalidNetConfig, cfg.ConntrackMax)
	}
	if cfg.ConntrackMax > ConntrackHardSafetyCeiling {
		return fmt.Errorf("%w: conntrack_max %d exceeds hard safety ceiling of %d (~670MB RAM limit)",
			ErrInvalidNetConfig, cfg.ConntrackMax, ConntrackHardSafetyCeiling)
	}
	if cfg.ConntrackTCPTimeWait < 0 {
		return fmt.Errorf("%w: conntrack_tcp_time_wait cannot be negative (%d)", ErrInvalidNetConfig, cfg.ConntrackTCPTimeWait)
	}
	return nil
}

// SocketWatchdog defines the domain port for inspecting kernel socket metrics,
// applying sysctl adjustments, and performing health audits.
type SocketWatchdog interface {
	CollectMetrics(ctx context.Context) (*SocketMetrics, error)
	ApplyKernelTuning(ctx context.Context, cfg KernelNetConfig) error
	CheckHealth(ctx context.Context) error
}
