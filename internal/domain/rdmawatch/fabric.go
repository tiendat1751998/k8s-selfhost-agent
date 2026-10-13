// Package rdmawatch provides domain contracts, telemetry monitors, and resilience engines
// for RoCEv2 PFC deadlock detection, GPU RDMA fabric asymmetry isolation, pod IPC_LOCK admission,
// and staggered checkpoint coordination.
//
// PHYSICAL HARDWARE BOTTLENECK & FABRIC RESILIENCE ANALYSIS:
// 1. PCIe Gen4/Gen5 Bus Bandwidth & Host-to-Device Bottlenecks:
//    PCIe Gen4 x16 caps throughput at ~31.5 GB/s bidirectional (~63 GB/s for PCIe Gen5 x16).
//    When GPUDirect RDMA is bypassed or host staging buffers are used, checkpoint dumping and
//    gradient reduction crossing the host PCIe root complex saturate CPU memory channels,
//    inducing severe bus contention and PCIe AER (Advanced Error Reporting) stalls.
// 2. NVLink Interconnect Latency & Bandwidth:
//    NVLink 4.0 provides 900 GB/s bidirectional bandwidth per GPU (H100/NVSwitch 3). If any single
//    NVLink lane degrades (e.g. dropping from 900 GB/s to <700 GB/s or link errors accumulate),
//    all-to-all tensor parallelism collapses, resulting in an ~85% drop in distributed training throughput.
// 3. RoCEv2 PFC Pause Storm Propagation Across Leaf-Spine Fabrics:
//    Priority Flow Control (PFC, IEEE 802.1Qbb) operates on lossless Traffic Class 3 (DSCP 26 / CS3).
//    Under congestion, downstream leaf switches send PFC pause frames upstream. If buffer queues
//    stay saturated, pause frames propagate up to spine switches and back down to uninvolved nodes,
//    triggering a cluster-wide PFC pause storm (>10,000 pauses/sec) and deadlocking innocent traffic.
// 4. NIC Rx/Tx Ring Buffer Drops & ECN / WRED Inadequacy:
//    ConnectX-6/7/8 HCAs have finite hardware packet buffers (e.g. 512KB-4MB per port). When PFC is
//    delayed or ECN (Explicit Congestion Notification, RFC 3168) marking threshold (K_min/K_max)
//    is misconfigured, ring buffer overflow drops RoCEv2 packets. RoCEv2 lacks TCP sliding-window
//    recovery: a single dropped packet causes Go-Back-N retransmission storms at line rate (400Gbps).
// 5. Locked Memory (RLIMIT_MEMLOCK) Exhaustion:
//    RDMA kernel bypass requires pinned physical RAM pages (`ibv_reg_mr` syscall) to prevent OS paging
//    during hardware DMA transfers. If `ulimit -l` is restricted or container lacks `IPC_LOCK`,
//    PyTorch/NCCL crashes with `ibv_reg_mr failed: Cannot allocate memory` (ENOMEM).
//
// REAL-WORLD HARDWARE FAILURE MODES:
// 1. RoCEv2 Circular Buffer Deadlock:
//    In multi-rail torus or Fat-Tree topologies with multi-tenant AllGather/ReduceScatter traffic,
//    buffer queues on Switch A wait for Switch B, which waits for Switch C, which waits for Switch A.
//    PFC pause frames circle infinitely, causing total fabric deadlock stalling 1024 GPUs at 0% utilization.
// 2. PyTorch `ibv_reg_mr failed: Cannot allocate memory`:
//    Distributed training jobs crash immediately at rank initialization when memory registration exceeds
//    locked memory limits, requiring preflight validation of `IPC_LOCK` and unlimited memlock.
// 3. Asymmetric NVLink Speed Degradation:
//    A damaged cable, dirty optical connector, or degraded NVSwitch port down-negotiates one link
//    from 900 GB/s to 50 GB/s. Because NCCL AllReduce synchronizes synchronously across all ranks,
//    the entire cluster slows to the speed of the slowest link, causing an 85% throughput collapse.
// 4. CUDA Fork Deadlocks in Serving Runtimes:
//    Calling `fork()` in Python (Gunicorn/FastAPI worker multiprocessing) after initializing the CUDA
//    driver copies poisoned driver mutex states into child worker pods, causing permanent hangs on
//    the first CUDA API call.
package rdmawatch

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// RFC 7807 problem error codes for RDMA & GPU Fabric incidents.
const (
	CodeRDMAPFCDeadlock       = "ERR_RDMA_PFC_DEADLOCK"
	CodeGPUFabricAsymmetric   = "ERR_GPU_FABRIC_ASYMMETRIC"
	CodeRDMAMemlockRestricted = "ERR_RDMA_MEMLOCK_RESTRICTED"
)

