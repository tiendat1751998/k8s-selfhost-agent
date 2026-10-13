package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	domain "github.com/datdt/k8sselfhost/internal/domain/numamemory"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/numamemory"
)

func setupNUMATestRouter(mgr *usecase.NUMAMemoryManager) *chi.Mux {
	handler := adapthttp.NewNUMAMemoryHandler(mgr, zap.NewNop())
	r := chi.NewRouter()
	r.Route("/api/v1/kernel", handler.RegisterRoutes)
	return r
}

func TestNUMAMemoryHandler_GetNUMAStatus_Success(t *testing.T) {
	mgr := usecase.NewNUMAMemoryManager(
		usecase.WithInitialNUMAZones([]domain.NUMAZoneStatus{
			{NodeID: 0, TotalRAMBytes: 32 * 1024 * 1024 * 1024, FreeRAMBytes: 16 * 1024 * 1024 * 1024, ZoneReclaimMode: 0, SpinlockDetected: false},
		}),
	)
	r := setupNUMATestRouter(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/kernel/memory/numa-status", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Zones []domain.NUMAZoneStatus `json:"zones"`
		Count int                     `json:"count"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Count != 1 || len(resp.Zones) != 1 {
		t.Fatalf("expected 1 zone, got %d", resp.Count)
	}
}

func TestNUMAMemoryHandler_GetNUMAStatus_Locked(t *testing.T) {
	mgr := usecase.NewNUMAMemoryManager(
		usecase.WithInitialNUMAZones([]domain.NUMAZoneStatus{
			{NodeID: 0, TotalRAMBytes: 32 * 1024 * 1024 * 1024, FreeRAMBytes: 512 * 1024 * 1024, ZoneReclaimMode: 1, SpinlockDetected: true},
		}),
	)
	r := setupNUMATestRouter(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/kernel/memory/numa-status", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 Internal Server Error, got %d: %s", rec.Code, rec.Body.String())
	}

	var prob adapthttp.ProblemDetails
	if err := json.NewDecoder(rec.Body).Decode(&prob); err != nil {
		t.Fatalf("failed to decode problem details: %v", err)
	}

	if prob.Code != domain.CodeNUMAZoneLocked {
		t.Fatalf("expected code %s, got %s", domain.CodeNUMAZoneLocked, prob.Code)
	}
}

func TestNUMAMemoryHandler_TuneZoneReclaim(t *testing.T) {
	mgr := usecase.NewNUMAMemoryManager(
		usecase.WithInitialNUMAZones([]domain.NUMAZoneStatus{
			{NodeID: 0, TotalRAMBytes: 32 * 1024 * 1024 * 1024, FreeRAMBytes: 16 * 1024 * 1024 * 1024, ZoneReclaimMode: 1, SpinlockDetected: true},
		}),
	)
	r := setupNUMATestRouter(mgr)

	body := []byte(`{"target_mode": 0}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/kernel/memory/zone-reclaim-tune", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "applied" {
		t.Fatalf("expected status applied, got %v", resp["status"])
	}
}

func TestNUMAMemoryHandler_ApplyDirtyLimits(t *testing.T) {
	mgr := usecase.NewNUMAMemoryManager()
	r := setupNUMATestRouter(mgr)

	body := []byte(`{"dirty_background_bytes": 268435456, "dirty_bytes": 1073741824}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/kernel/memory/dirty-limits", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestNUMAMemoryHandler_ApplyDirtyLimits_Saturation(t *testing.T) {
	mgr := usecase.NewNUMAMemoryManager()
	mgr.SetCurrentDirtyBytes(2 * 1024 * 1024 * 1024) // 2GB dirty
	r := setupNUMATestRouter(mgr)

	body := []byte(`{"dirty_background_bytes": 268435456, "dirty_bytes": 1073741824}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/kernel/memory/dirty-limits", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 Service Unavailable, got %d: %s", rec.Code, rec.Body.String())
	}

	var prob adapthttp.ProblemDetails
	if err := json.NewDecoder(rec.Body).Decode(&prob); err != nil {
		t.Fatalf("failed to decode problem details: %v", err)
	}

	if prob.Code != domain.CodeDirtyPageFlushSaturation {
		t.Fatalf("expected code %s, got %s", domain.CodeDirtyPageFlushSaturation, prob.Code)
	}
}

func TestNUMAMemoryHandler_CheckHugePages(t *testing.T) {
	mgr := usecase.NewNUMAMemoryManager(
		usecase.WithInitialHugePages([]domain.HugePagePoolStatus{
			{PageSizeKB: 2048, TotalPages: 1024, FreePages: 512, FragmentationIdx: 0.15},
		}),
	)
	r := setupNUMATestRouter(mgr)

	body := []byte(`{"page_size_kb": 2048, "requested_pages": 4}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/kernel/memory/hugepages-check", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestNUMAMemoryHandler_CheckHugePages_Fragmented(t *testing.T) {
	mgr := usecase.NewNUMAMemoryManager(
		usecase.WithInitialHugePages([]domain.HugePagePoolStatus{
			{PageSizeKB: 1048576, TotalPages: 4, FreePages: 1, FragmentationIdx: 0.85},
		}),
	)
	r := setupNUMATestRouter(mgr)

	body := []byte(`{"page_size_kb": 1048576, "requested_pages": 10}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/kernel/memory/hugepages-check", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInsufficientStorage {
		t.Fatalf("expected 507 Insufficient Storage, got %d: %s", rec.Code, rec.Body.String())
	}

	var prob adapthttp.ProblemDetails
	if err := json.NewDecoder(rec.Body).Decode(&prob); err != nil {
		t.Fatalf("failed to decode problem details: %v", err)
	}

	if prob.Code != domain.CodeHugePagesFragmented {
		t.Fatalf("expected code %s, got %s", domain.CodeHugePagesFragmented, prob.Code)
	}
}
