// Package hardware provides domain contracts, telemetry representations, and error models
// for bare-metal hardware health auditing, including memory controller EDAC, CPU thermal
// throttling, and power supply redundancy watchdogs.
//
// =========================================================================================
// PHYSICAL HARDWARE BOTTLENECK ANALYSIS & REAL-WORLD FAILURE MODES (ZERO-COMPLACENCY NOTE)
// =========================================================================================
//
// 1. PHYSICAL HARDWARE BOTTLENECK ANALYSIS:
//   - Memory Controller EDAC Interrupt Overhead:
//     When correctable memory errors occur, memory controllers (IMC) generate System Management
//     Interrupts (SMI) or Machine Check Exceptions (MCE). Under SMI-based reporting, all CPU cores
//     are forced into System Management Mode (SMM), halting the OS scheduler. This introduces
//     multi-millisecond tail latency spikes (p99/p999 latency inflation). Furthermore, accessing
//     EDAC sysfs registers (`/sys/devices/system/edac/mc/mc*/`) requires acquiring kernel IMC
//     spinlocks across memory channels, causing bus locking under high polling frequencies.
//   - PCIe AER Interrupt Storms Pinning CPU Core 0:
//     PCIe root ports and NVMe/NIC endpoint switches report correctable link errors (bad TLP/DLLP
//     replays) through MSI/MSI-X interrupts. In default Linux smp_affinity configurations, IRQs
//     route exclusively to CPU Core 0 (`smp_affinity = 1`). A degraded physical link can emit
//     millions of AER interrupts per second, saturating `ksoftirqd/0`, triggering RCU CPU stall
//     warnings, and preventing heartbeat processing while the host appears unresponsive.
//   - CPU Package MSR Thermal Sensor Query Latency:
//     Reading IA32_PACKAGE_THERM_STATUS (0x1B1) or coretemp hwmon requires `rdmsr` instructions
//     or sysfs context switches. `rdmsr` acts as a serializing instruction on x86-64 pipelines,
//     flushing speculative execution buffers. Thermal sensor diodes also employ digital filtering
//     hysteresis (100ms - 250ms), meaning instantaneous polling creates serializing pipeline stalls
//     without providing sub-millisecond fidelity.
//   - Dual-PSU Load Sharing / PDU Capacity Limits:
//     Enterprise servers feature redundant PSUs operating in active-active load sharing (each at
//     ~45-50% load). When one PSU or feed fails, the remaining unit instantaneously assumes 100%
//     load. During peak compute workloads (e.g. AVX-512 vectorization), this instantaneous shift
//     causes input power factor degradation and inrush spikes that can trip the branch circuit
//     breaker of the rack PDU, cascading into a catastrophic multi-rack blackout.
//
// 2. REAL-WORLD FAILURE MODES:
//   - ECC Correctable Error Bursts (>10,000/min):
//     Single-bit correctable errors (CE) handled by SECDED or Chipkill indicate deteriorating DRAM
//     cell charge or solder ball fatigue. When error rates surge (>100 errors/hour, escalating to
//     >10,000/min), the statistical probability of two adjacent bit-flips in the same burst word
//     approaches unity, rapidly collapsing into an uncorrectable multi-bit fatal MCE (panic) or
//     Silent Data Corruption (SDC).
//   - Silent CPU Thermal Throttling (PROCHOT# Asserted):
//     Thermal Interface Material (TIM) degradation or fan assembly failure triggers BD-PROCHOT,
//     dropping core frequencies from 3.6 GHz to 800 MHz (LFM). The node remains 'Ready' in Kubelet
//     because heartbeats succeed, but container request latency inflates 15x-25x, creating
//     unexplained upstream timeout storms across microservices.
//   - Single PSU Failure Cascading into Whole-Rack PDU Trip:
//     Loss of feed redundancy shifts total rack current onto a single PDU line. Without proactive
//     RAPL CPU wattage capping (limiting package power to <=70%), simultaneous workload peaks
//     exceed the 80% continuous load safety rating of the PDU breaker.
// =========================================================================================
package hardware

import (
	"context"
	"errors"
	"time"
)

// Standard domain error constants for bare-metal hardware watchdogs.
var (
	// ErrECCMemoryCorrectableSurge is returned when memory correctable error rate exceeds the critical threshold (>100/hr).
	ErrECCMemoryCorrectableSurge = errors.New("ecc memory correctable error surge detected")

	// ErrCPUThermalThrottled is returned when CPU package temperature exceeds 85°C or frequency is throttled below 50%.
	ErrCPUThermalThrottled = errors.New("cpu package thermal throttling active")

	// ErrPSURedundancyFailed is returned when dual-PSU power redundancy is lost.
	ErrPSURedundancyFailed = errors.New("power supply redundancy lost")
)

// Standard error codes for RFC 7807 problem details responses.
const (
	CodeECCMemoryCorrectableSurge = "ECC_MEMORY_CORRECTABLE_SURGE"
	CodeCPUThermalThrottled       = "CPU_THERMAL_THROTTLED"
	CodePSURedundancyFailed       = "PSU_REDUNDANCY_FAILED"
	CodeValidation                = "VALIDATION_ERROR"
	CodeInternalError             = "INTERNAL_ERROR"
	CodeNodeNotFound              = "NODE_NOT_FOUND"
)

