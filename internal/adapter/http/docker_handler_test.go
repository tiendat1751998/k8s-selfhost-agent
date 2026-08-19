package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	domain "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

type testDockerRepo struct {
	tokensErr     error
	nodeDetailErr error
	swarmInfoErr  error
}

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

func (r *testDockerRepo) GetSwarmJoinTokens(ctx context.Context) (*domain.SwarmTokens, error) {
	if r.tokensErr != nil {
		return nil, r.tokensErr
	}
	return &domain.SwarmTokens{
		WorkerToken:  "SWMTKN-1-worker-token",
		ManagerToken: "SWMTKN-1-manager-token",
		ManagerAddr:  "10.10.10.133:2377",
	}, nil
}

func (r *testDockerRepo) DrainNode(ctx context.Context, nodeID string) error {
	return nil
}

func (r *testDockerRepo) ActivateNode(ctx context.Context, nodeID string) error {
	return nil
}

func (r *testDockerRepo) RemoveNode(ctx context.Context, nodeID string, force bool) error {
	return nil
}

func (r *testDockerRepo) GetNodeDetails(ctx context.Context, nodeID string) (*domain.NodeDetails, error) {
	if r.nodeDetailErr != nil {
		return nil, r.nodeDetailErr
	}
	if nodeID == "non-existent" {
		return nil, nil
	}
	return &domain.NodeDetails{
		ID:            nodeID,
		Hostname:      "node-1",
		Role:          "manager",
		Availability:  "active",
		Status:        "ready",
		EngineVersion: "27.1.2",
		OS:            "linux",
		Architecture:  "x86_64",
		CPUs:          4000000000,
		Memory:        8589934592,
		IP:            "10.10.10.133",
		Labels:        map[string]string{"env": "prod"},
		JoinedAt:      time.Now(),
	}, nil
}

func (r *testDockerRepo) GetSwarmInfo(ctx context.Context) (*domain.SwarmInfo, error) {
	if r.swarmInfoErr != nil {
		return nil, r.swarmInfoErr
	}
	return &domain.SwarmInfo{
		ID:           "swarm-12345",
		NodeCount:    3,
		ManagerCount: 1,
		WorkerCount:  2,
		CreatedAt:    time.Now(),
		IsManager:    true,
	}, nil
}

type mockComputeHostRepo struct {
	hosts map[string]domain.ComputeHost
}

func newMockComputeHostRepo() *mockComputeHostRepo {
	return &mockComputeHostRepo{
		hosts: make(map[string]domain.ComputeHost),
	}
}

func (m *mockComputeHostRepo) Create(ctx context.Context, host *domain.ComputeHost) error {
	if host.ID == "" {
		host.ID = "host-" + host.Name
	}
	m.hosts[host.ID] = *host
	return nil
}

func (m *mockComputeHostRepo) GetByID(ctx context.Context, id string) (*domain.ComputeHost, error) {
	h, ok := m.hosts[id]
	if !ok {
		return nil, nil
	}
	return &h, nil
}

func (m *mockComputeHostRepo) List(ctx context.Context, tenantID string) ([]domain.ComputeHost, error) {
	var result []domain.ComputeHost
	for _, h := range m.hosts {
		result = append(result, h)
	}
	return result, nil
}

func (m *mockComputeHostRepo) Update(ctx context.Context, host *domain.ComputeHost) error {
	if _, ok := m.hosts[host.ID]; !ok {
		return fmt.Errorf("host not found")
	}
	m.hosts[host.ID] = *host
	return nil
}

func (m *mockComputeHostRepo) Delete(ctx context.Context, id string) error {
	if _, ok := m.hosts[id]; !ok {
		return fmt.Errorf("host not found")
	}
	delete(m.hosts, id)
	return nil
}

