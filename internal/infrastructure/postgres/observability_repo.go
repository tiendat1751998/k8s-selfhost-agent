package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datdt/k8sselfhost/internal/domain/observability"
)

type observabilityRepo struct {
	db *pgxpool.Pool
}

// NewObservabilityRepo creates a new Postgres-backed Observability repository.
func NewObservabilityRepo(db *pgxpool.Pool) observability.Repository {
	return &observabilityRepo{db: db}
}

func (r *observabilityRepo) ListSLODefinitions(ctx context.Context) ([]observability.SLODefinition, error) {
	query := `
		SELECT id, service, target, indicator_type, window, created_at, updated_at 
		FROM slo_definitions 
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying slo definitions: %w", err)
	}
	defer rows.Close()

	var defs []observability.SLODefinition
	for rows.Next() {
		var d observability.SLODefinition
		if err := rows.Scan(&d.ID, &d.Service, &d.Target, &d.IndicatorType, &d.Window, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning slo definition: %w", err)
		}
		defs = append(defs, d)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating slo definitions: %w", err)
	}

	return defs, nil
}

func (r *observabilityRepo) CreateSLODefinition(ctx context.Context, d *observability.SLODefinition) error {
	query := `
		INSERT INTO slo_definitions (service, target, indicator_type, window, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`
	err := r.db.QueryRow(ctx, query,
		d.Service, d.Target, d.IndicatorType, d.Window, d.CreatedAt, d.UpdatedAt,
	).Scan(&d.ID)
	
	if err != nil {
		return fmt.Errorf("inserting slo definition: %w", err)
	}
	return nil
}

func (r *observabilityRepo) ListSLOSnapshots(ctx context.Context) ([]observability.SLOSnapshot, error) {
	query := `
		SELECT id, slo_id, service, target, actual, burn_rate, error_budget, budget_status, recorded_at 
		FROM slo_snapshots 
		ORDER BY recorded_at DESC 
		LIMIT 100
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying slo snapshots: %w", err)
	}
	defer rows.Close()

	var snaps []observability.SLOSnapshot
	for rows.Next() {
		var s observability.SLOSnapshot
		if err := rows.Scan(&s.ID, &s.SLOID, &s.Service, &s.Target, &s.Actual, &s.BurnRate, &s.ErrorBudget, &s.BudgetStatus, &s.RecordedAt); err != nil {
			return nil, fmt.Errorf("scanning slo snapshot: %w", err)
		}
		snaps = append(snaps, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating slo snapshots: %w", err)
	}

	return snaps, nil
}

func (r *observabilityRepo) CreateSLOSnapshot(ctx context.Context, s *observability.SLOSnapshot) error {
	query := `
		INSERT INTO slo_snapshots (slo_id, service, target, actual, burn_rate, error_budget, budget_status, recorded_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`
	err := r.db.QueryRow(ctx, query,
		s.SLOID, s.Service, s.Target, s.Actual, s.BurnRate, s.ErrorBudget, s.BudgetStatus, s.RecordedAt,
	).Scan(&s.ID)
	
	if err != nil {
		return fmt.Errorf("inserting slo snapshot: %w", err)
	}
	return nil
}
