package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/datdt/k8sselfhost/internal/domain/ecosystem"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

type ecosystemRepo struct {
	pool DBTX
}

// NewEcosystemRepo creates a new PostgreSQL-backed Ecosystem repository.
func NewEcosystemRepo(pool DBTX) ecosystem.Repository {
	return &ecosystemRepo{pool: pool}
}

func (r *ecosystemRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.pool)
}

// GetAll retrieves all detected ecosystem tools for a tenant.
func (r *ecosystemRepo) GetAll(ctx context.Context, tenantID string) ([]ecosystem.DetectedTool, error) {
	if tenantID == "" {
		tenantID = "default-tenant"
	}

	query := `
		SELECT id, name, category, status, version, endpoint, source, health, last_checked, metadata, tenant_id
		FROM ecosystem_tools
		WHERE tenant_id = $1
		ORDER BY name ASC
	`

	rows, err := r.getDB(ctx).Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("querying ecosystem tools: %w", err)
	}
	defer rows.Close()

	items := make([]ecosystem.DetectedTool, 0)
	for rows.Next() {
		var tool ecosystem.DetectedTool
		var metadataJSON []byte

		if err := rows.Scan(
			&tool.ID,
			&tool.Name,
			&tool.Category,
			&tool.Status,
			&tool.Version,
			&tool.Endpoint,
			&tool.Source,
			&tool.Health,
			&tool.LastChecked,
			&metadataJSON,
			&tool.TenantID,
		); err != nil {
			return nil, fmt.Errorf("scanning ecosystem tool row: %w", err)
		}

		tool.Metadata = make(map[string]string)
		if len(metadataJSON) > 0 {
			_ = json.Unmarshal(metadataJSON, &tool.Metadata)
		}

		items = append(items, tool)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating ecosystem tools: %w", err)
	}

	return items, nil
}

// GetByID retrieves a single detected tool by tenant and ID.
func (r *ecosystemRepo) GetByID(ctx context.Context, tenantID, id string) (*ecosystem.DetectedTool, error) {
	if tenantID == "" {
		tenantID = "default-tenant"
	}

	query := `
		SELECT id, name, category, status, version, endpoint, source, health, last_checked, metadata, tenant_id
		FROM ecosystem_tools
		WHERE tenant_id = $1 AND id = $2
	`

	var tool ecosystem.DetectedTool
	var metadataJSON []byte

	err := r.getDB(ctx).QueryRow(ctx, query, tenantID, id).Scan(
		&tool.ID,
		&tool.Name,
		&tool.Category,
		&tool.Status,
		&tool.Version,
		&tool.Endpoint,
		&tool.Source,
		&tool.Health,
		&tool.LastChecked,
		&metadataJSON,
		&tool.TenantID,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying ecosystem tool by ID %q: %w", id, err)
	}

	tool.Metadata = make(map[string]string)
	if len(metadataJSON) > 0 {
		_ = json.Unmarshal(metadataJSON, &tool.Metadata)
	}

	return &tool, nil
}

// Create registers a new detected tool or manual tool.
func (r *ecosystemRepo) Create(ctx context.Context, tool *ecosystem.DetectedTool) error {
	if tool == nil {
		return fmt.Errorf("tool cannot be nil")
	}

	if err := tool.Validate(); err != nil {
		return err
	}

	if tool.ID == "" {
		tool.ID = uuid.NewString()
	}

	tenantID := tenancy.TenantIDFromContext(ctx)
	if tool.TenantID == "" {
		if tenantID != "" {
			tool.TenantID = tenantID
		} else {
			tool.TenantID = "default-tenant"
		}
	}

	now := time.Now().UTC()
	if tool.LastChecked.IsZero() {
		tool.LastChecked = now
	}
	if tool.Status == "" {
		tool.Status = ecosystem.StatusDetected
	}
	if tool.Health == "" {
		tool.Health = ecosystem.HealthHealthy
	}
	if tool.Source == "" {
		tool.Source = ecosystem.SourceManual
	}
	if tool.Metadata == nil {
		tool.Metadata = make(map[string]string)
	}

	metadataJSON, err := json.Marshal(tool.Metadata)
	if err != nil {
		return fmt.Errorf("marshaling tool metadata: %w", err)
	}

	query := `
		INSERT INTO ecosystem_tools (
			id, name, category, status, version, endpoint, source, health, last_checked, metadata, tenant_id, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (name, tenant_id) DO UPDATE SET
			category     = EXCLUDED.category,
			status       = EXCLUDED.status,
			version      = EXCLUDED.version,
			endpoint     = EXCLUDED.endpoint,
			source       = EXCLUDED.source,
			health       = EXCLUDED.health,
			last_checked = EXCLUDED.last_checked,
			metadata     = EXCLUDED.metadata,
			updated_at   = EXCLUDED.updated_at
		RETURNING id
	`

	err = r.getDB(ctx).QueryRow(ctx, query,
		tool.ID,
		tool.Name,
		tool.Category,
		tool.Status,
		tool.Version,
		tool.Endpoint,
		tool.Source,
		tool.Health,
		tool.LastChecked,
		metadataJSON,
		tool.TenantID,
		now,
		now,
	).Scan(&tool.ID)
	if err != nil {
		return fmt.Errorf("inserting ecosystem tool: %w", err)
	}

	return nil
}

