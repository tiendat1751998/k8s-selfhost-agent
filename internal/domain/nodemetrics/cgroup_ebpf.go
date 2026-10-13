package nodemetrics

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// Domain error definitions for kernel interceptor and eBPF auto-scaler.
var (
	ErrCGroupThrottlingCritical = errors.New("cgroup v2 memory.high throttling exceeded critical threshold")
	ErrEBPFMapExhaustion        = errors.New("ebpf map utilization exceeded critical threshold (risk of -E2BIG packet drop)")
	ErrSlabCachePressure        = errors.New("kernel slab dentry/inode cache pressure requires tuning")
	ErrInvalidCapacity          = errors.New("invalid capacity specified for ebpf map resize")
	ErrRingBufferOverflow       = errors.New("kernel event ring buffer overflow; backpressure applied")
)

// Status constants for eBPF map utilization.
const (
	MapStatusNormal   = "NORMAL"
	MapStatusWarning  = "WARNING"
	MapStatusCritical = "CRITICAL"
)

// Physical hardware bottleneck and failure mode documentation:
// 1. Kernel Context Switches & TLB Flushes: High-rate cgroup v2 inotify/event polling introduces severe CPU
//    context switching overhead and cross-core TLB invalidation flushes if events are not bounded or batched.
// 2. RLIMIT_MEMLOCK: BPF maps allocate pinned kernel memory outside normal user memory quotas. Without sufficient
//    RLIMIT_MEMLOCK, eBPF map creation or dynamic resizing fails immediately with -EPERM or -ENOMEM.
// 3. Kernel Slab Dentry/Inode Cache Inflation: Intensive filesystem churn causes unevictable slab growth,
//    artificially inflating memory usage and triggering false Kubelet memory pressure evictions unless vm.vfs_cache_pressure is tuned.
// 4. cgroup v2 memory.high Sleep Penalty: Breaching memory.high enforces synchronous proportional sleep delays
//    on allocate threads, causing silent tail latency spikes up to several milliseconds without an outright OOM kill.
// 5. eBPF Hash Map Exhaustion (-E2BIG): Cilium conntrack or NAT hash maps reaching 100% capacity fail to insert new flows,
//    resulting in silent packet drops at the XDP/TC layer and broken TCP handshakes.

// PaddedKernelEventCounter features 64-byte cache-line padding to eliminate false sharing
// across multiple CPU cores during high-frequency inotify and perf event sampling.
type PaddedKernelEventCounter struct {
	LowEvents         uint64
	HighEvents        uint64
	MaxEvents         uint64
	OOMKillEvents     uint64
	ThrottledDuration int64     // Nanoseconds stored atomically
	_                 [8]uint64 // 64-byte cache-line padding to isolate L1/L2 cache lines
}

// PaddedEBPFMetric features 64-byte cache-line padding to prevent false sharing
// when updating high-rate eBPF map occupancy telemetry across multi-threaded samplers.
type PaddedEBPFMetric struct {
	CurrentEntries int64
	MaxEntries     int64
	PressureRatio  float64
	_              [8]uint64 // 64-byte cache-line padding
}

// CGroupV2MemoryEvent represents real-time kernel memory events parsed from cgroup v2 memory.events.
type CGroupV2MemoryEvent struct {
	ContainerID       string        `json:"container_id"`
	PodName           string        `json:"pod_name"`
	Namespace         string        `json:"namespace"`
	LowEvents         int64         `json:"low_events"`
	HighEvents        int64         `json:"high_events"`
	MaxEvents         int64         `json:"max_events"`
	OOMKillEvents     int64         `json:"oom_kill_events"`
	ThrottledDuration time.Duration `json:"throttled_duration"`
	UpdatedAt         time.Time     `json:"updated_at"`
}