func (m *mockComputeHostRepo) UpdateStatus(ctx context.Context, id string, status string, lastHealthCheck time.Time) error {
	h, ok := m.hosts[id]
	if !ok {
		return fmt.Errorf("host not found")
	}
	h.Status = status
	h.LastHealthCheck = &lastHealthCheck
	m.hosts[id] = h
	return nil
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

func TestDockerHandler_GetSwarmInfo(t *testing.T) {
	repo := &testDockerRepo{}
	handler := NewDockerHandler(repo)

	r := chi.NewRouter()
	r.Route("/docker", handler.RegisterRoutes)

	req := httptest.NewRequest(http.MethodGet, "/docker/swarm", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var info domain.SwarmInfo
	if err := json.NewDecoder(w.Body).Decode(&info); err != nil {
		t.Fatalf("failed to decode swarm info: %v", err)
	}
	if info.ID != "swarm-12345" || info.NodeCount != 3 || info.ManagerCount != 1 || info.WorkerCount != 2 || !info.IsManager {
		t.Errorf("unexpected swarm info: %+v", info)
	}
}

type testPasswordVerifier struct {
	validPassword string
}

func (v *testPasswordVerifier) VerifyPassword(ctx context.Context, userID, password string) error {
	if password == v.validPassword {
		return nil
	}
	return fmt.Errorf("invalid password")
}

func TestDockerHandler_GetSwarmTokens(t *testing.T) {
	repo := &testDockerRepo{}
	verifier := &testPasswordVerifier{
		validPassword: "secret-admin-password",
	}
	handler := NewDockerHandler(repo, verifier)

	r := chi.NewRouter()
	r.Route("/docker", handler.RegisterRoutes)

	t.Run("Forbidden for viewer", func(t *testing.T) {
		body, _ := json.Marshal(TokenViewRequest{Password: "secret-admin-password"})
		req := httptest.NewRequest(http.MethodPost, "/docker/swarm/tokens", bytes.NewReader(body))
		ctx := context.WithValue(req.Context(), tenancy.UserRoleKey, "viewer")
		ctx = context.WithValue(ctx, tenancy.UserIDKey, "usr-admin-1")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req.WithContext(ctx))

		if w.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", w.Code)
		}
	})

	t.Run("Forbidden for operator", func(t *testing.T) {
		body, _ := json.Marshal(TokenViewRequest{Password: "secret-admin-password"})
		req := httptest.NewRequest(http.MethodPost, "/docker/swarm/tokens", bytes.NewReader(body))
		ctx := context.WithValue(req.Context(), tenancy.UserRoleKey, "operator")
		ctx = context.WithValue(ctx, tenancy.UserIDKey, "usr-admin-1")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req.WithContext(ctx))

		if w.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", w.Code)
		}
	})

	t.Run("Forbidden for tenant_admin", func(t *testing.T) {
		body, _ := json.Marshal(TokenViewRequest{Password: "secret-admin-password"})
		req := httptest.NewRequest(http.MethodPost, "/docker/swarm/tokens", bytes.NewReader(body))
		ctx := context.WithValue(req.Context(), tenancy.UserRoleKey, "tenant_admin")
		ctx = context.WithValue(ctx, tenancy.UserIDKey, "usr-admin-1")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req.WithContext(ctx))

		if w.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", w.Code)
		}
	})

	t.Run("Bad request for empty password", func(t *testing.T) {
		body, _ := json.Marshal(TokenViewRequest{Password: ""})
		req := httptest.NewRequest(http.MethodPost, "/docker/swarm/tokens", bytes.NewReader(body))
		ctx := context.WithValue(req.Context(), tenancy.UserRoleKey, "platform_admin")
		ctx = context.WithValue(ctx, tenancy.UserIDKey, "usr-admin-1")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req.WithContext(ctx))

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("Forbidden for wrong password", func(t *testing.T) {
		body, _ := json.Marshal(TokenViewRequest{Password: "wrong-password"})
		req := httptest.NewRequest(http.MethodPost, "/docker/swarm/tokens", bytes.NewReader(body))
		ctx := context.WithValue(req.Context(), tenancy.UserRoleKey, "platform_admin")
		ctx = context.WithValue(ctx, tenancy.UserIDKey, "usr-admin-1")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req.WithContext(ctx))

		if w.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", w.Code)
		}
	})

	t.Run("Allowed for platform_admin with valid password and masked response", func(t *testing.T) {
		// Use fresh handler to reset rate limiter
		handlerFresh := NewDockerHandler(repo, verifier)
		rFresh := chi.NewRouter()
		rFresh.Route("/docker", handlerFresh.RegisterRoutes)

		body, _ := json.Marshal(TokenViewRequest{Password: "secret-admin-password"})
		req := httptest.NewRequest(http.MethodPost, "/docker/swarm/tokens", bytes.NewReader(body))
		ctx := context.WithValue(req.Context(), tenancy.UserRoleKey, "platform_admin")
		ctx = context.WithValue(ctx, tenancy.UserIDKey, "usr-admin-1")
		w := httptest.NewRecorder()
		rFresh.ServeHTTP(w, req.WithContext(ctx))

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d (body: %s)", w.Code, w.Body.String())
		}

		var resp SwarmTokensResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode swarm tokens: %v", err)
		}
		if resp.WorkerToken != "SWMTKN-1-worker-token" || resp.ManagerToken != "SWMTKN-1-manager-token" || resp.ManagerAddr != "10.10.10.133:2377" {
			t.Errorf("unexpected full tokens: %+v", resp)
		}
		if resp.WorkerTokenMasked != "SWMTKN-1-***...er-token" || resp.ManagerTokenMasked != "SWMTKN-1-***...er-token" {
			t.Errorf("unexpected masked tokens: %+v", resp)
		}
		if resp.ExpiresInSeconds != 60 {
			t.Errorf("expected ExpiresInSeconds 60, got %d", resp.ExpiresInSeconds)
		}
	})

	t.Run("Rate limited after 3 views in window", func(t *testing.T) {
		limiter := NewTokenViewLimiter(3, 15*time.Minute)
		handlerWithLimiter := NewDockerHandler(repo, verifier, limiter)
		rLimiter := chi.NewRouter()
		rLimiter.Route("/docker", handlerWithLimiter.RegisterRoutes)

		for i := 1; i <= 3; i++ {
			body, _ := json.Marshal(TokenViewRequest{Password: "secret-admin-password"})
			req := httptest.NewRequest(http.MethodPost, "/docker/swarm/tokens", bytes.NewReader(body))
			ctx := context.WithValue(req.Context(), tenancy.UserRoleKey, "platform_admin")
			ctx = context.WithValue(ctx, tenancy.UserIDKey, "usr-rate-test")
			w := httptest.NewRecorder()
			rLimiter.ServeHTTP(w, req.WithContext(ctx))

			if w.Code != http.StatusOK {
				t.Fatalf("request %d: expected status 200, got %d", i, w.Code)
			}
		}

		// 4th request should be rate-limited (429)
		body, _ := json.Marshal(TokenViewRequest{Password: "secret-admin-password"})
		req := httptest.NewRequest(http.MethodPost, "/docker/swarm/tokens", bytes.NewReader(body))
		ctx := context.WithValue(req.Context(), tenancy.UserRoleKey, "platform_admin")
		ctx = context.WithValue(ctx, tenancy.UserIDKey, "usr-rate-test")
		w := httptest.NewRecorder()
		rLimiter.ServeHTTP(w, req.WithContext(ctx))

		if w.Code != http.StatusTooManyRequests {
			t.Fatalf("4th request: expected status 429, got %d (body: %s)", w.Code, w.Body.String())
		}
		if w.Header().Get("Retry-After") == "" {
			t.Errorf("expected Retry-After header to be present")
		}
	})
}

