// Package nodemetrics provides domain models, interfaces, and evaluation rules for
// Linux cgroup v2 memory throttling interception, eBPF connection tracking map
// auto-scaling, and kernel slab cache reclamation tuning.
package nodemetrics

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// RFC 7807 error codes for kernel and eBPF events.
const (
	CodeCGroupV2MemoryThrottled = "ERR_CGROUPV2_MEMORY_HIGH_THROTTLED"
	CodeEBPFMapSaturated        = "ERR_EBPF_MAP_SATURATED"
	CodeEBPFMemoryLimitExceeded = "ERR_EBPF_MAP_MEMORY_EXCEEDS_HOST_LIMIT"
	CodeSlabTuningFailed        = "ERR_SLAB_TUNING_FAILED"
)

// Sentinel domain errors.
var (
	ErrCGroupV2MemoryThrottled = &domainerrors.DomainError{
		Code:    CodeCGroupV2MemoryThrottled,
		Message: "Container is experiencing kernel-level memory.high throttling.",
		Err:     errors.New("kernel proportional sleep penalty active"),
	}
	ErrEBPFMapSaturated = &domainerrors.DomainError{
		Code:    CodeEBPFMapSaturated,
		Message: "eBPF map utilization exceeded critical threshold (>80%). Silent packet drop imminent.",
		Err:     errors.New("ebpf map saturation limit reached"),
	}
	ErrEBPFMemoryLimitExceeded = &domainerrors.DomainError{
		Code:    CodeEBPFMemoryLimitExceeded,
		Message: "Requested eBPF map capacity exceeds safe host memory limit (10% RAM guardrail).",
		Err:     errors.New("ebpf memory guardrail violation"),
	}
)

// Operational status constants for eBPF maps.
const (
	StatusNormal   = "NORMAL"
	StatusWarning  = "WARNING"
	StatusCritical = "CRITICAL"

	// MaxEBPFMapEntries defines the hard ceiling (2^21 entries = 2,097,152) for Cilium/Calico conntrack maps.
	MaxEBPFMapEntries int64 = 2097152

	// DefaultBytesPerEBPFEntry accounts for tuple + entry struct + bpf_htab_elem overhead in kernel slab memory.
	DefaultBytesPerEBPFEntry int64 = 256

	// MaxHostRAMFraction is the maximum allowed fraction of host RAM dedicated to eBPF conntrack maps (10%).
	MaxHostRAMFraction = 0.10

	// SysctlVFSNodeCachePressure is the Linux sysctl key for VFS dentry/inode reclamation.
	SysctlVFSNodeCachePressure = "vm.vfs_cache_pressure"

	// DefaultVFSNodeCachePressure is the tuned value (150) to aggressively reclaim dentry and inode slab caches.
	DefaultVFSNodeCachePressure = 150
)

// CGroupV2MemoryEvent represents parsed memory.events metrics for a cgroup v2 container.
type CGroupV2MemoryEvent struct {
	ContainerID       string        `json:"container_id"`
	PodName           string        `json:"pod_name"`
	Namespace         string        `json:"namespace"`
	LowEvents         int64         `json:"low_events"`
	HighEvents        int64         `json:"high_events"` // Number of kernel proportional sleep penalties
	MaxEvents         int64         `json:"max_events"`  // Number of times hitting OOM ceiling
	OOMKillEvents     int64         `json:"oom_kill_events"`
	ThrottledDuration time.Duration `json:"throttled_duration"`
	UpdatedAt         time.Time     `json:"updated_at"`
	Recommendation    string        `json:"recommendation,omitempty"`
}

// EBPFMapUtilization represents current saturation and auto-scaling metrics of an eBPF map.
type EBPFMapUtilization struct {
	MapName         string  `json:"map_name"`
	CurrentEntries  int64   `json:"current_entries"`
	MaxEntries      int64   `json:"max_entries"`
	PressureRatio   float64 `json:"pressure_ratio"` // Current / Max
	Status          string  `json:"status"`         // NORMAL, WARNING, CRITICAL
	RecommendedSize int64   `json:"recommended_size"`
}

