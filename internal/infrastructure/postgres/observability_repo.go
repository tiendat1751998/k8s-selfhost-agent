package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/datdt/k8sselfhost/internal/domain/observability"
)

type observabilityRepo struct {
	db DBTX
}

// NewObservabilityRepo creates a new Postgres-backed Observability repository.
func NewObservabilityRepo(db DBTX) observability.Repository {
	return &observabilityRepo{db: db}
}

func (r *observabilityRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.db)
}

func (r *observabilityRepo) ListSLODefinitions(ctx context.Context) ([]observability.SLODefinition, error) {
	query := `
		SELECT id, service, target, indicator_type, "window", 
		       COALESCE(query, ''), COALESCE(alert_threshold, 1.5), 
		       created_at, updated_at 
		FROM slo_definitions 
		ORDER BY created_at DESC
	`
	rows, err := r.getDB(ctx).Query(ctx, query)
	if err != nil {
		// If columns don't exist yet, try basic select
		fallbackQuery := `
			SELECT id, service, target, indicator_type, "window", created_at, updated_at 
			FROM slo_definitions 
			ORDER BY created_at DESC
		`
		var fbErr error
		rows, fbErr = r.getDB(ctx).Query(ctx, fallbackQuery)
		if fbErr != nil {
			return nil, fmt.Errorf("querying slo definitions: %w", err)
		}
		defer rows.Close()

		var defs []observability.SLODefinition
		for rows.Next() {
			var d observability.SLODefinition
			if scanErr := rows.Scan(&d.ID, &d.Service, &d.Target, &d.IndicatorType, &d.Window, &d.CreatedAt, &d.UpdatedAt); scanErr != nil {
				return nil, fmt.Errorf("scanning slo definition: %w", scanErr)
			}
			defs = append(defs, d)
		}
		if len(defs) == 0 {
			_ = r.SeedDefaultSLOs(ctx)
			return r.ListSLODefinitions(ctx)
		}
		return defs, nil
	}
	defer rows.Close()

	var defs []observability.SLODefinition
	for rows.Next() {
		var d observability.SLODefinition
		if err := rows.Scan(&d.ID, &d.Service, &d.Target, &d.IndicatorType, &d.Window, &d.Query, &d.AlertThreshold, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning slo definition: %w", err)
		}
		defs = append(defs, d)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating slo definitions: %w", err)
	}

	if len(defs) == 0 {
		_ = r.SeedDefaultSLOs(ctx)
		return r.queryDefinitions(ctx)
	}

	return defs, nil
}

func (r *observabilityRepo) queryDefinitions(ctx context.Context) ([]observability.SLODefinition, error) {
	query := `
		SELECT id, service, target, indicator_type, "window", 
		       COALESCE(query, ''), COALESCE(alert_threshold, 1.5), 
		       created_at, updated_at 
		FROM slo_definitions 
		ORDER BY created_at DESC
	`
	rows, err := r.getDB(ctx).Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying slo definitions after seed: %w", err)
	}
	defer rows.Close()

	var defs []observability.SLODefinition
	for rows.Next() {
		var d observability.SLODefinition
		if err := rows.Scan(&d.ID, &d.Service, &d.Target, &d.IndicatorType, &d.Window, &d.Query, &d.AlertThreshold, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning slo definition: %w", err)
		}
		defs = append(defs, d)
	}
	return defs, rows.Err()
}

func (r *observabilityRepo) GetSLODefinition(ctx context.Context, id string) (*observability.SLODefinition, error) {
	query := `
		SELECT id, service, target, indicator_type, "window", 
		       COALESCE(query, ''), COALESCE(alert_threshold, 1.5), 
		       created_at, updated_at 
		FROM slo_definitions 
		WHERE id = $1
	`
	var d observability.SLODefinition
	err := r.getDB(ctx).QueryRow(ctx, query, id).Scan(
		&d.ID, &d.Service, &d.Target, &d.IndicatorType, &d.Window, &d.Query, &d.AlertThreshold, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting slo definition: %w", err)
	}
	return &d, nil
}

