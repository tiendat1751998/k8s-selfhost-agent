package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/infrastructure/iac"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

func TestIaCHandler_RBAC_RestrictedToPlatformAdminOnly(t *testing.T) {
	tfRunner := iac.NewTerraformRunner("echo")
	ansibleRunner := iac.NewAnsibleRunner("echo")
	handler := NewIaCHandler(tfRunner, ansibleRunner)

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	endpoints := []struct {
		method string
		path   string
		body   string
	}{
		{"GET", "/status", ""},
		{"POST", "/terraform/plan", `{"workingDir":"."}`},
		{"POST", "/terraform/apply", `{"workingDir":"."}`},
		{"POST", "/ansible/playbook", `{"playbookFile":"test.yaml"}`},
	}

	disallowedRoles := []string{"tenant_admin", "operator", "viewer", "developer"}

	for _, role := range disallowedRoles {
		for _, ep := range endpoints {
			t.Run(ep.method+" "+ep.path+" disallowed for role "+role, func(t *testing.T) {
				var req *http.Request
				if ep.body != "" {
					req = httptest.NewRequest(ep.method, ep.path, bytes.NewReader([]byte(ep.body)))
					req.Header.Set("Content-Type", "application/json")
				} else {
					req = httptest.NewRequest(ep.method, ep.path, nil)
				}
				ctx := context.WithValue(req.Context(), tenancy.UserRoleKey, role)
				req = req.WithContext(ctx)

				rec := httptest.NewRecorder()
				r.ServeHTTP(rec, req)

				assert.Equal(t, http.StatusForbidden, rec.Code, "role %s should receive 403 Forbidden on %s", role, ep.path)
			})
		}
	}

	// Unauthenticated request should be rejected (401 Unauthorized or 403 Forbidden)
	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path+" disallowed for unauthenticated", func(t *testing.T) {
			var req *http.Request
			if ep.body != "" {
				req = httptest.NewRequest(ep.method, ep.path, bytes.NewReader([]byte(ep.body)))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(ep.method, ep.path, nil)
			}

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			assert.True(t, rec.Code == http.StatusForbidden || rec.Code == http.StatusUnauthorized,
				"expected 401 or 403 for unauthenticated, got %d", rec.Code)
		})
	}

	// Platform admin is allowed
	t.Run("GET /status allowed for platform_admin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/status", nil)
		ctx := context.WithValue(req.Context(), tenancy.UserRoleKey, "platform_admin")
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
	})
}
