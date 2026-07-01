package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"

	"github.com/datdt/k8sselfhost/internal/domain/drift"
	"github.com/datdt/k8sselfhost/pkg/errors"
)

// DriftRepo implements drift.Repository using PostgreSQL.
type DriftRepo struct {
	pool *pgxpool.Pool
}

// NewDriftRepo creates a new PostgreSQL-backed drift repository.
func NewDriftRepo(pool *pgxpool.Pool) *DriftRepo {
	return &DriftRepo{pool: pool}
}

func (r *DriftRepo) detectSwarmDrift(ctx context.Context) {
	cli, err := client.NewClientWithOpts(
		client.WithHost("tcp://10.10.10.133:2375"),
		client.WithVersion("1.41"),
	)
	if err != nil {
		return
	}
	defer cli.Close()

	services, err := cli.ServiceList(ctx, types.ServiceListOptions{})
	if err != nil {
		return
	}

	for _, s := range services {
		expectedReplicas := uint64(1)
		actualReplicas := uint64(0)
		if s.Spec.Mode.Replicated != nil && s.Spec.Mode.Replicated.Replicas != nil {
			actualReplicas = *s.Spec.Mode.Replicated.Replicas
		}

		if actualReplicas != expectedReplicas {
			var exists bool
			err := r.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM drift_records WHERE cluster = $1 AND resource = $2 AND status = 'drifted')",
				"cls-dev-local", s.Spec.Name).Scan(&exists)
			if err != nil {
				logger.Get().Error("failed to check if drift record exists", zap.Error(err))
				continue
			}

			if !exists {
				expectedState := fmt.Sprintf("deploy:\n  replicas: %d\n  image: %s", expectedReplicas, s.Spec.TaskTemplate.ContainerSpec.Image)
				actualState := fmt.Sprintf("deploy:\n  replicas: %d\n  image: %s", actualReplicas, s.Spec.TaskTemplate.ContainerSpec.Image)
				diff := fmt.Sprintf("--- Expected (GitOps)\n+++ Actual (Live Swarm)\n@@ -2,2 +2,2 @@\n-  replicas: %d\n+  replicas: %d\n", expectedReplicas, actualReplicas)

				err := r.Create(ctx, &drift.DriftRecord{
					Cluster:       "cls-dev-local",
					Namespace:     "default",
					Resource:      s.Spec.Name,
					ResourceKind:  "Service",
					ExpectedState: expectedState,
					ActualState:   actualState,
					Diff:          diff,
					Status:        drift.Drifted,
					DetectedAt:    time.Now().UTC(),
				})
				if err != nil {
					logger.Get().Error("failed to create drift record", zap.Error(err))
				}
			}
		} else {
			_, err := r.pool.Exec(ctx, "UPDATE drift_records SET status = 'in_sync' WHERE cluster = $1 AND resource = $2 AND status = 'drifted'",
				"cls-dev-local", s.Spec.Name)
			if err != nil {
				logger.Get().Error("failed to update drift status to in_sync", zap.Error(err))
			}
		}
	}
}

func (r *DriftRepo) List(ctx context.Context, cluster string, status *drift.DriftStatus, limit, offset int) ([]drift.DriftRecord, int, error) {
	r.detectSwarmDrift(ctx)

	query := `SELECT id, cluster, namespace, resource, resource_kind, expected_state, actual_state, diff, status, detected_at FROM drift_records WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM drift_records WHERE 1=1`
	args := []interface{}{}
	idx := 1

	if cluster != "" {
		query += fmt.Sprintf(" AND cluster = $%d", idx)
		countQuery += fmt.Sprintf(" AND cluster = $%d", idx)
		args = append(args, cluster)
		idx++
	}
	if status != nil {
		query += fmt.Sprintf(" AND status = $%d", idx)
		countQuery += fmt.Sprintf(" AND status = $%d", idx)
		args = append(args, string(*status))
		idx++
	}

	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, errors.Wrap(err, "counting drift records")
	}

	query += fmt.Sprintf(" ORDER BY detected_at DESC LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, errors.Wrap(err, "listing drift records")
	}
	defer rows.Close()

	var records []drift.DriftRecord
	for rows.Next() {
		var d drift.DriftRecord
		if err := rows.Scan(&d.ID, &d.Cluster, &d.Namespace, &d.Resource, &d.ResourceKind, &d.ExpectedState, &d.ActualState, &d.Diff, &d.Status, &d.DetectedAt); err != nil {
			return nil, 0, errors.Wrap(err, "scanning drift record")
		}
		records = append(records, d)
	}
	return records, total, nil
}

func (r *DriftRepo) Create(ctx context.Context, d *drift.DriftRecord) error {
	query := `INSERT INTO drift_records (cluster, namespace, resource, resource_kind, expected_state, actual_state, diff, status, detected_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id`

	err := r.pool.QueryRow(ctx, query,
		d.Cluster, d.Namespace, d.Resource, d.ResourceKind,
		d.ExpectedState, d.ActualState, d.Diff,
		string(d.Status), d.DetectedAt,
	).Scan(&d.ID)
	if err != nil {
		return errors.Wrap(err, "inserting drift record")
	}
	return nil
}

func (r *DriftRepo) Resolve(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE drift_records SET status = 'in_sync' WHERE id = $1`, id)
	if err != nil {
		return errors.Wrap(err, "resolving drift record")
	}
	return nil
}

// Ensure interface compliance.
var _ drift.Repository = (*DriftRepo)(nil)
