// Package transitnet defines domain models, telemetry monitors, and contract interfaces
// for Hybrid Cloud Transit Gateway, Asymmetric Routing Shield, VPC Peering MTU validation,
// NLB cross-zone balancing, and CoreDNS negative caching sanitization.
//
// =========================================================================================
// ENGINEERING NOTE: PHYSICAL HARDWARE BOTTLENECK & REAL-WORLD FAILURE ANALYSIS
// =========================================================================================
// 1. Cross-Cloud WAN Latency & Asymmetric Path Delays:
//    In enterprise hybrid cloud topologies (AWS DirectConnect / Azure ExpressRoute combined
//    with backup IPsec VPN tunnels), physical fiber propagation and routing hop counts vary
//    drastically. Primary DirectConnect latency typically measures 3-5ms RTT, whereas backup
//    IPsec tunnels across public internet transit measure 40-80ms RTT. When asymmetric routing
//    occurs (outbound via DirectConnect, return via IPsec), TCP connections experience severe
//    packet reordering, spurious retransmissions, duplicate ACKs, and catastrophic congestion
//    window collapse (CUBIC/BBR ssthresh reduction).
//
// 2. Stateful Firewall Session Table Drops (The Asymmetric Routing Killer):
//    Enterprise perimeter middleboxes (Palo Alto, Fortinet, AWS Network Firewall) maintain
//    stateful connection tracking (conntrack) tables. TCP SYN packets egressing via DirectConnect
//    initialize state on the primary firewall. If the remote endpoint returns SYN/ACK over the
//    backup IPsec link, the secondary firewall has no state for the 5-tuple and drops the return
//    packet with TCP RST or silent drop. The connection hangs indefinitely at SYN_SENT.
//
// 3. Jumbo Frame MTU Drop & VPC Peering MTU Black Holes:
//    AWS VPC intra-region networking supports Jumbo Frames (9001 bytes). Inter-region VPC
//    peering, DirectConnect, and Transit Gateway attachments enforce standard Ethernet MTU
//    (1500 bytes) or VPN MTU (1380-1420 bytes due to IPsec ESP / WireGuard encapsulation).
//    When middleboxes silently drop packets exceeding 1500 bytes with DF (Don't Fragment) bit
//    set without generating ICMP Type 3 Code 4 (Fragmentation Needed), Path MTU Discovery
//    (RFC 1191) fails. Short TCP handshakes (< 1400 bytes) succeed, while subsequent TLS
//    ClientHello / ServerHello or database bulk inserts (> 1500 bytes) freeze in an MTU black hole.
//
// 4. NLB Single AZ Overload & Zone Capacity Imbalance:
//    AWS Network Load Balancers (NLBs) default to cross-zone load balancing DISABLED. Each NLB
//    node routes traffic solely to registered targets in its own Availability Zone. If Route 53
//    or client DNS resolvers cache a single NLB IP (or ingress traffic concentrates in us-east-1a),
//    the backend pods in AZ-a suffer CPU starvation, connection exhaustion, and OOM kills, while
//    targets in AZ-b and AZ-c remain 60% idle. Automatic cross-zone balancing enforcement is
//    mandatory when any single AZ request share exceeds 60%.
//
// 5. CoreDNS Negative Caching (NXDOMAIN 300s) & UDP Socket Buffer Starvation:
//    During transient BGP reconvergence or VPN tunnel flap (1-3 seconds), internal database
//    hostnames (e.g., db.aurora.internal) fail resolution temporarily. Standard CoreDNS
//    configurations cache negative responses (NXDOMAIN) for up to 300 seconds (5 minutes).
//    Consequently, database queries remain hard-failed long after the physical network link has
//    recovered. Restricting denial cache TTL to 5 seconds (`cache denial 5 <zone>`) and sizing
//    Linux UDP socket receive buffers (`net.core.rmem_max >= 16MB`) eliminates both the 5-minute
//    blacklisting window and packet drop under high DNS query burst rates.
// =========================================================================================
package transitnet

import (
	"context"
	"errors"
	"time"
)

// RFC 7807 problem details error codes.
const (
	CodeBGPAsymmetricPath  = "BGP_ASYMMETRIC_PATH"
	CodeVPCPathMTUBlackHole = "VPC_PATH_MTU_BLACK_HOLE"
	CodeNLBAZImbalance     = "NLB_AZ_IMBALANCE"
	CodeDirectConnectDown  = "DIRECT_CONNECT_DOWN"
	CodeInvalidEndpoint    = "INVALID_ENDPOINT"
)

