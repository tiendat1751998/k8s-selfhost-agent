package http

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/audit"
)

func TestAuditAndTenancy_RBACAlignment(t *testing.T) {
	adminToken, err := middleware.GenerateJWT("admin", "platform_admin", "org-alpha")
	require.NoError(t, err)

	tenantAdminToken, err := middleware.GenerateJWT("tenant-boss", "tenant_admin", "org-alpha")
	require.NoError(t, err)

	operatorToken, err := middleware.GenerateJWT("ops-user", "operator", "org-alpha")
	require.NoError(t, err)

	viewerToken, err := middleware.GenerateJWT("view-user", "viewer", "org-alpha")
	require.NoError(t, err)

	auditRepo := &mockAuditHandlerRepo{
		findings: []audit.AuditFinding{
			{ID: "f1", Category: "security", Severity: "medium", Status: "open"},
		},
	}
	platform := &PlatformHandlers{
		Audit:   NewAuditHandler(auditRepo),
		Tenancy: NewTenancyHandler(nil),
	}

	r := chi.NewRouter()
	r.Use(middleware.JWTAuthMiddleware)
	RegisterPlatformRoutes(r, platform)

	t.Run("Audit GET findings allowed for operator and tenant_admin", func(t *testing.T) {
		for _, token := range []string{tenantAdminToken, operatorToken, adminToken} {
			req := httptest.NewRequest(http.MethodGet, "/audit/findings", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusOK, rec.Code, "expected 200 OK for token: %s", rec.Body.String())
		}
	})

	t.Run("Audit GET logs allowed for operator and tenant_admin", func(t *testing.T) {
		for _, token := range []string{tenantAdminToken, operatorToken, adminToken} {
			req := httptest.NewRequest(http.MethodGet, "/audit/logs", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusOK, rec.Code)
		}
	})

	t.Run("Audit POST run rejected for tenant_admin, operator, and viewer", func(t *testing.T) {
		for _, token := range []string{tenantAdminToken, operatorToken, viewerToken} {
			req := httptest.NewRequest(http.MethodPost, "/audit/run", bytes.NewReader([]byte("{}")))
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusForbidden, rec.Code)
		}
	})

	t.Run("Tenancy cross-tenant quota update rejected for tenant_admin with 403", func(t *testing.T) {
		// tenant_admin belonging to org-alpha attempts to modify quota for org-beta
		req := httptest.NewRequest(http.MethodPut, "/tenancy/organizations/org-beta/quota", bytes.NewReader([]byte(`{"cpu":"10"}`)))
		req.Header.Set("Authorization", "Bearer "+tenantAdminToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code, "expected 403 Forbidden on cross-tenant quota update")
	})

	t.Run("Tenancy own-tenant quota update allowed for tenant_admin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/tenancy/organizations/org-alpha/quota", bytes.NewReader([]byte(`{"cpu":"10"}`)))
		req.Header.Set("Authorization", "Bearer "+tenantAdminToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "expected 200 OK on own tenant quota update: %s", rec.Body.String())
	})

	t.Run("Tenancy create organization rejected for tenant_admin with 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/tenancy/organizations", bytes.NewReader([]byte(`{"name":"New Corp"}`)))
		req.Header.Set("Authorization", "Bearer "+tenantAdminToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code, "expected 403 Forbidden for tenant_admin creating org")
	})

	t.Run("Tenancy delete organization rejected for tenant_admin with 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/tenancy/organizations/org-alpha", nil)
		req.Header.Set("Authorization", "Bearer "+tenantAdminToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code, "expected 403 Forbidden for tenant_admin deleting org")
	})

	t.Run("Tenancy global RBAC update rejected for tenant_admin with 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/tenancy/rbac", bytes.NewReader([]byte(`{"matrix":{}}`)))
		req.Header.Set("Authorization", "Bearer "+tenantAdminToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code, "expected 403 Forbidden for tenant_admin updating global RBAC")
	})
}
