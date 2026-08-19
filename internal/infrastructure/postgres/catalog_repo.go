package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/datdt/k8sselfhost/internal/domain/catalog"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

type catalogRepo struct {
	pool DBTX
}

// NewCatalogRepo creates a new PostgreSQL-backed Service Catalog repository.
func NewCatalogRepo(pool DBTX) catalog.Repository {
	return &catalogRepo{pool: pool}
}

func (r *catalogRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.pool)
}

// Create persists a new ServiceEntry with JSONB tags and annotations.
func (r *catalogRepo) Create(ctx context.Context, entry *catalog.ServiceEntry) error {
	if entry == nil {
		return fmt.Errorf("service entry cannot be nil")
	}

	if entry.ID == "" {
		entry.ID = uuid.NewString()
	}

	tenantID := tenancy.TenantIDFromContext(ctx)
	userRole := tenancy.UserRoleFromContext(ctx)
	if userRole != "platform_admin" && tenantID != "" {
		entry.TenantID = tenantID
	}
	if entry.TenantID == "" {
		entry.TenantID = "default-tenant"
	}

	if entry.Type == "" {
		entry.Type = catalog.TypeService
	}
	if entry.Lifecycle == "" {
		entry.Lifecycle = catalog.LifecycleDevelopment
	}
	if entry.Tags == nil {
		entry.Tags = []string{}
	}
	if entry.Annotations == nil {
		entry.Annotations = map[string]string{}
	}

	now := time.Now().UTC()
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}
	entry.UpdatedAt = now

	tagsJSON, err := json.Marshal(entry.Tags)
	if err != nil {
		return fmt.Errorf("marshaling tags: %w", err)
	}

	annotationsJSON, err := json.Marshal(entry.Annotations)
	if err != nil {
		return fmt.Errorf("marshaling annotations: %w", err)
	}

	query := `
		INSERT INTO service_catalog (
			id, name, description, type, lifecycle,
			owner_team, owner_email, repo_url, docs_url,
			tags, annotations, tenant_id, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	_, err = r.getDB(ctx).Exec(ctx, query,
		entry.ID,
		entry.Name,
		entry.Description,
		entry.Type,
		entry.Lifecycle,
		entry.OwnerTeam,
		entry.OwnerEmail,
		entry.RepoURL,
		entry.DocsURL,
		tagsJSON,
		annotationsJSON,
		entry.TenantID,
		entry.CreatedAt,
		entry.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("creating service catalog entry: %w", err)
	}

	return nil
}

// GetByID retrieves a ServiceEntry by ID.
func (r *catalogRepo) GetByID(ctx context.Context, id string) (*catalog.ServiceEntry, error) {
	query := `
		SELECT id, name, description, type, lifecycle,
		       owner_team, owner_email, repo_url, docs_url,
		       tags, annotations, tenant_id, created_at, updated_at
		FROM service_catalog
		WHERE id = $1
	`
	query, args := BuildTenantQuery(ctx, query, id)

	var entry catalog.ServiceEntry
	var tagsRaw []byte
	var annotationsRaw []byte

	err := r.getDB(ctx).QueryRow(ctx, query, args...).Scan(
		&entry.ID,
		&entry.Name,
		&entry.Description,
		&entry.Type,
		&entry.Lifecycle,
		&entry.OwnerTeam,
		&entry.OwnerEmail,
		&entry.RepoURL,
		&entry.DocsURL,
		&tagsRaw,
		&annotationsRaw,
		&entry.TenantID,
		&entry.CreatedAt,
		&entry.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("getting service catalog entry by id: %w", err)
	}

	entry.Tags = []string{}
	if len(tagsRaw) > 0 {
		if err := json.Unmarshal(tagsRaw, &entry.Tags); err != nil {
			return nil, fmt.Errorf("unmarshaling tags: %w", err)
		}
	}
	if entry.Tags == nil {
		entry.Tags = []string{}
	}

	entry.Annotations = map[string]string{}
	if len(annotationsRaw) > 0 {
		if err := json.Unmarshal(annotationsRaw, &entry.Annotations); err != nil {
			return nil, fmt.Errorf("unmarshaling annotations: %w", err)
		}
	}
	if entry.Annotations == nil {
		entry.Annotations = map[string]string{}
	}

	return &entry, nil
}

// List returns catalog entries matching tenantID and filter options.
func (r *catalogRepo) List(ctx context.Context, tenantID string, filter catalog.ListFilter) ([]catalog.ServiceEntry, error) {
	if tenantID != "" && tenancy.TenantIDFromContext(ctx) == "" {
		ctx = context.WithValue(ctx, tenancy.TenantIDKey, tenantID)
	}

	baseQuery := "FROM service_catalog WHERE 1=1"
	var args []any
	argIdx := 1

	userRole := tenancy.UserRoleFromContext(ctx)
	if userRole == "platform_admin" && tenantID != "" {
		baseQuery += fmt.Sprintf(" AND tenant_id = $%d", argIdx)
		args = append(args, tenantID)
		argIdx++
	}

	if filter.Type != "" {
		baseQuery += fmt.Sprintf(" AND type = $%d", argIdx)
		args = append(args, filter.Type)
		argIdx++
	}

	if filter.Lifecycle != "" {
		baseQuery += fmt.Sprintf(" AND lifecycle = $%d", argIdx)
		args = append(args, filter.Lifecycle)
		argIdx++
	}

	if filter.OwnerTeam != "" {
		baseQuery += fmt.Sprintf(" AND owner_team = $%d", argIdx)
		args = append(args, filter.OwnerTeam)
		argIdx++
	}

	if filter.Search != "" {
		searchPattern := fmt.Sprintf("%%%s%%", filter.Search)
		baseQuery += fmt.Sprintf(" AND (name ILIKE $%d OR description ILIKE $%d)", argIdx, argIdx)
		args = append(args, searchPattern)
		argIdx++
	}

	selectQuery := fmt.Sprintf(
		"SELECT id, name, description, type, lifecycle, owner_team, owner_email, repo_url, docs_url, tags, annotations, tenant_id, created_at, updated_at %s ORDER BY created_at DESC",
		baseQuery,
	)
	selectQuery, args = BuildTenantQuery(ctx, selectQuery, args...)

	rows, err := r.getDB(ctx).Query(ctx, selectQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("listing service catalog entries: %w", err)
	}
	defer rows.Close()

	entries := make([]catalog.ServiceEntry, 0)
	for rows.Next() {
		var entry catalog.ServiceEntry
		var tagsRaw []byte
		var annotationsRaw []byte

		if err := rows.Scan(
			&entry.ID,
			&entry.Name,
			&entry.Description,
			&entry.Type,
			&entry.Lifecycle,
			&entry.OwnerTeam,
			&entry.OwnerEmail,
			&entry.RepoURL,
			&entry.DocsURL,
			&tagsRaw,
			&annotationsRaw,
			&entry.TenantID,
			&entry.CreatedAt,
			&entry.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning service catalog entry: %w", err)
		}

		entry.Tags = []string{}
		if len(tagsRaw) > 0 {
			if err := json.Unmarshal(tagsRaw, &entry.Tags); err != nil {
				return nil, fmt.Errorf("unmarshaling tags: %w", err)
			}
		}
		if entry.Tags == nil {
			entry.Tags = []string{}
		}

		entry.Annotations = map[string]string{}
		if len(annotationsRaw) > 0 {
			if err := json.Unmarshal(annotationsRaw, &entry.Annotations); err != nil {
				return nil, fmt.Errorf("unmarshaling annotations: %w", err)
			}
		}
		if entry.Annotations == nil {
			entry.Annotations = map[string]string{}
		}

		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating service catalog entries: %w", err)
	}

	return entries, nil
}

// Update updates an existing ServiceEntry.
func (r *catalogRepo) Update(ctx context.Context, entry *catalog.ServiceEntry) error {
	if entry == nil {
		return fmt.Errorf("service entry cannot be nil")
	}
	if entry.ID == "" {
		return fmt.Errorf("service entry ID cannot be empty")
	}

	if entry.Type == "" {
		entry.Type = catalog.TypeService
	}
	if entry.Lifecycle == "" {
		entry.Lifecycle = catalog.LifecycleDevelopment
	}
	if entry.Tags == nil {
		entry.Tags = []string{}
	}
	if entry.Annotations == nil {
		entry.Annotations = map[string]string{}
	}

	entry.UpdatedAt = time.Now().UTC()

	tagsJSON, err := json.Marshal(entry.Tags)
	if err != nil {
		return fmt.Errorf("marshaling tags: %w", err)
	}

	annotationsJSON, err := json.Marshal(entry.Annotations)
	if err != nil {
		return fmt.Errorf("marshaling annotations: %w", err)
	}

	query := `
		UPDATE service_catalog
		SET name = $1,
		    description = $2,
		    type = $3,
		    lifecycle = $4,
		    owner_team = $5,
		    owner_email = $6,
		    repo_url = $7,
		    docs_url = $8,
		    tags = $9,
		    annotations = $10,
		    updated_at = $11
		WHERE id = $12
	`
	query, args := BuildTenantQuery(ctx, query,
		entry.Name,
		entry.Description,
		entry.Type,
		entry.Lifecycle,
		entry.OwnerTeam,
		entry.OwnerEmail,
		entry.RepoURL,
		entry.DocsURL,
		tagsJSON,
		annotationsJSON,
		entry.UpdatedAt,
		entry.ID,
	)

	cmd, err := r.getDB(ctx).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("updating service catalog entry: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("service catalog entry not found: %s", entry.ID)
	}

	return nil
}

// Delete removes a ServiceEntry by ID.
func (r *catalogRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM service_catalog WHERE id = $1`
	query, args := BuildTenantQuery(ctx, query, id)

	cmd, err := r.getDB(ctx).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("deleting service catalog entry: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("service catalog entry not found: %s", id)
	}

	return nil
}

