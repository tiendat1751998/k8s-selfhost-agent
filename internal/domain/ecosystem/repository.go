package ecosystem

import "context"

// Repository defines persistent storage operations for detected ecosystem tools.
type Repository interface {
	GetAll(ctx context.Context, tenantID string) ([]DetectedTool, error)
	GetByID(ctx context.Context, tenantID, id string) (*DetectedTool, error)
	Create(ctx context.Context, tool *DetectedTool) error
	BulkUpsert(ctx context.Context, tools []DetectedTool) error
	Delete(ctx context.Context, tenantID, id string) error
	GetSummary(ctx context.Context, tenantID string) (*EcosystemSummary, error)
}
