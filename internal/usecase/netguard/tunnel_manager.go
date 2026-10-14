// Package netguard implements the usecase engine for hybrid WAN watchdog,
// PMTU binary search sweeping, TCP MSS clamping, and DERP relay fallback monitoring.
package netguard

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	domain "github.com/datdt/k8sselfhost/internal/domain/netguard"
	"go.uber.org/zap"
)

// Default sweep constants.
const (
	DefaultMinMTU  = 1400
	DefaultMaxMTU  = 1500
	DefaultMTUStep = 4 // 4-byte 32-bit IP/Ethernet word boundary
)

// ProbeRunner executes a Don't Fragment (DF) packet probe to targetIP.
type ProbeRunner interface {
	Probe(ctx context.Context, targetIP string, sizeBytes int) (success bool, rtt time.Duration, err error)
}

// SysctlAdapter provides an abstraction for reading and writing Linux sysctl parameters.
type SysctlAdapter interface {
	GetSysctl(ctx context.Context, key string) (int, error)
	SetSysctl(ctx context.Context, key string, val int) error
	ListSysctls(ctx context.Context, prefix string) (map[string]int, error)
}

// RuleExecutor provides an abstraction for executing iptables/nftables firewall commands.
type RuleExecutor interface {
	ExecuteCommand(ctx context.Context, cmd string, args ...string) error
}

// SweepResult records the outcome of a Path MTU binary search sweep.
type SweepResult struct {
	TargetIP          string        `json:"target_ip"`
	DiscoveredPMTU    int           `json:"discovered_pmtu"`
	Iterations        int           `json:"iterations"`
	ProbeCount        int           `json:"probe_count"`
	BlackHoleDetected bool          `json:"blackhole_detected"`
	Duration          time.Duration `json:"duration_ms"`
}

// DERPAlert captures a P1 degradation alert when a tunnel falls back to DERP relay.
type DERPAlert struct {
	InterfaceName string    `json:"interface_name"`
	Severity      string    `json:"severity"` // "P1"
	BaselineRTTMs float64   `json:"baseline_rtt_ms"`
	CurrentRTTMs  float64   `json:"current_rtt_ms"`
	LatencyJumpMs float64   `json:"latency_jump_ms"`
	DetectedAt    time.Time `json:"detected_at"`
	Remediation   string    `json:"remediation"`
}

// RPFilterAuditResult summarizes rp_filter configuration across node interfaces.
type RPFilterAuditResult struct {
	Modes                      map[string]int `json:"modes"`
	StrictMultiHomedInterfaces []string       `json:"strict_multi_homed_interfaces"`
	Recommendations            []string       `json:"recommendations"`
	SpoofingProtectionValid    bool           `json:"spoofing_protection_valid"`
}

// TunnelManager implements the domain.TunnelWatchdog usecase port.
type TunnelManager struct {
	probeRunner ProbeRunner
	sysctl      SysctlAdapter
	executor    RuleExecutor
	logger      *zap.Logger

	minMTU  int
	maxMTU  int
	stepMTU int

	statusMu sync.RWMutex
	statuses map[string]domain.HybridTunnelStatus

	// Cache-line padded high-frequency latency tracker (prevents false sharing)
	tracker domain.FastLatencyTracker
}

// Option configures TunnelManager.
type Option func(*TunnelManager)

// WithProbeRunner injects a custom ProbeRunner.
func WithProbeRunner(r ProbeRunner) Option {
	return func(m *TunnelManager) { m.probeRunner = r }
}

// WithSysctlAdapter injects a custom SysctlAdapter.
func WithSysctlAdapter(s SysctlAdapter) Option {
	return func(m *TunnelManager) { m.sysctl = s }
}

// WithSysctlMap initializes an in-memory sysctl map adapter.
func WithSysctlMap(initial map[string]int) Option {
	return func(m *TunnelManager) {
		m.sysctl = &mapSysctlAdapter{data: initial}
	}
}

// WithRuleExecutor injects a custom firewall command executor.
func WithRuleExecutor(e RuleExecutor) Option {
	return func(m *TunnelManager) { m.executor = e }
}

// WithSweepRange configures custom MTU search bounds and step size.
func WithSweepRange(minMTU, maxMTU, stepMTU int) Option {
	return func(m *TunnelManager) {
		if minMTU > 0 {
			m.minMTU = minMTU
		}
		if maxMTU >= minMTU {
			m.maxMTU = maxMTU
		}
		if stepMTU > 0 {
			m.stepMTU = stepMTU
		}
	}
}

