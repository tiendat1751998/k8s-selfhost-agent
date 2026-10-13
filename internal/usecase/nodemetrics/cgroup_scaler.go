package nodemetrics

import (
	"bufio"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/nodemetrics"
)

// Constants for SRE alert codes and kernel tuning thresholds.
const (
	ErrCodeCGroupThrottlingAlert = "ERR_CGROUP_MEMORY_HIGH_THROTTLED"
	ErrCodeEBPFMapPressureAlert  = "ERR_EBPF_MAP_CAPACITY_CRITICAL"
	ErrCodeSlabCacheTuningAlert  = "ERR_KERNEL_SLAB_PRESSURE_EVICTION_RISK"

	DefaultVFSCachePressure     = 150
	EBPFPressureCriticalThreshold = 0.80
)

// Physical Bottleneck & Failure Mode Documentation:
// 1. Kernel Context Switches & Cross-Core TLB Invalidation:
//    cgroup v2 memory event polling over /sys/fs/cgroup/* generates heavy inotify wakeups. If threads
//    repeatedly cross CPU cores or rapidly inspect hierarchies, kernel TLB shootdowns degrade IPC.
// 2. eBPF Map Pinned Memory & RLIMIT_MEMLOCK:
//    Cilium conntrack maps (bpf_ct_any4_global) reside in non-swappable kernel memory. Scaling maps dynamically
//    without sufficient RLIMIT_MEMLOCK or careful atomic resize can fail or drop connections. Pre-allocating
//    or non-disruptive map pinning avoids dropping active TCP connections during runtime expansion.
// 3. cgroup v2 memory.high Proportional Sleep Penalties:
//    Unlike memory.max (which invokes the immediate OOM killer), breaching memory.high induces synchronous
//    throttling delays (sleep penalty) in alloc_pages(). Pods experience invisible latency degradation
//    without standard OOM alerts unless memory.events is actively parsed. Setting memory.high.factor = 0
//    or elevating memory limits resolves the silent stall.
// 4. Kernel Slab Dentry/Inode Cache Inflation:
//    Build pods or high-churn IO containers accumulate massive dentry and inode slab objects in memory.
//    Because the Linux kernel treats slab reclaim cautiously, Kubelet's cgroup manager may mistake slab
//    cache as active memory pressure and evict innocent workloads. Applying vm.vfs_cache_pressure = 150
//    forces aggressive slab reclaim, while mounting high-IO scratch dirs as tmpfs bypasses inode caches.

// CGroupRecommendation holds operational advice when cgroup v2 throttling or OOM is detected.
type CGroupRecommendation struct {
	ContainerID       string `json:"container_id"`
	PodName           string `json:"pod_name"`
	Namespace         string `json:"namespace"`
	Issue             string `json:"issue"`
	ActionRecommended string `json:"action_recommended"`
}

// SlabTuningReport describes the kernel slab cache state and applied remediation.
type SlabTuningReport struct {
	NodeName         string    `json:"node_name"`
	VFSCachePressure int       `json:"vfs_cache_pressure"`
	Recommendation   string    `json:"recommendation"`
	AppliedAt        time.Time `json:"applied_at"`
}

// CGroupScalerUsecase orchestrates cgroup v2 event watching, eBPF map scaling, and slab cache tuning.
type CGroupScalerUsecase struct {
	interceptor nodemetrics.KernelInterceptorManager
	alertSink   nodemetrics.SREAlertSink
}

// NewCGroupScalerUsecase instantiates the cgroup and eBPF scaling engine.
func NewCGroupScalerUsecase(
	interceptor nodemetrics.KernelInterceptorManager,
	alertSink nodemetrics.SREAlertSink,
) *CGroupScalerUsecase {
	return &CGroupScalerUsecase{
		interceptor: interceptor,
		alertSink:   alertSink,
	}
}

// ParseCGroupV2MemoryEvents parses the raw newline-delimited key-value contents of a cgroup v2 `memory.events` file.
// Format:
//   low <count>
//   high <count>
//   max <count>
//   oom <count>
//   oom_kill <count>
func ParseCGroupV2MemoryEvents(rawContent string) (low, high, max, oom, oomKill int64) {
	scanner := bufio.NewScanner(strings.NewReader(rawContent))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		val, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			continue
		}
		switch parts[0] {
		case "low":
			low = val
		case "high":
			high = val
		case "max":
			max = val
		case "oom":
			oom = val
		case "oom_kill":
			oomKill = val
		}
	}
	return
}

