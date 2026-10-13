package transitnet_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	domain "github.com/datdt/k8sselfhost/internal/domain/transitnet"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/transitnet"
	"go.uber.org/zap"
)

// Mock BGPRouteAuditor
type mockBGPAuditor struct {
	primaryPath          string
	returnPath           string
	rtt                  time.Duration
	loss                 float64
	directConnectHealthy bool
	err                  error
	appliedPrepends      int
	appliedLink          string
}

func (m *mockBGPAuditor) GetRoutePaths(_ context.Context, _ string) (usecase.RouteAuditData, error) {
	if m.err != nil {
		return usecase.RouteAuditData{}, m.err
	}
	return usecase.RouteAuditData{
		PrimaryPath:          m.primaryPath,
		ReturnPath:           m.returnPath,
		RTTLatency:           m.rtt,
		PacketLossRate:       m.loss,
		DirectConnectHealthy: m.directConnectHealthy,
	}, nil
}

func (m *mockBGPAuditor) ApplyBGPPrepending(_ context.Context, link string, prepends int) error {
	m.appliedLink = link
	m.appliedPrepends = prepends
	return nil
}

// Mock VPCPeeringProber
type mockVPCProber struct {
	sourceVPC string
	targetVPC string
	sourceMTU int
	targetMTU int
	pass      bool
	err       error
}

func (m *mockVPCProber) ProbePath(_ context.Context, _ string, testBytes int) (usecase.VPCProbeData, error) {
	if m.err != nil {
		return usecase.VPCProbeData{}, m.err
	}
	effective := m.sourceMTU
	if m.targetMTU < effective {
		effective = m.targetMTU
	}
	pass := m.pass
	if testBytes > effective {
		pass = false // MTU mismatch drop
	}
	return usecase.VPCProbeData{
		SourceVPCID:  m.sourceVPC,
		TargetVPCID:  m.targetVPC,
		SourceMTU:    m.sourceMTU,
		TargetMTU:    m.targetMTU,
		ProbeSuccess: pass,
	}, nil
}

// Mock NLBProvider
type mockNLBProvider struct {
	crossZoneEnabled bool
	shares           map[string]float64
	setCalls         int
}

func (m *mockNLBProvider) GetNLBStatus(_ context.Context, _ string) (bool, map[string]float64, error) {
	return m.crossZoneEnabled, m.shares, nil
}

func (m *mockNLBProvider) SetCrossZoneBalancing(_ context.Context, _ string, enabled bool) error {
	m.crossZoneEnabled = enabled
	m.setCalls++
	return nil
}

// Mock CoreDNSAdapter
type mockCoreDNSAdapter struct {
	corefile string
}

func (m *mockCoreDNSAdapter) GetCorefile(_ context.Context) (string, error) {
	return m.corefile, nil
}

func (m *mockCoreDNSAdapter) ApplyCorefile(_ context.Context, content string) error {
	m.corefile = content
	return nil
}

func TestRoutingManager_AuditRoutingSymmetry_Symmetric(t *testing.T) {
	bgpMock := &mockBGPAuditor{
		primaryPath:          "dx-con-us-east-1",
		returnPath:           "dx-con-us-east-1",
		rtt:                  4 * time.Millisecond,
		loss:                 0.0,
		directConnectHealthy: true,
	}

	mgr := usecase.NewRoutingManager(
		usecase.WithBGPAuditor(bgpMock),
		usecase.WithLogger(zap.NewNop()),
	)

	integrity, err := mgr.AuditRoutingSymmetry(context.Background(), "10.100.1.50")
	if err != nil {
		t.Fatalf("expected nil error on symmetric path, got: %v", err)
	}
	if integrity.IsAsymmetric {
		t.Errorf("expected IsAsymmetric=false, got true")
	}
	if integrity.PrimaryPath != "dx-con-us-east-1" || integrity.ReturnPath != "dx-con-us-east-1" {
		t.Errorf("unexpected path names: %+v", integrity)
	}
}

