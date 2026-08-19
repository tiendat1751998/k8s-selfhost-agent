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

	t.Run("Pprof endpoints require platform_admin role when enabled", func(t *testing.T) {
		t.Setenv("K8S_ENABLE_PPROF", "true")
		pprofRouter := NewRouterWithWS(healthHandler, nil, platform)

		// 1. Unauthenticated request -> 401 Unauthorized
		reqUnauth := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
		recUnauth := httptest.NewRecorder()
		pprofRouter.ServeHTTP(recUnauth, reqUnauth)
		if recUnauth.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized for unauthenticated pprof request, got %d", recUnauth.Code)
		}

		// 2. Viewer request -> 403 Forbidden
		reqViewer := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
		reqViewer.Header.Set("Authorization", "Bearer "+viewerToken)
		recViewer := httptest.NewRecorder()
		pprofRouter.ServeHTTP(recViewer, reqViewer)
		if recViewer.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for viewer on pprof, got %d", recViewer.Code)
		}

		// 3. Operator request -> 403 Forbidden
		reqOp := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
		reqOp.Header.Set("Authorization", "Bearer "+operatorToken)
		recOp := httptest.NewRecorder()
		pprofRouter.ServeHTTP(recOp, reqOp)
		if recOp.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for operator on pprof, got %d", recOp.Code)
		}

		// 4. Platform admin request -> 200 OK
		reqAdmin := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
		reqAdmin.Header.Set("Authorization", "Bearer "+adminToken)
		recAdmin := httptest.NewRecorder()
		pprofRouter.ServeHTTP(recAdmin, reqAdmin)
		if recAdmin.Code != http.StatusOK {
			t.Errorf("expected 200 OK for platform_admin on pprof, got %d", recAdmin.Code)
		}
	})

	t.Run("Metrics endpoint respects METRICS_TOKEN", func(t *testing.T) {
		// 1. Default (no token configured): open access
		t.Setenv("METRICS_TOKEN", "")
		metricsRouter := NewRouterWithWS(healthHandler, nil, platform)
		reqOpen := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		recOpen := httptest.NewRecorder()
		metricsRouter.ServeHTTP(recOpen, reqOpen)
		if recOpen.Code != http.StatusOK {
			t.Errorf("expected 200 OK for open /metrics, got %d", recOpen.Code)
		}

		// 2. METRICS_TOKEN set: unauthenticated request is 401
		t.Setenv("METRICS_TOKEN", "k8s-metrics-super-secret-token")
		reqNoAuth := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		recNoAuth := httptest.NewRecorder()
		metricsRouter.ServeHTTP(recNoAuth, reqNoAuth)
		if recNoAuth.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized for /metrics without token, got %d", recNoAuth.Code)
		}

		// 3. METRICS_TOKEN set: authenticated request with correct token is 200 OK
		reqAuthed := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		reqAuthed.Header.Set("Authorization", "Bearer k8s-metrics-super-secret-token")
		recAuthed := httptest.NewRecorder()
		metricsRouter.ServeHTTP(recAuthed, reqAuthed)
		if recAuthed.Code != http.StatusOK {
			t.Errorf("expected 200 OK for /metrics with valid bearer token, got %d", recAuthed.Code)
		}
	})
}
