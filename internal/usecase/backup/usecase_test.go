package backup_test

import (
	"context"
	"testing"

	"github.com/datdt/k8sselfhost/internal/domain/backup"
	usecaseBackup "github.com/datdt/k8sselfhost/internal/usecase/backup"
)

type mockBackupRepo struct {
	jobs     map[string]*backup.BackupJob
	restores map[string]*backup.RestoreJob
	storages map[string]*backup.BackupStorage
	policies map[string]*backup.BackupPolicy
}

func newMockBackupRepo() *mockBackupRepo {
	return &mockBackupRepo{
		jobs:     make(map[string]*backup.BackupJob),
		restores: make(map[string]*backup.RestoreJob),
		storages: make(map[string]*backup.BackupStorage),
		policies: make(map[string]*backup.BackupPolicy),
	}
}

func (m *mockBackupRepo) CreateStorage(ctx context.Context, storage *backup.BackupStorage) error {
	m.storages[storage.ID] = storage
	return nil
}

func (m *mockBackupRepo) ListStorages(ctx context.Context, tenantID string) ([]*backup.BackupStorage, error) {
	var list []*backup.BackupStorage
	for _, s := range m.storages {
		if s.TenantID == tenantID {
			list = append(list, s)
		}
	}
	return list, nil
}

func (m *mockBackupRepo) GetStorage(ctx context.Context, id string) (*backup.BackupStorage, error) {
	return m.storages[id], nil
}

func (m *mockBackupRepo) CreatePolicy(ctx context.Context, policy *backup.BackupPolicy) error {
	m.policies[policy.ID] = policy
	return nil
}

func (m *mockBackupRepo) ListPolicies(ctx context.Context, tenantID string) ([]*backup.BackupPolicy, error) {
	var list []*backup.BackupPolicy
	for _, p := range m.policies {
		if p.TenantID == tenantID {
			list = append(list, p)
		}
	}
	return list, nil
}

func (m *mockBackupRepo) GetPolicy(ctx context.Context, id string) (*backup.BackupPolicy, error) {
	return m.policies[id], nil
}

func (m *mockBackupRepo) CreateJob(ctx context.Context, job *backup.BackupJob) error {
	m.jobs[job.ID] = job
	return nil
}

func (m *mockBackupRepo) ListJobs(ctx context.Context, tenantID string) ([]*backup.BackupJob, error) {
	var list []*backup.BackupJob
	for _, j := range m.jobs {
		if j.TenantID == tenantID {
			list = append(list, j)
		}
	}
	return list, nil
}

func (m *mockBackupRepo) GetJob(ctx context.Context, id string) (*backup.BackupJob, error) {
	return m.jobs[id], nil
}

func (m *mockBackupRepo) UpdateJob(ctx context.Context, job *backup.BackupJob) error {
	m.jobs[job.ID] = job
	return nil
}

func (m *mockBackupRepo) CreateRestore(ctx context.Context, restore *backup.RestoreJob) error {
	m.restores[restore.ID] = restore
	return nil
}

func (m *mockBackupRepo) ListRestores(ctx context.Context, tenantID string) ([]*backup.RestoreJob, error) {
	var list []*backup.RestoreJob
	for _, r := range m.restores {
		if r.TenantID == tenantID {
			list = append(list, r)
		}
	}
	return list, nil
}

func (m *mockBackupRepo) GetRestore(ctx context.Context, id string) (*backup.RestoreJob, error) {
	return m.restores[id], nil
}

func (m *mockBackupRepo) UpdateRestore(ctx context.Context, restore *backup.RestoreJob) error {
	m.restores[restore.ID] = restore
	return nil
}

type mockRunner struct {
	enqueuedBackups  []string
	enqueuedRestores []string
}

func (r *mockRunner) EnqueueBackup(jobID string) bool {
	r.enqueuedBackups = append(r.enqueuedBackups, jobID)
	return true
}

func (r *mockRunner) EnqueueRestore(restoreID string) bool {
	r.enqueuedRestores = append(r.enqueuedRestores, restoreID)
	return true
}

func TestUsecase_TriggerBackup(t *testing.T) {
	repo := newMockBackupRepo()
	uc := usecaseBackup.NewUsecase(repo)
	runner := &mockRunner{}
	uc.SetRunner(runner)

	job := backup.NewBackupJob("policy-1", "storage-1", "production_db")
	job.TenantID = "tenant-test"

	err := uc.TriggerBackup(context.Background(), job)
	if err != nil {
		t.Fatalf("TriggerBackup failed: %v", err)
	}

	if len(runner.enqueuedBackups) != 1 {
		t.Fatalf("expected 1 enqueued backup, got %d", len(runner.enqueuedBackups))
	}
	if runner.enqueuedBackups[0] != job.ID {
		t.Errorf("expected enqueued job ID '%s', got '%s'", job.ID, runner.enqueuedBackups[0])
	}

	jobs, err := uc.ListJobs(context.Background(), "tenant-test")
	if err != nil {
		t.Fatalf("ListJobs failed: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job listed, got %d", len(jobs))
	}
}

func TestUsecase_TriggerRestore(t *testing.T) {
	repo := newMockBackupRepo()
	uc := usecaseBackup.NewUsecase(repo)
	runner := &mockRunner{}
	uc.SetRunner(runner)

	restore := backup.NewRestoreJob("job-1", "localhost", "production_db_restored")
	restore.TenantID = "tenant-test"

	err := uc.TriggerRestore(context.Background(), restore)
	if err != nil {
		t.Fatalf("TriggerRestore failed: %v", err)
	}

	if len(runner.enqueuedRestores) != 1 {
		t.Fatalf("expected 1 enqueued restore, got %d", len(runner.enqueuedRestores))
	}
	if runner.enqueuedRestores[0] != restore.ID {
		t.Errorf("expected enqueued restore ID '%s', got '%s'", restore.ID, runner.enqueuedRestores[0])
	}

	restores, err := uc.ListRestores(context.Background(), "tenant-test")
	if err != nil {
		t.Fatalf("ListRestores failed: %v", err)
	}
	if len(restores) != 1 {
		t.Fatalf("expected 1 restore listed, got %d", len(restores))
	}
}
