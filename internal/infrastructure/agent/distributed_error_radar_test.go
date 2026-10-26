package agent_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	domainLogging "github.com/datdt/k8sselfhost/internal/domain/logging"
	docker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/infrastructure/agent"
	infraLogging "github.com/datdt/k8sselfhost/internal/infrastructure/logging"
)

type mockRadarLogClient struct {
	entries []agent.LogSearchResult
	err     error
}

func (m *mockRadarLogClient) GetNodeLogs(ctx context.Context, hostEndpoint, authToken, app, tail, since, until, q, level string, k8sMeta ...string) (string, error) {
	return "", nil
}

func (m *mockRadarLogClient) SearchNodeLogs(ctx context.Context, hostEndpoint, authToken string, req agent.SearchLogsRequest) ([]agent.LogSearchResult, error) {
	return m.entries, m.err
}

func (m *mockRadarLogClient) SearchClusterLogs(ctx context.Context, hosts []docker.ComputeHost, req agent.SearchLogsRequest) ([]agent.LogSearchResult, error) {
	return nil, nil
}

func (m *mockRadarLogClient) ListNodeServices(ctx context.Context, hostEndpoint, authToken string) ([]string, error) {
	return nil, nil
}

func (m *mockRadarLogClient) GetNodeServices(ctx context.Context, hostEndpoint, authToken string) ([]string, error) {
	return nil, nil
}

func (m *mockRadarLogClient) StreamLogs(ctx context.Context, agentURL, service string, onLine func(line string)) error {
	return nil
}

func TestDistributedAgentLogRepo_SummarizeErrors_HttpEndpoint(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	// Mock edge k8s-agent server responding to /logs/errors
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/logs/errors" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"groups": []map[string]any{
					{
						"service":        "auth-service",
						"level":          "error",
						"fingerprint":    "err_jwt_expired",
						"sample_message": "JWT token has expired",
						"count":          15,
						"first_seen":     now.Add(-5 * time.Minute),
						"last_seen":      now,
					},
					{
						"service":        "session-1.slice",
						"level":          "error",
						"fingerprint":    "noise_slice",
						"sample_message": "noise message",
						"count":          99,
						"first_seen":     now,
						"last_seen":      now,
					},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	hostRepo := &mockComputeHostRepo{
		hosts: []docker.ComputeHost{
			{
				ID:       "host-1",
				Name:     "node-primary",
				Endpoint: srv.URL,
				Status:   "connected",
			},
		},
	}

	repo := agent.NewDistributedAgentLogRepo(hostRepo, &mockRadarLogClient{})
	groups, err := repo.SummarizeErrors(context.Background(), domainLogging.LogFilter{Limit: 10})
	if err != nil {
		t.Fatalf("SummarizeErrors failed: %v", err)
	}

	// session-1.slice should be discarded
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d: %+v", len(groups), groups)
	}

	if groups[0].Fingerprint != "err_jwt_expired" || groups[0].Count != 15 || groups[0].Node != "node-primary" {
		t.Errorf("unexpected group data: %+v", groups[0])
	}
}

func TestDistributedAgentLogRepo_SummarizeErrors_FallbackSearchNodeLogs(t *testing.T) {
	now := time.Now().UTC()
	hostRepo := &mockComputeHostRepo{
		hosts: []docker.ComputeHost{
			{
				ID:       "host-2",
				Name:     "node-worker-2",
				Endpoint: "http://127.0.0.1:59999", // Unreachable direct endpoint to trigger fallback
				Status:   "connected",
			},
		},
	}

	client := &mockRadarLogClient{
		entries: []agent.LogSearchResult{
			{
				Service:   "db-service",
				Level:     "error",
				Message:   "connection refused: port 5432",
				Timestamp: now.Add(-2 * time.Minute),
			},
			{
				Service:   "db-service",
				Level:     "error",
				Message:   "connection refused: port 5432",
				Timestamp: now,
			},
		},
	}

	memRepo := infraLogging.NewMemoryLogRepo(100)
	repo := agent.NewDistributedAgentLogRepo(hostRepo, client, memRepo)

	groups, err := repo.SummarizeErrors(context.Background(), domainLogging.LogFilter{Limit: 50})
	if err != nil {
		t.Fatalf("SummarizeErrors fallback failed: %v", err)
	}

	if len(groups) != 1 {
		t.Fatalf("expected 1 group from fallback, got %d", len(groups))
	}
	if groups[0].Count != 2 || groups[0].Service != "db-service" {
		t.Errorf("unexpected fallback summary: %+v", groups[0])
	}
}

