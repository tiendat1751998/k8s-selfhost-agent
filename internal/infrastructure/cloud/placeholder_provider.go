package cloud

import (
	"context"
	"errors"
	"fmt"

	domain "github.com/datdt/k8sselfhost/internal/domain/cloud"
)

// ErrProviderNotConfigured indicates the real cloud SDK is not yet installed.
var ErrProviderNotConfigured = errors.New("cloud provider SDK not configured")

var _ domain.Provider = (*PlaceholderProvider)(nil)

// PlaceholderProvider implements domain.Provider returning ErrProviderNotConfigured.
type PlaceholderProvider struct {
	name string
}

// NewPlaceholderProvider creates a new PlaceholderProvider for a cloud provider.
func NewPlaceholderProvider(name string) *PlaceholderProvider {
	return &PlaceholderProvider{name: name}
}

// ValidateCredentials validates account credentials against cloud provider SDK.
func (p *PlaceholderProvider) ValidateCredentials(ctx context.Context, account *domain.CloudAccount) error {
	return fmt.Errorf("%s: %w", p.name, ErrProviderNotConfigured)
}

// ListClusters retrieves list of Kubernetes clusters from cloud provider.
func (p *PlaceholderProvider) ListClusters(ctx context.Context, account *domain.CloudAccount) ([]domain.CloudCluster, error) {
	return nil, fmt.Errorf("%s: %w", p.name, ErrProviderNotConfigured)
}

// GetKubeconfig retrieves kubeconfig for a cluster from cloud provider.
func (p *PlaceholderProvider) GetKubeconfig(ctx context.Context, account *domain.CloudAccount, clusterName string) ([]byte, error) {
	return nil, fmt.Errorf("%s: %w", p.name, ErrProviderNotConfigured)
}

// ListNodeGroups retrieves managed node groups for a cluster from cloud provider.
func (p *PlaceholderProvider) ListNodeGroups(ctx context.Context, account *domain.CloudAccount, clusterName string) ([]domain.CloudNodeGroup, error) {
	return nil, fmt.Errorf("%s: %w", p.name, ErrProviderNotConfigured)
}

// ScaleNodeGroup scales a node group desired capacity on cloud provider.
func (p *PlaceholderProvider) ScaleNodeGroup(ctx context.Context, account *domain.CloudAccount, clusterName string, nodeGroupName string, desiredSize int) error {
	return fmt.Errorf("%s: %w", p.name, ErrProviderNotConfigured)
}