// Standard domain errors.
var (
	ErrBGPAsymmetricPath = errors.New(
		"asymmetric BGP routing detected: outbound and inbound paths traverse divergent transit gateways",
	)
	ErrVPCPathMTUBlackHole = errors.New(
		"VPC peering MTU black hole detected: frame size exceeds effective path MTU without fragmentation notification",
	)
	ErrNLBAZImbalance = errors.New(
		"NLB single AZ request share exceeds 60% with cross-zone load balancing disabled",
	)
	ErrDirectConnectLinkUnhealthy = errors.New(
		"primary DirectConnect link unhealthy: cannot alter BGP AS-Path prepending without active primary path",
	)
)

// RoutingPathIntegrity models the symmetry, latency, and packet loss state of hybrid cloud routing.
type RoutingPathIntegrity struct {
	PrimaryPath    string        `json:"primary_path"`
	ReturnPath     string        `json:"return_path"`
	IsAsymmetric   bool          `json:"is_asymmetric"`
	RTTLatency     time.Duration `json:"rtt_latency"`
	PacketLossRate float64       `json:"packet_loss_rate"`
}

// PeeringMTUValidation models VPC peering MTU probing and TCP MSS clamping requirements.
type PeeringMTUValidation struct {
	SourceVPCID       string `json:"source_vpc_id"`
	TargetVPCID       string `json:"target_vpc_id"`
	ConfiguredMSS     int    `json:"configured_mss"`
	EffectivePathMTU  int    `json:"effective_path_mtu"`
	BlackHoleDetected bool   `json:"black_hole_detected"`
}

// NLBZoneDistribution models the traffic share per Availability Zone for AWS Network Load Balancers.
type NLBZoneDistribution struct {
	LoadBalancerARN   string             `json:"load_balancer_arn"`
	CrossZoneEnabled  bool               `json:"cross_zone_enabled"`
	ZoneRequestShares map[string]float64 `json:"zone_request_shares"`
	ImbalanceDetected bool               `json:"imbalance_detected"`
}

// FastRoutingLatencyTracker tracks high-frequency probe latency metrics lock-free.
// 64-byte cache-line padding prevents false sharing across CPU cores under high-rate polling (MESI/MOESI).
// 5 uint64 fields (40 bytes) + 24 bytes padding = exactly 64 bytes.
type FastRoutingLatencyTracker struct {
	ProbeCount          uint64
	TotalLatencyUs      uint64
	MinLatencyUs        uint64
	MaxLatencyUs        uint64
	AsymmetricDropCount uint64
	_                   [24]byte // Cache line padding (64 bytes total)
}

// FastPacketLossCounter tracks packet counts and loss rates across multi-core workers.
// 5 uint64 fields (40 bytes) + 24 bytes padding = exactly 64 bytes.
type FastPacketLossCounter struct {
	TxPackets        uint64
	RxPackets        uint64
	LostPackets      uint64
	CorruptedPackets uint64
	OutboundDrops    uint64
	_                [24]byte // Cache line padding (64 bytes total)
}

// FastNLBDistributionMonitor tracks NLB request distribution across AZs lock-free.
// 5 uint64 fields (40 bytes) + 24 bytes padding = exactly 64 bytes.
type FastNLBDistributionMonitor struct {
	TotalRequests        uint64
	ZoneARequests        uint64
	ZoneBRequests        uint64
	ZoneCRequests        uint64
	ImbalancedDetections uint64
	_                    [24]byte // Cache line padding (64 bytes total)
}

// HybridCloudNetworkManager defines the domain port for hybrid routing audit,
// MTU validation, NLB cross-zone balancing, and CoreDNS negative caching sanitization.
type HybridCloudNetworkManager interface {
	// AuditRoutingSymmetry audits forward and return routing paths to identify asymmetric routing.
	AuditRoutingSymmetry(ctx context.Context, targetEndpoint string) (*RoutingPathIntegrity, error)

	// ValidateVPCPathMTU probes VPC peering MTU black holes and derives effective Path MTU.
	ValidateVPCPathMTU(ctx context.Context, targetHost string, testBytes int) (*PeeringMTUValidation, error)

	// AuditNLBCrossZoneBalance audits NLB AZ traffic shares and enables cross-zone balancing if imbalanced.
	AuditNLBCrossZoneBalance(ctx context.Context, lbARN string) (*NLBZoneDistribution, error)

	// SanitizeCoreDNSNegativeCache audits and limits negative NXDOMAIN caching TTL to prevent blacklisting.
	SanitizeCoreDNSNegativeCache(ctx context.Context, domainZone string, maxNegativeTTLSeconds int) error
}