// BulkUpsert inserts or updates multiple ecosystem tools in a batch.
func (r *ecosystemRepo) BulkUpsert(ctx context.Context, tools []ecosystem.DetectedTool) error {
	if len(tools) == 0 {
		return nil
	}

	now := time.Now().UTC()
	db := r.getDB(ctx)

	for i := range tools {
		t := &tools[i]
		if t.ID == "" {
			t.ID = uuid.NewString()
		}
		if t.TenantID == "" {
			t.TenantID = "default-tenant"
		}
		if t.LastChecked.IsZero() {
			t.LastChecked = now
		}
		if t.Metadata == nil {
			t.Metadata = make(map[string]string)
		}

		metadataJSON, err := json.Marshal(t.Metadata)
		if err != nil {
			return fmt.Errorf("marshaling tool metadata for %q: %w", t.Name, err)
		}

		query := `
			INSERT INTO ecosystem_tools (
				id, name, category, status, version, endpoint, source, health, last_checked, metadata, tenant_id, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			ON CONFLICT (name, tenant_id) DO UPDATE SET
				category     = EXCLUDED.category,
				status       = EXCLUDED.status,
				version      = EXCLUDED.version,
				endpoint     = EXCLUDED.endpoint,
				source       = EXCLUDED.source,
				health       = EXCLUDED.health,
				last_checked = EXCLUDED.last_checked,
				metadata     = EXCLUDED.metadata,
				updated_at   = EXCLUDED.updated_at
		`

		_, err = db.Exec(ctx, query,
			t.ID,
			t.Name,
			t.Category,
			t.Status,
			t.Version,
			t.Endpoint,
			t.Source,
			t.Health,
			t.LastChecked,
			metadataJSON,
			t.TenantID,
			now,
			now,
		)
		if err != nil {
			return fmt.Errorf("upserting ecosystem tool %q: %w", t.Name, err)
		}
	}

	return nil
}

// Delete removes an ecosystem tool by ID for a tenant.
func (r *ecosystemRepo) Delete(ctx context.Context, tenantID, id string) error {
	if tenantID == "" {
		tenantID = "default-tenant"
	}

	query := `DELETE FROM ecosystem_tools WHERE tenant_id = $1 AND id = $2`
	result, err := r.getDB(ctx).Exec(ctx, query, tenantID, id)
	if err != nil {
		return fmt.Errorf("deleting ecosystem tool %q: %w", id, err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("tool not found: %s", id)
	}

	return nil
}

// GetSummary computes summary statistics of detected tools for a tenant.
func (r *ecosystemRepo) GetSummary(ctx context.Context, tenantID string) (*ecosystem.EcosystemSummary, error) {
	if tenantID == "" {
		tenantID = "default-tenant"
	}

	tools, err := r.GetAll(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("getting tools for summary: %w", err)
	}

	summary := &ecosystem.EcosystemSummary{
		Total:      len(tools),
		Healthy:    0,
		Degraded:   0,
		ByCategory: make(map[string]int),
	}

	for _, t := range tools {
		summary.ByCategory[t.Category]++
		if t.Health == ecosystem.HealthHealthy {
			summary.Healthy++
		} else if t.Health == ecosystem.HealthDegraded || t.Status == ecosystem.StatusUnreachable {
			summary.Degraded++
		}
	}

	return summary, nil
}
