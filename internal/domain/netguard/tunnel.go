// Package netguard defines domain models, watchdog interfaces, and metric trackers
// for hybrid WAN, site-to-site VPNs, MTU black hole detection, and BGP routing.
//
// =========================================================================================
// ENGINEERING NOTE: PHYSICAL HARDWARE BOTTLENECK & REAL-WORLD FAILURE ANALYSIS
// =========================================================================================
// 1. Cross-Datacenter WAN Latency & Delay-Bandwidth Product (BDP):
//    In multi-cloud and on-premise hybrid WAN topologies, physical fiber propagation delays
//    impose an unyielding floor (~5us/km). High BDP paths (e.g., 10Gbps @ 40ms RTT = 50MB BDP)
//    suffer catastrophic throughput collapse if MTU mismatch causes packet loss or TCP window
//    stalls. When intermediate middleboxes fragment or drop frames, TCP drops into Reno/CUBIC
//    congestion avoidance, collapsing line rate to a fraction of available capacity.
//
// 2. Encapsulation MTU Overhead:
//    Encapsulation protocols inflate outer frame headers:
//      - WireGuard: 80 bytes (20B outer IPv4/40B IPv6 + 8B UDP + 16B WireGuard + 16B Poly1305 MAC)
//      - IPsec ESP: 56-72 bytes (20B outer IP + 8B ESP header + IV + padding + 16B ICV)
//      - VXLAN / Geneve: 50-54 bytes (20B outer IP + 8B UDP + 8B VXLAN + 14B inner Ethernet)
//    Standard Ethernet MTU is 1500 bytes. When payload packets are sized at 1500 bytes without
//    MSS clamping, encapsulated packets reach 1550-1580 bytes. If the WAN underlay does not
//    support jumbo frames (9000B), fragmentation or packet drops occur.
//
// 3. NIC Checksum Offload Quirks with Nested Tunnels:
//    Commodity NICs (e.g., Intel ixgbe, Realtek r8169) only calculate L4 checksums on the
//    outermost IP/UDP header. Inner packet checksums inside nested tunnels (e.g., WireGuard
//    over VXLAN) fail hardware offloading, resulting in corrupted frames silently dropped by
//    the remote kernel stack or severe CPU softirq starvation on ksoftirqd cores.
//
// 4. Router ICMP Rate Limiting & MTU Black Holes:
//    When a DF (Don't Fragment) packet exceeds intermediate link MTU, RFC 1191 Path MTU
//    Discovery (PMTUD) requires routers to return ICMP Type 3, Code 4 ("Fragmentation Needed").
//    Modern carrier edge routers aggressively rate-limit or silently drop ICMP Type 3 Code 4.
//    Consequently, small packets (curl headers, TCP SYN/ACK < 1400B) succeed, whereas large
//    data packets (TLS ServerHello, API JSON payloads > 1420B) time out indefinitely.
//
// 5. Silent DERP Relay Fallbacks:
//    Tailscale/WireGuard meshes employ STUN hole punching for p2p transport. If symmetric NAT
//    or stateful enterprise firewalls close UDP mappings, tunnels silently fall back to DERP
//    relay servers. Latency immediately degrades from 5ms (direct p2p) to 180ms+ (relayed),
//    introducing packet loss and massive jitter without raising connection-down alerts.
//
// 6. Linux rp_filter=1 Strict Drops on Multi-Homed Hybrid Nodes:
//    Under BGP multipath or asymmetric hybrid WAN routing, ingress packets arrive on one WAN
//    interface while the return route is preferred via another (or VPN tunnel). With strict
//    rp_filter=1, the Linux kernel drops all return traffic at the FIB lookup stage,
//    manifesting as mysterious one-way connection failures. Switching to loose mode (rp_filter=2)
//    with explicit RFC 1918 ingress anti-spoofing filters is strictly required.
// =========================================================================================
package netguard

import (
	"context"
	"errors"
	"time"
)

// Standard domain error codes (RFC 7807 problem details).
const (
	CodeTunnelMTUBlackHole     = "TUNNEL_MTU_BLACK_HOLE"
	CodeDERPRelayDegradation   = "DERP_RELAY_DEGRADATION"
	CodeAsymmetricRoutingDrop  = "ASYMMETRIC_ROUTING_DROP"
	CodeMSSClampingFailed      = "MSS_CLAMPING_FAILED"
	CodeInvalidTunnelConfig    = "INVALID_TUNNEL_CONFIG"
)

