package apfguard_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	domain "github.com/datdt/k8sselfhost/internal/domain/apfguard"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/apfguard"
)

type mockClusterClient struct {
	mu sync.Mutex

	// Priority levels
	priorityLevels []usecase.PriorityLevelInfo
	queueStatuses  []domain.APFQueueStatus
	patchedShares  map[string]int32

	// Watch cache relist
	chunkedCalls    int
	namespaces      []string
	chunkListError  error

	// HPA / Metrics
	metricsAvailable bool
	metricsStatus    int
	metricsErr       error
	currentReplicas  int32
	minReplicas      int32
	maxReplicas      int32
	scaledReplicas   int32
}

func (m *mockClusterClient) ListPriorityLevels(ctx context.Context) ([]usecase.PriorityLevelInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.priorityLevels, nil
}

func (m *mockClusterClient) GetPriorityLevelStatuses(ctx context.Context) ([]domain.APFQueueStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.queueStatuses, nil
}

func (m *mockClusterClient) PatchPriorityLevelShares(ctx context.Context, name string, shares int32) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.patchedShares == nil {
		m.patchedShares = make(map[string]int32)
	}
	m.patchedShares[name] = shares
	return nil
}

func (m *mockClusterClient) ListChunked(ctx context.Context, namespace string, limit int, continueToken string) (int, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chunkedCalls++
	if m.chunkListError != nil {
		return 0, "", m.chunkListError
	}
	if continueToken == "" {
		return limit, "token-page-2", nil
	}
	return 250, "", nil // final chunk
}

func (m *mockClusterClient) GetActiveNamespaces(ctx context.Context) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.namespaces, nil
}

func (m *mockClusterClient) ProbeMetricsAPI(ctx context.Context) (bool, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.metricsAvailable, m.metricsStatus, m.metricsErr
}

func (m *mockClusterClient) GetDeploymentScale(ctx context.Context, namespace, deployment string) (int32, int32, int32, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.currentReplicas, m.minReplicas, m.maxReplicas, nil
}

func (m *mockClusterClient) UpdateDeploymentScale(ctx context.Context, namespace, deployment string, targetReplicas int32) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scaledReplicas = targetReplicas
	m.currentReplicas = targetReplicas
	return nil
}

func TestAPFManager_LeaderElectionStarvationGuard(t *testing.T) {
	client := &mockClusterClient{
		priorityLevels: []usecase.PriorityLevelInfo{
			{Name: "system-leader-election", AssuredConcurrencyShares: 10, IsPrivileged: true},
			{Name: "workload-low", AssuredConcurrencyShares: 20, IsPrivileged: false},
		},
		queueStatuses: []domain.APFQueueStatus{
			domain.NewAPFQueueStatus("system-leader-election", 3, 10, 10, 1),
			domain.NewAPFQueueStatus("workload-low", 0, 5, 20, 0),
		},
	}

	mgr := usecase.NewAPFManager(client, zap.NewNop())
	statuses, err := mgr.AuditAPFQueueStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error auditing APF statuses: %v", err)
	}

	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}

	// Verify alert was logged and shares were patched for system-leader-election
	alerts := mgr.GetStarvationAlerts()
	if len(alerts) == 0 {
		t.Fatalf("expected at least 1 starvation alert for system-leader-election")
	}
	if alerts[0].PriorityLevel != "system-leader-election" {
		t.Fatalf("expected alert for system-leader-election, got: %s", alerts[0].PriorityLevel)
	}
	if alerts[0].Code != "LEADER_ELECTION_QUEUE_STARVED" {
		t.Fatalf("expected code LEADER_ELECTION_QUEUE_STARVED, got: %s", alerts[0].Code)
	}

	// Check that shares were increased
	client.mu.Lock()
	patched, exists := client.patchedShares["system-leader-election"]
	client.mu.Unlock()
	if !exists || patched <= 10 {
		t.Fatalf("expected patched shares > 10, got %d (exists=%v)", patched, exists)
	}
}

func TestAPFManager_RollbackGuardrail_UnprivilegedCapExceeded(t *testing.T) {
	client := &mockClusterClient{
		priorityLevels: []usecase.PriorityLevelInfo{
			{Name: "system-leader-election", AssuredConcurrencyShares: 10, IsPrivileged: true},
			{Name: "workload-low", AssuredConcurrencyShares: 50, IsPrivileged: false},
			{Name: "workload-high", AssuredConcurrencyShares: 25, IsPrivileged: false},
		},
	}

	mgr := usecase.NewAPFManager(client, zap.NewNop())
	// Total capacity = 100. 80% limit = 80 shares.
	// Currently workload-low (50) + workload-high (25) = 75 shares.
	// Increasing workload-low to 60 would make total unprivileged = 60 + 25 = 85 > 80.
	err := mgr.AdjustPriorityLevelShares(context.Background(), "workload-low", 60)
	if !errors.Is(err, domain.ErrUnprivilegedCapExceeded) {
		t.Fatalf("expected ErrUnprivilegedCapExceeded, got: %v", err)
	}

	// But increasing privileged level (system-leader-election) is not blocked by unprivileged cap
	errPriv := mgr.AdjustPriorityLevelShares(context.Background(), "system-leader-election", 20)
	if errPriv != nil {
		t.Fatalf("expected privileged adjustment to succeed, got: %v", errPriv)
	}
}

