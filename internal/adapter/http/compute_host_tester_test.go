package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/datdt/k8sselfhost/internal/domain/provider/docker"
)

func TestTestAgentHostConnection_AuthHeader(t *testing.T) {
	t.Run("sends Authorization header to metrics endpoint from env fallback", func(t *testing.T) {
		t.Setenv("AGENT_AUTH_TOKEN", "agent-env-token-xyz")

		var metricsAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
				return
			}
			if r.URL.Path == "/metrics" {
				metricsAuth = r.Header.Get("Authorization")
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"hostname": "test-agent",
					"os":       "linux",
				})
				return
			}
			http.NotFound(w, r)
		}))
		defer srv.Close()

		host := &docker.ComputeHost{
			ID:       "agent-1",
			HostType: "agent",
			Endpoint: srv.URL,
		}

		_, info, err := testAgentHostConnection(context.Background(), host)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info["hostname"] != "test-agent" {
			t.Errorf("expected hostname test-agent, got %v", info["hostname"])
		}
		if metricsAuth != "Bearer agent-env-token-xyz" {
			t.Fatalf("expected Authorization 'Bearer agent-env-token-xyz', got %q", metricsAuth)
		}
	})

	t.Run("per-host label auth_token wins over env in testAgentHostConnection", func(t *testing.T) {
		t.Setenv("AGENT_AUTH_TOKEN", "agent-env-token-xyz")

		var metricsAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
				return
			}
			if r.URL.Path == "/metrics" {
				metricsAuth = r.Header.Get("Authorization")
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"hostname": "test-agent",
				})
				return
			}
			http.NotFound(w, r)
		}))
		defer srv.Close()

		host := &docker.ComputeHost{
			ID:       "agent-2",
			HostType: "agent",
			Endpoint: srv.URL,
			Labels: map[string]string{
				"auth_token": "label-token-priority",
			},
		}

		_, _, err := testAgentHostConnection(context.Background(), host)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if metricsAuth != "Bearer label-token-priority" {
			t.Fatalf("expected Authorization 'Bearer label-token-priority', got %q", metricsAuth)
		}
	})
}
