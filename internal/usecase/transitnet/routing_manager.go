// Package transitnet implements the application usecase engine for Hybrid Cloud
// Transit Gateway routing integrity, MTU black hole detection, NLB cross-zone balancing,
// and CoreDNS negative caching sanitization.
package transitnet

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	domain "github.com/datdt/k8sselfhost/internal/domain/transitnet"
	"go.uber.org/zap"
)

// RouteAuditData models data returned from the BGP infrastructure provider.
type RouteAuditData struct {
	PrimaryPath          string
	ReturnPath           string
	RTTLatency           time.Duration
	PacketLossRate       float64
	DirectConnectHealthy bool
}

// BGPRouteAuditor abstracts BGP route discovery and AS-Path manipulation.
type BGPRouteAuditor interface {
	GetRoutePaths(ctx context.Context, targetEndpoint string) (RouteAuditData, error)
	ApplyBGPPrepending(ctx context.Context, link string, prepends int) error
}

// VPCProbeData models MTU discovery probe telemetry.
type VPCProbeData struct {
	SourceVPCID  string
	TargetVPCID  string
	SourceMTU    int
	TargetMTU    int
	ProbeSuccess bool
}

// VPCPeeringProber abstracts ICMP/TCP Path MTU probes across VPC peering connections.
type VPCPeeringProber interface {
	ProbePath(ctx context.Context, targetHost string, testBytes int) (VPCProbeData, error)
}

// NLBProvider abstracts AWS Elastic Load Balancing API calls for NLB instances.
type NLBProvider interface {
	GetNLBStatus(ctx context.Context, lbARN string) (crossZoneEnabled bool, zoneShares map[string]float64, err error)
	SetCrossZoneBalancing(ctx context.Context, lbARN string, enabled bool) error
}

// CoreDNSAdapter abstracts reading and updating the CoreDNS ConfigMap Corefile.
type CoreDNSAdapter interface {
	GetCorefile(ctx context.Context) (string, error)
	ApplyCorefile(ctx context.Context, content string) error
}

// AsymmetricRoutingAlert models P0 alerts generated upon detecting divergent return paths.
type AsymmetricRoutingAlert struct {
	AlertID        string    `json:"alert_id"`
	Severity       string    `json:"severity"` // "P0"
	TargetEndpoint string    `json:"target_endpoint"`
	PrimaryPath    string    `json:"primary_path"`
	ReturnPath     string    `json:"return_path"`
	Recommendation string    `json:"recommendation"`
	DetectedAt     time.Time `json:"detected_at"`
}

// RoutingManager implements domain.HybridCloudNetworkManager.
type RoutingManager struct {
	bgpAuditor     BGPRouteAuditor
	vpcProber      VPCPeeringProber
	nlbProvider    NLBProvider
	coreDNSAdapter CoreDNSAdapter
	logger         *zap.Logger

	alertMu      sync.RWMutex
	activeAlerts []AsymmetricRoutingAlert

	// Cache-line padded lock-free metric counters (MESI/MOESI aligned)
	tracker     domain.FastRoutingLatencyTracker
	lossCounter domain.FastPacketLossCounter
	nlbMonitor  domain.FastNLBDistributionMonitor
}

// Option configures RoutingManager.
type Option func(*RoutingManager)

// WithBGPAuditor injects a BGPRouteAuditor.
func WithBGPAuditor(a BGPRouteAuditor) Option {
	return func(m *RoutingManager) { m.bgpAuditor = a }
}

// WithVPCProber injects a VPCPeeringProber.
func WithVPCProber(p VPCPeeringProber) Option {
	return func(m *RoutingManager) { m.vpcProber = p }
}

// WithNLBProvider injects an NLBProvider.
func WithNLBProvider(n NLBProvider) Option {
	return func(m *RoutingManager) { m.nlbProvider = n }
}

// WithCoreDNSAdapter injects a CoreDNSAdapter.
func WithCoreDNSAdapter(c CoreDNSAdapter) Option {
	return func(m *RoutingManager) { m.coreDNSAdapter = c }
}