func TestDockerHandler_NodeDetails(t *testing.T) {
	repo := &testDockerRepo{}
	handler := NewDockerHandler(repo)

	r := chi.NewRouter()
	r.Route("/docker", handler.RegisterRoutes)

	t.Run("Found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/docker/nodes/node-123", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var details domain.NodeDetails
		if err := json.NewDecoder(w.Body).Decode(&details); err != nil {
			t.Fatalf("failed to decode node details: %v", err)
		}
		if details.ID != "node-123" || details.Hostname != "node-1" || details.Role != "manager" {
			t.Errorf("unexpected node details: %+v", details)
		}
	})

	t.Run("Not Found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/docker/nodes/non-existent", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})
}

func TestDockerHandler_NodeOperations(t *testing.T) {
	repo := &testDockerRepo{}
	handler := NewDockerHandler(repo)

	r := chi.NewRouter()
	r.Route("/docker", handler.RegisterRoutes)

	t.Run("Drain Node", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/docker/nodes/n1/drain", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})

	t.Run("Activate Node", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/docker/nodes/n1/activate", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})

	t.Run("Remove Node", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/docker/nodes/n1?force=true", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})
}

func TestDockerHandler_ComputeHostsCRUD(t *testing.T) {
	repo := &testDockerRepo{}
	hostRepo := newMockComputeHostRepo()
	handler := NewDockerHandler(repo, hostRepo)

	r := chi.NewRouter()
	r.Route("/docker", handler.RegisterRoutes)

	// 1. Create Host
	createBody := `{
		"name": "remote-docker-1",
		"host_type": "docker",
		"endpoint": "tcp://10.10.10.134:2375",
		"tls_enabled": false,
		"labels": {"region": "us-east"}
	}`
	req := httptest.NewRequest(http.MethodPost, "/docker/hosts", bytes.NewBufferString(createBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var created domain.ComputeHost
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("failed to decode created host: %v", err)
	}
	if created.Name != "remote-docker-1" || created.ID == "" {
		t.Errorf("unexpected created host: %+v", created)
	}

	// 2. List Hosts
	req = httptest.NewRequest(http.MethodGet, "/docker/hosts", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var listResp struct {
		Data []domain.ComputeHost `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&listResp); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}
	if len(listResp.Data) != 1 || listResp.Data[0].Name != "remote-docker-1" {
		t.Errorf("unexpected list response: %+v", listResp.Data)
	}

	// 3. Update Host
	updateBody := `{"name": "remote-docker-updated"}`
	req = httptest.NewRequest(http.MethodPut, "/docker/hosts/"+created.ID, bytes.NewBufferString(updateBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// 4. Delete Host
	req = httptest.NewRequest(http.MethodDelete, "/docker/hosts/"+created.ID, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	// 5. Test Host Not Found after deletion
	req = httptest.NewRequest(http.MethodPost, "/docker/hosts/"+created.ID+"/test", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for deleted host test, got %d", w.Code)
	}
}
