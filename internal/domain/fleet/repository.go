package fleet

import "context"

// Repository defines data access for the multi-cluster fleet.
type Repository interface {
	ListClusters(ctx context.Context) ([]Cluster, error)
	GetCluster(ctx context.Context, id string) (*Cluster, error)
	RegisterCluster(ctx context.Context, c *Cluster) error
	UpdateClusterStatus(ctx context.Context, id, status, version string, nodes int) error
	RemoveCluster(ctx context.Context, id string) error
}
