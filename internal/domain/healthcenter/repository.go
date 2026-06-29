package healthcenter

import "context"

// Repository defines data access for the health center.
type Repository interface {
	GetStatuses(ctx context.Context) ([]ComponentStatus, error)
	UpdateStatus(ctx context.Context, cs *ComponentStatus) error
}
