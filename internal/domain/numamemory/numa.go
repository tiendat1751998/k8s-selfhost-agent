// Package numamemory provides domain contracts, telemetry structures, cache-line padded
// performance primitives, and error declarations for NUMA zone memory management,
// asynchronous dirty page writeback tuning, HugePage defragmentation, and kernel slab monitoring.
//
// ENGINEERING NOTE — PHYSICAL HARDWARE BOTTLENECKS & PRODUCTION FAILURE MODES:
// 1. Cross-Socket QPI/UPI Interconnect Latency vs Local NUMA Access:
//    In multi-socket NUMA systems (e.g. Intel Ultra Path Interconnect - UPI, AMD Infinity Fabric),
//    local NUMA memory access incurs ~35-40ns latency, whereas cross-socket remote memory traversal
//    traversing UPI links incurs ~100-120ns latency plus link queueing delay.
//    However, misconfigured kernel zone reclamation (vm.zone_reclaim_mode = 1) forces kswapd/direct
//    reclaim to synchronously compact and evict local node memory when a single NUMA zone runs low,
//    inducing catastrophic hundred-millisecond CPU spinlocks and thread freezes, even when
//    adjacent NUMA nodes possess tens of gigabytes of unallocated RAM. Setting vm.zone_reclaim_mode = 0
//    sacrifices negligible UPI latency delta in favor of preventing fatal CPU spinlock lockups.
//
// 2. Synchronous Dirty Page Writeback Stalls Blocking Kernel Block Queues:
//    When memory dirty pages accumulate under default percentage-based limits (e.g. vm.dirty_ratio = 20
//    on a 512GB host = 102GB dirty page buffer), writeout activity shifts from asynchronous background
//    flusher threads (wb_work) to synchronous throttle (balance_dirty_pages). Processes attempting
//    write(2) syscalls are blocked in TASK_UNINTERRUPTIBLE (D-state) while the kernel attempts to flush
//    tens of gigabytes through NVMe/SATA I/O block queues, freezing node disk I/O at 100% saturation.
//    Enforcing absolute byte limits (vm.dirty_background_bytes = 256MB, vm.dirty_bytes = 1GB) eliminates
//    I/O queue saturation spikes and enforces smooth, predictable flusher cadence.
//
// 3. Linux Buddy Allocator Physical Memory Fragmentation & HugePage Allocation:
//    Linux organizes physical frames into 2^order power-of-two page blocks via the buddy allocator.
//    High-performance network/AI workloads (DPDK, vLLM, TensorRT-LLM) require contiguous 2MiB (order-9)
//    or 1GiB (order-18) physical HugePages. Over system uptime, unmovable page allocations fragment physical
//    address space, driving the fragmentation index above 0.70. Under this condition, hugepage allocation
//    syscalls (mmap MAP_HUGETLB) fail with ENOMEM, leaving DPDK and LLM pods in Pending/Unschedulable
//    cluster state. Triggering kernel compaction via /proc/sys/vm/compact_memory coalesces adjacent buddy
//    blocks into contiguous frames.
//
// 4. Kernel Slab SUnreclaim Memory Pressure & Inode/Dentry Leaks:
//    Unreclaimable kernel slab structures (SUnreclaim in /proc/meminfo), populated by dentries, vfs inodes,
//    and kernel buffer heads, cannot be freed under memory pressure. If SUnreclaim exceeds 30% of total
//    host RAM due to dentry leaks or orphaned namespace mounts, host available memory drops dangerously,
//    tripping system Out-Of-Memory (OOM) killer invocations that crash vital system daemons and workloads.
package numamemory

import (
	"context"
	"errors"
)

// Standard domain error definitions.
var (
	// ErrNUMAZoneLocked indicates NUMA zone memory compaction or zone reclaim spinlock lockup.
	ErrNUMAZoneLocked = errors.New("numa zone locked by synchronous reclaim spinlock")

	// ErrDirtyPageFlushSaturation indicates dirty page accumulation exceeding thresholds, causing synchronous flush saturation.
	ErrDirtyPageFlushSaturation = errors.New("dirty page flush saturation blocking kernel write queues")

	// ErrHugePagesFragmented indicates physical memory fragmentation preventing contiguous HugePages allocation.
	ErrHugePagesFragmented = errors.New("physical memory buddy allocator fragmented; hugepage allocation impossible")

	// ErrSlabUnreclaimableLeak indicates kernel slab SUnreclaim exceeding 30% total RAM, indicating dentry/inode leak.
	ErrSlabUnreclaimableLeak = errors.New("kernel slab unreclaimable memory leak detected exceeding threshold")
)

