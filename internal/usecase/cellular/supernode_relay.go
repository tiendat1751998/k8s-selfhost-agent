// Package cellular implements regional supernode relay election and hub-and-spoke mesh topology generation.
package cellular

import (
	"sort"
)

// TunnelType categorizes network overlay connections.
type TunnelType string

const (
	// TunnelTypeSpokeToHub connects an edge spoke node to its regional supernode.
	TunnelTypeSpokeToHub TunnelType = "SPOKE_TO_HUB"
	// TunnelTypeSupernodeTrunk connects two regional supernode relays across WAN.
	TunnelTypeSupernodeTrunk TunnelType = "SUPERNODE_TRUNK"
)

// MeshNode represents an edge host participating in the cellular overlay mesh.
type MeshNode struct {
	ID             string  `json:"id"`
	RegionID       string  `json:"region_id"`
	CellID         string  `json:"cell_id"`
	HasPublicIP    bool    `json:"has_public_ip"`
	PublicIP       string  `json:"public_ip,omitempty"`
	PrivateIP      string  `json:"private_ip"`
	BandwidthMbps  float64 `json:"bandwidth_mbps"`
	CPUHeadroomPct float64 `json:"cpu_headroom_pct"`
	UptimeSeconds  uint64  `json:"uptime_seconds"`
}

// TunnelConnection represents an encrypted overlay link between two nodes.
type TunnelConnection struct {
	SourceNodeID string     `json:"source_node_id"`
	TargetNodeID string     `json:"target_node_id"`
	Type         TunnelType `json:"type"`
	RegionID     string     `json:"region_id,omitempty"`
}

// RegionalCell tracks topology state and supernode assignment for a geographic/network cell.
type RegionalCell struct {
	RegionID         string   `json:"region_id"`
	SupernodeRelayID string   `json:"supernode_relay_id"`
	NodeCount        int      `json:"node_count"`
	SpokeNodeIDs     []string `json:"spoke_node_ids"`
}

// TopologyPlan contains computed hub-and-spoke routing and tunnel reduction statistics.
type TopologyPlan struct {
	TotalNodes          int                      `json:"total_nodes"`
	RegionCount         int                      `json:"region_count"`
	Regions             map[string]*RegionalCell `json:"regions"`
	SupernodeRelays     map[string]string        `json:"supernode_relays"`
	Tunnels             []TunnelConnection       `json:"tunnels"`
	CellularTunnelCount int                      `json:"cellular_tunnel_count"`
	FullMeshTunnelCount int                      `json:"full_mesh_tunnel_count"`
	TunnelReductionPct  float64                  `json:"tunnel_reduction_pct"`
}

// SupernodeRelayEngine calculates regional partitioning and supernode election.
type SupernodeRelayEngine struct{}

// NewSupernodeRelayEngine instantiates the cellular topology generator.
func NewSupernodeRelayEngine() *SupernodeRelayEngine {
	return &SupernodeRelayEngine{}
}

// CalculateSupernodeScore calculates deterministic election weight for a candidate node.
func CalculateSupernodeScore(node MeshNode) float64 {
	score := 0.0
	if node.HasPublicIP {
		score += 1000000.0
	}
	score += node.BandwidthMbps * 2.0
	score += node.CPUHeadroomPct * 10.0

	uptimeHours := float64(node.UptimeSeconds) / 3600.0
	if uptimeHours > 720.0 {
		uptimeHours = 720.0
	}
	score += uptimeHours * 1.5

	return score
}

// ElectSupernode deterministically elects the optimal relay host from a candidate pool.
func ElectSupernode(nodes []MeshNode) MeshNode {
	if len(nodes) == 0 {
		return MeshNode{}
	}

	best := nodes[0]
	bestScore := CalculateSupernodeScore(best)

	for i := 1; i < len(nodes); i++ {
		cand := nodes[i]
		candScore := CalculateSupernodeScore(cand)

		if candScore > bestScore {
			best = cand
			bestScore = candScore
		} else if candScore == bestScore && cand.ID < best.ID {
			// Deterministic tie breaker
			best = cand
			bestScore = candScore
		}
	}

	return best
}

// ComputeCellularTopology partitions nodes into regional cells and constructs the O(N) overlay plan.
func (e *SupernodeRelayEngine) ComputeCellularTopology(nodes []MeshNode) (*TopologyPlan, error) {
	if len(nodes) == 0 {
		return &TopologyPlan{
			Regions:         make(map[string]*RegionalCell),
			SupernodeRelays: make(map[string]string),
		}, nil
	}

	// 1. Group nodes by RegionID
	regionalNodes := make(map[string][]MeshNode)
	for _, node := range nodes {
		region := node.RegionID
		if region == "" {
			region = "default-region"
		}
		regionalNodes[region] = append(regionalNodes[region], node)
	}

	plan := &TopologyPlan{
		TotalNodes:      len(nodes),
		RegionCount:     len(regionalNodes),
		Regions:         make(map[string]*RegionalCell, len(regionalNodes)),
		SupernodeRelays: make(map[string]string, len(regionalNodes)),
		Tunnels:         make([]TunnelConnection, 0, len(nodes)*2),
	}

	// Sort region keys for deterministic order
	regionKeys := make([]string, 0, len(regionalNodes))
	for r := range regionalNodes {
		regionKeys = append(regionKeys, r)
	}
	sort.Strings(regionKeys)

	// 2. Elect supernode per region & create intra-region spoke-to-hub tunnels
	supernodes := make([]MeshNode, 0, len(regionKeys))
	for _, rKey := range regionKeys {
		rNodes := regionalNodes[rKey]
		supernode := ElectSupernode(rNodes)
		supernodes = append(supernodes, supernode)
		plan.SupernodeRelays[rKey] = supernode.ID

		cell := &RegionalCell{
			RegionID:         rKey,
			SupernodeRelayID: supernode.ID,
			NodeCount:        len(rNodes),
			SpokeNodeIDs:     make([]string, 0, len(rNodes)-1),
		}

		for _, node := range rNodes {
			if node.ID == supernode.ID {
				continue
			}
			cell.SpokeNodeIDs = append(cell.SpokeNodeIDs, node.ID)
			plan.Tunnels = append(plan.Tunnels, TunnelConnection{
				SourceNodeID: node.ID,
				TargetNodeID: supernode.ID,
				Type:         TunnelTypeSpokeToHub,
				RegionID:     rKey,
			})
		}
		plan.Regions[rKey] = cell
	}

	// 3. Connect regional supernodes via inter-region trunks (R*(R-1)/2 trunks)
	for i := 0; i < len(supernodes); i++ {
		for j := i + 1; j < len(supernodes); j++ {
			plan.Tunnels = append(plan.Tunnels, TunnelConnection{
				SourceNodeID: supernodes[i].ID,
				TargetNodeID: supernodes[j].ID,
				Type:         TunnelTypeSupernodeTrunk,
			})
		}
	}

	// 4. Compute reduction metrics vs O(N^2) Full Mesh: N*(N-1)/2
	plan.CellularTunnelCount = len(plan.Tunnels)
	n := len(nodes)
	if n > 1 {
		plan.FullMeshTunnelCount = (n * (n - 1)) / 2
		if plan.FullMeshTunnelCount > 0 {
			diff := float64(plan.FullMeshTunnelCount - plan.CellularTunnelCount)
			plan.TunnelReductionPct = (diff / float64(plan.FullMeshTunnelCount)) * 100.0
		}
	}

	return plan, nil
}
