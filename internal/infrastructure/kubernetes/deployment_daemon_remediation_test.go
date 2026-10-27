package kubernetes

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	domainDocker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
)

type mockHostRepo struct {
	hosts []domainDocker.ComputeHost
}

func (m *mockHostRepo) ListAll(ctx context.Context) ([]domainDocker.ComputeHost, error) {
	return m.hosts, nil
}

func (m *mockHostRepo) List(ctx context.Context, tenantID string) ([]domainDocker.ComputeHost, error) {
	return m.hosts, nil
}

func (m *mockHostRepo) GetByID(ctx context.Context, id string) (*domainDocker.ComputeHost, error) {
	for _, h := range m.hosts {
		if h.ID == id {
			return &h, nil
		}
	}
	return nil, nil
}

func (m *mockHostRepo) Create(ctx context.Context, host *domainDocker.ComputeHost) error {
	return nil
}

func (m *mockHostRepo) Update(ctx context.Context, host *domainDocker.ComputeHost) error {
	return nil
}

func (m *mockHostRepo) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockHostRepo) UpdateStatus(ctx context.Context, id string, status string, lastHealthCheck time.Time) error {
	return nil
}

func TestDeploymentDaemon_Scale_Returns422(t *testing.T) {
	repo := &deploymentRepo{}
	err := repo.Scale(context.Background(), "daemon", "", "node-1", "kubelet", 3)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "systemd host daemons cannot be scaled or deleted") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestDeploymentDaemon_Delete_Returns422(t *testing.T) {
	repo := &deploymentRepo{}
	err := repo.Delete(context.Background(), "daemon", "", "node-1", "dockerd")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "systemd host daemons cannot be scaled or deleted") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestDeploymentDaemon_Restart_GuardsK8sAgent(t *testing.T) {
	repo := &deploymentRepo{}
	err := repo.Restart(context.Background(), "daemon", "", "node-1", "k8s-agent")
	if err == nil {
		t.Fatal("expected error for k8s-agent restart, got nil")
	}
	if !strings.Contains(err.Error(), "k8s-agent cannot be restarted via HTTP RPC") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestDeploymentDaemon_Restart_GuardsPostgres(t *testing.T) {
	repo := &deploymentRepo{}
	for _, pgName := range []string{"postgres", "postgresql", "PostgreSQL"} {
		err := repo.Restart(context.Background(), "daemon", "", "node-1", pgName)
		if err == nil {
			t.Fatalf("expected error for %s restart, got nil", pgName)
		}
		if !strings.Contains(err.Error(), "postgresql is a protected control-plane database service") {
			t.Fatalf("unexpected error message for %s: %v", pgName, err)
		}
	}
}

func TestDeploymentDaemon_Restart_Success(t *testing.T) {
	var receivedAction, receivedTarget string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/remediate" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var req map[string]string
		_ = json.Unmarshal(body, &req)
		receivedAction = req["action"]
		receivedTarget = req["target"]

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"details":"restarted"}`))
	}))
	defer server.Close()

	hostRepo := &mockHostRepo{
		hosts: []domainDocker.ComputeHost{
			{
				ID:       "host-1",
				Name:     "worker-node",
				Endpoint: server.URL,
			},
		},
	}

	repo := &deploymentRepo{
		computeHostRepo: hostRepo,
	}

	// Test dockerd -> mapped to docker
	err := repo.Restart(context.Background(), "daemon", "", "worker-node", "dockerd")
	if err != nil {
		t.Fatalf("dockerd restart failed: %v", err)
	}
	if receivedAction != "restart_service" || receivedTarget != "docker" {
		t.Fatalf("expected action restart_service target docker, got action=%s target=%s", receivedAction, receivedTarget)
	}

	// Test kubelet
	err = repo.Restart(context.Background(), "daemon", "", "worker-node", "kubelet")
	if err != nil {
		t.Fatalf("kubelet restart failed: %v", err)
	}
	if receivedAction != "restart_service" || receivedTarget != "kubelet" {
		t.Fatalf("expected action restart_service target kubelet, got action=%s target=%s", receivedAction, receivedTarget)
	}
}

func TestDeploymentDaemon_Restart_AgentFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("circuit breaker open"))
	}))
	defer server.Close()

	hostRepo := &mockHostRepo{
		hosts: []domainDocker.ComputeHost{
			{
				ID:       "host-1",
				Name:     "worker-node",
				Endpoint: server.URL,
			},
		},
	}

	repo := &deploymentRepo{
		computeHostRepo: hostRepo,
	}

	err := repo.Restart(context.Background(), "daemon", "", "worker-node", "containerd")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "agent remediation error (HTTP 429)") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
