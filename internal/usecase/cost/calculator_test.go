package cost

import (
	"context"
	"testing"

	domainCost "github.com/datdt/k8sselfhost/internal/domain/cost"
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

type mockComputeHostProvider struct {
	hosts []domainDocker.ComputeHost
	err   error
}

func (m *mockComputeHostProvider) ListAll(ctx context.Context) ([]domainDocker.ComputeHost, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.hosts, nil
}

func TestCostCalculator_GetClusterCosts_EmptyFallback(t *testing.T) {
	mp := &mockMetricsProvider{}
	hp := &mockComputeHostProvider{}
	calc := NewCalculator(hp, mp)

	costs, err := calc.GetClusterCosts(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(costs) != 0 {
		t.Fatalf("expected empty slice for no hosts, got %d items", len(costs))
	}
}

func TestCostCalculator_GetClusterCosts_WithHosts(t *testing.T) {
	hp := &mockComputeHostProvider{
		hosts: []domainDocker.ComputeHost{
			{ID: "host-1", Name: "k8smaster"},
			{ID: "host-2", Name: "worker1"},
		},
	}
	mp := &mockMetricsProvider{
		agentMetrics: map[string]*usecaseMetrics.AgentMetrics{
			"host-1": {
				Hostname: "k8smaster",
				CPUCount: 4,
				MemTotal: 16 * 1024 * 1024 * 1024, // 16GB
				DiskTotal: 100 * 1024 * 1024 * 1024, // 100GB
				Status:   "online",
			},
			"host-2": {
				Hostname: "worker1",
				CPUCount: 8,
				MemTotal: 32 * 1024 * 1024 * 1024, // 32GB
				DiskTotal: 200 * 1024 * 1024 * 1024, // 200GB
				Status:   "online",
			},
		},
	}

	calc := NewCalculator(hp, mp)
	costs, err := calc.GetClusterCosts(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(costs) != 1 {
		t.Fatalf("expected 1 cluster cost aggregate, got %d", len(costs))
	}

	c := costs[0]
	// CPU: (4 + 8) * 0.035 * 720 = 12 * 25.2 = 302.4
	// RAM: (16 + 32) * 0.0045 * 720 = 48 * 3.24 = 155.52
	// Storage: (100 + 200) * 0.0001 * 720 = 300 * 0.072 = 21.6
	// Total: 302.4 + 155.52 + 21.6 = 479.52 -> 480
	if c.MonthlyCost != 480 {
		t.Errorf("expected monthly cost ~480, got %d", c.MonthlyCost)
	}
	if c.CPUCost != 302 {
		t.Errorf("expected CPU cost ~302, got %d", c.CPUCost)
	}
	if c.MemoryCost != 156 {
		t.Errorf("expected Memory cost ~156, got %d", c.MemoryCost)
	}
}

func TestCostCalculator_GetNamespaceCosts_And_Waste(t *testing.T) {
	mp := &mockMetricsProvider{
		snapshot: &usecaseMetrics.SystemOverview{
			Containers: []usecaseMetrics.ContainerMetrics{
				{
					ContainerID:   "c1234567890ab",
					ContainerName: "k8s_app_production_pod1",
					State:         "running",
					CPUPercent:    45.0,
					MemoryUsed:    2 * 1024 * 1024 * 1024, // 2GB
					MemoryPercent: 50.0,
				},
				{
					ContainerID:   "c9876543210fe",
					ContainerName: "monitoring-prometheus",
					State:         "running",
					CPUPercent:    2.0, // <10% -> waste
					MemoryUsed:    256 * 1024 * 1024,
					MemoryPercent: 4.0, // <10% -> waste
				},
				{
					ContainerID:   "c555555555555",
					ContainerName: "redis-cache",
					State:         "running",
					CPUPercent:    1.0, // <10% -> waste
					MemoryUsed:    512 * 1024 * 1024,
					MemoryPercent: 15.0,
				},
			},
		},
	}

	calc := NewCalculator(nil, mp)

	// Namespaces
	nsCosts, err := calc.GetNamespaceCosts(context.Background())
	if err != nil {
		t.Fatalf("unexpected error getting namespace costs: %v", err)
	}
	if len(nsCosts) == 0 {
		t.Fatal("expected namespace costs, got none")
	}

	// Waste
	waste, err := calc.GetResourceWaste(context.Background())
	if err != nil {
		t.Fatalf("unexpected error getting waste: %v", err)
	}
	if len(waste) != 2 {
		t.Fatalf("expected 2 waste items, got %d", len(waste))
	}

	// Prometheus should have high/critical severity since both CPU and Mem < 5%
	var foundProm bool
	for _, w := range waste {
		if w.Resource == "monitoring-prometheus" {
			foundProm = true
			if w.Severity != "critical" && w.Severity != "high" {
				t.Errorf("expected critical or high severity for prometheus, got %s", w.Severity)
			}
			if w.Type != "idle_workload" {
				t.Errorf("expected idle_workload type, got %s", w.Type)
			}
		}
	}
	if !foundProm {
		t.Error("prometheus waste item not found")
	}
}

func TestExtractNamespace(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/k8s_web_frontend_123", "frontend"},
		{"prometheus-server", "monitoring"},
		{"traefik-proxy", "ingress"},
		{"postgres-primary", "database"},
		{"k8s-agent-daemon", "system"},
		{"vault-sec", "security"},
		{"custom-service", "default"},
	}

	for _, tc := range tests {
		got := ExtractNamespace(tc.input)
		if got != tc.expected {
			t.Errorf("ExtractNamespace(%q) = %q, expected %q", tc.input, got, tc.expected)
		}
	}
}

var _ domainCost.Repository = (*Calculator)(nil)