// Standard RFC 7807 and SRE error codes.
const (
	CodeNUMAZoneLocked            = "NUMA_ZONE_LOCKED"
	CodeDirtyPageFlushSaturation  = "DIRTY_PAGE_FLUSH_SATURATION"
	CodeHugePagesFragmented       = "HUGEPAGES_FRAGMENTED"
	CodeSlabUnreclaimableLeak     = "SLAB_UNRECLAIMABLE_LEAK"
	CodeValidationFailed          = "VALIDATION_FAILED"
)

// NUMAZoneStatus represents the allocation, capacity, and reclaim lock status of a physical NUMA node.
type NUMAZoneStatus struct {
	NodeID           int   `json:"node_id"`
	TotalRAMBytes    int64 `json:"total_ram_bytes"`
	FreeRAMBytes     int64 `json:"free_ram_bytes"`
	ZoneReclaimMode  int   `json:"zone_reclaim_mode"`
	SpinlockDetected bool  `json:"spinlock_detected"`
}

// DirtyPageConfig defines Linux kernel asynchronous and synchronous dirty page writeback byte thresholds.
type DirtyPageConfig struct {
	DirtyBackgroundBytes   int64 `json:"dirty_background_bytes"`
	DirtyBytes             int64 `json:"dirty_bytes"`
	CurrentDirtyBytes      int64 `json:"current_dirty_bytes"`
	SynchronousFlushActive bool  `json:"synchronous_flush_active"`
}

// HugePagePoolStatus represents buddy allocator allocation metrics and fragmentation index for a specific HugePage size.
type HugePagePoolStatus struct {
	PageSizeKB       int     `json:"page_size_kb"`
	TotalPages       int     `json:"total_pages"`
	FreePages        int     `json:"free_pages"`
	FragmentationIdx float64 `json:"fragmentation_idx"`
}

// Cache-Line Padded Performance Counters (64-byte alignment).
// These structs ensure that high-frequency metric tracking on multi-core SMP and NUMA architectures
// does not trigger false sharing (MESI/MOESI cache line bouncing) across L1/L2 caches.

// NUMASpinlockCounter tracks NUMA node lock contention events with 64-byte cache line padding.
type NUMASpinlockCounter struct {
	SpinlockCount uint64
	_             [56]byte // 64 - 8 = 56 bytes padding to complete 64-byte L1 cache line
}

// DirtyPageTracker tracks dirty page bytes and flush saturation with 64-byte cache line padding.
type DirtyPageTracker struct {
	DirtyBytes uint64
	_          [56]byte // 64 - 8 = 56 bytes padding to complete 64-byte L1 cache line
}

// HugePageAllocationMetric tracks allocated HugePages and allocation requests with 64-byte cache line padding.
type HugePageAllocationMetric struct {
	AllocatedPages uint64
	_              [56]byte // 64 - 8 = 56 bytes padding to complete 64-byte L1 cache line
}

// LinuxMemorySubsystemManager defines low-level kernel memory interaction and tuning contracts.
type LinuxMemorySubsystemManager interface {
	// AuditNUMAZones retrieves memory capacity and zone reclaim configuration across all NUMA nodes.
	AuditNUMAZones(ctx context.Context) ([]NUMAZoneStatus, error)

	// TuneNUMAZoneReclaim adjusts vm.zone_reclaim_mode across the kernel memory subsystem.
	TuneNUMAZoneReclaim(ctx context.Context, targetMode int) error

	// ApplyDirtyPageByteLimits sets vm.dirty_background_bytes and vm.dirty_bytes limits.
	ApplyDirtyPageByteLimits(ctx context.Context, backgroundBytes, totalBytes int64) error

	// CheckHugePageAvailability audits HugePage allocation pools and calculates buddy fragmentation index.
	CheckHugePageAvailability(ctx context.Context, sizeKB int, requestedPages int) (*HugePagePoolStatus, error)

	// TriggerCompactionIfFragmented initiates /proc/sys/vm/compact_memory if fragmentation exceeds threshold.
	TriggerCompactionIfFragmented(ctx context.Context, threshold float64) error

	// AuditSlabUnreclaimable checks SUnreclaim bytes against total host RAM.
	AuditSlabUnreclaimable(ctx context.Context) (unreclaimMB int64, err error)
}
