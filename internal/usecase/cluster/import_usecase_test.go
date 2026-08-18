package cluster

import (
	"context"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/fleet"
	"go.uber.org/zap"
)

type mockFleetRepo struct {
	registeredCluster *fleet.Cluster
}

func (m *mockFleetRepo) ListClusters(ctx context.Context) ([]fleet.Cluster, error) {
	return nil, nil
}
func (m *mockFleetRepo) GetCluster(ctx context.Context, id string) (*fleet.Cluster, error) {
	return nil, nil
}
func (m *mockFleetRepo) RegisterCluster(ctx context.Context, c *fleet.Cluster) error {
	m.registeredCluster = c
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

type mockDiscovery struct{}

func (d *mockDiscovery) DiscoverResources(ctx context.Context, kubeconfig []byte) (map[string]interface{}, error) {
	return map[string]interface{}{
		"version":     "v1.28.0",
		"nodes_count": 3,
	}, nil
}

func (d *mockDiscovery) ValidateConnectivity(ctx context.Context, kubeconfig []byte) error {
	return nil
}

func TestImportCluster_PassesRawKubeconfig(t *testing.T) {
	repo := &mockFleetRepo{}
	discovery := &mockDiscovery{}
	logger := zap.NewNop()

	usecase := NewImportUsecase(repo, discovery, logger)
	rawKubeconfig := []byte("apiVersion: v1\nkind: Config\nclusters: []")

	c, err := usecase.ImportCluster(context.Background(), "cluster-1", "test-cluster", "default", "us-east-1", "kubernetes", rawKubeconfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c == nil {
		t.Fatal("expected cluster, got nil")
	}

	if repo.registeredCluster == nil {
		t.Fatal("expected cluster to be registered in repo, but got nil")
	}

	// Verify that EncryptedToken contains the raw kubeconfig string, not an encrypted string
	if repo.registeredCluster.EncryptedToken != string(rawKubeconfig) {
		t.Errorf("expected raw kubeconfig %q in EncryptedToken, got %q", string(rawKubeconfig), repo.registeredCluster.EncryptedToken)
	}
}
