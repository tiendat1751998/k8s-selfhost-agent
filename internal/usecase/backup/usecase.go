package backup

import (
	"context"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/backup"
)

type JobRunner interface {
	EnqueueBackup(jobID string) bool
	EnqueueRestore(restoreID string) bool
}

type Usecase struct {
	repo   backup.Repository
	runner JobRunner
}

func NewUsecase(repo backup.Repository) *Usecase {
	return &Usecase{repo: repo}
}

func (u *Usecase) SetRunner(runner JobRunner) {
	u.runner = runner
}

func (u *Usecase) CreateStorage(ctx context.Context, storage *backup.BackupStorage) error {
	return u.repo.CreateStorage(ctx, storage)
}

func (u *Usecase) ListStorages(ctx context.Context, tenantID string) ([]*backup.BackupStorage, error) {
	return u.repo.ListStorages(ctx, tenantID)
}

func (u *Usecase) CreatePolicy(ctx context.Context, policy *backup.BackupPolicy) error {
	return u.repo.CreatePolicy(ctx, policy)
}

func (u *Usecase) ListPolicies(ctx context.Context, tenantID string) ([]*backup.BackupPolicy, error) {
	return u.repo.ListPolicies(ctx, tenantID)
}

func (u *Usecase) TriggerBackup(ctx context.Context, job *backup.BackupJob) error {
	if job.Status == "" {
		job.Status = backup.StatusPending
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = time.Now().UTC()
	}
	if job.UpdatedAt.IsZero() {
		job.UpdatedAt = time.Now().UTC()
	}
	if err := u.repo.CreateJob(ctx, job); err != nil {
		return err
	}
	if u.runner != nil {
		u.runner.EnqueueBackup(job.ID)
	}
	return nil
}

func (u *Usecase) ListJobs(ctx context.Context, tenantID string) ([]*backup.BackupJob, error) {
	return u.repo.ListJobs(ctx, tenantID)
}

func (u *Usecase) TriggerRestore(ctx context.Context, restore *backup.RestoreJob) error {
	if restore.Status == "" {
		restore.Status = backup.StatusPending
	}
	if restore.CreatedAt.IsZero() {
		restore.CreatedAt = time.Now().UTC()
	}
	if restore.UpdatedAt.IsZero() {
		restore.UpdatedAt = time.Now().UTC()
	}
	if err := u.repo.CreateRestore(ctx, restore); err != nil {
		return err
	}
	if u.runner != nil {
		u.runner.EnqueueRestore(restore.ID)
	}
	return nil
}

func (u *Usecase) ListRestores(ctx context.Context, tenantID string) ([]*backup.RestoreJob, error) {
	return u.repo.ListRestores(ctx, tenantID)
}

