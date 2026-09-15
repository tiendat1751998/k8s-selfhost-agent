package agent_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	domainLogging "github.com/datdt/k8sselfhost/internal/domain/logging"
	docker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/infrastructure/agent"
	infraLogging "github.com/datdt/k8sselfhost/internal/infrastructure/logging"
)

// mockComputeHostRepo implements docker.ComputeHostRepository for unit tests.
type mockComputeHostRepo struct {
	hosts []docker.ComputeHost
	err   error
}

func (m *mockComputeHostRepo) Create(ctx context.Context, host *docker.ComputeHost) error {
	return nil
}
func (m *mockComputeHostRepo) GetByID(ctx context.Context, id string) (*docker.ComputeHost, error) {
	for _, h := range m.hosts {
		if h.ID == id {
			return &h, nil
		}
	}
	return nil, fmt.Errorf("host not found")
}
func (m *mockComputeHostRepo) List(ctx context.Context, tenantID string) ([]docker.ComputeHost, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.hosts, nil
}
func (m *mockComputeHostRepo) ListAll(ctx context.Context) ([]docker.ComputeHost, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.hosts, nil
}
func (m *mockComputeHostRepo) Update(ctx context.Context, host *docker.ComputeHost) error {
	return nil
}
func (m *mockComputeHostRepo) Delete(ctx context.Context, id string) error {
	return nil
}
func (m *mockComputeHostRepo) UpdateStatus(ctx context.Context, id string, status string, lastHealthCheck time.Time) error {
	return nil
}

// mockLogClient implements agent.LogClientInterface for unit tests.
type mockLogClient struct {
	clusterResults []agent.LogSearchResult
	clusterErr     error
	services       []string
	hostServices   map[string][]string
	servicesErr    error
	lastHosts      []docker.ComputeHost
	lastReq        agent.SearchLogsRequest
}

func (m *mockLogClient) GetNodeLogs(ctx context.Context, hostEndpoint, authToken, app, tail, since, until, q, level string) (string, error) {
	return "", nil
}

func (m *mockLogClient) SearchNodeLogs(ctx context.Context, hostEndpoint, authToken string, req agent.SearchLogsRequest) ([]agent.LogSearchResult, error) {
	return nil, nil
}

func (m *mockLogClient) SearchClusterLogs(ctx context.Context, hosts []docker.ComputeHost, req agent.SearchLogsRequest) ([]agent.LogSearchResult, error) {
	m.lastHosts = hosts
	m.lastReq = req
	if m.clusterErr != nil {
		return nil, m.clusterErr
	}
	return m.clusterResults, nil
}

func (m *mockLogClient) ListNodeServices(ctx context.Context, hostEndpoint, authToken string) ([]string, error) {
	if m.servicesErr != nil {
		return nil, m.servicesErr
	}
	if m.hostServices != nil {
		if s, ok := m.hostServices[hostEndpoint]; ok {
			return s, nil
		}
	}
	return m.services, nil
}

func (m *mockLogClient) GetNodeServices(ctx context.Context, hostEndpoint, authToken string) ([]string, error) {
	return m.ListNodeServices(ctx, hostEndpoint, authToken)
}