// WithLogger injects a logger.
func WithLogger(l *zap.Logger) Option {
	return func(m *RoutingManager) {
		if l != nil {
			m.logger = l
		}
	}
}

// NewRoutingManager creates a new instance of RoutingManager.
func NewRoutingManager(opts ...Option) *RoutingManager {
	mgr := &RoutingManager{
		logger: zap.NewNop(),
	}
	for _, opt := range opts {
		opt(mgr)
	}
	return mgr
}

// AuditRoutingSymmetry audits forward and return routing paths to identify asymmetric routing.
func (m *RoutingManager) AuditRoutingSymmetry(ctx context.Context, targetEndpoint string) (*domain.RoutingPathIntegrity, error) {
	if strings.TrimSpace(targetEndpoint) == "" {
		return nil, fmt.Errorf("target endpoint cannot be empty")
	}
	if m.bgpAuditor == nil {
		return nil, fmt.Errorf("BGP route auditor dependency not configured")
	}

	data, err := m.bgpAuditor.GetRoutePaths(ctx, targetEndpoint)
	if err != nil {
		return nil, fmt.Errorf("querying BGP route paths for %s: %w", targetEndpoint, err)
	}

	// Lock-free atomic metric recording with cache line isolation
	atomic.AddUint64(&m.tracker.ProbeCount, 1)
	atomic.AddUint64(&m.tracker.TotalLatencyUs, uint64(data.RTTLatency.Microseconds()))
	atomic.AddUint64(&m.lossCounter.TxPackets, 1)
	if data.PacketLossRate > 0 {
		atomic.AddUint64(&m.lossCounter.LostPackets, 1)
	} else {
		atomic.AddUint64(&m.lossCounter.RxPackets, 1)
	}

	isAsymmetric := data.PrimaryPath != data.ReturnPath
	integrity := &domain.RoutingPathIntegrity{
		PrimaryPath:    data.PrimaryPath,
		ReturnPath:     data.ReturnPath,
		IsAsymmetric:   isAsymmetric,
		RTTLatency:     data.RTTLatency,
		PacketLossRate: data.PacketLossRate,
	}

	if isAsymmetric {
		atomic.AddUint64(&m.tracker.AsymmetricDropCount, 1)
		alert := AsymmetricRoutingAlert{
			AlertID:        fmt.Sprintf("ALERT-ASYM-%d", time.Now().UnixNano()),
			Severity:       "P0",
			TargetEndpoint: targetEndpoint,
			PrimaryPath:    data.PrimaryPath,
			ReturnPath:     data.ReturnPath,
			Recommendation: fmt.Sprintf(
				"Apply BGP AS-Path Prepending (x3) on backup link %s to force symmetric transit via %s",
				data.ReturnPath, data.PrimaryPath,
			),
			DetectedAt: time.Now().UTC(),
		}

		m.alertMu.Lock()
		m.activeAlerts = append(m.activeAlerts, alert)
		m.alertMu.Unlock()

		m.logger.Warn("Asymmetric BGP routing detected",
			zap.String("target", targetEndpoint),
			zap.String("primary_path", data.PrimaryPath),
			zap.String("return_path", data.ReturnPath),
		)
		return integrity, domain.ErrBGPAsymmetricPath
	}

	return integrity, nil
}

// EnforceBGPPrependingGuardrail verifies primary DirectConnect link health before applying AS-Path prepends.
func (m *RoutingManager) EnforceBGPPrependingGuardrail(ctx context.Context, backupLink string, prepends int) error {
	if m.bgpAuditor == nil {
		return fmt.Errorf("BGP route auditor dependency not configured")
	}

	data, err := m.bgpAuditor.GetRoutePaths(ctx, "healthcheck-probe")
	if err != nil {
		return fmt.Errorf("verifying primary DirectConnect health: %w", err)
	}

	if !data.DirectConnectHealthy {
		m.logger.Error("Rollback guardrail activated: primary DirectConnect is unhealthy",
			zap.String("backup_link", backupLink),
		)
		return domain.ErrDirectConnectLinkUnhealthy
	}

	if err := m.bgpAuditor.ApplyBGPPrepending(ctx, backupLink, prepends); err != nil {
		return fmt.Errorf("applying AS-Path prepending on %s: %w", backupLink, err)
	}

	m.logger.Info("BGP AS-Path prepending applied successfully under operational DirectConnect link",
		zap.String("backup_link", backupLink),
		zap.Int("prepends", prepends),
	)
	return nil
}