// Standard domain errors.
var (
	ErrRDMAPFCDeadlock = &domainerrors.DomainError{
		Code:    CodeRDMAPFCDeadlock,
		Message: "RoCEv2 PFC circular buffer deadlock detected across RDMA fabric. High pause frame rate with zero GPU progress.",
		Err:     errors.New("rdma pfc deadlock detected"),
	}
	ErrGPUFabricAsymmetric = &domainerrors.DomainError{
		Code:    CodeGPUFabricAsymmetric,
		Message: "Asymmetric NVLink/RDMA fabric bandwidth degradation detected across GPU cluster interconnect.",
		Err:     errors.New("gpu fabric asymmetric bandwidth degradation"),
	}
	ErrRDMAMemlockRestricted = &domainerrors.DomainError{
		Code:    CodeRDMAMemlockRestricted,
		Message: "Pod RDMA preflight rejected: missing IPC_LOCK capability or unlimited memlock ulimit.",
		Err:     errors.New("rdma memlock restricted or ipc_lock missing"),
	}
)

// Thresholds and Kubernetes taint constants for fabric degradation and deadlocks.
const (
	FabricDegradedTaintKey    = "gpu.kubernetes.io/degraded-fabric"
	FabricDegradedTaintValue  = "true"
	FabricDegradedTaintEffect = "NoSchedule"
	FabricDegradedTaintString = "gpu.kubernetes.io/degraded-fabric=true:NoSchedule"

	NVLinkBaselineBandwidthGBps     = 900.0
	NVLinkDegradedThresholdGBps     = 700.0
	NVLinkStddevThresholdRatio      = 0.15 // 15% standard deviation relative to baseline
	PFCDeadlockPauseThreshold       = 10000.0 // 10,000 pauses/sec
	GPUDeadlockUtilizationThreshold = 0.0   // 0% GPU utilization
)

// GPUFabricBandwidth models bandwidth and degradation metrics for a GPU interconnect link.
type GPUFabricBandwidth struct {
	DeviceID      string  `json:"device_id"`
	NVLinkID      int     `json:"nvlink_id"`
	BandwidthGBps float64 `json:"bandwidth_gbps"`
	IsDegraded    bool    `json:"is_degraded"`
	ErrorCount    uint64  `json:"error_count"`
}

// RDMAWatchdogStatus captures point-in-time status of RoCEv2 PFC pause counters and deadlock state.
type RDMAWatchdogStatus struct {
	InterfaceName    string  `json:"interface_name"`
	PFCPausesRx      uint64  `json:"pfc_pauses_rx"`
	PFCPausesTx      uint64  `json:"pfc_pauses_tx"`
	DeadlockDetected bool    `json:"deadlock_detected"`
	MemlockUnlimited bool    `json:"memlock_unlimited"`
	PauseRateRxSec   float64 `json:"pause_rate_rx_sec,omitempty"`
	GPUUtilization   float64 `json:"gpu_utilization_pct,omitempty"`
	AlertEmitted     bool    `json:"alert_emitted,omitempty"`
	RemediationHint  string  `json:"remediation_hint,omitempty"`
}

// CheckpointWritePolicy specifies parameters for staggered distributed checkpoint writes.
type CheckpointWritePolicy struct {
	MaxConcurrentWrites int           `json:"max_concurrent_writes"`
	LocalSpoolPath      string        `json:"local_spool_path"`
	RemoteStorageTarget string        `json:"remote_storage_target"`
	MaxJitterWindow     time.Duration `json:"max_jitter_window"`
}

// GPUAIResilienceEngine defines core capabilities for RDMA fabric watchdog and GPU cluster resilience.
type GPUAIResilienceEngine interface {
	AuditGPUFabricBandwidth(ctx context.Context) ([]GPUFabricBandwidth, error)
	DetectPFCDeadlock(ctx context.Context, nicName string) (*RDMAWatchdogStatus, error)
	ValidatePodRDMAPreflight(ctx context.Context, podYAML []byte) (isValid bool, reason string, err error)
	CoordinateStaggeredCheckpoint(ctx context.Context, rankID, totalRanks int, policy CheckpointWritePolicy) (delay time.Duration, err error)
}

// PaddedPFCAccumulator features 64-byte cache-line padding to prevent false sharing
// across multi-core CPUs under high-rate RoCEv2 PFC pause frame tracking (MESI/MOESI protocol).
// Memory layout: 3x uint64 (24 bytes) + 40 bytes pad = 64 bytes (1 L1 cache line).
type PaddedPFCAccumulator struct {
	rxPauses         atomic.Uint64
	txPauses         atomic.Uint64
	sampleDurationMs atomic.Uint64
	_                [40]byte // 64-byte cache line alignment padding
}

