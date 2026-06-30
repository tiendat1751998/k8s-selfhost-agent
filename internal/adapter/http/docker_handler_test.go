package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	domain "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
)

type testDockerRepo struct{}

func (r *testDockerRepo) ListContainers(ctx context.Context) ([]domain.Container, error) {
	return []domain.Container{
		{ID: "c1", Name: "test-container", Image: "redis:alpine", Status: "Up 2 hours", State: "running", Created: time.Now()},
	}, nil
}

func (r *testDockerRepo) ListNodes(ctx context.Context) ([]domain.Node, error) {
	return []domain.Node{
		{ID: "n1", Name: "test-node", Role: "manager", Availability: "active", Status: "ready", Version: "27.1.2", UpdatedAt: time.Now()},
	}, nil
}

func (r *testDockerRepo) ListServices(ctx context.Context) ([]domain.Service, error) {
	return []domain.Service{
		{ID: "s1", Name: "test-service", Image: "nginx:latest", Replicas: 2, Ports: []string{"80:80"}, UpdatedAt: time.Now()},
	}, nil
}

func (r *testDockerRepo) ScaleService(ctx context.Context, serviceID string, replicas int) error {
	return nil
}

func (r *testDockerRepo) UpdateNodeAvailability(ctx context.Context, nodeID string, availability string) error {
	return nil
}

func (r *testDockerRepo) ToggleContainer(ctx context.Context, containerID string, action string) error {
	return nil
}

func (r *testDockerRepo) DeleteService(ctx context.Context, serviceID string) error {
	return nil
}

func (r *testDockerRepo) RestartService(ctx context.Context, serviceID string) error {
	return nil
}

func (r *testDockerRepo) CreateService(ctx context.Context, name string, image string, replicas int, port int) error {
	return nil
}

func (r *testDockerRepo) GetLogs(ctx context.Context, targetID string, targetType string) (string, error) {
	return "mock logs", nil
}

func TestDockerHandler_ListContainers(t *testing.T) {
	repo := &testDockerRepo{}
	handler := NewDockerHandler(repo)

	r := chi.NewRouter()
	r.Route("/docker", handler.RegisterRoutes)

	req := httptest.NewRequest(http.MethodGet, "/docker/containers", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		Data []domain.Container `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Data) != 1 || resp.Data[0].Name != "test-container" {
		t.Errorf("unexpected containers response: %+v", resp.Data)
	}
}

func TestDockerHandler_ListNodes(t *testing.T) {
	repo := &testDockerRepo{}
	handler := NewDockerHandler(repo)

	r := chi.NewRouter()
	r.Route("/docker", handler.RegisterRoutes)

	req := httptest.NewRequest(http.MethodGet, "/docker/nodes", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		Data []domain.Node `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Data) != 1 || resp.Data[0].Name != "test-node" {
		t.Errorf("unexpected nodes response: %+v", resp.Data)
	}
}

func TestDockerHandler_ListServices(t *testing.T) {
	repo := &testDockerRepo{}
	handler := NewDockerHandler(repo)

	r := chi.NewRouter()
	r.Route("/docker", handler.RegisterRoutes)

	req := httptest.NewRequest(http.MethodGet, "/docker/services", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		Data []domain.Service `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Data) != 1 || resp.Data[0].Name != "test-service" {
		t.Errorf("unexpected services response: %+v", resp.Data)
	}
}
