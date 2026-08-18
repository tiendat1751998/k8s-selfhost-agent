package backup

import "context"

type Repository interface {
	CreateStorage(ctx context.Context, storage *BackupStorage) error
	ListStorages(ctx context.Context, tenantID string) ([]*BackupStorage, error)
	GetStorage(ctx context.Context, id string) (*BackupStorage, error)

	CreatePolicy(ctx context.Context, policy *BackupPolicy) error
	ListPolicies(ctx context.Context, tenantID string) ([]*BackupPolicy, error)
	GetPolicy(ctx context.Context, id string) (*BackupPolicy, error)

	CreateJob(ctx context.Context, job *BackupJob) error
	ListJobs(ctx context.Context, tenantID string) ([]*BackupJob, error)
	GetJob(ctx context.Context, id string) (*BackupJob, error)
	UpdateJob(ctx context.Context, job *BackupJob) error

	CreateRestore(ctx context.Context, restore *RestoreJob) error
	ListRestores(ctx context.Context, tenantID string) ([]*RestoreJob, error)
	GetRestore(ctx context.Context, id string) (*RestoreJob, error)
	UpdateRestore(ctx context.Context, restore *RestoreJob) error
}

