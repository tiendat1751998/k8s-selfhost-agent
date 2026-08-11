// Package postgres provides PostgreSQL repository implementations.
package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// IncidentRepo implements incident.Repository using PostgreSQL.
type IncidentRepo struct {
	pool DBTX
}

// NewIncidentRepo creates a new PostgreSQL-backed incident repository.
func NewIncidentRepo(pool DBTX) *IncidentRepo {
	return &IncidentRepo{pool: pool}
}

func (r *IncidentRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.pool)
}

// Create persists a new incident and assigns a generated UUID.
func (r *IncidentRepo) Create(ctx context.Context, inc *incident.Incident) error {
	rawData, err := json.Marshal(inc.RawData)
	if err != nil {
		return errors.Wrap(err, "marshaling raw data")
	}

	tenantID := middleware.TenantIDFromContext(ctx)
	if tenantID == "" {
		tenantID = "org-google"
	}

	query := `
		INSERT INTO incidents (cluster_name, namespace, pod_name, type, status, severity, message, raw_data, tenant_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id`

	err = r.getDB(ctx).QueryRow(ctx, query,
		inc.ClusterName,
		inc.Namespace,
		inc.PodName,
		string(inc.Type),
		string(inc.Status),
		string(inc.Severity),
		inc.Message,
		rawData,
		tenantID,
		inc.CreatedAt,
		inc.UpdatedAt,
	).Scan(&inc.ID)

	if err != nil {
		return errors.Wrap(err, "inserting incident")
	}

	return nil
}

// GetByID retrieves an incident by its unique identifier.
func (r *IncidentRepo) GetByID(ctx context.Context, id string) (*incident.Incident, error) {
	query := `
		SELECT id, cluster_name, namespace, pod_name, type, status, severity, message, raw_data, created_at, updated_at, resolved_at
		FROM incidents
		WHERE id = $1`

	query, args := BuildTenantQuery(ctx, query, id)
	inc, err := r.scanIncident(r.getDB(ctx).QueryRow(ctx, query, args...))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errors.NewNotFound("incident", id)
		}
		return nil, errors.Wrap(err, "querying incident by id")
	}

	return inc, nil
}

// Update saves changes to an existing incident.
func (r *IncidentRepo) Update(ctx context.Context, inc *incident.Incident) error {
	rawData, err := json.Marshal(inc.RawData)
	if err != nil {
		return errors.Wrap(err, "marshaling raw data")
	}

	inc.UpdatedAt = time.Now().UTC()

	query := `
		UPDATE incidents
		SET status = $1, severity = $2, message = $3, raw_data = $4, updated_at = $5, resolved_at = $6
		WHERE id = $7`

	tag, err := r.getDB(ctx).Exec(ctx, query,
		string(inc.Status),
		string(inc.Severity),
		inc.Message,
		rawData,
		inc.UpdatedAt,
		inc.ResolvedAt,
		inc.ID,
	)
	if err != nil {
		return errors.Wrap(err, "updating incident")
	}

	if tag.RowsAffected() == 0 {
		return errors.NewNotFound("incident", inc.ID)
	}

	return nil
}

