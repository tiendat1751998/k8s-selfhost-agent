package deployment

import (
	"context"
)

// Repository defines data access and mutations for applications/deployments.
type Repository interface {
	List(ctx context.Context) ([]Application, error)
	Scale(ctx context.Context, targetType, targetCluster, namespace, name string, replicas int) error
	Restart(ctx context.Context, targetType, targetCluster, namespace, name string) error
	Delete(ctx context.Context, targetType, targetCluster, namespace, name string) error
	Create(ctx context.Context, app Application) error
}