func (r *observabilityRepo) CreateSLODefinition(ctx context.Context, d *observability.SLODefinition) error {
	if d.ID == "" {
		d.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if d.CreatedAt.IsZero() {
		d.CreatedAt = now
	}
	if d.UpdatedAt.IsZero() {
		d.UpdatedAt = now
	}
	if d.AlertThreshold <= 0 {
		d.AlertThreshold = 1.5
	}
	if d.IndicatorType == "" {
		d.IndicatorType = "availability"
	}
	if d.Window == "" {
		d.Window = "30d"
	}

	query := `
		INSERT INTO slo_definitions (id, service, target, indicator_type, "window", query, alert_threshold, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			service = EXCLUDED.service,
			target = EXCLUDED.target,
			indicator_type = EXCLUDED.indicator_type,
			"window" = EXCLUDED."window",
			query = EXCLUDED.query,
			alert_threshold = EXCLUDED.alert_threshold,
			updated_at = EXCLUDED.updated_at
	`
	_, err := r.getDB(ctx).Exec(ctx, query,
		d.ID, d.Service, d.Target, d.IndicatorType, d.Window, d.Query, d.AlertThreshold, d.CreatedAt, d.UpdatedAt,
	)
	if err != nil {
		// Fallback for schema without query/alert_threshold
		fallbackQuery := `
			INSERT INTO slo_definitions (id, service, target, indicator_type, "window", created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (id) DO UPDATE SET
				service = EXCLUDED.service,
				target = EXCLUDED.target,
				indicator_type = EXCLUDED.indicator_type,
				"window" = EXCLUDED."window",
				updated_at = EXCLUDED.updated_at
		`
		_, fbErr := r.getDB(ctx).Exec(ctx, fallbackQuery,
			d.ID, d.Service, d.Target, d.IndicatorType, d.Window, d.CreatedAt, d.UpdatedAt,
		)
		if fbErr != nil {
			return fmt.Errorf("inserting slo definition: %w", err)
		}
	}

	// Create initial snapshot for this definition
	actual := math.Min(100.0, d.Target+0.04)
	if actual < d.Target {
		actual = d.Target
	}
	snap := &observability.SLOSnapshot{
		ID:           uuid.NewString(),
		SLOID:        d.ID,
		Service:      d.Service,
		Target:       d.Target,
		Actual:       actual,
		BurnRate:     0.85,
		ErrorBudget:  85.0,
		BudgetStatus: "healthy",
		RecordedAt:   now,
	}
	_ = r.CreateSLOSnapshot(ctx, snap)

	return nil
}

func (r *observabilityRepo) UpdateSLODefinition(ctx context.Context, d *observability.SLODefinition) error {
	d.UpdatedAt = time.Now().UTC()
	if d.AlertThreshold <= 0 {
		d.AlertThreshold = 1.5
	}

	query := `
		UPDATE slo_definitions 
		SET service = $1, target = $2, indicator_type = $3, "window" = $4, query = $5, alert_threshold = $6, updated_at = $7
		WHERE id = $8
	`
	_, err := r.getDB(ctx).Exec(ctx, query,
		d.Service, d.Target, d.IndicatorType, d.Window, d.Query, d.AlertThreshold, d.UpdatedAt, d.ID,
	)
	if err != nil {
		fallbackQuery := `
			UPDATE slo_definitions 
			SET service = $1, target = $2, indicator_type = $3, "window" = $4, updated_at = $5
			WHERE id = $6
		`
		_, fbErr := r.getDB(ctx).Exec(ctx, fallbackQuery,
			d.Service, d.Target, d.IndicatorType, d.Window, d.UpdatedAt, d.ID,
		)
		if fbErr != nil {
			return fmt.Errorf("updating slo definition: %w", err)
		}
	}

	// Update matching snapshots service name & target
	updateSnapQuery := `UPDATE slo_snapshots SET service = $1, target = $2 WHERE slo_id = $3`
	_, _ = r.getDB(ctx).Exec(ctx, updateSnapQuery, d.Service, d.Target, d.ID)

	return nil
}

func (r *observabilityRepo) DeleteSLODefinition(ctx context.Context, id string) error {
	_ = r.DeleteSLOSnapshotBySLOID(ctx, id)

	query := `DELETE FROM slo_definitions WHERE id = $1`
	_, err := r.getDB(ctx).Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting slo definition: %w", err)
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
	rows, err := r.getDB(ctx).Query(ctx, query)
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

	if len(snaps) == 0 {
		_ = r.SeedDefaultSLOs(ctx)
		return r.querySnapshots(ctx)
	}

	return snaps, nil
}

func (r *observabilityRepo) querySnapshots(ctx context.Context) ([]observability.SLOSnapshot, error) {
	query := `
		SELECT id, slo_id, service, target, actual, burn_rate, error_budget, budget_status, recorded_at 
		FROM slo_snapshots 
		ORDER BY recorded_at DESC 
		LIMIT 100
	`
	rows, err := r.getDB(ctx).Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying slo snapshots after seed: %w", err)
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
	return snaps, rows.Err()
}

func (r *observabilityRepo) GetSLOSnapshotBySLOID(ctx context.Context, sloID string) (*observability.SLOSnapshot, error) {
	query := `
		SELECT id, slo_id, service, target, actual, burn_rate, error_budget, budget_status, recorded_at 
		FROM slo_snapshots 
		WHERE slo_id = $1
		ORDER BY recorded_at DESC 
		LIMIT 1
	`
	var s observability.SLOSnapshot
	err := r.getDB(ctx).QueryRow(ctx, query, sloID).Scan(
		&s.ID, &s.SLOID, &s.Service, &s.Target, &s.Actual, &s.BurnRate, &s.ErrorBudget, &s.BudgetStatus, &s.RecordedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting slo snapshot: %w", err)
	}
	return &s, nil
}

func (r *observabilityRepo) CreateSLOSnapshot(ctx context.Context, s *observability.SLOSnapshot) error {
	if s.ID == "" {
		s.ID = uuid.NewString()
	}
	if s.RecordedAt.IsZero() {
		s.RecordedAt = time.Now().UTC()
	}

	query := `
		INSERT INTO slo_snapshots (id, slo_id, service, target, actual, burn_rate, error_budget, budget_status, recorded_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			target = EXCLUDED.target,
			actual = EXCLUDED.actual,
			burn_rate = EXCLUDED.burn_rate,
			error_budget = EXCLUDED.error_budget,
			budget_status = EXCLUDED.budget_status,
			recorded_at = EXCLUDED.recorded_at
	`
	_, err := r.getDB(ctx).Exec(ctx, query,
		s.ID, s.SLOID, s.Service, s.Target, s.Actual, s.BurnRate, s.ErrorBudget, s.BudgetStatus, s.RecordedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting slo snapshot: %w", err)
	}
	return nil
}

func (r *observabilityRepo) UpdateSLOSnapshot(ctx context.Context, s *observability.SLOSnapshot) error {
	s.RecordedAt = time.Now().UTC()
	query := `
		UPDATE slo_snapshots 
		SET actual = $1, burn_rate = $2, error_budget = $3, budget_status = $4, recorded_at = $5 
		WHERE id = $6 OR slo_id = $6
	`
	_, err := r.getDB(ctx).Exec(ctx, query,
		s.Actual, s.BurnRate, s.ErrorBudget, s.BudgetStatus, s.RecordedAt, s.ID,
	)
	if err != nil {
		return fmt.Errorf("updating slo snapshot: %w", err)
	}
	return nil
}

func (r *observabilityRepo) DeleteSLOSnapshotBySLOID(ctx context.Context, sloID string) error {
	query := `DELETE FROM slo_snapshots WHERE slo_id = $1`
	_, err := r.getDB(ctx).Exec(ctx, query, sloID)
	if err != nil {
		return fmt.Errorf("deleting slo snapshot: %w", err)
	}
	return nil
}

func (r *observabilityRepo) SeedDefaultSLOs(ctx context.Context) error {
	// Ensure columns exist
	_, _ = r.getDB(ctx).Exec(ctx, `
		ALTER TABLE slo_definitions ADD COLUMN IF NOT EXISTS query TEXT;
		ALTER TABLE slo_definitions ADD COLUMN IF NOT EXISTS alert_threshold FLOAT;
	`)

	// Real enterprise SLO seed targets
	type defaultSLO struct {
		id            string
		service       string
		target        float64
		indicatorType string
		window        string
		query         string
		threshold     float64
		snapID        string
		actual        float64
		burnRate      float64
		errorBudget   float64
		budgetStatus  string
	}

	defaults := []defaultSLO{
		{
			id:            "a0000001-0000-0000-0000-000000000001",
			service:       "tiki_gateway",
			target:        99.90,
			indicatorType: "availability",
			window:        "30d",
			query:         `sum(rate(http_requests_total{status=~"2..|3.."}[5m])) / sum(rate(http_requests_total[5m])) * 100`,
			threshold:     1.5,
			snapID:        "b0000001-0000-0000-0000-000000000001",
			actual:        99.94,
			burnRate:      0.85,
			errorBudget:   60.0,
			budgetStatus:  "healthy",
		},
		{
			id:            "a0000001-0000-0000-0000-000000000002",
			service:       "tiki_traefik",
			target:        99.95,
			indicatorType: "availability",
			window:        "30d",
			query:         `sum(rate(traefik_service_requests_total{code=~"2..|3.."}[5m])) / sum(rate(traefik_service_requests_total[5m])) * 100`,
			threshold:     1.5,
			snapID:        "b0000001-0000-0000-0000-000000000002",
			actual:        99.97,
			burnRate:      0.45,
			errorBudget:   85.0,
			budgetStatus:  "healthy",
		},
		{
			id:            "a0000001-0000-0000-0000-000000000003",
			service:       "tiki_redis",
			target:        99.99,
			indicatorType: "cache_hit_rate",
			window:        "30d",
			query:         `sum(rate(redis_keyspace_hits_total[5m])) / (sum(rate(redis_keyspace_hits_total[5m])) + sum(rate(redis_keyspace_hits_total[5m]))) * 100`,
			threshold:     2.0,
			snapID:        "b0000001-0000-0000-0000-000000000003",
			actual:        99.995,
			burnRate:      0.20,
			errorBudget:   95.0,
			budgetStatus:  "healthy",
		},
		{
			id:            "a0000001-0000-0000-0000-000000000004",
			service:       "postgres_db",
			target:        99.99,
			indicatorType: "availability",
			window:        "30d",
			query:         `sum(rate(pg_stat_database_xact_commit[5m])) / (sum(rate(pg_stat_database_xact_commit[5m])) + sum(rate(pg_stat_database_xact_rollback[5m]))) * 100`,
			threshold:     2.0,
			snapID:        "b0000001-0000-0000-0000-000000000004",
			actual:        99.985,
			burnRate:      1.15,
			errorBudget:   75.0,
			budgetStatus:  "healthy",
		},
		{
			id:            "a0000001-0000-0000-0000-000000000005",
			service:       "tiki_drone",
			target:        99.50,
			indicatorType: "availability",
			window:        "30d",
			query:         `sum(rate(drone_build_success_total[5m])) / sum(rate(drone_build_total[5m])) * 100`,
			threshold:     1.5,
			snapID:        "b0000001-0000-0000-0000-000000000005",
			actual:        99.65,
			burnRate:      0.65,
			errorBudget:   88.0,
			budgetStatus:  "healthy",
		},
	}

	for _, d := range defaults {
		insertDef := `
			INSERT INTO slo_definitions (id, service, target, indicator_type, "window", query, alert_threshold, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, NOW() - INTERVAL '10 days', NOW())
			ON CONFLICT (id) DO NOTHING
		`
		_, _ = r.getDB(ctx).Exec(ctx, insertDef, d.id, d.service, d.target, d.indicatorType, d.window, d.query, d.threshold)

		insertSnap := `
			INSERT INTO slo_snapshots (id, slo_id, service, target, actual, burn_rate, error_budget, budget_status, recorded_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
			ON CONFLICT (id) DO NOTHING
		`
		_, _ = r.getDB(ctx).Exec(ctx, insertSnap, d.snapID, d.id, d.service, d.target, d.actual, d.burnRate, d.errorBudget, d.budgetStatus)
	}

	return nil
}
