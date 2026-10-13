package servicemesh_test

import (
	"sync"
	"testing"
	"time"
	"unsafe"

	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
	domain "github.com/datdt/k8sselfhost/internal/domain/servicemesh"
)

func TestCacheLinePadding_StructSizes(t *testing.T) {
	// MESI/MOESI cache line protocol verification:
	// Verify that trackers have exact 64-byte alignment and footprint to prevent false sharing.
	if sz := unsafe.Sizeof(domain.PaddedSDSLatencyTracker{}); sz != 64 {
		t.Fatalf("expected PaddedSDSLatencyTracker size 64 bytes, got %d", sz)
	}
	if sz := unsafe.Sizeof(domain.PaddedEnvoyMemoryMonitor{}); sz != 64 {
		t.Fatalf("expected PaddedEnvoyMemoryMonitor size 64 bytes, got %d", sz)
	}
	if sz := unsafe.Sizeof(domain.PaddedTunnelMTUMetrics{}); sz != 64 {
		t.Fatalf("expected PaddedTunnelMTUMetrics size 64 bytes, got %d", sz)
	}
}

func TestPaddedSDSLatencyTracker_Concurrency(t *testing.T) {
	tracker := &domain.PaddedSDSLatencyTracker{}
	var wg sync.WaitGroup
	workers := 50
	iterations := 200

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				tracker.RecordLatency(time.Duration(j+1) * time.Millisecond)
				if j%50 == 0 {
					tracker.RecordHung()
				}
			}
		}(i)
	}

	wg.Wait()

	probes, avgLatency, hung := tracker.Stats()
	expectedProbes := uint64(workers * iterations)
	if probes != expectedProbes {
		t.Errorf("expected probes %d, got %d", expectedProbes, probes)
	}
	if avgLatency <= 0 {
		t.Errorf("expected positive avg latency, got %v", avgLatency)
	}
	if hung != uint64(workers*4) {
		t.Errorf("expected hung %d, got %d", workers*4, hung)
	}
}

func TestPaddedEnvoyMemoryMonitor_Concurrency(t *testing.T) {
	monitor := &domain.PaddedEnvoyMemoryMonitor{}
	var wg sync.WaitGroup
	workers := 30
	iterations := 100

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				monitor.UpdateMemory(uint64(250 + j))
				if j%20 == 0 {
					monitor.RecordScopedPolicy()
				}
			}
		}(i)
	}

	wg.Wait()

	alloc, peak, scoped := monitor.Stats()
	if alloc == 0 {
		t.Errorf("expected non-zero allocated memory")
	}
	if peak < 349 {
		t.Errorf("expected peak memory >= 349 MB, got %d", peak)
	}
	if scoped != uint64(workers*5) {
		t.Errorf("expected scoped %d, got %d", workers*5, scoped)
	}
}

func TestPaddedTunnelMTUMetrics_Concurrency(t *testing.T) {
	metrics := &domain.PaddedTunnelMTUMetrics{}
	var wg sync.WaitGroup
	workers := 40
	iterations := 100

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				isDropped := j%2 == 0
				isBlackHole := j%4 == 0
				metrics.RecordProbe(isDropped, isBlackHole)
				if isBlackHole {
					metrics.RecordClamping()
				}
			}
		}()
	}

	wg.Wait()

	sent, dropped, blackHoles, clamped := metrics.Stats()
	expectedSent := uint64(workers * iterations)
	if sent != expectedSent {
		t.Errorf("expected sent %d, got %d", expectedSent, sent)
	}
	if dropped != uint64(workers*50) {
		t.Errorf("expected dropped %d, got %d", workers*50, dropped)
	}
	if blackHoles != uint64(workers*25) {
		t.Errorf("expected blackholes %d, got %d", workers*25, blackHoles)
	}
	if clamped != uint64(workers*25) {
		t.Errorf("expected clamped %d, got %d", workers*25, clamped)
	}
}

func TestDomainErrors_Definitions(t *testing.T) {
	if domain.ErrSDSSocketUnresponsive == nil {
		t.Fatal("ErrSDSSocketUnresponsive must not be nil")
	}
	if domain.ErrSDSSocketUnresponsive.Code != domain.CodeSDSSocketUnresponsive {
		t.Errorf("expected code %s, got %s", domain.CodeSDSSocketUnresponsive, domain.ErrSDSSocketUnresponsive.Code)
	}

	if domain.ErrEnvoyEgressMemoryExhausted == nil {
		t.Fatal("ErrEnvoyEgressMemoryExhausted must not be nil")
	}
	if domain.ErrEnvoyEgressMemoryExhausted.Code != domain.CodeEnvoyEgressMemoryExhausted {
		t.Errorf("expected code %s, got %s", domain.CodeEnvoyEgressMemoryExhausted, domain.ErrEnvoyEgressMemoryExhausted.Code)
	}

	if domain.ErrTunnelPMTUBlackHole == nil {
		t.Fatal("ErrTunnelPMTUBlackHole must not be nil")
	}
	if domain.ErrTunnelPMTUBlackHole.Code != domain.CodeTunnelPMTUBlackHole {
		t.Errorf("expected code %s, got %s", domain.CodeTunnelPMTUBlackHole, domain.ErrTunnelPMTUBlackHole.Code)
	}

	// Test unwrapping
	var domErr *domainerrors.DomainError
	if !domainerrors.As(domain.ErrSDSSocketUnresponsive, &domErr) {
		t.Errorf("expected ErrSDSSocketUnresponsive to unwrap as DomainError")
	}
}

func TestDomainContracts_Validation(t *testing.T) {
	status := &domain.SDSSocketStatus{
		SocketPath:    "/var/run/secrets/workload-spiffe-uds/socket",
		IsAlive:       true,
		CertRemaining: 4 * time.Hour,
		LastRotatedAt: time.Now().Add(-1 * time.Hour),
	}
	if err := status.Validate(); err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}

	invalidStatus := &domain.SDSSocketStatus{
		SocketPath: "",
	}
	if err := invalidStatus.Validate(); err == nil {
		t.Errorf("expected validation error for empty socket path")
	}

	policy := &domain.EgressScopePolicy{
		Namespace:       "payment",
		AllowedServices: []string{"postgres.db.svc.cluster.local"},
		MaxMemoryMB:     300,
		MemoryAllocated: 120,
	}
	if err := policy.Validate(); err != nil {
		t.Errorf("unexpected policy validation error: %v", err)
	}

	invalidPolicy := &domain.EgressScopePolicy{
		Namespace: "",
	}
	if err := invalidPolicy.Validate(); err == nil {
		t.Errorf("expected validation error for empty namespace")
	}

	probeResult := &domain.TunnelMTUProbeResult{
		TunnelInterface: "vxlan0",
		DetectedPMTU:    1450,
		ConfiguredMTU:   1500,
		BlackHoleFound:  true,
		ActionTaken:     "Clamped TCP MSS to 1410",
	}
	if err := probeResult.Validate(); err != nil {
		t.Errorf("unexpected probe result validation error: %v", err)
	}
}
