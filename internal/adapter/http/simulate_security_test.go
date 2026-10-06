package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

func TestSimulateIncident_SecurityGating(t *testing.T) {
	incRepo := newMockIncidentRepo()
	reportRepo := newMockReportRepo()
	prRepo := newMockPRRepo()

	h := NewHandler(incRepo, reportRepo, prRepo, nil, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	t.Run("rejected with 403 when caller is tenant_admin", func(t *testing.T) {
		ctx := tenancy.WithUserRole(context.Background(), "tenant_admin")
		req := httptest.NewRequest(http.MethodPost, "/incidents/simulate", bytes.NewReader([]byte("{}"))).WithContext(ctx)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for tenant_admin, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("rejected with 403 when caller is viewer", func(t *testing.T) {
		ctx := tenancy.WithUserRole(context.Background(), "viewer")
		req := httptest.NewRequest(http.MethodPost, "/incidents/simulate", bytes.NewReader([]byte("{}"))).WithContext(ctx)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for viewer, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("rejected with 403 when environment is production via SetEnvironment", func(t *testing.T) {
		prodHandler := NewHandler(incRepo, reportRepo, prRepo, nil, nil)
		prodHandler.SetEnvironment("production")
		prodRouter := chi.NewRouter()
		prodHandler.RegisterRoutes(prodRouter)

		ctx := tenancy.WithUserRole(context.Background(), "platform_admin")
		req := httptest.NewRequest(http.MethodPost, "/incidents/simulate", bytes.NewReader([]byte("{}"))).WithContext(ctx)
		rec := httptest.NewRecorder()
		prodRouter.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden in production, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("rejected with 403 when ENV=production env var is set", func(t *testing.T) {
		_ = os.Setenv("ENV", "production")
		defer os.Unsetenv("ENV")

		ctx := tenancy.WithUserRole(context.Background(), "platform_admin")
		req := httptest.NewRequest(http.MethodPost, "/incidents/simulate", bytes.NewReader([]byte("{}"))).WithContext(ctx)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden when ENV=production, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("allowed with 201 when platform_admin in dev environment", func(t *testing.T) {
		ctx := tenancy.WithUserRole(context.Background(), "platform_admin")
		req := httptest.NewRequest(http.MethodPost, "/incidents/simulate", bytes.NewReader([]byte("{}"))).WithContext(ctx)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created for platform_admin in dev, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}
