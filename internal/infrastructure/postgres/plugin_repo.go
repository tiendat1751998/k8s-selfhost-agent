package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/datdt/k8sselfhost/internal/domain/plugin"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

type pluginRepo struct {
	pool DBTX
}

// NewPluginRepo creates a new PostgreSQL-backed Plugin repository.
func NewPluginRepo(pool DBTX) plugin.Repository {
	return &pluginRepo{pool: pool}
}

func (r *pluginRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.pool)
}

// Create persists a new Plugin with JSONB permissions and config.
func (r *pluginRepo) Create(ctx context.Context, p *plugin.Plugin) error {
	if p == nil {
		return fmt.Errorf("plugin cannot be nil")
	}

	if p.ID == "" {
		p.ID = uuid.NewString()
	}

	tenantID := tenancy.TenantIDFromContext(ctx)
	userRole := tenancy.UserRoleFromContext(ctx)
	if userRole != "platform_admin" && tenantID != "" {
		p.TenantID = tenantID
	}
	if p.TenantID == "" {
		p.TenantID = "default-tenant"
	}

	if p.Version == "" {
		p.Version = "1.0.0"
	}
	if p.Category == "" {
		p.Category = plugin.CategoryDevtools
	}
	if p.Permissions == nil {
		p.Permissions = []string{}
	}
	if p.Config == nil {
		p.Config = map[string]string{}
	}

	now := time.Now().UTC()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now

	permsJSON, err := json.Marshal(p.Permissions)
	if err != nil {
		return fmt.Errorf("marshaling permissions: %w", err)
	}

	configJSON, err := json.Marshal(p.Config)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	query := `
		INSERT INTO plugins (
			id, name, version, description, author,
			enabled, entry_point, icon, category,
			permissions, config, tenant_id, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	_, err = r.getDB(ctx).Exec(ctx, query,
		p.ID,
		p.Name,
		p.Version,
		p.Description,
		p.Author,
		p.Enabled,
		p.EntryPoint,
		p.Icon,
		p.Category,
		permsJSON,
		configJSON,
		p.TenantID,
		p.CreatedAt,
		p.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return plugin.ErrDuplicateName
		}
		return fmt.Errorf("creating plugin: %w", err)
	}

	return nil
}

// GetByID retrieves a Plugin by its ID.
func (r *pluginRepo) GetByID(ctx context.Context, id string) (*plugin.Plugin, error) {
	query := `
		SELECT id, name, version, description, author,
		       enabled, entry_point, icon, category,
		       permissions, config, tenant_id, created_at, updated_at
		FROM plugins
		WHERE id = $1
	`
	query, args := BuildTenantQuery(ctx, query, id)

	var p plugin.Plugin
	var permsRaw []byte
	var configRaw []byte

	err := r.getDB(ctx).QueryRow(ctx, query, args...).Scan(
		&p.ID,
		&p.Name,
		&p.Version,
		&p.Description,
		&p.Author,
		&p.Enabled,
		&p.EntryPoint,
		&p.Icon,
		&p.Category,
		&permsRaw,
		&configRaw,
		&p.TenantID,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("getting plugin by id: %w", err)
	}

	p.Permissions = []string{}
	if len(permsRaw) > 0 {
		if err := json.Unmarshal(permsRaw, &p.Permissions); err != nil {
			return nil, fmt.Errorf("unmarshaling permissions: %w", err)
		}
	}
	if p.Permissions == nil {
		p.Permissions = []string{}
	}

	p.Config = map[string]string{}
	if len(configRaw) > 0 {
		if err := json.Unmarshal(configRaw, &p.Config); err != nil {
			return nil, fmt.Errorf("unmarshaling config: %w", err)
		}
	}
	if p.Config == nil {
		p.Config = map[string]string{}
	}

	return &p, nil
}

// List returns plugins matching tenantID and enabled filter.
func (r *pluginRepo) List(ctx context.Context, tenantID string, enabledOnly bool) ([]plugin.Plugin, error) {
	if tenantID != "" && tenancy.TenantIDFromContext(ctx) == "" {
		ctx = context.WithValue(ctx, tenancy.TenantIDKey, tenantID)
	}

	baseQuery := "FROM plugins WHERE 1=1"
	var args []any
	argIdx := 1

	userRole := tenancy.UserRoleFromContext(ctx)
	if userRole == "platform_admin" && tenantID != "" {
		baseQuery += fmt.Sprintf(" AND tenant_id = $%d", argIdx)
		args = append(args, tenantID)
		argIdx++
	}

	if enabledOnly {
		baseQuery += fmt.Sprintf(" AND enabled = $%d", argIdx)
		args = append(args, true)
		argIdx++
	}

	selectQuery := fmt.Sprintf(
		"SELECT id, name, version, description, author, enabled, entry_point, icon, category, permissions, config, tenant_id, created_at, updated_at %s ORDER BY created_at DESC",
		baseQuery,
	)
	selectQuery, args = BuildTenantQuery(ctx, selectQuery, args...)

	rows, err := r.getDB(ctx).Query(ctx, selectQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("listing plugins: %w", err)
	}
	defer rows.Close()

	plugins := make([]plugin.Plugin, 0)
	for rows.Next() {
		var p plugin.Plugin
		var permsRaw []byte
		var configRaw []byte

		if err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.Version,
			&p.Description,
			&p.Author,
			&p.Enabled,
			&p.EntryPoint,
			&p.Icon,
			&p.Category,
			&permsRaw,
			&configRaw,
			&p.TenantID,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning plugin: %w", err)
		}

		p.Permissions = []string{}
		if len(permsRaw) > 0 {
			_ = json.Unmarshal(permsRaw, &p.Permissions)
		}
		if p.Permissions == nil {
			p.Permissions = []string{}
		}

		p.Config = map[string]string{}
		if len(configRaw) > 0 {
			_ = json.Unmarshal(configRaw, &p.Config)
		}
		if p.Config == nil {
			p.Config = map[string]string{}
		}

		plugins = append(plugins, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating plugin rows: %w", err)
	}

	return plugins, nil
}

// Update updates an existing plugin.
func (r *pluginRepo) Update(ctx context.Context, p *plugin.Plugin) error {
	if p == nil {
		return fmt.Errorf("plugin cannot be nil")
	}

	p.UpdatedAt = time.Now().UTC()

	permsJSON, err := json.Marshal(p.Permissions)
	if err != nil {
		return fmt.Errorf("marshaling permissions: %w", err)
	}

	configJSON, err := json.Marshal(p.Config)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	query := `
		UPDATE plugins
		SET name = $1,
		    version = $2,
		    description = $3,
		    author = $4,
		    enabled = $5,
		    entry_point = $6,
		    icon = $7,
		    category = $8,
		    permissions = $9,
		    config = $10,
		    updated_at = $11
		WHERE id = $12
	`
	query, args := BuildTenantQuery(ctx, query,
		p.Name,
		p.Version,
		p.Description,
		p.Author,
		p.Enabled,
		p.EntryPoint,
		p.Icon,
		p.Category,
		permsJSON,
		configJSON,
		p.UpdatedAt,
		p.ID,
	)

	ct, err := r.getDB(ctx).Exec(ctx, query, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return plugin.ErrDuplicateName
		}
		return fmt.Errorf("updating plugin: %w", err)
	}

	if ct.RowsAffected() == 0 {
		return plugin.ErrNotFound
	}

	return nil
}

// Delete removes a plugin by its ID.
func (r *pluginRepo) Delete(ctx context.Context, id string) error {
	query := "DELETE FROM plugins WHERE id = $1"
	query, args := BuildTenantQuery(ctx, query, id)

	ct, err := r.getDB(ctx).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("deleting plugin: %w", err)
	}

	if ct.RowsAffected() == 0 {
		return plugin.ErrNotFound
	}

	return nil
}

// Toggle enables or disables a plugin by ID.
func (r *pluginRepo) Toggle(ctx context.Context, id string, enabled bool) error {
	query := `
		UPDATE plugins
		SET enabled = $1,
		    updated_at = $2
		WHERE id = $3
	`
	query, args := BuildTenantQuery(ctx, query, enabled, time.Now().UTC(), id)

	ct, err := r.getDB(ctx).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("toggling plugin: %w", err)
	}

	if ct.RowsAffected() == 0 {
		return plugin.ErrNotFound
	}

	return nil
}

// GetStats returns aggregated plugin metrics for a tenant.
func (r *pluginRepo) GetStats(ctx context.Context, tenantID string) (*plugin.PluginStats, error) {
	if tenantID != "" && tenancy.TenantIDFromContext(ctx) == "" {
		ctx = context.WithValue(ctx, tenancy.TenantIDKey, tenantID)
	}

	stats := &plugin.PluginStats{
		ByCategory: make(map[string]int),
	}

	userRole := tenancy.UserRoleFromContext(ctx)

	// 1. Total counts
	var totalQuery string
	var totalArgs []any
	if userRole == "platform_admin" && tenantID != "" {
		totalQuery = `
			SELECT
				COUNT(*),
				COALESCE(COUNT(*) FILTER (WHERE enabled = true), 0),
				COALESCE(COUNT(*) FILTER (WHERE enabled = false), 0)
			FROM plugins
			WHERE tenant_id = $1
		`
		totalArgs = []any{tenantID}
	} else {
		totalQuery = `
			SELECT
				COUNT(*),
				COALESCE(COUNT(*) FILTER (WHERE enabled = true), 0),
				COALESCE(COUNT(*) FILTER (WHERE enabled = false), 0)
			FROM plugins
		`
	}
	totalQuery, totalArgs = BuildTenantQuery(ctx, totalQuery, totalArgs...)

	err := r.getDB(ctx).QueryRow(ctx, totalQuery, totalArgs...).Scan(
		&stats.Total,
		&stats.Enabled,
		&stats.Disabled,
	)
	if err != nil {
		return nil, fmt.Errorf("querying plugin totals: %w", err)
	}

	// 2. Counts by category
	var catQuery string
	var catArgs []any
	if userRole == "platform_admin" && tenantID != "" {
		catQuery = `
			SELECT category, COUNT(*)
			FROM plugins
			WHERE tenant_id = $1
			GROUP BY category
		`
		catArgs = []any{tenantID}
	} else {
		catQuery = `
			SELECT category, COUNT(*)
			FROM plugins
			GROUP BY category
		`
	}
	catQuery, catArgs = BuildTenantQuery(ctx, catQuery, catArgs...)

	rows, err := r.getDB(ctx).Query(ctx, catQuery, catArgs...)
	if err != nil {
		return nil, fmt.Errorf("querying plugin category stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cat string
		var count int
		if err := rows.Scan(&cat, &count); err != nil {
			return nil, fmt.Errorf("scanning category row: %w", err)
		}
		stats.ByCategory[cat] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating category stats rows: %w", err)
	}

	return stats, nil
}
