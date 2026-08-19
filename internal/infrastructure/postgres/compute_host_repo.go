package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/pkg/crypto"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

type computeHostRepo struct {
	pool DBTX
}

// NewComputeHostRepo creates a new PostgreSQL-backed Compute Host repository.
func NewComputeHostRepo(pool DBTX) docker.ComputeHostRepository {
	return &computeHostRepo{pool: pool}
}

func (r *computeHostRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.pool)
}

// Create persists a new ComputeHost with TLS private key encrypted at rest.
func (r *computeHostRepo) Create(ctx context.Context, host *docker.ComputeHost) error {
	if host == nil {
		return fmt.Errorf("compute host cannot be nil")
	}

	if host.ID == "" {
		host.ID = uuid.NewString()
	}

	tenantID := tenancy.TenantIDFromContext(ctx)
	userRole := tenancy.UserRoleFromContext(ctx)
	if userRole != "platform_admin" && tenantID != "" {
		host.TenantID = tenantID
	}
	if host.TenantID == "" {
		host.TenantID = "default-tenant"
	}

	if host.HostType == "" {
		host.HostType = "docker"
	}

	if host.Status == "" {
		host.Status = "pending"
	}

	if host.Labels == nil {
		host.Labels = make(map[string]string)
	}

	now := time.Now().UTC()
	if host.CreatedAt.IsZero() {
		host.CreatedAt = now
	}
	host.UpdatedAt = now

	var encKey string
	if host.TLSKey != "" {
		encrypted, err := crypto.Encrypt(host.TLSKey)
		if err != nil {
			return fmt.Errorf("encrypting compute host TLS key: %w", err)
		}
		encKey = encrypted
	}

	labelsJSON, err := json.Marshal(host.Labels)
	if err != nil {
		return fmt.Errorf("marshaling compute host labels: %w", err)
	}

	query := `
		INSERT INTO compute_hosts (id, name, host_type, endpoint, tls_enabled, tls_ca, tls_cert, tls_key, api_version, status, last_health_check, labels, tenant_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`

	_, err = r.getDB(ctx).Exec(ctx, query,
		host.ID,
		host.Name,
		host.HostType,
		host.Endpoint,
		host.TLSEnabled,
		host.TLSCA,
		host.TLSCert,
		encKey,
		host.APIVersion,
		host.Status,
		host.LastHealthCheck,
		labelsJSON,
		host.TenantID,
		host.CreatedAt,
		host.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("creating compute host: %w", err)
	}

	return nil
}

// GetByID retrieves a ComputeHost by ID and decrypts stored TLS private key.
func (r *computeHostRepo) GetByID(ctx context.Context, id string) (*docker.ComputeHost, error) {
	query := `
		SELECT id, name, host_type, endpoint, tls_enabled, tls_ca, tls_cert, tls_key, api_version, status, last_health_check, labels, tenant_id, created_at, updated_at
		FROM compute_hosts
		WHERE id = $1
	`
	query, args := BuildTenantQuery(ctx, query, id)

	var host docker.ComputeHost
	var encKey string
	var labelsBytes []byte

	err := r.getDB(ctx).QueryRow(ctx, query, args...).Scan(
		&host.ID,
		&host.Name,
		&host.HostType,
		&host.Endpoint,
		&host.TLSEnabled,
		&host.TLSCA,
		&host.TLSCert,
		&encKey,
		&host.APIVersion,
		&host.Status,
		&host.LastHealthCheck,
		&labelsBytes,
		&host.TenantID,
		&host.CreatedAt,
		&host.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("getting compute host by id: %w", err)
	}

	if encKey != "" {
		decrypted, err := crypto.Decrypt(encKey)
		if err != nil {
			return nil, fmt.Errorf("decrypting compute host TLS key: %w", err)
		}
		host.TLSKey = decrypted
	}

	if len(labelsBytes) > 0 {
		if err := json.Unmarshal(labelsBytes, &host.Labels); err != nil {
			host.Labels = make(map[string]string)
		}
	} else {
		host.Labels = make(map[string]string)
	}

	return &host, nil
}

// List returns compute hosts matching tenantID.
func (r *computeHostRepo) List(ctx context.Context, tenantID string) ([]docker.ComputeHost, error) {
	var query string
	var args []any

	if tenantID != "" {
		query = `
			SELECT id, name, host_type, endpoint, tls_enabled, tls_ca, tls_cert, tls_key, api_version, status, last_health_check, labels, tenant_id, created_at, updated_at
			FROM compute_hosts
			WHERE tenant_id = $1
			ORDER BY name ASC
		`
		args = []any{tenantID}
	} else {
		query = `
			SELECT id, name, host_type, endpoint, tls_enabled, tls_ca, tls_cert, tls_key, api_version, status, last_health_check, labels, tenant_id, created_at, updated_at
			FROM compute_hosts
			ORDER BY name ASC
		`
	}

	query, args = BuildTenantQuery(ctx, query, args...)

	rows, err := r.getDB(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing compute hosts: %w", err)
	}
	defer rows.Close()

	return scanComputeHosts(rows)
}