func TestRoutingManager_AuditRoutingSymmetry_Asymmetric(t *testing.T) {
	bgpMock := &mockBGPAuditor{
		primaryPath:          "dx-con-us-east-1",
		returnPath:           "vpn-ipsec-backup-01",
		rtt:                  65 * time.Millisecond,
		loss:                 0.02,
		directConnectHealthy: true,
	}

	mgr := usecase.NewRoutingManager(
		usecase.WithBGPAuditor(bgpMock),
		usecase.WithLogger(zap.NewNop()),
	)

	integrity, err := mgr.AuditRoutingSymmetry(context.Background(), "10.100.1.50")
	if !errors.Is(err, domain.ErrBGPAsymmetricPath) {
		t.Fatalf("expected ErrBGPAsymmetricPath, got: %v", err)
	}
	if integrity == nil || !integrity.IsAsymmetric {
		t.Fatalf("expected integrity.IsAsymmetric=true, got %+v", integrity)
	}

	alerts := mgr.GetActiveAlerts()
	if len(alerts) != 1 {
		t.Fatalf("expected 1 P0 alert, got %d", len(alerts))
	}
	if alerts[0].Severity != "P0" {
		t.Errorf("expected alert severity P0, got %s", alerts[0].Severity)
	}
	if !strings.Contains(alerts[0].Recommendation, "AS-Path Prepending (x3)") {
		t.Errorf("alert recommendation missing AS-Path Prepending: %s", alerts[0].Recommendation)
	}
}

func TestRoutingManager_ApplyBGPPrepending_Guardrail(t *testing.T) {
	t.Run("DirectConnect operational: applies prepends successfully", func(t *testing.T) {
		bgpMock := &mockBGPAuditor{
			primaryPath:          "dx-con-us-east-1",
			returnPath:           "vpn-ipsec-backup-01",
			directConnectHealthy: true,
		}
		mgr := usecase.NewRoutingManager(
			usecase.WithBGPAuditor(bgpMock),
			usecase.WithLogger(zap.NewNop()),
		)

		err := mgr.EnforceBGPPrependingGuardrail(context.Background(), "vpn-ipsec-backup-01", 3)
		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}
		if bgpMock.appliedPrepends != 3 || bgpMock.appliedLink != "vpn-ipsec-backup-01" {
			t.Errorf("prepends not applied properly: %d on %s", bgpMock.appliedPrepends, bgpMock.appliedLink)
		}
	})

	t.Run("DirectConnect unhealthy: rollback guardrail blocks alteration", func(t *testing.T) {
		bgpMock := &mockBGPAuditor{
			primaryPath:          "dx-con-us-east-1",
			returnPath:           "vpn-ipsec-backup-01",
			directConnectHealthy: false, // Primary link down!
		}
		mgr := usecase.NewRoutingManager(
			usecase.WithBGPAuditor(bgpMock),
			usecase.WithLogger(zap.NewNop()),
		)

		err := mgr.EnforceBGPPrependingGuardrail(context.Background(), "vpn-ipsec-backup-01", 3)
		if !errors.Is(err, domain.ErrDirectConnectLinkUnhealthy) {
			t.Fatalf("expected ErrDirectConnectLinkUnhealthy, got: %v", err)
		}
		if bgpMock.appliedPrepends != 0 {
			t.Errorf("guardrail violated: prepends were applied when primary link was down")
		}
	})
}

func TestRoutingManager_ValidateVPCPathMTU(t *testing.T) {
	t.Run("Passing probe without black hole", func(t *testing.T) {
		vpcMock := &mockVPCProber{
			sourceVPC: "vpc-100",
			targetVPC: "vpc-200",
			sourceMTU: 1500,
			targetMTU: 1500,
			pass:      true,
		}
		mgr := usecase.NewRoutingManager(
			usecase.WithVPCProber(vpcMock),
			usecase.WithLogger(zap.NewNop()),
		)

		res, err := mgr.ValidateVPCPathMTU(context.Background(), "172.16.1.10", 1400)
		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}
		if res.BlackHoleDetected || res.EffectivePathMTU != 1500 {
			t.Errorf("unexpected MTU validation result: %+v", res)
		}
	})

	t.Run("Mismatched Jumbo 9001 vs Standard 1500 causes black hole", func(t *testing.T) {
		vpcMock := &mockVPCProber{
			sourceVPC: "vpc-jumbo",
			targetVPC: "vpc-standard",
			sourceMTU: 9001,
			targetMTU: 1500,
			pass:      true,
		}
		mgr := usecase.NewRoutingManager(
			usecase.WithVPCProber(vpcMock),
			usecase.WithLogger(zap.NewNop()),
		)

		res, err := mgr.ValidateVPCPathMTU(context.Background(), "172.16.1.10", 9001)
		if !errors.Is(err, domain.ErrVPCPathMTUBlackHole) {
			t.Fatalf("expected ErrVPCPathMTUBlackHole, got: %v", err)
		}
		if res == nil || !res.BlackHoleDetected {
			t.Fatalf("expected BlackHoleDetected=true, got %+v", res)
		}
		if res.EffectivePathMTU != 1500 {
			t.Errorf("expected EffectivePathMTU=1500, got %d", res.EffectivePathMTU)
		}
		if res.ConfiguredMSS != 1460 {
			t.Errorf("expected ConfiguredMSS=1460 (1500-40), got %d", res.ConfiguredMSS)
		}

		rules := mgr.GenerateMSSClampingRules(res.EffectivePathMTU)
		if len(rules) == 0 || !strings.Contains(rules[0], "--set-mss 1460") {
			t.Errorf("invalid MSS clamping rules generated: %v", rules)
		}
	})
}