// Operational thresholds and Kubernetes metadata constants.
const (
	DegradedNodeLabelKey          = "node.kubernetes.io/hardware-degraded"
	DegradedNodeLabelValue        = "true"
	ThermalThrottledTaintKey      = "node.kubernetes.io/thermal-throttled"
	ThermalThrottledTaintEffect   = "NoSchedule"
	DefaultDrainGracePeriod       = 120 * time.Second
	ECCErrorRateThresholdPerHour  = 100.0
	MaxAllowedPackageTempC        = 85.0
	FrequencyThrottleRatio        = 0.50
	RAPLPowerCapRatio             = 0.70
)

// CacheLinePaddedCounter provides a 64-byte aligned counter to prevent false sharing
// across CPU cores under high-scale telemetry polling under the MESI/MOESI protocol.
type CacheLinePaddedCounter struct {
	Value uint64
	_     [56]byte // 64 bytes total cache-line size (8 bytes value + 56 bytes padding)
}

// ECCDIMMStatus represents low-level memory error counters and health state for a DIMM slot.
// 64-byte cache line padding protects high-frequency polling telemetry from cross-core false sharing.
type ECCDIMMStatus struct {
	SlotID           string    `json:"slot_id"`
	CorrectableCount uint64    `json:"correctable_count"`
	ErrorRatePerMin  float64   `json:"error_rate_per_min"`
	IsDegraded       bool      `json:"is_degraded"`
	LastEventTime    time.Time `json:"last_event_time"`
	_                [56]byte  `json:"-"`
}

// CPUThermalStatus represents package temperature and clock frequency telemetry.
// 64-byte cache line padding protects concurrent reads/writes from core cache line invalidations.
type CPUThermalStatus struct {
	PackageTempC        float64  `json:"package_temp_c"`
	IsThrottled         bool     `json:"is_throttled"`
	BaseFrequencyMHz    float64  `json:"base_frequency_mhz"`
	CurrentFrequencyMHz float64  `json:"current_frequency_mhz"`
	_                   [32]byte `json:"-"`
}

// PowerSupplyStatus represents hardware telemetry and redundancy state for an individual server PSU.
// 64-byte cache line padding isolates telemetry data across multiple polling threads.
type PowerSupplyStatus struct {
	PSUID          string   `json:"psu_id"`
	IsPresent      bool     `json:"is_present"`
	InputVoltage   float64  `json:"input_voltage"`
	OutputPowerW   float64  `json:"output_power_w"`
	RedundancyLost bool     `json:"redundancy_lost"`
	_              [40]byte `json:"-"`
}

// HardwareWatchdogEngine defines the orchestration interface for bare-metal hardware health.
type HardwareWatchdogEngine interface {
	AuditECCMemoryErrors(ctx context.Context) ([]ECCDIMMStatus, error)
	AuditCPUThermalThrottling(ctx context.Context) (*CPUThermalStatus, error)
	AuditPowerSupplyRedundancy(ctx context.Context) ([]PowerSupplyStatus, error)
	DrainAndCordonDegradedNode(ctx context.Context, nodeName string, reason string) error
	TaintThermalThrottledNode(ctx context.Context, nodeName string) error
}

// EDACReader abstracts reading low-level memory controller error statistics.
type EDACReader interface {
	ReadDIMMErrors(ctx context.Context) ([]ECCDIMMStatus, error)
}

// CPUThermalReader abstracts reading CPU package temperature and frequency metrics.
type CPUThermalReader interface {
	ReadCPUThermal(ctx context.Context) (*CPUThermalStatus, error)
}

// PowerSupplyReader abstracts reading IPMI / BMC power supply redundancy data.
type PowerSupplyReader interface {
	ReadPowerSupplies(ctx context.Context) ([]PowerSupplyStatus, error)
}

// NodeRemediator abstracts Kubernetes node lifecycle operations for bare-metal hosts.
type NodeRemediator interface {
	CordonNode(ctx context.Context, nodeName string) error
	LabelNode(ctx context.Context, nodeName string, key, value string) error
	TaintNode(ctx context.Context, nodeName string, key, value, effect string) error
	DrainNode(ctx context.Context, nodeName string, gracePeriod time.Duration) error
	EvictLatencyCriticalPods(ctx context.Context, nodeName string) ([]string, error)
}

// RAPLController abstracts Intel/AMD Running Average Power Limit (RAPL) power capping.
type RAPLController interface {
	SetRAPLPowerLimit(ctx context.Context, ratio float64) error
	GetRAPLPowerLimit(ctx context.Context) (float64, error)
}

// AlertSeverity represents alert importance levels.
type AlertSeverity string

const (
	SeverityP0      AlertSeverity = "P0"
	SeverityWarning AlertSeverity = "WARNING"
)

// HardwareAlert represents an operational alert event emitted by watchdogs.
type HardwareAlert struct {
	Severity  AlertSeverity `json:"severity"`
	Component string        `json:"component"`
	Target    string        `json:"target"`
	Message   string        `json:"message"`
	Timestamp time.Time     `json:"timestamp"`
}

// HardwareAlertSink defines the port for broadcasting urgent bare-metal hardware alerts.
type HardwareAlertSink interface {
	EmitAlert(ctx context.Context, alert HardwareAlert) error
}
