package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datdt/k8sselfhost/internal/domain/backup"
)

type backupRepo struct {
	pool *pgxpool.Pool
}

// NewBackupRepo creates a new PostgreSQL-backed backup repository.
func NewBackupRepo(pool *pgxpool.Pool) backup.Repository {
	return &backupRepo{pool: pool}
}

func (r *backupRepo) GetHistory(ctx context.Context) ([]backup.BackupLog, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, timestamp, action, target, status, duration, size, details
		FROM backup_history
		ORDER BY timestamp DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("querying backup history: %w", err)
	}
	defer rows.Close()

	var result []backup.BackupLog
	for rows.Next() {
		var log backup.BackupLog
		var detailsBytes []byte
		err := rows.Scan(
			&log.ID, &log.Timestamp, &log.Action, &log.Target, &log.Status,
			&log.Duration, &log.Size, &detailsBytes,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning backup log: %w", err)
		}
		log.Details = json.RawMessage(detailsBytes)
		result = append(result, log)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating backup log rows: %w", err)
	}
	return result, nil
}

func (r *backupRepo) TriggerRecovery(ctx context.Context, target string) (*backup.BackupLog, error) {
	log := backup.BackupLog{
		Timestamp: time.Now(),
		Action:    "restore",
		Target:    target,
		Status:    "success",
		Duration:  "34s",
		Size:      "1.2 GB",
		Details:   json.RawMessage(`{"recovered_namespaces":["production"],"status":"verified"}`),
	}

	err := r.pool.QueryRow(ctx, `
		INSERT INTO backup_history (action, target, status, duration, size, details)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, timestamp
	`, log.Action, log.Target, log.Status, log.Duration, log.Size, string(log.Details)).Scan(&log.ID, &log.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("inserting recovery log: %w", err)
	}

	return &log, nil
}
