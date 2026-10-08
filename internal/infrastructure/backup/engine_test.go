package backup_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	domainBackup "github.com/datdt/k8sselfhost/internal/domain/backup"
	infraBackup "github.com/datdt/k8sselfhost/internal/infrastructure/backup"
	"github.com/datdt/k8sselfhost/internal/infrastructure/backup/drivers"
	"github.com/datdt/k8sselfhost/internal/infrastructure/backup/storage"
)

type MockBackupRepo struct {
	mu       sync.RWMutex
	storages map[string]*domainBackup.BackupStorage
	policies map[string]*domainBackup.BackupPolicy
	jobs     map[string]*domainBackup.BackupJob
	restores map[string]*domainBackup.RestoreJob
}

func NewMockBackupRepo() *MockBackupRepo {
	return &MockBackupRepo{
		storages: make(map[string]*domainBackup.BackupStorage),
		policies: make(map[string]*domainBackup.BackupPolicy),
		jobs:     make(map[string]*domainBackup.BackupJob),
		restores: make(map[string]*domainBackup.RestoreJob),
	}
}

func (m *MockBackupRepo) CreateStorage(ctx context.Context, s *domainBackup.BackupStorage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s.ID == "" {
		s.ID = "storage-1"
	}
	m.storages[s.ID] = s
	return nil
}

func (m *MockBackupRepo) ListStorages(ctx context.Context, tenantID string) ([]*domainBackup.BackupStorage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []*domainBackup.BackupStorage
	for _, s := range m.storages {
		if s.TenantID == tenantID {
			list = append(list, s)
		}
	}
	return list, nil
}

func (m *MockBackupRepo) GetStorage(ctx context.Context, id string) (*domainBackup.BackupStorage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.storages[id]
	if !ok {
		return nil, os.ErrNotExist
	}
	return s, nil
}

func (m *MockBackupRepo) CreatePolicy(ctx context.Context, p *domainBackup.BackupPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p.ID == "" {
		p.ID = "policy-1"
	}
	m.policies[p.ID] = p
	return nil
}

func (m *MockBackupRepo) ListPolicies(ctx context.Context, tenantID string) ([]*domainBackup.BackupPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []*domainBackup.BackupPolicy
	for _, p := range m.policies {
		if p.TenantID == tenantID {
			list = append(list, p)
		}
	}
	return list, nil
}

func (m *MockBackupRepo) GetPolicy(ctx context.Context, id string) (*domainBackup.BackupPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.policies[id]
	if !ok {
		return nil, os.ErrNotExist
	}
	return p, nil
}

func (m *MockBackupRepo) CreateJob(ctx context.Context, j *domainBackup.BackupJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if j.ID == "" {
		j.ID = "job-1"
	}
	m.jobs[j.ID] = j
	return nil
}

func (m *MockBackupRepo) ListJobs(ctx context.Context, tenantID string) ([]*domainBackup.BackupJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []*domainBackup.BackupJob
	for _, j := range m.jobs {
		if j.TenantID == tenantID {
			list = append(list, j)
		}
	}
	return list, nil
}

func (m *MockBackupRepo) GetJob(ctx context.Context, id string) (*domainBackup.BackupJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	j, ok := m.jobs[id]
	if !ok {
		return nil, os.ErrNotExist
	}
	return j, nil
}

func (m *MockBackupRepo) UpdateJob(ctx context.Context, j *domainBackup.BackupJob) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs[j.ID] = j
	return nil
}

func (m *MockBackupRepo) ReconcileStaleJobs(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, j := range m.jobs {
		if j.Status == "running" {
			j.Status = "failed"
			j.ErrorMessage = "interrupted by restart"
			j.UpdatedAt = time.Now().UTC()
		}
	}
	return nil
}

func (m *MockBackupRepo) CreateRestore(ctx context.Context, r *domainBackup.RestoreJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r.ID == "" {
		r.ID = "restore-1"
	}
	m.restores[r.ID] = r
	return nil
}

func (m *MockBackupRepo) ListRestores(ctx context.Context, tenantID string) ([]*domainBackup.RestoreJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []*domainBackup.RestoreJob
	for _, r := range m.restores {
		if r.TenantID == tenantID {
			list = append(list, r)
		}
	}
	return list, nil
}

func (m *MockBackupRepo) GetRestore(ctx context.Context, id string) (*domainBackup.RestoreJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.restores[id]
	if !ok {
		return nil, os.ErrNotExist
	}
	return r, nil
}

func (m *MockBackupRepo) UpdateRestore(ctx context.Context, r *domainBackup.RestoreJob) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.restores[r.ID] = r
	return nil
}

type MockDriver struct {
	typeName string
}

