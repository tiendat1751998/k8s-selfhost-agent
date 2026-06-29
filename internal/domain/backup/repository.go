package backup

import "context"

// Repository defines data access interfaces for Backup & Disaster Recovery.
type Repository interface {
	GetHistory(ctx context.Context) ([]BackupLog, error)
	TriggerRecovery(ctx context.Context, target string) (*BackupLog, error)
}
