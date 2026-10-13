package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	domain "github.com/datdt/k8sselfhost/internal/domain/ebpfwatch"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/ebpfwatch"
)

func setupEBPFWatchTestRouter(btfSupported bool) (*chi.Mux, *usecase.WatchdogManager) {
	mgr := usecase.NewWatchdogManager(
		usecase.WithBTFChecker(func(context.Context) bool { return btfSupported }),
		usecase.WithLogger(zap.NewNop()),
	)
	handler := adapthttp.NewEBPFWatchHandler(mgr, zap.NewNop())

	r := chi.NewRouter()
	r.Route("/api/v1/ebpf", handler.RegisterRoutes)

	return r, mgr
}

func TestEBPFWatchHandler_GetPressure_Success(t *testing.T) {
	r, mgr := setupEBPFWatchTestRouter(true)
	mgr.RegisterProbe("probe_execve", 256)
	mgr.RecordTelemetry("probe_execve", domain.RingBufferTelemetry{
		ProbeName:       "probe_execve",
		TotalEvents:     15000,
		LostEvents:      0,
		BufferUsagePct:  45.0,
		AvgProcTimeNs:   650,
		AllocatedPages:  256,
		SampleTimestamp: time.Now().UTC(),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ebpf/probes/pressure", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Count       int                          `json:"count"`
		Telemetries []domain.RingBufferTelemetry `json:"telemetries"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Count != 1 || len(resp.Telemetries) != 1 {
		t.Errorf("expected 1 telemetry, got count=%d len=%d", resp.Count, len(resp.Telemetries))
	}
	if resp.Telemetries[0].ProbeName != "probe_execve" {
		t.Errorf("expected probe_execve, got %s", resp.Telemetries[0].ProbeName)
	}
}

func TestEBPFWatchHandler_GetPressure_OverflowError(t *testing.T) {
	r, mgr := setupEBPFWatchTestRouter(true)
	mgr.RegisterProbe("probe_drop", 256)
	mgr.RecordTelemetry("probe_drop", domain.RingBufferTelemetry{
		ProbeName:       "probe_drop",
		TotalEvents:     80000,
		LostEvents:      12,
		BufferUsagePct:  88.0,
		AvgProcTimeNs:   1100,
		AllocatedPages:  256,
		SampleTimestamp: time.Now().UTC(),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ebpf/probes/pressure", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 Internal Server Error for ring buffer overflow, got %d: %s", rec.Code, rec.Body.String())
	}

	var prob adapthttp.ProblemDetails
	if err := json.NewDecoder(rec.Body).Decode(&prob); err != nil {
		t.Fatalf("failed to decode RFC 7807 problem details: %v", err)
	}

	if prob.Code != domain.CodeRingBufferOverflow {
		t.Errorf("expected problem code %s, got %s", domain.CodeRingBufferOverflow, prob.Code)
	}
}

func TestEBPFWatchHandler_ExpandProbe_Success(t *testing.T) {
	r, mgr := setupEBPFWatchTestRouter(true)
	mgr.RegisterProbe("probe_scalable", 256)

	payload := []byte(`{"pages": 512}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ebpf/probes/probe_scalable/expand", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Probe string `json:"probe"`
		Pages int    `json:"pages"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Pages != 512 {
		t.Errorf("expected pages to be 512, got %d", resp.Pages)
	}

	// Verify in usecase manager
	actualPages, _ := mgr.GetProbePages("probe_scalable")
	if actualPages != 512 {
		t.Errorf("expected manager pages to be 512, got %d", actualPages)
	}
}

func TestEBPFWatchHandler_ExpandProbe_NotFound(t *testing.T) {
	r, _ := setupEBPFWatchTestRouter(true)

	payload := []byte(`{"pages": 512}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ebpf/probes/nonexistent/expand", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d", rec.Code)
	}
}

func TestEBPFWatchHandler_ValidateHook_SafeTracepoint(t *testing.T) {
	r, _ := setupEBPFWatchTestRouter(true)

	payload := []byte(`{"hook_type": "tracepoint", "symbol": "raw_syscalls:sys_enter"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ebpf/hooks/validate", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Valid bool   `json:"valid"`
		Hook  string `json:"hook_type"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if !resp.Valid {
		t.Errorf("expected valid to be true")
	}
}

func TestEBPFWatchHandler_ValidateHook_RejectsVolatileKprobe(t *testing.T) {
	r, _ := setupEBPFWatchTestRouter(true)

	payload := []byte(`{"hook_type": "kprobe_raw", "symbol": "do_exit"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ebpf/hooks/validate", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for volatile kprobe, got %d: %s", rec.Code, rec.Body.String())
	}

	var prob adapthttp.ProblemDetails
	if err := json.NewDecoder(rec.Body).Decode(&prob); err != nil {
		t.Fatalf("failed to decode problem details: %v", err)
	}

	if prob.Code != domain.CodeUnstableKprobeRejected {
		t.Errorf("expected code %s, got %s", domain.CodeUnstableKprobeRejected, prob.Code)
	}
}

func TestEBPFWatchHandler_ValidateHook_RejectsMissingBTF(t *testing.T) {
	// Router with BTF disabled
	r, _ := setupEBPFWatchTestRouter(false)

	payload := []byte(`{"hook_type": "kprobe_raw", "symbol": "tcp_v4_connect"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ebpf/hooks/validate", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity && rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 422 or 400 when BTF is missing, got %d: %s", rec.Code, rec.Body.String())
	}

	var prob adapthttp.ProblemDetails
	if err := json.NewDecoder(rec.Body).Decode(&prob); err != nil {
		t.Fatalf("failed to decode problem details: %v", err)
	}

	if prob.Code != domain.CodeBTFMissing {
		t.Errorf("expected code %s, got %s", domain.CodeBTFMissing, prob.Code)
	}
}
