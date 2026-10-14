package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
	deploymentUsecase "github.com/datdt/k8sselfhost/internal/usecase/deployment"
)

func TestDeploymentHandler_SecurityAndAccessChecks(t *testing.T) {
	repo := &mockDeploymentRepoForHandler{}
	uc := deploymentUsecase.NewUsecase(repo)
	h := NewDeploymentHandler(uc)
	r := chi.NewRouter()
	r.Route("/deployments", h.RegisterRoutes)

	tenantCtx := tenancy.WithUserRole(context.Background(), "tenant_admin")
	tenantCtx = tenancy.WithTenantID(tenantCtx, "tenant-a")

	operatorCtx := tenancy.WithUserRole(context.Background(), "operator")
	operatorCtx = tenancy.WithTenantID(operatorCtx, "tenant-a")

	adminCtx := tenancy.WithUserRole(context.Background(), "platform_admin")
	adminCtx = tenancy.WithTenantID(adminCtx, "default-tenant")

	t.Run("non-admin rejected on kube-system scale", func(t *testing.T) {
		body := `{"type":"kubernetes","namespace":"kube-system","name":"coredns","replicas":2}`
		req := httptest.NewRequest(http.MethodPost, "/deployments/scale", bytes.NewBufferString(body)).WithContext(tenantCtx)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 on kube-system scale, got %d", rec.Code)
		}
	})

	t.Run("non-admin rejected on cross-tenant restart", func(t *testing.T) {
		body := `{"type":"kubernetes","namespace":"tenant-b","name":"app1"}`
		req := httptest.NewRequest(http.MethodPost, "/deployments/restart", bytes.NewBufferString(body)).WithContext(tenantCtx)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 on cross-tenant restart, got %d", rec.Code)
		}
	})

	t.Run("operator role rejected on delete deployment", func(t *testing.T) {
		body := `{"type":"kubernetes","namespace":"tenant-a","name":"app1"}`
		req := httptest.NewRequest(http.MethodPost, "/deployments/delete", bytes.NewBufferString(body)).WithContext(operatorCtx)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for operator on delete deployment, got %d", rec.Code)
		}
	})

	t.Run("tenant_admin allowed on own namespace delete deployment", func(t *testing.T) {
		body := `{"type":"kubernetes","namespace":"tenant-a","name":"app1"}`
		req := httptest.NewRequest(http.MethodPost, "/deployments/delete", bytes.NewBufferString(body)).WithContext(tenantCtx)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for tenant_admin on own namespace delete, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("platform_admin allowed on any namespace", func(t *testing.T) {
		body := `{"type":"kubernetes","namespace":"kube-system","name":"coredns"}`
		req := httptest.NewRequest(http.MethodPost, "/deployments/restart", bytes.NewBufferString(body)).WithContext(adminCtx)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for platform_admin on kube-system restart, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}