func TestDistributedAgentLogRepo_SummarizeErrors_SkipsOfflineHosts(t *testing.T) {
	now := time.Now().UTC()

	connectedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"groups": []map[string]any{
				{
					"service":        "web-service",
					"level":          "error",
					"fingerprint":    "err_500",
					"sample_message": "internal error",
					"count":          1,
					"first_seen":     now,
					"last_seen":      now,
				},
			},
		})
	}))
	defer connectedSrv.Close()

	hostRepo := &mockComputeHostRepo{
		hosts: []docker.ComputeHost{
			{
				ID:       "host-online",
				Name:     "node-online",
				Endpoint: connectedSrv.URL,
				Status:   "connected",
			},
			{
				ID:       "host-dead",
				Name:     "node-dead",
				Endpoint: "http://192.0.2.1:9999", // blackhole IP
				Status:   "disconnected",
			},
		},
	}

	repo := agent.NewDistributedAgentLogRepo(hostRepo, &mockRadarLogClient{})
	// Cluster-wide query (targetNode is "")
	groups, err := repo.SummarizeErrors(context.Background(), domainLogging.LogFilter{Limit: 10})
	if err != nil {
		t.Fatalf("SummarizeErrors failed: %v", err)
	}

	if len(groups) != 1 {
		t.Fatalf("expected 1 group from online host only, got %d", len(groups))
	}
	if groups[0].Node != "node-online" {
		t.Errorf("expected node-online, got %s", groups[0].Node)
	}
}

func TestDistributedAgentLogRepo_SummarizeErrors_FiltersNoiseServices(t *testing.T) {
	now := time.Now().UTC()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"groups": []map[string]any{
				{"service": "user@1000.service", "level": "error", "fingerprint": "f1", "sample_message": "m1", "count": 10, "first_seen": now, "last_seen": now},
				{"service": "user-runtime-dir@1000.service", "level": "error", "fingerprint": "f2", "sample_message": "m2", "count": 5, "first_seen": now, "last_seen": now},
				{"service": "init.scope", "level": "error", "fingerprint": "f3", "sample_message": "m3", "count": 2, "first_seen": now, "last_seen": now},
				{"service": "system.slice", "level": "error", "fingerprint": "f4", "sample_message": "m4", "count": 3, "first_seen": now, "last_seen": now},
				{"service": "-- no entries --", "level": "error", "fingerprint": "f5", "sample_message": "m5", "count": 1, "first_seen": now, "last_seen": now},
				{"service": "payment-api", "level": "error", "fingerprint": "f6", "sample_message": "real error", "count": 7, "first_seen": now, "last_seen": now},
			},
		})
	}))
	defer srv.Close()

	hostRepo := &mockComputeHostRepo{
		hosts: []docker.ComputeHost{
			{ID: "host-1", Name: "node-1", Endpoint: srv.URL, Status: "connected"},
		},
	}
	repo := agent.NewDistributedAgentLogRepo(hostRepo, &mockRadarLogClient{})
	groups, err := repo.SummarizeErrors(context.Background(), domainLogging.LogFilter{Limit: 10})
	if err != nil {
		t.Fatalf("SummarizeErrors failed: %v", err)
	}

	if len(groups) != 1 {
		t.Fatalf("expected only 1 non-noise group (payment-api), got %d: %+v", len(groups), groups)
	}
	if groups[0].Service != "payment-api" {
		t.Errorf("expected payment-api, got %s", groups[0].Service)
	}
}

