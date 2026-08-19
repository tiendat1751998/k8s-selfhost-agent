package http

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/pkg/health"
)

func TestRouter_RBACEnforcement(t *testing.T) {
	healthHandler := health.NewHandler(5 * time.Second)

	// Create tokens for different roles
	adminToken, err := middleware.GenerateJWT("admin-user", "platform_admin", "default-tenant")
	if err != nil {
		t.Fatalf("failed to generate admin token: %v", err)
	}

	operatorToken, err := middleware.GenerateJWT("operator-user", "operator", "default-tenant")
	if err != nil {
		t.Fatalf("failed to generate operator token: %v", err)
	}

	viewerToken, err := middleware.GenerateJWT("viewer-user", "viewer", "default-tenant")
	if err != nil {
		t.Fatalf("failed to generate viewer token: %v", err)
	}

	// Setup platform handlers with instantiated handler objects
	platform := &PlatformHandlers{
		Settings:    NewSettingsHandler(nil, nil),
		Fleet:       NewFleetHandler(nil, nil, nil),
		Cloud:       NewCloudHandler(nil, nil, nil),
		Tenancy:     NewTenancyHandler(nil),
		Backup:      NewBackupHandler(nil),
		Automation:  NewAutomationHandler(nil),
		Docker:      NewDockerHandler(nil),
		Agents:      NewAgentHandler(nil, nil),
		Catalog:     NewCatalogHandler(nil, nil),
		Deployments: NewDeploymentHandler(nil),
		K8s:         NewK8sResourceHandler(nil, nil),
	}

	router := NewRouterWithWS(healthHandler, nil, platform)

	t.Run("Admin-only routes reject non-platform_admin", func(t *testing.T) {
		adminRoutes := []string{
			"/api/v1/settings",
			"/api/v1/fleet",
			"/api/v1/cloud/accounts",
			"/api/v1/tenancy/organizations",
		}

		for _, route := range adminRoutes {
			// Viewer should be 403 Forbidden
			req := httptest.NewRequest(http.MethodGet, route, nil)
			req.Header.Set("Authorization", "Bearer "+viewerToken)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Errorf("[%s] expected 403 Forbidden for viewer, got %d", route, rec.Code)
			}

			// Operator should be 403 Forbidden
			req = httptest.NewRequest(http.MethodGet, route, nil)
			req.Header.Set("Authorization", "Bearer "+operatorToken)
			rec = httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Errorf("[%s] expected 403 Forbidden for operator, got %d", route, rec.Code)
			}
		}
	})

	t.Run("Admin-only K8s apply rejects non-platform_admin", func(t *testing.T) {
		// Viewer should be 403 Forbidden
		req := httptest.NewRequest(http.MethodPost, "/api/v1/k8s/local/apply", bytes.NewBufferString(`{}`))
		req.Header.Set("Authorization", "Bearer "+viewerToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("[POST /api/v1/k8s/local/apply] expected 403 Forbidden for viewer, got %d", rec.Code)
		}

		// Operator should be 403 Forbidden
		req = httptest.NewRequest(http.MethodPost, "/api/v1/k8s/local/apply", bytes.NewBufferString(`{}`))
		req.Header.Set("Authorization", "Bearer "+operatorToken)
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("[POST /api/v1/k8s/local/apply] expected 403 Forbidden for operator, got %d", rec.Code)
		}
	})

	t.Run("Operator+ mutating routes reject viewer", func(t *testing.T) {
		mutatingEndpoints := []struct {
			method string
			path   string
		}{
			{http.MethodPost, "/api/v1/backup/storages"},
			{http.MethodPost, "/api/v1/automation/rules"},
			{http.MethodPost, "/api/v1/docker/services/s1/scale"},
			{http.MethodPost, "/api/v1/agents/tasks"},
			{http.MethodPost, "/api/v1/catalog/services"},
			{http.MethodPost, "/api/v1/deployments"},
			{http.MethodDelete, "/api/v1/automation/rules/r1"},
			{http.MethodPut, "/api/v1/catalog/services/s1"},
		}

		for _, ep := range mutatingEndpoints {
			req := httptest.NewRequest(ep.method, ep.path, bytes.NewBufferString(`{}`))
			req.Header.Set("Authorization", "Bearer "+viewerToken)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			// Should be 403 Forbidden for viewer
			if rec.Code != http.StatusForbidden {
				t.Errorf("[%s %s] expected 403 Forbidden for viewer, got %d", ep.method, ep.path, rec.Code)
			}
		}
	})

	t.Run("Admin token is accepted on /api/v1", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 OK for admin on root /api/v1, got %d", rec.Code)
		}
	})
}
