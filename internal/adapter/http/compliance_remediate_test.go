package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	"github.com/datdt/k8sselfhost/internal/domain/compliance"
)

func setupComplianceRemediateRouter(repo compliance.Repository) (*adapthttp.ComplianceHandler, *chi.Mux) {
	handler := adapthttp.NewComplianceHandler(repo, adapthttp.WithBootScan(false))
	r := chi.NewRouter()
	handler.RegisterRoutes(r)
	return handler, r
}

func TestComplianceHandler_RemediateViolation_NotFound(t *testing.T) {
	repo := &mockComplianceRepo{}
	_, r := setupComplianceRemediateRouter(repo)

	req := httptest.NewRequest(http.MethodPost, "/violations/99999999-9999-9999-9999-999999999999/remediate", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestComplianceHandler_RemediateViolation_ManualInfraKubelet(t *testing.T) {
	violID := "11111111-1111-1111-1111-111111111111"
	repo := &mockComplianceRepo{
		violations: []compliance.Violation{
			{
				ID:       violID,
				Policy:   "CIS 4.2.1 Kubelet API LAN Exposure",
				Resource: "kubelet",
				Message:  "Kubelet HTTPS API :10250 exposed on LAN interface 10.10.10.133",
				Resolved: false,
			},
		},
	}
	_, r := setupComplianceRemediateRouter(repo)

	req := httptest.NewRequest(http.MethodPost, "/violations/"+violID+"/remediate", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, violID, resp["violation_id"])
	assert.Equal(t, "manual_action_required", resp["status"])
	assert.False(t, resp["automated"].(bool))
	assert.True(t, resp["success"].(bool))
	assert.NotEmpty(t, resp["commands"])

	// Invariant: underlying network/TLS violation MUST NOT be marked resolved blindly!
	v, _ := repo.GetViolation(context.Background(), violID)
	assert.False(t, v.Resolved)
}

func TestComplianceHandler_RemediateViolation_ManualInfraRedis(t *testing.T) {
	violID := "22222222-2222-2222-2222-222222222222"
	repo := &mockComplianceRepo{
		violations: []compliance.Violation{
			{
				ID:       violID,
				Policy:   "NSA-CISA 2.4 Unprotected Database & Cache Ports",
				Resource: "redis",
				Message:  "Redis service port 6379 exposed on LAN interface 10.10.10.133",
				Resolved: false,
			},
		},
	}
	_, r := setupComplianceRemediateRouter(repo)

	req := httptest.NewRequest(http.MethodPost, "/violations/"+violID+"/remediate", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, violID, resp["violation_id"])
	assert.Equal(t, "manual_action_required", resp["status"])
	assert.False(t, resp["automated"].(bool))
	assert.Contains(t, resp["summary"].(string), "Redis")
	commands := resp["commands"].([]interface{})
	assert.NotEmpty(t, commands)
}

func TestComplianceHandler_RemediateViolation_ManualInfraPostgresTLS(t *testing.T) {
	violID := "33333333-3333-3333-3333-333333333333"
	repo := &mockComplianceRepo{
		violations: []compliance.Violation{
			{
				ID:       violID,
				Policy:   "NSA-CISA 3.1 Unencrypted Database Transport",
				Resource: "postgres",
				Message:  "PostgreSQL :5432 operating without TLS enforcement (sslmode=disable)",
				Resolved: false,
			},
		},
	}
	_, r := setupComplianceRemediateRouter(repo)

	req := httptest.NewRequest(http.MethodPost, "/violations/"+violID+"/remediate", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, violID, resp["violation_id"])
	assert.Equal(t, "manual_action_required", resp["status"])
	assert.False(t, resp["automated"].(bool))
	assert.Contains(t, resp["summary"].(string), "TLS")
}

func TestComplianceHandler_RemediateViolation_AutomatedZombies(t *testing.T) {
	reapDispatched := false
	agentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/remediate" {
			reapDispatched = true
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			return
		}
		http.NotFound(w, r)
	}))
	defer agentServer.Close()

	violID := "44444444-4444-4444-4444-444444444444"
	resolved := false
	repo := &mockComplianceRepo{
		violations: []compliance.Violation{
			{
				ID:       violID,
				Policy:   "CIS Linux Process Integrity",
				Resource: "process-table",
				Message:  "1 zombie process(es) detected on k8snode1",
				Resolved: false,
			},
		},
	}

	handler := adapthttp.NewComplianceHandler(
		repo,
		adapthttp.WithBootScan(false),
		adapthttp.WithComplianceRemediatorStaticHosts([]adapthttp.HostTarget{
			{Name: "k8snode1", Endpoint: agentServer.URL, Status: "ready"},
		}),
	)
	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/violations/"+violID+"/remediate", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, violID, resp["violation_id"])
	assert.Equal(t, "remediated", resp["status"])
	assert.True(t, resp["automated"].(bool))
	assert.True(t, reapDispatched)
	_ = resolved
}

func TestComplianceHandler_ResolveViolation_DirectEndpoint(t *testing.T) {
	violID := "55555555-5555-5555-5555-555555555555"
	repo := &mockComplianceRepo{
		violations: []compliance.Violation{
			{
				ID:       violID,
				Policy:   "CIS 4.2.1 Kubelet API LAN Exposure",
				Resource: "kubelet",
				Message:  "Kubelet HTTPS API :10250 exposed on LAN",
				Resolved: false,
			},
		},
	}
	_, r := setupComplianceRemediateRouter(repo)

	req := httptest.NewRequest(http.MethodPost, "/violations/"+violID+"/resolve", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, violID, resp["violation_id"])
	assert.Equal(t, "resolved", resp["status"])
	assert.True(t, resp["resolved"].(bool))

	v, _ := repo.GetViolation(context.Background(), violID)
	assert.True(t, v.Resolved)
}

func TestComplianceHandler_RemediateViolation_WithConfirmParam(t *testing.T) {
	violID := "66666666-6666-6666-6666-666666666666"
	repo := &mockComplianceRepo{
		violations: []compliance.Violation{
			{
				ID:       violID,
				Policy:   "Redis unprotected LAN port",
				Resource: "redis",
				Message:  "Redis listening on 6379",
				Resolved: false,
			},
		},
	}
	_, r := setupComplianceRemediateRouter(repo)

	req := httptest.NewRequest(http.MethodPost, "/violations/"+violID+"/remediate?confirm=true", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "remediated", resp["status"])
	v, _ := repo.GetViolation(context.Background(), violID)
	assert.True(t, v.Resolved)
}

