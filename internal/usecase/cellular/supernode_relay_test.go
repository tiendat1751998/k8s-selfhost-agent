package cellular

import (
	"fmt"
	"testing"
)

func TestElectSupernode(t *testing.T) {
	nodes := []MeshNode{
		{
			ID:             "node-low-spec",
			RegionID:       "ap-southeast-1",
			HasPublicIP:    false,
			BandwidthMbps:  100,
			CPUHeadroomPct: 20,
			UptimeSeconds:  3600,
		},
		{
			ID:             "node-high-spec-private",
			RegionID:       "ap-southeast-1",
			HasPublicIP:    false,
			BandwidthMbps:  10000,
			CPUHeadroomPct: 80,
			UptimeSeconds:  86400,
		},
		{
			ID:             "node-relay-candidate",
			RegionID:       "ap-southeast-1",
			HasPublicIP:    true,
			PublicIP:       "203.0.113.10",
			BandwidthMbps:  1000,
			CPUHeadroomPct: 75,
			UptimeSeconds:  172800,
		},
	}

	best := ElectSupernode(nodes)
	if best.ID != "node-relay-candidate" {
		t.Fatalf("expected node-relay-candidate to be elected due to public IP and stability, got %s", best.ID)
	}
}

func TestComputeCellularTopology_SmallCluster(t *testing.T) {
	engine := NewSupernodeRelayEngine()

	nodes := []MeshNode{
		{ID: "node-1", RegionID: "us-east-1", HasPublicIP: true, BandwidthMbps: 1000},
		{ID: "node-2", RegionID: "us-east-1", HasPublicIP: false, BandwidthMbps: 100},
		{ID: "node-3", RegionID: "us-east-1", HasPublicIP: false, BandwidthMbps: 100},
		{ID: "node-4", RegionID: "eu-central-1", HasPublicIP: true, BandwidthMbps: 1000},
		{ID: "node-5", RegionID: "eu-central-1", HasPublicIP: false, BandwidthMbps: 100},
	}

	plan, err := engine.ComputeCellularTopology(nodes)
	if err != nil {
		t.Fatalf("failed to compute topology: %v", err)
	}

	if plan.TotalNodes != 5 {
		t.Fatalf("expected 5 total nodes, got %d", plan.TotalNodes)
	}
	if plan.RegionCount != 2 {
		t.Fatalf("expected 2 regions, got %d", plan.RegionCount)
	}

	// In us-east-1 (3 nodes): 2 spokes -> supernode (node-1).
	// In eu-central-1 (2 nodes): 1 spoke -> supernode (node-4).
	// Inter-region trunk: node-1 <-> node-4 (1 trunk).
	// Total cellular tunnels = 2 + 1 + 1 = 4 tunnels.
	// Full mesh for 5 nodes = 5*4/2 = 10 tunnels.
	if plan.CellularTunnelCount != 4 {
		t.Fatalf("expected 4 cellular tunnels, got %d", plan.CellularTunnelCount)
	}
	if plan.FullMeshTunnelCount != 10 {
		t.Fatalf("expected 10 full mesh tunnels, got %d", plan.FullMeshTunnelCount)
	}

	// 60% reduction: (10 - 4) / 10 = 60.0%
	if plan.TunnelReductionPct != 60.0 {
		t.Fatalf("expected 60.0%% reduction, got %.2f%%", plan.TunnelReductionPct)
	}
}

func TestComputeCellularTopology_2000Nodes_99PctReduction(t *testing.T) {
	engine := NewSupernodeRelayEngine()

	const totalNodes = 2000
	const regionCount = 4
	regions := []string{"us-east-1", "us-west-2", "eu-central-1", "ap-southeast-1"}

	nodes := make([]MeshNode, totalNodes)
	for i := 0; i < totalNodes; i++ {
		region := regions[i%regionCount]
		hasPub := (i < regionCount) // First node of each region has public IP
		nodes[i] = MeshNode{
			ID:             fmt.Sprintf("node-%04d", i),
			RegionID:       region,
			HasPublicIP:    hasPub,
			BandwidthMbps:  float64(100 + (i % 900)),
			CPUHeadroomPct: float64(50 + (i % 50)),
			UptimeSeconds:  uint64(i * 3600),
		}
	}

	plan, err := engine.ComputeCellularTopology(nodes)
	if err != nil {
		t.Fatalf("failed to compute 2000 node topology: %v", err)
	}

	t.Logf("2,000 Nodes Plan: Cellular Tunnels = %d, Full Mesh = %d, Reduction = %.4f%%",
		plan.CellularTunnelCount, plan.FullMeshTunnelCount, plan.TunnelReductionPct)

	// Full Mesh for 2,000 nodes = 2,000 * 1,999 / 2 = 1,999,000
	if plan.FullMeshTunnelCount != 1999000 {
		t.Fatalf("expected 1,999,000 full mesh tunnels, got %d", plan.FullMeshTunnelCount)
	}

	// Cellular: 4 regions, 500 nodes each.
	// Spoke tunnels per region = 499. Across 4 regions = 499 * 4 = 1,996.
	// Inter-region trunks = 4 * 3 / 2 = 6.
	// Total cellular tunnels = 1,996 + 6 = 2,002.
	if plan.CellularTunnelCount != 2002 {
		t.Fatalf("expected 2,002 cellular tunnels, got %d", plan.CellularTunnelCount)
	}

	// Reduction requirement: > 99%
	if plan.TunnelReductionPct <= 99.0 {
		t.Fatalf("expected > 99.0%% tunnel reduction, got %.2f%%", plan.TunnelReductionPct)
	}
}
