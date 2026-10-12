package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/explorer"
)

type mockExplorerRepo struct {
	items []explorer.Resource
}

func (m *mockExplorerRepo) Search(ctx context.Context, kind, cluster, namespace, query string, limit, offset int) ([]explorer.Resource, int, error) {
	return m.items, len(m.items), nil
}

func (m *mockExplorerRepo) GetByID(ctx context.Context, id string) (*explorer.Resource, error) {
	if len(m.items) > 0 {
		return &m.items[0], nil
	}
	return nil, nil
}

func (m *mockExplorerRepo) SyncResource(ctx context.Context, res *explorer.Resource) error {
	return nil
}

func TestExplorerHandler_SearchResources_NilReplicasNoPanic(t *testing.T) {
	repo := &mockExplorerRepo{
		items: []explorer.Resource{
			{
				ID:        "uid-nil-rep",
				Kind:      "Deployment",
				Name:      "nil-replicas-app",
				Namespace: "default",
				Cluster:   "dev-cluster",
				Status:    "Ready",
			},
		},
	}

	handler := NewExplorerHandler(repo)
	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/?kind=deployment&namespace=default", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}
}
