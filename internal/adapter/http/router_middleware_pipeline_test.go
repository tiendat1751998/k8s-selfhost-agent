package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
)

type pipelineMockAuditRepo struct {
	mockAuditHandlerRepo
	actions []string
}

func (m *pipelineMockAuditRepo) RecordAction(ctx context.Context, actor, action, targetType, targetID, targetName, result string, details map[string]interface{}, ipAddress, userAgent string) error {
	m.actions = append(m.actions, action)
	return nil
}

func TestRouter_MiddlewarePipeline_PanicRecoveryWithHeaders(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")
	r := NewRouterWithWS(nil, nil, nil)
	r.Get("/panic-test", func(w http.ResponseWriter, r *http.Request) {
		panic("simulated unhandled exception")
	})

	initialCount := middleware.GetRequestCount()

	req := httptest.NewRequest(http.MethodGet, "/panic-test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	// 1. Status code is 500 JSON
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	// 2. SecurityHeaders preserved
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))

	// 3. CORS headers preserved
	assert.Equal(t, "http://localhost:3000", rec.Header().Get("Access-Control-Allow-Origin"))

	// 4. Request counter incremented
	assert.Greater(t, middleware.GetRequestCount(), initialCount)

	// 5. JSON error payload returned without leaking stack trace
	var body map[string]string
	err := json.Unmarshal(rec.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "internal server error", body["error"])
}

func TestRouter_MiddlewarePipeline_TenantGuardAndMutationAudit(t *testing.T) {
	auditRepo := &pipelineMockAuditRepo{}
	auditHandler := NewAuditHandler(auditRepo)
	msgHandler := NewMessagingHandler(nil)

	platform := &PlatformHandlers{
		Audit:        auditHandler,
		MessagingSRE: msgHandler,
	}

	r := NewRouterWithWS(nil, nil, platform)

	userToken, err := middleware.GenerateJWT("user-1", "tenant_admin", "org-alpha")
	require.NoError(t, err)

	adminToken, err := middleware.GenerateJWT("admin-1", "platform_admin", "org-alpha")
	require.NoError(t, err)

	// 1. Cross-tenant access blocked by RequireTenantParam
	t.Run("Cross-tenant access blocked with 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/messaging/tenants/org-beta/backlog-alerts", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
		var resp map[string]string
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		assert.Equal(t, "access denied: cross-tenant access prohibited", resp["error"])
	})

	// 2. Matching tenant access allowed
	t.Run("Matching tenant access allowed with 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/messaging/tenants/org-alpha/backlog-alerts", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	// 3. Platform admin bypasses tenant boundary
	t.Run("Platform admin allowed cross-tenant access with 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/messaging/tenants/org-beta/backlog-alerts", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	// 4. MutationAudit captures mutating requests in /api/v1
	t.Run("Mutation audit records mutating POST", func(t *testing.T) {
		initialActions := len(auditRepo.actions)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/telemetry", bytes.NewReader([]byte(`{"metric":"cpu"}`)))
		req.Header.Set("Authorization", "Bearer "+userToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusAccepted, rec.Code)
		time.Sleep(50 * time.Millisecond) // Allow audit recording
		assert.Greater(t, len(auditRepo.actions), initialActions)
	})

	// 5. MutationAudit skips GET requests
	t.Run("Mutation audit skips safe GET requests", func(t *testing.T) {
		initialActions := len(auditRepo.actions)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, initialActions, len(auditRepo.actions))
	})
}
