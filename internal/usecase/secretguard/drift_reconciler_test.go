package secretguard_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	domain "github.com/datdt/k8sselfhost/internal/domain/secretguard"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/secretguard"
)

// mockDriftDetector implements domain.DriftDetector for testing.
type mockDriftDetector struct {
	mu                  sync.Mutex
	policies            []domain.ExternalSecretPolicy
	mutatedIntervals    map[string]time.Duration
	rolloutsTriggered   []string
	backupExists        bool
	auditErr            error
	mutateErr           error
	rolloutErr          error
	verifyErr           error
}

func newMockDetector() *mockDriftDetector {
	return &mockDriftDetector{
		mutatedIntervals: make(map[string]time.Duration),
	}
}

func (m *mockDriftDetector) AuditExternalSecrets(_ context.Context, _ string) ([]domain.ExternalSecretPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.auditErr != nil {
		return nil, m.auditErr
	}
	return m.policies, nil
}

func (m *mockDriftDetector) MutateRefreshInterval(_ context.Context, ns, name string, minInterval time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.mutateErr != nil {
		return m.mutateErr
	}
	key := ns + "/" + name
	m.mutatedIntervals[key] = minInterval
	return nil
}

func (m *mockDriftDetector) TriggerGracefulRollout(_ context.Context, targetDeployment string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.rolloutErr != nil {
		return m.rolloutErr
	}
	m.rolloutsTriggered = append(m.rolloutsTriggered, targetDeployment)
	return nil
}

func (m *mockDriftDetector) VerifySealedKeyBackup(_ context.Context, _ string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.verifyErr != nil {
		return false, m.verifyErr
	}
	return m.backupExists, nil
}

// mockHealthChecker implements usecase.WorkloadHealthChecker.
type mockHealthChecker struct {
	errorRate float64
	err       error
}

func (m *mockHealthChecker) GetWorkloadErrorRate(_ context.Context, _ string) (float64, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.errorRate, nil
}

func TestDriftReconciler_PollingStormMitigation(t *testing.T) {
	t.Parallel()

	detector := newMockDetector()
	detector.policies = []domain.ExternalSecretPolicy{
		{
			Namespace:          "tenant-a",
			Name:               "high-freq-secret",
			Provider:           domain.ProviderVault,
			ConfiguredInterval: 10 * time.Second, // Severe polling storm hazard
		},
		{
			Namespace:          "tenant-b",
			Name:               "safe-secret",
			Provider:           domain.ProviderAWS,
			ConfiguredInterval: 2 * time.Hour,
		},
	}

	reconciler := usecase.NewDriftReconciler(detector, nil)
	ctx := context.Background()

	result, err := reconciler.AuditAndMitigatePollingStorms(ctx, "")
	if err != nil {
		t.Fatalf("unexpected error during polling storm mitigation: %v", err)
	}

	if result.AuditedCount != 2 {
		t.Errorf("expected 2 audited policies, got %d", result.AuditedCount)
	}
	if result.ViolationsFound != 1 {
		t.Errorf("expected 1 violation, got %d", result.ViolationsFound)
	}
	if result.MutatedCount != 1 {
		t.Errorf("expected 1 mutated policy, got %d", result.MutatedCount)
	}

	// Verify that the mutated interval is at least 1h and at most 1h + 10m jitter
	mutatedInterval, exists := detector.mutatedIntervals["tenant-a/high-freq-secret"]
	if !exists {
		t.Fatalf("expected mutation for tenant-a/high-freq-secret, none found")
	}
	if mutatedInterval < domain.MinSafetyInterval {
		t.Errorf("expected interval >= 1h, got %v", mutatedInterval)
	}
	if mutatedInterval > domain.MinSafetyInterval+domain.MaxJitter {
		t.Errorf("expected interval <= 1h10m, got %v", mutatedInterval)
	}

	// Ensure tenant-b/safe-secret was NOT mutated
	if _, mutated := detector.mutatedIntervals["tenant-b/safe-secret"]; mutated {
		t.Errorf("safe-secret should not have been mutated")
	}
}

func TestDriftReconciler_SecretHashDrift_TriggersRollout(t *testing.T) {
	t.Parallel()

	detector := newMockDetector()
	healthChecker := &mockHealthChecker{errorRate: 0.01} // 1% error rate (healthy, < 5%)
	reconciler := usecase.NewDriftReconciler(detector, healthChecker)
	ctx := context.Background()

	policy := domain.ExternalSecretPolicy{
		Namespace:           "billing",
		Name:                "pg-creds",
		Provider:            domain.ProviderVault,
		ReloaderAutoRestart: false, // In-memory pool needs restart
		SecretDataHash:      "hash-v1",
	}

	// First registration establishes baseline
	if err := reconciler.TrackSecretBaseline(policy); err != nil {
		t.Fatalf("failed to track baseline: %v", err)
	}

	// Now secret rotates upstream -> hash becomes hash-v2
	policyV2 := policy
	policyV2.SecretDataHash = "hash-v2"

	err := reconciler.ReconcileSecretDataHash(ctx, policyV2, "billing-api")
	if err != nil {
		t.Fatalf("unexpected error in drift reconciliation: %v", err)
	}

	if len(detector.rolloutsTriggered) != 1 || detector.rolloutsTriggered[0] != "billing-api" {
		t.Fatalf("expected graceful rollout for billing-api, got %v", detector.rolloutsTriggered)
	}
}

