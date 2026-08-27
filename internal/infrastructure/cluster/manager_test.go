package cluster

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/fleet"
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
			name:     "docker provider without kubeconfig import method",
			provider: "docker",
			wantErr:  true,
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

