package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"

	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
	deploymentUsecase "github.com/datdt/k8sselfhost/internal/usecase/deployment"
)

func TestDeploymentHandler_WorkloadBOLA_Protection(t *testing.T) {
	uc := deploymentUsecase.NewUsecase(nil)
	handler := NewDeploymentHandler(uc)

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	// Workload mutations that attempt global/host docker or empty namespace
	endpoints := []struct {
		name string
		path string
		body string
	}{
		{"Scale docker workload", "/scale", `{"type":"docker","name":"host-daemon","replicas":2}`},
		{"Scale other tenant namespace", "/scale", `{"type":"k8s","name":"host-daemon","namespace":"other-tenant","replicas":2}`},
		{"UpdateResources docker", "/resources", `{"type":"docker","name":"host-daemon","replicas":1}`},
		{"UpdateResources other tenant namespace", "/resources", `{"type":"k8s","name":"host-daemon","namespace":"other-tenant","replicas":1}`},
		{"Restart docker workload", "/restart", `{"type":"docker","name":"host-daemon"}`},
		{"Restart other tenant namespace", "/restart", `{"type":"k8s","name":"host-daemon","namespace":"other-tenant"}`},
		{"Delete docker workload", "/delete", `{"type":"docker","name":"host-daemon"}`},
		{"Delete other tenant namespace", "/delete", `{"type":"k8s","name":"host-daemon","namespace":"other-tenant"}`},
		{"Rollback docker workload", "/rollback", `{"type":"docker","name":"host-daemon"}`},
		{"Pause docker workload", "/pause", `{"type":"docker","name":"host-daemon"}`},
		{"Resume docker workload", "/resume", `{"type":"docker","name":"host-daemon"}`},
	}

	nonAdminRoles := []string{"tenant_admin", "operator", "viewer"}

	for _, tc := range endpoints {
		for _, role := range nonAdminRoles {
			t.Run(tc.name+" forbidden for "+role, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodPost, tc.path, bytes.NewReader([]byte(tc.body)))
				req.Header.Set("Content-Type", "application/json")

				ctx := context.WithValue(req.Context(), tenancy.UserRoleKey, role)
				ctx = context.WithValue(ctx, tenancy.TenantIDKey, "tenant-a")
				req = req.WithContext(ctx)

				rec := httptest.NewRecorder()
				r.ServeHTTP(rec, req)

				assert.Equal(t, http.StatusForbidden, rec.Code,
					"expected 403 Forbidden for non-admin %s on %s with body %s, got %d", role, tc.path, tc.body, rec.Code)
			})
		}
	}
}
