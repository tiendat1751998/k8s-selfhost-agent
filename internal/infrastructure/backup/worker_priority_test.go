package backup_test

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"

	domainBackup "github.com/datdt/k8sselfhost/internal/domain/backup"
	infraBackup "github.com/datdt/k8sselfhost/internal/infrastructure/backup"
	"github.com/datdt/k8sselfhost/internal/infrastructure/backup/drivers"
	"github.com/datdt/k8sselfhost/internal/infrastructure/backup/storage"
)

type orderTrackingDriver struct {
	restoreExecuted atomic.Bool
	backupExecuted  atomic.Bool
}

func (d *orderTrackingDriver) Type() string { return "postgres" }
func (d *orderTrackingDriver) ValidateConnection(ctx context.Context, opts domainBackup.DumpOptions) error {
	return nil
}
func (d *orderTrackingDriver) Dump(ctx context.Context, opts domainBackup.DumpOptions, writer io.Writer) (*domainBackup.DumpResult, error) {
	d.backupExecuted.Store(true)
	return &domainBackup.DumpResult{UncompressedBytes: 100}, nil
}
func (d *orderTrackingDriver) Restore(ctx context.Context, opts domainBackup.RestoreOptions, reader io.Reader) error {
	d.restoreExecuted.Store(true)
	return nil
}

func TestWorkerPool_PERF09_DedicatedRestoreWorkerAndQueuePriority(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	repo := NewMockBackupRepo()
	driver := &orderTrackingDriver{}
	driverReg := drivers.NewDriverRegistry()
	driverReg.Register(driver)

	localStorage := storage.NewLocalStorage(filepath.Join(tmpDir, "local"))
	storageReg := storage.NewStorageRegistry(localStorage)

	engine := infraBackup.NewEngine(repo, driverReg, storageReg)
	logger := zap.NewNop()

	pool := infraBackup.NewWorkerPool(engine, logger, 50)

	// Seed Storage
	storageModel := &domainBackup.BackupStorage{
		ID:       "storage-priority-1",
		TenantID: "tenant-dev",
		Name:     "Local Disk",
		Type:     "local",
		Endpoint: filepath.Join(tmpDir, "local"),
	}
	if err := repo.CreateStorage(ctx, storageModel); err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Seed Policy
	policy := &domainBackup.BackupPolicy{
		ID:                "policy-priority-1",
		TenantID:          "tenant-dev",
		Name:              "Daily Backup",
		DBType:            "postgres",
		DBHost:            "127.0.0.1",
		DBPort:            5432,
		DBName:            "test_db",
		StorageID:         storageModel.ID,
		CompressionLevel:  1,
		EncryptionEnabled: false,
		Enabled:           true,
	}
	if err := repo.CreatePolicy(ctx, policy); err != nil {
		t.Fatalf("failed to create policy: %v", err)
	}

	// Seed BackupJob
	job := &domainBackup.BackupJob{
		ID:          "job-priority-1",
		TenantID:    "tenant-dev",
		PolicyID:    policy.ID,
		Status:      "completed",
		StoragePath: "backups/job-priority-1.dump",
	}
	if err := repo.CreateJob(ctx, job); err != nil {
		t.Fatalf("failed to create job: %v", err)
	}

	// Create physical dummy backup file in storage directory
	uploadStorage := storage.NewLocalStorage(filepath.Join(tmpDir, "local"))
	_, uploadErr := uploadStorage.UploadStream(ctx, "backups/job-priority-1.dump", bytes.NewReader([]byte("backup-data")), 11, nil)
	if uploadErr != nil {
		t.Fatalf("failed to upload dummy backup: %v", uploadErr)
	}

	// Seed Restore
	restore := &domainBackup.RestoreJob{
		ID:           "res-urgent-1",
		TenantID:     "tenant-dev",
		BackupJobID:  job.ID,
		TargetDBHost: "127.0.0.1",
		TargetDBName: "test_db",
		Status:       "pending",
	}
	if err := repo.CreateRestore(ctx, restore); err != nil {
		t.Fatalf("failed to create restore: %v", err)
	}

	// Enqueue restore
	if !pool.EnqueueRestore(restore.ID) {
		t.Fatal("failed to enqueue restore")
	}

	// Start pool with 1 backup worker (+ 1 dedicated restore worker)
	pool.Start(1)
	defer pool.Stop()

	// Wait for restore to complete
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if driver.restoreExecuted.Load() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if !driver.restoreExecuted.Load() {
		t.Fatal("expected restore to be processed promptly by dedicated restore worker")
	}
}
