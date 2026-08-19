package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/capacity"
)

type capacityRepo struct {
	pool DBTX
}

// NewCapacityRepo creates a new PostgreSQL-backed capacity repository.
func NewCapacityRepo(pool DBTX) capacity.Repository {
	return &capacityRepo{pool: pool}
}

func (r *capacityRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.pool)
}

func (r *capacityRepo) List(ctx context.Context, cluster string) ([]capacity.Forecast, error) {
	var query string
	var args []interface{}

	if cluster != "" && cluster != "all" {
		query = `
			SELECT id, cluster, resource_type, current_usage, forecast_7d, forecast_30d, forecast_90d, exhaustion_at, status, recorded_at
			FROM capacity_forecasts
			WHERE cluster = $1
			ORDER BY recorded_at DESC
		`
		args = []interface{}{cluster}
	} else {
		query = `
			SELECT id, cluster, resource_type, current_usage, forecast_7d, forecast_30d, forecast_90d, exhaustion_at, status, recorded_at
			FROM capacity_forecasts
			ORDER BY recorded_at DESC
		`
	}

	rows, err := r.getDB(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying capacity forecasts: %w", err)
	}
	defer rows.Close()

	var result []capacity.Forecast
	for rows.Next() {
		var f capacity.Forecast
		var resType string
		err := rows.Scan(
			&f.ID, &f.Cluster, &resType, &f.CurrentUsage,
			&f.Forecast7d, &f.Forecast30d, &f.Forecast90d,
			&f.ExhaustionAt, &f.Status, &f.RecordedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning capacity forecast: %w", err)
		}
		f.ResourceType = capacity.ResourceType(resType)
		result = append(result, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating capacity forecast rows: %w", err)
	}
	return result, nil
}

func (r *capacityRepo) Record(ctx context.Context, f *capacity.Forecast) error {
	if f.RecordedAt.IsZero() {
		f.RecordedAt = time.Now().UTC()
	}

	query := `
		INSERT INTO capacity_forecasts (cluster, resource_type, current_usage, forecast_7d, forecast_30d, forecast_90d, exhaustion_at, status, recorded_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`
	err := r.getDB(ctx).QueryRow(ctx, query,
		f.Cluster, string(f.ResourceType), f.CurrentUsage,
		f.Forecast7d, f.Forecast30d, f.Forecast90d,
		f.ExhaustionAt, f.Status, f.RecordedAt,
	).Scan(&f.ID)

	if err != nil {
		return fmt.Errorf("inserting capacity forecast: %w", err)
	}
	return nil
}