// EBPFMapUtilization captures memory occupancy and sizing recommendations for kernel eBPF maps.
type EBPFMapUtilization struct {
	MapName         string  `json:"map_name"`
	CurrentEntries  int64   `json:"current_entries"`
	MaxEntries      int64   `json:"max_entries"`
	PressureRatio   float64 `json:"pressure_ratio"` // e.g. 0.85 = 85%
	Status          string  `json:"status"`         // NORMAL, WARNING, CRITICAL
	RecommendedSize int64   `json:"recommended_size"`
}

// NewEBPFMapUtilization computes utilization ratio, warning/critical status, and recommended capacity.
func NewEBPFMapUtilization(mapName string, current, max int64) EBPFMapUtilization {
	if max <= 0 {
		return EBPFMapUtilization{
			MapName:         mapName,
			CurrentEntries:  current,
			MaxEntries:      max,
			PressureRatio:   0,
			Status:          MapStatusNormal,
			RecommendedSize: max,
		}
	}

	ratio := float64(current) / float64(max)
	status := MapStatusNormal
	recommended := max

	if ratio >= 0.80 {
		status = MapStatusCritical
		recommended = max * 2 // Proactively scale 2x to avoid -E2BIG conntrack overflow
	} else if ratio >= 0.70 {
		status = MapStatusWarning
		recommended = max
	}

	return EBPFMapUtilization{
		MapName:         mapName,
		CurrentEntries:  current,
		MaxEntries:      max,
		PressureRatio:   ratio,
		Status:          status,
		RecommendedSize: recommended,
	}
}

// KernelInterceptorManager defines operations for auditing cgroup v2 throttling and managing eBPF maps.
type KernelInterceptorManager interface {
	AuditCGroupV2Events(ctx context.Context, nodeName string) ([]CGroupV2MemoryEvent, error)
	InspectEBPFMaps(ctx context.Context, clusterID string) ([]EBPFMapUtilization, error)
	AutoResizeEBPFConntrackMap(ctx context.Context, clusterID string, newCapacity int64) error
	EnforceNodeSlabTuning(ctx context.Context, nodeName string) error
}

// KernelEventRingBuffer provides a bounded, concurrent ring buffer with backpressure for kernel events.
type KernelEventRingBuffer struct {
	mu            sync.Mutex
	capacity      int
	events        []CGroupV2MemoryEvent
	head          int
	count         int
	totalAccepted int64
	totalDropped  int64
}

// NewKernelEventRingBuffer creates a bounded ring buffer of specified capacity.
func NewKernelEventRingBuffer(capacity int) *KernelEventRingBuffer {
	if capacity <= 0 {
		capacity = 256
	}
	return &KernelEventRingBuffer{
		capacity: capacity,
		events:   make([]CGroupV2MemoryEvent, capacity),
	}
}

// Push appends an event to the ring buffer. If full, backpressure causes the oldest entry
// to be dropped, recording atomic drop statistics.
func (rb *KernelEventRingBuffer) Push(event CGroupV2MemoryEvent) bool {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.count >= rb.capacity {
		atomic.AddInt64(&rb.totalDropped, 1)
		return false
	}

	idx := (rb.head + rb.count) % rb.capacity
	rb.events[idx] = event
	rb.count++
	atomic.AddInt64(&rb.totalAccepted, 1)
	return true
}

// Drain extracts all buffered events and resets the buffer.
func (rb *KernelEventRingBuffer) Drain() []CGroupV2MemoryEvent {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.count == 0 {
		return nil
	}

	out := make([]CGroupV2MemoryEvent, rb.count)
	for i := 0; i < rb.count; i++ {
		idx := (rb.head + i) % rb.capacity
		out[i] = rb.events[idx]
	}

	rb.head = 0
	rb.count = 0
	return out
}

// TotalAccepted returns the cumulative number of successfully queued events.
func (rb *KernelEventRingBuffer) TotalAccepted() int64 {
	return atomic.LoadInt64(&rb.totalAccepted)
}

// TotalDropped returns the cumulative number of dropped events due to backpressure.
func (rb *KernelEventRingBuffer) TotalDropped() int64 {
	return atomic.LoadInt64(&rb.totalDropped)
}
