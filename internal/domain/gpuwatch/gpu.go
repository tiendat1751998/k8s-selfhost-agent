// Package gpuwatch provides domain models, telemetry metrics, and hardware watchdog
// contracts for NVIDIA GPU clusters, XID hardware fault isolation, NCCL distributed training
// deadlock detection, and vLLM/Triton PagedAttention KV-cache OOM preemption.
//
// PHYSICAL HARDWARE BOTTLENECK & ARCHITECTURAL ANALYSIS:
// 1. PCIe Gen4/Gen5 Bus Bandwidth & Host-to-Device Bottlenecks:
//    PCIe Gen4 x16 caps throughput at ~31.5 GB/s bidirectional (~63 GB/s in Gen5 x16).
//    During severe KV-cache eviction or host RAM offloading, streaming gigabytes of attention
//    matrices across the PCIe bus induces severe bus contention and latency spikes unless
//    proactively scheduled before GPU HBM reaches hard limits.
// 2. NVLink Interconnect Latency & Bandwidth:
//    NVLink 4.0 provides 900 GB/s bidirectional bandwidth per GPU (H100/NVSwitch). Cross-socket
//    and cross-node communications crossing PCIe boundaries without GPUDirect RDMA / InfiniBand
//    collapse multi-GPU pipeline parallelism.
// 3. GPU HBM Memory Bandwidth Saturation:
//    HBM3/HBM3e memory provides 3.35 TB/s bandwidth. Under dense attention kernels, memory bus
//    utilization approaching 100% starves concurrent CUDA streams, exacerbating thermal throttling.
// 4. PCIe Device Reset Timeouts & AER Lockup:
//    Hardware faults (Secondary Bus Reset / Function Level Reset - FLR) require 1000ms+ recovery
//    windows. If the userspace CUDA driver hangs, kernel PCIe AER (Advanced Error Reporting)
//    drops the device off the bus (XID 79), requiring immediate node isolation.
// 5. Userspace CUDA Driver Context Switches & Stalls:
//    Blocking calls such as cudaStreamSynchronize lock Python worker runtimes in epoll/futex wait.
// 6. Real-World Failure Modes:
//    - NVIDIA XID 31 (MMU Fault) & XID 43 (Engine Crash / Stopped Processing):
//      Causes zombie Python PyTorch/Megatron/DeepSpeed training processes to stall indefinitely.
//      In a 64-node NCCL AllReduce ring (512 GPUs), one deadlocked GPU holds the entire cluster
//      at a synchronization barrier, accumulating 100% idle GPU power costs (~350-700W/GPU).
//    - vLLM / Triton PagedAttention KV-Cache OOM:
//      Under sudden 32K token context bursts, dynamic block allocation exhausts available VRAM.
//      Triggering proactive offload/swapping to CPU host RAM at 85% utilization prevents fatal
//      CUDA OutOfMemoryError and service restarts.
package gpuwatch

