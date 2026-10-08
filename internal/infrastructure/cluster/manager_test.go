package cluster

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/fleet"
	infraConfig "github.com/datdt/k8sselfhost/internal/infrastructure/config"
	"github.com/datdt/k8sselfhost/internal/pkg/crypto"
)

type mockFleetRepo struct {
	cluster *fleet.Cluster
}

func (m *mockFleetRepo) ListClusters(ctx context.Context) ([]fleet.Cluster, error) {
	return nil, nil
}
func (m *mockFleetRepo) GetCluster(ctx context.Context, id string) (*fleet.Cluster, error) {
	if m.cluster != nil && m.cluster.ID == id {
		return m.cluster, nil
	}
	return nil, nil
}
func (m *mockFleetRepo) RegisterCluster(ctx context.Context, c *fleet.Cluster) error {
	return nil
}
func (m *mockFleetRepo) UpdateClusterStatus(ctx context.Context, id, status, version string, nodes int) error {
	return nil
}
func (m *mockFleetRepo) UpdateHealth(ctx context.Context, id, healthStatus string, lastCheck time.Time) error {
	return nil
}
func (m *mockFleetRepo) UpdateDiscoveredResources(ctx context.Context, id string, resources map[string]interface{}) error {
	return nil
}
func (m *mockFleetRepo) RemoveCluster(ctx context.Context, id string) error {
	return nil
}

func TestGetK8sClient_Decryption(t *testing.T) {
	origKey := os.Getenv("ENCRYPTION_KEY")
	defer os.Setenv("ENCRYPTION_KEY", origKey)
	os.Setenv("ENCRYPTION_KEY", "12345678901234567890123456789012")

	validKubeconfig := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://127.0.0.1:6443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    token: test-token
`

	// Test 1: Cluster with encrypted token
	encrypted, err := crypto.Encrypt(validKubeconfig)
	if err != nil {
		t.Fatalf("failed to encrypt test kubeconfig: %v", err)
	}

	repo := &mockFleetRepo{
		cluster: &fleet.Cluster{
			ID:             "test-cluster-1",
			Provider:       "kubernetes",
			EncryptedToken: encrypted,
		},
	}

	mgr := NewClientManager(repo)
	client, err := mgr.GetK8sClient(context.Background(), "test-cluster-1")
	if err != nil {
		t.Fatalf("failed to get k8s client with encrypted token: %v", err)
	}
	if client == nil {
		t.Fatal("expected k8s client, got nil")
	}

	// Test 2: Cluster with raw/already decrypted token
	repoRaw := &mockFleetRepo{
		cluster: &fleet.Cluster{
			ID:             "test-cluster-2",
			Provider:       "kubernetes",
			EncryptedToken: validKubeconfig,
		},
	}

	mgrRaw := NewClientManager(repoRaw)
	clientRaw, err := mgrRaw.GetK8sClient(context.Background(), "test-cluster-2")
	if err != nil {
		t.Fatalf("failed to get k8s client with raw token: %v", err)
	}
	if clientRaw == nil {
		t.Fatal("expected k8s client, got nil")
	}
}

func TestGetK8sClient_Providers(t *testing.T) {
	validKubeconfig := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://127.0.0.1:6443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    token: test-token
`

	tests := []struct {
		name         string
		provider     string
		importMethod string
		wantErr      bool
		errContains  string
	}{
		{
			name:     "onprem provider",
			provider: "onprem",
			wantErr:  false,
		},
		{
			name:     "k3s provider",
			provider: "k3s",
			wantErr:  false,
		},
		{
			name:     "baremetal provider",
			provider: "baremetal",
			wantErr:  false,
		},
		{
			name:     "azure provider",
			provider: "azure",
			wantErr:  false,
		},
		{
			name:     "custom provider",
			provider: "custom",
			wantErr:  false,
		},
		{
			name:        "docker provider without kubeconfig import method",
			provider:    "docker",
			wantErr:     true,
			errContains: "has provider docker, not kubernetes",
		},
		{
			name:         "docker provider with kubeconfig import method",
			provider:     "docker",
			importMethod: "kubeconfig",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockFleetRepo{
				cluster: &fleet.Cluster{
					ID:             "cluster-" + tt.provider + "-" + tt.importMethod,
					Provider:       tt.provider,
					ImportMethod:   tt.importMethod,
					EncryptedToken: validKubeconfig,
				},
			}
			mgr := NewClientManager(repo)
			client, err := mgr.GetK8sClient(context.Background(), "cluster-"+tt.provider+"-"+tt.importMethod)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetK8sClient() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if tt.errContains != "" && (err == nil || !strings.Contains(err.Error(), tt.errContains)) {
					t.Fatalf("expected error containing %q, got %v", tt.errContains, err)
				}
			} else {
				if client == nil {
					t.Fatal("expected non-nil clientset")
				}
			}
		})
	}
}

