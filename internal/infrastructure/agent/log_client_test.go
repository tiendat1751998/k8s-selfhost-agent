package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	docker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
)

func TestAgentLogClient_GetNodeLogs(t *testing.T) {
	expectedToken := "test-agent-secret"
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/logs" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+expectedToken {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("unauthorized"))
			return
		}
		if r.URL.Query().Get("app") != "traefik" {
			t.Errorf("expected query param app=traefik, got %s", r.URL.Query().Get("app"))
		}
		if r.URL.Query().Get("tail") != "50" {
			t.Errorf("expected query param tail=50, got %s", r.URL.Query().Get("tail"))
		}

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("2026-08-24T12:00:00Z [traefik] [INFO] healthy log 1\n2026-08-24T12:01:00Z [traefik] [ERROR] connection error\n"))
	}))
	defer mockServer.Close()

	client := NewAgentLogClient()

	t.Run("Successful GetNodeLogs", func(t *testing.T) {
		logs, err := client.GetNodeLogs(context.Background(), mockServer.URL, expectedToken, "traefik", "50", "15m", "", "", "error")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(logs, "healthy log 1") || !strings.Contains(logs, "connection error") {
			t.Errorf("expected logs content, got %s", logs)
		}
	})

	t.Run("Unauthorized GetNodeLogs", func(t *testing.T) {
		_, err := client.GetNodeLogs(context.Background(), mockServer.URL, "wrong-token", "traefik", "50", "", "", "", "")
		if err == nil {
			t.Fatalf("expected authorization error, got nil")
		}
		if !strings.Contains(err.Error(), "401") {
			t.Errorf("expected error to mention 401, got %v", err)
		}
	})
}

func TestAgentLogClient_SearchNodeLogs(t *testing.T) {
	expectedToken := "test-agent-secret"
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/logs/search" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+expectedToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		var req SearchLogsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		results := []LogSearchResult{
			{
				Timestamp: time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC),
				Service:   "db",
				Message:   "query timeout on users table",
				Level:     "error",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"results": results,
			"total":   1,
		})
	}))
	defer mockServer.Close()

	client := NewAgentLogClient()

	results, err := client.SearchNodeLogs(context.Background(), mockServer.URL, expectedToken, SearchLogsRequest{
		Query: "timeout",
		Level: "error",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("unexpected search error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Service != "db" || !strings.Contains(results[0].Message, "query timeout") {
		t.Errorf("unexpected result: %+v", results[0])
	}
}

func TestAgentLogClient_SearchClusterLogs_ScatterGather(t *testing.T) {
	var node1Hits, node2Hits, node3Hits int32

	t1 := time.Date(2026, 8, 24, 10, 5, 0, 0, time.UTC)
	t2 := time.Date(2026, 8, 24, 10, 1, 0, 0, time.UTC)
	t3 := time.Date(2026, 8, 24, 10, 10, 0, 0, time.UTC)

	// Node 1: returns log at t1
	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&node1Hits, 1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []LogSearchResult{
				{Timestamp: t1, Service: "traefik", Message: "log from node 1", Level: "warn"},
			},
		})
	}))
	defer srv1.Close()

	// Node 2: returns log at t2 (earliest)
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&node2Hits, 1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []LogSearchResult{
				{Timestamp: t2, Service: "postgres", Message: "log from node 2", Level: "error"},
			},
		})
	}))
	defer srv2.Close()

	// Node 3: returns log at t3 (latest)
	srv3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&node3Hits, 1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []LogSearchResult{
				{Timestamp: t3, Service: "nats", Message: "log from node 3", Level: "info"},
			},
		})
	}))
	defer srv3.Close()

	// Node 4: Failing node (simulates network timeout/error)
	srvFailing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal agent crash"))
	}))
	defer srvFailing.Close()

	hosts := []docker.ComputeHost{
		{
			ID:       "host-1",
			Name:     "worker-node-1",
			HostType: "agent",
			Endpoint: srv1.URL,
			Status:   "connected",
			Labels:   map[string]string{"auth_token": "token-1"},
		},
		{
			ID:       "host-2",
			Name:     "worker-node-2",
			HostType: "agent",
			Endpoint: srv2.URL,
			Status:   "ready",
			Labels:   map[string]string{"token": "token-2"},
		},
		{
			ID:       "host-3",
			Name:     "worker-node-3",
			HostType: "agent",
			Endpoint: srv3.URL,
			Status:   "connected",
		},
		{
			ID:       "host-4",
			Name:     "failing-node",
			HostType: "agent",
			Endpoint: srvFailing.URL,
			Status:   "connected",
		},
		{
			ID:       "host-5-disconnected",
			Name:     "offline-node",
			HostType: "agent",
			Endpoint: "http://127.0.0.1:59999",
			Status:   "disconnected",
		},
	}

	client := NewAgentLogClient(WithClusterTimeout(2 * time.Second))

	t.Run("Parallel search and chronological merge", func(t *testing.T) {
		results, err := client.SearchClusterLogs(context.Background(), hosts, SearchLogsRequest{
			Limit: 10,
		})
		if err != nil {
			t.Fatalf("SearchClusterLogs error: %v", err)
		}

		if len(results) != 3 {
			t.Fatalf("expected 3 merged results (ignoring failing and disconnected hosts), got %d", len(results))
		}

		// Verify chronological order: t2 (10:01) -> t1 (10:05) -> t3 (10:10)
		if results[0].Timestamp != t2 || results[0].NodeID != "host-2" || results[0].NodeName != "worker-node-2" {
			t.Errorf("expected first result from node 2 at t2, got %+v", results[0])
		}
		if results[1].Timestamp != t1 || results[1].NodeID != "host-1" || results[1].NodeName != "worker-node-1" {
			t.Errorf("expected second result from node 1 at t1, got %+v", results[1])
		}
		if results[2].Timestamp != t3 || results[2].NodeID != "host-3" || results[2].NodeName != "worker-node-3" {
			t.Errorf("expected third result from node 3 at t3, got %+v", results[2])
		}
	})

	t.Run("Limit capping", func(t *testing.T) {
		results, err := client.SearchClusterLogs(context.Background(), hosts, SearchLogsRequest{
			Limit: 2,
		})
		if err != nil {
			t.Fatalf("SearchClusterLogs error: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 capped results, got %d", len(results))
		}
	})
}
