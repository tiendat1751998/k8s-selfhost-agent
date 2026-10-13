package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	domain "github.com/datdt/k8sselfhost/internal/domain/netscaler"
)

type mockSocketWatchdog struct {
	metricsFn     func(ctx context.Context) (*domain.SocketMetrics, error)
	applyTuningFn func(ctx context.Context, cfg domain.KernelNetConfig) error
	healthFn      func(ctx context.Context) error
}

func (m *mockSocketWatchdog) CollectMetrics(ctx context.Context) (*domain.SocketMetrics, error) {
	if m.metricsFn != nil {
		return m.metricsFn(ctx)
	}
	return domain.NewSocketMetrics(100, 500, 28232, 0, 1000, 262144)
}

func (m *mockSocketWatchdog) ApplyKernelTuning(ctx context.Context, cfg domain.KernelNetConfig) error {
	if m.applyTuningFn != nil {
		return m.applyTuningFn(ctx, cfg)
	}
	return nil
}

func (m *mockSocketWatchdog) CheckHealth(ctx context.Context) error {
	if m.healthFn != nil {
		return m.healthFn(ctx)
	}
	return nil
}

func setupNetscalerRouter(watchdog domain.SocketWatchdog) *chi.Mux {
	handler := adapthttp.NewNetscalerHandler(watchdog, zap.NewNop())
	r := chi.NewRouter()
	r.Route("/api/v1", handler.RegisterRoutes)
	return r
}

func TestNetscalerHandler_GetSocketMetrics_Success(t *testing.T) {
	mockWatchdog := &mockSocketWatchdog{
		metricsFn: func(_ context.Context) (*domain.SocketMetrics, error) {
			return domain.NewSocketMetrics(22500, 24000, 28232, 10, 200000, 262144)
		},
	}
	r := setupNetscalerRouter(mockWatchdog)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/kernel/network/socket-metrics", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp domain.SocketMetrics
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, int64(22500), resp.TCPTimeWaitCount)
	assert.Equal(t, int64(24000), resp.AllocatedPorts)
	assert.InDelta(t, 85.01, resp.PortUtilizationPct, 0.01)
	assert.Equal(t, int64(10), resp.ListenBacklogDrops)
	assert.Equal(t, int64(200000), resp.ConntrackEntries)
	assert.InDelta(t, 76.29, resp.ConntrackUsagePct, 0.01)
}

func TestNetscalerHandler_ApplyTuning_Success(t *testing.T) {
	applied := false
	mockWatchdog := &mockSocketWatchdog{
		applyTuningFn: func(_ context.Context, cfg domain.KernelNetConfig) error {
			assert.Equal(t, 65535, cfg.Somaxconn)
			assert.Equal(t, 65535, cfg.TCPMaxSynBacklog)
			assert.Equal(t, 1, cfg.TCPTwReuse)
			assert.Equal(t, int64(524288), cfg.ConntrackMax)
			assert.Equal(t, 30, cfg.ConntrackTCPTimeWait)
			applied = true
			return nil
		},
	}
	r := setupNetscalerRouter(mockWatchdog)

	body := []byte(`{
		"somaxconn": 65535,
		"tcp_max_syn_backlog": 65535,
		"tcp_tw_reuse": 1,
		"conntrack_max": 524288,
		"conntrack_tcp_time_wait": 30
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/kernel/network/apply-tuning", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, applied)

	var resp map[string]interface{}
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "applied", resp["status"])
}

func TestNetscalerHandler_ApplyTuning_ValidationFailure(t *testing.T) {
	mockWatchdog := &mockSocketWatchdog{}
	r := setupNetscalerRouter(mockWatchdog)

	// Exceeds conntrack ceiling
	body := []byte(`{
		"somaxconn": 65535,
		"tcp_max_syn_backlog": 65535,
		"tcp_tw_reuse": 1,
		"conntrack_max": 99999999,
		"conntrack_tcp_time_wait": 30
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/kernel/network/apply-tuning", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var problem adapthttp.ProblemDetails
	err := json.NewDecoder(rec.Body).Decode(&problem)
	require.NoError(t, err)
	assert.Equal(t, domain.CodeInvalidNetConfig, problem.Code)
}

func TestNetscalerHandler_ApplyTuning_KernelSysctlFailure(t *testing.T) {
	mockWatchdog := &mockSocketWatchdog{
		applyTuningFn: func(_ context.Context, _ domain.KernelNetConfig) error {
			return domain.ErrKernelSysctlFailed
		},
	}
	r := setupNetscalerRouter(mockWatchdog)

	body := []byte(`{
		"somaxconn": 65535,
		"tcp_max_syn_backlog": 65535,
		"tcp_tw_reuse": 1,
		"conntrack_max": 524288,
		"conntrack_tcp_time_wait": 30
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/kernel/network/apply-tuning", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	var problem adapthttp.ProblemDetails
	err := json.NewDecoder(rec.Body).Decode(&problem)
	require.NoError(t, err)
	assert.Equal(t, domain.CodeKernelSysctlFailed, problem.Code)
}

func TestNetscalerHandler_CheckHealth_Healthy(t *testing.T) {
	mockWatchdog := &mockSocketWatchdog{
		healthFn: func(_ context.Context) error {
			return nil
		},
	}
	r := setupNetscalerRouter(mockWatchdog)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/kernel/network/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]interface{}
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "healthy", resp["status"])
}

func TestNetscalerHandler_CheckHealth_EphemeralPortsExhausted(t *testing.T) {
	mockWatchdog := &mockSocketWatchdog{
		healthFn: func(_ context.Context) error {
			return domain.ErrNetEphemeralPortsExhausted
		},
	}
	r := setupNetscalerRouter(mockWatchdog)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/kernel/network/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// RFC 7807 503 Service Unavailable
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	var problem adapthttp.ProblemDetails
	err := json.NewDecoder(rec.Body).Decode(&problem)
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, problem.Status)
	assert.Equal(t, domain.CodeNetEphemeralPortsExhausted, problem.Code)
}

func TestNetscalerHandler_CheckHealth_ConntrackSaturation(t *testing.T) {
	mockWatchdog := &mockSocketWatchdog{
		healthFn: func(_ context.Context) error {
			return domain.ErrNetConntrackSaturation
		},
	}
	r := setupNetscalerRouter(mockWatchdog)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/kernel/network/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// RFC 7807 500 Internal Server Error
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	var problem adapthttp.ProblemDetails
	err := json.NewDecoder(rec.Body).Decode(&problem)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, problem.Status)
	assert.Equal(t, domain.CodeNetConntrackSaturation, problem.Code)
}
