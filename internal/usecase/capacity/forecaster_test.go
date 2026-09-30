package capacity

import (
	"context"
	"testing"
	"time"

	domainCapacity "github.com/datdt/k8sselfhost/internal/domain/capacity"
	domainDocker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	usecaseMetrics "github.com/datdt/k8sselfhost/internal/usecase/metrics"
)

type mockMetricsProvider struct {
	snapshot     *usecaseMetrics.SystemOverview
	agentMetrics map[string]*usecaseMetrics.AgentMetrics
}

func (m *mockMetricsProvider) GetLastSnapshot() *usecaseMetrics.SystemOverview {
	return m.snapshot
}

func (m *mockMetricsProvider) GetAgentMetrics() map[string]*usecaseMetrics.AgentMetrics {
	return m.agentMetrics
}

type mockCapacityRepo struct {
	items []domainCapacity.Forecast
}

func (m *mockCapacityRepo) List(ctx context.Context, cluster string) ([]domainCapacity.Forecast, error) {
	return m.items, nil
}

func (m *mockCapacityRepo) Record(ctx context.Context, f *domainCapacity.Forecast) error {
	m.items = append(m.items, *f)
	return nil
}

func TestForecaster_List_GracefulEmptyFallback(t *testing.T) {
	mp := &mockMetricsProvider{}
	forecaster := NewForecaster(mp)

	forecasts, err := forecaster.List(context.Background(), "fleet-primary")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(forecasts) != 0 {
		t.Fatalf("expected empty slice when no metrics available, got %d", len(forecasts))
	}
}

func TestForecaster_List_WithLiveSnapshot(t *testing.T) {
	mp := &mockMetricsProvider{
		snapshot: &usecaseMetrics.SystemOverview{
			TotalNodes:       2,
			TotalCPUPercent:  45.0,
			TotalMemPercent:  72.0,
			TotalDiskPercent: 88.0,
		},
	}

	forecaster := NewForecaster(mp)
	forecasts, err := forecaster.List(context.Background(), "fleet-primary")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(forecasts) != 3 {
		t.Fatalf("expected 3 forecast resources (cpu, memory, storage), got %d", len(forecasts))
	}

	var cpuFc, memFc, diskFc domainCapacity.Forecast
	for _, fc := range forecasts {
		switch fc.ResourceType {
		case domainCapacity.ResourceCPU:
			cpuFc = fc
		case domainCapacity.ResourceMemory:
			memFc = fc
		case domainCapacity.ResourceStorage:
			diskFc = fc
		}
	}

	// CPU: 45% -> healthy (<70%)
	if cpuFc.Status != "healthy" {
		t.Errorf("expected CPU status healthy, got %s", cpuFc.Status)
	}
	if cpuFc.Forecast7d <= 45.0 || cpuFc.Forecast30d <= cpuFc.Forecast7d || cpuFc.Forecast90d <= cpuFc.Forecast30d {
		t.Errorf("expected increasing projection trend for CPU: 7d=%.1f, 30d=%.1f, 90d=%.1f", cpuFc.Forecast7d, cpuFc.Forecast30d, cpuFc.Forecast90d)
	}

	// Memory: 72% -> warning (70-85%)
	if memFc.Status != "warning" {
		t.Errorf("expected Memory status warning, got %s", memFc.Status)
	}

	// Storage: 88% -> critical (>85%)
	if diskFc.Status != "critical" {
		t.Errorf("expected Storage status critical, got %s", diskFc.Status)
	}
	if diskFc.ExhaustionAt == nil {
		t.Errorf("expected exhaustion runway calculated for storage")
	}
}

func TestComputeProjection_Thresholds(t *testing.T) {
	now := time.Now().UTC()

	// Healthy (<70%)
	_, fc30, _, _, status := ComputeProjection(50.0, 0.2, now)
	if status != "healthy" {
		t.Errorf("expected healthy for 50%% with 30d=%.1f, got %s", fc30, status)
	}

	// Warning (70-85%)
	_, fc30, _, _, status = ComputeProjection(75.0, 0.2, now)
	if status != "warning" {
		t.Errorf("expected warning for 75%% with 30d=%.1f, got %s", fc30, status)
	}

	// Critical (>85%)
	_, fc30, _, _, status = ComputeProjection(86.0, 0.2, now)
	if status != "critical" {
		t.Errorf("expected critical for 86%% with 30d=%.1f, got %s", fc30, status)
	}
}

