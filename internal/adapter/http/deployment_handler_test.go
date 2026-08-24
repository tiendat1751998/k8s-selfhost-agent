package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/deployment"
	deploymentUsecase "github.com/datdt/k8sselfhost/internal/usecase/deployment"
)

type mockDeploymentRepoForHandler struct {
	apps                        []deployment.Application
	lastUpdateTargetType        string
	lastUpdateTargetCluster     string
	lastUpdateNamespace         string
	lastUpdateName              string
	lastUpdateMemoryLimitBytes  int64
	lastUpdateMemoryReservBytes int64
	lastUpdateNanoCPUs          int64
	lastUpdateReplicas          int
}

func (m *mockDeploymentRepoForHandler) List(ctx context.Context) ([]deployment.Application, error) {
	return m.apps, nil
}

func (m *mockDeploymentRepoForHandler) Scale(ctx context.Context, targetType, targetCluster, namespace, name string, replicas int) error {
	return nil
}

func (m *mockDeploymentRepoForHandler) Restart(ctx context.Context, targetType, targetCluster, namespace, name string) error {
	return nil
}

func (m *mockDeploymentRepoForHandler) Delete(ctx context.Context, targetType, targetCluster, namespace, name string) error {
	return nil
}

func (m *mockDeploymentRepoForHandler) Create(ctx context.Context, app deployment.Application) error {
	m.apps = append(m.apps, app)
	return nil
}

func (m *mockDeploymentRepoForHandler) UpdateResources(ctx context.Context, targetType, targetCluster, namespace, name string, memoryLimitBytes, memoryReservBytes, nanoCPUs int64, replicas int) error {
	m.lastUpdateTargetType = targetType
	m.lastUpdateTargetCluster = targetCluster
	m.lastUpdateNamespace = namespace
	m.lastUpdateName = name
	m.lastUpdateMemoryLimitBytes = memoryLimitBytes
	m.lastUpdateMemoryReservBytes = memoryReservBytes
	m.lastUpdateNanoCPUs = nanoCPUs
	m.lastUpdateReplicas = replicas
	return nil
}

func setupDeploymentTestRouter(repo deployment.Repository) http.Handler {
	uc := deploymentUsecase.NewUsecase(repo)
	h := NewDeploymentHandler(uc)
	r := chi.NewRouter()
	r.Route("/deployments", h.RegisterRoutes)
	return r
}

func TestDeploymentHandler_UpdateResources(t *testing.T) {
	repo := &mockDeploymentRepoForHandler{}
	router := setupDeploymentTestRouter(repo)

	t.Run("successful kubernetes update", func(t *testing.T) {
		body := `{
			"type": "kubernetes",
			"cluster": "prod-us-east",
			"namespace": "default",
			"name": "frontend",
			"replicas": 3,
			"memory_limit": "2GiB",
			"memory_reservation": "512MiB",
			"cpu_limit": "2000m",
			"cpu_reservation": "500m"
		}`

		req := httptest.NewRequest(http.MethodPost, "/deployments/resources", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		if repo.lastUpdateName != "frontend" {
			t.Errorf("expected name frontend, got %s", repo.lastUpdateName)
		}
		if repo.lastUpdateReplicas != 3 {
			t.Errorf("expected replicas 3, got %d", repo.lastUpdateReplicas)
		}
		if repo.lastUpdateMemoryLimitBytes != 2*1024*1024*1024 {
			t.Errorf("expected memory limit %d, got %d", 2*1024*1024*1024, repo.lastUpdateMemoryLimitBytes)
		}
		if repo.lastUpdateMemoryReservBytes != 512*1024*1024 {
			t.Errorf("expected memory reservation %d, got %d", 512*1024*1024, repo.lastUpdateMemoryReservBytes)
		}
		if repo.lastUpdateNanoCPUs != 2000000000 {
			t.Errorf("expected nanoCPUs %d, got %d", 2000000000, repo.lastUpdateNanoCPUs)
		}
	})

	t.Run("successful docker swarm update without namespace", func(t *testing.T) {
		body := `{
			"type": "docker",
			"cluster": "swarm-cluster",
			"name": "tiki_redis",
			"replicas": 2,
			"memory_limit": "1GiB",
			"memory_reservation": "256MiB",
			"cpu_limit": "1",
			"cpu_reservation": "200m"
		}`

		req := httptest.NewRequest(http.MethodPost, "/deployments/resources", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		if repo.lastUpdateName != "tiki_redis" {
			t.Errorf("expected name tiki_redis, got %s", repo.lastUpdateName)
		}
	})

	t.Run("validation failure for empty name", func(t *testing.T) {
		body := `{
			"type": "kubernetes",
			"namespace": "default",
			"name": "",
			"replicas": 1
		}`

		req := httptest.NewRequest(http.MethodPost, "/deployments/resources", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", rec.Code)
		}
	})

	t.Run("validation failure for missing namespace on kubernetes", func(t *testing.T) {
		body := `{
			"type": "kubernetes",
			"namespace": "",
			"name": "frontend",
			"replicas": 1
		}`

		req := httptest.NewRequest(http.MethodPost, "/deployments/resources", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", rec.Code)
		}
	})

	t.Run("validation failure for invalid replicas", func(t *testing.T) {
		body := `{
			"type": "kubernetes",
			"namespace": "default",
			"name": "frontend",
			"replicas": 105
		}`

		req := httptest.NewRequest(http.MethodPost, "/deployments/resources", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", rec.Code)
		}
	})
}

func TestDeploymentHandler_OtherRoutes(t *testing.T) {
	repo := &mockDeploymentRepoForHandler{
		apps: []deployment.Application{
			{Name: "app1", Replicas: 2},
		},
	}
	router := setupDeploymentTestRouter(repo)

	// GET /
	req := httptest.NewRequest(http.MethodGet, "/deployments", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for list, got %d", rec.Code)
	}

	// GET /templates
	req = httptest.NewRequest(http.MethodGet, "/deployments/templates", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for templates, got %d", rec.Code)
	}

	// POST /scale
	scaleBody := `{"type":"kubernetes","namespace":"default","name":"app1","replicas":4}`
	req = httptest.NewRequest(http.MethodPost, "/deployments/scale", bytes.NewBufferString(scaleBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for scale, got %d", rec.Code)
	}

	// POST /restart
	restartBody := `{"type":"kubernetes","namespace":"default","name":"app1"}`
	req = httptest.NewRequest(http.MethodPost, "/deployments/restart", bytes.NewBufferString(restartBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for restart, got %d", rec.Code)
	}

	// POST /delete
	delBody := `{"type":"kubernetes","namespace":"default","name":"app1"}`
	req = httptest.NewRequest(http.MethodPost, "/deployments/delete", bytes.NewBufferString(delBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for delete, got %d", rec.Code)
	}

	// POST /
	createBody := `{"name":"app2","image":"nginx:alpine","namespace":"default","replicas":1}`
	req = httptest.NewRequest(http.MethodPost, "/deployments", bytes.NewBufferString(createBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for create, got %d", rec.Code)
	}
}