// ListAll returns all compute hosts across all tenants without tenant filtering.
func (r *computeHostRepo) ListAll(ctx context.Context) ([]docker.ComputeHost, error) {
	query := `
		SELECT id, name, host_type, endpoint, tls_enabled, tls_ca, tls_cert, tls_key, api_version, status, last_health_check, labels, tenant_id, created_at, updated_at
		FROM compute_hosts
		ORDER BY name ASC
	`

	rows, err := r.getDB(ctx).Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing all compute hosts: %w", err)
	}
	defer rows.Close()

	return scanComputeHosts(rows)
}

func scanComputeHosts(rows pgx.Rows) ([]docker.ComputeHost, error) {
	hosts := make([]docker.ComputeHost, 0)
	for rows.Next() {
		var host docker.ComputeHost
		var encKey string
		var labelsBytes []byte

		if err := rows.Scan(
			&host.ID,
			&host.Name,
			&host.HostType,
			&host.Endpoint,
			&host.TLSEnabled,
			&host.TLSCA,
			&host.TLSCert,
			&encKey,
			&host.APIVersion,
			&host.Status,
			&host.LastHealthCheck,
			&labelsBytes,
			&host.TenantID,
			&host.CreatedAt,
			&host.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning compute host: %w", err)
		}

		if len(labelsBytes) > 0 {
			if err := json.Unmarshal(labelsBytes, &host.Labels); err != nil {
				host.Labels = make(map[string]string)
			}
		} else {
			host.Labels = make(map[string]string)
		}

		// Mask TLSKey in list queries
		host.TLSKey = ""

		hosts = append(hosts, host)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating compute hosts: %w", err)
	}

	return hosts, nil
}

// Update updates an existing ComputeHost.
func (r *computeHostRepo) Update(ctx context.Context, host *docker.ComputeHost) error {
	if host == nil {
		return fmt.Errorf("compute host cannot be nil")
	}

	host.UpdatedAt = time.Now().UTC()

	var encKey string
	if host.TLSKey != "" {
		var err error
		encKey, err = crypto.Encrypt(host.TLSKey)
		if err != nil {
			return fmt.Errorf("encrypting compute host TLS key: %w", err)
		}
	}

	if host.Labels == nil {
		host.Labels = make(map[string]string)
	}
	labelsJSON, err := json.Marshal(host.Labels)
	if err != nil {
		return fmt.Errorf("marshaling compute host labels: %w", err)
	}

	query := `
		UPDATE compute_hosts
		SET name = $1,
		    host_type = $2,
		    endpoint = $3,
		    tls_enabled = $4,
		    tls_ca = $5,
		    tls_cert = $6,
		    tls_key = CASE WHEN $7 = '' THEN tls_key ELSE $7 END,
		    api_version = $8,
		    status = $9,
		    labels = $10,
		    updated_at = $11
		WHERE id = $12
	`

	query, args := BuildTenantQuery(ctx, query,
		host.Name,
		host.HostType,
		host.Endpoint,
		host.TLSEnabled,
		host.TLSCA,
		host.TLSCert,
		encKey,
		host.APIVersion,
		host.Status,
		labelsJSON,
		host.UpdatedAt,
		host.ID,
	)

	cmd, err := r.getDB(ctx).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("updating compute host: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("compute host not found: %s", host.ID)
	}

	return nil
}

// Delete removes a ComputeHost by ID.
func (r *computeHostRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM compute_hosts WHERE id = $1`
	query, args := BuildTenantQuery(ctx, query, id)

	cmd, err := r.getDB(ctx).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("deleting compute host: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("compute host not found: %s", id)
	}

	return nil
}

// UpdateStatus updates the operational status and last health check timestamp of a compute host.
func (r *computeHostRepo) UpdateStatus(ctx context.Context, id string, status string, lastHealthCheck time.Time) error {
	now := time.Now().UTC()
	query := `
		UPDATE compute_hosts
		SET status = $1,
		    last_health_check = $2,
		    updated_at = $3
		WHERE id = $4
	`
	query, args := BuildTenantQuery(ctx, query, status, lastHealthCheck, now, id)

	cmd, err := r.getDB(ctx).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("updating compute host status: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("compute host not found: %s", id)
	}

	return nil
}