// ValidateVPCPathMTU probes VPC peering MTU black holes and derives effective Path MTU.
func (m *RoutingManager) ValidateVPCPathMTU(ctx context.Context, targetHost string, testBytes int) (*domain.PeeringMTUValidation, error) {
	if strings.TrimSpace(targetHost) == "" {
		return nil, fmt.Errorf("target host cannot be empty")
	}
	if m.vpcProber == nil {
		return nil, fmt.Errorf("VPC peering prober dependency not configured")
	}

	probeData, err := m.vpcProber.ProbePath(ctx, targetHost, testBytes)
	if err != nil {
		return nil, fmt.Errorf("probing VPC path MTU for %s: %w", targetHost, err)
	}

	effectiveMTU := probeData.SourceMTU
	if probeData.TargetMTU < effectiveMTU {
		effectiveMTU = probeData.TargetMTU
	}
	if effectiveMTU <= 0 {
		effectiveMTU = 1500
	}

	// MSS = MTU - 40 bytes (20B IPv4 + 20B TCP header)
	configuredMSS := effectiveMTU - 40
	if configuredMSS < 536 {
		configuredMSS = 536 // RFC 791 absolute minimum
	}

	blackHoleDetected := !probeData.ProbeSuccess || (testBytes > effectiveMTU)

	result := &domain.PeeringMTUValidation{
		SourceVPCID:       probeData.SourceVPCID,
		TargetVPCID:       probeData.TargetVPCID,
		ConfiguredMSS:     configuredMSS,
		EffectivePathMTU:  effectiveMTU,
		BlackHoleDetected: blackHoleDetected,
	}

	if blackHoleDetected {
		atomic.AddUint64(&m.lossCounter.OutboundDrops, 1)
		m.logger.Warn("VPC peering MTU black hole detected",
			zap.String("target_host", targetHost),
			zap.Int("effective_mtu", effectiveMTU),
			zap.Int("probe_bytes", testBytes),
		)
		return result, domain.ErrVPCPathMTUBlackHole
	}

	return result, nil
}

// GenerateMSSClampingRules creates iptables and nftables TCP MSS clamping rules.
func (m *RoutingManager) GenerateMSSClampingRules(effectivePMTU int) []string {
	targetMSS := effectivePMTU - 40
	if targetMSS < 536 {
		targetMSS = 536
	}
	return []string{
		fmt.Sprintf("iptables -t mangle -A FORWARD -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --set-mss %d", targetMSS),
		fmt.Sprintf("nft add rule inet filter forward tcp flags syn tcp option maxseg size set %d", targetMSS),
	}
}

// AuditNLBCrossZoneBalance audits NLB AZ traffic shares and enables cross-zone balancing if imbalanced.
func (m *RoutingManager) AuditNLBCrossZoneBalance(ctx context.Context, lbARN string) (*domain.NLBZoneDistribution, error) {
	if strings.TrimSpace(lbARN) == "" {
		return nil, fmt.Errorf("load balancer ARN cannot be empty")
	}
	if m.nlbProvider == nil {
		return nil, fmt.Errorf("NLB provider dependency not configured")
	}

	crossZoneEnabled, shares, err := m.nlbProvider.GetNLBStatus(ctx, lbARN)
	if err != nil {
		return nil, fmt.Errorf("fetching NLB status for %s: %w", lbARN, err)
	}

	imbalanceDetected := false
	for _, share := range shares {
		if share > 0.60 && !crossZoneEnabled {
			imbalanceDetected = true
			break
		}
	}

	atomic.AddUint64(&m.nlbMonitor.TotalRequests, 100)
	if imbalanceDetected {
		atomic.AddUint64(&m.nlbMonitor.ImbalancedDetections, 1)
		m.logger.Warn("NLB single AZ overload detected (>60% share); auto-enabling cross-zone load balancing",
			zap.String("lb_arn", lbARN),
		)
		// Automatically enable cross-zone load balancing
		if err := m.nlbProvider.SetCrossZoneBalancing(ctx, lbARN, true); err != nil {
			return nil, fmt.Errorf("auto-enabling cross-zone load balancing on %s: %w", lbARN, err)
		}
		crossZoneEnabled = true
	}

	dist := &domain.NLBZoneDistribution{
		LoadBalancerARN:   lbARN,
		CrossZoneEnabled:  crossZoneEnabled,
		ZoneRequestShares: shares,
		ImbalanceDetected: imbalanceDetected,
	}

	if imbalanceDetected {
		return dist, domain.ErrNLBAZImbalance
	}

	return dist, nil
}

