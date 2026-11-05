package agent_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	domainLogging "github.com/datdt/k8sselfhost/internal/domain/logging"
	docker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/infrastructure/agent"
)

type fairMockHostRepo struct {
	hosts []docker.ComputeHost
}

func (m *fairMockHostRepo) List(ctx context.Context, tenantID string) ([]docker.ComputeHost, error) {
	return m.hosts, nil
}
func (m *fairMockHostRepo) ListAll(ctx context.Context) ([]docker.ComputeHost, error) {
	return m.hosts, nil
}
func (m *fairMockHostRepo) GetByID(ctx context.Context, id string) (*docker.ComputeHost, error) {
	return nil, nil
}
func (m *fairMockHostRepo) Create(ctx context.Context, host *docker.ComputeHost) error { return nil }
func (m *fairMockHostRepo) Update(ctx context.Context, host *docker.ComputeHost) error { return nil }
func (m *fairMockHostRepo) Save(ctx context.Context, host *docker.ComputeHost) error { return nil }
func (m *fairMockHostRepo) Delete(ctx context.Context, id string) error               { return nil }
func (m *fairMockHostRepo) UpdateStatus(ctx context.Context, id string, status string, lastHealthCheck time.Time) error {
	return nil
}

type fairMockLogClient struct {
	clusterResults   []agent.LogSearchResult
	unreachableHosts map[string]bool
}

func (m *fairMockLogClient) GetNodeLogs(ctx context.Context, hostEndpoint, authToken, app, tail, since, until, q, level string, k8sMeta ...string) (string, error) {
	return "", nil
}
func (m *fairMockLogClient) SearchNodeLogs(ctx context.Context, hostEndpoint, authToken string, req agent.SearchLogsRequest) ([]agent.LogSearchResult, error) {
	return nil, nil
}
func (m *fairMockLogClient) SearchClusterLogs(ctx context.Context, hosts []docker.ComputeHost, req agent.SearchLogsRequest) ([]agent.LogSearchResult, error) {
	return m.clusterResults, nil
}
func (m *fairMockLogClient) ListNodeServices(ctx context.Context, hostEndpoint, authToken string) ([]string, error) {
	return nil, nil
}
func (m *fairMockLogClient) GetNodeServices(ctx context.Context, hostEndpoint, authToken string) ([]string, error) {
	return []string{"app"}, nil
}
func (m *fairMockLogClient) StreamLogs(ctx context.Context, agentURL, service string, onLine func(line string)) error {
	return nil
}
func (m *fairMockLogClient) IsHostUnreachable(hostEndpoint string) bool {
	if m.unreachableHosts != nil {
		return m.unreachableHosts[hostEndpoint]
	}
	return false
}

func TestDistributedAgentLogRepo_FairMergeAllNodes(t *testing.T) {
	now := time.Now().UTC()
	var results []agent.LogSearchResult

	// Node 1: Noisy node with 30 logs in last 10 seconds
	for i := 0; i < 30; i++ {
		results = append(results, agent.LogSearchResult{
			Timestamp: now.Add(-time.Duration(i) * time.Second),
			NodeID:    "node-noisy",
			NodeName:  "worker-noisy",
			Service:   "chat-api",
			Message:   fmt.Sprintf("noisy log line %d", i),
			Level:     "info",
		})
	}

	// Node 2: Quiet node with 2 logs 50 seconds ago
	results = append(results, agent.LogSearchResult{
		Timestamp: now.Add(-50 * time.Second),
		NodeID:    "node-quiet",
		NodeName:  "worker-quiet",
		Service:   "worker-cron",
		Message:   "quiet cron ran",
		Level:     "info",
	})
	results = append(results, agent.LogSearchResult{
		Timestamp: now.Add(-55 * time.Second),
		NodeID:    "node-quiet",
		NodeName:  "worker-quiet",
		Service:   "worker-cron",
		Message:   "quiet cron finished",
		Level:     "info",
	})

	hosts := []docker.ComputeHost{
		{ID: "node-noisy", Name: "worker-noisy", Endpoint: "http://10.0.0.1:9100", Status: "connected"},
		{ID: "node-quiet", Name: "worker-quiet", Endpoint: "http://10.0.0.2:9100", Status: "connected"},
	}

	hostRepo := &fairMockHostRepo{hosts: hosts}
	logClient := &fairMockLogClient{clusterResults: results}
	repo := agent.NewDistributedAgentLogRepo(hostRepo, logClient)

	// Query All Nodes (targetNode == "") with limit 10
	res, err := repo.QueryLogs(context.Background(), domainLogging.LogFilter{
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("QueryLogs failed: %v", err)
	}

	foundQuiet := false
	for _, entry := range res.Entries {
		if entry.Attributes["node_id"] == "node-quiet" {
			foundQuiet = true
			break
		}
	}

	if !foundQuiet {
		t.Errorf("fairMergeLogEntries failed: quiet node entries were completely starved by noisy node")
	}
}

func TestDistributedAgentLogRepo_ListServices_SkipsUnreachableHost(t *testing.T) {
	hosts := []docker.ComputeHost{
		{ID: "node-online", Name: "worker-1", Endpoint: "http://10.0.0.1:9100", Status: "connected"},
		{ID: "node-offline", Name: "worker-2", Endpoint: "http://10.0.0.2:9100", Status: "connected"},
	}
	hostRepo := &fairMockHostRepo{hosts: hosts}
	logClient := &fairMockLogClient{
		unreachableHosts: map[string]bool{
			"http://10.0.0.2:9100": true,
		},
	}
	repo := agent.NewDistributedAgentLogRepo(hostRepo, logClient)

	start := time.Now()
	services, err := repo.ListServices(context.Background())
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(services) != 1 || services[0] != "app" {
		t.Errorf("unexpected services: %v", services)
	}
	if elapsed > 1*time.Second {
		t.Errorf("expected fast-fail for unreachable host, took %v", elapsed)
	}
}