import (
	"bufio"
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// RFC 7807 problem error codes for GPU AI cluster incidents.
const (
	CodeGPUXIDHardwareFault = "ERR_GPU_XID_HARDWARE_FAULT"
	CodeKVCacheOOMRisk      = "ERR_KV_CACHE_OOM_RISK"
	CodeNCCLHangDetected    = "ERR_NCCL_HANG_DETECTED"
)

// Standard domain error constants.
var (
	ErrGPUXIDHardwareFault = &domainerrors.DomainError{
		Code:    CodeGPUXIDHardwareFault,
		Message: "NVIDIA GPU critical hardware XID fault detected. Node isolation and zombie eviction required.",
		Err:     errors.New("critical gpu xid hardware fault"),
	}
	ErrKVCacheOOMRisk = &domainerrors.DomainError{
		Code:    CodeKVCacheOOMRisk,
		Message: "vLLM/Triton PagedAttention KV-cache pressure exceeded safe threshold (>=85%). Preemptive CPU swap required.",
		Err:     errors.New("kv-cache oom risk threshold exceeded"),
	}
	ErrNCCLHangDetected = &domainerrors.DomainError{
		Code:    CodeNCCLHangDetected,
		Message: "Distributed training NCCL AllReduce progress hang detected (>5m zero progress).",
		Err:     errors.New("nccl allreduce ring hang detected"),
	}
)

// GPU health states.
type GPUHealthState string

const (
	GPUHealthy   GPUHealthState = "HEALTHY"
	GPUThrottled GPUHealthState = "THROTTLED"
	GPUZombie    GPUHealthState = "ZOMBIE"
	GPULost      GPUHealthState = "LOST"
)

// Kubernetes node taint constants for faulty GPU isolation.
const (
	FaultyGPUTaintKey    = "nvidia.com/gpu"
	FaultyGPUTaintValue  = "faulty"
	FaultyGPUTaintEffect = "NoSchedule"
	FaultyGPUTaintString = "nvidia.com/gpu=faulty:NoSchedule"

	// KVCachePreemptThresholdPct defines the utilization watermark (85%) where proactive KV-cache swapping begins.
	KVCachePreemptThresholdPct = 85.0

	// NCCLMaxIdleTimeout defines the maximum duration with zero loss delta before marking an AllReduce job deadlocked.
	NCCLMaxIdleTimeout = 5 * time.Minute
)

// GPUCardTelemetry models point-in-time telemetry and health state for an individual GPU.
type GPUCardTelemetry struct {
	Index           int            `json:"index"`
	PCIAddress      string         `json:"pci_address"`
	Model           string         `json:"model"`
	VRAMTotalBytes  uint64         `json:"vram_total_bytes"`
	VRAMUsedBytes   uint64         `json:"vram_used_bytes"`
	VRAMUsagePct    float64        `json:"vram_usage_pct"`
	KVCacheUsagePct float64        `json:"kv_cache_usage_pct"`
	TemperatureC    int            `json:"temperature_c"`
	HealthState     GPUHealthState `json:"health_state"`
	RecentXIDErrors []int          `json:"recent_xid_errors"`
	LastPolledAt    time.Time      `json:"last_polled_at"`
}

// PaddedGPUCounters tracks GPU telemetry counters atomically.
type PaddedGPUCounters struct {
	vramUsedBytes     atomic.Uint64
	kvCacheUsageMilli atomic.Uint64 // Permille: 85000 = 85.000%
	pollCount         atomic.Uint64
	faultCount        atomic.Uint64
}

// RecordPoll atomically updates current telemetry counters.
func (c *PaddedGPUCounters) RecordPoll(vramUsed uint64, kvCacheMilli uint64) {
	c.vramUsedBytes.Store(vramUsed)
	c.kvCacheUsageMilli.Store(kvCacheMilli)
	c.pollCount.Add(1)
}

// RecordFault atomically increments hardware fault occurrences.
func (c *PaddedGPUCounters) RecordFault() {
	c.faultCount.Add(1)
}

// GetVRAMUsedBytes returns current recorded VRAM usage.
func (c *PaddedGPUCounters) GetVRAMUsedBytes() uint64 {
	return c.vramUsedBytes.Load()
}

// GetKVCacheUsagePct returns current KV-cache usage percentage.
func (c *PaddedGPUCounters) GetKVCacheUsagePct() float64 {
	return float64(c.kvCacheUsageMilli.Load()) / 1000.0
}

// GetPollCount returns total completed polling iterations.
func (c *PaddedGPUCounters) GetPollCount() uint64 {
	return c.pollCount.Load()
}

// GetFaultCount returns total hardware fault incidents encountered.
func (c *PaddedGPUCounters) GetFaultCount() uint64 {
	return c.faultCount.Load()
}

// XIDEvent represents a parsed NVIDIA kernel error event.
type XIDEvent struct {
	PCIAddress string    `json:"pci_address"`
	XID        int       `json:"xid"`
	Message    string    `json:"message"`
	Timestamp  time.Time `json:"timestamp"`
	IsCritical bool      `json:"is_critical"`
}

// GPUClusterManager defines the domain interface for GPU cluster telemetry and SRE remediation.
type GPUClusterManager interface {
	ScanNodeGPUs(ctx context.Context, node string) ([]GPUCardTelemetry, error)
	TaintFaultyGPUNode(ctx context.Context, node string, pciAddr string, xid int) error
	EvictZombiePod(ctx context.Context, podName, namespace string) error
	CheckNCCLProgress(ctx context.Context, jobID string, maxIdleDuration time.Duration) (bool, error)
}

// IsCriticalXID determines if an XID error code represents an unrecoverable hardware fault.
// Critical faults:
// - XID 31: MMU Fault (page fault, memory corruption, broken HBM)
// - XID 43: GPU stopped processing / engine crash
// - XID 45: Engine reset failure
// - XID 79: GPU has fallen off the bus (fatal PCIe AER failure)
// Non-critical informational errors (e.g. XID 13, 32, 62) are recoverable by application runtime.
func IsCriticalXID(xid int) bool {
	switch xid {
	case 31, 43, 45, 79:
		return true
	default:
		return false
	}
}

// Regular expression to parse standard NVIDIA dmesg XID error lines:
// e.g. "NVRM: Xid (PCI:0000:01:00.0): 31, pid=12345, name=python3, MMU Fault: ..."
var dmesgXIDRegex = regexp.MustCompile(`NVRM:\s+Xid\s+\(PCI:([0-9a-fA-F:\.]+)\):\s+(\d+)(?:,\s*(.*))?`)

// ParseDmesgXIDErrors parses kernel dmesg output for NVIDIA XID hardware fault signatures.
func ParseDmesgXIDErrors(dmesgLog string) []XIDEvent {
	var events []XIDEvent
	scanner := bufio.NewScanner(strings.NewReader(dmesgLog))

	for scanner.Scan() {
		line := scanner.Text()
		matches := dmesgXIDRegex.FindStringSubmatch(line)
		if len(matches) >= 3 {
			pciAddr := matches[1]
			xidVal, err := strconv.Atoi(matches[2])
			if err != nil {
				continue
			}

			msg := ""
			if len(matches) >= 4 {
				msg = strings.TrimSpace(matches[3])
			}

			events = append(events, XIDEvent{
				PCIAddress: pciAddr,
				XID:        xidVal,
				Message:    msg,
				Timestamp:  time.Now().UTC(),
				IsCritical: IsCriticalXID(xidVal),
			})
		}
	}

	return events
}

// EvaluateKVCacheRisk checks whether PagedAttention KV-cache pressure warrants preemptive host offloading.
func EvaluateKVCacheRisk(usagePct float64) error {
	if usagePct >= KVCachePreemptThresholdPct {
		return ErrKVCacheOOMRisk
	}
	return nil
}

// IsNCCLHang checks whether an idle training duration exceeds the 5-minute stall threshold.
func IsNCCLHang(idleDuration time.Duration) bool {
	return idleDuration > NCCLMaxIdleTimeout
}
