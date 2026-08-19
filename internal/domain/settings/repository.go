package settings

import "context"

// Repository defines persistent storage operations for platform settings.
type Repository interface {
	GetByCategory(ctx context.Context, tenantID, category string) ([]Setting, error)
	GetByKey(ctx context.Context, tenantID, category, key string) (*Setting, error)
	Upsert(ctx context.Context, setting *Setting) error
	BulkUpsert(ctx context.Context, settings []Setting) error
	GetAll(ctx context.Context, tenantID string) ([]Setting, error)
}
