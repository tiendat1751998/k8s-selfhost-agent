package drift

import "context"

// Repository defines data access for drift detection.
type Repository interface {
	List(ctx context.Context, cluster string, status *DriftStatus, limit, offset int) ([]DriftRecord, int, error)
	Create(ctx context.Context, d *DriftRecord) error
	Resolve(ctx context.Context, id string) error
}
