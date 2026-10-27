package kubernetes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	domainDocker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
)

type mockComputeHostRepo struct {
	hosts []domainDocker.ComputeHost
}

func (m *mockComputeHostRepo) Create(ctx context.Context, host *domainDocker.ComputeHost) error {
	return nil
}

func (m *mockComputeHostRepo) GetByID(ctx context.Context, id string) (*domainDocker.ComputeHost, error) {
	for _, h := range m.hosts {
		if h.ID == id {
			return &h, nil
		}
	}
	return nil, nil
}

func (m *mockComputeHostRepo) List(ctx context.Context, tenantID string) ([]domainDocker.ComputeHost, error) {
	return m.hosts, nil
}

func (m *mockComputeHostRepo) ListAll(ctx context.Context) ([]domainDocker.ComputeHost, error) {
	return m.hosts, nil
}

func (m *mockComputeHostRepo) Update(ctx context.Context, host *domainDocker.ComputeHost) error {
	return nil
}

func (m *mockComputeHostRepo) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockComputeHostRepo) UpdateStatus(ctx context.Context, id string, status string, lastHealthCheck time.Time) error {
	return nil
}

func TestDeploymentRepo_PurgeFallbackFleetWorkloads(t *testing.T) {
	// When K8s client is nil and no compute host repo is wired, List() MUST NOT return hardcoded fake workloads.
	repo := NewDeploymentRepoWithInterface(nil, nil, nil, nil)
	apps, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) != 0 {
		t.Fatalf("expected 0 apps (zero fake fallbacks), got %d apps: %+v", len(apps), apps)
	}
}

func TestDeploymentRepo_DiscoverFleetDaemons(t *testing.T) {
	onlineServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/metrics":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"hostname": "node-60",
				"detected_services": ["k8s-agent", "postgres", "containerd"],
				"top_processes": [
					{"name": "k8s-agent", "pid": 101, "cpu_percent": 0.5, "memory_bytes": 47185920, "state": "running"},
					{"name": "postgres", "pid": 202, "cpu_percent": 1.2, "memory_bytes": 268435456, "state": "running"},
					{"name": "containerd", "pid": 303, "cpu_percent": 0.3, "memory_bytes": 62914560, "state": "running"}
				]
			}`))
		case "/k8s/health":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"node_name": "node-60",
				"containerd_healthy": true,
				"kubelet_healthy": false
			}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer onlineServer.Close()

	hostRepo := &mockComputeHostRepo{
		hosts: []domainDocker.ComputeHost{
			{
				ID:       "host-1",
				Name:     "node-60",
				Endpoint: onlineServer.URL,
				HostType: "k8s",
				Labels:   map[string]string{"auth_token": "test-token"},
			},
			{
				ID:       "host-2",
				Name:     "node-offline",
				Endpoint: "http://127.0.0.1:59998",
				HostType: "k8s",
			},
		},
	}

	repo := NewDeploymentRepoWithInterface(nil, nil, nil, nil, WithComputeHostRepo(hostRepo))
	apps, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}

	if len(apps) == 0 {
		t.Fatalf("expected discovered live fleet daemons, got 0")
	}

	appsByHostAndName := make(map[string]map[string]interface{})
	for _, app := range apps {
		key := app.Target + "/" + app.Name
		appsByHostAndName[key] = map[string]interface{}{
			"namespace": app.Namespace,
			"target":    app.Target,
			"type":      app.Type,
			"status":    app.Status,
			"replicas":  app.Replicas,
			"port":      app.Port,
		}
	}

	// 1. Verify online host daemons
	agentApp, ok := appsByHostAndName["node-60/k8s-agent"]
	if !ok {
		t.Fatalf("expected node-60/k8s-agent discovered")
	}
	if agentApp["status"] != "healthy" || agentApp["replicas"] != 1 || agentApp["type"] != "daemon" || agentApp["port"] != 9100 {
		t.Errorf("unexpected node-60/k8s-agent: %+v", agentApp)
	}

	pgApp, ok := appsByHostAndName["node-60/postgres"]
	if !ok {
		t.Fatalf("expected node-60/postgres discovered")
	}
	if pgApp["status"] != "healthy" || pgApp["replicas"] != 1 || pgApp["type"] != "daemon" || pgApp["port"] != 5432 {
		t.Errorf("unexpected node-60/postgres: %+v", pgApp)
	}

	containerdApp, ok := appsByHostAndName["node-60/containerd"]
	if !ok {
		t.Fatalf("expected node-60/containerd discovered")
	}
	if containerdApp["status"] != "healthy" || containerdApp["replicas"] != 1 || containerdApp["type"] != "daemon" {
		t.Errorf("unexpected node-60/containerd: %+v", containerdApp)
	}

	// 2. Verify offline host daemon mapped as down
	offlineApp, ok := appsByHostAndName["node-offline/k8s-agent"]
	if !ok {
		t.Fatalf("expected node-offline/k8s-agent created as down entry")
	}
	if offlineApp["status"] != "down" || offlineApp["replicas"] != 0 {
		t.Errorf("expected node-offline/k8s-agent down with 0 replicas, got: %+v", offlineApp)
	}
}
