package fleet

import (
	"context"
	"time"
)

// Repository defines data access for the multi-cluster fleet.
type Repository interface {
	ListClusters(ctx context.Context) ([]Cluster, error)
	GetCluster(ctx context.Context, id string) (*Cluster, error)
	RegisterCluster(ctx context.Context, c *Cluster) error
	UpdateClusterStatus(ctx context.Context, id, status, version string, nodes int) error
	UpdateHealth(ctx context.Context, id, healthStatus string, lastCheck time.Time) error
	UpdateDiscoveredResources(ctx context.Context, id string, resources map[string]interface{}) error
	RemoveCluster(ctx context.Context, id string) error
}
