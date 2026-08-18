package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/datdt/k8sselfhost/internal/domain/promotion"
)

type promotionRepo struct {
	db DBTX
}

// NewPromotionRepo creates a new Postgres-backed Promotion repository.
func NewPromotionRepo(db DBTX) promotion.Repository {
	return &promotionRepo{db: db}
}

func (r *promotionRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.db)
}

func (r *promotionRepo) List(ctx context.Context, status string, limit, offset int) ([]promotion.Promotion, int, error) {
	var query string
	var countQuery string
	var args []interface{}
	var countArgs []interface{}
	
	if status != "" && status != "all" {
		query = `
			SELECT id, service, version, from_env, to_env, status, requester, approver, approved_at, completed_at, created_at 
			FROM promotions 
			WHERE status = $1 
			ORDER BY created_at DESC 
			LIMIT $2 OFFSET $3
		`
		countQuery = `SELECT COUNT(*) FROM promotions WHERE status = $1`
		args = []interface{}{status, limit, offset}
		countArgs = []interface{}{status}
	} else {
		query = `
			SELECT id, service, version, from_env, to_env, status, requester, approver, approved_at, completed_at, created_at 
			FROM promotions 
			ORDER BY created_at DESC 
			LIMIT $1 OFFSET $2
		`
		countQuery = `SELECT COUNT(*) FROM promotions`
		args = []interface{}{limit, offset}
	}

	var total int
	err := r.getDB(ctx).QueryRow(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting promotions: %w", err)
	}

	rows, err := r.getDB(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("querying promotions: %w", err)
	}
	defer rows.Close()

	var promos []promotion.Promotion
	for rows.Next() {
		var p promotion.Promotion
		var approver *string
		
		if err := rows.Scan(
			&p.ID, &p.Service, &p.Version, &p.FromEnv, &p.ToEnv, &p.Status, 
			&p.Requester, &approver, &p.ApprovedAt, &p.CompletedAt, &p.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scanning promotion: %w", err)
		}
		
		if approver != nil {
			p.Approver = *approver
		}

		promos = append(promos, p)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating promotions: %w", err)
	}

	return promos, total, nil
}

func (r *promotionRepo) GetByID(ctx context.Context, id string) (*promotion.Promotion, error) {
	query := `
		SELECT id, service, version, from_env, to_env, status, requester, approver, approved_at, completed_at, created_at 
		FROM promotions 
		WHERE id = $1
	`
	var p promotion.Promotion
	var approver *string

	err := r.getDB(ctx).QueryRow(ctx, query, id).Scan(
		&p.ID, &p.Service, &p.Version, &p.FromEnv, &p.ToEnv, &p.Status, 
		&p.Requester, &approver, &p.ApprovedAt, &p.CompletedAt, &p.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("promotion not found")
		}
		return nil, fmt.Errorf("querying promotion: %w", err)
	}

	if approver != nil {
		p.Approver = *approver
	}

	return &p, nil
}

func (r *promotionRepo) Create(ctx context.Context, p *promotion.Promotion) error {
	query := `
		INSERT INTO promotions (service, version, from_env, to_env, status, requester, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`
	err := r.getDB(ctx).QueryRow(ctx, query,
		p.Service, p.Version, string(p.FromEnv), string(p.ToEnv), string(p.Status), p.Requester, p.CreatedAt,
	).Scan(&p.ID)
	
	if err != nil {
		return fmt.Errorf("inserting promotion: %w", err)
	}
	
	return nil
}

func (r *promotionRepo) Approve(ctx context.Context, id, approver string) error {
	query := `
		UPDATE promotions 
		SET status = 'approved', approver = $1, approved_at = NOW() 
		WHERE id = $2 AND status = 'pending'
	`
	cmd, err := r.getDB(ctx).Exec(ctx, query, approver, id)
	if err != nil {
		return fmt.Errorf("approving promotion: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("promotion not found or not in pending status")
	}
	return nil
}

func (r *promotionRepo) Reject(ctx context.Context, id, rejecter string) error {
	query := `
		UPDATE promotions 
		SET status = 'rejected', approver = $1, approved_at = NOW() 
		WHERE id = $2 AND status = 'pending'
	`
	cmd, err := r.getDB(ctx).Exec(ctx, query, rejecter, id)
	if err != nil {
		return fmt.Errorf("rejecting promotion: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("promotion not found or not in pending status")
	}
	return nil
}

func (r *promotionRepo) Complete(ctx context.Context, id string) error {
	query := `
		UPDATE promotions 
		SET status = 'completed', completed_at = NOW() 
		WHERE id = $1 AND (status = 'approved' OR status = 'promoting')
	`
	cmd, err := r.getDB(ctx).Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("completing promotion: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("promotion not found or not in approved/promoting status")
	}
	return nil
}