func TestGetK8sRestConfig(t *testing.T) {
	validKubeconfig := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://127.0.0.1:6443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    token: test-token
`
	repo := &mockFleetRepo{
		cluster: &fleet.Cluster{
			ID:             "rest-cluster-1",
			Provider:       "kubernetes",
			EncryptedToken: validKubeconfig,
		},
	}

	mgr := NewClientManager(repo)

	// Test empty clusterID
	_, err := mgr.GetK8sRestConfig(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty clusterID, got nil")
	}

	// Test non-existent cluster
	_, err = mgr.GetK8sRestConfig(context.Background(), "non-existent")
	if err == nil {
		t.Fatal("expected error for non-existent cluster, got nil")
	}

	// Test valid cluster
	cfg, err := mgr.GetK8sRestConfig(context.Background(), "rest-cluster-1")
	if err != nil {
		t.Fatalf("GetK8sRestConfig failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil rest.Config")
	}
	if cfg.Host != "https://127.0.0.1:6443" {
		t.Fatalf("expected host https://127.0.0.1:6443, got %s", cfg.Host)
	}
	if cfg.QPS != 50 {
		t.Fatalf("expected default QPS 50, got %f", cfg.QPS)
	}
	if cfg.Burst != 100 {
		t.Fatalf("expected default Burst 100, got %d", cfg.Burst)
	}
	if cfg.Timeout != 30*time.Second {
		t.Fatalf("expected default Timeout 30s, got %v", cfg.Timeout)
	}

	// Test caching - second call returns same instance
	cfgCached, err := mgr.GetK8sRestConfig(context.Background(), "rest-cluster-1")
	if err != nil {
		t.Fatalf("GetK8sRestConfig cached failed: %v", err)
	}
	if cfgCached != cfg {
		t.Fatal("expected cached config pointer to match")
	}
}

func TestGetK8sRestConfig_CustomConfig(t *testing.T) {
	validKubeconfig := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://127.0.0.1:6443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    token: test-token
`
	repo := &mockFleetRepo{
		cluster: &fleet.Cluster{
			ID:             "rest-cluster-custom",
			Provider:       "kubernetes",
			EncryptedToken: validKubeconfig,
		},
	}

	orig := infraConfig.GetK8sClientConfig()
	defer infraConfig.SetK8sClientConfig(orig)

	infraConfig.SetK8sClientConfig(infraConfig.K8sClientConfig{
		QPS:     75,
		Burst:   150,
		Timeout: 45 * time.Second,
	})

	mgr := NewClientManager(repo)
	cfg, err := mgr.GetK8sRestConfig(context.Background(), "rest-cluster-custom")
	if err != nil {
		t.Fatalf("GetK8sRestConfig failed: %v", err)
	}
	if cfg.QPS != 75 {
		t.Fatalf("expected QPS 75, got %f", cfg.QPS)
	}
	if cfg.Burst != 150 {
		t.Fatalf("expected Burst 150, got %d", cfg.Burst)
	}
	if cfg.Timeout != 45*time.Second {
		t.Fatalf("expected Timeout 45s, got %v", cfg.Timeout)
	}
}

func TestGetDockerClient_EnvFallback(t *testing.T) {
	repo := &mockFleetRepo{
		cluster: &fleet.Cluster{
			ID:             "docker-cluster-1",
			Provider:       "docker",
			EncryptedToken: "",
		},
	}

	t.Run("defaults to unix socket when env unset", func(t *testing.T) {
		t.Setenv("DOCKER_HOST", "")
		t.Setenv("K8S_DOCKER_HOST", "")

		mgr := NewClientManager(repo)
		cli, err := mgr.GetDockerClient(context.Background(), "docker-cluster-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cli == nil {
			t.Fatal("expected non-nil docker client")
		}
	})

	t.Run("uses DOCKER_HOST when set", func(t *testing.T) {
		t.Setenv("DOCKER_HOST", "tcp://custom-docker.internal:2376")

		mgr := NewClientManager(repo)
		cli, err := mgr.GetDockerClient(context.Background(), "docker-cluster-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cli == nil {
			t.Fatal("expected non-nil docker client")
		}
	})
}