func (d *MockDriver) Type() string { return d.typeName }
func (d *MockDriver) ValidateConnection(ctx context.Context, opts domainBackup.DumpOptions) error {
	return nil
}
func (d *MockDriver) Dump(ctx context.Context, opts domainBackup.DumpOptions, writer io.Writer) (*domainBackup.DumpResult, error) {
	data := "CREATE TABLE users (id serial primary key, name text);\nINSERT INTO users (name) VALUES ('alice'), ('bob');\n"
	n, err := io.WriteString(writer, data)
	return &domainBackup.DumpResult{
		UncompressedBytes: int64(n),
		Duration:          10 * time.Millisecond,
	}, err
}
func (d *MockDriver) Restore(ctx context.Context, opts domainBackup.RestoreOptions, reader io.Reader) error {
	content, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	if !strings.Contains(string(content), "CREATE TABLE users") {
		return os.ErrInvalid
	}
	return nil
}

func TestEngine_BackupAndRestoreFlow(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	repo := NewMockBackupRepo()
	driverReg := drivers.NewDriverRegistry()
	driverReg.Register(&MockDriver{typeName: "postgres"})

	localStorage := storage.NewLocalStorage(filepath.Join(tmpDir, "local"))
	storageReg := storage.NewStorageRegistry(localStorage)

	engine := infraBackup.NewEngine(repo, driverReg, storageReg)

	// 1. Seed Storage
	storageModel := &domainBackup.BackupStorage{
		ID:       "storage-local-1",
		TenantID: "tenant-dev",
		Name:     "Local Disk",
		Type:     "local",
		Endpoint: filepath.Join(tmpDir, "local"),
	}
	_ = repo.CreateStorage(ctx, storageModel)

	// 2. Seed Policy
	policy := &domainBackup.BackupPolicy{
		ID:                "policy-1",
		TenantID:          "tenant-dev",
		Name:              "Daily PG Backup",
		DBType:            "postgres",
		DBHost:            "127.0.0.1",
		DBPort:            5432,
		DBName:            "production_db",
		StorageID:         storageModel.ID,
		CompressionLevel:  3,
		EncryptionEnabled: false,
		Enabled:           true,
	}
	_ = repo.CreatePolicy(ctx, policy)

	// 3. Seed BackupJob
	job := &domainBackup.BackupJob{
		ID:         "job-1",
		TenantID:   "tenant-dev",
		PolicyID:   policy.ID,
		Status:     "pending",
		BackupType: "full",
	}
	_ = repo.CreateJob(ctx, job)

	// 4. Execute Backup
	if err := engine.ExecuteBackup(ctx, job.ID); err != nil {
		t.Fatalf("ExecuteBackup failed: %v", err)
	}

	updatedJob, err := repo.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob failed: %v", err)
	}
	if updatedJob.Status != "completed" {
		t.Errorf("expected status 'completed', got '%s'", updatedJob.Status)
	}
	if updatedJob.SizeBytes == 0 || updatedJob.CompressedSizeBytes == 0 {
		t.Errorf("expected non-zero sizes, got raw=%d, comp=%d", updatedJob.SizeBytes, updatedJob.CompressedSizeBytes)
	}
	if len(updatedJob.ChecksumSHA256) != 64 {
		t.Errorf("expected 64 char SHA256 checksum, got %s", updatedJob.ChecksumSHA256)
	}

	// 5. Execute Restore
	restoreJob := &domainBackup.RestoreJob{
		ID:           "restore-1",
		TenantID:     "tenant-dev",
		BackupJobID:  job.ID,
		TargetDBHost: "127.0.0.1",
		TargetDBName: "production_db_restored",
		Status:       "pending",
	}
	_ = repo.CreateRestore(ctx, restoreJob)

	if err := engine.ExecuteRestore(ctx, restoreJob.ID); err != nil {
		t.Fatalf("ExecuteRestore failed: %v", err)
	}

	updatedRestore, err := repo.GetRestore(ctx, restoreJob.ID)
	if err != nil {
		t.Fatalf("GetRestore failed: %v", err)
	}
	if updatedRestore.Status != "completed" {
		t.Errorf("expected restore status 'completed', got '%s'", updatedRestore.Status)
	}
}

type cancellableDumpDriver struct {
	started chan struct{}
}

func (d *cancellableDumpDriver) Type() string { return "postgres" }
func (d *cancellableDumpDriver) ValidateConnection(ctx context.Context, opts domainBackup.DumpOptions) error {
	return nil
}
func (d *cancellableDumpDriver) Dump(ctx context.Context, opts domainBackup.DumpOptions, writer io.Writer) (*domainBackup.DumpResult, error) {
	_, _ = io.WriteString(writer, "PARTIAL BACKUP DATA HEADER\n")
	select {
	case <-d.started:
	default:
		close(d.started)
	}
	<-ctx.Done()
	return nil, ctx.Err()
}
func (d *cancellableDumpDriver) Restore(ctx context.Context, opts domainBackup.RestoreOptions, reader io.Reader) error {
	return nil
}

