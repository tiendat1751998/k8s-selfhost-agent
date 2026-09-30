package metrics

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestCollector_CollectOnce_StableNodeSorting_EmptyAndSingle(t *testing.T) {
	ctx := context.Background()
	collector := NewCollector(nil, nil, nil, zap.NewNop())

	// Test 0 nodes
	overview, err := collector.CollectOnce(ctx)
	if err != nil {
		t.Fatalf("CollectOnce failed with 0 nodes: %v", err)
	}
	if len(overview.Nodes) != 0 {
		t.Fatalf("expected 0 nodes, got %d", len(overview.Nodes))
	}

	// Test 1 node
	collector.SetAgentMetric("node-1", &AgentMetrics{
		Hostname: "single-node",
		Status:   "online",
		LastSeen: time.Now().UTC(),
	})
	overview, err = collector.CollectOnce(ctx)
	if err != nil {
		t.Fatalf("CollectOnce failed with 1 node: %v", err)
	}
	if len(overview.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(overview.Nodes))
	}
	if overview.Nodes[0].NodeID != "node-1" || overview.Nodes[0].Status != "ready" {
		t.Fatalf("unexpected single node: %+v", overview.Nodes[0])
	}
}

func TestCollector_CollectOnce_StableNodeSorting_MultiNodeDeterminism(t *testing.T) {
	ctx := context.Background()
	collector := NewCollector(nil, nil, nil, zap.NewNop())

	// Setup nodes:
	// Priority 1: Status == "ready" comes before non-ready/offline
	// Priority 2: NodeName alphabetical ascending (case-insensitive)
	// Priority 3: NodeID ascending as absolute tie-breaker
	testNodes := []struct {
		id       string
		hostname string
		status   string
	}{
		{id: "zeta-id", hostname: "zeta-node", status: "online"},
		{id: "alpha-down-id", hostname: "alpha-node", status: "offline"},
		{id: "beta-id", hostname: "Beta-Node", status: "online"},
		{id: "alpha-ready-id", hostname: "alpha-node", status: "online"},
		{id: "tie-id-2", hostname: "gamma-node", status: "offline"},
		{id: "tie-id-1", hostname: "gamma-node", status: "offline"},
		{id: "gamma-ready-id", hostname: "GAMMA-NODE", status: "online"},
		{id: "omega-down-id", hostname: "omega-node", status: "offline"},
	}

	for _, n := range testNodes {
		collector.SetAgentMetric(n.id, &AgentMetrics{
			Hostname: n.hostname,
			Status:   n.status,
			LastSeen: time.Now().UTC(),
		})
	}

	expectedOrder := []struct {
		expectedID     string
		expectedName   string
		expectedStatus string
	}{
		{"alpha-ready-id", "alpha-node", "ready"},
		{"beta-id", "Beta-Node", "ready"},
		{"gamma-ready-id", "GAMMA-NODE", "ready"},
		{"zeta-id", "zeta-node", "ready"},
		{"alpha-down-id", "alpha-node", "down"},
		{"tie-id-1", "gamma-node", "down"},
		{"tie-id-2", "gamma-node", "down"},
		{"omega-down-id", "omega-node", "down"},
	}

	// Run 50 iterations to ensure Go map iteration randomization never causes order fluctuation
	for iter := 0; iter < 50; iter++ {
		overview, err := collector.CollectOnce(ctx)
		if err != nil {
			t.Fatalf("iter %d: CollectOnce failed: %v", iter, err)
		}

		if len(overview.Nodes) != len(expectedOrder) {
			t.Fatalf("iter %d: expected %d nodes, got %d", iter, len(expectedOrder), len(overview.Nodes))
		}

		for i, exp := range expectedOrder {
			actual := overview.Nodes[i]
			if actual.NodeID != exp.expectedID || actual.Status != exp.expectedStatus || actual.NodeName != exp.expectedName {
				t.Fatalf("iter %d, index %d: expected (ID=%s, Name=%s, Status=%s), got (ID=%s, Name=%s, Status=%s)",
					iter, i, exp.expectedID, exp.expectedName, exp.expectedStatus,
					actual.NodeID, actual.NodeName, actual.Status)
			}
		}
	}
}