func TestForecaster_Record(t *testing.T) {
	repo := &mockCapacityRepo{}
	forecaster := NewForecaster(nil, WithRepository(repo))

	fc := &domainCapacity.Forecast{
		ID:           "test-fc",
		Cluster:      "fleet-primary",
		ResourceType: domainCapacity.ResourceCPU,
		CurrentUsage: 55.0,
	}

	err := forecaster.Record(context.Background(), fc)
	if err != nil {
		t.Fatalf("unexpected error recording forecast: %v", err)
	}
	if len(repo.items) != 1 {
		t.Fatalf("expected 1 item recorded in repo, got %d", len(repo.items))
	}
}


func TestForecaster_ListNodeHeadroom_EmptyFallback(t *testing.T) {
	mp := &mockMetricsProvider{}
	forecaster := NewForecaster(mp)

	headrooms, err := forecaster.ListNodeHeadroom(context.Background(), "fleet-primary")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(headrooms) != 0 {
		t.Fatalf("expected empty slice when no snapshot nodes, got %d", len(headrooms))
	}
}

func TestForecaster_ListNodeHeadroom_FactualMetrics(t *testing.T) {
	mp := &mockMetricsProvider{
		snapshot: &usecaseMetrics.SystemOverview{
			Nodes: []usecaseMetrics.NodeMetrics{
				{
					NodeID:         "node-wrk-02",
					NodeName:       "k8sworker-down",
					Role:           "worker",
					Status:         "down",
					CPUPercent:     0.0,
					MemoryTotal:    0,
					MemoryUsed:     0,
					MemoryPercent:  0.0,
					ContainerCount: 0,
					RunningCount:   0,
				},
				{
					NodeID:         "node-wrk-01",
					NodeName:       "k8sworker-01",
					Role:           "worker",
					Status:         "ready",
					CPUPercent:     72.0,
					MemoryTotal:    32 * 1024 * 1024 * 1024,
					MemoryUsed:     24 * 1024 * 1024 * 1024,
					MemoryPercent:  75.0,
					ContainerCount: 35,
					RunningCount:   35,
				},
				{
					NodeID:         "node-master-01",
					NodeName:       "k8smaster",
					Role:           "manager",
					Status:         "ready",
					CPUPercent:     25.0,
					MemoryTotal:    16 * 1024 * 1024 * 1024,
					MemoryUsed:     4 * 1024 * 1024 * 1024,
					MemoryPercent:  25.0,
					ContainerCount: 15,
					RunningCount:   15,
				},
			},
		},
		agentMetrics: map[string]*usecaseMetrics.AgentMetrics{
			"node-wrk-01": {
				Hostname: "k8sworker-01",
				CPUCount: 8,
			},
			"node-master-01": {
				Hostname: "k8smaster",
				CPUCount: 4,
			},
		},
	}

	forecaster := NewForecaster(mp)
	items, err := forecaster.ListNodeHeadroom(context.Background(), "fleet-primary")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(items) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(items))
	}

	// Ready nodes first, alphabetical: k8smaster, k8sworker-01, then down: k8sworker-down
	if items[0].Name != "k8smaster" {
		t.Errorf("expected 1st node to be k8smaster, got %s", items[0].Name)
	}
	if items[1].Name != "k8sworker-01" {
		t.Errorf("expected 2nd node to be k8sworker-01, got %s", items[1].Name)
	}
	if items[2].Name != "k8sworker-down" {
		t.Errorf("expected 3rd node to be k8sworker-down, got %s", items[2].Name)
	}

	// 1. Control plane node checks
	cp := items[0]
	if cp.Role != "control-plane" {
		t.Errorf("expected k8smaster role control-plane, got %s", cp.Role)
	}
	if cp.CPUTotalCores != 4.0 {
		t.Errorf("expected k8smaster CPUTotalCores 4.0, got %v", cp.CPUTotalCores)
	}
	if cp.CPUAllocatedCores != 1.0 {
		t.Errorf("expected k8smaster CPUAllocatedCores 1.0, got %v", cp.CPUAllocatedCores)
	}
	if cp.CPUUsagePercent != 25.0 {
		t.Errorf("expected k8smaster CPUUsagePercent 25.0, got %v", cp.CPUUsagePercent)
	}
	if cp.MemTotalGiB != 16.0 {
		t.Errorf("expected k8smaster MemTotalGiB 16.0, got %v", cp.MemTotalGiB)
	}
	if cp.MemAllocatedGiB != 4.0 {
		t.Errorf("expected k8smaster MemAllocatedGiB 4.0, got %v", cp.MemAllocatedGiB)
	}
	if cp.MemUsagePercent != 25.0 {
		t.Errorf("expected k8smaster MemUsagePercent 25.0, got %v", cp.MemUsagePercent)
	}
	if cp.PodCount != 15 {
		t.Errorf("expected k8smaster PodCount 15, got %d", cp.PodCount)
	}
	if cp.PodCapacity != 60 {
		t.Errorf("expected k8smaster PodCapacity 60, got %d", cp.PodCapacity)
	}
	if cp.BinPackingScore != 25.0 {
		t.Errorf("expected k8smaster BinPackingScore 25.0, got %v", cp.BinPackingScore)
	}
	if cp.HeadroomPercent != 75.0 {
		t.Errorf("expected k8smaster HeadroomPercent 75.0, got %v", cp.HeadroomPercent)
	}
	if cp.Status != "healthy" {
		t.Errorf("expected k8smaster Status healthy, got %s", cp.Status)
	}

	// 2. Worker node checks
	wrk := items[1]
	if wrk.Role != "worker" {
		t.Errorf("expected k8sworker-01 role worker, got %s", wrk.Role)
	}
	if wrk.CPUTotalCores != 8.0 {
		t.Errorf("expected k8sworker-01 CPUTotalCores 8.0, got %v", wrk.CPUTotalCores)
	}
	if wrk.CPUAllocatedCores != 5.8 {
		t.Errorf("expected k8sworker-01 CPUAllocatedCores 5.8, got %v", wrk.CPUAllocatedCores)
	}
	if wrk.MemTotalGiB != 32.0 {
		t.Errorf("expected k8sworker-01 MemTotalGiB 32.0, got %v", wrk.MemTotalGiB)
	}
	if wrk.MemAllocatedGiB != 24.0 {
		t.Errorf("expected k8sworker-01 MemAllocatedGiB 24.0, got %v", wrk.MemAllocatedGiB)
	}
	if wrk.PodCount != 35 {
		t.Errorf("expected k8sworker-01 PodCount 35, got %d", wrk.PodCount)
	}
	if wrk.PodCapacity != 70 {
		t.Errorf("expected k8sworker-01 PodCapacity 70, got %d", wrk.PodCapacity)
	}
	if wrk.BinPackingScore != 73.5 {
		t.Errorf("expected k8sworker-01 BinPackingScore 73.5, got %v", wrk.BinPackingScore)
	}
	if wrk.HeadroomPercent != 25.0 {
		t.Errorf("expected k8sworker-01 HeadroomPercent 25.0, got %v", wrk.HeadroomPercent)
	}
	if wrk.Status != "warning" {
		t.Errorf("expected k8sworker-01 Status warning, got %s", wrk.Status)
	}

	// 3. Down node checks
	down := items[2]
	if down.CPUTotalCores != 4.0 {
		t.Errorf("expected k8sworker-down default CPUTotalCores 4.0, got %v", down.CPUTotalCores)
	}
	if down.MemTotalGiB != 8.0 {
		t.Errorf("expected k8sworker-down default MemTotalGiB 8.0, got %v", down.MemTotalGiB)
	}
	if down.Status != "critical" {
		t.Errorf("expected k8sworker-down Status critical, got %s", down.Status)
	}
}

var _ domainCapacity.Repository = (*Forecaster)(nil)
var _ = domainDocker.ComputeHost{}