// SanitizeCoreDNSNegativeCache audits and limits negative NXDOMAIN caching TTL to prevent blacklisting.
func (m *RoutingManager) SanitizeCoreDNSNegativeCache(ctx context.Context, domainZone string, maxNegativeTTLSeconds int) error {
	if strings.TrimSpace(domainZone) == "" {
		return fmt.Errorf("domain zone cannot be empty")
	}
	if maxNegativeTTLSeconds <= 0 {
		maxNegativeTTLSeconds = 5
	}
	if m.coreDNSAdapter == nil {
		return fmt.Errorf("CoreDNS adapter dependency not configured")
	}

	corefile, err := m.coreDNSAdapter.GetCorefile(ctx)
	if err != nil {
		return fmt.Errorf("reading Corefile: %w", err)
	}

	denialDirective := fmt.Sprintf("cache denial %d %s", maxNegativeTTLSeconds, domainZone)
	if strings.Contains(corefile, denialDirective) {
		return nil
	}

	var sanitized string
	lines := strings.Split(corefile, "\n")
	foundCache := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "cache") && !strings.HasPrefix(trimmed, "cache denial") {
			foundCache = true
			sanitized += line + "\n"
			indent := line[:len(line)-len(trimmed)]
			sanitized += fmt.Sprintf("%s%s\n", indent, denialDirective)
			continue
		}
		sanitized += line + "\n"
	}

	if !foundCache {
		// Insert before final closing bracket
		lastClose := strings.LastIndex(corefile, "}")
		if lastClose != -1 {
			sanitized = corefile[:lastClose] + fmt.Sprintf("    %s\n", denialDirective) + corefile[lastClose:]
		} else {
			sanitized = corefile + fmt.Sprintf("\n%s\n", denialDirective)
		}
	}

	sanitized = strings.TrimRight(sanitized, "\n") + "\n"
	if err := m.coreDNSAdapter.ApplyCorefile(ctx, sanitized); err != nil {
		return fmt.Errorf("applying sanitized Corefile: %w", err)
	}

	m.logger.Info("Sanitized CoreDNS negative caching TTL to 5s",
		zap.String("zone", domainZone),
		zap.Int("max_denial_ttl", maxNegativeTTLSeconds),
	)
	return nil
}

// GetActiveAlerts returns currently active routing alerts.
func (m *RoutingManager) GetActiveAlerts() []AsymmetricRoutingAlert {
	m.alertMu.RLock()
	defer m.alertMu.RUnlock()
	res := make([]AsymmetricRoutingAlert, len(m.activeAlerts))
	copy(res, m.activeAlerts)
	return res
}

// GetFastLatencyTracker returns current state of the latency tracker.
func (m *RoutingManager) GetFastLatencyTracker() domain.FastRoutingLatencyTracker {
	return domain.FastRoutingLatencyTracker{
		ProbeCount:          atomic.LoadUint64(&m.tracker.ProbeCount),
		TotalLatencyUs:      atomic.LoadUint64(&m.tracker.TotalLatencyUs),
		MinLatencyUs:        atomic.LoadUint64(&m.tracker.MinLatencyUs),
		MaxLatencyUs:        atomic.LoadUint64(&m.tracker.MaxLatencyUs),
		AsymmetricDropCount: atomic.LoadUint64(&m.tracker.AsymmetricDropCount),
	}
}
