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

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	"github.com/datdt/k8sselfhost/internal/domain/cloudresilience"
)

// stubCloudResilienceEngine implements domain.CloudResilienceEngine for HTTP handler testing.
type stubCloudResilienceEngine struct {
	checkBurstFunc   func(ctx context.Context, volumeID string) (*cloudresilience.StorageBurstStatus, error)
	upgradeGP3Func   func(ctx context.Context, volumeID string) error
	listenSpotFunc   func(ctx context.Context, handler func(event cloudresilience.SpotTerminationEvent)) error
	drainNodeFunc    func(ctx context.Context, nodeName string, gracePeriod time.Duration) error
	monitorSNATFunc  func(ctx context.Context, gatewayID string) (*cloudresilience.NATGatewayMetrics, error)
}

func (s *stubCloudResilienceEngine) CheckEBSBurstBalance(ctx context.Context, volumeID string) (*cloudresilience.StorageBurstStatus, error) {
	if s.checkBurstFunc != nil {
		return s.checkBurstFunc(ctx, volumeID)
	}
	return &cloudresilience.StorageBurstStatus{
		VolumeID:        volumeID,
		VolumeType:      "gp2",
		BurstBalancePct: 80.0,
		CurrentIOPS:     500,
	}, nil
}

func (s *stubCloudResilienceEngine) UpgradeVolumeToGP3(ctx context.Context, volumeID string) error {
	if s.upgradeGP3Func != nil {
		return s.upgradeGP3Func(ctx, volumeID)
	}
	return nil
}

func (s *stubCloudResilienceEngine) ListenSpotTerminationNotices(ctx context.Context, handler func(event cloudresilience.SpotTerminationEvent)) error {
	if s.listenSpotFunc != nil {
		return s.listenSpotFunc(ctx, handler)
	}
	return nil
}

func (s *stubCloudResilienceEngine) DrainNodeGracefully(ctx context.Context, nodeName string, gracePeriod time.Duration) error {
	if s.drainNodeFunc != nil {
		return s.drainNodeFunc(ctx, nodeName, gracePeriod)
	}
	return nil
}

func (s *stubCloudResilienceEngine) MonitorNATGatewaySNAT(ctx context.Context, gatewayID string) (*cloudresilience.NATGatewayMetrics, error) {
	if s.monitorSNATFunc != nil {
		return s.monitorSNATFunc(ctx, gatewayID)
	}
	return &cloudresilience.NATGatewayMetrics{
		GatewayID:         gatewayID,
		AllocatedIPs:      1,
		ActiveConnections: 1000,
		IsSaturated:       false,
	}, nil
}

func setupCloudResilienceTestRouter(engine cloudresilience.CloudResilienceEngine) *chi.Mux {
	r := chi.NewRouter()
	handler := adapthttp.NewCloudResilienceHandler(engine, nil)
	handler.WireToRouter(r)
	return r
}

func TestCloudResilienceHandler_GetBurstStatus_Success(t *testing.T) {
	engine := &stubCloudResilienceEngine{
		checkBurstFunc: func(ctx context.Context, volumeID string) (*cloudresilience.StorageBurstStatus, error) {
			return &cloudresilience.StorageBurstStatus{
				VolumeID:        volumeID,
				VolumeType:      "gp2",
				BurstBalancePct: 15.0,
				CurrentIOPS:     1000,
				NeedsUpgrade:    true,
			}, nil
		},
	}

	router := setupCloudResilienceTestRouter(engine)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cloud/storage/burst-status?volume_id=vol-12345678", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp cloudresilience.StorageBurstStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.VolumeID != "vol-12345678" || !resp.NeedsUpgrade {
		t.Errorf("unexpected burst status: %+v", resp)
	}
}

func TestCloudResilienceHandler_GetBurstStatus_Exhausted_500_RFC7807(t *testing.T) {
	engine := &stubCloudResilienceEngine{
		checkBurstFunc: func(ctx context.Context, volumeID string) (*cloudresilience.StorageBurstStatus, error) {
			return nil, cloudresilience.ErrCloudEBSBurstExhausted
		},
	}

	router := setupCloudResilienceTestRouter(engine)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cloud/storage/burst-status?volume_id=vol-exhausted", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("expected application/problem+json, got %s", ct)
	}

	var prob adapthttp.ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &prob); err != nil {
		t.Fatalf("failed to parse RFC 7807 problem details: %v", err)
	}
	if prob.Status != 500 || prob.Code != "ERR_CLOUD_EBS_BURST_EXHAUSTED" {
		t.Errorf("unexpected problem details: %+v", prob)
	}
}

