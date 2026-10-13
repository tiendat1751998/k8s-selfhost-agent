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
	"go.uber.org/zap"

	adapterhttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	domain "github.com/datdt/k8sselfhost/internal/domain/hardware"
)

// fakeHardwareEngine implements domain.HardwareWatchdogEngine for HTTP handler tests.
type fakeHardwareEngine struct {
	eccStatuses  []domain.ECCDIMMStatus
	eccErr       error
	thermalState *domain.CPUThermalStatus
	thermalErr   error
	psuStatuses  []domain.PowerSupplyStatus
	psuErr       error
	drainErr     error
	taintErr     error

	drainedNode   string
	drainedReason string
}

func (f *fakeHardwareEngine) AuditECCMemoryErrors(ctx context.Context) ([]domain.ECCDIMMStatus, error) {
	return f.eccStatuses, f.eccErr
}

func (f *fakeHardwareEngine) AuditCPUThermalThrottling(ctx context.Context) (*domain.CPUThermalStatus, error) {
	return f.thermalState, f.thermalErr
}

func (f *fakeHardwareEngine) AuditPowerSupplyRedundancy(ctx context.Context) ([]domain.PowerSupplyStatus, error) {
	return f.psuStatuses, f.psuErr
}

func (f *fakeHardwareEngine) DrainAndCordonDegradedNode(ctx context.Context, nodeName string, reason string) error {
	f.drainedNode = nodeName
	f.drainedReason = reason
	return f.drainErr
}

func (f *fakeHardwareEngine) TaintThermalThrottledNode(ctx context.Context, nodeName string) error {
	return f.taintErr
}

func setupHardwareTestRouter(engine domain.HardwareWatchdogEngine) *chi.Mux {
	r := chi.NewRouter()
	handler := adapterhttp.NewHardwareHandler(engine, zap.NewNop())
	r.Route("/api/v1/hardware", func(sub chi.Router) {
		handler.RegisterRoutes(sub)
	})
	return r
}

func TestHardwareHandler_GetECCDIMMStatus_Healthy_200(t *testing.T) {
	engine := &fakeHardwareEngine{
		eccStatuses: []domain.ECCDIMMStatus{
			{SlotID: "mc0/dimm0", CorrectableCount: 4, ErrorRatePerMin: 0.05, LastEventTime: time.Now()},
		},
	}
	r := setupHardwareTestRouter(engine)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/hardware/ecc/dimm-status", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}
	if resp["status"] != "healthy" {
		t.Errorf("expected status healthy, got %v", resp["status"])
	}
}

func TestHardwareHandler_GetECCDIMMStatus_Surge_500_RFC7807(t *testing.T) {
	engine := &fakeHardwareEngine{
		eccStatuses: []domain.ECCDIMMStatus{
			{SlotID: "mc0/dimm1", CorrectableCount: 500, ErrorRatePerMin: 5.2, IsDegraded: true},
		},
		eccErr: domain.ErrECCMemoryCorrectableSurge,
	}
	r := setupHardwareTestRouter(engine)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/hardware/ecc/dimm-status", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected HTTP 500, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "application/problem+json" {
		t.Errorf("expected application/problem+json, got %s", rec.Header().Get("Content-Type"))
	}

	var problem adapterhttp.ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to decode RFC 7807 problem details: %v", err)
	}
	if problem.Code != domain.CodeECCMemoryCorrectableSurge {
		t.Errorf("expected code %s, got %s", domain.CodeECCMemoryCorrectableSurge, problem.Code)
	}
}

func TestHardwareHandler_GetCPUThermalStatus_Nominal_200(t *testing.T) {
	engine := &fakeHardwareEngine{
		thermalState: &domain.CPUThermalStatus{
			PackageTempC:        55.0,
			IsThrottled:         false,
			BaseFrequencyMHz:    3600.0,
			CurrentFrequencyMHz: 3600.0,
		},
	}
	r := setupHardwareTestRouter(engine)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/hardware/cpu/thermal-status", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}
	if resp["status"] != "nominal" {
		t.Errorf("expected nominal status, got %v", resp["status"])
	}
}

