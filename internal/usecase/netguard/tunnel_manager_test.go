package netguard_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	domain "github.com/datdt/k8sselfhost/internal/domain/netguard"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/netguard"
)

// mockProbeRunner simulates ICMP/UDP DF packet probes with an MTU cutoff.
type mockProbeRunner struct {
	maxPassingSize int
	probeCount     int
	mu             sync.Mutex
}

func (m *mockProbeRunner) Probe(ctx context.Context, targetIP string, sizeBytes int) (bool, time.Duration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.probeCount++
	if sizeBytes <= m.maxPassingSize {
		return true, 12 * time.Millisecond, nil
	}
	// Simulate packet drop / timeout (MTU blackhole, no ICMP fragmentation response)
	return false, 2 * time.Second, nil
}

// TestPMTUBinarySearch_Within5Iterations verifies that the binary search DF sweeper
// identifies true PMTU (1420 bytes) in <= 5 iterations within [1400, 1500] range.
func TestPMTUBinarySearch_Within5Iterations(t *testing.T) {
	runner := &mockProbeRunner{maxPassingSize: 1420}
	mgr := usecase.NewTunnelManager(
		usecase.WithProbeRunner(runner),
		usecase.WithSweepRange(1400, 1500, 4), // 4-byte word boundary alignment
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := mgr.SweepPMTU(ctx, "10.100.0.1")
	if err != nil {
		t.Fatalf("unexpected error during PMTU sweep: %v", err)
	}

	if result.DiscoveredPMTU != 1420 {
		t.Errorf("expected discovered PMTU 1420, got %d", result.DiscoveredPMTU)
	}

	if result.Iterations > 5 {
		t.Errorf("expected sweep to complete in <= 5 iterations, took %d iterations", result.Iterations)
	}

	if !result.BlackHoleDetected {
		t.Errorf("expected BlackHoleDetected=true since 1500B drops but 1400B passes")
	}

	t.Logf("PMTU sweep succeeded: PMTU=%dB, iterations=%d, probes=%d, blackhole=%v",
		result.DiscoveredPMTU, result.Iterations, runner.probeCount, result.BlackHoleDetected)
}

// TestPMTUBinarySearch_BlackHoleAllFailing verifies error handling when all packets drop.
func TestPMTUBinarySearch_BlackHoleAllFailing(t *testing.T) {
	runner := &mockProbeRunner{maxPassingSize: 1200} // Even 1400B fails
	mgr := usecase.NewTunnelManager(
		usecase.WithProbeRunner(runner),
		usecase.WithSweepRange(1400, 1500, 4),
	)

	ctx := context.Background()
	_, err := mgr.SweepPMTU(ctx, "10.100.0.1")
	if !errors.Is(err, domain.ErrTunnelMTUBlackHole) {
		t.Fatalf("expected ErrTunnelMTUBlackHole, got: %v", err)
	}
}

// TestMSSClamping_CalculationAndRules verifies MSS calculations and rule generation.
func TestMSSClamping_CalculationAndRules(t *testing.T) {
	mgr := usecase.NewTunnelManager()

	// IPv4: 1420 - 40 = 1380
	mssV4, err := mgr.CalculateMSS(1420, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mssV4 != 1380 {
		t.Errorf("expected IPv4 MSS=1380, got %d", mssV4)
	}

	// IPv6: 1420 - 60 = 1360
	mssV6, err := mgr.CalculateMSS(1420, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mssV6 != 1360 {
		t.Errorf("expected IPv6 MSS=1360, got %d", mssV6)
	}

	// Verify iptables commands
	iptRules := mgr.GenerateIPTablesMSSRules("wg0", 1380)
	if len(iptRules) < 2 {
		t.Fatalf("expected at least 2 iptables rules (POSTROUTING & FORWARD), got %d", len(iptRules))
	}
	for _, rule := range iptRules {
		if !strings.Contains(rule, "--set-mss 1380") {
			t.Errorf("rule missing '--set-mss 1380': %s", rule)
		}
		if !strings.Contains(rule, "-o wg0") {
			t.Errorf("rule missing '-o wg0': %s", rule)
		}
	}

	// Verify nftables command
	nftRules := mgr.GenerateNFTablesMSSRules("wg0", 1380)
	if len(nftRules) == 0 {
		t.Fatalf("expected nftables rules, got 0")
	}
	for _, rule := range nftRules {
		if !strings.Contains(rule, "tcp option maxseg size set 1380") {
			t.Errorf("rule missing 'tcp option maxseg size set 1380': %s", rule)
		}
	}
}

// TestDERPRelayFallback_TriggersP1Alert tests detection of DERP relay fallback with latency jump > 100ms.
func TestDERPRelayFallback_TriggersP1Alert(t *testing.T) {
	mgr := usecase.NewTunnelManager()

	// Baseline direct tunnel: 15ms
	baseline := domain.HybridTunnelStatus{
		InterfaceName: "tailscale0",
		TransportMode: domain.TransportP2PDirect,
		RTTLatencyMs:  15.0,
	}
	mgr.RegisterTunnelStatus(baseline)

	// Degraded tunnel: DERP relay fallback with 145ms RTT (jump = 130ms > 100ms)
	degraded := domain.HybridTunnelStatus{
		InterfaceName: "tailscale0",
		TransportMode: domain.TransportDERPRelay,
		RTTLatencyMs:  145.0,
	}

	alert, err := mgr.EvaluateDERPFallback(degraded)
	if !errors.Is(err, domain.ErrDERPRelayDegradation) {
		t.Fatalf("expected ErrDERPRelayDegradation, got %v", err)
	}

	if alert == nil || alert.Severity != "P1" {
		t.Errorf("expected P1 severity alert, got %+v", alert)
	}
	if alert.LatencyJumpMs <= 100.0 {
		t.Errorf("expected latency jump > 100ms, got %f", alert.LatencyJumpMs)
	}
}

// TestAuditRPFilter_StrictMultiHomedWarning tests auditing rp_filter sysctl modes.
func TestAuditRPFilter_StrictMultiHomedWarning(t *testing.T) {
	mockSysctl := map[string]int{
		"net.ipv4.conf.all.rp_filter":     1,
		"net.ipv4.conf.default.rp_filter": 1,
		"net.ipv4.conf.eth0.rp_filter":    1, // WAN
		"net.ipv4.conf.wg0.rp_filter":     1, // VPN tunnel - strict causes asymmetric drops!
	}

	mgr := usecase.NewTunnelManager(
		usecase.WithSysctlMap(mockSysctl),
	)

	ctx := context.Background()
	audit, err := mgr.AuditRPFilterDetailed(ctx)
	if err != nil {
		t.Fatalf("unexpected error auditing rp_filter: %v", err)
	}

	if len(audit.StrictMultiHomedInterfaces) == 0 {
		t.Errorf("expected wg0 to be flagged as strict multi-homed interface")
	}

	foundWg0 := false
	for _, iface := range audit.StrictMultiHomedInterfaces {
		if iface == "wg0" {
			foundWg0 = true
			break
		}
	}
	if !foundWg0 {
		t.Errorf("expected wg0 in StrictMultiHomedInterfaces, got: %v", audit.StrictMultiHomedInterfaces)
	}
}

// TestConcurrentProbes_NoRace tests concurrency on cache-line padded latency tracker.
func TestConcurrentProbes_NoRace(t *testing.T) {
	runner := &mockProbeRunner{maxPassingSize: 1450}
	mgr := usecase.NewTunnelManager(
		usecase.WithProbeRunner(runner),
	)

	var wg sync.WaitGroup
	const workers = 10
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_, _ = mgr.SweepPMTU(ctx, "10.100.0.1")
		}()
	}

	wg.Wait()
}
