package promotion

import "context"

// Repository defines data access for deployment promotions.
type Repository interface {
	List(ctx context.Context, status string, limit, offset int) ([]Promotion, int, error)
	GetByID(ctx context.Context, id string) (*Promotion, error)
	Create(ctx context.Context, p *Promotion) error
	Approve(ctx context.Context, id, approver string) error
	Complete(ctx context.Context, id string) error
}
