package agent_test

import (
	"context"
	"sync"
	"testing"
	"time"

	domainLogging "github.com/datdt/k8sselfhost/internal/domain/logging"
	docker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/infrastructure/agent"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

type catalogMockLogClient struct {
	mu           sync.Mutex
	hostServices map[string][]string
	unreachable  map[string]bool
	recentLogs   map[string][]agent.LogSearchResult
	clusterLogs  map[string][]agent.LogSearchResult
	clusterCalls []agent.SearchLogsRequest
}

func (m *catalogMockLogClient) GetNodeLogs(ctx context.Context, hostEndpoint, authToken, app, tail, since, until, q, level string, k8sMeta ...string) (string, error) {
	return "", nil
}

func (m *catalogMockLogClient) SearchNodeLogs(ctx context.Context, hostEndpoint, authToken string, req agent.SearchLogsRequest) ([]agent.LogSearchResult, error) {
	if logs, ok := m.recentLogs[hostEndpoint]; ok {
		return logs, nil
	}
	return nil, nil
}

func (m *catalogMockLogClient) SearchClusterLogs(ctx context.Context, hosts []docker.ComputeHost, req agent.SearchLogsRequest) ([]agent.LogSearchResult, error) {
	m.mu.Lock()
	m.clusterCalls = append(m.clusterCalls, req)
	m.mu.Unlock()
	if m.clusterLogs != nil {
		return m.clusterLogs[req.App], nil
	}
	return nil, nil
}

func (m *catalogMockLogClient) ListNodeServices(ctx context.Context, hostEndpoint, authToken string) ([]string, error) {
	return m.GetNodeServices(ctx, hostEndpoint, authToken)
}

func (m *catalogMockLogClient) GetNodeServices(ctx context.Context, hostEndpoint, authToken string) ([]string, error) {
	if s, ok := m.hostServices[hostEndpoint]; ok {
		return s, nil
	}
	return []string{}, nil
}

func (m *catalogMockLogClient) StreamLogs(ctx context.Context, agentURL, service string, onLine func(line string)) error {
	return nil
}

func (m *catalogMockLogClient) IsHostUnreachable(hostEndpoint string) bool {
	if m.unreachable != nil && m.unreachable[hostEndpoint] {
		return true
	}
	return false
}

func TestDistributedAgentLogRepo_ListServicesCatalog(t *testing.T) {
	ctx := tenancy.WithUserRole(context.Background(), tenancy.PlatformAdminRole)
	hosts := []docker.ComputeHost{
		{ID: "node-1", Name: "k8smater133", Endpoint: "http://10.10.10.133:9100", Status: "connected", Labels: map[string]string{"auth_token": "tok-1"}},
		{ID: "node-2", Name: "k8sworker", Endpoint: "http://10.10.10.134:9100", Status: "connected", Labels: map[string]string{"auth_token": "tok-2"}},
		{ID: "node-3", Name: "k8smasterdeb", Endpoint: "http://10.10.10.60:9100", Status: "offline"},
		{ID: "node-4", Name: "k8sunreachable", Endpoint: "http://10.10.10.99:9100", Status: "connected"},
	}

	hostRepo := &mockComputeHostRepo{hosts: hosts}
	client := &catalogMockLogClient{
		hostServices: map[string][]string{
			"http://10.10.10.133:9100": {"auth-svc", "cart-svc"},
			"http://10.10.10.134:9100": {"kubelet.service", "cron.service", "k8s-agent.service"},
		},
		unreachable: map[string]bool{
			"http://10.10.10.99:9100": true,
		},
	}

	repo := agent.NewDistributedAgentLogRepo(hostRepo, client)

	start := time.Now()
	allServices, byNode, err := repo.ListServicesCatalog(ctx)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error from ListServicesCatalog: %v", err)
	}

	if duration > 100*time.Millisecond {
		t.Errorf("ListServicesCatalog took too long (%v), expected fast execution", duration)
	}

	// For platform_admin: individual host units are consolidated into "journalctl"
	expectedServices := []string{"auth-svc", "cart-svc", "journalctl"}
	if len(allServices) != len(expectedServices) {
		t.Fatalf("expected %d merged services (%v), got %d: %v", len(expectedServices), expectedServices, len(allServices), allServices)
	}
	for i, exp := range expectedServices {
		if allServices[i] != exp {
			t.Errorf("service mismatch at %d: expected %s, got %s", i, exp, allServices[i])
		}
	}

	if len(byNode) < 4 {
		t.Fatalf("expected at least 4 nodes in byNode, got %d: %v", len(byNode), byNode)
	}

	workerServices, ok := byNode["k8sworker"]
	if !ok {
		t.Fatalf("expected k8sworker in byNode")
	}
	if len(workerServices) != 1 || workerServices[0] != "journalctl" {
		t.Errorf("unexpected k8sworker services (expected [journalctl]): %v", workerServices)
	}

	materServices, ok := byNode["k8smater133"]
	if !ok {
		t.Fatalf("expected k8smater133 in byNode")
	}
	if len(materServices) != 3 || materServices[0] != "auth-svc" || materServices[1] != "cart-svc" || materServices[2] != "journalctl" {
		t.Errorf("unexpected k8smater133 services (expected [auth-svc, cart-svc, journalctl]): %v", materServices)
	}

	masterServices, ok := byNode["k8smasterdeb"]
	if !ok {
		t.Fatalf("expected k8smasterdeb in byNode")
	}
	if masterServices == nil {
		t.Errorf("expected non-nil empty slice for offline k8smasterdeb, got nil")
	}
	if len(masterServices) != 0 {
		t.Errorf("expected 0 services for offline k8smasterdeb, got %v", masterServices)
	}

	unreachableServices, ok := byNode["k8sunreachable"]
	if !ok {
		t.Fatalf("expected k8sunreachable in byNode")
	}
	if unreachableServices == nil || len(unreachableServices) != 0 {
		t.Errorf("expected empty non-nil slice for unreachable host, got %v", unreachableServices)
	}

	// Verify ListServices backward-compat
	legacyServices, lErr := repo.ListServices(ctx)
	if lErr != nil {
		t.Fatalf("ListServices error: %v", lErr)
	}
	if len(legacyServices) != len(allServices) {
		t.Errorf("ListServices mismatch: got %d services, expected %d", len(legacyServices), len(allServices))
	}

	// Non-platform admin verification
	viewerCtx := tenancy.WithUserRole(context.Background(), "viewer")
	viewerServices, viewerByNode, vErr := repo.ListServicesCatalog(viewerCtx)
	if vErr != nil {
		t.Fatalf("viewer ListServicesCatalog error: %v", vErr)
	}
	for _, s := range viewerServices {
		if s == "journalctl" || s == "kubelet.service" || s == "cron.service" {
			t.Errorf("viewer should not see host service %q", s)
		}
	}
	for _, svcs := range viewerByNode {
		for _, s := range svcs {
			if s == "journalctl" || s == "kubelet.service" || s == "cron.service" {
				t.Errorf("viewer node should not contain host service %q", s)
			}
		}
	}
}

