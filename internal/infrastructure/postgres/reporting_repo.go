package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/datdt/k8sselfhost/internal/domain/reporting"
)

type reportingRepo struct {
	db DBTX
}

// NewReportingRepo creates a new Postgres-backed Reporting repository.
func NewReportingRepo(db DBTX) reporting.Repository {
	return &reportingRepo{db: db}
}

func (r *reportingRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.db)
}

func (r *reportingRepo) ListReports(ctx context.Context, limit, offset int) ([]reporting.Report, int, error) {
	query := `
		SELECT id, title, type, format, status, file_url, created_by, created_at, expires_at 
		FROM reports 
		ORDER BY created_at DESC 
		LIMIT $1 OFFSET $2
	`
	countQuery := `SELECT COUNT(*) FROM reports`

	var total int
	err := r.getDB(ctx).QueryRow(ctx, countQuery).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting reports: %w", err)
	}

	rows, err := r.getDB(ctx).Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("querying reports: %w", err)
	}
	defer rows.Close()

	var reps []reporting.Report
	for rows.Next() {
		var rep reporting.Report
		if err := rows.Scan(
			&rep.ID, &rep.Title, &rep.Type, &rep.Format, &rep.Status, 
			&rep.FileURL, &rep.CreatedBy, &rep.CreatedAt, &rep.ExpiresAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scanning report: %w", err)
		}
		reps = append(reps, rep)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating reports: %w", err)
	}

	return reps, total, nil
}

func (r *reportingRepo) GetReport(ctx context.Context, id string) (*reporting.Report, error) {
	query := `
		SELECT id, title, type, format, status, file_url, created_by, created_at, expires_at 
		FROM reports 
		WHERE id = $1
	`
	var rep reporting.Report
	err := r.getDB(ctx).QueryRow(ctx, query, id).Scan(
		&rep.ID, &rep.Title, &rep.Type, &rep.Format, &rep.Status, 
		&rep.FileURL, &rep.CreatedBy, &rep.CreatedAt, &rep.ExpiresAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("report not found")
		}
		return nil, fmt.Errorf("querying report: %w", err)
	}
	return &rep, nil
}

func (r *reportingRepo) CreateReport(ctx context.Context, rep *reporting.Report) error {
	query := `
		INSERT INTO reports (title, type, format, status, created_by, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`
	err := r.getDB(ctx).QueryRow(ctx, query,
		rep.Title, rep.Type, rep.Format, rep.Status, rep.CreatedBy, rep.CreatedAt, rep.ExpiresAt,
	).Scan(&rep.ID)
	
	if err != nil {
		return fmt.Errorf("inserting report: %w", err)
	}
	
	return nil
}

func (r *reportingRepo) UpdateStatus(ctx context.Context, id, status, fileURL string) error {
	query := `
		UPDATE reports 
		SET status = $1, file_url = $2 
		WHERE id = $3
	`
	cmd, err := r.getDB(ctx).Exec(ctx, query, status, fileURL, id)
	if err != nil {
		return fmt.Errorf("updating report status: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("report not found")
	}
	return nil
}

func (r *reportingRepo) DeleteReport(ctx context.Context, id string) error {
	query := `DELETE FROM reports WHERE id = $1`
	cmd, err := r.getDB(ctx).Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting report: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("report not found")
	}
	return nil
}
