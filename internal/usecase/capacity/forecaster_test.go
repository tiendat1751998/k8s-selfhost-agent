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

var _ domainCapacity.Repository = (*Forecaster)(nil)
var _ = domainDocker.ComputeHost{}