func TestAPFManager_WatchCache410_JitterAndPagination(t *testing.T) {
	client := &mockClusterClient{
		namespaces: []string{"kube-system", "production"},
	}

	mgr := usecase.NewAPFManager(client, zap.NewNop())

	var capturedJitter time.Duration
	mgr.SetJitterGenerator(func(min, max time.Duration) time.Duration {
		capturedJitter = 10 * time.Second
		return capturedJitter
	})
	mgr.SetSleepFn(func(ctx context.Context, d time.Duration) error {
		return nil // skip actual sleeping in unit test
	})

	policy := domain.WatchCacheCompactionPolicy{
		MaxJitterWindow: 30 * time.Second,
		BatchSize:       500,
		CompactionAge:   5 * time.Minute,
	}

	err := mgr.MitigateWatchCacheCompactionThunderingHerd(context.Background(), policy)
	if err != nil {
		t.Fatalf("unexpected error mitigating thundering herd: %v", err)
	}

	if capturedJitter < 5*time.Second || capturedJitter > 30*time.Second {
		t.Fatalf("jitter %v out of bounds [5s, 30s]", capturedJitter)
	}

	// 2 namespaces, 2 chunks each = 4 chunked calls
	client.mu.Lock()
	calls := client.chunkedCalls
	client.mu.Unlock()
	if calls != 4 {
		t.Fatalf("expected 4 chunked calls across 2 namespaces, got %d", calls)
	}
}

func TestAPFManager_HPAFallbackScaler(t *testing.T) {
	client := &mockClusterClient{
		metricsAvailable: false,
		metricsStatus:    503,
		metricsErr:       domain.ErrMetricsServerUnavailable,
		currentReplicas:  4,
		minReplicas:      4,
		maxReplicas:      20,
	}

	mgr := usecase.NewAPFManager(client, zap.NewNop())

	// Case 1: First probe down (< 2m). Fallback should NOT be triggered yet.
	report, err := mgr.CheckHPAHealthAndApplyFallback(context.Background(), "prod", "api-gateway")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.FallbackTriggered {
		t.Fatalf("expected fallback not triggered on first probe (<2m)")
	}

	// Case 2: Manually simulate metrics server down for > 2m (e.g. 150 seconds ago)
	mgr.SetMetricsUnavailableSince(time.Now().Add(-150 * time.Second))

	reportFallback, err := mgr.CheckHPAHealthAndApplyFallback(context.Background(), "prod", "api-gateway")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reportFallback.FallbackTriggered {
		t.Fatalf("expected fallback triggered after >2m downtime")
	}

	// Fallback calculation: Max(current, ceil(minReplicas * 1.5))
	// minReplicas = 4 -> 4 * 1.5 = 6. Max(4, 6) = 6.
	if reportFallback.TargetReplicas != 6 {
		t.Fatalf("expected target replicas 6, got %d", reportFallback.TargetReplicas)
	}

	client.mu.Lock()
	scaled := client.scaledReplicas
	client.mu.Unlock()
	if scaled != 6 {
		t.Fatalf("expected scaled replicas 6, got %d", scaled)
	}

	// Case 3: Metrics API recovers. Fallback flag reset.
	client.mu.Lock()
	client.metricsAvailable = true
	client.metricsStatus = 200
	client.metricsErr = nil
	client.mu.Unlock()

	reportHealthy, err := mgr.CheckHPAHealthAndApplyFallback(context.Background(), "prod", "api-gateway")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reportHealthy.MetricsAPIAvailable {
		t.Fatalf("expected metrics api available")
	}
	if reportHealthy.FallbackTriggered {
		t.Fatalf("expected fallback not triggered when metrics healthy")
	}
}

func TestAPFManager_ConcurrentSafety(t *testing.T) {
	client := &mockClusterClient{
		priorityLevels: []usecase.PriorityLevelInfo{
			{Name: "system-leader-election", AssuredConcurrencyShares: 10, IsPrivileged: true},
		},
		queueStatuses: []domain.APFQueueStatus{
			domain.NewAPFQueueStatus("system-leader-election", 0, 5, 10, 0),
		},
		namespaces:       []string{"default"},
		metricsAvailable: true,
		metricsStatus:    200,
		currentReplicas:  2,
		minReplicas:      2,
		maxReplicas:      10,
	}

	mgr := usecase.NewAPFManager(client, zap.NewNop())
	mgr.SetSleepFn(func(ctx context.Context, d time.Duration) error { return nil })

	const routines = 20
	var wg sync.WaitGroup
	wg.Add(routines * 3)

	for i := 0; i < routines; i++ {
		go func() {
			defer wg.Done()
			_, _ = mgr.AuditAPFQueueStatus(context.Background())
		}()

		go func() {
			defer wg.Done()
			_ = mgr.MitigateWatchCacheCompactionThunderingHerd(context.Background(), domain.DefaultWatchCacheCompactionPolicy())
		}()

		go func() {
			defer wg.Done()
			_, _ = mgr.CheckHPAHealthAndApplyFallback(context.Background(), "default", "web")
		}()
	}

	wg.Wait()
}