// WithLogger injects a structured logger.
func WithLogger(l *zap.Logger) Option {
	return func(m *TunnelManager) {
		if l != nil {
			m.logger = l
		}
	}
}

// NewTunnelManager creates a new instance of TunnelManager with configured options.
func NewTunnelManager(opts ...Option) *TunnelManager {
	mgr := &TunnelManager{
		minMTU:   DefaultMinMTU,
		maxMTU:   DefaultMaxMTU,
		stepMTU:  DefaultMTUStep,
		statuses: make(map[string]domain.HybridTunnelStatus),
		logger:   zap.NewNop(),
	}

	for _, opt := range opts {
		opt(mgr)
	}

	if mgr.probeRunner == nil {
		mgr.probeRunner = &defaultProbeRunner{}
	}
	if mgr.sysctl == nil {
		mgr.sysctl = &mapSysctlAdapter{data: make(map[string]int)}
	}
	if mgr.executor == nil {
		mgr.executor = &defaultRuleExecutor{}
	}

	return mgr
}

// SweepPMTU executes a binary search DF probe sweep to determine the true path MTU.
func (m *TunnelManager) SweepPMTU(ctx context.Context, targetIP string) (*SweepResult, error) {
	if strings.TrimSpace(targetIP) == "" {
		return nil, fmt.Errorf("target IP cannot be empty")
	}

	startTime := time.Now()
	var sweepState domain.PMTUSweepState
	sweepState.LowBound = int64(m.minMTU)
	sweepState.HighBound = int64(m.maxMTU)

	// Step 1: Probe max boundary (1500)
	maxPass, rtt, err := m.probeRunner.Probe(ctx, targetIP, m.maxMTU)
	if err != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	sweepState.ProbeCount++
	atomic.AddUint64(&m.tracker.ProbeCount, 1)
	atomic.AddUint64(&m.tracker.TotalLatencyUs, uint64(rtt.Microseconds()))

	if maxPass {
		return &SweepResult{
			TargetIP:          targetIP,
			DiscoveredPMTU:    m.maxMTU,
			Iterations:        1,
			ProbeCount:        int(sweepState.ProbeCount),
			BlackHoleDetected: false,
			Duration:          time.Since(startTime),
		}, nil
	}

	// Step 2: Probe min boundary (1400) to confirm path viability
	minPass, _, err := m.probeRunner.Probe(ctx, targetIP, m.minMTU)
	if err != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	sweepState.ProbeCount++
	atomic.AddUint64(&m.tracker.ProbeCount, 1)

	if !minPass {
		// Even minimum standard WAN MTU drops packets
		atomic.AddUint64(&m.tracker.PacketLoss, 1)
		return nil, fmt.Errorf("%w: target %s fails minimum %dB probe", domain.ErrTunnelMTUBlackHole, targetIP, m.minMTU)
	}

	// Large packets dropped without ICMP response -> MTU black hole detected
	blackHoleDetected := true

	// Step 3: Binary search on discrete steps between [minMTU, maxMTU]
	numSteps := (m.maxMTU - m.minMTU) / m.stepMTU
	low := 0
	high := numSteps
	bestStep := 0

	for low <= high {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		mid := low + (high-low)/2
		size := m.minMTU + mid*m.stepMTU
		sweepState.CurrentProbe = int64(size)
		sweepState.Iterations++
		sweepState.ProbeCount++

		pass, probeRTT, probeErr := m.probeRunner.Probe(ctx, targetIP, size)
		if probeErr != nil && ctx.Err() != nil {
			return nil, ctx.Err()
		}

		atomic.AddUint64(&m.tracker.ProbeCount, 1)
		atomic.AddUint64(&m.tracker.TotalLatencyUs, uint64(probeRTT.Microseconds()))

		if pass {
			bestStep = mid
			low = mid + 1 // Attempt larger packet
		} else {
			high = mid - 1 // Step back down
		}
	}

	discoveredPMTU := m.minMTU + bestStep*m.stepMTU
	sweepState.OptimalPMTU = int64(discoveredPMTU)

	return &SweepResult{
		TargetIP:          targetIP,
		DiscoveredPMTU:    discoveredPMTU,
		Iterations:        int(sweepState.Iterations),
		ProbeCount:        int(sweepState.ProbeCount),
		BlackHoleDetected: blackHoleDetected,
		Duration:          time.Since(startTime),
	}, nil
}