// Stats returns aggregate metrics for the catalog scoped to a tenant.
func (r *catalogRepo) Stats(ctx context.Context, tenantID string) (*catalog.CatalogStats, error) {
	if tenantID != "" && tenancy.TenantIDFromContext(ctx) == "" {
		ctx = context.WithValue(ctx, tenancy.TenantIDKey, tenantID)
	}

	stats := &catalog.CatalogStats{
		ByType:      make(map[string]int),
		ByLifecycle: make(map[string]int),
	}

	userRole := tenancy.UserRoleFromContext(ctx)

	// 1. Total count
	var totalQuery string
	var totalArgs []any
	if userRole == "platform_admin" && tenantID != "" {
		totalQuery = `SELECT COUNT(*) FROM service_catalog WHERE tenant_id = $1`
		totalArgs = []any{tenantID}
	} else {
		totalQuery = `SELECT COUNT(*) FROM service_catalog`
	}
	totalQuery, totalArgs = BuildTenantQuery(ctx, totalQuery, totalArgs...)

	err := r.getDB(ctx).QueryRow(ctx, totalQuery, totalArgs...).Scan(&stats.Total)
	if err != nil {
		return nil, fmt.Errorf("getting catalog total count: %w", err)
	}

	// 2. Count by type
	var typeQuery string
	var typeArgs []any
	if userRole == "platform_admin" && tenantID != "" {
		typeQuery = `SELECT type, COUNT(*) FROM service_catalog WHERE tenant_id = $1 GROUP BY type`
		typeArgs = []any{tenantID}
	} else {
		typeQuery = `SELECT type, COUNT(*) FROM service_catalog GROUP BY type`
	}
	typeQuery, typeArgs = BuildTenantQuery(ctx, typeQuery, typeArgs...)

	typeRows, err := r.getDB(ctx).Query(ctx, typeQuery, typeArgs...)
	if err != nil {
		return nil, fmt.Errorf("getting catalog stats by type: %w", err)
	}
	defer typeRows.Close()

	for typeRows.Next() {
		var serviceType string
		var count int
		if err := typeRows.Scan(&serviceType, &count); err != nil {
			return nil, fmt.Errorf("scanning stats by type: %w", err)
		}
		stats.ByType[serviceType] = count
	}
	if err := typeRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating stats by type: %w", err)
	}

	// 3. Count by lifecycle
	var lifecycleQuery string
	var lifecycleArgs []any
	if userRole == "platform_admin" && tenantID != "" {
		lifecycleQuery = `SELECT lifecycle, COUNT(*) FROM service_catalog WHERE tenant_id = $1 GROUP BY lifecycle`
		lifecycleArgs = []any{tenantID}
	} else {
		lifecycleQuery = `SELECT lifecycle, COUNT(*) FROM service_catalog GROUP BY lifecycle`
	}
	lifecycleQuery, lifecycleArgs = BuildTenantQuery(ctx, lifecycleQuery, lifecycleArgs...)

	lifecycleRows, err := r.getDB(ctx).Query(ctx, lifecycleQuery, lifecycleArgs...)
	if err != nil {
		return nil, fmt.Errorf("getting catalog stats by lifecycle: %w", err)
	}
	defer lifecycleRows.Close()

	for lifecycleRows.Next() {
		var stage string
		var count int
		if err := lifecycleRows.Scan(&stage, &count); err != nil {
			return nil, fmt.Errorf("scanning stats by lifecycle: %w", err)
		}
		stats.ByLifecycle[stage] = count
	}
	if err := lifecycleRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating stats by lifecycle: %w", err)
	}

	return stats, nil
}

var _ catalog.Repository = (*catalogRepo)(nil)
