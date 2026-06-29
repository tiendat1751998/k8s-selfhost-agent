package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datdt/k8sselfhost/internal/domain/cost"
)

type costRepo struct {
	pool *pgxpool.Pool
}

// NewCostRepo creates a new PostgreSQL-backed cost repository.
func NewCostRepo(pool *pgxpool.Pool) cost.Repository {
	return &costRepo{pool: pool}
}

func (r *costRepo) GetClusterCosts(ctx context.Context) ([]cost.ClusterCost, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, provider, monthly_cost, daily_cost, cpu_cost, memory_cost, storage_cost, network_cost, trend, updated_at
		FROM cluster_costs
		ORDER BY monthly_cost DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("querying cluster costs: %w", err)
	}
	defer rows.Close()

	var result []cost.ClusterCost
	for rows.Next() {
		var c cost.ClusterCost
		err := rows.Scan(
			&c.ID, &c.Name, &c.Provider, &c.MonthlyCost, &c.DailyCost,
			&c.CPUCost, &c.MemoryCost, &c.StorageCost, &c.NetworkCost,
			&c.Trend, &c.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning cluster cost: %w", err)
		}
		result = append(result, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating cluster cost rows: %w", err)
	}
	return result, nil
}

func (r *costRepo) GetNamespaceCosts(ctx context.Context) ([]cost.NamespaceCost, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, namespace, cluster, cpu_requested, memory_requested, monthly_cost, utilization, updated_at
		FROM namespace_costs
		ORDER BY monthly_cost DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("querying namespace costs: %w", err)
	}
	defer rows.Close()

	var result []cost.NamespaceCost
	for rows.Next() {
		var c cost.NamespaceCost
		err := rows.Scan(
			&c.ID, &c.Namespace, &c.Cluster, &c.CPURequested, &c.MemoryRequested,
			&c.MonthlyCost, &c.Utilization, &c.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning namespace cost: %w", err)
		}
		result = append(result, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating namespace cost rows: %w", err)
	}
	return result, nil
}

func (r *costRepo) GetResourceWaste(ctx context.Context) ([]cost.ResourceWaste, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, type, resource, namespace, cluster, cpu_util, mem_util, wasted_cost, severity, updated_at
		FROM resource_waste
		ORDER BY wasted_cost DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("querying resource waste: %w", err)
	}
	defer rows.Close()

	var result []cost.ResourceWaste
	for rows.Next() {
		var w cost.ResourceWaste
		err := rows.Scan(
			&w.ID, &w.Type, &w.Resource, &w.Namespace, &w.Cluster,
			&w.CPUUtil, &w.MemUtil, &w.WastedCost, &w.Severity, &w.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning resource waste: %w", err)
		}
		result = append(result, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating resource waste rows: %w", err)
	}
	return result, nil
}
