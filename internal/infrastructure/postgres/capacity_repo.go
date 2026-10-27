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
	ctx = EnsureTenantContext(ctx)
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

	query, args = BuildTenantQuery(ctx, query, args...)
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
	ctx = EnsureTenantContext(ctx)
	if f.RecordedAt.IsZero() {
		f.RecordedAt = time.Now().UTC()
	}

	tenantID := TenantIDOrDefault(ctx)

	query := `
		INSERT INTO capacity_forecasts (cluster, resource_type, current_usage, forecast_7d, forecast_30d, forecast_90d, exhaustion_at, status, recorded_at, tenant_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`
	err := r.getDB(ctx).QueryRow(ctx, query,
		f.Cluster, string(f.ResourceType), f.CurrentUsage,
		f.Forecast7d, f.Forecast30d, f.Forecast90d,
		f.ExhaustionAt, f.Status, f.RecordedAt, tenantID,
	).Scan(&f.ID)

	if err != nil {
		return fmt.Errorf("inserting capacity forecast: %w", err)
	}
	return nil
}

func (r *capacityRepo) ListSnapshots(ctx context.Context, cluster string) ([]capacity.Snapshot, error) {
	ctx = EnsureTenantContext(ctx)
	query := `
		SELECT cluster, resource_type, current_usage, recorded_at
		FROM capacity_forecasts
	`
	var args []interface{}
	if cluster != "" && cluster != "all" {
		query += " WHERE cluster = $1 ORDER BY recorded_at ASC"
		args = append(args, cluster)
	} else {
		query += " ORDER BY recorded_at ASC"
	}

	query, args = BuildTenantQuery(ctx, query, args...)
	rows, err := r.getDB(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying capacity snapshots: %w", err)
	}
	defer rows.Close()

	type snapKey struct {
		sec int64
	}
	snapMap := make(map[snapKey]*capacity.Snapshot)
	var orderedKeys []snapKey

	for rows.Next() {
		var cl, resType string
		var usage float64
		var recordedAt time.Time
		if err := rows.Scan(&cl, &resType, &usage, &recordedAt); err != nil {
			return nil, fmt.Errorf("scanning capacity snapshot row: %w", err)
		}
		k := snapKey{sec: recordedAt.Unix()}
		s, ok := snapMap[k]
		if !ok {
			s = &capacity.Snapshot{
				Cluster:    cl,
				RecordedAt: recordedAt,
			}
			snapMap[k] = s
			orderedKeys = append(orderedKeys, k)
		}
		switch capacity.ResourceType(resType) {
		case capacity.ResourceCPU:
			s.CPU = usage
		case capacity.ResourceMemory:
			s.Memory = usage
		case capacity.ResourceStorage:
			s.Storage = usage
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating capacity snapshot rows: %w", err)
	}

	result := make([]capacity.Snapshot, 0, len(orderedKeys))
	for _, k := range orderedKeys {
		result = append(result, *snapMap[k])
	}
	return result, nil
}

