package observability

import "context"

// Repository defines the data access interface for observability data.
type Repository interface {
	// SLO Definitions
	ListSLODefinitions(ctx context.Context) ([]SLODefinition, error)
	GetSLODefinition(ctx context.Context, id string) (*SLODefinition, error)
	CreateSLODefinition(ctx context.Context, d *SLODefinition) error
	UpdateSLODefinition(ctx context.Context, d *SLODefinition) error
	DeleteSLODefinition(ctx context.Context, id string) error

	// SLO Snapshots
	ListSLOSnapshots(ctx context.Context) ([]SLOSnapshot, error)
	GetSLOSnapshotBySLOID(ctx context.Context, sloID string) (*SLOSnapshot, error)
	CreateSLOSnapshot(ctx context.Context, s *SLOSnapshot) error
	UpdateSLOSnapshot(ctx context.Context, s *SLOSnapshot) error
	DeleteSLOSnapshotBySLOID(ctx context.Context, sloID string) error

	// Seed helper
	SeedDefaultSLOs(ctx context.Context) error
}
