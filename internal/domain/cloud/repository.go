package cloud

import (
	"context"
)

// Provider defines cloud provider operations for Kubernetes cluster lifecycle and discovery.
type Provider interface {
	ValidateCredentials(ctx context.Context, account *CloudAccount) error
	ListClusters(ctx context.Context, account *CloudAccount) ([]CloudCluster, error)
	GetKubeconfig(ctx context.Context, account *CloudAccount, clusterName string) ([]byte, error)
	ListNodeGroups(ctx context.Context, account *CloudAccount, clusterName string) ([]CloudNodeGroup, error)
	ScaleNodeGroup(ctx context.Context, account *CloudAccount, clusterName string, nodeGroupName string, desiredSize int) error
}

// AccountRepository defines persistent storage operations for cloud provider accounts.
type AccountRepository interface {
	Create(ctx context.Context, account *CloudAccount) error
	GetByID(ctx context.Context, id string) (*CloudAccount, error)
	List(ctx context.Context, tenantID string) ([]CloudAccount, error)
	Update(ctx context.Context, account *CloudAccount) error
	Delete(ctx context.Context, id string) error
}
