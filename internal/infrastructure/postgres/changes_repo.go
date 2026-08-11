package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/datdt/k8sselfhost/internal/domain/changes"
)

type changesRepo struct {
	db DBTX
}

// NewChangesRepo creates a new Postgres-backed Changes repository.
func NewChangesRepo(db DBTX) changes.Repository {
	return &changesRepo{db: db}
}

func (r *changesRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.db)
}

func (r *changesRepo) ListRequests(ctx context.Context, status *changes.ChangeStatus, limit, offset int) ([]changes.ChangeRequest, int, error) {
	var query string
	var countQuery string
	var args []interface{}
	var countArgs []interface{}
	
	if status != nil {
		query = `
			SELECT id, title, description, type, status, requester, approver, cluster, namespace, resource, scheduled_at, approved_at, created_at, updated_at 
			FROM change_requests 
			WHERE status = $1 
			ORDER BY created_at DESC 
			LIMIT $2 OFFSET $3
		`
		countQuery = `SELECT COUNT(*) FROM change_requests WHERE status = $1`
		args = []interface{}{string(*status), limit, offset}
		countArgs = []interface{}{string(*status)}
	} else {
		query = `
			SELECT id, title, description, type, status, requester, approver, cluster, namespace, resource, scheduled_at, approved_at, created_at, updated_at 
			FROM change_requests 
			ORDER BY created_at DESC 
			LIMIT $1 OFFSET $2
		`
		countQuery = `SELECT COUNT(*) FROM change_requests`
		args = []interface{}{limit, offset}
	}

	var total int
	err := r.getDB(ctx).QueryRow(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting change requests: %w", err)
	}

	rows, err := r.getDB(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("querying change requests: %w", err)
	}
	defer rows.Close()

	var reqs []changes.ChangeRequest
	for rows.Next() {
		var req changes.ChangeRequest
		var approver *string
		
		if err := rows.Scan(
			&req.ID, &req.Title, &req.Description, &req.Type, &req.Status, &req.Requester, 
			&approver, &req.Cluster, &req.Namespace, &req.Resource, 
			&req.ScheduledAt, &req.ApprovedAt, &req.CreatedAt, &req.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scanning change request: %w", err)
		}
		
		if approver != nil {
			req.Approver = *approver
		}

		reqs = append(reqs, req)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating change requests: %w", err)
	}

	return reqs, total, nil
}

func (r *changesRepo) GetRequest(ctx context.Context, id string) (*changes.ChangeRequest, error) {
	query := `
		SELECT id, title, description, type, status, requester, approver, cluster, namespace, resource, scheduled_at, approved_at, created_at, updated_at 
		FROM change_requests 
		WHERE id = $1
	`
	var req changes.ChangeRequest
	var approver *string

	err := r.getDB(ctx).QueryRow(ctx, query, id).Scan(
		&req.ID, &req.Title, &req.Description, &req.Type, &req.Status, &req.Requester, 
		&approver, &req.Cluster, &req.Namespace, &req.Resource, 
		&req.ScheduledAt, &req.ApprovedAt, &req.CreatedAt, &req.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("change request not found")
		}
		return nil, fmt.Errorf("querying change request: %w", err)
	}

	if approver != nil {
		req.Approver = *approver
	}

	return &req, nil
}

func (r *changesRepo) CreateRequest(ctx context.Context, req *changes.ChangeRequest) error {
	query := `
		INSERT INTO change_requests (title, description, type, status, requester, cluster, namespace, resource, scheduled_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id
	`
	err := r.getDB(ctx).QueryRow(ctx, query,
		req.Title, req.Description, string(req.Type), string(req.Status), req.Requester, 
		req.Cluster, req.Namespace, req.Resource, req.ScheduledAt, req.CreatedAt, req.UpdatedAt,
	).Scan(&req.ID)
	
	if err != nil {
		return fmt.Errorf("inserting change request: %w", err)
	}
	
	return nil
}

func (r *changesRepo) ApproveRequest(ctx context.Context, id, approver string) error {
	query := `
		UPDATE change_requests 
		SET status = 'approved', approver = $1, approved_at = NOW(), updated_at = NOW() 
		WHERE id = $2 AND status = 'pending'
	`
	cmd, err := r.getDB(ctx).Exec(ctx, query, approver, id)
	if err != nil {
		return fmt.Errorf("approving change request: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("change request not found or not in pending status")
	}
	return nil
}

func (r *changesRepo) RejectRequest(ctx context.Context, id, approver string) error {
	query := `
		UPDATE change_requests 
		SET status = 'rejected', approver = $1, updated_at = NOW() 
		WHERE id = $2 AND status = 'pending'
	`
	cmd, err := r.getDB(ctx).Exec(ctx, query, approver, id)
	if err != nil {
		return fmt.Errorf("rejecting change request: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("change request not found or not in pending status")
	}
	return nil
}

func (r *changesRepo) ListWindows(ctx context.Context) ([]changes.MaintenanceWindow, error) {
	query := `
		SELECT id, title, cluster, start_at, end_at, active, created_at 
		FROM maintenance_windows 
		ORDER BY start_at DESC
	`
	rows, err := r.getDB(ctx).Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying maintenance windows: %w", err)
	}
	defer rows.Close()

	var windows []changes.MaintenanceWindow
	for rows.Next() {
		var w changes.MaintenanceWindow
		if err := rows.Scan(&w.ID, &w.Title, &w.Cluster, &w.StartAt, &w.EndAt, &w.Active, &w.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning maintenance window: %w", err)
		}
		windows = append(windows, w)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating maintenance windows: %w", err)
	}

	return windows, nil
}

func (r *changesRepo) CreateWindow(ctx context.Context, w *changes.MaintenanceWindow) error {
	query := `
		INSERT INTO maintenance_windows (title, cluster, start_at, end_at, active, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`
	err := r.getDB(ctx).QueryRow(ctx, query,
		w.Title, w.Cluster, w.StartAt, w.EndAt, w.Active, w.CreatedAt,
	).Scan(&w.ID)
	
	if err != nil {
		return fmt.Errorf("inserting maintenance window: %w", err)
	}
	
	return nil
}
