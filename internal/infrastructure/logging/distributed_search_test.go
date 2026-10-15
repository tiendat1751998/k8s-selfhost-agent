package logging_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/infrastructure/logging"
)

type mockHostLister struct {
	hosts []docker.ComputeHost
}

func (m *mockHostLister) ListAll(ctx context.Context) ([]docker.ComputeHost, error) {
	return m.hosts, nil
}

func TestDistributedCoordinator_ScatterGatherMerge(t *testing.T) {
	baseTime := time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC)

	// Agent 1: logs at base + 10s, base + 30s, base + 50s
	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer token-agent-1" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		resp := map[string]any{
			"results": []map[string]any{
				{"timestamp": baseTime.Add(10 * time.Second).Format(time.RFC3339Nano), "service": "cart", "message": "msg-10"},
				{"timestamp": baseTime.Add(30 * time.Second).Format(time.RFC3339Nano), "service": "cart", "message": "msg-30"},
				{"timestamp": baseTime.Add(50 * time.Second).Format(time.RFC3339Nano), "service": "cart", "message": "msg-50"},
			},
			"total": 3,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv1.Close()

	// Agent 2: logs at base + 20s, base + 40s, base + 60s
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer token-agent-2" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		resp := map[string]any{
			"results": []map[string]any{
				{"timestamp": baseTime.Add(20 * time.Second).Format(time.RFC3339Nano), "service": "auth", "message": "msg-20"},
				{"timestamp": baseTime.Add(40 * time.Second).Format(time.RFC3339Nano), "service": "auth", "message": "msg-40"},
				{"timestamp": baseTime.Add(60 * time.Second).Format(time.RFC3339Nano), "service": "auth", "message": "msg-60"},
			},
			"total": 3,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv2.Close()

	// Agent 3: logs at base + 15s, base + 25s, base + 35s
	srv3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"results": []map[string]any{
				{"timestamp": baseTime.Add(15 * time.Second).Format(time.RFC3339Nano), "service": "payment", "message": "msg-15"},
				{"timestamp": baseTime.Add(25 * time.Second).Format(time.RFC3339Nano), "service": "payment", "message": "msg-25"},
				{"timestamp": baseTime.Add(35 * time.Second).Format(time.RFC3339Nano), "service": "payment", "message": "msg-35"},
			},
			"total": 3,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv3.Close()

	lister := &mockHostLister{
		hosts: []docker.ComputeHost{
			{
				ID:       "host-1",
				Name:     "worker-01",
				HostType: "agent",
				Endpoint: srv1.URL,
				Status:   "connected",
				Labels:   map[string]string{"auth_token": "token-agent-1"},
			},
			{
				ID:       "host-2",
				Name:     "worker-02",
				HostType: "agent",
				Endpoint: srv2.URL,
				Status:   "connected",
				Labels:   map[string]string{"bearer_token": "token-agent-2"},
			},
			{
				ID:       "host-3",
				Name:     "worker-03",
				HostType: "agent",
				Endpoint: srv3.URL,
				Status:   "online",
			},
		},
	}

	coordinator := logging.NewDistributedCoordinator(lister)
	ctx := context.Background()

	result, err := coordinator.Search(ctx, logging.DistributedSearchRequest{
		Query: "msg",
		Limit: 5,
	})
	if err != nil {
		t.Fatalf("coordinator.Search failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if len(result.Entries) != 5 {
		t.Fatalf("expected 5 merged entries, got %d", len(result.Entries))
	}

	// Verify Min-Heap chronological order: 10, 15, 20, 25, 30
	expectedMessages := []string{"msg-10", "msg-15", "msg-20", "msg-25", "msg-30"}
	for i, exp := range expectedMessages {
		if result.Entries[i].Message != exp {
			t.Errorf("entry[%d] expected message %s, got %s (timestamp: %v)",
				i, exp, result.Entries[i].Message, result.Entries[i].Timestamp)
		}
	}

	if result.NodesQueried != 3 {
		t.Errorf("expected 3 nodes queried, got %d", result.NodesQueried)
	}
	if result.NodesFailed != 0 {
		t.Errorf("expected 0 nodes failed, got %d", result.NodesFailed)
	}
}

func TestDistributedCoordinator_NodeFailureGraceful(t *testing.T) {
	baseTime := time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC)

	healthySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"results": []map[string]any{
				{"timestamp": baseTime.Format(time.RFC3339Nano), "service": "api", "message": "healthy-msg"},
			},
			"total": 1,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer healthySrv.Close()

	brokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer brokenSrv.Close()

	lister := &mockHostLister{
		hosts: []docker.ComputeHost{
			{
				ID:       "host-ok",
				Name:     "worker-ok",
				HostType: "agent",
				Endpoint: healthySrv.URL,
				Status:   "connected",
			},
			{
				ID:       "host-err",
				Name:     "worker-err",
				HostType: "agent",
				Endpoint: brokenSrv.URL,
				Status:   "connected",
			},
		},
	}

	coordinator := logging.NewDistributedCoordinator(lister)
	result, err := coordinator.Search(context.Background(), logging.DistributedSearchRequest{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected search error: %v", err)
	}

	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry from healthy node, got %d", len(result.Entries))
	}
	if result.Entries[0].Message != "healthy-msg" {
		t.Errorf("unexpected message: %s", result.Entries[0].Message)
	}
	if result.NodesQueried != 2 {
		t.Errorf("expected 2 nodes queried, got %d", result.NodesQueried)
	}
	if result.NodesFailed != 1 {
		t.Errorf("expected 1 node failed, got %d", result.NodesFailed)
	}
}

func TestDistributedCoordinator_SkipOfflineNodes(t *testing.T) {
	baseTime := time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC)

	healthySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"results": []map[string]any{
				{"timestamp": baseTime.Format(time.RFC3339Nano), "service": "api", "message": "online-msg"},
			},
			"total": 1,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer healthySrv.Close()

	lister := &mockHostLister{
		hosts: []docker.ComputeHost{
			{
				ID:       "host-online",
				Name:     "worker-online",
				HostType: "agent",
				Endpoint: healthySrv.URL,
				Status:   "connected",
			},
			{
				ID:       "host-offline",
				Name:     "worker-offline",
				HostType: "agent",
				Endpoint: "http://192.0.2.1:9100", // unroutable test IP
				Status:   "disconnected",
			},
			{
				ID:       "host-docker",
				Name:     "docker-daemon",
				HostType: "docker",
				Endpoint: "tcp://10.0.0.1:2375",
				Status:   "connected",
			},
		},
	}

	coordinator := logging.NewDistributedCoordinator(lister)
	result, err := coordinator.Search(context.Background(), logging.DistributedSearchRequest{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected search error: %v", err)
	}

	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}
	if result.NodesQueried != 1 {
		t.Errorf("expected only 1 online node queried, got %d", result.NodesQueried)
	}
	if result.NodesFailed != 0 {
		t.Errorf("expected 0 failed nodes, got %d", result.NodesFailed)
	}
}
