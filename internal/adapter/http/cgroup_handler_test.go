package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	domain "github.com/datdt/k8sselfhost/internal/domain/nodemetrics"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/nodemetrics"
)

func setupCGroupTestRouter() (*chi.Mux, *usecase.CGroupScalerService) {
	scaler := usecase.NewCGroupScalerService(
		usecase.WithLogger(zap.NewNop()),
		usecase.WithHostRAM(16*1024*1024*1024), // 16GB RAM
	)
	handler := adapthttp.NewCGroupHandler(scaler, zap.NewNop())

	r := chi.NewRouter()
	r.Route("/api/v1/kernel", handler.RegisterRoutes)

	return r, scaler
}

func TestCGroupHandler_GetCGroupEvents(t *testing.T) {
	r, scaler := setupCGroupTestRouter()

	scaler.RegisterCGroupEvent("node-beta", domain.CGroupV2MemoryEvent{
		ContainerID:       "c-200",
		PodName:           "ingress-router",
		Namespace:         "kube-system",
		HighEvents:        35,
		ThrottledDuration: 30 * time.Second,
		UpdatedAt:         time.Now().UTC(),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/kernel/nodes/node-beta/cgroup-events", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		NodeName string                       `json:"node_name"`
		Events   []domain.CGroupV2MemoryEvent `json:"events"`
		Count    int                          `json:"count"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if resp.Count != 1 || len(resp.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", resp.Count)
	}
}

func TestCGroupHandler_GetEBPFMaps(t *testing.T) {
	r, scaler := setupCGroupTestRouter()

	scaler.RegisterEBPFMap("cluster-eu", domain.EBPFMapUtilization{
		MapName:        "cilium_ct4_global",
		CurrentEntries: 220000,
		MaxEntries:     262144,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/kernel/clusters/cluster-eu/ebpf-maps", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		ClusterID string                      `json:"cluster_id"`
		Maps      []domain.EBPFMapUtilization `json:"maps"`
		Count     int                         `json:"count"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if resp.Count != 1 || resp.Maps[0].Status != domain.StatusCritical {
		t.Errorf("expected 1 map with CRITICAL, got %+v", resp)
	}
}

func TestCGroupHandler_ResizeEBPFMap_SuccessAndGuardrailRejection(t *testing.T) {
	r, _ := setupCGroupTestRouter()

	// 1. Success case
	payload := []byte(`{"new_capacity": 524288}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/kernel/clusters/cluster-eu/ebpf-maps/resize", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	// 2. Host RAM Guardrail violation (>10% RAM) -> 422
	payloadExcess := []byte(`{"new_capacity": 10000000}`)
	reqExcess := httptest.NewRequest(http.MethodPost, "/api/v1/kernel/clusters/cluster-eu/ebpf-maps/resize", bytes.NewReader(payloadExcess))
	reqExcess.Header.Set("Content-Type", "application/json")
	recExcess := httptest.NewRecorder()
	r.ServeHTTP(recExcess, reqExcess)

	if recExcess.Code != http.StatusUnprocessableEntity && recExcess.Code != http.StatusBadRequest {
		t.Fatalf("expected 422 or 400 for guardrail, got %d", recExcess.Code)
	}

	var prob adapthttp.ProblemDetails
	if err := json.NewDecoder(recExcess.Body).Decode(&prob); err != nil {
		t.Fatalf("failed to decode problem: %v", err)
	}
	if prob.Code != domain.CodeEBPFMemoryLimitExceeded && prob.Code != "BAD_REQUEST" {
		t.Errorf("unexpected error code: %s", prob.Code)
	}
}

func TestCGroupHandler_SlabTune(t *testing.T) {
	r, scaler := setupCGroupTestRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/kernel/nodes/node-beta/slab-tune", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	val, found := scaler.GetNodeSysctl("node-beta", "vm.vfs_cache_pressure")
	if !found || val != 150 {
		t.Errorf("expected sysctl 150 applied in usecase scaler, got found=%v val=%d", found, val)
	}
}
