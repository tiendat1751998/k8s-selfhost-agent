package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/runbook"
)

type runbookRepo struct {
	db *pgxpool.Pool
}

// NewRunbookRepo creates a new Postgres-backed Runbook repository.
func NewRunbookRepo(db *pgxpool.Pool) runbook.Repository {
	return &runbookRepo{db: db}
}

func (r *runbookRepo) List(ctx context.Context, category string, limit, offset int) ([]runbook.Runbook, int, error) {
	tenantID := middleware.TenantIDFromContext(ctx)
	userRole := middleware.UserRoleFromContext(ctx)

	var query string
	var countQuery string
	var args []interface{}
	var countArgs []interface{}

	useTenantFilter := userRole != "platform_admin" && tenantID != ""
	
	if category != "" && category != "all" {
		if useTenantFilter {
			query = `
				SELECT id, title, category, content, tags, author, steps_count, last_used_at, tenant_id, created_at, updated_at 
				FROM runbooks 
				WHERE category = $1 AND tenant_id = $4
				ORDER BY created_at DESC 
				LIMIT $2 OFFSET $3
			`
			countQuery = `SELECT COUNT(*) FROM runbooks WHERE category = $1 AND tenant_id = $2`
			args = []interface{}{category, limit, offset, tenantID}
			countArgs = []interface{}{category, tenantID}
		} else {
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
		}
	} else {
		if useTenantFilter {
			query = `
				SELECT id, title, category, content, tags, author, steps_count, last_used_at, tenant_id, created_at, updated_at 
				FROM runbooks 
				WHERE tenant_id = $3
				ORDER BY created_at DESC 
				LIMIT $1 OFFSET $2
			`
			countQuery = `SELECT COUNT(*) FROM runbooks WHERE tenant_id = $1`
			args = []interface{}{limit, offset, tenantID}
			countArgs = []interface{}{tenantID}
		} else {
			query = `
				SELECT id, title, category, content, tags, author, steps_count, last_used_at, tenant_id, created_at, updated_at 
				FROM runbooks 
				ORDER BY created_at DESC 
				LIMIT $1 OFFSET $2
			`
			countQuery = `SELECT COUNT(*) FROM runbooks`
			args = []interface{}{limit, offset}
			countArgs = []interface{}{}
		}
	}

	var total int
	err := r.db.QueryRow(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting runbooks: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
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
	tenantID := middleware.TenantIDFromContext(ctx)
	userRole := middleware.UserRoleFromContext(ctx)

	var query string
	var args []interface{}
	if userRole != "platform_admin" && tenantID != "" {
		query = `
			SELECT id, title, category, content, tags, author, steps_count, last_used_at, tenant_id, created_at, updated_at 
			FROM runbooks 
			WHERE id = $1 AND tenant_id = $2
		`
		args = []interface{}{id, tenantID}
	} else {
		query = `
			SELECT id, title, category, content, tags, author, steps_count, last_used_at, tenant_id, created_at, updated_at 
			FROM runbooks 
			WHERE id = $1
		`
		args = []interface{}{id}
	}

	var rb runbook.Runbook
	err := r.db.QueryRow(ctx, query, args...).Scan(
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
	err := r.db.QueryRow(ctx, query,
		rb.Title, rb.Category, rb.Content, rb.Tags, rb.Author, rb.StepsCount, rb.TenantID, rb.CreatedAt, rb.UpdatedAt,
	).Scan(&rb.ID)
	
	if err != nil {
		return fmt.Errorf("inserting runbook: %w", err)
	}
	return nil
}

func (r *runbookRepo) Update(ctx context.Context, rb *runbook.Runbook) error {
	rb.UpdatedAt = time.Now()
	tenantID := middleware.TenantIDFromContext(ctx)
	userRole := middleware.UserRoleFromContext(ctx)

	var query string
	var args []interface{}
	if userRole != "platform_admin" && tenantID != "" {
		query = `
			UPDATE runbooks 
			SET title = $1, category = $2, content = $3, tags = $4, author = $5, steps_count = $6, updated_at = $7
			WHERE id = $8 AND tenant_id = $9
		`
		args = []interface{}{rb.Title, rb.Category, rb.Content, rb.Tags, rb.Author, rb.StepsCount, rb.UpdatedAt, rb.ID, tenantID}
	} else {
		query = `
			UPDATE runbooks 
			SET title = $1, category = $2, content = $3, tags = $4, author = $5, steps_count = $6, updated_at = $7
			WHERE id = $8
		`
		args = []interface{}{rb.Title, rb.Category, rb.Content, rb.Tags, rb.Author, rb.StepsCount, rb.UpdatedAt, rb.ID}
	}

	cmd, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("updating runbook: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("runbook not found")
	}
	return nil
}

func (r *runbookRepo) Delete(ctx context.Context, id string) error {
	tenantID := middleware.TenantIDFromContext(ctx)
	userRole := middleware.UserRoleFromContext(ctx)

	var query string
	var args []interface{}
	if userRole != "platform_admin" && tenantID != "" {
		query = `DELETE FROM runbooks WHERE id = $1 AND tenant_id = $2`
		args = []interface{}{id, tenantID}
	} else {
		query = `DELETE FROM runbooks WHERE id = $1`
		args = []interface{}{id}
	}

	cmd, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("deleting runbook: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("runbook not found")
	}
	return nil
}
