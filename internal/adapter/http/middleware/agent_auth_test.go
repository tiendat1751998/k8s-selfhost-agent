package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

func TestMatchAgentAuthToken_ValidTokenOnRegisterAndIngest(t *testing.T) {
	origToken := os.Getenv("AGENT_AUTH_TOKEN")
	defer os.Setenv("AGENT_AUTH_TOKEN", origToken)

	testSecret := "super-secure-agent-token-987"
	os.Setenv("AGENT_AUTH_TOKEN", testSecret)

	cases := []struct {
		name       string
		method     string
		path       string
		authHeader string
		agentToken string
		want       bool
	}{
		{
			name:       "Register with Bearer",
			method:     http.MethodPost,
			path:       "/agents/register",
			authHeader: "Bearer " + testSecret,
			want:       true,
		},
		{
			name:       "Register with api prefix and Bearer",
			method:     http.MethodPost,
			path:       "/api/v1/agents/register",
			authHeader: "Bearer " + testSecret,
			want:       true,
		},
		{
			name:       "Register with X-Agent-Token",
			method:     http.MethodPost,
			path:       "/agents/register",
			agentToken: testSecret,
			want:       true,
		},
		{
			name:       "Logs Ingest with Bearer",
			method:     http.MethodPost,
			path:       "/api/v1/logs/ingest",
			authHeader: "Bearer " + testSecret,
			want:       true,
		},
		{
			name:       "Logs Ingest with X-Agent-Token",
			method:     http.MethodPost,
			path:       "/api/v1/logs/ingest",
			agentToken: testSecret,
			want:       true,
		},
		{
			name:       "Short Logs Ingest path",
			method:     http.MethodPost,
			path:       "/logs/ingest",
			agentToken: testSecret,
			want:       true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			if tc.agentToken != "" {
				req.Header.Set("X-Agent-Token", tc.agentToken)
			}
			got := matchAgentAuthToken(req)
			if got != tc.want {
				t.Fatalf("matchAgentAuthToken(%s %s) = %v; want %v", tc.method, tc.path, got, tc.want)
			}
		})
	}
}

func TestMatchAgentAuthToken_InvalidToken(t *testing.T) {
	origToken := os.Getenv("AGENT_AUTH_TOKEN")
	defer os.Setenv("AGENT_AUTH_TOKEN", origToken)

	testSecret := "super-secure-agent-token-987"
	os.Setenv("AGENT_AUTH_TOKEN", testSecret)

	cases := []struct {
		name       string
		path       string
		authHeader string
		agentToken string
	}{
		{
			name:       "Bad Bearer on Ingest",
			path:       "/api/v1/logs/ingest",
			authHeader: "Bearer wrong-token",
		},
		{
			name:       "Bad X-Agent-Token on Register",
			path:       "/agents/register",
			agentToken: "wrong-token",
		},
		{
			name: "No token on Ingest",
			path: "/api/v1/logs/ingest",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			if tc.agentToken != "" {
				req.Header.Set("X-Agent-Token", tc.agentToken)
			}
			if matchAgentAuthToken(req) {
				t.Fatalf("expected matchAgentAuthToken to return false for invalid token on %s", tc.path)
			}
		})
	}
}

func TestMatchAgentAuthToken_UnauthorizedRoutes(t *testing.T) {
	origToken := os.Getenv("AGENT_AUTH_TOKEN")
	defer os.Setenv("AGENT_AUTH_TOKEN", origToken)

	testSecret := "super-secure-agent-token-987"
	os.Setenv("AGENT_AUTH_TOKEN", testSecret)

	cases := []struct {
		name   string
		method string
		path   string
	}{
		{
			name:   "GET search with agent token",
			method: http.MethodGet,
			path:   "/api/v1/logs/search",
		},
		{
			name:   "POST deployments with agent token",
			method: http.MethodPost,
			path:   "/api/v1/deployments",
		},
		{
			name:   "GET ingest (wrong method)",
			method: http.MethodGet,
			path:   "/api/v1/logs/ingest",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("X-Agent-Token", testSecret)
			if matchAgentAuthToken(req) {
				t.Fatalf("matchAgentAuthToken returned true for unauthorized route %s %s", tc.method, tc.path)
			}
		})
	}
}

func TestJWTAuthMiddleware_AgentToken_Integration(t *testing.T) {
	origToken := os.Getenv("AGENT_AUTH_TOKEN")
	defer os.Setenv("AGENT_AUTH_TOKEN", origToken)

	testSecret := "super-secure-agent-token-987"
	os.Setenv("AGENT_AUTH_TOKEN", testSecret)

	var recordedUser, recordedRole, recordedTenant string
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recordedUser = tenancy.UserIDFromContext(r.Context())
		recordedRole = tenancy.UserRoleFromContext(r.Context())
		recordedTenant = tenancy.TenantIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	handler := JWTAuthMiddleware(dummyHandler)

	// 1. Valid X-Agent-Token on /api/v1/logs/ingest -> 200
	req := httptest.NewRequest(http.MethodPost, "/api/v1/logs/ingest", nil)
	req.Header.Set("X-Agent-Token", testSecret)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid agent token on ingest, got %d", rec.Code)
	}
	if recordedUser != "edge-agent" || recordedRole != "platform_admin" || recordedTenant != "default-tenant" {
		t.Errorf("unexpected identity context: user=%q, role=%q, tenant=%q", recordedUser, recordedRole, recordedTenant)
	}

	// 2. Invalid X-Agent-Token on /api/v1/logs/ingest -> 401
	badReq := httptest.NewRequest(http.MethodPost, "/api/v1/logs/ingest", nil)
	badReq.Header.Set("X-Agent-Token", "invalid-token")
	badRec := httptest.NewRecorder()
	handler.ServeHTTP(badRec, badReq)

	if badRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid agent token on ingest, got %d", badRec.Code)
	}

	// 3. Valid X-Agent-Token on unauthorized route /api/v1/deployments -> 401
	unauthReq := httptest.NewRequest(http.MethodPost, "/api/v1/deployments", nil)
	unauthReq.Header.Set("X-Agent-Token", testSecret)
	unauthRec := httptest.NewRecorder()
	handler.ServeHTTP(unauthRec, unauthReq)

	if unauthRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for agent token on unauthorized route, got %d", unauthRec.Code)
	}
}
