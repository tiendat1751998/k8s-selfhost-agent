package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/datdt/k8sselfhost/internal/domain/settings"
)

type settingsRepo struct {
	pool DBTX
}

// NewSettingsRepo creates a new PostgreSQL-backed Settings repository.
func NewSettingsRepo(pool DBTX) settings.Repository {
	return &settingsRepo{pool: pool}
}

func (r *settingsRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.pool)
}

// GetByCategory retrieves all settings in a category for a given tenant.
func (r *settingsRepo) GetByCategory(ctx context.Context, tenantID, category string) ([]settings.Setting, error) {
	if tenantID == "" {
		tenantID = "default-tenant"
	}

	query := `
		SELECT id, category, key, value, tenant_id, updated_by, updated_at
		FROM platform_settings
		WHERE tenant_id = $1 AND category = $2
		ORDER BY key ASC
	`

	rows, err := r.getDB(ctx).Query(ctx, query, tenantID, category)
	if err != nil {
		return nil, fmt.Errorf("getting settings by category %q: %w", category, err)
	}
	defer rows.Close()

	items := make([]settings.Setting, 0)
	for rows.Next() {
		var s settings.Setting
		var updatedBy *string
		if err := rows.Scan(
			&s.ID,
			&s.Category,
			&s.Key,
			&s.Value,
			&s.TenantID,
			&updatedBy,
			&s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning setting row: %w", err)
		}
		if updatedBy != nil {
			s.UpdatedBy = *updatedBy
		}
		items = append(items, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating settings by category: %w", err)
	}

	return items, nil
}

// GetByKey retrieves a single setting by tenant, category, and key.
func (r *settingsRepo) GetByKey(ctx context.Context, tenantID, category, key string) (*settings.Setting, error) {
	if tenantID == "" {
		tenantID = "default-tenant"
	}

	query := `
		SELECT id, category, key, value, tenant_id, updated_by, updated_at
		FROM platform_settings
		WHERE tenant_id = $1 AND category = $2 AND key = $3
	`

	var s settings.Setting
	var updatedBy *string
	err := r.getDB(ctx).QueryRow(ctx, query, tenantID, category, key).Scan(
		&s.ID,
		&s.Category,
		&s.Key,
		&s.Value,
		&s.TenantID,
		&updatedBy,
		&s.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("getting setting by key [%s/%s]: %w", category, key, err)
	}
	if updatedBy != nil {
		s.UpdatedBy = *updatedBy
	}

	return &s, nil
}

// Upsert creates or updates a setting scoped by category, key, and tenant.
func (r *settingsRepo) Upsert(ctx context.Context, setting *settings.Setting) error {
	if setting == nil {
		return fmt.Errorf("setting cannot be nil")
	}

	if setting.ID == "" {
		setting.ID = uuid.NewString()
	}
	if setting.TenantID == "" {
		setting.TenantID = "default-tenant"
	}

	setting.UpdatedAt = time.Now().UTC()

	query := `
		INSERT INTO platform_settings (id, category, key, value, tenant_id, updated_by, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (category, key, tenant_id)
		DO UPDATE SET
			value = EXCLUDED.value,
			updated_by = EXCLUDED.updated_by,
			updated_at = EXCLUDED.updated_at
		RETURNING id, updated_at
	`

	err := r.getDB(ctx).QueryRow(ctx, query,
		setting.ID,
		setting.Category,
		setting.Key,
		setting.Value,
		setting.TenantID,
		setting.UpdatedBy,
		setting.UpdatedAt,
	).Scan(&setting.ID, &setting.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upserting setting [%s/%s]: %w", setting.Category, setting.Key, err)
	}

	return nil
}

// BulkUpsert creates or updates multiple settings within a single transaction.
func (r *settingsRepo) BulkUpsert(ctx context.Context, items []settings.Setting) error {
	if len(items) == 0 {
		return nil
	}

	db := r.getDB(ctx)
	if HasTx(ctx) {
		for i := range items {
			if err := r.Upsert(ctx, &items[i]); err != nil {
				return fmt.Errorf("bulk upsert setting [%s/%s]: %w", items[i].Category, items[i].Key, err)
			}
		}
		return nil
	}

	if pool, ok := db.(interface {
		Begin(ctx context.Context) (pgx.Tx, error)
	}); ok {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("beginning transaction for bulk upsert: %w", err)
		}
		defer func() {
			_ = tx.Rollback(ctx)
		}()

		txCtx := InjectTx(ctx, tx)
		for i := range items {
			if err := r.Upsert(txCtx, &items[i]); err != nil {
				return fmt.Errorf("bulk upsert setting [%s/%s]: %w", items[i].Category, items[i].Key, err)
			}
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("committing transaction for bulk upsert: %w", err)
		}
		return nil
	}

	for i := range items {
		if err := r.Upsert(ctx, &items[i]); err != nil {
			return fmt.Errorf("bulk upsert setting [%s/%s]: %w", items[i].Category, items[i].Key, err)
		}
	}
	return nil
}

// GetAll retrieves all settings for a given tenant ordered by category and key.
func (r *settingsRepo) GetAll(ctx context.Context, tenantID string) ([]settings.Setting, error) {
	if tenantID == "" {
		tenantID = "default-tenant"
	}

	query := `
		SELECT id, category, key, value, tenant_id, updated_by, updated_at
		FROM platform_settings
		WHERE tenant_id = $1
		ORDER BY category ASC, key ASC
	`

	rows, err := r.getDB(ctx).Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("getting all settings for tenant %q: %w", tenantID, err)
	}
	defer rows.Close()

	items := make([]settings.Setting, 0)
	for rows.Next() {
		var s settings.Setting
		var updatedBy *string
		if err := rows.Scan(
			&s.ID,
			&s.Category,
			&s.Key,
			&s.Value,
			&s.TenantID,
			&updatedBy,
			&s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning setting row: %w", err)
		}
		if updatedBy != nil {
			s.UpdatedBy = *updatedBy
		}
		items = append(items, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating all settings: %w", err)
	}

	return items, nil
}

var _ settings.Repository = (*settingsRepo)(nil)
