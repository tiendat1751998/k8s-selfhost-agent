package metrics

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

func TestCollector_HostProbe_Enabled(t *testing.T) {
	collector := NewCollector(
		nil, nil, nil, zap.NewNop(),
		WithHostProbe(true),
	)

	overview, err := collector.CollectOnce(context.Background())
	if err != nil {
		t.Fatalf("CollectOnce with host probe failed: %v", err)
	}

	if overview == nil {
		t.Fatal("expected non-nil overview")
	}

	if overview.TotalNodes != 1 {
		t.Errorf("expected TotalNodes = 1 for probed host, got %d", overview.TotalNodes)
	}

	if len(overview.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(overview.Nodes))
	}

	node := overview.Nodes[0]
	if node.Role != "standalone" {
		t.Errorf("expected node role = standalone, got %s", node.Role)
	}
	if node.Status != "ready" {
		t.Errorf("expected node status = ready, got %s", node.Status)
	}
	if node.Source != "host" {
		t.Errorf("expected node source = host, got %s", node.Source)
	}
	if node.MemoryTotal <= 0 {
		t.Errorf("expected positive MemoryTotal, got %d", node.MemoryTotal)
	}
	if node.ContainerCount != 0 {
		t.Errorf("expected 0 containers on host without docker, got %d", node.ContainerCount)
	}
	if overview.TotalContainers != 0 {
		t.Errorf("expected TotalContainers = 0, got %d", overview.TotalContainers)
	}
	if overview.RunningContainers != 0 {
		t.Errorf("expected RunningContainers = 0, got %d", overview.RunningContainers)
	}
	if overview.RequestsPerSec < 0 {
		t.Errorf("expected non-negative RequestsPerSec, got %f", overview.RequestsPerSec)
	}
}

func TestCollector_HostProbe_DisabledDefault(t *testing.T) {
	collector := NewCollector(nil, nil, nil, zap.NewNop())
	overview, err := collector.CollectOnce(context.Background())
	if err != nil {
		t.Fatalf("CollectOnce failed: %v", err)
	}

	if overview.TotalNodes != 0 {
		t.Errorf("expected TotalNodes = 0 when host probe is disabled and no agents, got %d", overview.TotalNodes)
	}
}

func TestTPSCollector_ActivePlatformService_WhenNoContainers(t *testing.T) {
	metricsCollector := NewCollector(
		nil, nil, nil, zap.NewNop(),
		WithHostProbe(true),
		WithRequestCountFn(func() int64 { return 42 }),
	)

	tpsCollector := NewTPSCollector(
		metricsCollector, nil, zap.NewNop(),
		WithTPSRequestCountFn(func() int64 { return 42 }),
	)

	snap, err := tpsCollector.Collect(context.Background())
	if err != nil {
		t.Fatalf("TPS Collect failed: %v", err)
	}

	if snap == nil {
		t.Fatal("expected non-nil snapshot")
	}

	if len(snap.Services) == 0 {
		t.Fatal("expected at least 1 active platform service when host node is present")
	}

	apiSvc := snap.Services[0]
	if apiSvc.ServiceName != "api-server" {
		t.Errorf("expected ServiceName = api-server, got %s", apiSvc.ServiceName)
	}
	if apiSvc.ContainerCount != 0 {
		t.Errorf("expected ContainerCount = 0 when no docker containers are running, got %d", apiSvc.ContainerCount)
	}
	if apiSvc.Status != "healthy" {
		t.Errorf("expected Status = healthy, got %s", apiSvc.Status)
	}
}