// Standard domain errors.
var (
	ErrTunnelMTUBlackHole = errors.New(
		"tunnel MTU black hole detected: large packets dropped without ICMP type 3 code 4",
	)
	ErrDERPRelayDegradation = errors.New(
		"P1 ALERT: tunnel degraded to DERP relay fallback with extreme latency penalty (>100ms)",
	)
	ErrAsymmetricRoutingDrop = errors.New(
		"asymmetric routing packet drop detected: strict rp_filter dropped ingress return path",
	)
	ErrInvalidInterface = errors.New("invalid or empty network interface name")
	ErrInvalidPMTU      = errors.New("invalid PMTU value: must be between 576 and 9000 bytes")
)

// TunnelTransportMode specifies whether the tunnel is direct peer-to-peer or relay fallback.
type TunnelTransportMode string

const (
	// TransportP2PDirect indicates low-latency direct UDP peer-to-peer transport.
	TransportP2PDirect TunnelTransportMode = "p2p_direct"
	// TransportDERPRelay indicates degraded relayed transport via DERP relay node.
	TransportDERPRelay TunnelTransportMode = "derp_relay"
)

// HybridTunnelStatus captures the operational health and routing configuration of a WAN tunnel.
type HybridTunnelStatus struct {
	InterfaceName   string              `json:"interface_name"`
	TunnelType      string              `json:"tunnel_type"` // e.g. "wireguard", "ipsec", "vxlan", "tailscale"
	TransportMode   TunnelTransportMode `json:"transport_mode"`
	ConfiguredMTU   int                 `json:"configured_mtu"`
	DetectedPMTU    int                 `json:"detected_pmtu"`
	MSSClampingOn   bool                `json:"mss_clamping_on"`
	RPFilterMode    int                 `json:"rp_filter_mode"` // 0: off, 1: strict, 2: loose
	RTTLatencyMs    float64             `json:"rtt_latency_ms"`
	PacketLossPct   float64             `json:"packet_loss_pct"`
	LastTestedAt    time.Time           `json:"last_tested_at"`
}

// PMTUSweepState tracks high-frequency binary search sweep iterations.
// 64-byte cache-line padding prevents false sharing across CPU cores under high-rate probe sweeps (MESI/MOESI).
// 6 uint64 fields (48 bytes) + 16 bytes padding = exactly 64 bytes.
type PMTUSweepState struct {
	LowBound     int64
	HighBound    int64
	CurrentProbe int64
	OptimalPMTU  int64
	ProbeCount   int64
	Iterations   int64
	_            [16]byte // Cache line padding (64 bytes total)
}

// FastLatencyTracker tracks lock-free / high-frequency probe latency metrics.
// 64-byte cache-line padding prevents false sharing across CPU cores.
// 5 uint64 fields (40 bytes) + 24 bytes padding = exactly 64 bytes.
type FastLatencyTracker struct {
	ProbeCount     uint64
	TotalLatencyUs uint64
	MinLatencyUs   uint64
	MaxLatencyUs   uint64
	PacketLoss     uint64
	_              [24]byte // Cache line padding (64 bytes total)
}

// FastTunnelMetrics tracks per-tunnel high-throughput metrics on multi-core workers.
// 5 uint64 fields (40 bytes) + 24 bytes padding = exactly 64 bytes.
type FastTunnelMetrics struct {
	TxBytes   uint64
	RxBytes   uint64
	TxPackets uint64
	RxPackets uint64
	Drops     uint64
	_         [24]byte // Cache line padding (64 bytes total)
}

// TunnelWatchdog defines the primary port for tunnel PMTU sweeping, rp_filter auditing,
// MSS clamping enforcement, and transport health monitoring.
type TunnelWatchdog interface {
	// ProbePathMTU discovers the true path MTU using binary search DF sweeps.
	ProbePathMTU(ctx context.Context, targetIP string) (int, error)
	// AuditRPFilter checks net.ipv4.conf.*.rp_filter sysctl modes on node interfaces.
	AuditRPFilter(ctx context.Context) (map[string]int, error)
	// EnforceMSSClamping generates and applies iptables/nftables TCP MSS clamping rules.
	EnforceMSSClamping(ctx context.Context, iface string, pmtu int) error
	// CheckTransportHealth returns health status and triggers P1 alerts on DERP fallback.
	CheckTransportHealth(ctx context.Context) ([]HybridTunnelStatus, error)
}