func TestCloudResilienceHandler_GetBurstStatus_MissingVolumeID_400(t *testing.T) {
	router := setupCloudResilienceTestRouter(&stubCloudResilienceEngine{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cloud/storage/burst-status", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestCloudResilienceHandler_UpgradeGP3_Success(t *testing.T) {
	engine := &stubCloudResilienceEngine{}
	router := setupCloudResilienceTestRouter(engine)

	payload := []byte(`{"volume_id":"vol-upgrade-123"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cloud/storage/upgrade-gp3", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["status"] != "upgraded" || resp["volume_id"] != "vol-upgrade-123" {
		t.Errorf("unexpected upgrade response: %+v", resp)
	}
}

func TestCloudResilienceHandler_UpgradeGP3_Validation_400(t *testing.T) {
	router := setupCloudResilienceTestRouter(&stubCloudResilienceEngine{})

	payload := []byte(`{"volume_id":""}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cloud/storage/upgrade-gp3", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for empty volume_id, got %d", rec.Code)
	}
}

func TestCloudResilienceHandler_GetSNATMetrics_Healthy(t *testing.T) {
	engine := &stubCloudResilienceEngine{
		monitorSNATFunc: func(ctx context.Context, gatewayID string) (*cloudresilience.NATGatewayMetrics, error) {
			return &cloudresilience.NATGatewayMetrics{
				GatewayID:            gatewayID,
				AllocatedIPs:         2,
				ActiveConnections:    25000,
				PortAllocationErrors: 0,
				IsSaturated:          false,
			}, nil
		},
	}

	router := setupCloudResilienceTestRouter(engine)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cloud/nat/snat-metrics?gateway_id=nat-healthy-01", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var metrics cloudresilience.NATGatewayMetrics
	if err := json.Unmarshal(rec.Body.Bytes(), &metrics); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if metrics.IsSaturated {
		t.Errorf("expected IsSaturated=false")
	}
}

func TestCloudResilienceHandler_GetSNATMetrics_Saturated_503_RFC7807(t *testing.T) {
	engine := &stubCloudResilienceEngine{
		monitorSNATFunc: func(ctx context.Context, gatewayID string) (*cloudresilience.NATGatewayMetrics, error) {
			return nil, cloudresilience.ErrNATGatewaySaturated
		},
	}

	router := setupCloudResilienceTestRouter(engine)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cloud/nat/snat-metrics?gateway_id=nat-saturated-01", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("expected application/problem+json, got %s", ct)
	}

	var prob adapthttp.ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &prob); err != nil {
		t.Fatalf("failed to parse problem details: %v", err)
	}
	if prob.Status != 503 || prob.Code != "ERR_NAT_GATEWAY_SATURATED" {
		t.Errorf("unexpected problem details: %+v", prob)
	}
}

func TestCloudResilienceHandler_DrainNode_Success(t *testing.T) {
	engine := &stubCloudResilienceEngine{}
	router := setupCloudResilienceTestRouter(engine)

	payload := []byte(`{"node_name":"node-spot-worker-1","grace_period_sec":90}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cloud/spot/drain-node", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["status"] != "drained" || resp["node_name"] != "node-spot-worker-1" {
		t.Errorf("unexpected drain response: %+v", resp)
	}
}

func TestCloudResilienceHandler_DrainNode_ActiveDraining_503_RFC7807(t *testing.T) {
	engine := &stubCloudResilienceEngine{
		drainNodeFunc: func(ctx context.Context, nodeName string, gracePeriod time.Duration) error {
			return cloudresilience.ErrCloudSpotTerminationDraining
		},
	}

	router := setupCloudResilienceTestRouter(engine)
	payload := []byte(`{"node_name":"node-spot-draining"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cloud/spot/drain-node", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("expected application/problem+json, got %s", ct)
	}

	var prob adapthttp.ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &prob); err != nil {
		t.Fatalf("failed to decode problem details: %v", err)
	}
	if prob.Status != 503 || prob.Code != "ERR_CLOUD_SPOT_TERMINATION_DRAINING" {
		t.Errorf("unexpected problem details: %+v", prob)
	}
}