// Record atomically updates PFC pause frame accumulators.
func (a *PaddedPFCAccumulator) Record(rx, tx uint64, durationMs uint64) {
	a.rxPauses.Store(rx)
	a.txPauses.Store(tx)
	a.sampleDurationMs.Store(durationMs)
}

// GetRxPauses returns the current Rx pause count.
func (a *PaddedPFCAccumulator) GetRxPauses() uint64 {
	return a.rxPauses.Load()
}

// GetTxPauses returns the current Tx pause count.
func (a *PaddedPFCAccumulator) GetTxPauses() uint64 {
	return a.txPauses.Load()
}

// GetRxRateSec returns the pause rate in frames per second based on stored sample duration.
func (a *PaddedPFCAccumulator) GetRxRateSec() float64 {
	durMs := a.sampleDurationMs.Load()
	if durMs == 0 {
		return 0.0
	}
	return float64(a.rxPauses.Load()) / (float64(durMs) / 1000.0)
}

// PaddedFabricBandwidthMonitor features 64-byte cache-line padding to eliminate false sharing
// across CPU cores under high-frequency NVLink and RDMA fabric bandwidth auditing.
// Memory layout: 3x uint64 (24 bytes) + 40 bytes pad = 64 bytes (1 L1 cache line).
type PaddedFabricBandwidthMonitor struct {
	bandwidthMilli atomic.Uint64 // Milli-GBps: 900000 = 900.000 GB/s
	sampleCount    atomic.Uint64
	errorCount     atomic.Uint64
	_              [40]byte // 64-byte cache line alignment padding
}

// Record atomically updates recorded bandwidth in milli-GBps and error counts.
func (m *PaddedFabricBandwidthMonitor) Record(bandwidthGBps float64, errors uint64) {
	m.bandwidthMilli.Store(uint64(bandwidthGBps * 1000.0))
	m.sampleCount.Add(1)
	m.errorCount.Store(errors)
}

// GetBandwidthGBps returns the current recorded link bandwidth in GB/s.
func (m *PaddedFabricBandwidthMonitor) GetBandwidthGBps() float64 {
	return float64(m.bandwidthMilli.Load()) / 1000.0
}

// GetSampleCount returns the total number of bandwidth audit samples recorded.
func (m *PaddedFabricBandwidthMonitor) GetSampleCount() uint64 {
	return m.sampleCount.Load()
}

// GetErrorCount returns total hardware link error counts recorded.
func (m *PaddedFabricBandwidthMonitor) GetErrorCount() uint64 {
	return m.errorCount.Load()
}

// PaddedRDMACounters aggregates high-frequency RDMA telemetry counters with 64-byte cache-line padding.
// Memory layout: 5x uint64 (40 bytes) + 24 bytes pad = 64 bytes (1 L1 cache line).
type PaddedRDMACounters struct {
	pfcPausesRx    atomic.Uint64
	pfcPausesTx    atomic.Uint64
	bandwidthMilli atomic.Uint64
	rxDrops        atomic.Uint64
	deadlockAlerts atomic.Uint64
	_              [24]byte // 64-byte cache line alignment padding
}

// RecordTelemetry atomically updates telemetry counters.
func (c *PaddedRDMACounters) RecordTelemetry(rxPauses, txPauses uint64, bandwidthGBps float64, drops uint64) {
	c.pfcPausesRx.Store(rxPauses)
	c.pfcPausesTx.Store(txPauses)
	c.bandwidthMilli.Store(uint64(bandwidthGBps * 1000.0))
	c.rxDrops.Store(drops)
}

// IncrementDeadlockAlert atomically increments deadlock alerts.
func (c *PaddedRDMACounters) IncrementDeadlockAlert() {
	c.deadlockAlerts.Add(1)
}

// GetPFCPausesRx returns recorded Rx pause count.
func (c *PaddedRDMACounters) GetPFCPausesRx() uint64 {
	return c.pfcPausesRx.Load()
}

// GetPFCPausesTx returns recorded Tx pause count.
func (c *PaddedRDMACounters) GetPFCPausesTx() uint64 {
	return c.pfcPausesTx.Load()
}

// GetBandwidthGBps returns recorded bandwidth in GB/s.
func (c *PaddedRDMACounters) GetBandwidthGBps() float64 {
	return float64(c.bandwidthMilli.Load()) / 1000.0
}

// GetRxDrops returns recorded packet drops.
func (c *PaddedRDMACounters) GetRxDrops() uint64 {
	return c.rxDrops.Load()
}

// GetDeadlockAlerts returns total deadlock alerts emitted.
func (c *PaddedRDMACounters) GetDeadlockAlerts() uint64 {
	return c.deadlockAlerts.Load()
}