func TestRoutingManager_AuditNLBCrossZoneBalance(t *testing.T) {
	t.Run("Balanced traffic (< 60% per AZ)", func(t *testing.T) {
		nlbMock := &mockNLBProvider{
			crossZoneEnabled: false,
			shares: map[string]float64{
				"us-east-1a": 0.35,
				"us-east-1b": 0.35,
				"us-east-1c": 0.30,
			},
		}
		mgr := usecase.NewRoutingManager(
			usecase.WithNLBProvider(nlbMock),
			usecase.WithLogger(zap.NewNop()),
		)

		res, err := mgr.AuditNLBCrossZoneBalance(context.Background(), "arn:aws:nlb:prod")
		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}
		if res.ImbalanceDetected {
			t.Errorf("expected ImbalanceDetected=false")
		}
		if nlbMock.setCalls != 0 {
			t.Errorf("expected 0 modifications when balanced")
		}
	})

	t.Run("Imbalance > 60% automatically enables cross-zone balancing", func(t *testing.T) {
		nlbMock := &mockNLBProvider{
			crossZoneEnabled: false,
			shares: map[string]float64{
				"us-east-1a": 0.78,
				"us-east-1b": 0.22,
			},
		}
		mgr := usecase.NewRoutingManager(
			usecase.WithNLBProvider(nlbMock),
			usecase.WithLogger(zap.NewNop()),
		)

		res, err := mgr.AuditNLBCrossZoneBalance(context.Background(), "arn:aws:nlb:prod")
		if !errors.Is(err, domain.ErrNLBAZImbalance) {
			t.Fatalf("expected ErrNLBAZImbalance, got: %v", err)
		}
		if res == nil || !res.ImbalanceDetected {
			t.Fatalf("expected ImbalanceDetected=true, got %+v", res)
		}
		if !res.CrossZoneEnabled {
			t.Errorf("expected CrossZoneEnabled=true after auto-enabling")
		}
		if nlbMock.setCalls != 1 || !nlbMock.crossZoneEnabled {
			t.Errorf("NLB provider cross zone setting was not updated")
		}
	})
}

func TestRoutingManager_SanitizeCoreDNSNegativeCache(t *testing.T) {
	initialCorefile := `.:53 {
    errors
    health
    kubernetes cluster.local in-addr.arpa ip6.arpa {
        pods insecure
        fallthrough in-addr.arpa ip6.arpa
    }
    forward . /etc/resolv.conf
    cache 30
    reload
}`

	dnsMock := &mockCoreDNSAdapter{corefile: initialCorefile}
	mgr := usecase.NewRoutingManager(
		usecase.WithCoreDNSAdapter(dnsMock),
		usecase.WithLogger(zap.NewNop()),
	)

	err := mgr.SanitizeCoreDNSNegativeCache(context.Background(), "aurora.internal", 5)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	sanitized := dnsMock.corefile
	if !strings.Contains(sanitized, "cache denial 5 aurora.internal") {
		t.Fatalf("sanitized Corefile missing denial cache limit:\n%s", sanitized)
	}
}

func TestRoutingManager_ConcurrentMetrics(t *testing.T) {
	bgpMock := &mockBGPAuditor{
		primaryPath:          "dx-1",
		returnPath:           "vpn-1",
		rtt:                  20 * time.Millisecond,
		directConnectHealthy: true,
	}
	mgr := usecase.NewRoutingManager(
		usecase.WithBGPAuditor(bgpMock),
		usecase.WithLogger(zap.NewNop()),
	)

	const workers = 20
	const iterations = 50
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_, _ = mgr.AuditRoutingSymmetry(context.Background(), "10.0.0.1")
			}
		}()
	}

	wg.Wait()

	tracker := mgr.GetFastLatencyTracker()
	expectedProbes := uint64(workers * iterations)
	if tracker.ProbeCount != expectedProbes {
		t.Fatalf("expected ProbeCount=%d, got %d", expectedProbes, tracker.ProbeCount)
	}
}
