package k8sresilience_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	domain "github.com/datdt/k8sselfhost/internal/domain/k8sresilience"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/k8sresilience"
)

// In-memory test client implementing resilience ports.
type mockClusterClient struct {
	mu           sync.Mutex
	webhooks     map[string]*usecase.WebhookConfig
	webhookPods  map[string]bool // webhookName -> ready
	probeError   map[string]bool // webhookName -> fail probe
	namespaces   map[string]*usecase.NamespaceInfo
	orphanCRDs   map[string][]string
	nodePLEG     map[string]time.Duration
	nodeCRIAlive map[string]bool
	restarts     map[string]int
}

func newMockClusterClient() *mockClusterClient {
	return &mockClusterClient{
		webhooks:     make(map[string]*usecase.WebhookConfig),
		webhookPods:  make(map[string]bool),
		probeError:   make(map[string]bool),
		namespaces:   make(map[string]*usecase.NamespaceInfo),
		orphanCRDs:   make(map[string][]string),
		nodePLEG:     make(map[string]time.Duration),
		nodeCRIAlive: make(map[string]bool),
		restarts:     make(map[string]int),
	}
}

func (m *mockClusterClient) GetWebhookConfigurations(ctx context.Context) ([]usecase.WebhookConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	res := make([]usecase.WebhookConfig, 0, len(m.webhooks))
	for _, w := range m.webhooks {
		res = append(res, *w)
	}
	return res, nil
}

func (m *mockClusterClient) PatchWebhookFailurePolicy(ctx context.Context, webhookName, policy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	w, exists := m.webhooks[webhookName]
	if !exists {
		return domain.ErrWebhookNotFound
	}
	w.FailurePolicy = policy
	return nil
}

func (m *mockClusterClient) GetWebhookPodStatus(ctx context.Context, webhookName string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ready, exists := m.webhookPods[webhookName]
	if !exists {
		return false, nil
	}
	return ready, nil
}

func (m *mockClusterClient) ProbeWebhookEndpoint(ctx context.Context, endpointURL string) (time.Duration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for name, w := range m.webhooks {
		if w.EndpointURL == endpointURL && m.probeError[name] {
			return 100 * time.Millisecond, errors.New("connection refused")
		}
	}
	return 20 * time.Millisecond, nil
}

func (m *mockClusterClient) GetNamespaceInfo(ctx context.Context, namespace string) (*usecase.NamespaceInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ns, exists := m.namespaces[namespace]
	if !exists {
		return nil, domain.ErrNamespaceNotStuck
	}
	return ns, nil
}

func (m *mockClusterClient) ListOrphanCRDFinalizers(ctx context.Context, namespace string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.orphanCRDs[namespace], nil
}

func (m *mockClusterClient) PatchRemoveFinalizers(ctx context.Context, namespace string, finalizersToRemove []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	ns, exists := m.namespaces[namespace]
	if !exists {
		return domain.ErrNamespaceNotStuck
	}
	remaining := make([]string, 0)
	removeSet := make(map[string]bool)
	for _, f := range finalizersToRemove {
		removeSet[f] = true
	}
	for _, f := range ns.Finalizers {
		if !removeSet[f] {
			remaining = append(remaining, f)
		}
	}
	ns.Finalizers = remaining
	return nil
}

func (m *mockClusterClient) GetNodePLEGMetrics(ctx context.Context, nodeName string) (time.Duration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, exists := m.nodePLEG[nodeName]
	if !exists {
		return 0, domain.ErrNodeNotFound
	}
	return d, nil
}

func (m *mockClusterClient) PingContainerdSocket(ctx context.Context, nodeName string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	alive, exists := m.nodeCRIAlive[nodeName]
	if !exists {
		return false, domain.ErrNodeNotFound
	}
	return alive, nil
}

func (m *mockClusterClient) RestartContainerd(ctx context.Context, nodeName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.nodeCRIAlive[nodeName]; !exists {
		return domain.ErrNodeNotFound
	}
	m.restarts[nodeName]++
	m.nodeCRIAlive[nodeName] = true
	m.nodePLEG[nodeName] = 20 * time.Second
	return nil
}

