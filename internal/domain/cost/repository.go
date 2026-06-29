package cost

import "context"

// Repository defines data access interfaces for resource cost and waste metrics.
type Repository interface {
	GetClusterCosts(ctx context.Context) ([]ClusterCost, error)
	GetNamespaceCosts(ctx context.Context) ([]NamespaceCost, error)
	GetResourceWaste(ctx context.Context) ([]ResourceWaste, error)
}
