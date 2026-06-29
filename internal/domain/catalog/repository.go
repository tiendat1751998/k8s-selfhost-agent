package catalog

import "context"

// Repository defines the data access interface for the service catalog.
type Repository interface {
	List(ctx context.Context, category string, limit, offset int) ([]ServiceTemplate, int, error)
	GetByID(ctx context.Context, id string) (*ServiceTemplate, error)
	Create(ctx context.Context, t *ServiceTemplate) error
	Update(ctx context.Context, t *ServiceTemplate) error
	Delete(ctx context.Context, id string) error
	IncrementDeployCount(ctx context.Context, id string) error
}
