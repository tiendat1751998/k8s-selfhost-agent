package servicemesh_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	domain "github.com/datdt/k8sselfhost/internal/domain/servicemesh"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/servicemesh"
)

type mockProber struct {
	statusFunc func(ctx context.Context, socketPath string) (*domain.SDSSocketStatus, error)
}

func (m *mockProber) Probe(ctx context.Context, socketPath string) (*domain.SDSSocketStatus, error) {
	if m.statusFunc != nil {
		return m.statusFunc(ctx, socketPath)
	}
	return &domain.SDSSocketStatus{
		SocketPath:    socketPath,
		IsAlive:       true,
		CertRemaining: 12 * time.Hour,
		LastRotatedAt: time.Now().Add(-2 * time.Hour),
	}, nil
}

type mockRecoverer struct {
	mu           sync.Mutex
	recoverCount int
	recoverFunc  func(ctx context.Context, socketPath string) error
}

func (m *mockRecoverer) Recover(ctx context.Context, socketPath string) error {
	m.mu.Lock()
	m.recoverCount++
	m.mu.Unlock()
	if m.recoverFunc != nil {
		return m.recoverFunc(ctx, socketPath)
	}
	return nil
}

func (m *mockRecoverer) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.recoverCount
}

func TestMeshManager_AuditSDSSocket_Healthy(t *testing.T) {
	prober := &mockProber{}
	recoverer := &mockRecoverer{}
	mgr := usecase.NewMeshManager(usecase.WithProber(prober), usecase.WithRecoverer(recoverer))

	ctx := context.Background()
	status, err := mgr.AuditSDSSocket(ctx, "/var/run/secrets/workload-spiffe-uds/socket")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.IsAlive {
		t.Errorf("expected socket to be alive")
	}
	if recoverer.Count() != 0 {
		t.Errorf("expected no recoveries triggered for healthy socket, got %d", recoverer.Count())
	}
}