func TestDriftReconciler_SecretHashDrift_ReloaderAutoRestart(t *testing.T) {
	t.Parallel()

	detector := newMockDetector()
	reconciler := usecase.NewDriftReconciler(detector, nil)
	ctx := context.Background()

	policy := domain.ExternalSecretPolicy{
		Namespace:           "billing",
		Name:                "pg-creds",
		Provider:            domain.ProviderVault,
		ReloaderAutoRestart: true, // Auto reload enabled -> no manual rollout restart needed
		SecretDataHash:      "hash-v1",
	}

	_ = reconciler.TrackSecretBaseline(policy)

	policyV2 := policy
	policyV2.SecretDataHash = "hash-v2"

	err := reconciler.ReconcileSecretDataHash(ctx, policyV2, "billing-api")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should NOT trigger manual rollout restart since ReloaderAutoRestart handles it
	if len(detector.rolloutsTriggered) != 0 {
		t.Fatalf("expected 0 rollouts triggered because ReloaderAutoRestart is true, got %d", len(detector.rolloutsTriggered))
	}
}

func TestDriftReconciler_RolloutSafetyGuardrail_HaltsOnDegradation(t *testing.T) {
	t.Parallel()

	detector := newMockDetector()
	// High error rate 8% (> 5% threshold)
	healthChecker := &mockHealthChecker{errorRate: 0.08}
	reconciler := usecase.NewDriftReconciler(detector, healthChecker)
	ctx := context.Background()

	policy := domain.ExternalSecretPolicy{
		Namespace:           "billing",
		Name:                "pg-creds",
		Provider:            domain.ProviderVault,
		ReloaderAutoRestart: false,
		SecretDataHash:      "hash-v1",
	}
	_ = reconciler.TrackSecretBaseline(policy)

	policyV2 := policy
	policyV2.SecretDataHash = "hash-v2"

	err := reconciler.ReconcileSecretDataHash(ctx, policyV2, "billing-api")
	if !errors.Is(err, domain.ErrRolloutDegraded) {
		t.Fatalf("expected ErrRolloutDegraded due to 8%% error rate, got %v", err)
	}

	// Zero rollouts should be performed
	if len(detector.rolloutsTriggered) != 0 {
		t.Fatalf("expected rollout to be halted, but found %d rollouts executed", len(detector.rolloutsTriggered))
	}
}

func TestDriftReconciler_VerifyMasterKeyBackup(t *testing.T) {
	t.Parallel()

	detector := newMockDetector()
	detector.backupExists = true
	reconciler := usecase.NewDriftReconciler(detector, nil)
	ctx := context.Background()

	ok, err := reconciler.VerifyMasterKeyBackupBeforeRotation(ctx, "kms://aws:eu-west-1:keys/master-sealed")
	if err != nil || !ok {
		t.Fatalf("expected backup to be verified, got ok=%v, err=%v", ok, err)
	}

	// When backup is missing:
	detector.backupExists = false
	ok, err = reconciler.VerifyMasterKeyBackupBeforeRotation(ctx, "kms://aws:eu-west-1:keys/master-sealed")
	if !errors.Is(err, domain.ErrMasterKeyBackupMissing) {
		t.Fatalf("expected ErrMasterKeyBackupMissing, got %v", err)
	}
	if ok {
		t.Fatalf("expected ok=false when backup is missing")
	}
}

func TestDriftReconciler_ConcurrentMultiTenantReconciliation(t *testing.T) {
	t.Parallel()

	detector := newMockDetector()
	detector.backupExists = true
	healthChecker := &mockHealthChecker{errorRate: 0.01}
	reconciler := usecase.NewDriftReconciler(detector, healthChecker)

	const workerCount = 20
	var wg sync.WaitGroup
	wg.Add(workerCount)

	for i := 0; i < workerCount; i++ {
		go func(workerID int) {
			defer wg.Done()
			ctx := context.Background()

			policy := domain.ExternalSecretPolicy{
				Namespace:           "tenant-shard",
				Name:                "creds",
				Provider:            domain.ProviderVault,
				ConfiguredInterval:  10 * time.Second,
				ReloaderAutoRestart: workerID%2 == 0,
				SecretDataHash:      "v1",
			}

			_ = reconciler.TrackSecretBaseline(policy)

			policy.SecretDataHash = "v2"
			_ = reconciler.ReconcileSecretDataHash(ctx, policy, "service-pod")

			_, _ = reconciler.VerifyMasterKeyBackupBeforeRotation(ctx, "kms://test-key")

			snapshot := reconciler.GetTelemetrySnapshot()
			if snapshot.DriftDetected == 0 && workerID > 10 {
				// verify counters are readable
			}
		}(i)
	}

	wg.Wait()
}
