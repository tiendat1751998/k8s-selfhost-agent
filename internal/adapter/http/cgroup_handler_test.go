package http_test

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

	adapter "github.com/datdt/k8sselfhost/internal/adapter/http"
	nodemetricsDomain "github.com/datdt/k8sselfhost/internal/domain/nodemetrics"
	nodemetricsUsecase "github.com/datdt/k8sselfhost/internal/usecase/nodemetrics"
)

type mockKernelManager struct {
	events      []nodemetricsDomain.CGroupV2MemoryEvent
	maps        []nodemetricsDomain.EBPFMapUtilization
	auditErr    error
	resizeErr   error
	tuneErr     error
	resizedCap  int64
}

func (m *mockKernelManager) AuditCGroupV2Events(ctx context.Context, nodeName string) ([]nodemetricsDomain.CGroupV2MemoryEvent, error) {
	if m.auditErr != nil {
		return nil, m.auditErr
	}
	return m.events, nil
}

func (m *mockKernelManager) InspectEBPFMaps(ctx context.Context, clusterID string) ([]nodemetricsDomain.EBPFMapUtilization, error) {
	return m.maps, nil
}

func (m *mockKernelManager) AutoResizeEBPFConntrackMap(ctx context.Context, clusterID string, newCapacity int64) error {
	if m.resizeErr != nil {
		return m.resizeErr
	}
	m.resizedCap = newCapacity
	return nil
}

func (m *mockKernelManager) EnforceNodeSlabTuning(ctx context.Context, nodeName string) error {
	if m.tuneErr != nil {
		return m.tuneErr
	}
	return nil
}

func setupTestRouter(handler *adapter.CGroupHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Route("/api/v1", func(api chi.Router) {
		handler.RegisterRoutes(api)
	})
	return r
}

func TestCGroupHandler_GetCGroupEvents(t *testing.T) {
	mockMgr := &mockKernelManager{
		events: []nodemetricsDomain.CGroupV2MemoryEvent{
			{
				ContainerID:       "cont-99",
				PodName:           "payment-pod",
				Namespace:         "prod",
				HighEvents:        45,
				ThrottledDuration: 120 * time.Millisecond,
				UpdatedAt:         time.Now(),
			},
		},
	}
	usecase := nodemetricsUsecase.NewCGroupScalerUsecase(mockMgr, nil)
	handler := adapter.NewCGroupHandler(usecase, mockMgr)
	router := setupTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/nodes/worker-node-1/kernel/cgroup/events", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Events          []nodemetricsDomain.CGroupV2MemoryEvent `json:"events"`
		Recommendations []nodemetricsUsecase.CGroupRecommendation `json:"recommendations"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}

	if len(resp.Events) != 1 || len(resp.Recommendations) != 1 {
		t.Fatalf("expected 1 event and 1 recommendation, got %d and %d", len(resp.Events), len(resp.Recommendations))
	}
}

func TestCGroupHandler_TuneSlabCache(t *testing.T) {
	mockMgr := &mockKernelManager{}
	usecase := nodemetricsUsecase.NewCGroupScalerUsecase(mockMgr, nil)
	handler := adapter.NewCGroupHandler(usecase, mockMgr)
	router := setupTestRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/nodes/worker-node-1/kernel/slab/tune", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var report nodemetricsUsecase.SlabTuningReport
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("failed to decode slab tuning response: %v", err)
	}

	if report.VFSCachePressure != 150 {
		t.Fatalf("expected VFSCachePressure=150, got %d", report.VFSCachePressure)
	}
}

func TestCGroupHandler_GetEBPFMaps(t *testing.T) {
	mockMgr := &mockKernelManager{
		maps: []nodemetricsDomain.EBPFMapUtilization{
			nodemetricsDomain.NewEBPFMapUtilization("cilium_ct_any4_global", 450000, 512000),
		},
	}
	usecase := nodemetricsUsecase.NewCGroupScalerUsecase(mockMgr, nil)
	handler := adapter.NewCGroupHandler(usecase, mockMgr)
	router := setupTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/cluster-prod/ebpf/maps", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var maps []nodemetricsDomain.EBPFMapUtilization
	if err := json.Unmarshal(rec.Body.Bytes(), &maps); err != nil {
		t.Fatalf("failed to decode ebpf maps response: %v", err)
	}

	if len(maps) != 1 || maps[0].MapName != "cilium_ct_any4_global" {
		t.Fatalf("unexpected maps response: %+v", maps)
	}
}

func TestCGroupHandler_ResizeConntrackMap(t *testing.T) {
	mockMgr := &mockKernelManager{}
	usecase := nodemetricsUsecase.NewCGroupScalerUsecase(mockMgr, nil)
	handler := adapter.NewCGroupHandler(usecase, mockMgr)
	router := setupTestRouter(handler)

	// Valid resize
	body, _ := json.Marshal(map[string]int64{"capacity": 1024000})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/cluster-prod/ebpf/conntrack/resize", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if mockMgr.resizedCap != 1024000 {
		t.Fatalf("expected resized capacity 1024000, got %d", mockMgr.resizedCap)
	}

	// Invalid negative capacity
	invalidBody, _ := json.Marshal(map[string]int64{"capacity": -10})
	badReq := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/cluster-prod/ebpf/conntrack/resize", bytes.NewReader(invalidBody))
	badReq.Header.Set("Content-Type", "application/json")
	badRec := httptest.NewRecorder()
	router.ServeHTTP(badRec, badReq)

	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for negative capacity, got %d", badRec.Code)
	}
}

func TestCGroupHandler_ErrorPropagation(t *testing.T) {
	mockMgr := &mockKernelManager{
		auditErr: errors.New("cannot connect to node agent"),
	}
	usecase := nodemetricsUsecase.NewCGroupScalerUsecase(mockMgr, nil)
	handler := adapter.NewCGroupHandler(usecase, mockMgr)
	router := setupTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/nodes/worker-down/kernel/cgroup/events", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500 when kernel audit fails, got %d", rec.Code)
	}
}
