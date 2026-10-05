package metrics

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"go.uber.org/zap"
)

func TestCollector_ScrapeAgent_AuthHeader(t *testing.T) {
	dummyResp := map[string]interface{}{
		"hostname":       "test-host",
		"os":             "linux",
		"uptime_seconds": 100,
	}

	t.Run("per-host label auth_token wins over env", func(t *testing.T) {
		t.Setenv("AGENT_AUTH_TOKEN", "fallback-env-token")
		var receivedAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(dummyResp)
		}))
		defer srv.Close()

		hostRepo := &mockComputeHostRepo{
			hosts: []docker.ComputeHost{
				{
					ID:       "host-1",
					Endpoint: srv.URL,
					Labels: map[string]string{
						"auth_token": "label-token-priority",
					},
				},
			},
		}

		c := NewCollector(nil, hostRepo, nil, zap.NewNop(), WithHTTPClient(srv.Client()))
		c.ScrapeAgent(context.Background(), hostRepo.hosts[0])

		if receivedAuth != "Bearer label-token-priority" {
			t.Errorf("expected 'Bearer label-token-priority', got %q", receivedAuth)
		}
	})

	t.Run("per-host label token wins over env", func(t *testing.T) {
		t.Setenv("AGENT_AUTH_TOKEN", "fallback-env-token")
		var receivedAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(dummyResp)
		}))
		defer srv.Close()

		hostRepo := &mockComputeHostRepo{
			hosts: []docker.ComputeHost{
				{
					ID:       "host-2",
					Endpoint: srv.URL,
					Labels: map[string]string{
						"token": "label-token-fallback",
					},
				},
			},
		}

		c := NewCollector(nil, hostRepo, nil, zap.NewNop(), WithHTTPClient(srv.Client()))
		c.ScrapeAgent(context.Background(), hostRepo.hosts[0])

		if receivedAuth != "Bearer label-token-fallback" {
			t.Errorf("expected 'Bearer label-token-fallback', got %q", receivedAuth)
		}
	})

	t.Run("falls back to AGENT_AUTH_TOKEN env when no label", func(t *testing.T) {
		t.Setenv("AGENT_AUTH_TOKEN", "fallback-env-token")
		var receivedAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(dummyResp)
		}))
		defer srv.Close()

		hostRepo := &mockComputeHostRepo{
			hosts: []docker.ComputeHost{
				{
					ID:       "host-3",
					Endpoint: srv.URL,
					Labels:   nil,
				},
			},
		}

		c := NewCollector(nil, hostRepo, nil, zap.NewNop(), WithHTTPClient(srv.Client()))
		c.ScrapeAgent(context.Background(), hostRepo.hosts[0])

		if receivedAuth != "Bearer fallback-env-token" {
			t.Errorf("expected 'Bearer fallback-env-token', got %q", receivedAuth)
		}
	})
}
