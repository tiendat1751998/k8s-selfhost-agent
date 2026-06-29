package runbook

import "context"

// Repository defines the data access interface for runbooks.
type Repository interface {
	List(ctx context.Context, category string, limit, offset int) ([]Runbook, int, error)
	GetByID(ctx context.Context, id string) (*Runbook, error)
	Create(ctx context.Context, r *Runbook) error
	Update(ctx context.Context, r *Runbook) error
	Delete(ctx context.Context, id string) error
}