func TestMeshManager_AuditSDSSocket_HungSocket(t *testing.T) {
	prober := &mockProber{
		statusFunc: func(ctx context.Context, socketPath string) (*domain.SDSSocketStatus, error) {
			select {
			case <-time.After(3500 * time.Millisecond):
				return &domain.SDSSocketStatus{IsAlive: true}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}
	recoverer := &mockRecoverer{}
	mgr := usecase.NewMeshManager(
		usecase.WithProber(prober),
		usecase.WithRecoverer(recoverer),
		usecase.WithProbeTimeout(50*time.Millisecond), // short timeout for quick test execution
	)

	ctx := context.Background()
	start := time.Now()
	status, err := mgr.AuditSDSSocket(ctx, "/var/run/secrets/workload-spiffe-uds/socket")
	elapsed := time.Since(start)

	if !errors.Is(err, domain.ErrSDSSocketUnresponsive) {
		t.Fatalf("expected ErrSDSSocketUnresponsive, got %v", err)
	}
	if status != nil && status.IsAlive {
		t.Errorf("expected status to not be alive")
	}
	if elapsed > 2*time.Second {
		t.Errorf("probe took too long: %v", elapsed)
	}
	if recoverer.Count() != 1 {
		t.Errorf("expected automatic recovery trigger on hung socket, got %d", recoverer.Count())
	}
}

func TestMeshManager_AuditSDSSocket_ExpiringCertTriggersRecovery(t *testing.T) {
	prober := &mockProber{
		statusFunc: func(ctx context.Context, socketPath string) (*domain.SDSSocketStatus, error) {
			return &domain.SDSSocketStatus{
				SocketPath:    socketPath,
				IsAlive:       true,
				CertRemaining: 1 * time.Hour, // < 2h threshold!
				LastRotatedAt: time.Now().Add(-23 * time.Hour),
			}, nil
		},
	}
	recoverer := &mockRecoverer{}
	mgr := usecase.NewMeshManager(usecase.WithProber(prober), usecase.WithRecoverer(recoverer))

	ctx := context.Background()
	status, err := mgr.AuditSDSSocket(ctx, "/var/run/secrets/workload-spiffe-uds/socket")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.IsAlive {
		t.Errorf("expected socket to be alive")
	}
	if recoverer.Count() != 1 {
		t.Errorf("expected automatic recovery trigger when cert remaining < 2h, got %d", recoverer.Count())
	}
}

func TestMeshManager_RecoverSDSSocket(t *testing.T) {
	recoverer := &mockRecoverer{}
	mgr := usecase.NewMeshManager(usecase.WithRecoverer(recoverer))

	ctx := context.Background()
	if err := mgr.RecoverSDSSocket(ctx, "/var/run/secrets/workload-spiffe-uds/socket"); err != nil {
		t.Fatalf("unexpected recovery error: %v", err)
	}
	if recoverer.Count() != 1 {
		t.Errorf("expected 1 recovery, got %d", recoverer.Count())
	}

	if err := mgr.RecoverSDSSocket(ctx, ""); err == nil {
		t.Errorf("expected error for empty socket path")
	}
}

func TestMeshManager_EnforceEgressScope_RestrictsMemory(t *testing.T) {
	mgr := usecase.NewMeshManager()
	ctx := context.Background()

	// Seed high memory usage > 300MB
	mgr.SetAllocatedMemory("checkout-prod", 420)

	deps := []string{"payment-svc.payment.svc.cluster.local", "catalog-db.data.svc.cluster.local"}
	if err := mgr.EnforceEgressScope(ctx, "checkout-prod", deps); err != nil {
		t.Fatalf("unexpected egress scoping error: %v", err)
	}

	policy, crYAML, err := mgr.GetEgressScope("checkout-prod")
	if err != nil {
		t.Fatalf("unexpected error retrieving policy: %v", err)
	}
	if policy.MemoryAllocated >= 100 {
		t.Errorf("expected memory to drop < 100MB, got %d MB", policy.MemoryAllocated)
	}
	if !strings.Contains(crYAML, "kind: Sidecar") {
		t.Errorf("expected Sidecar CR in generated YAML: %s", crYAML)
	}
	if !strings.Contains(crYAML, "payment-svc.payment.svc.cluster.local") {
		t.Errorf("expected dependency in Sidecar hosts: %s", crYAML)
	}
	if !strings.Contains(crYAML, "./*") || !strings.Contains(crYAML, "istio-system/*") {
		t.Errorf("expected local namespace and istio-system in Sidecar hosts: %s", crYAML)
	}
}

func TestMeshManager_EnforceEgressScope_RollbackGuardrail(t *testing.T) {
	mgr := usecase.NewMeshManager()
	ctx := context.Background()

	mgr.SetAllocatedMemory("analytics", 350)

	// Context with simulated dependency break requiring fallback
	ctxFallback := usecase.WithTriggerFallback(ctx, true)
	deps := []string{"unreachable-foreign-service"}

	if err := mgr.EnforceEgressScope(ctxFallback, "analytics", deps); err != nil {
		t.Fatalf("expected graceful fallback, got error: %v", err)
	}

	_, crYAML, err := mgr.GetEgressScope("analytics")
	if err != nil {
		t.Fatalf("unexpected error retrieving policy: %v", err)
	}
	if !strings.Contains(crYAML, "*/analytics") {
		t.Errorf("expected fallback scope '*/analytics' in Sidecar YAML: %s", crYAML)
	}
}

func TestMeshManager_EnforceEgressScope_MemoryExhaustedError(t *testing.T) {
	mgr := usecase.NewMeshManager()
	ctx := context.Background()

	mgr.SetAllocatedMemory("unbounded-svc", 500)
	// Force failure flag to simulate unable to contain memory
	ctxFail := usecase.WithEnforceFailure(ctx, true)

	err := mgr.EnforceEgressScope(ctxFail, "unbounded-svc", []string{"dep1"})
	if !errors.Is(err, domain.ErrEnvoyEgressMemoryExhausted) {
		t.Fatalf("expected ErrEnvoyEgressMemoryExhausted, got %v", err)
	}
}

func TestMeshManager_AuditTunnelMTUBlackHole_DetectsAndRemediates(t *testing.T) {
	mgr := usecase.NewMeshManager()
	ctx := context.Background()

	// Probe target IP with standard 1500 MTU payload across tunnel
	result, err := mgr.AuditTunnelMTUBlackHole(ctx, "10.244.2.15", 1500)
	if err != nil {
		t.Fatalf("unexpected audit error: %v", err)
	}
	if !result.BlackHoleFound {
		t.Errorf("expected black hole to be detected for payload > PMTU")
	}
	if result.DetectedPMTU != 1450 {
		t.Errorf("expected detected PMTU 1450, got %d", result.DetectedPMTU)
	}
	if !strings.Contains(result.ActionTaken, "1410") {
		t.Errorf("expected action to clamp MSS to 1410B (1450-40), got %s", result.ActionTaken)
	}
}

func TestMeshManager_AuditTunnelMTUBlackHole_UnresolvableError(t *testing.T) {
	mgr := usecase.NewMeshManager()
	ctx := context.Background()

	ctxFail := usecase.WithTunnelPatcherFailure(ctx, true)
	_, err := mgr.AuditTunnelMTUBlackHole(ctxFail, "10.244.2.15", 1500)
	if !errors.Is(err, domain.ErrTunnelPMTUBlackHole) {
		t.Fatalf("expected ErrTunnelPMTUBlackHole, got %v", err)
	}
}

func TestMeshManager_HighGoroutineConcurrency(t *testing.T) {
	prober := &mockProber{}
	recoverer := &mockRecoverer{}
	mgr := usecase.NewMeshManager(usecase.WithProber(prober), usecase.WithRecoverer(recoverer))
	ctx := context.Background()
	var wg sync.WaitGroup
	workers := 25

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, _ = mgr.AuditSDSSocket(ctx, "/var/run/secrets/workload-spiffe-uds/socket")
			_ = mgr.EnforceEgressScope(ctx, "concurrent-ns", []string{"dep-a"})
			_, _ = mgr.AuditTunnelMTUBlackHole(ctx, "10.244.1.20", 1400)
		}(i)
	}

	wg.Wait()

	sdsProbes, _, _ := mgr.SDSLatencyStats()
	if sdsProbes == 0 {
		t.Errorf("expected SDS latency metrics recorded")
	}
	_, _, scoped := mgr.MemoryStats()
	if scoped == 0 {
		t.Errorf("expected scoped memory policies recorded")
	}
	probesSent, _, _, _ := mgr.TunnelStats()
	if probesSent == 0 {
		t.Errorf("expected tunnel probe metrics recorded")
	}
}