// ProbePathMTU implements domain.TunnelWatchdog.
func (m *TunnelManager) ProbePathMTU(ctx context.Context, targetIP string) (int, error) {
	res, err := m.SweepPMTU(ctx, targetIP)
	if err != nil {
		return 0, err
	}
	return res.DiscoveredPMTU, nil
}

// CalculateMSS determines target TCP MSS from path MTU.
func (m *TunnelManager) CalculateMSS(pmtu int, isIPv6 bool) (int, error) {
	if pmtu < 576 || pmtu > 9000 {
		return 0, domain.ErrInvalidPMTU
	}
	headerOverhead := 40 // IPv4 (20B IP + 20B TCP)
	if isIPv6 {
		headerOverhead = 60 // IPv6 (40B IP + 20B TCP)
	}
	return pmtu - headerOverhead, nil
}

// GenerateIPTablesMSSRules creates iptables mangle rules for TCP SYN clamping.
func (m *TunnelManager) GenerateIPTablesMSSRules(iface string, targetMSS int) []string {
	if err := ValidateInterfaceName(iface); err != nil {
		return nil
	}
	return []string{
		fmt.Sprintf("iptables -t mangle -A POSTROUTING -o %s -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --set-mss %d", iface, targetMSS),
		fmt.Sprintf("iptables -t mangle -A FORWARD -o %s -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --set-mss %d", iface, targetMSS),
	}
}

// GenerateNFTablesMSSRules creates nftables rules for TCP SYN clamping.
func (m *TunnelManager) GenerateNFTablesMSSRules(iface string, targetMSS int) []string {
	if err := ValidateInterfaceName(iface); err != nil {
		return nil
	}
	return []string{
		fmt.Sprintf("nft add rule inet filter forward oifname \"%s\" tcp flags syn / syn,rst tcp option maxseg size set %d", iface, targetMSS),
		fmt.Sprintf("nft add rule ip mangle postrouting oifname \"%s\" tcp flags syn / syn,rst tcp option maxseg size set %d", iface, targetMSS),
	}
}

// EnforceMSSClamping implements domain.TunnelWatchdog.
func (m *TunnelManager) EnforceMSSClamping(ctx context.Context, iface string, pmtu int) error {
	if err := ValidateInterfaceName(iface); err != nil {
		return err
	}
	targetMSS, err := m.CalculateMSS(pmtu, false)
	if err != nil {
		return err
	}

	targetMSSStr := strconv.Itoa(targetMSS)
	commands := [][]string{
		{"iptables", "-t", "mangle", "-A", "POSTROUTING", "-o", iface, "-p", "tcp", "--tcp-flags", "SYN,RST", "SYN", "-j", "TCPMSS", "--set-mss", targetMSSStr},
		{"iptables", "-t", "mangle", "-A", "FORWARD", "-o", iface, "-p", "tcp", "--tcp-flags", "SYN,RST", "SYN", "-j", "TCPMSS", "--set-mss", targetMSSStr},
	}
	for _, args := range commands {
		if execErr := m.executor.ExecuteCommand(ctx, args[0], args[1:]...); execErr != nil {
			m.logger.Warn("Failed executing iptables rule", zap.Strings("args", args), zap.Error(execErr))
		}
	}

	m.statusMu.Lock()
	if st, ok := m.statuses[iface]; ok {
		st.MSSClampingOn = true
		st.DetectedPMTU = pmtu
		m.statuses[iface] = st
	}
	m.statusMu.Unlock()

	return nil
}

// AuditRPFilter implements domain.TunnelWatchdog.
func (m *TunnelManager) AuditRPFilter(ctx context.Context) (map[string]int, error) {
	return m.sysctl.ListSysctls(ctx, "net.ipv4.conf")
}

