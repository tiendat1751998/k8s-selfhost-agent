package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/backup"
	backupUsecase "github.com/datdt/k8sselfhost/internal/usecase/backup"
)

type mockBackupHandlerRepo struct {
	storages map[string]*backup.BackupStorage
	policies map[string]*backup.BackupPolicy
	jobs     map[string]*backup.BackupJob
	restores map[string]*backup.RestoreJob
}

func newMockBackupHandlerRepo() *mockBackupHandlerRepo {
	return &mockBackupHandlerRepo{
		storages: make(map[string]*backup.BackupStorage),
		policies: make(map[string]*backup.BackupPolicy),
		jobs:     make(map[string]*backup.BackupJob),
		restores: make(map[string]*backup.RestoreJob),
	}
}

func (m *mockBackupHandlerRepo) CreateStorage(ctx context.Context, storage *backup.BackupStorage) error {
	if storage.ID == "" {
		storage.ID = "stor-1"
	}
	storage.CreatedAt = time.Now().UTC()
	storage.UpdatedAt = storage.CreatedAt
	m.storages[storage.ID] = storage
	return nil
}

func (m *mockBackupHandlerRepo) ListStorages(ctx context.Context, tenantID string) ([]*backup.BackupStorage, error) {
	var list []*backup.BackupStorage
	for _, s := range m.storages {
		list = append(list, s)
	}
	return list, nil
}

func (m *mockBackupHandlerRepo) GetStorage(ctx context.Context, id string) (*backup.BackupStorage, error) {
	if s, ok := m.storages[id]; ok {
		return s, nil
	}
	return nil, nil
}

func (m *mockBackupHandlerRepo) CreatePolicy(ctx context.Context, policy *backup.BackupPolicy) error {
	if policy.ID == "" {
		policy.ID = "pol-1"
	}
	m.policies[policy.ID] = policy
	return nil
}

func (m *mockBackupHandlerRepo) ListPolicies(ctx context.Context, tenantID string) ([]*backup.BackupPolicy, error) {
	var list []*backup.BackupPolicy
	for _, p := range m.policies {
		list = append(list, p)
	}
	return list, nil
}

func (m *mockBackupHandlerRepo) GetPolicy(ctx context.Context, id string) (*backup.BackupPolicy, error) {
	if p, ok := m.policies[id]; ok {
		return p, nil
	}
	return nil, nil
}

func (m *mockBackupHandlerRepo) CreateJob(ctx context.Context, job *backup.BackupJob) error {
	if job.ID == "" {
		job.ID = "job-1"
	}
	m.jobs[job.ID] = job
	return nil
}

func (m *mockBackupHandlerRepo) ListJobs(ctx context.Context, tenantID string) ([]*backup.BackupJob, error) {
	var list []*backup.BackupJob
	for _, j := range m.jobs {
		list = append(list, j)
	}
	return list, nil
}

func (m *mockBackupHandlerRepo) GetJob(ctx context.Context, id string) (*backup.BackupJob, error) {
	if j, ok := m.jobs[id]; ok {
		return j, nil
	}
	return nil, nil
}

func (m *mockBackupHandlerRepo) UpdateJob(ctx context.Context, job *backup.BackupJob) error {
	m.jobs[job.ID] = job
	return nil
}

func (m *mockBackupHandlerRepo) CreateRestore(ctx context.Context, restore *backup.RestoreJob) error {
	if restore.ID == "" {
		restore.ID = "res-1"
	}
	m.restores[restore.ID] = restore
	return nil
}

func (m *mockBackupHandlerRepo) ListRestores(ctx context.Context, tenantID string) ([]*backup.RestoreJob, error) {
	var list []*backup.RestoreJob
	for _, r := range m.restores {
		list = append(list, r)
	}
	return list, nil
}

func (m *mockBackupHandlerRepo) GetRestore(ctx context.Context, id string) (*backup.RestoreJob, error) {
	if r, ok := m.restores[id]; ok {
		return r, nil
	}
	return nil, nil
}

func (m *mockBackupHandlerRepo) UpdateRestore(ctx context.Context, restore *backup.RestoreJob) error {
	m.restores[restore.ID] = restore
	return nil
}

func setupBackupTestRouter(repo backup.Repository) http.Handler {
	uc := backupUsecase.NewUsecase(repo)
	h := NewBackupHandler(uc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r
}

func TestBackupHandler_CreateStorage_NoCredentialsLeaked(t *testing.T) {
	repo := newMockBackupHandlerRepo()
	router := setupBackupTestRouter(repo)

	secretKey := "super-secret-aws-secret-access-key-999"
	accessKey := "AKIAIOSFODNN7EXAMPLE"

	payload := `{
		"name": "s3-backup-storage",
		"type": "s3",
		"endpoint": "https://s3.amazonaws.com",
		"bucket": "my-secure-backups",
		"credentials": {
			"access_key": "` + accessKey + `",
			"secret_key": "` + secretKey + `"
		}
	}`

	req := httptest.NewRequest(http.MethodPost, "/storages", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if strings.Contains(body, secretKey) {
		t.Fatalf("CRITICAL SECURITY LEAK: secret key %q leaked in CreateStorage response: %s", secretKey, body)
	}
	if strings.Contains(body, accessKey) {
		t.Fatalf("CRITICAL SECURITY LEAK: access key %q leaked in CreateStorage response: %s", accessKey, body)
	}
	if strings.Contains(body, `"credentials"`) {
		t.Fatalf("credentials field should not be serialized in response: %s", body)
	}
}

func TestBackupHandler_ListStorages_NoCredentialsLeaked(t *testing.T) {
	repo := newMockBackupHandlerRepo()
	secretKey := "super-secret-minio-key-888"
	accessKey := "minioadmin"

	repo.storages["stor-1"] = &backup.BackupStorage{
		ID:       "stor-1",
		TenantID: "default",
		Name:     "minio-storage",
		Type:     "s3",
		Endpoint: "http://minio:9000",
		Bucket:   "backups",
		Credentials: map[string]string{
			"access_key": accessKey,
			"secret_key": secretKey,
		},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	router := setupBackupTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/storages", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if strings.Contains(body, secretKey) {
		t.Fatalf("CRITICAL SECURITY LEAK: secret key %q leaked in ListStorages response: %s", secretKey, body)
	}
	if strings.Contains(body, accessKey) {
		t.Fatalf("CRITICAL SECURITY LEAK: access key %q leaked in ListStorages response: %s", accessKey, body)
	}
	if strings.Contains(body, `"credentials"`) {
		t.Fatalf("credentials field should not be serialized in response: %s", body)
	}
}
