package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

func TestRequireTenantParam(t *testing.T) {
	setupRouter := func(guard func(http.Handler) http.Handler) *chi.Mux {
		r := chi.NewRouter()
		r.With(guard).Get("/tenants/{tenantId}/alerts", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})
		r.With(guard).Get("/orgs/{id}/details", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})
		return r
	}

	guard := RequireTenantParam("tenantId", "id")
	router := setupRouter(guard)

	t.Run("Platform admin bypasses cross-tenant checks", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-bravo/alerts", nil)
		ctx := tenancy.WithUserRole(req.Context(), "platform_admin")
		ctx = tenancy.WithTenantID(ctx, "tenant-alpha")
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for platform_admin, got %d", rec.Code)
		}
	})

	t.Run("Non-admin missing tenant context returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-bravo/alerts", nil)
		ctx := tenancy.WithUserRole(req.Context(), "tenant_admin")
		// No tenant ID set
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d", rec.Code)
		}
		var resp map[string]string
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["error"] != "access denied: missing tenant context" {
			t.Errorf("unexpected error: %v", resp["error"])
		}
	})

	t.Run("Non-admin matching tenant succeeds (case-insensitive)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/tenants/TENANT-ALPHA/alerts", nil)
		ctx := tenancy.WithUserRole(req.Context(), "operator")
		ctx = tenancy.WithTenantID(ctx, "tenant-alpha")
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for matching tenant, got %d", rec.Code)
		}
	})

	t.Run("Non-admin cross-tenant returns 403 prohibited", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-charlie/alerts", nil)
		ctx := tenancy.WithUserRole(req.Context(), "tenant_admin")
		ctx = tenancy.WithTenantID(ctx, "tenant-alpha")
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden on cross-tenant access, got %d", rec.Code)
		}
		var resp map[string]string
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["error"] != "access denied: cross-tenant access prohibited" {
			t.Errorf("unexpected error: %v", resp["error"])
		}
	})

	t.Run("Non-admin matching second param name succeeds", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/orgs/tenant-alpha/details", nil)
		ctx := tenancy.WithUserRole(req.Context(), "tenant_admin")
		ctx = tenancy.WithTenantID(ctx, "tenant-alpha")
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for matching 'id' param, got %d", rec.Code)
		}
	})

	t.Run("Non-admin whitespace tenant param fails closed 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/tenants/%20/alerts", nil)
		ctx := tenancy.WithUserRole(req.Context(), "tenant_admin")
		ctx = tenancy.WithTenantID(ctx, "tenant-alpha")
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for whitespace param, got %d", rec.Code)
		}
		var resp map[string]string
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["error"] != "access denied: invalid empty tenant parameter" {
			t.Errorf("unexpected error for whitespace param: %v", resp["error"])
		}
	})

	t.Run("Non-admin missing required tenant param on route fails closed 403", func(t *testing.T) {
		rMissing := chi.NewRouter()
		rMissing.With(guard).Get("/no-params/alerts", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/no-params/alerts", nil)
		ctx := tenancy.WithUserRole(req.Context(), "tenant_admin")
		ctx = tenancy.WithTenantID(ctx, "tenant-alpha")
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		rMissing.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for missing route param, got %d", rec.Code)
		}
		var resp map[string]string
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["error"] != "access denied: missing required tenant parameter" {
			t.Errorf("unexpected error for missing route param: %v", resp["error"])
		}
	})
}
