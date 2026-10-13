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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	adapterhttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	"github.com/datdt/k8sselfhost/internal/domain/rdmawatch"
)

type mockResilienceEngine struct {
	links            []rdmawatch.GPUFabricBandwidth
	auditErr         error
	status           *rdmawatch.RDMAWatchdogStatus
	deadlockErr      error
	preflightValid   bool
	preflightReason  string
	preflightErr     error
	checkpointDelay  time.Duration
	checkpointErr    error
}

func (m *mockResilienceEngine) AuditGPUFabricBandwidth(ctx context.Context) ([]rdmawatch.GPUFabricBandwidth, error) {
	if m.auditErr != nil {
		return m.links, m.auditErr
	}
	return m.links, nil
}

func (m *mockResilienceEngine) DetectPFCDeadlock(ctx context.Context, nicName string) (*rdmawatch.RDMAWatchdogStatus, error) {
	if m.deadlockErr != nil {
		return m.status, m.deadlockErr
	}
	return m.status, nil
}

func (m *mockResilienceEngine) ValidatePodRDMAPreflight(ctx context.Context, podYAML []byte) (bool, string, error) {
	return m.preflightValid, m.preflightReason, m.preflightErr
}

func (m *mockResilienceEngine) CoordinateStaggeredCheckpoint(ctx context.Context, rankID, totalRanks int, policy rdmawatch.CheckpointWritePolicy) (time.Duration, error) {
	if m.checkpointErr != nil {
		return 0, m.checkpointErr
	}
	return m.checkpointDelay, nil
}

func setupRDMARouter(engine rdmawatch.GPUAIResilienceEngine) *chi.Mux {
	handler := adapterhttp.NewRDMAWatchHandler(engine)
	r := chi.NewRouter()
	r.Route("/api/v1/ai/rdma", handler.RegisterRoutes)
	return r
}

func TestRDMAWatchHandler_AuditFabricBandwidth_Healthy(t *testing.T) {
	engine := &mockResilienceEngine{
		links: []rdmawatch.GPUFabricBandwidth{
			{DeviceID: "gpu-0", NVLinkID: 0, BandwidthGBps: 900.0, IsDegraded: false},
			{DeviceID: "gpu-0", NVLinkID: 1, BandwidthGBps: 898.0, IsDegraded: false},
		},
	}
	r := setupRDMARouter(engine)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/rdma/fabric-bandwidth", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "HEALTHY", resp["status"])
	assert.Equal(t, float64(2), resp["count"])
}

func TestRDMAWatchHandler_AuditFabricBandwidth_DegradedAsymmetric(t *testing.T) {
	engine := &mockResilienceEngine{
		links: []rdmawatch.GPUFabricBandwidth{
			{DeviceID: "gpu-0", NVLinkID: 0, BandwidthGBps: 50.0, IsDegraded: true},
		},
		auditErr: rdmawatch.ErrGPUFabricAsymmetric,
	}
	r := setupRDMARouter(engine)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/rdma/fabric-bandwidth", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/problem+json")

	var problem adapterhttp.ProblemDetails
	err := json.Unmarshal(rec.Body.Bytes(), &problem)
	require.NoError(t, err)
	assert.Equal(t, rdmawatch.CodeGPUFabricAsymmetric, problem.Code)
}

func TestRDMAWatchHandler_PFCDeadlock_Detected(t *testing.T) {
	engine := &mockResilienceEngine{
		status: &rdmawatch.RDMAWatchdogStatus{
			InterfaceName:    "mlx5_0",
			PFCPausesRx:      65000,
			DeadlockDetected: true,
			AlertEmitted:     true,
		},
		deadlockErr: rdmawatch.ErrRDMAPFCDeadlock,
	}
	r := setupRDMARouter(engine)

	body, _ := json.Marshal(map[string]string{"nic_name": "mlx5_0"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/rdma/pfc-deadlock-check", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/problem+json")

	var problem adapterhttp.ProblemDetails
	err := json.Unmarshal(rec.Body.Bytes(), &problem)
	require.NoError(t, err)
	assert.Equal(t, rdmawatch.CodeRDMAPFCDeadlock, problem.Code)
}

func TestRDMAWatchHandler_PFCDeadlock_Healthy(t *testing.T) {
	engine := &mockResilienceEngine{
		status: &rdmawatch.RDMAWatchdogStatus{
			InterfaceName:    "mlx5_0",
			PFCPausesRx:      120,
			DeadlockDetected: false,
		},
	}
	r := setupRDMARouter(engine)

	body, _ := json.Marshal(map[string]string{"nic_name": "mlx5_0"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/rdma/pfc-deadlock-check", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp rdmawatch.RDMAWatchdogStatus
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.DeadlockDetected)
}

func TestRDMAWatchHandler_PFCDeadlock_MissingNIC(t *testing.T) {
	engine := &mockResilienceEngine{}
	r := setupRDMARouter(engine)

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/rdma/pfc-deadlock-check", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRDMAWatchHandler_PodPreflight_Valid(t *testing.T) {
	engine := &mockResilienceEngine{
		preflightValid:  true,
		preflightReason: "pod RDMA preflight passed",
	}
	r := setupRDMARouter(engine)

	body, _ := json.Marshal(map[string]string{"pod_yaml": "apiVersion: v1\nkind: Pod\n"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/rdma/pod-preflight", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp["valid"].(bool))
}

func TestRDMAWatchHandler_PodPreflight_InvalidMissingIPCLock(t *testing.T) {
	engine := &mockResilienceEngine{
		preflightValid:  false,
		preflightReason: "missing IPC_LOCK capability",
		preflightErr:    rdmawatch.ErrRDMAMemlockRestricted,
	}
	r := setupRDMARouter(engine)

	body, _ := json.Marshal(map[string]string{"pod_yaml": "apiVersion: v1\nkind: Pod\n"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/rdma/pod-preflight", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/problem+json")

	var problem adapterhttp.ProblemDetails
	err := json.Unmarshal(rec.Body.Bytes(), &problem)
	require.NoError(t, err)
	assert.Equal(t, rdmawatch.CodeRDMAMemlockRestricted, problem.Code)
}

func TestRDMAWatchHandler_StaggerCheckpoint_Valid(t *testing.T) {
	engine := &mockResilienceEngine{
		checkpointDelay: 4 * time.Second,
	}
	r := setupRDMARouter(engine)

	payload := map[string]interface{}{
		"rank_id":     2,
		"total_ranks": 8,
		"policy": map[string]interface{}{
			"max_concurrent_writes": 4,
			"max_jitter_window_ms": 16000,
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/rdma/stagger-checkpoint", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(2), resp["rank_id"])
	assert.Equal(t, float64(4000), resp["delay_ms"])
}

func TestRDMAWatchHandler_StaggerCheckpoint_InvalidRanks(t *testing.T) {
	engine := &mockResilienceEngine{}
	r := setupRDMARouter(engine)

	payload := map[string]interface{}{
		"rank_id":     10,
		"total_ranks": 8,
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/rdma/stagger-checkpoint", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
