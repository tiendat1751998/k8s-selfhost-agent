package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"

	"github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

func TestTestAgentHostConnection_AuthHeader(t *testing.T) {
	t.Run("sends Authorization header to metrics endpoint from env fallback", func(t *testing.T) {
		t.Setenv("AGENT_AUTH_TOKEN", "agent-env-token-xyz")
		t.Setenv("EGRESS_ALLOW_PRIVATE_CIDRS", "127.0.0.0/8")

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
		t.Setenv("EGRESS_ALLOW_PRIVATE_CIDRS", "127.0.0.0/8")

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

func TestComputeHostTester_SSRFBlocked(t *testing.T) {
	ctx := context.Background()

	t.Run("k8s host probe blocks metadata endpoint", func(t *testing.T) {
		host := &docker.ComputeHost{
			ID:       "k8s-meta",
			HostType: "k8s",
			Endpoint: "https://169.254.169.254:443",
		}
		_, err := testK8sHostConnection(ctx, host)
		if err == nil || !strings.Contains(err.Error(), "blocked") {
			t.Fatalf("expected error containing 'blocked', got: %v", err)
		}
	})

	t.Run("database host probe blocks metadata endpoint", func(t *testing.T) {
		host := &docker.ComputeHost{
			ID:       "db-meta",
			HostType: "database",
			Endpoint: "169.254.169.254:5432",
		}
		_, err := testDatabaseHostConnection(ctx, host)
		if err == nil || !strings.Contains(err.Error(), "blocked") {
			t.Fatalf("expected error containing 'blocked', got: %v", err)
		}
	})

	t.Run("custom host probe blocks metadata endpoint", func(t *testing.T) {
		host := &docker.ComputeHost{
			ID:       "custom-meta",
			HostType: "custom",
			Endpoint: "http://169.254.169.254/status",
		}
		_, err := testCustomHostConnection(ctx, host)
		if err == nil || !strings.Contains(err.Error(), "blocked") {
			t.Fatalf("expected error containing 'blocked', got: %v", err)
		}
	})

	t.Run("agent host probe blocks loopback by default", func(t *testing.T) {
		host := &docker.ComputeHost{
			ID:       "agent-loopback",
			HostType: "agent",
			Endpoint: "http://127.0.0.1:9100",
		}
		_, _, err := testAgentHostConnection(ctx, host)
		if err == nil || !strings.Contains(err.Error(), "blocked") {
			t.Fatalf("expected error containing 'blocked', got: %v", err)
		}
	})
}

func TestComputeHostTest_RBAC_Enforcement(t *testing.T) {
	hostRepo := newMockComputeHostRepo()
	handler := NewDockerHandler(nil, hostRepo)

	r := chi.NewRouter()
	r.Route("/docker", handler.RegisterRoutes)
	r.Route("/hosts", handler.RegisterHostRoutes)

	host := &docker.ComputeHost{
		ID:       "host-rbac-test",
		Name:     "rbac-node",
		HostType: "agent",
		Endpoint: "http://192.168.1.10:9100",
		TenantID: "default-tenant",
	}
	_ = hostRepo.Create(context.Background(), host)

	t.Run("operator role receives 403 Forbidden on /docker/hosts/{id}/test", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/docker/hosts/"+host.ID+"/test", nil)
		ctx := context.WithValue(req.Context(), tenancy.UserRoleKey, "operator")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("operator role receives 403 Forbidden on standalone /hosts/{id}/test", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/hosts/"+host.ID+"/test", nil)
		ctx := context.WithValue(req.Context(), tenancy.UserRoleKey, "operator")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("viewer role receives 403 Forbidden on /hosts/{id}/test", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/hosts/"+host.ID+"/test", nil)
		ctx := context.WithValue(req.Context(), tenancy.UserRoleKey, "viewer")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("unauthenticated request receives 401 Unauthorized on /hosts/{id}/test", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/hosts/"+host.ID+"/test", nil)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}
