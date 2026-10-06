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
		name     string
		input    string
		labels   map[string]string
		expected string
	}{
		{"k8s container name metadata", "/k8s_web_frontend_123", nil, "frontend"},
		{"k8s pod infra metadata", "k8s_POD_nginx_kube-system_uid_0", nil, "kube-system"},
		{"dashboard container does not match db substring", "dashboard", nil, "default"},
		{"custom service without labels", "custom-service", nil, "default"},
		{"k8s pod namespace label", "prometheus-server", map[string]string{"io.kubernetes.pod.namespace": "monitoring"}, "monitoring"},
		{"docker stack namespace label", "traefik-proxy", map[string]string{"com.docker.stack.namespace": "ingress"}, "ingress"},
		{"docker compose project label", "redis-cache", map[string]string{"com.docker.compose.project": "ecommerce"}, "ecommerce"},
		{"generic namespace label", "vault-sec", map[string]string{"namespace": "security"}, "security"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractNamespace(tc.input, tc.labels)
			if got != tc.expected {
				t.Errorf("ExtractNamespace(%q, %v) = %q, expected %q", tc.input, tc.labels, got, tc.expected)
			}
		})
	}
}

func TestCostCalculator_GetNamespaceCosts_NoArtificialFloors(t *testing.T) {
	mp := &mockMetricsProvider{
		snapshot: &usecaseMetrics.SystemOverview{
			Containers: []usecaseMetrics.ContainerMetrics{
				{
					ContainerID:   "c-micro",
					ContainerName: "micro-worker",
					State:         "running",
					CPUPercent:    1.0, // 0.01 vCPU (< 0.1 vCPU floor)
					MemoryUsed:    50 * 1024 * 1024, // ~0.05 GB (< 0.25 GB floor)
					MemoryPercent: 1.0,
					Labels:        map[string]string{"namespace": "microservices"},
				},
			},
		},
	}

	calc := NewCalculator(nil, mp)
	nsCosts, err := calc.GetNamespaceCosts(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nsCosts) != 1 {
		t.Fatalf("expected 1 namespace cost, got %d", len(nsCosts))
	}

	c := nsCosts[0]
	if c.Namespace != "microservices" {
		t.Errorf("expected namespace 'microservices', got %s", c.Namespace)
	}
	// CPU: 1% = 0.01 vCPU. Old code forced 0.10 vCPU.
	if c.CPURequested != "0.01 vCPU" {
		t.Errorf("expected factual '0.01 vCPU', got %s (artificial floor detected)", c.CPURequested)
	}
	// Memory: 50MB / 1024^3 = 0.05 GB. Old code forced 0.25 GB.
	if c.MemoryRequested != "0.05 GB" {
		t.Errorf("expected factual '0.05 GB', got %s (artificial floor detected)", c.MemoryRequested)
	}
	// Cost: (0.01*0.035 + 0.05*0.0045)*720 = (0.00035 + 0.000225)*720 = 0.414 -> rounds to 0. Old code forced $1.00.
	if c.MonthlyCost != 0 {
		t.Errorf("expected factual cost 0, got %d (artificial price floor detected)", c.MonthlyCost)
	}
}

var _ domainCost.Repository = (*Calculator)(nil)