func TestWebhookDeadlockDetectionAndBypass(t *testing.T) {
	client := newMockClusterClient()
	client.webhooks["calico-webhook"] = &usecase.WebhookConfig{
		Name:          "calico-webhook",
		Type:          "validating",
		FailurePolicy: "Fail",
		EndpointURL:   "https://calico-webhook.kube-system.svc:443/validate",
	}
	client.webhookPods["calico-webhook"] = false
	client.probeError["calico-webhook"] = true

	mgr := usecase.NewResilienceManager(client, nil)
	ctx := context.Background()

	// Perform 5 audits to accumulate >= 5 consecutive probe failures
	for i := 0; i < 5; i++ {
		statuses, err := mgr.AuditWebhooks(ctx)
		if err != nil {
			t.Fatalf("AuditWebhooks failed: %v", err)
		}
		if len(statuses) != 1 {
			t.Fatalf("expected 1 webhook status, got %d", len(statuses))
		}
	}

	// Verify deadlock is detected
	statuses, err := mgr.AuditWebhooks(ctx)
	if err != nil {
		t.Fatalf("AuditWebhooks failed: %v", err)
	}
	if !statuses[0].IsDeadlocked {
		t.Fatalf("expected calico-webhook to be flagged as deadlocked")
	}

	// Emergency bypass
	if err := mgr.EmergencyBypassDeadlockedWebhook(ctx, "calico-webhook"); err != nil {
		t.Fatalf("EmergencyBypassDeadlockedWebhook failed: %v", err)
	}

	// Check that policy is now Ignore
	if client.webhooks["calico-webhook"].FailurePolicy != "Ignore" {
		t.Fatalf("expected policy to be Ignore, got %s", client.webhooks["calico-webhook"].FailurePolicy)
	}

	// Guardrail: Attempting to bypass a healthy webhook must be rejected
	client.webhooks["healthy-webhook"] = &usecase.WebhookConfig{
		Name:          "healthy-webhook",
		Type:          "validating",
		FailurePolicy: "Fail",
		EndpointURL:   "https://healthy.svc:443/validate",
	}
	client.webhookPods["healthy-webhook"] = true
	client.probeError["healthy-webhook"] = false

	// Audit once for healthy webhook
	_, _ = mgr.AuditWebhooks(ctx)
	if err := mgr.EmergencyBypassDeadlockedWebhook(ctx, "healthy-webhook"); err == nil {
		t.Fatalf("expected error bypassing healthy webhook, got nil")
	}

	// Now simulate calico-webhook pods recovering and becoming Ready
	client.webhookPods["calico-webhook"] = true
	client.probeError["calico-webhook"] = false

	// Next audit should detect readiness and restore policy to Fail
	restoredStatuses, err := mgr.AuditWebhooks(ctx)
	if err != nil {
		t.Fatalf("AuditWebhooks failed: %v", err)
	}
	if restoredStatuses[0].FailurePolicy != "Fail" {
		t.Fatalf("expected calico-webhook policy to be auto-restored to Fail, got %s", restoredStatuses[0].FailurePolicy)
	}
}

func TestTerminatingNamespaceFinalizerReaper(t *testing.T) {
	client := newMockClusterClient()
	now := time.Now().UTC()
	stuckTime := now.Add(-15 * time.Minute) // 15 mins ago (> 10m threshold)
	recentTime := now.Add(-5 * time.Minute) // 5 mins ago (< 10m threshold)

	client.namespaces["stuck-tenant"] = &usecase.NamespaceInfo{
		Name:                "stuck-tenant",
		Phase:               "Terminating",
		DeletionTimestamp:   &stuckTime,
		TerminatingDuration: 15 * time.Minute,
		Finalizers:          []string{"kubernetes", "custom.crd.io/orphan-finalizer"},
	}
	client.orphanCRDs["stuck-tenant"] = []string{"custom.crd.io/orphan-finalizer"}

	client.namespaces["recent-tenant"] = &usecase.NamespaceInfo{
		Name:                "recent-tenant",
		Phase:               "Terminating",
		DeletionTimestamp:   &recentTime,
		TerminatingDuration: 5 * time.Minute,
		Finalizers:          []string{"custom.crd.io/orphan-finalizer"},
	}

	client.namespaces["active-tenant"] = &usecase.NamespaceInfo{
		Name:                "active-tenant",
		Phase:               "Active",
		TerminatingDuration: 0,
		Finalizers:          []string{"kubernetes"},
	}

	mgr := usecase.NewResilienceManager(client, nil)
	ctx := context.Background()

	// 1. Guardrail: active namespace should fail
	if err := mgr.ReapOrphanedFinalizers(ctx, "active-tenant"); !errors.Is(err, domain.ErrNamespaceNotStuck) {
		t.Fatalf("expected ErrNamespaceNotStuck for active-tenant, got %v", err)
	}

	// 2. Guardrail: terminating for < 10m should fail
	if err := mgr.ReapOrphanedFinalizers(ctx, "recent-tenant"); !errors.Is(err, domain.ErrNamespaceNotStuck) {
		t.Fatalf("expected ErrNamespaceNotStuck for recent-tenant (< 10m), got %v", err)
	}

	// 3. Successful reaping of stuck namespace (> 10m)
	if err := mgr.ReapOrphanedFinalizers(ctx, "stuck-tenant"); err != nil {
		t.Fatalf("expected successful reaping for stuck-tenant, got: %v", err)
	}

	// Verify the orphan finalizer was removed and "kubernetes" finalizer kept
	if len(client.namespaces["stuck-tenant"].Finalizers) != 1 || client.namespaces["stuck-tenant"].Finalizers[0] != "kubernetes" {
		t.Fatalf("expected only 'kubernetes' finalizer remaining, got %v", client.namespaces["stuck-tenant"].Finalizers)
	}
}

