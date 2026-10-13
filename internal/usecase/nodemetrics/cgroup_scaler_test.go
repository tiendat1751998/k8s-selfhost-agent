package nodemetrics_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	domain "github.com/datdt/k8sselfhost/internal/domain/nodemetrics"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/nodemetrics"
)

type testKernelManager struct {
	events      []domain.CGroupV2MemoryEvent
	maps        []domain.EBPFMapUtilization
	resizedSize int64
	tunedNode   string
	auditErr    error
	resizeErr   error
	tuneErr     error
}

func (m *testKernelManager) AuditCGroupV2Events(ctx context.Context, nodeName string) ([]domain.CGroupV2MemoryEvent, error) {
	if m.auditErr != nil {
		return nil, m.auditErr
	}
	return m.events, nil
}

func (m *testKernelManager) InspectEBPFMaps(ctx context.Context, clusterID string) ([]domain.EBPFMapUtilization, error) {
	return m.maps, nil
}

func (m *testKernelManager) AutoResizeEBPFConntrackMap(ctx context.Context, clusterID string, newCapacity int64) error {
	if m.resizeErr != nil {
		return m.resizeErr
	}
	m.resizedSize = newCapacity
	return nil
}

func (m *testKernelManager) EnforceNodeSlabTuning(ctx context.Context, nodeName string) error {
	if m.tuneErr != nil {
		return m.tuneErr
	}
	m.tunedNode = nodeName
	return nil
}

type testAlertSink struct {
	alerts []domain.SREAlertEvent
}

func (s *testAlertSink) EmitAlert(ctx context.Context, event domain.SREAlertEvent) error {
	s.alerts = append(s.alerts, event)
	return nil
}

func TestCGroupScalerUsecase_ParseCGroupV2MemoryEvents(t *testing.T) {
	rawContent := `low 14
high 85
max 3
oom 2
oom_kill 1`

	low, high, max, oom, oomKill := usecase.ParseCGroupV2MemoryEvents(rawContent)
	if low != 14 || high != 85 || max != 3 || oom != 2 || oomKill != 1 {
		t.Fatalf("unexpected parsed values: low=%d, high=%d, max=%d, oom=%d, oomKill=%d",
			low, high, max, oom, oomKill)
	}
}

func TestCGroupScalerUsecase_AuditCGroupThrottling(t *testing.T) {
	events := []domain.CGroupV2MemoryEvent{
		{
			ContainerID:       "container-normal",
			PodName:           "api-gateway",
			Namespace:         "prod",
			LowEvents:         0,
			HighEvents:        0,
			MaxEvents:         0,
			OOMKillEvents:     0,
			ThrottledDuration: 0,
			UpdatedAt:         time.Now(),
		},
		{
			ContainerID:       "container-throttled",
			PodName:           "checkout-service",
			Namespace:         "prod",
			LowEvents:         5,
			HighEvents:        120, // Sleep penalty throttle active
			MaxEvents:         1,
			OOMKillEvents:     1,
			ThrottledDuration: 850 * time.Millisecond,
			UpdatedAt:         time.Now(),
		},
	}

	mgr := &testKernelManager{events: events}
	sink := &testAlertSink{}
	sc := usecase.NewCGroupScalerUsecase(mgr, sink)

	ctx := context.Background()
	audited, recommendations, err := sc.AuditCGroupThrottling(ctx, "worker-node-1")
	if err != nil {
		t.Fatalf("unexpected error auditing cgroup: %v", err)
	}

	if len(audited) != 2 {
		t.Fatalf("expected 2 audited events, got %d", len(audited))
	}
	if len(recommendations) != 1 {
		t.Fatalf("expected 1 recommendation for throttled container, got %d", len(recommendations))
	}

	rec := recommendations[0]
	if rec.ContainerID != "container-throttled" {
		t.Fatalf("expected recommendation for container-throttled, got %s", rec.ContainerID)
	}
	if !strings.Contains(rec.ActionRecommended, "memory.high.factor = 0") {
		t.Fatalf("expected recommendation to contain 'memory.high.factor = 0', got: %s", rec.ActionRecommended)
	}

	// Verify alert emission
	if len(sink.alerts) == 0 {
		t.Fatalf("expected SRE alert to be emitted for throttled container")
	}
}

func TestCGroupScalerUsecase_MonitorAndScaleEBPFMaps(t *testing.T) {
	maps := []domain.EBPFMapUtilization{
		domain.NewEBPFMapUtilization("cilium_ct_any4_global", 450000, 512000), // > 80% (approx 87.8%)
		domain.NewEBPFMapUtilization("cilium_ipcache", 12000, 100000),        // 12%
	}

	mgr := &testKernelManager{maps: maps}
	sink := &testAlertSink{}
	sc := usecase.NewCGroupScalerUsecase(mgr, sink)

	ctx := context.Background()
	results, err := sc.MonitorAndScaleEBPFMaps(ctx, "cluster-eu-west")
	if err != nil {
		t.Fatalf("unexpected error monitoring ebpf maps: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 maps monitored, got %d", len(results))
	}

	// Verify auto-resize call was triggered for map over 80%
	if mgr.resizedSize != 1024000 {
		t.Fatalf("expected conntrack map to be resized to 1024000, got %d", mgr.resizedSize)
	}

	if len(sink.alerts) == 0 {
		t.Fatalf("expected SRE alert emitted for critical eBPF map pressure")
	}
}

func TestCGroupScalerUsecase_TuneKernelSlabCache(t *testing.T) {
	mgr := &testKernelManager{}
	sink := &testAlertSink{}
	sc := usecase.NewCGroupScalerUsecase(mgr, sink)

	ctx := context.Background()
	report, err := sc.TuneKernelSlabCache(ctx, "k8s-node-02")
	if err != nil {
		t.Fatalf("unexpected error tuning slab cache: %v", err)
	}

	if report.NodeName != "k8s-node-02" {
		t.Fatalf("expected node k8s-node-02, got %s", report.NodeName)
	}
	if report.VFSCachePressure != 150 {
		t.Fatalf("expected vfs_cache_pressure = 150, got %d", report.VFSCachePressure)
	}
	if !strings.Contains(report.Recommendation, "tmpfs") {
		t.Fatalf("expected tmpfs recommendation, got %s", report.Recommendation)
	}
	if mgr.tunedNode != "k8s-node-02" {
		t.Fatalf("expected manager to tune node k8s-node-02, got %s", mgr.tunedNode)
	}
}

func TestCGroupScalerUsecase_ErrorHandling(t *testing.T) {
	expectedErr := errors.New("network timeout communicating with node agent")
	mgr := &testKernelManager{auditErr: expectedErr, resizeErr: expectedErr, tuneErr: expectedErr}
	sc := usecase.NewCGroupScalerUsecase(mgr, nil)
	ctx := context.Background()

	_, _, err := sc.AuditCGroupThrottling(ctx, "node-err")
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected wrapped auditErr, got %v", err)
	}

	mgr.maps = []domain.EBPFMapUtilization{
		domain.NewEBPFMapUtilization("cilium_ct_any4_global", 450000, 512000),
	}
	_, err = sc.MonitorAndScaleEBPFMaps(ctx, "cluster-err")
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected wrapped resizeErr, got %v", err)
	}

	_, err = sc.TuneKernelSlabCache(ctx, "node-err")
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected wrapped tuneErr, got %v", err)
	}
}
