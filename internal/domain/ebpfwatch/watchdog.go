// Package ebpfwatch defines domain entities, safety contracts, and hardware-level
// guardrails for eBPF runtime security, ring buffer telemetry, and kernel probe safety.
//
// =========================================================================================
// CRITICAL ENGINEERING NOTE: PHYSICAL HARDWARE BOTTLENECKS & REAL-WORLD FAILURE MODES
// =========================================================================================
// 1. Physical Hardware Bottlenecks:
//   - Kernel Context Switches: High-frequency eBPF program invocation on hot kernel paths
//     (e.g., sys_enter/sys_exit) incurs kernel transition overhead. In multi-tenant nodes
//     handling >500k syscalls/s, per-core register state preservation degrades application IPC.
//   - TLB Shootdowns on Probe Attach: Dynamic kprobe/ftrace patching modifies kernel text
//     pages, requiring inter-processor interrupts (IPIs) and TLB flushes across all online CPUs.
//     Mass-attaching probes under high node load causes micro-freezes and CPU soft-lockups.
//   - Memory Locking Limits (RLIMIT_MEMLOCK): Legacy BPF maps and perf event rings depend
//     on locked physical RAM pages. Exceeding RLIMIT_MEMLOCK fails probe loading unless
//     migrated to kernel cgroup-based memory accounting (Linux >= 5.11).
//   - L1 Instruction Cache Eviction: Large, unrolled eBPF bytecode loaded into kernel text
//     displaces hot OS kernel paths from CPU L1i caches (32KB per core), introducing micro-latency
//     spikes in networking stacks and scheduler dispatch loops.
//   - Go GC P99 Latency Spikes: High-throughput ingestion in userspace creates allocation pressure.
//     If userspace ring buffer event consumption stutters during a GC mark phase or STW pause,
//     the kernel ring buffer fills up and drops subsequent events.
//
// 2. Real-World Failure Modes:
//   - Silent Tetragon / Falco Audit Blindness: Under massive traffic surges (>500,000 syscalls/s),
//     bpf_ringbuf_reserve() fails and returns NULL. If the eBPF bytecode discards events without
//     incrementing a lost events counter or signaling userspace, security observability goes
//     completely blind while CPU counters appear normal.
//   - Kernel Panic / Soft Lockup via Volatile Symbols: Attaching raw kprobes to unstable,
//     preemptible, or inlined internal symbols (e.g., do_exit, schedule, finish_task_switch)
//     corrupts kernel register states or enters recursive probe execution, triggering panic.
//   - BPF Verifier Rejection on Minor Upgrades: Unstable kernel struct layouts between point
//     releases break direct pointer dereferences; BPF Type Format (BTF) and Compile Once -
//     Run Everywhere (CO-RE) are mandatory to ensure portable, safe verification.
// =========================================================================================
package ebpfwatch

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"time"

	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// RFC 7807 error codes for eBPF runtime watchdog.
const (
	CodeRingBufferOverflow    = "ERR_RINGBUF_OVERFLOW"
	CodeUnstableKprobeRejected = "ERR_UNSTABLE_KPROBE_REJECTED"
	CodeBTFMissing            = "ERR_BTF_MISSING"
	CodeCPUQuotaExceeded      = "ERR_EBPF_CPU_QUOTA_EXCEEDED"
)

// Standard domain errors.
var (
	ErrRingBufferOverflow = &domainerrors.DomainError{
		Code:    CodeRingBufferOverflow,
		Message: "eBPF ring buffer overflow: dropped kernel events detected, saturation >= 70%",
		Err:     errors.New("ringbuf reserve capacity exhausted"),
	}
	ErrUnstableKprobeRejected = &domainerrors.DomainError{
		Code:    CodeUnstableKprobeRejected,
		Message: "Raw kprobe rejected: target symbol is volatile, inlined, or unexported",
		Err:     errors.New("unstable kernel symbol probe target prohibited"),
	}
	ErrBTFMissing = &domainerrors.DomainError{
		Code:    CodeBTFMissing,
		Message: "Kernel BTF (/sys/kernel/btf/vmlinux) not available; CO-RE verification required",
		Err:     errors.New("kernel btf vmlinux unavailable"),
	}
	ErrCPUQuotaExceeded = &domainerrors.DomainError{
		Code:    CodeCPUQuotaExceeded,
		Message: "eBPF CPU utilization exceeded maximum allowable limit (>5% of node capacity)",
		Err:     errors.New("ebpf cpu utilization guardrail triggered"),
	}
)

// Sizing and safety constants for eBPF ring buffers.
const (
	MinRingBufferPages    = 64
	DefaultRingBufferPages = 256
	MaxRingBufferPages    = 4096
	PressureThresholdPct  = 70.0
	MaxEBPFCPUQuotaPct    = 5.0
)

// ProbeHookType represents the attachment mechanism of an eBPF probe.
type ProbeHookType string

const (
	HookTracepoint ProbeHookType = "tracepoint"
	HookFentry     ProbeHookType = "fentry"
	HookFexit      ProbeHookType = "fexit"
	HookKprobeRaw  ProbeHookType = "kprobe_raw"
)

// IsValid validates whether the probe hook type is supported.
func (h ProbeHookType) IsValid() bool {
	switch h {
	case HookTracepoint, HookFentry, HookFexit, HookKprobeRaw:
		return true
	default:
		return false
	}
}

