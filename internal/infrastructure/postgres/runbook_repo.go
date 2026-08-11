package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/runbook"
)

type runbookRepo struct {
	db DBTX
}

// NewRunbookRepo creates a new Postgres-backed Runbook repository.
func NewRunbookRepo(db DBTX) runbook.Repository {
	return &runbookRepo{db: db}
}

func (r *runbookRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.db)
}

func (r *runbookRepo) List(ctx context.Context, category string, limit, offset int) ([]runbook.Runbook, int, error) {
	var query string
	var countQuery string
	var args []interface{}
	var countArgs []interface{}

	if category != "" && category != "all" {
		query = `
			SELECT id, title, category, content, tags, author, steps_count, last_used_at, tenant_id, created_at, updated_at 
			FROM runbooks 
			WHERE category = $1 
			ORDER BY created_at DESC 
			LIMIT $2 OFFSET $3
		`
		countQuery = `SELECT COUNT(*) FROM runbooks WHERE category = $1`
		args = []interface{}{category, limit, offset}
		countArgs = []interface{}{category}
	} else {
		query = `
			SELECT id, title, category, content, tags, author, steps_count, last_used_at, tenant_id, created_at, updated_at 
			FROM runbooks 
			ORDER BY created_at DESC 
			LIMIT $1 OFFSET $2
		`
		countQuery = `SELECT COUNT(*) FROM runbooks`
		args = []interface{}{limit, offset}
	}
	
	query, args = BuildTenantQuery(ctx, query, args...)
	countQuery, countArgs = BuildTenantQuery(ctx, countQuery, countArgs...)

	var total int
	err := r.getDB(ctx).QueryRow(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting runbooks: %w", err)
	}

	rows, err := r.getDB(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("querying runbooks: %w", err)
	}
	defer rows.Close()

	var runbooks []runbook.Runbook
	for rows.Next() {
		var rb runbook.Runbook
		if err := rows.Scan(
			&rb.ID, &rb.Title, &rb.Category, &rb.Content, &rb.Tags,
			&rb.Author, &rb.StepsCount, &rb.LastUsedAt, &rb.TenantID, &rb.CreatedAt, &rb.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scanning runbook: %w", err)
		}
		runbooks = append(runbooks, rb)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating runbooks: %w", err)
	}

	return runbooks, total, nil
}

func (r *runbookRepo) GetByID(ctx context.Context, id string) (*runbook.Runbook, error) {
	query := `
		SELECT id, title, category, content, tags, author, steps_count, last_used_at, tenant_id, created_at, updated_at 
		FROM runbooks 
		WHERE id = $1
	`
	query, args := BuildTenantQuery(ctx, query, id)

	var rb runbook.Runbook
	err := r.getDB(ctx).QueryRow(ctx, query, args...).Scan(
		&rb.ID, &rb.Title, &rb.Category, &rb.Content, &rb.Tags,
		&rb.Author, &rb.StepsCount, &rb.LastUsedAt, &rb.TenantID, &rb.CreatedAt, &rb.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("runbook not found")
		}
		return nil, fmt.Errorf("querying runbook: %w", err)
	}
	return &rb, nil
}

func (r *runbookRepo) Create(ctx context.Context, rb *runbook.Runbook) error {
	tenantID := middleware.TenantIDFromContext(ctx)
	userRole := middleware.UserRoleFromContext(ctx)
	if userRole != "platform_admin" && tenantID != "" {
		rb.TenantID = tenantID
	}
	if rb.TenantID == "" {
		rb.TenantID = "default-tenant"
	}

	query := `
		INSERT INTO runbooks (title, category, content, tags, author, steps_count, tenant_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`
	err := r.getDB(ctx).QueryRow(ctx, query,
		rb.Title, rb.Category, rb.Content, rb.Tags, rb.Author, rb.StepsCount, rb.TenantID, rb.CreatedAt, rb.UpdatedAt,
	).Scan(&rb.ID)
	
	if err != nil {
		return fmt.Errorf("inserting runbook: %w", err)
	}
	return nil
}

func (r *runbookRepo) Update(ctx context.Context, rb *runbook.Runbook) error {
	rb.UpdatedAt = time.Now()
	query := `
		UPDATE runbooks 
		SET title = $1, category = $2, content = $3, tags = $4, author = $5, steps_count = $6, updated_at = $7
		WHERE id = $8
	`
	query, args := BuildTenantQuery(ctx, query, rb.Title, rb.Category, rb.Content, rb.Tags, rb.Author, rb.StepsCount, rb.UpdatedAt, rb.ID)

	cmd, err := r.getDB(ctx).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("updating runbook: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("runbook not found")
	}
	return nil
}

func (r *runbookRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM runbooks WHERE id = $1`
	query, args := BuildTenantQuery(ctx, query, id)

	cmd, err := r.getDB(ctx).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("deleting runbook: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("runbook not found")
	}
	return nil
}