// List retrieves incidents matching the given filter with pagination.
func (r *IncidentRepo) List(ctx context.Context, filter incident.Filter) ([]*incident.Incident, int64, error) {
	baseQuery := "FROM incidents WHERE 1=1"
	args := []interface{}{}
	argIdx := 1

	if filter.ClusterName != "" {
		baseQuery += fmt.Sprintf(" AND cluster_name = $%d", argIdx)
		args = append(args, filter.ClusterName)
		argIdx++
	}
	if filter.Namespace != "" {
		baseQuery += fmt.Sprintf(" AND namespace = $%d", argIdx)
		args = append(args, filter.Namespace)
		argIdx++
	}
	if filter.Status != nil {
		baseQuery += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, string(*filter.Status))
		argIdx++
	}
	if filter.Type != nil {
		baseQuery += fmt.Sprintf(" AND type = $%d", argIdx)
		args = append(args, string(*filter.Type))
		argIdx++
	}
	if filter.Severity != nil {
		baseQuery += fmt.Sprintf(" AND severity = $%d", argIdx)
		args = append(args, string(*filter.Severity))
		argIdx++
	}

	// Count total
	var total int64
	countQuery := "SELECT COUNT(*) " + baseQuery
	countQuery, countArgs := BuildTenantQuery(ctx, countQuery, args...)
	if err := r.getDB(ctx).QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, errors.Wrap(err, "counting incidents")
	}

	// Apply pagination
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	selectQuery := fmt.Sprintf(
		"SELECT id, cluster_name, namespace, pod_name, type, status, severity, message, raw_data, created_at, updated_at, resolved_at %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d",
		baseQuery, argIdx, argIdx+1,
	)
	selectArgs := append(args, limit, offset)
	selectQuery, selectArgs = BuildTenantQuery(ctx, selectQuery, selectArgs...)

	rows, err := r.getDB(ctx).Query(ctx, selectQuery, selectArgs...)
	if err != nil {
		return nil, 0, errors.Wrap(err, "querying incidents")
	}
	defer rows.Close()

	var incidents []*incident.Incident
	for rows.Next() {
		inc, scanErr := r.scanIncidentFromRows(rows)
		if scanErr != nil {
			return nil, 0, errors.Wrap(scanErr, "scanning incident row")
		}
		incidents = append(incidents, inc)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, errors.Wrap(err, "iterating incident rows")
	}

	return incidents, total, nil
}

// GetByPodAndType finds an active (non-resolved) incident for a specific pod and type.
func (r *IncidentRepo) GetByPodAndType(ctx context.Context, namespace, podName string, incidentType incident.Type) (*incident.Incident, error) {
	query := `
		SELECT id, cluster_name, namespace, pod_name, type, status, severity, message, raw_data, created_at, updated_at, resolved_at
		FROM incidents
		WHERE namespace = $1 AND pod_name = $2 AND type = $3 AND status != $4
		ORDER BY created_at DESC
		LIMIT 1`

	query, args := BuildTenantQuery(ctx, query, namespace, podName, string(incidentType), string(incident.StatusResolved))
	inc, err := r.scanIncident(r.getDB(ctx).QueryRow(ctx, query, args...))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, errors.Wrap(err, "querying incident by pod and type")
	}

	return inc, nil
}

func (r *IncidentRepo) scanIncident(row pgx.Row) (*incident.Incident, error) {
	var inc incident.Incident
	var rawData []byte
	var incType, status, severity string

	err := row.Scan(
		&inc.ID,
		&inc.ClusterName,
		&inc.Namespace,
		&inc.PodName,
		&incType,
		&status,
		&severity,
		&inc.Message,
		&rawData,
		&inc.CreatedAt,
		&inc.UpdatedAt,
		&inc.ResolvedAt,
	)
	if err != nil {
		return nil, err
	}

	inc.Type = incident.Type(incType)
	inc.Status = incident.Status(status)
	inc.Severity = incident.Severity(severity)

	if len(rawData) > 0 {
		if unmarshalErr := json.Unmarshal(rawData, &inc.RawData); unmarshalErr != nil {
			return nil, fmt.Errorf("unmarshaling raw data: %w", unmarshalErr)
		}
	}

	return &inc, nil
}

func (r *IncidentRepo) scanIncidentFromRows(rows pgx.Rows) (*incident.Incident, error) {
	var inc incident.Incident
	var rawData []byte
	var incType, status, severity string

	err := rows.Scan(
		&inc.ID,
		&inc.ClusterName,
		&inc.Namespace,
		&inc.PodName,
		&incType,
		&status,
		&severity,
		&inc.Message,
		&rawData,
		&inc.CreatedAt,
		&inc.UpdatedAt,
		&inc.ResolvedAt,
	)
	if err != nil {
		return nil, err
	}

	inc.Type = incident.Type(incType)
	inc.Status = incident.Status(status)
	inc.Severity = incident.Severity(severity)

	if len(rawData) > 0 {
		if unmarshalErr := json.Unmarshal(rawData, &inc.RawData); unmarshalErr != nil {
			return nil, fmt.Errorf("unmarshaling raw data: %w", unmarshalErr)
		}
	}

	return &inc, nil
}