func TestEngine_CancelledJob_EndsFailedAndPartialArtifactDeleted(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewMockBackupRepo()

	driver := &cancellableDumpDriver{started: make(chan struct{})}
	driverReg := drivers.NewDriverRegistry()
	driverReg.Register(driver)

	localPath := filepath.Join(tmpDir, "local")
	localStorage := storage.NewLocalStorage(localPath)
	storageReg := storage.NewStorageRegistry(localStorage)

	engine := infraBackup.NewEngine(repo, driverReg, storageReg)

	storageModel := &domainBackup.BackupStorage{
		ID:       "storage-local-2",
		TenantID: "tenant-dev",
		Name:     "Local Disk",
		Type:     "local",
		Endpoint: localPath,
	}
	_ = repo.CreateStorage(context.Background(), storageModel)

	policy := &domainBackup.BackupPolicy{
		ID:                "policy-2",
		TenantID:          "tenant-dev",
		Name:              "Daily Test Policy",
		DBType:            "postgres",
		DBHost:            "127.0.0.1",
		DBPort:            5432,
		DBName:            "test_db",
		StorageID:         storageModel.ID,
		CompressionLevel:  1,
		EncryptionEnabled: false,
		Enabled:           true,
	}
	_ = repo.CreatePolicy(context.Background(), policy)

	job := &domainBackup.BackupJob{
		ID:         "job-cancel-1",
		TenantID:   "tenant-dev",
		PolicyID:   policy.ID,
		Status:     "pending",
		BackupType: "full",
	}
	_ = repo.CreateJob(context.Background(), job)

	jobCtx, cancel := context.WithCancel(context.Background())

	execDone := make(chan error, 1)
	go func() {
		execDone <- engine.ExecuteBackup(jobCtx, job.ID)
	}()

	// Wait for dump to start
	select {
	case <-driver.started:
	case <-time.After(2 * time.Second):
		t.Fatal("driver Dump was not started in time")
	}

	// Wait a moment for partial file to be created on disk
	time.Sleep(50 * time.Millisecond)

	// Now cancel job context
	cancel()

	var execErr error
	select {
	case execErr = <-execDone:
	case <-time.After(2 * time.Second):
		t.Fatal("ExecuteBackup did not return within 2s after cancel")
	}

	if execErr == nil {
		t.Fatal("expected ExecuteBackup to return an error on cancellation, got nil")
	}

	// 1. Verify job in repo ended with status 'failed' (proves update did not use cancelled ctx)
	updatedJob, err := repo.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("failed to retrieve updated job: %v", err)
	}
	if updatedJob.Status != "failed" {
		t.Errorf("expected job status 'failed', got '%s'", updatedJob.Status)
	}
	if updatedJob.ErrorMessage == "" {
		t.Error("expected non-empty error message on failed job")
	}

	// 2. Verify partial artifact was deleted from storage
	if updatedJob.StoragePath != "" {
		exists, err := localStorage.Exists(context.Background(), updatedJob.StoragePath)
		if err != nil {
			t.Fatalf("failed to check storage path existence: %v", err)
		}
		if exists {
			t.Errorf("expected partial artifact at %s to be deleted, but it still exists", updatedJob.StoragePath)
		}
	}
}

func TestEngine_ReconcileStaleJobs_MarksRunningRowsFailed(t *testing.T) {
	ctx := context.Background()
	repo := NewMockBackupRepo()
	driverReg := drivers.NewDriverRegistry()
	storageReg := storage.NewStorageRegistry(storage.NewLocalStorage(""))
	engine := infraBackup.NewEngine(repo, driverReg, storageReg)

	_ = repo.CreateJob(ctx, &domainBackup.BackupJob{ID: "job-running", TenantID: "tenant-1", Status: "running", BackupType: "full"})
	_ = repo.CreateJob(ctx, &domainBackup.BackupJob{ID: "job-completed", TenantID: "tenant-1", Status: "completed", BackupType: "full"})
	_ = repo.CreateJob(ctx, &domainBackup.BackupJob{ID: "job-pending", TenantID: "tenant-1", Status: "pending", BackupType: "full"})

	if err := engine.ReconcileStaleJobs(ctx); err != nil {
		t.Fatalf("ReconcileStaleJobs failed: %v", err)
	}

	j1, _ := repo.GetJob(ctx, "job-running")
	if j1.Status != "failed" {
		t.Errorf("expected running job to be marked 'failed', got '%s'", j1.Status)
	}
	if j1.ErrorMessage != "interrupted by restart" {
		t.Errorf("expected error message 'interrupted by restart', got '%s'", j1.ErrorMessage)
	}

	j2, _ := repo.GetJob(ctx, "job-completed")
	if j2.Status != "completed" {
		t.Errorf("expected completed job to stay 'completed', got '%s'", j2.Status)
	}

	j3, _ := repo.GetJob(ctx, "job-pending")
	if j3.Status != "pending" {
		t.Errorf("expected pending job to stay 'pending', got '%s'", j3.Status)
	}
}