// KernelInterceptorManager defines operations for monitoring cgroup v2 throttling,
// scaling eBPF maps, and tuning kernel slab memory.
type KernelInterceptorManager interface {
	AuditCGroupV2Events(ctx context.Context, nodeName string) ([]CGroupV2MemoryEvent, error)
	InspectEBPFMaps(ctx context.Context, clusterID string) ([]EBPFMapUtilization, error)
	AutoResizeEBPFConntrackMap(ctx context.Context, clusterID string, newCapacity int64) error
	EnforceNodeSlabTuning(ctx context.Context, nodeName string) error
}

// ParseCGroupMemoryEvents parses raw content from a cgroup v2 `memory.events` file into a CGroupV2MemoryEvent.
func ParseCGroupMemoryEvents(raw string) (CGroupV2MemoryEvent, error) {
	var event CGroupV2MemoryEvent
	event.UpdatedAt = time.Now().UTC()

	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		val, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue
		}

		switch fields[0] {
		case "low":
			event.LowEvents = val
		case "high":
			event.HighEvents = val
		case "max":
			event.MaxEvents = val
		case "oom":
			// Handled in max/oom_kill
		case "oom_kill":
			event.OOMKillEvents = val
		}
	}

	if err := scanner.Err(); err != nil {
		return event, fmt.Errorf("reading cgroup memory events: %w", err)
	}

	return event, nil
}

// IsThrottlingSpike detects whether a container is undergoing direct memory reclaim delay spikes:
// > 20 events in 30s OR > 10 events in 1 minute.
func IsThrottlingSpike(highDelta int64, window time.Duration) bool {
	if window <= 30*time.Second {
		return highDelta > 20
	}
	if window <= 1*time.Minute {
		return highDelta > 10
	}
	ratePerSec := float64(highDelta) / window.Seconds()
	return ratePerSec > (10.0 / 60.0)
}

// CalculateEBPFRecommendation calculates map pressure ratio, saturation status,
// and recommended scaled capacity (doubling current max, capped at 2,097,152 entries).
func CalculateEBPFRecommendation(currentEntries, maxEntries int64) (float64, string, int64) {
	if maxEntries <= 0 {
		return 0, StatusNormal, 0
	}

	ratio := float64(currentEntries) / float64(maxEntries)

	var status string
	var recommended int64

	if ratio >= 0.80 {
		status = StatusCritical
		recommended = maxEntries * 2
	} else if ratio >= 0.75 {
		status = StatusWarning
		recommended = maxEntries * 2
	} else {
		status = StatusNormal
		recommended = maxEntries
	}

	if recommended > MaxEBPFMapEntries {
		recommended = MaxEBPFMapEntries
	}

	return ratio, status, recommended
}

// ValidateEBPFMapMemory ensures that requested map size does not exceed the 10% host RAM guardrail
// or the hard ceiling of 2,097,152 entries.
func ValidateEBPFMapMemory(newCapacity int64, hostRAMBytes int64) error {
	if newCapacity <= 0 {
		return errors.New("capacity must be greater than zero")
	}
	if newCapacity > MaxEBPFMapEntries {
		return fmt.Errorf("%w: requested capacity %d exceeds absolute ceiling %d", ErrEBPFMemoryLimitExceeded, newCapacity, MaxEBPFMapEntries)
	}

	if hostRAMBytes <= 0 {
		hostRAMBytes = 4 * 1024 * 1024 * 1024 // 4GB default safe fallback
	}

	requiredBytes := newCapacity * DefaultBytesPerEBPFEntry
	maxAllowedBytes := int64(float64(hostRAMBytes) * MaxHostRAMFraction)

	if requiredBytes > maxAllowedBytes {
		return fmt.Errorf("%w: required %d bytes exceeds 10%% host RAM limit (%d bytes)",
			ErrEBPFMemoryLimitExceeded, requiredBytes, maxAllowedBytes)
	}

	return nil
}