func TestHardwareHandler_GetCPUThermalStatus_Throttled_503_RFC7807(t *testing.T) {
	engine := &fakeHardwareEngine{
		thermalState: &domain.CPUThermalStatus{
			PackageTempC:        89.5,
			IsThrottled:         true,
			BaseFrequencyMHz:    3600.0,
			CurrentFrequencyMHz: 1200.0,
		},
		thermalErr: domain.ErrCPUThermalThrottled,
	}
	r := setupHardwareTestRouter(engine)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/hardware/cpu/thermal-status", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected HTTP 503, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "application/problem+json" {
		t.Errorf("expected application/problem+json, got %s", rec.Header().Get("Content-Type"))
	}

	var problem adapterhttp.ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to decode RFC 7807 problem details: %v", err)
	}
	if problem.Code != domain.CodeCPUThermalThrottled {
		t.Errorf("expected code %s, got %s", domain.CodeCPUThermalThrottled, problem.Code)
	}
}

func TestHardwareHandler_GetPSURedundancyStatus_Nominal_200(t *testing.T) {
	engine := &fakeHardwareEngine{
		psuStatuses: []domain.PowerSupplyStatus{
			{PSUID: "PSU0", IsPresent: true, InputVoltage: 230, OutputPowerW: 420},
			{PSUID: "PSU1", IsPresent: true, InputVoltage: 230, OutputPowerW: 415},
		},
	}
	r := setupHardwareTestRouter(engine)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/hardware/psu/redundancy-status", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHardwareHandler_GetPSURedundancyStatus_Failed_500_RFC7807(t *testing.T) {
	engine := &fakeHardwareEngine{
		psuStatuses: []domain.PowerSupplyStatus{
			{PSUID: "PSU0", IsPresent: true, RedundancyLost: true},
			{PSUID: "PSU1", IsPresent: false, RedundancyLost: true},
		},
		psuErr: domain.ErrPSURedundancyFailed,
	}
	r := setupHardwareTestRouter(engine)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/hardware/psu/redundancy-status", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected HTTP 500, got %d: %s", rec.Code, rec.Body.String())
	}

	var problem adapterhttp.ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to decode RFC 7807 problem details: %v", err)
	}
	if problem.Code != domain.CodePSURedundancyFailed {
		t.Errorf("expected code %s, got %s", domain.CodePSURedundancyFailed, problem.Code)
	}
}

func TestHardwareHandler_DrainDegradedNode_Success_200(t *testing.T) {
	engine := &fakeHardwareEngine{}
	r := setupHardwareTestRouter(engine)

	body := []byte(`{"reason":"ECC CE burst on DIMM slot mc0/dimm1"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/hardware/nodes/worker-node-42/drain-degraded", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if engine.drainedNode != "worker-node-42" {
		t.Errorf("expected drained node worker-node-42, got %s", engine.drainedNode)
	}
	if engine.drainedReason != "ECC CE burst on DIMM slot mc0/dimm1" {
		t.Errorf("expected reason to match, got %s", engine.drainedReason)
	}
}

func TestHardwareHandler_DrainDegradedNode_EngineError_500(t *testing.T) {
	engine := &fakeHardwareEngine{
		drainErr: errors.New("kubernetes api eviction error: 504 gateway timeout"),
	}
	r := setupHardwareTestRouter(engine)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/hardware/nodes/worker-node-99/drain-degraded", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected HTTP 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHardwareHandler_DrainDegradedNode_MissingNode_400(t *testing.T) {
	engine := &fakeHardwareEngine{}
	r := setupHardwareTestRouter(engine)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/hardware/nodes//drain-degraded", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// Either 400 or 404 from chi router URL matching empty parameter
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
		t.Fatalf("expected HTTP 400 or 404 for empty node param, got %d", rec.Code)
	}
}
