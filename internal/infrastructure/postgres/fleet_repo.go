package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/fleet"
	"github.com/datdt/k8sselfhost/internal/pkg/crypto"
)

type fleetRepo struct {
	pool DBTX
}

// NewFleetRepo creates a new PostgreSQL fleet repository.
func NewFleetRepo(pool DBTX) fleet.Repository {
	return &fleetRepo{pool: pool}
}

func (r *fleetRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.pool)
}

func (r *fleetRepo) ListClusters(ctx context.Context) ([]fleet.Cluster, error) {
	query := `
		SELECT id, name, "group", region, provider, status, version, nodes, encrypted_token, tenant_id, created_at, updated_at
		FROM fleet_clusters
		ORDER BY name ASC
	`
	
	query, args := BuildTenantQuery(ctx, query)

	rows, err := r.getDB(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing fleet clusters: %w", err)
	}
	defer rows.Close()

	var clusters []fleet.Cluster
	for rows.Next() {
		var c fleet.Cluster
		var encryptedToken *string
		if err := rows.Scan(
			&c.ID, &c.Name, &c.Group, &c.Region, &c.Provider,
			&c.Status, &c.Version, &c.Nodes, &encryptedToken, &c.TenantID, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning fleet cluster: %w", err)
		}
		if encryptedToken != nil && *encryptedToken != "" {
			c.EncryptedToken, _ = crypto.Decrypt(*encryptedToken)
		}
		clusters = append(clusters, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return clusters, nil
}

func (r *fleetRepo) GetCluster(ctx context.Context, id string) (*fleet.Cluster, error) {
	query := `
		SELECT id, name, "group", region, provider, status, version, nodes, encrypted_token, tenant_id, created_at, updated_at
		FROM fleet_clusters
		WHERE id = $1
	`
	query, args := BuildTenantQuery(ctx, query, id)

	var c fleet.Cluster
	var encryptedToken *string
	err := r.getDB(ctx).QueryRow(ctx, query, args...).Scan(
		&c.ID, &c.Name, &c.Group, &c.Region, &c.Provider,
		&c.Status, &c.Version, &c.Nodes, &encryptedToken, &c.TenantID, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("getting fleet cluster: %w", err)
	}
	if encryptedToken != nil && *encryptedToken != "" {
		c.EncryptedToken, _ = crypto.Decrypt(*encryptedToken)
	}
	return &c, nil
}

func (r *fleetRepo) RegisterCluster(ctx context.Context, c *fleet.Cluster) error {
	now := time.Now().UTC()
	encToken, err := crypto.Encrypt(c.EncryptedToken)
	if err != nil {
		return fmt.Errorf("encrypting token: %w", err)
	}

	tenantID := middleware.TenantIDFromContext(ctx)
	userRole := middleware.UserRoleFromContext(ctx)
	if userRole != "platform_admin" && tenantID != "" {
		c.TenantID = tenantID
	}
	if c.TenantID == "" {
		c.TenantID = "default-tenant"
	}

	query := `
		INSERT INTO fleet_clusters (id, name, "group", region, provider, status, version, nodes, encrypted_token, tenant_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			"group" = EXCLUDED."group",
			region = EXCLUDED.region,
			provider = EXCLUDED.provider,
			status = EXCLUDED.status,
			version = EXCLUDED.version,
			nodes = EXCLUDED.nodes,
			encrypted_token = EXCLUDED.encrypted_token,
			tenant_id = EXCLUDED.tenant_id,
			updated_at = EXCLUDED.updated_at
	`
	_, err = r.getDB(ctx).Exec(ctx, query,
		c.ID, c.Name, c.Group, c.Region, c.Provider,
		c.Status, c.Version, c.Nodes, encToken, c.TenantID, now, now,
	)
	if err != nil {
		return fmt.Errorf("registering fleet cluster: %w", err)
	}
	return nil
}

func (r *fleetRepo) UpdateClusterStatus(ctx context.Context, id, status, version string, nodes int) error {
	query := `
		UPDATE fleet_clusters
		SET status = $2, version = $3, nodes = $4, updated_at = $5
		WHERE id = $1
	`
	query, args := BuildTenantQuery(ctx, query, id, status, version, nodes, time.Now().UTC())

	_, err := r.getDB(ctx).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("updating fleet cluster status: %w", err)
	}
	return nil
}

func (r *fleetRepo) RemoveCluster(ctx context.Context, id string) error {
	query := `DELETE FROM fleet_clusters WHERE id = $1`
	query, args := BuildTenantQuery(ctx, query, id)

	_, err := r.getDB(ctx).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("removing fleet cluster: %w", err)
	}
	return nil
}