// AuditCGroupThrottling inspects memory.events for containers on a node, detecting silent proportional
// sleep penalty throttles and recommending memory limit adjustments or memory.high.factor = 0.
func (u *CGroupScalerUsecase) AuditCGroupThrottling(
	ctx context.Context,
	nodeName string,
) ([]nodemetrics.CGroupV2MemoryEvent, []CGroupRecommendation, error) {
	if u.interceptor == nil {
		return nil, nil, fmt.Errorf("kernel interceptor manager not configured")
	}

	events, err := u.interceptor.AuditCGroupV2Events(ctx, nodeName)
	if err != nil {
		return nil, nil, fmt.Errorf("auditing cgroup v2 events for node %s: %w", nodeName, err)
	}

	var recommendations []CGroupRecommendation

	for _, evt := range events {
		if evt.HighEvents > 0 || evt.OOMKillEvents > 0 {
			issueDesc := fmt.Sprintf(
				"Detected %d memory.high sleep penalty delays and %d OOM kills (Throttled duration: %s)",
				evt.HighEvents,
				evt.OOMKillEvents,
				evt.ThrottledDuration,
			)

			action := "Increase container memory limit, or configure memory.high.factor = 0 to eliminate synchronous sleep penalty delays"
			if evt.OOMKillEvents > 0 {
				action = "Urgent: Increase container memory limit and review memory leak. Configure memory.high.factor = 0 to prevent latency degradation prior to OOM."
			}

			recommendations = append(recommendations, CGroupRecommendation{
				ContainerID:       evt.ContainerID,
				PodName:           evt.PodName,
				Namespace:         evt.Namespace,
				Issue:             issueDesc,
				ActionRecommended: action,
			})

			if u.alertSink != nil {
				alert := nodemetrics.SREAlertEvent{
					ID:        fmt.Sprintf("cgroup-high-%s-%d", evt.ContainerID, time.Now().UnixNano()),
					NodeName:  nodeName,
					Severity:  "WARNING",
					ErrorCode: ErrCodeCGroupThrottlingAlert,
					Message: fmt.Sprintf(
						"Pod '%s/%s' (Container %s) on node '%s' is suffering cgroup v2 sleep penalty throttling: %d high events, %s duration.",
						evt.Namespace,
						evt.PodName,
						evt.ContainerID,
						nodeName,
						evt.HighEvents,
						evt.ThrottledDuration,
					),
					CreatedAt: time.Now().UTC(),
				}
				_ = u.alertSink.EmitAlert(ctx, alert)
			}
		}
	}

	return events, recommendations, nil
}

// MonitorAndScaleEBPFMaps inspects eBPF map utilization (e.g. cilium_bpf_map_pressure) and triggers
// dynamic auto-scaling of `bpf-ct-global-any-max` when occupancy exceeds 80% without dropping active TCP connections.
func (u *CGroupScalerUsecase) MonitorAndScaleEBPFMaps(
	ctx context.Context,
	clusterID string,
) ([]nodemetrics.EBPFMapUtilization, error) {
	if u.interceptor == nil {
		return nil, fmt.Errorf("kernel interceptor manager not configured")
	}

	maps, err := u.interceptor.InspectEBPFMaps(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("inspecting ebpf maps for cluster %s: %w", clusterID, err)
	}

	for _, m := range maps {
		if m.PressureRatio >= EBPFPressureCriticalThreshold || m.Status == nodemetrics.MapStatusCritical {
			newCap := m.RecommendedSize
			if newCap <= m.MaxEntries {
				newCap = m.MaxEntries * 2
			}

			if err := u.interceptor.AutoResizeEBPFConntrackMap(ctx, clusterID, newCap); err != nil {
				return nil, fmt.Errorf("auto-resizing ebpf map %s to capacity %d: %w", m.MapName, newCap, err)
			}

			if u.alertSink != nil {
				alert := nodemetrics.SREAlertEvent{
					ID:        fmt.Sprintf("ebpf-scale-%s-%d", m.MapName, time.Now().UnixNano()),
					NodeName:  clusterID,
					Severity:  "CRITICAL",
					ErrorCode: ErrCodeEBPFMapPressureAlert,
					Message: fmt.Sprintf(
						"eBPF map '%s' occupancy reached %.1f%% (>80%% threshold). Scaled capacity from %d to %d without dropping TCP connections.",
						m.MapName,
						m.PressureRatio*100,
						m.MaxEntries,
						newCap,
					),
					CreatedAt: time.Now().UTC(),
				}
				_ = u.alertSink.EmitAlert(ctx, alert)
			}
		}
	}

	return maps, nil
}

// TuneKernelSlabCache evaluates dentry and inode cache pressure, applies vm.vfs_cache_pressure = 150,
// and recommends tmpfs mounts to prevent false Kubelet memory pressure evictions.
func (u *CGroupScalerUsecase) TuneKernelSlabCache(
	ctx context.Context,
	nodeName string,
) (*SlabTuningReport, error) {
	if u.interceptor == nil {
		return nil, fmt.Errorf("kernel interceptor manager not configured")
	}

	if err := u.interceptor.EnforceNodeSlabTuning(ctx, nodeName); err != nil {
		return nil, fmt.Errorf("enforcing slab tuning on node %s: %w", nodeName, err)
	}

	report := &SlabTuningReport{
		NodeName:         nodeName,
		VFSCachePressure: DefaultVFSCachePressure,
		Recommendation:   "Applied vm.vfs_cache_pressure = 150. Recommend configuring tmpfs mounts for ephemeral scratch directories (e.g. /tmp, build workspace) to prevent inode/dentry slab inflation and false Kubelet evictions.",
		AppliedAt:        time.Now().UTC(),
	}

	if u.alertSink != nil {
		alert := nodemetrics.SREAlertEvent{
			ID:        fmt.Sprintf("slab-tune-%s-%d", nodeName, time.Now().UnixNano()),
			NodeName:  nodeName,
			Severity:  "INFO",
			ErrorCode: ErrCodeSlabCacheTuningAlert,
			Message: fmt.Sprintf(
				"Enforced kernel slab tuning on node '%s': vm.vfs_cache_pressure set to %d. Recommended tmpfs scratch mounts.",
				nodeName,
				DefaultVFSCachePressure,
			),
			CreatedAt: time.Now().UTC(),
		}
		_ = u.alertSink.EmitAlert(ctx, alert)
	}

	return report, nil
}