func TestDistributedAgentLogRepo_QueryLogs(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	hosts := []docker.ComputeHost{
		{ID: "node-1", Name: "worker-1", Endpoint: "http://10.0.0.1:9100", Status: "connected"},
		{ID: "node-2", Name: "worker-2", Endpoint: "http://10.0.0.2:9100", Status: "connected"},
	}
	hostRepo := &mockComputeHostRepo{hosts: hosts}

	clusterResults := []agent.LogSearchResult{
		{Timestamp: now.Add(-30 * time.Second), NodeID: "node-1", NodeName: "worker-1", Service: "auth-svc", Message: "User login succeeded", Level: "info"},
		{Timestamp: now.Add(-20 * time.Second), NodeID: "node-2", NodeName: "worker-2", Service: "order-svc", Message: "Payment processed", Level: "info"},
		{Timestamp: now.Add(-10 * time.Second), NodeID: "node-1", NodeName: "worker-1", Service: "auth-svc", Message: "Token refreshed", Level: "debug"},
	}
	logClient := &mockLogClient{clusterResults: clusterResults}
	memRepo := infraLogging.NewMemoryLogRepo(100)

	// Ingest a local in-memory log entry
	_ = memRepo.IngestBatch(ctx, []domainLogging.LogEntry{
		{
			Timestamp:     now.Add(-5 * time.Second),
			TenantID:      "tenant-1",
			ClusterID:     "cluster-1",
			Namespace:     "default",
			PodName:       "gateway-svc",
			ContainerName: "gateway-svc",
			Stream:        "stdout",
			LogLevel:      domainLogging.LogLevelWarn,
			Message:       "High request latency detected",
		},
	})

	repo := agent.NewDistributedAgentLogRepo(hostRepo, logClient, memRepo)

	// 1. Query with all results
	filter := domainLogging.LogFilter{
		TenantID:  "tenant-1",
		ClusterID: "cluster-1",
		Limit:     10,
		Offset:    0,
	}

	result, err := repo.QueryLogs(ctx, filter)
	if err != nil {
		t.Fatalf("QueryLogs failed: %v", err)
	}

	if result.TotalCount != 4 {
		t.Fatalf("expected TotalCount=4, got %d", result.TotalCount)
	}
	if len(result.Entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(result.Entries))
	}
	if result.HasMore {
		t.Fatalf("expected HasMore=false, got true")
	}

	// Verify chronological order (ascending)
	for i := 0; i < len(result.Entries)-1; i++ {
		if result.Entries[i].Timestamp.After(result.Entries[i+1].Timestamp) {
			t.Errorf("entries not sorted chronologically: [%d] %v > [%d] %v",
				i, result.Entries[i].Timestamp, i+1, result.Entries[i+1].Timestamp)
		}
	}

	// 2. Query with Pagination (Limit=2, Offset=1)
	filterPaged := domainLogging.LogFilter{
		TenantID:  "tenant-1",
		ClusterID: "cluster-1",
		Limit:     2,
		Offset:    1,
	}
	resultPaged, err := repo.QueryLogs(ctx, filterPaged)
	if err != nil {
		t.Fatalf("Paged QueryLogs failed: %v", err)
	}
	if len(resultPaged.Entries) != 2 {
		t.Fatalf("expected 2 paged entries, got %d", len(resultPaged.Entries))
	}
	if !resultPaged.HasMore {
		t.Fatalf("expected HasMore=true for offset 1 limit 2 out of 4")
	}
	if resultPaged.Entries[0].Message != "Payment processed" {
		t.Errorf("expected first paged entry to be 'Payment processed', got '%s'", resultPaged.Entries[0].Message)
	}
}


func TestDistributedAgentLogRepo_Pagination_NewestFirstReSortedAscending(t *testing.T) {
	ctx := context.Background()
	t0 := time.Date(2026, 9, 16, 4, 19, 0, 0, time.UTC)
	t1 := time.Date(2026, 9, 16, 5, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 9, 16, 6, 18, 0, 0, time.UTC)

	clusterResults := []agent.LogSearchResult{
		{Timestamp: t0, NodeID: "n1", Service: "app", Message: "oldest log 04:19", Level: "info"},
		{Timestamp: t1, NodeID: "n1", Service: "app", Message: "middle log 05:00", Level: "info"},
		{Timestamp: t2, NodeID: "n1", Service: "app", Message: "newest log 06:18", Level: "info"},
	}
	hostRepo := &mockComputeHostRepo{hosts: []docker.ComputeHost{{ID: "n1", Endpoint: "http://10.0.0.1:9100"}}}
	logClient := &mockLogClient{clusterResults: clusterResults}
	memRepo := infraLogging.NewMemoryLogRepo(100)
	repo := agent.NewDistributedAgentLogRepo(hostRepo, logClient, memRepo)

	// With limit 1, offset 0: should return the NEWEST log (t2 06:18), NOT the oldest log (t0 04:19)
	resLimit1, err := repo.QueryLogs(ctx, domainLogging.LogFilter{Limit: 1, Offset: 0})
	if err != nil {
		t.Fatalf("QueryLogs failed: %v", err)
	}
	if len(resLimit1.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(resLimit1.Entries))
	}
	if resLimit1.Entries[0].Message != "newest log 06:18" {
		t.Fatalf("expected newest log 'newest log 06:18', got '%s'", resLimit1.Entries[0].Message)
	}

	// With limit 2, offset 0: should return the 2 newest logs (t1 05:00, t2 06:18) sorted ascending
	resLimit2, err := repo.QueryLogs(ctx, domainLogging.LogFilter{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("QueryLogs failed: %v", err)
	}
	if len(resLimit2.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(resLimit2.Entries))
	}
	if resLimit2.Entries[0].Message != "middle log 05:00" || resLimit2.Entries[1].Message != "newest log 06:18" {
		t.Fatalf("expected [middle log 05:00, newest log 06:18], got [%s, %s]",
			resLimit2.Entries[0].Message, resLimit2.Entries[1].Message)
	}
}