// Volatile kernel symbols that must NEVER be attached to via raw kprobes.
var volatileKernelSymbols = map[string]struct{}{
	"do_exit":            {},
	"schedule":           {},
	"__schedule":         {},
	"finish_task_switch": {},
	"try_to_wake_up":     {},
	"panic":              {},
	"arch_cpu_idle":      {},
}

// CheckSymbolSafety checks if a given symbol is safe for the specified hook type.
func CheckSymbolSafety(hookType ProbeHookType, symbol string) error {
	trimmed := strings.TrimSpace(symbol)
	if trimmed == "" {
		return &domainerrors.DomainError{
			Code:    CodeUnstableKprobeRejected,
			Message: "Kernel symbol name cannot be empty",
			Err:     errors.New("empty symbol name"),
		}
	}

	if hookType == HookKprobeRaw {
		if _, isVolatile := volatileKernelSymbols[trimmed]; isVolatile {
			return ErrUnstableKprobeRejected
		}
		if strings.HasPrefix(trimmed, "inline_") || strings.Contains(trimmed, "_rcu_") {
			return ErrUnstableKprobeRejected
		}
	}

	return nil
}

// PaddedRingBufferCounter provides an atomic counter set for eBPF ring buffer telemetry.
type PaddedRingBufferCounter struct {
	TotalEvents atomic.Uint64
	LostEvents  atomic.Uint64
	ProcTimeNs  atomic.Int64
}

// IncTotalEvents atomically increments the total event counter.
func (c *PaddedRingBufferCounter) IncTotalEvents() uint64 {
	return c.TotalEvents.Add(1)
}

// IncLostEvents atomically increments the lost event counter.
func (c *PaddedRingBufferCounter) IncLostEvents() uint64 {
	return c.LostEvents.Add(1)
}

// AddProcTime atomically accumulates processing time in nanoseconds.
func (c *PaddedRingBufferCounter) AddProcTime(ns int64) int64 {
	return c.ProcTimeNs.Add(ns)
}

// GetAvgProcTimeNs calculates average processing time per event in nanoseconds.
func (c *PaddedRingBufferCounter) GetAvgProcTimeNs() int64 {
	total := c.TotalEvents.Load()
	if total == 0 {
		return 0
	}
	return c.ProcTimeNs.Load() / int64(total)
}

// PaddedSamplingController provides an atomic controller
// for adaptive rate limiting and lightweight fallback mode.
type PaddedSamplingController struct {
	SamplingRatePct atomic.Uint32
	LightweightMode atomic.Bool
}

// SetSamplingRate sets the adaptive event sampling rate percentage (1..100).
func (s *PaddedSamplingController) SetSamplingRate(pct uint32) {
	if pct > 100 {
		pct = 100
	}
	s.SamplingRatePct.Store(pct)
}

// GetSamplingRate retrieves the current event sampling rate percentage.
func (s *PaddedSamplingController) GetSamplingRate() uint32 {
	rate := s.SamplingRatePct.Load()
	if rate == 0 {
		return 100
	}
	return rate
}

// SetLightweight toggles lightweight audit mode.
func (s *PaddedSamplingController) SetLightweight(enabled bool) {
	s.LightweightMode.Store(enabled)
}

// IsLightweight returns whether lightweight audit mode is active.
func (s *PaddedSamplingController) IsLightweight() bool {
	return s.LightweightMode.Load()
}

// RingBufferTelemetry captures real-time utilization, event loss, and performance of a probe ring buffer.
type RingBufferTelemetry struct {
	ProbeName       string    `json:"probe_name"`
	TotalEvents     uint64    `json:"total_events"`
	LostEvents      uint64    `json:"lost_events"`
	BufferUsagePct  float64   `json:"buffer_usage_pct"`
	AvgProcTimeNs   int64     `json:"avg_proc_time_ns"`
	SampleTimestamp time.Time `json:"sample_timestamp"`
	AllocatedPages  int       `json:"allocated_pages"`
	SamplingRatePct float64   `json:"sampling_rate_pct"`
}

// IsOverloaded returns true if buffer usage exceeds 70% or any kernel events were dropped.
func (t RingBufferTelemetry) IsOverloaded() bool {
	return t.BufferUsagePct >= PressureThresholdPct || t.LostEvents > 0
}

// CalculateExpandedPages calculates the next safe ring buffer page capacity (doubling).
func (t RingBufferTelemetry) CalculateExpandedPages() int {
	pages := t.AllocatedPages
	if pages <= 0 {
		pages = DefaultRingBufferPages
	}
	doubled := pages * 2
	if doubled > MaxRingBufferPages {
		return MaxRingBufferPages
	}
	return doubled
}

// KernelSafetyValidator defines the contract for kernel probe verification,
// BTF support checking, and ring buffer pressure auditing.
type KernelSafetyValidator interface {
	CheckBTFSupport(ctx context.Context) (bool, error)
	AuditRingBufferPressure(ctx context.Context) ([]RingBufferTelemetry, error)
	AutoExpandRingBuffer(ctx context.Context, probe string, newPages int) error
	ValidateHookSafety(hookType ProbeHookType, symbol string) error
}
