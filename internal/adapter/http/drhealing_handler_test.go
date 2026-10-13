package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/drhealing"
)

type mockDRResilienceManager struct {
	locks               []drhealing.BackupStorageLockStatus
	breakLockErr        error
	breakLockCalls      []string
	repairDBErr         error
	repairDBCalls       []string
	auditProtected      bool
	auditErr            error
}

func (m *mockDRResilienceManager) InspectBackupLocks(ctx context.Context) ([]drhealing.BackupStorageLockStatus, error) {
	return m.locks, nil
}

func (m *mockDRResilienceManager) BreakStaleLock(ctx context.Context, locationName string) error {
	m.breakLockCalls = append(m.breakLockCalls, locationName)
	return m.breakLockErr
}

func (m *mockDRResilienceManager) RepairEdgeDatabase(ctx context.Context, nodeID, dbPath string) error {
	m.repairDBCalls = append(m.repairDBCalls, fmt.Sprintf("%s:%s", nodeID, dbPath))
	return m.repairDBErr
}

func (m *mockDRResilienceManager) AuditKMSKeyProtection(ctx context.Context, keyARN string) (bool, error) {
	if m.auditErr != nil {
		return false, m.auditErr
	}
	return m.auditProtected, nil
}

func setupDRHealingTestRouter(mgr drhealing.DRResilienceManager) *chi.Mux {
	r := chi.NewRouter()
	handler := NewDRHealingHandler(mgr, zap.NewNop())
	r.Route("/api/v1/dr", func(sub chi.Router) {
		handler.RegisterRoutes(sub)
	})
	return r
}

func TestDRHealingHandler_InspectBackupLocks(t *testing.T) {
	mgr := &mockDRResilienceManager{
		locks: []drhealing.BackupStorageLockStatus{
			{
				LocationName:  "velero-kopia-s3",
				Provider:      "aws",
				Bucket:        "corp-backups",
				IsLocked:      true,
				LockCreatedBy: "velero-pod-crash",
				LockAge:       80 * time.Minute,
				IsStale:       true,
				LastCheckedAt: time.Now().UTC(),
			},
		},
	}
	router := setupDRHealingTestRouter(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dr/backup/locks", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data  []drhealing.BackupStorageLockStatus `json:"data"`
		Count int                                 `json:"count"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Count != 1 || len(resp.Data) != 1 {
		t.Fatalf("expected 1 lock, got %d", resp.Count)
	}
	if !resp.Data[0].IsStale {
		t.Errorf("expected lock to be marked stale")
	}
}

func TestDRHealingHandler_BreakStaleLock_Success(t *testing.T) {
	mgr := &mockDRResilienceManager{
		breakLockErr: nil,
	}
	router := setupDRHealingTestRouter(mgr)

	body := []byte(`{"location_name":"velero-kopia-s3"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dr/backup/locks/break", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 (RFC 7807), got %d: %s", rr.Code, rr.Body.String())
	}

	var prob drhealing.RFC7807Error
	if err := json.NewDecoder(rr.Body).Decode(&prob); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if prob.Code != "ERR_STALE_BACKUP_LOCK_CLEARED" {
		t.Errorf("expected code ERR_STALE_BACKUP_LOCK_CLEARED, got %s", prob.Code)
	}
}

func TestDRHealingHandler_BreakStaleLock_ConflictWhenPodRunning(t *testing.T) {
	mgr := &mockDRResilienceManager{
		breakLockErr: drhealing.ErrBackupPodStillRunning,
	}
	router := setupDRHealingTestRouter(mgr)

	body := []byte(`{"location_name":"velero-kopia-s3"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dr/backup/locks/break", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected status 409 Conflict, got %d: %s", rr.Code, rr.Body.String())
	}

	var prob drhealing.RFC7807Error
	if err := json.NewDecoder(rr.Body).Decode(&prob); err != nil {
		t.Fatalf("failed to decode error body: %v", err)
	}
	if prob.Code != "ERR_BACKUP_POD_STILL_RUNNING" {
		t.Errorf("expected code ERR_BACKUP_POD_STILL_RUNNING, got %s", prob.Code)
	}
}

func TestDRHealingHandler_RepairEdgeDatabase_Success(t *testing.T) {
	mgr := &mockDRResilienceManager{
		repairDBErr: nil,
	}
	router := setupDRHealingTestRouter(mgr)

	body := []byte(`{"node_id":"edge-01","db_path":"/var/lib/k8s/state.db"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dr/edge/repair-db", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}
	if resp["status"] != "repaired" {
		t.Errorf("expected status repaired, got %v", resp["status"])
	}
}

func TestDRHealingHandler_RepairEdgeDatabase_CorruptionFailure(t *testing.T) {
	mgr := &mockDRResilienceManager{
		repairDBErr: drhealing.ErrDatabaseCorrupt,
	}
	router := setupDRHealingTestRouter(mgr)

	body := []byte(`{"node_id":"edge-01","db_path":"/var/lib/k8s/state.db"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dr/edge/repair-db", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d: %s", rr.Code, rr.Body.String())
	}

	var prob drhealing.RFC7807Error
	if err := json.NewDecoder(rr.Body).Decode(&prob); err != nil {
		t.Fatalf("failed to decode error body: %v", err)
	}
	if prob.Code != "ERR_DATABASE_CORRUPT" {
		t.Errorf("expected code ERR_DATABASE_CORRUPT, got %s", prob.Code)
	}
}

func TestDRHealingHandler_AuditKMSKeyProtection_Protected(t *testing.T) {
	mgr := &mockDRResilienceManager{
		auditProtected: true,
	}
	router := setupDRHealingTestRouter(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dr/kms/audit-protection?key_arn=arn:aws:kms:us-east-1:123456789012:key/test", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}
	if resp["protected"] != true {
		t.Errorf("expected protected=true, got %v", resp["protected"])
	}
}

func TestDRHealingHandler_AuditKMSKeyProtection_Unprotected(t *testing.T) {
	mgr := &mockDRResilienceManager{
		auditProtected: false,
		auditErr:       drhealing.ErrKMSDeletionUnprotected,
	}
	router := setupDRHealingTestRouter(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dr/kms/audit-protection?key_arn=arn:aws:kms:us-east-1:123456789012:key/unprotected", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 Bad Request, got %d: %s", rr.Code, rr.Body.String())
	}

	var prob drhealing.RFC7807Error
	if err := json.NewDecoder(rr.Body).Decode(&prob); err != nil {
		t.Fatalf("failed to decode error body: %v", err)
	}
	if prob.Code != "ERR_KMS_DELETION_UNPROTECTED" {
		t.Errorf("expected code ERR_KMS_DELETION_UNPROTECTED, got %s", prob.Code)
	}
}

func TestDRHealingHandler_ValidationErrors(t *testing.T) {
	mgr := &mockDRResilienceManager{}
	router := setupDRHealingTestRouter(mgr)

	// Missing location_name
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/dr/backup/locks/break", bytes.NewReader([]byte(`{"location_name":""}`)))
	req1.Header.Set("Content-Type", "application/json")
	rr1 := httptest.NewRecorder()
	router.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusBadRequest {
		t.Errorf("expected 400 on empty location_name, got %d", rr1.Code)
	}

	// Missing key_arn query param
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/dr/kms/audit-protection", nil)
	rr2 := httptest.NewRecorder()
	router.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusBadRequest {
		t.Errorf("expected 400 on missing key_arn param, got %d", rr2.Code)
	}
}
