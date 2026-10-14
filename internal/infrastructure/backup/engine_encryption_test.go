package backup_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	domainBackup "github.com/datdt/k8sselfhost/internal/domain/backup"
	infraBackup "github.com/datdt/k8sselfhost/internal/infrastructure/backup"
	"github.com/datdt/k8sselfhost/internal/infrastructure/backup/drivers"
	"github.com/datdt/k8sselfhost/internal/infrastructure/backup/storage"
)

func TestEngine_Encryption_EmptyKeyRejected(t *testing.T) {
	tempDir := t.TempDir()
	ctx := context.Background()

	repo := NewMockBackupRepo()
	driverReg := drivers.NewDriverRegistry()
	driverReg.Register(&MockDriver{typeName: "postgres"})

	localStorage := storage.NewLocalStorage(filepath.Join(tempDir, "local"))
	storageReg := storage.NewStorageRegistry(localStorage)

	engine := infraBackup.NewEngine(repo, driverReg, storageReg)

	_ = repo.CreateStorage(ctx, &domainBackup.BackupStorage{
		ID:       "storage-1",
		TenantID: "tenant-1",
		Name:     "Local",
		Type:     "local",
		Endpoint: filepath.Join(tempDir, "local"),
	})

	_ = repo.CreatePolicy(ctx, &domainBackup.BackupPolicy{
		ID:                "policy-empty-key",
		TenantID:          "tenant-1",
		Name:              "Daily Empty Key",
		DBType:            "postgres",
		StorageID:         "storage-1",
		EncryptionEnabled: true,
		EncryptionKeyID:   "", // Empty key must be rejected
	})

	_ = repo.CreateJob(ctx, &domainBackup.BackupJob{
		ID:       "job-empty-key",
		TenantID: "tenant-1",
		PolicyID: "policy-empty-key",
		Status:   "pending",
	})

	err := engine.ExecuteBackup(ctx, "job-empty-key")
	if err == nil {
		t.Fatal("expected error when EncryptionKeyID is empty with EncryptionEnabled=true, got nil")
	}

	job, _ := repo.GetJob(ctx, "job-empty-key")
	if job.Status != "failed" {
		t.Fatalf("expected job status 'failed', got '%s'", job.Status)
	}
	if !strings.Contains(job.ErrorMessage, "EncryptionKeyID is empty") {
		t.Fatalf("expected error message to mention empty EncryptionKeyID, got %q", job.ErrorMessage)
	}
}

func TestEngine_Encryption_ShortKeyDerived(t *testing.T) {
	tempDir := t.TempDir()
	ctx := context.Background()

	repo := NewMockBackupRepo()
	driverReg := drivers.NewDriverRegistry()
	driverReg.Register(&MockDriver{typeName: "postgres"})

	localStorage := storage.NewLocalStorage(filepath.Join(tempDir, "local"))
	storageReg := storage.NewStorageRegistry(localStorage)

	engine := infraBackup.NewEngine(repo, driverReg, storageReg)

	_ = repo.CreateStorage(ctx, &domainBackup.BackupStorage{
		ID:       "storage-1",
		TenantID: "tenant-1",
		Name:     "Local",
		Type:     "local",
		Endpoint: filepath.Join(tempDir, "local"),
	})

	_ = repo.CreatePolicy(ctx, &domainBackup.BackupPolicy{
		ID:                "policy-short-key",
		TenantID:          "tenant-1",
		Name:              "Daily Short Key",
		DBType:            "postgres",
		StorageID:         "storage-1",
		EncryptionEnabled: true,
		EncryptionKeyID:   "short-key-123", // deterministic SHA-256 derived key
	})

	_ = repo.CreateJob(ctx, &domainBackup.BackupJob{
		ID:       "job-short-key",
		TenantID: "tenant-1",
		PolicyID: "policy-short-key",
		Status:   "pending",
	})

	err := engine.ExecuteBackup(ctx, "job-short-key")
	if err != nil {
		t.Fatalf("expected short key to derive 32-byte key and succeed, got: %v", err)
	}

	job, _ := repo.GetJob(ctx, "job-short-key")
	if job.Status != "completed" {
		t.Fatalf("expected job status 'completed', got '%s'", job.Status)
	}
}
