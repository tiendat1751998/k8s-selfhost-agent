package observability

import "context"

// Repository defines the data access interface for observability data.
type Repository interface {
	// SLO
	ListSLODefinitions(ctx context.Context) ([]SLODefinition, error)
	CreateSLODefinition(ctx context.Context, d *SLODefinition) error
	ListSLOSnapshots(ctx context.Context) ([]SLOSnapshot, error)
	CreateSLOSnapshot(ctx context.Context, s *SLOSnapshot) error
}