func TestDistributedAgentLogRepo_GetHistogram(t *testing.T) {
	ctx := context.Background()
	baseTime := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)

	clusterResults := []agent.LogSearchResult{
		{Timestamp: baseTime.Add(10 * time.Second), NodeID: "node-1", Service: "api", Message: "Request OK", Level: "info"},
		{Timestamp: baseTime.Add(20 * time.Second), NodeID: "node-1", Service: "api", Message: "Request warning", Level: "warn"},
		{Timestamp: baseTime.Add(70 * time.Second), NodeID: "node-1", Service: "api", Message: "DB error", Level: "error"},
	}
	hostRepo := &mockComputeHostRepo{hosts: []docker.ComputeHost{{ID: "node-1", Endpoint: "http://10.0.0.1:9100"}}}
	logClient := &mockLogClient{clusterResults: clusterResults}
	memRepo := infraLogging.NewMemoryLogRepo(100)

	repo := agent.NewDistributedAgentLogRepo(hostRepo, logClient, memRepo)

	buckets, err := repo.GetHistogram(ctx, domainLogging.LogFilter{}, 60)
	if err != nil {
		t.Fatalf("GetHistogram failed: %v", err)
	}

	if len(buckets) != 2 {
		t.Fatalf("expected 2 histogram buckets, got %d", len(buckets))
	}

	// First bucket: 1 info, 1 warn -> total 2
	if buckets[0].TotalCount != 2 {
		t.Errorf("expected bucket[0].TotalCount=2, got %d", buckets[0].TotalCount)
	}
	if buckets[0].LevelCount["info"] != 1 || buckets[0].LevelCount["warn"] != 1 {
		t.Errorf("unexpected bucket[0] level counts: %v", buckets[0].LevelCount)
	}

	// Second bucket: 1 error -> total 1
	if buckets[1].TotalCount != 1 {
		t.Errorf("expected bucket[1].TotalCount=1, got %d", buckets[1].TotalCount)
	}
	if buckets[1].LevelCount["error"] != 1 {
		t.Errorf("unexpected bucket[1] error count: %v", buckets[1].LevelCount)
	}
}

func TestDistributedAgentLogRepo_GetStatus(t *testing.T) {
	ctx := context.Background()
	hostRepo := &mockComputeHostRepo{hosts: []docker.ComputeHost{{ID: "node-1", Endpoint: "http://10.0.0.1:9100"}}}
	logClient := &mockLogClient{}
	memRepo := infraLogging.NewMemoryLogRepo(100)

	repo := agent.NewDistributedAgentLogRepo(hostRepo, logClient, memRepo)

	status, err := repo.GetStatus(ctx)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if status.Engine != "Distributed Edge LogEngine (k8s-agent)" {
		t.Errorf("expected Engine='Distributed Edge LogEngine (k8s-agent)', got '%s'", status.Engine)
	}
	if status.Status != "connected" {
		t.Errorf("expected Status='connected', got '%s'", status.Status)
	}
	if status.RetentionDays != 7 {
		t.Errorf("expected RetentionDays=7, got %d", status.RetentionDays)
	}
}

func TestDistributedAgentLogRepo_TailLogs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hostRepo := &mockComputeHostRepo{}
	logClient := &mockLogClient{}
	memRepo := infraLogging.NewMemoryLogRepo(100)

	repo := agent.NewDistributedAgentLogRepo(hostRepo, logClient, memRepo)

	ch, err := repo.TailLogs(ctx, domainLogging.LogFilter{TenantID: "tenant-1"})
	if err != nil {
		t.Fatalf("TailLogs failed: %v", err)
	}

	entry := domainLogging.LogEntry{
		Timestamp: time.Now().UTC(),
		TenantID:  "tenant-1",
		ClusterID: "cluster-1",
		Message:   "stream test message",
		Stream:    "stdout",
		LogLevel:  domainLogging.LogLevelInfo,
	}

	_ = repo.IngestBatch(ctx, []domainLogging.LogEntry{entry})

	select {
	case received := <-ch:
		if received.Message != "stream test message" {
			t.Errorf("expected message 'stream test message', got '%s'", received.Message)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for log entry from TailLogs")
	}
}

func TestDistributedAgentLogRepo_FallbackWhenNoHosts(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	// Empty hosts
	hostRepo := &mockComputeHostRepo{hosts: []docker.ComputeHost{}}
	logClient := &mockLogClient{}
	memRepo := infraLogging.NewMemoryLogRepo(100)

	_ = memRepo.IngestBatch(ctx, []domainLogging.LogEntry{
		{
			Timestamp: now,
			TenantID:  "tenant-1",
			ClusterID: "cluster-1",
			Message:   "fallback in-memory message",
			Stream:    "stdout",
			LogLevel:  domainLogging.LogLevelInfo,
		},
	})

	repo := agent.NewDistributedAgentLogRepo(hostRepo, logClient, memRepo)

	res, err := repo.QueryLogs(ctx, domainLogging.LogFilter{TenantID: "tenant-1", Limit: 10})
	if err != nil {
		t.Fatalf("expected fallback query to succeed, got error: %v", err)
	}
	if len(res.Entries) != 1 || res.Entries[0].Message != "fallback in-memory message" {
		t.Fatalf("expected 1 fallback entry, got %v", res.Entries)
	}
}

