package backup_test

import (
	"context"
	"io"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"

	domainBackup "github.com/datdt/k8sselfhost/internal/domain/backup"
	infraBackup "github.com/datdt/k8sselfhost/internal/infrastructure/backup"
	"github.com/datdt/k8sselfhost/internal/infrastructure/backup/drivers"
	"github.com/datdt/k8sselfhost/internal/infrastructure/backup/storage"
)

type blockingTestDriver struct {
	started   chan struct{}
	cancelled atomic.Bool
}

func (b *blockingTestDriver) Type() string { return "postgres" }
func (b *blockingTestDriver) ValidateConnection(ctx context.Context, opts domainBackup.DumpOptions) error {
	return nil
}
func (b *blockingTestDriver) Dump(ctx context.Context, opts domainBackup.DumpOptions, writer io.Writer) (*domainBackup.DumpResult, error) {
	select {
	case <-b.started:
	default:
		close(b.started)
	}
	<-ctx.Done()
	b.cancelled.Store(true)
	return nil, ctx.Err()
}
func (b *blockingTestDriver) Restore(ctx context.Context, opts domainBackup.RestoreOptions, reader io.Reader) error {
	select {
	case <-b.started:
	default:
		close(b.started)
	}
	<-ctx.Done()
	b.cancelled.Store(true)
	return ctx.Err()
}

func TestWorkerPool_PromptShutdown_UnderOneSecond(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	repo := NewMockBackupRepo()
	driver := &blockingTestDriver{started: make(chan struct{})}
	driverReg := drivers.NewDriverRegistry()
	driverReg.Register(driver)

	localStorage := storage.NewLocalStorage(filepath.Join(tmpDir, "local"))
	storageReg := storage.NewStorageRegistry(localStorage)

	engine := infraBackup.NewEngine(repo, driverReg, storageReg)
	logger := zap.NewNop()

	pool := infraBackup.NewWorkerPool(engine, logger, 10)

	// Seed Storage
	storageModel := &domainBackup.BackupStorage{
		ID:       "storage-1",
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
		ID:                "policy-1",
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
		ID:         "job-blocking-1",
		TenantID:   "tenant-dev",
		PolicyID:   policy.ID,
		Status:     "pending",
		BackupType: "full",
	}
	if err := repo.CreateJob(ctx, job); err != nil {
		t.Fatalf("failed to create job: %v", err)
	}

	pool.Start(1)

	// Enqueue the job
	enqueued := pool.EnqueueBackup(job.ID)
	if !enqueued {
		t.Fatal("failed to enqueue backup job")
	}

	// Wait for the driver to start dumping
	select {
	case <-driver.started:
	case <-time.After(2 * time.Second):
		t.Fatal("driver Dump was not started in time")
	}

	// Prompt shutdown test: Stop() MUST return in < 1s
	stopDone := make(chan struct{})
	startStop := time.Now()
	go func() {
		pool.Stop()
		close(stopDone)
	}()

	select {
	case <-stopDone:
		stopDuration := time.Since(startStop)
		if stopDuration >= 1*time.Second {
			t.Fatalf("pool.Stop() took %v, expected < 1s", stopDuration)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("pool.Stop() hung for > 1s (prompt shutdown failure)")
	}

	if !driver.cancelled.Load() {
		t.Error("expected job context to be cancelled during pool.Stop()")
	}
}

func TestWorkerPool_GoroutineLeak_Baseline(t *testing.T) {
	// Let any background tasks settle
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baselineGoroutines := runtime.NumGoroutine()

	tmpDir := t.TempDir()
	repo := NewMockBackupRepo()
	driverReg := drivers.NewDriverRegistry()
	driverReg.Register(&MockDriver{typeName: "postgres"})

	localStorage := storage.NewLocalStorage(filepath.Join(tmpDir, "local"))
	storageReg := storage.NewStorageRegistry(localStorage)

	engine := infraBackup.NewEngine(repo, driverReg, storageReg)
	logger := zap.NewNop()

	pool := infraBackup.NewWorkerPool(engine, logger, 10)
	pool.Start(5)

	// Give workers time to start
	time.Sleep(20 * time.Millisecond)
	duringWorkers := runtime.NumGoroutine()
	if duringWorkers <= baselineGoroutines {
		t.Logf("workers running: baseline=%d, active=%d", baselineGoroutines, duringWorkers)
	}

	pool.Stop()

	// Wait for goroutines to clean up
	runtime.GC()
	time.Sleep(50 * time.Millisecond)

	afterGoroutines := runtime.NumGoroutine()
	// Allow small delta (<= 1) for Go test runner internal threads
	if afterGoroutines > baselineGoroutines+1 {
		t.Fatalf("goroutine leak detected: baseline=%d, after=%d (diff=%d)",
			baselineGoroutines, afterGoroutines, afterGoroutines-baselineGoroutines)
	}
}
