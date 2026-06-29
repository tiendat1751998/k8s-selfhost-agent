package correlation

import "context"

// Repository defines data access for event correlation.
type Repository interface {
	ListCorrelated(ctx context.Context, status string, limit, offset int) ([]CorrelatedEvent, int, error)
	GetByID(ctx context.Context, id string) (*CorrelatedEvent, error)
	Create(ctx context.Context, c *CorrelatedEvent) error
	Resolve(ctx context.Context, id string) error
}
