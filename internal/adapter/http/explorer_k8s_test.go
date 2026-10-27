package http

import (
	"context"
	"encoding/json"
	"fmt"
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

type degradedMockExplorerRepo struct {
	err error
}

func (m *degradedMockExplorerRepo) Search(ctx context.Context, kind, cluster, namespace, query string, limit, offset int) ([]explorer.Resource, int, error) {
	return nil, 0, m.err
}

func (m *degradedMockExplorerRepo) GetByID(ctx context.Context, id string) (*explorer.Resource, error) {
	return nil, m.err
}

func (m *degradedMockExplorerRepo) SyncResource(ctx context.Context, res *explorer.Resource) error {
	return m.err
}

func TestExplorerHandler_K8sDegraded_Unreachable(t *testing.T) {
	degradedErr := fmt.Errorf("listing pods: dial tcp 10.10.10.60:6443: connectex: No connection could be made")
	repo := &degradedMockExplorerRepo{err: degradedErr}

	handler := NewExplorerHandler(repo)
	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/?kind=pod&namespace=default", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK for degraded K8s, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		K8sDegraded    bool                `json:"k8s_degraded"`
		DegradedReason string              `json:"degraded_reason"`
		Data           []explorer.Resource `json:"data"`
		Resources      []explorer.Resource `json:"resources"`
		Total          int                 `json:"total"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse json response: %v", err)
	}

	if !resp.K8sDegraded {
		t.Errorf("expected k8s_degraded true, got false")
	}
	expectedReason := "Kubernetes API server (:6443) is unreachable on registered control-plane nodes. Inspect live host daemons in Deployments or node telemetry in Infrastructure Hosts."
	if resp.DegradedReason != expectedReason {
		t.Errorf("expected degraded_reason %q, got %q", expectedReason, resp.DegradedReason)
	}
	if len(resp.Data) != 0 || len(resp.Resources) != 0 {
		t.Errorf("expected empty data/resources, got %d and %d", len(resp.Data), len(resp.Resources))
	}
	if resp.Total != 0 {
		t.Errorf("expected total 0, got %d", resp.Total)
	}
}

func TestExplorerHandler_K8sHealthy(t *testing.T) {
	repo := &mockExplorerRepo{
		items: []explorer.Resource{
			{ID: "p1", Kind: "Pod", Name: "pod-1", Namespace: "default", Cluster: "c1", Status: "Running"},
		},
	}
	handler := NewExplorerHandler(repo)
	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/?kind=pod&namespace=default", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		K8sDegraded bool                `json:"k8s_degraded"`
		Data        []explorer.Resource `json:"data"`
		Resources   []explorer.Resource `json:"resources"`
		Total       int                 `json:"total"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse json response: %v", err)
	}

	if resp.K8sDegraded {
		t.Errorf("expected k8s_degraded false, got true")
	}
	if len(resp.Data) != 1 || len(resp.Resources) != 1 {
		t.Errorf("expected 1 item in data and resources, got %d and %d", len(resp.Data), len(resp.Resources))
	}
	if resp.Total != 1 {
		t.Errorf("expected total 1, got %d", resp.Total)
	}
}