func TestDistributedAgentLogRepo_ListServicesCatalog_SupplementRecentLogs(t *testing.T) {
	ctx := tenancy.WithUserRole(context.Background(), tenancy.PlatformAdminRole)
	hosts := []docker.ComputeHost{
		{ID: "node-1", Name: "k8sworker", Endpoint: "http://10.10.10.134:9100", Status: "connected"},
	}

	hostRepo := &mockComputeHostRepo{hosts: hosts}
	client := &catalogMockLogClient{
		hostServices: map[string][]string{
			"http://10.10.10.134:9100": {}, // GetNodeServices returns empty
		},
		recentLogs: map[string][]agent.LogSearchResult{
			"http://10.10.10.134:9100": {
				{Service: "order-api"},
				{Service: "kubelet.service"},
				{Service: "cron.service"},
			},
		},
	}

	repo := agent.NewDistributedAgentLogRepo(hostRepo, client)
	allServices, byNode, err := repo.ListServicesCatalog(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// For platform_admin: order-api is kept, kubelet/cron consolidated to journalctl
	if len(allServices) != 2 || allServices[0] != "journalctl" || allServices[1] != "order-api" {
		t.Errorf("expected supplemented services [journalctl, order-api], got %v", allServices)
	}

	if s := byNode["k8sworker"]; len(s) != 2 {
		t.Errorf("expected 2 services for k8sworker, got %v", s)
	}
}

func TestDistributedAgentLogRepo_AllApps_HostLogStarvationBackfill(t *testing.T) {
	ctx := tenancy.WithUserRole(context.Background(), tenancy.PlatformAdminRole)
	hosts := []docker.ComputeHost{
		{ID: "node-1", Name: "k8smater133", Endpoint: "http://10.10.10.133:9100", Status: "connected", Labels: map[string]string{"auth_token": "tok-1"}},
	}
	hostRepo := &mockComputeHostRepo{hosts: hosts}

	now := time.Now().UTC()
	client := &catalogMockLogClient{
		hostServices: map[string][]string{
			"http://10.10.10.133:9100": {"db", "my-nginx", "dockerd-rootless.sh", "fwupd", "cron.service"},
		},
		clusterLogs: map[string][]agent.LogSearchResult{
			"": {
				{Service: "dockerd-rootless.sh", Message: "noisy dockerd line", Timestamp: now.Add(-1 * time.Second)},
				{Service: "fwupd", Message: "fwupd check", Timestamp: now.Add(-2 * time.Second)},
				{Service: "cron.service", Message: "cron tick", Timestamp: now.Add(-3 * time.Second)},
			},
			"db": {
				{Service: "db", Message: "PostgreSQL database ready to accept connections", Timestamp: now.Add(-10 * time.Second)},
			},
			"my-nginx": {
				{Service: "my-nginx", Message: "GET /index.html 200", Timestamp: now.Add(-5 * time.Second)},
			},
		},
	}

	repo := agent.NewDistributedAgentLogRepo(hostRepo, client)

	// Query All Apps with Limit 10
	res, err := repo.QueryLogs(ctx, domainLogging.LogFilter{
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("unexpected QueryLogs error: %v", err)
	}

	client.mu.Lock()
	calls := client.clusterCalls
	client.mu.Unlock()

	if len(calls) == 0 {
		t.Fatalf("expected SearchClusterLogs to be called")
	}
	if calls[0].Limit < 2000 {
		t.Errorf("expected initial All Apps edge query Limit >= 2000, got %d", calls[0].Limit)
	}

	if len(res.Entries) != 2 {
		t.Fatalf("expected 2 backfilled pure app entries (db, my-nginx), got %d entries: %v", len(res.Entries), res.Entries)
	}
	var foundDB, foundNginx bool
	for _, e := range res.Entries {
		if e.ContainerName == "db" || e.PodName == "db" {
			foundDB = true
		}
		if e.ContainerName == "my-nginx" || e.PodName == "my-nginx" {
			foundNginx = true
		}
	}
	if !foundDB || !foundNginx {
		t.Errorf("expected to find db and my-nginx in backfilled entries: %v", res.Entries)
	}
}