func TestKubeletPLEGWatchdog(t *testing.T) {
	client := newMockClusterClient()
	client.nodePLEG["worker-01"] = 30 * time.Second
	client.nodeCRIAlive["worker-01"] = true

	client.nodePLEG["worker-02"] = 210 * time.Second // 3m 30s (> 180s timeout)
	client.nodeCRIAlive["worker-02"] = false

	mgr := usecase.NewResilienceManager(client, nil)
	ctx := context.Background()

	// Healthy node
	p1, err := mgr.CheckPLEGHealth(ctx, "worker-01")
	if err != nil {
		t.Fatalf("CheckPLEGHealth failed: %v", err)
	}
	if !p1.IsHealthy || !p1.ContainerdResponsive {
		t.Fatalf("expected worker-01 to be healthy")
	}

	// Stalled node
	p2, err := mgr.CheckPLEGHealth(ctx, "worker-02")
	if err != nil {
		t.Fatalf("CheckPLEGHealth failed: %v", err)
	}
	if p2.IsHealthy {
		t.Fatalf("expected worker-02 to be unhealthy (>180s)")
	}
	if p2.ActionRequired != "containerd_restart_recommended" {
		t.Fatalf("expected action containerd_restart_recommended, got %s", p2.ActionRequired)
	}

	// Restart containerd
	if err := mgr.RestartStalledContainerd(ctx, "worker-02"); err != nil {
		t.Fatalf("RestartStalledContainerd failed: %v", err)
	}
	if client.restarts["worker-02"] != 1 {
		t.Fatalf("expected 1 restart recorded for worker-02, got %d", client.restarts["worker-02"])
	}

	// Recheck health after restart
	p2After, err := mgr.CheckPLEGHealth(ctx, "worker-02")
	if err != nil {
		t.Fatalf("CheckPLEGHealth failed: %v", err)
	}
	if !p2After.IsHealthy || !p2After.ContainerdResponsive {
		t.Fatalf("expected worker-02 to be healthy after containerd restart")
	}
}

func TestResilienceManager_AuditTrailAndConcurrency(t *testing.T) {
	client := newMockClusterClient()
	client.webhooks["hook-1"] = &usecase.WebhookConfig{
		Name:          "hook-1",
		Type:          "mutating",
		FailurePolicy: "Fail",
		EndpointURL:   "https://hook-1.svc:443/mutate",
	}
	client.webhookPods["hook-1"] = false
	client.probeError["hook-1"] = true

	client.nodePLEG["node-1"] = 50 * time.Second
	client.nodeCRIAlive["node-1"] = true

	mgr := usecase.NewResilienceManager(client, nil)
	ctx := context.Background()

	const workers = 30
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			_, _ = mgr.AuditWebhooks(ctx)
			_, _ = mgr.CheckPLEGHealth(ctx, "node-1")
		}()
	}
	wg.Wait()

	// Immutable audit trail check
	trail := mgr.GetAuditTrail()
	if trail == nil {
		t.Fatal("expected audit trail slice, got nil")
	}
}