func TestDistributedAgentLogRepo_QueryLogs_NodeFilter(t *testing.T) {
	ctx := context.Background()
	hosts := []docker.ComputeHost{
		{ID: "node-1", Name: "worker-1", Endpoint: "http://10.0.0.1:9100", Status: "connected"},
		{ID: "node-2", Name: "worker-2", Endpoint: "http://10.0.0.2:9100", Status: "connected"},
	}
	hostRepo := &mockComputeHostRepo{hosts: hosts}
	logClient := &mockLogClient{}
	repo := agent.NewDistributedAgentLogRepo(hostRepo, logClient)

	// 1. Filter by node ID "node-1"
	filterNode := domainLogging.LogFilter{
		SearchText: "critical error",
		Attributes: map[string]string{
			"node": "node-1",
		},
	}
	_, err := repo.QueryLogs(ctx, filterNode)
	if err != nil {
		t.Fatalf("QueryLogs failed: %v", err)
	}
	if len(logClient.lastHosts) != 1 || logClient.lastHosts[0].ID != "node-1" {
		t.Fatalf("expected only node-1 in candidate hosts, got %v", logClient.lastHosts)
	}
	// Acceptance criteria: SearchLogsRequest.Query does NOT contain node name
	if logClient.lastReq.Query != "critical error" {
		t.Fatalf("expected SearchLogsRequest.Query to be 'critical error', got '%s'", logClient.lastReq.Query)
	}

	// 2. Filter by node_name "worker-2"
	filterNodeName := domainLogging.LogFilter{
		SearchText: "oom kill",
		Attributes: map[string]string{
			"node_name": "worker-2",
		},
	}
	_, err = repo.QueryLogs(ctx, filterNodeName)
	if err != nil {
		t.Fatalf("QueryLogs failed: %v", err)
	}
	if len(logClient.lastHosts) != 1 || logClient.lastHosts[0].Name != "worker-2" {
		t.Fatalf("expected only worker-2 in candidate hosts, got %v", logClient.lastHosts)
	}
	if logClient.lastReq.Query != "oom kill" {
		t.Fatalf("expected SearchLogsRequest.Query to be 'oom kill', got '%s'", logClient.lastReq.Query)
	}

	// 3. No node filter -> queries all hosts
	filterAll := domainLogging.LogFilter{
		SearchText: "general search",
	}
	_, err = repo.QueryLogs(ctx, filterAll)
	if err != nil {
		t.Fatalf("QueryLogs failed: %v", err)
	}
	if len(logClient.lastHosts) != 2 {
		t.Fatalf("expected all 2 hosts when no node filter provided, got %d", len(logClient.lastHosts))
	}
}

func TestDistributedAgentLogRepo_ListServices(t *testing.T) {
	ctx := context.Background()
	hosts := []docker.ComputeHost{
		{ID: "node-1", Name: "worker-1", Endpoint: "http://10.0.0.1:9100", Status: "connected", Labels: map[string]string{"auth_token": "tok-1"}},
		{ID: "node-2", Name: "worker-2", Endpoint: "http://10.0.0.2:9100", Status: "connected", Labels: map[string]string{"token": "tok-2"}},
		{ID: "node-3", Name: "worker-3", Endpoint: "http://10.0.0.3:9100", Status: "disconnected"},
		{ID: "node-4", Name: "worker-4", Endpoint: "", Status: "connected"},
	}
	hostRepo := &mockComputeHostRepo{hosts: hosts}
	logClient := &mockLogClient{
		hostServices: map[string][]string{
			"http://10.0.0.1:9100": {"cart-svc", "auth-svc"},
			"http://10.0.0.2:9100": {"auth-svc", "payment-svc", "cart-svc"},
		},
	}
	repo := agent.NewDistributedAgentLogRepo(hostRepo, logClient)

	services, err := repo.ListServices(ctx)
	if err != nil {
		t.Fatalf("ListServices failed: %v", err)
	}

	// Should deduplicate and sort: auth-svc, cart-svc, payment-svc
	if len(services) != 3 {
		t.Fatalf("expected 3 deduplicated services, got %d: %v", len(services), services)
	}
	if services[0] != "auth-svc" || services[1] != "cart-svc" || services[2] != "payment-svc" {
		t.Errorf("unexpected sorted services: %v", services)
	}

	// When no active hosts exist, returns empty slice and nil error
	emptyRepo := agent.NewDistributedAgentLogRepo(&mockComputeHostRepo{hosts: nil}, logClient)
	emptyServices, err := emptyRepo.ListServices(ctx)
	if err != nil {
		t.Fatalf("expected nil error on empty hosts, got %v", err)
	}
	if len(emptyServices) != 0 {
		t.Errorf("expected empty services slice, got %v", emptyServices)
	}
}