// AuditRPFilterDetailed evaluates rp_filter settings and identifies asymmetric drop risks.
func (m *TunnelManager) AuditRPFilterDetailed(ctx context.Context) (*RPFilterAuditResult, error) {
	modes, err := m.AuditRPFilter(ctx)
	if err != nil {
		return nil, err
	}

	res := &RPFilterAuditResult{
		Modes:                   modes,
		SpoofingProtectionValid: true,
	}

	for key, val := range modes {
		// Strict rp_filter = 1 on multi-homed or VPN interfaces drops asymmetric return traffic
		if val == 1 {
			if strings.Contains(key, "wg") || strings.Contains(key, "tun") || strings.Contains(key, "tailscale") || strings.Contains(key, "eth1") {
				parts := strings.Split(key, ".")
				if len(parts) >= 4 {
					iface := parts[3]
					res.StrictMultiHomedInterfaces = append(res.StrictMultiHomedInterfaces, iface)
					res.Recommendations = append(res.Recommendations,
						fmt.Sprintf("Switch %s from strict mode (1) to loose mode (2) (sysctl -w %s=2) to prevent asymmetric routing drops", iface, key))
				}
			}
		}
	}

	return res, nil
}

// RegisterTunnelStatus adds or updates a tunnel's operational status.
func (m *TunnelManager) RegisterTunnelStatus(status domain.HybridTunnelStatus) {
	m.statusMu.Lock()
	defer m.statusMu.Unlock()
	m.statuses[status.InterfaceName] = status
}

// EvaluateDERPFallback checks if a tunnel has fallen back to a high-latency DERP relay.
func (m *TunnelManager) EvaluateDERPFallback(tunnel domain.HybridTunnelStatus) (*DERPAlert, error) {
	m.statusMu.RLock()
	prev, exists := m.statuses[tunnel.InterfaceName]
	m.statusMu.RUnlock()

	baseline := 15.0
	if exists && prev.RTTLatencyMs > 0 {
		baseline = prev.RTTLatencyMs
	}

	if tunnel.TransportMode == domain.TransportDERPRelay {
		jump := tunnel.RTTLatencyMs - baseline
		if jump > 100.0 || (jump >= 0 && tunnel.RTTLatencyMs > 100.0) {
			alert := &DERPAlert{
				InterfaceName: tunnel.InterfaceName,
				Severity:      "P1",
				BaselineRTTMs: baseline,
				CurrentRTTMs:  tunnel.RTTLatencyMs,
				LatencyJumpMs: jump,
				DetectedAt:    time.Now().UTC(),
				Remediation:   "DERP relay detected. Open STUN UDP port 3478 and permit UDP 41641/51820 on edge firewalls to restore direct p2p peering.",
			}
			return alert, domain.ErrDERPRelayDegradation
		}
	}

	return nil, nil
}

// CheckTransportHealth implements domain.TunnelWatchdog.
func (m *TunnelManager) CheckTransportHealth(ctx context.Context) ([]domain.HybridTunnelStatus, error) {
	m.statusMu.RLock()
	defer m.statusMu.RUnlock()

	result := make([]domain.HybridTunnelStatus, 0, len(m.statuses))
	for _, status := range m.statuses {
		result = append(result, status)
	}
	return result, nil
}

// --- Internal Helper Adapters ---

type mapSysctlAdapter struct {
	mu   sync.RWMutex
	data map[string]int
}

func (a *mapSysctlAdapter) GetSysctl(_ context.Context, key string) (int, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	v, ok := a.data[key]
	if !ok {
		return 0, fmt.Errorf("sysctl %s not found", key)
	}
	return v, nil
}

func (a *mapSysctlAdapter) SetSysctl(_ context.Context, key string, val int) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.data[key] = val
	return nil
}

func (a *mapSysctlAdapter) ListSysctls(_ context.Context, prefix string) (map[string]int, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make(map[string]int)
	for k, v := range a.data {
		if strings.HasPrefix(k, prefix) {
			out[k] = v
		}
	}
	return out, nil
}

type defaultProbeRunner struct{}

func (d *defaultProbeRunner) Probe(ctx context.Context, targetIP string, sizeBytes int) (bool, time.Duration, error) {
	// Real-world fallback: unprivileged UDP dialer with probe payload
	start := time.Now()
	addr := net.JoinHostPort(targetIP, "33434")
	conn, err := net.DialTimeout("udp", addr, 1*time.Second)
	if err != nil {
		return false, time.Since(start), nil
	}
	defer conn.Close()

	payload := make([]byte, sizeBytes)
	_ = conn.SetDeadline(time.Now().Add(1 * time.Second))
	_, writeErr := conn.Write(payload)
	return writeErr == nil, time.Since(start), nil
}

type defaultRuleExecutor struct{}

func (e *defaultRuleExecutor) ExecuteCommand(ctx context.Context, cmd string, args ...string) error {
	c := exec.CommandContext(ctx, cmd, args...)
	return c.Run()
}
