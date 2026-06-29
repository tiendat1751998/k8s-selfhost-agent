package explorer

import "context"

// Repository defines data access for resource explorer.
type Repository interface {
	Search(ctx context.Context, kind, cluster, namespace, query string, limit, offset int) ([]Resource, int, error)
	GetByID(ctx context.Context, id string) (*Resource, error)
	SyncResource(ctx context.Context, res *Resource) error
}
