package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/domain/explorer"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

type testExplorerRepo struct{}

func (m *testExplorerRepo) Search(ctx context.Context, kind, cluster, namespace, query string, limit, offset int) ([]explorer.Resource, int, error) {
	return []explorer.Resource{}, 0, nil
}

func (m *testExplorerRepo) GetByID(ctx context.Context, id string) (*explorer.Resource, error) {
	return nil, nil
}

func (m *testExplorerRepo) SyncResource(ctx context.Context, res *explorer.Resource) error {
	return nil
}

func TestExplorerHandler_TenantNamespaceIsolation(t *testing.T) {
	handler := NewExplorerHandler(&testExplorerRepo{})
	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	t.Run("Search forbidden when namespace is empty for non-admin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?kind=Pod", nil)
		ctx := context.WithValue(req.Context(), tenancy.UserRoleKey, "tenant_admin")
		ctx = context.WithValue(ctx, tenancy.TenantIDKey, "tenant-a")
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("Search forbidden when namespace is 'all' for non-admin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?namespace=all", nil)
		ctx := context.WithValue(req.Context(), tenancy.UserRoleKey, "operator")
		ctx = context.WithValue(ctx, tenancy.TenantIDKey, "tenant-a")
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("Search forbidden when namespace belongs to other tenant", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?namespace=tenant-b", nil)
		ctx := context.WithValue(req.Context(), tenancy.UserRoleKey, "tenant_admin")
		ctx = context.WithValue(ctx, tenancy.TenantIDKey, "tenant-a")
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("Search allowed for matching tenant namespace", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?namespace=tenant-a", nil)
		ctx := context.WithValue(req.Context(), tenancy.UserRoleKey, "tenant_admin")
		ctx = context.WithValue(ctx, tenancy.TenantIDKey, "tenant-a")
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("Search allowed across all namespaces for platform_admin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?namespace=all", nil)
		ctx := context.WithValue(req.Context(), tenancy.UserRoleKey, "platform_admin")
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("Sync forbidden for cluster-scoped / empty namespace by non-admin", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"kind":    "ClusterRole",
			"name":    "cr-1",
			"cluster": "k8s-prod",
		})
		req := httptest.NewRequest(http.MethodPost, "/sync", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), tenancy.UserRoleKey, "operator")
		ctx = context.WithValue(ctx, tenancy.TenantIDKey, "tenant-a")
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("Sync forbidden for cross-tenant namespace by non-admin", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"kind":      "Deployment",
			"name":      "deploy-1",
			"cluster":   "k8s-prod",
			"namespace": "tenant-b",
		})
		req := httptest.NewRequest(http.MethodPost, "/sync", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), tenancy.UserRoleKey, "tenant_admin")
		ctx = context.WithValue(ctx, tenancy.TenantIDKey, "tenant-a")
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("Sync allowed for matching tenant namespace", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"kind":      "Deployment",
			"name":      "deploy-1",
			"cluster":   "k8s-prod",
			"namespace": "tenant-a",
		})
		req := httptest.NewRequest(http.MethodPost, "/sync", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), tenancy.UserRoleKey, "tenant_admin")
		ctx = context.WithValue(ctx, tenancy.TenantIDKey, "tenant-a")
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
	})
}
