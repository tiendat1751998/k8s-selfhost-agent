package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	domainstorage "github.com/datdt/k8sselfhost/internal/domain/storage"
)

type mockStorageResiliencyManager struct {
	auditFn    func(ctx context.Context, clusterID string) ([]domainstorage.CephOSDStatus, error)
	repairFn   func(ctx context.Context, clusterID string, osdID int) error
	autoTuneFn func(ctx context.Context, clusterID string) (*domainstorage.MinIOScannerTuning, error)
	verifyDBFn func(ctx context.Context, dbConnString string) (bool, error)
}

func (m *mockStorageResiliencyManager) AuditCephCluster(ctx context.Context, clusterID string) ([]domainstorage.CephOSDStatus, error) {
	if m.auditFn != nil {
		return m.auditFn(ctx, clusterID)
	}
	return nil, nil
}

func (m *mockStorageResiliencyManager) TriggerCephOSDRepair(ctx context.Context, clusterID string, osdID int) error {
	if m.repairFn != nil {
		return m.repairFn(ctx, clusterID, osdID)
	}
	return nil
}

func (m *mockStorageResiliencyManager) AutoTuneMinIOScanner(ctx context.Context, clusterID string) (*domainstorage.MinIOScannerTuning, error) {
	if m.autoTuneFn != nil {
		return m.autoTuneFn(ctx, clusterID)
	}
	return nil, nil
}

func (m *mockStorageResiliencyManager) VerifySafeDatabaseTarget(ctx context.Context, dbConnString string) (bool, error) {
	if m.verifyDBFn != nil {
		return m.verifyDBFn(ctx, dbConnString)
	}
	return false, nil
}

func setupResiliencyTestRouter(h *StorageResiliencyHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Route("/api/v1", func(api chi.Router) {
		h.WireToRouter(api)
	})
	return r
}

func TestStorageResiliencyHandler_AuditCephCluster_Success(t *testing.T) {
	mockMgr := &mockStorageResiliencyManager{
		auditFn: func(ctx context.Context, clusterID string) ([]domainstorage.CephOSDStatus, error) {
			if clusterID != "cluster-alpha" {
				return nil, errors.New("unexpected cluster")
			}
			return []domainstorage.CephOSDStatus{
				{
					OSDID:         0,
					Host:          "node-1",
					Status:        domainstorage.OSDStatusUp,
					RocksDBHealth: domainstorage.RocksDBHealthOK,
					UpdatedAt:     time.Now(),
				},
			}, nil
		},
	}

	handler := NewStorageResiliencyHandler(mockMgr)
	router := setupResiliencyTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/cluster-alpha/storage/ceph/osds", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var osds []domainstorage.CephOSDStatus
	if err := json.NewDecoder(rr.Body).Decode(&osds); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(osds) != 1 || osds[0].OSDID != 0 {
		t.Errorf("unexpected osd response: %+v", osds)
	}
}

func TestStorageResiliencyHandler_TriggerCephOSDRepair_Success(t *testing.T) {
	repaired := false
	mockMgr := &mockStorageResiliencyManager{
		repairFn: func(ctx context.Context, clusterID string, osdID int) error {
			if clusterID == "cluster-alpha" && osdID == 4 {
				repaired = true
				return nil
			}
			return domainstorage.ErrCephOSDNotFound
		},
	}

	handler := NewStorageResiliencyHandler(mockMgr)
	router := setupResiliencyTestRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/cluster-alpha/storage/ceph/osds/4/repair", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}
	if !repaired {
		t.Errorf("expected OSD 4 repair to be triggered")
	}
}

func TestStorageResiliencyHandler_TriggerCephOSDRepair_NotFound(t *testing.T) {
	mockMgr := &mockStorageResiliencyManager{
		repairFn: func(ctx context.Context, clusterID string, osdID int) error {
			return domainstorage.ErrCephOSDNotFound
		},
	}

	handler := NewStorageResiliencyHandler(mockMgr)
	router := setupResiliencyTestRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/cluster-alpha/storage/ceph/osds/99/repair", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestStorageResiliencyHandler_TriggerCephOSDRepair_InvalidOSD(t *testing.T) {
	handler := NewStorageResiliencyHandler(&mockStorageResiliencyManager{})
	router := setupResiliencyTestRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/cluster-alpha/storage/ceph/osds/invalid/repair", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for non-integer osdId, got %d", rr.Code)
	}
}

func TestStorageResiliencyHandler_AutoTuneMinIOScanner_Success(t *testing.T) {
	mockMgr := &mockStorageResiliencyManager{
		autoTuneFn: func(ctx context.Context, clusterID string) (*domainstorage.MinIOScannerTuning, error) {
			return &domainstorage.MinIOScannerTuning{
				CurrentSpeed:     domainstorage.ScannerSpeedDefault,
				ActiveClientIOPS: 2400,
				HealQueueLength:  10,
				RecommendedSpeed: domainstorage.ScannerSpeedSlowest,
				AutoTunedAt:      time.Now(),
			}, nil
		},
	}

	handler := NewStorageResiliencyHandler(mockMgr)
	router := setupResiliencyTestRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/cluster-alpha/storage/minio/autotune", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var tuning domainstorage.MinIOScannerTuning
	if err := json.NewDecoder(rr.Body).Decode(&tuning); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}
	if tuning.RecommendedSpeed != domainstorage.ScannerSpeedSlowest {
		t.Errorf("expected recommended speed slowest, got %s", tuning.RecommendedSpeed)
	}
}

func TestStorageResiliencyHandler_VerifySafeDatabaseTarget_PrimaryBlocked(t *testing.T) {
	mockMgr := &mockStorageResiliencyManager{
		verifyDBFn: func(ctx context.Context, dbConnString string) (bool, error) {
			return false, domainstorage.ErrPrimaryDatabaseProtected
		},
	}

	handler := NewStorageResiliencyHandler(mockMgr)
	router := setupResiliencyTestRouter(handler)

	body := []byte(`{"db_conn_string": "postgres://postgres:secret@db-primary:5432/postgres"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/cluster-alpha/storage/database/safeguard", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// Primary target must be forbidden with 403 Forbidden
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 Forbidden on Primary node safeguard, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var res map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if res["safe"] != false {
		t.Errorf("expected safe=false, got %v", res["safe"])
	}
}

func TestStorageResiliencyHandler_VerifySafeDatabaseTarget_StandbyAllowed(t *testing.T) {
	mockMgr := &mockStorageResiliencyManager{
		verifyDBFn: func(ctx context.Context, dbConnString string) (bool, error) {
			return true, nil
		},
	}

	handler := NewStorageResiliencyHandler(mockMgr)
	router := setupResiliencyTestRouter(handler)

	body := []byte(`{"db_conn_string": "postgres://postgres:secret@db-standby:5432/postgres"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/cluster-alpha/storage/database/safeguard", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK on Standby replica target, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var res map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if res["safe"] != true {
		t.Errorf("expected safe=true, got %v", res["safe"])
	}
}

func TestStorageResiliencyHandler_VerifySafeDatabaseTarget_InvalidPayload(t *testing.T) {
	handler := NewStorageResiliencyHandler(&mockStorageResiliencyManager{})
	router := setupResiliencyTestRouter(handler)

	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/cluster-alpha/storage/database/safeguard", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 Bad Request on empty db_conn_string, got %d", rr.Code)
	}
}
