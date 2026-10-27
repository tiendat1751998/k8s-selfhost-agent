package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/datdt/k8sselfhost/internal/domain/correlation"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// CorrelationRepo implements correlation.Repository using PostgreSQL.
type CorrelationRepo struct {
	pool DBTX
}

// NewCorrelationRepo creates a new PostgreSQL-backed correlation repository.
func NewCorrelationRepo(pool DBTX) *CorrelationRepo {
	return &CorrelationRepo{pool: pool}
}

func (r *CorrelationRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.pool)
}

func (r *CorrelationRepo) ListCorrelated(ctx context.Context, status string, limit, offset int) ([]correlation.CorrelatedEvent, int, error) {
	ctx = EnsureTenantContext(ctx)
	query := `SELECT id, title, root_cause, severity, event_ids, event_count, cluster, namespace, status, correlated_at FROM correlated_events WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM correlated_events WHERE 1=1`
	args := []interface{}{}
	idx := 1

	if status != "" {
		query += fmt.Sprintf(" AND status = $%d", idx)
		countQuery += fmt.Sprintf(" AND status = $%d", idx)
		args = append(args, status)
		idx++
	}

	countQuery, countArgs := BuildTenantQuery(ctx, countQuery, args...)
	var total int
	if err := r.getDB(ctx).QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, errors.Wrap(err, "counting correlated events")
	}

	selectQuery := query + fmt.Sprintf(" ORDER BY correlated_at DESC LIMIT $%d OFFSET $%d", idx, idx+1)
	selectArgs := append(args, limit, offset)
	selectQuery, selectArgs = BuildTenantQuery(ctx, selectQuery, selectArgs...)

	rows, err := r.getDB(ctx).Query(ctx, selectQuery, selectArgs...)
	if err != nil {
		return nil, 0, errors.Wrap(err, "listing correlated events")
	}
	defer rows.Close()

	var events []correlation.CorrelatedEvent
	for rows.Next() {
		var e correlation.CorrelatedEvent
		var eventIDsJSON []byte
		if err := rows.Scan(&e.ID, &e.Title, &e.RootCause, &e.Severity, &eventIDsJSON, &e.EventCount, &e.Cluster, &e.Namespace, &e.Status, &e.CorrelatedAt); err != nil {
			return nil, 0, errors.Wrap(err, "scanning correlated event")
		}
		if err := json.Unmarshal(eventIDsJSON, &e.EventIDs); err != nil {
			return nil, 0, errors.Wrap(err, "unmarshaling event_ids")
		}
		events = append(events, e)
	}
	return events, total, nil
}

func (r *CorrelationRepo) GetByID(ctx context.Context, id string) (*correlation.CorrelatedEvent, error) {
	ctx = EnsureTenantContext(ctx)
	var e correlation.CorrelatedEvent
	var eventIDsJSON []byte
	query := `SELECT id, title, root_cause, severity, event_ids, event_count, cluster, namespace, status, correlated_at FROM correlated_events WHERE id = $1`
	query, args := BuildTenantQuery(ctx, query, id)
	err := r.getDB(ctx).QueryRow(ctx, query, args...).Scan(&e.ID, &e.Title, &e.RootCause, &e.Severity, &eventIDsJSON, &e.EventCount, &e.Cluster, &e.Namespace, &e.Status, &e.CorrelatedAt)
	if err != nil {
		return nil, errors.Wrap(err, "getting correlated event")
	}
	if err := json.Unmarshal(eventIDsJSON, &e.EventIDs); err != nil {
		return nil, errors.Wrap(err, "unmarshaling event_ids")
	}
	return &e, nil
}

func (r *CorrelationRepo) Create(ctx context.Context, c *correlation.CorrelatedEvent) error {
	ctx = EnsureTenantContext(ctx)
	eventIDsJSON, err := json.Marshal(c.EventIDs)
	if err != nil {
		return errors.Wrap(err, "marshaling event_ids")
	}

	tenantID := TenantIDOrDefault(ctx)

	query := `INSERT INTO correlated_events (title, root_cause, severity, event_ids, event_count, cluster, namespace, status, correlated_at, tenant_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id`

	err = r.getDB(ctx).QueryRow(ctx, query,
		c.Title, c.RootCause, c.Severity, eventIDsJSON, c.EventCount,
		c.Cluster, c.Namespace, c.Status, c.CorrelatedAt, tenantID,
	).Scan(&c.ID)
	if err != nil {
		return errors.Wrap(err, "inserting correlated event")
	}
	return nil
}

func (r *CorrelationRepo) Resolve(ctx context.Context, id string) error {
	ctx = EnsureTenantContext(ctx)
	query, args := BuildTenantQuery(ctx, `UPDATE correlated_events SET status = 'resolved' WHERE id = $1`, id)
	_, err := r.getDB(ctx).Exec(ctx, query, args...)
	if err != nil {
		return errors.Wrap(err, "resolving correlated event")
	}
	return nil
}

var _ correlation.Repository = (*CorrelationRepo)(nil)
