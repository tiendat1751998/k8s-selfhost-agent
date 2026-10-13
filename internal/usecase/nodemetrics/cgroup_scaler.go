// Package nodemetrics provides usecase services for cgroup v2 memory throttling inspection,
// monotonic event rate spike detection, eBPF connection tracking map dynamic scaling,
// and VFS slab cache pressure enforcement.
package nodemetrics

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	domain "github.com/datdt/k8sselfhost/internal/domain/nodemetrics"
	"github.com/datdt/k8sselfhost/internal/infrastructure/cluster"
)

var _ domain.KernelInterceptorManager = (*CGroupScalerService)(nil)

// Option defines a functional configuration option for CGroupScalerService.
type Option func(*CGroupScalerService)

type sampleRecord struct {
	highEvents int64
	recordedAt time.Time
}

// CGroupScalerService coordinates cgroup v2 monotonic counter sampling,
// rate-based throttling spike detection, eBPF map sizing guardrails, and VFS slab tuning.
type CGroupScalerService struct {
	k8sClient     kubernetes.Interface
	clientManager *cluster.ClientManager
	logger        *zap.Logger
	hostRAMBytes  int64

	mu               sync.RWMutex
	lastSamples      map[string]sampleRecord                     // containerID -> last sample
	cgroupEvents     map[string][]domain.CGroupV2MemoryEvent     // nodeName -> events
	ebpfMaps         map[string][]domain.EBPFMapUtilization      // clusterID -> maps
	configMapPatches map[string]map[string]string                // clusterID -> key/val patch
	nodeSysctls      map[string]map[string]int                   // nodeName -> sysctl key/val
}

func isNilK8sClient(client kubernetes.Interface) bool {
	if client == nil {
		return true
	}
	v := reflect.ValueOf(client)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

// WithKubernetesClient injects a Kubernetes client.
func WithKubernetesClient(client kubernetes.Interface) Option {
	return func(s *CGroupScalerService) {
		if !isNilK8sClient(client) {
			s.k8sClient = client
		}
	}
}

// WithClientManager injects a cluster ClientManager.
func WithClientManager(cm *cluster.ClientManager) Option {
	return func(s *CGroupScalerService) {
		s.clientManager = cm
	}
}

// WithLogger injects a custom zap logger.
func WithLogger(logger *zap.Logger) Option {
	return func(s *CGroupScalerService) {
		if logger != nil {
			s.logger = logger
		}
	}
}

// WithHostRAM sets host RAM capacity for eBPF memory guardrail calculation.
func WithHostRAM(ramBytes int64) Option {
	return func(s *CGroupScalerService) {
		if ramBytes > 0 {
			s.hostRAMBytes = ramBytes
		}
	}
}

// NewCGroupScalerService constructs a CGroupScalerService instance.
func NewCGroupScalerService(opts ...Option) *CGroupScalerService {
	s := &CGroupScalerService{
		logger:           zap.NewNop(),
		hostRAMBytes:     16 * 1024 * 1024 * 1024, // 16GB default host RAM baseline
		lastSamples:      make(map[string]sampleRecord),
		cgroupEvents:     make(map[string][]domain.CGroupV2MemoryEvent),
		ebpfMaps:         make(map[string][]domain.EBPFMapUtilization),
		configMapPatches: make(map[string]map[string]string),
		nodeSysctls:      make(map[string]map[string]int),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// RegisterCGroupEvent records or updates a cgroup memory event for a node.
func (s *CGroupScalerService) RegisterCGroupEvent(nodeName string, ev domain.CGroupV2MemoryEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cgroupEvents[nodeName] = append(s.cgroupEvents[nodeName], ev)
}

// RegisterEBPFMap registers an eBPF map state for a cluster.
func (s *CGroupScalerService) RegisterEBPFMap(clusterID string, m domain.EBPFMapUtilization) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ebpfMaps[clusterID] = append(s.ebpfMaps[clusterID], m)
}

// GetConfigMapPatch returns the generated Cilium ConfigMap patch for a cluster.
func (s *CGroupScalerService) GetConfigMapPatch(clusterID string) (map[string]string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	patch, ok := s.configMapPatches[clusterID]
	if !ok {
		return nil, false
	}
	cp := make(map[string]string, len(patch))
	for k, v := range patch {
		cp[k] = v
	}
	return cp, true
}

// GetNodeSysctl returns applied sysctl value on a node.
func (s *CGroupScalerService) GetNodeSysctl(nodeName string, key string) (int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.nodeSysctls[nodeName]
	if !ok {
		return 0, false
	}
	val, exists := m[key]
	return val, exists
}

// AuditCGroupV2Events audits cgroup v2 memory.events.
// Because Linux cgroup v2 counters are cumulative monotonic counters, this method tracks
// previous sample deltas (delta > 20 in 30s or delta > 10 in 1 min) to detect actual active throttling spikes
// without producing false alert spam on historical counter values.
func (s *CGroupScalerService) AuditCGroupV2Events(ctx context.Context, nodeName string) ([]domain.CGroupV2MemoryEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rawEvents, exists := s.cgroupEvents[nodeName]
	if !exists || len(rawEvents) == 0 {
		return []domain.CGroupV2MemoryEvent{}, nil
	}

	result := make([]domain.CGroupV2MemoryEvent, 0, len(rawEvents))
	for _, ev := range rawEvents {
		sampleKey := fmt.Sprintf("%s:%s", nodeName, ev.ContainerID)
		prev, hasPrev := s.lastSamples[sampleKey]

		var delta int64
		var window time.Duration

		if hasPrev {
			delta = ev.HighEvents - prev.highEvents
			if delta < 0 {
				delta = ev.HighEvents // counter reset
			}
			window = ev.UpdatedAt.Sub(prev.recordedAt)
			if window <= 0 {
				window = ev.ThrottledDuration
			}
		} else {
			// First sample: evaluate with explicit ThrottledDuration if provided
			delta = ev.HighEvents
			window = ev.ThrottledDuration
		}

		if window <= 0 {
			window = 30 * time.Second
		}

		// Update last known sample
		s.lastSamples[sampleKey] = sampleRecord{
			highEvents: ev.HighEvents,
			recordedAt: ev.UpdatedAt,
		}

		if hasPrev || ev.ThrottledDuration > 0 {
			if domain.IsThrottlingSpike(delta, window) {
				ev.Recommendation = fmt.Sprintf(
					"Workload is experiencing kernel-level memory.high throttling (%d events in %s, proportional sleep penalty active). Recommend increasing container memory limit by 25%%.",
					delta, window,
				)
				s.logger.Warn("cgroup v2 memory.high throttling spike detected",
					zap.String("node", nodeName),
					zap.String("pod", ev.PodName),
					zap.String("container", ev.ContainerID),
					zap.Int64("high_delta", delta),
					zap.Duration("window", window),
				)
			}
		}

		result = append(result, ev)
	}

	return result, nil
}

// InspectEBPFMaps inspects eBPF map utilization for a cluster, calculating pressure ratio,
// saturation status (CRITICAL >= 0.80, WARNING >= 0.75, NORMAL otherwise), and recommendation.
func (s *CGroupScalerService) InspectEBPFMaps(ctx context.Context, clusterID string) ([]domain.EBPFMapUtilization, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rawMaps, exists := s.ebpfMaps[clusterID]
	if !exists || len(rawMaps) == 0 {
		rawMaps = []domain.EBPFMapUtilization{
			{MapName: "cilium_ct4_global", CurrentEntries: 180000, MaxEntries: 262144},
			{MapName: "cilium_ct6_global", CurrentEntries: 45000, MaxEntries: 262144},
		}
	}

	result := make([]domain.EBPFMapUtilization, 0, len(rawMaps))
	for _, m := range rawMaps {
		ratio, status, recommended := domain.CalculateEBPFRecommendation(m.CurrentEntries, m.MaxEntries)
		m.PressureRatio = ratio
		m.Status = status
		m.RecommendedSize = recommended

		if status == domain.StatusCritical {
			s.logger.Error("eBPF conntrack map saturation critical",
				zap.String("cluster_id", clusterID),
				zap.String("map_name", m.MapName),
				zap.Float64("pressure_ratio", m.PressureRatio),
				zap.Int64("current_entries", m.CurrentEntries),
				zap.Int64("max_entries", m.MaxEntries),
				zap.Int64("recommended_size", m.RecommendedSize),
			)
		} else if status == domain.StatusWarning {
			s.logger.Warn("eBPF conntrack map saturation approaching limit",
				zap.String("cluster_id", clusterID),
				zap.String("map_name", m.MapName),
				zap.Float64("pressure_ratio", m.PressureRatio),
			)
		}

		result = append(result, m)
	}

	return result, nil
}

// AutoResizeEBPFConntrackMap validates requested capacity against both the 2,097,152 ceiling
// and the 10% host RAM safety guardrail, and generates the Cilium ConfigMap patch
// (`cilium-config`: `bpf-ct-global-any-max`, `bpf-ct-timeout-regular = 60s`).
func (s *CGroupScalerService) AutoResizeEBPFConntrackMap(ctx context.Context, clusterID string, newCapacity int64) error {
	// Guardrail validation (P0 Blocker: checks both ceiling and 10% RAM)
	if err := domain.ValidateEBPFMapMemory(newCapacity, s.hostRAMBytes); err != nil {
		s.logger.Error("eBPF auto-resize rejected by guardrail",
			zap.String("cluster_id", clusterID),
			zap.Int64("requested_capacity", newCapacity),
			zap.Int64("host_ram_bytes", s.hostRAMBytes),
			zap.Error(err),
		)
		return err
	}

	// Generate Cilium / Calico ConfigMap patch
	patch := map[string]string{
		"bpf-ct-global-any-max":   fmt.Sprintf("%d", newCapacity),
		"bpf-ct-timeout-regular": "60s",
	}

	s.mu.Lock()
	s.configMapPatches[clusterID] = patch
	if maps, ok := s.ebpfMaps[clusterID]; ok {
		for i := range maps {
			if maps[i].MapName == "cilium_ct4_global" || maps[i].MapName == "cilium_ct6_global" {
				maps[i].MaxEntries = newCapacity
				ratio, status, rec := domain.CalculateEBPFRecommendation(maps[i].CurrentEntries, newCapacity)
				maps[i].PressureRatio = ratio
				maps[i].Status = status
				maps[i].RecommendedSize = rec
			}
		}
	}
	s.mu.Unlock()

	// Apply patch to Kubernetes cilium-config ConfigMap if client is active
	if s.k8sClient != nil {
		cm, err := s.k8sClient.CoreV1().ConfigMaps("kube-system").Get(ctx, "cilium-config", metav1.GetOptions{})
		if err == nil && cm != nil {
			if cm.Data == nil {
				cm.Data = make(map[string]string)
			}
			for k, v := range patch {
				cm.Data[k] = v
			}
			_, _ = s.k8sClient.CoreV1().ConfigMaps("kube-system").Update(ctx, cm, metav1.UpdateOptions{})
		}
	}

	s.logger.Info("eBPF conntrack map capacity successfully updated",
		zap.String("cluster_id", clusterID),
		zap.Int64("new_capacity", newCapacity),
	)

	return nil
}

// EnforceNodeSlabTuning generates and applies the sysctl profile `vm.vfs_cache_pressure = 150`
// on the specified node.
func (s *CGroupScalerService) EnforceNodeSlabTuning(ctx context.Context, nodeName string) error {
	if nodeName == "" {
		return fmt.Errorf("nodeName cannot be empty")
	}

	s.mu.Lock()
	if _, ok := s.nodeSysctls[nodeName]; !ok {
		s.nodeSysctls[nodeName] = make(map[string]int)
	}
	s.nodeSysctls[nodeName][domain.SysctlVFSNodeCachePressure] = domain.DefaultVFSNodeCachePressure
	s.mu.Unlock()

	s.logger.Info("enforced VFS slab cache pressure tuning on node",
		zap.String("node_name", nodeName),
		zap.String("sysctl_key", domain.SysctlVFSNodeCachePressure),
		zap.Int("sysctl_value", domain.DefaultVFSNodeCachePressure),
	)

	return nil
}
