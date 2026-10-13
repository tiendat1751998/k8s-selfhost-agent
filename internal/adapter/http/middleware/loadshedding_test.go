package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/promotion"
)

func TestLoadShedding_UnderNormalCapacity_AllowsAll(t *testing.T) {
	monitor := middleware.NewAtomicCapacityMonitor(50.0, 10)
	mw := middleware.AdaptiveLoadShedding(middleware.LoadSheddingConfig{
		CPUThresholdPercent: 90.0,
		RetryAfterSeconds:   5,
		Monitor:             monitor,
	})

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	// Non-critical endpoint
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/metrics", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	assert.Equal(t, http.StatusOK, rec1.Code)

	// Critical endpoint
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusOK, rec2.Code)
}

func TestLoadShedding_ProtectsCriticalEndpoints(t *testing.T) {
	// Overload: CPU at 95% > 90%
	monitor := middleware.NewAtomicCapacityMonitor(95.0, 100)
	mw := middleware.AdaptiveLoadShedding(middleware.LoadSheddingConfig{
		CPUThresholdPercent: 90.0,
		RetryAfterSeconds:   5,
		Monitor:             monitor,
	})

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	}))

	// Critical endpoints should PASS THROUGH under overload
	criticalPaths := []string{
		"/healthz",
		"/readyz",
		"/livez",
		"/api/v1/auth/login",
		"/api/v1/billing/checkout",
		"/api/v1/deployments/create",
		"/api/v1/promotions/promote",
		"/api/v1/backup/restore",
		"/api/v1/dr/failover",
	}

	for _, path := range criticalPaths {
		t.Run("critical_"+path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusOK, rec.Code, "critical endpoint %s must not be shed", path)
		})
	}

	// Non-critical endpoints MUST BE SHED with HTTP 503 and Retry-After: 5
	nonCriticalPaths := []string{
		"/api/v1/analytics/realtime",
		"/api/v1/logs/preview",
		"/api/v1/metrics/export",
		"/api/v1/reports-center/summary",
		"/api/v1/search/query",
	}

	for _, path := range nonCriticalPaths {
		t.Run("noncritical_"+path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
			assert.Equal(t, "5", rec.Header().Get("Retry-After"))
			assert.Equal(t, "application/problem+json", rec.Header().Get("Content-Type"))

			var problem struct {
				Title  string `json:"title"`
				Status int    `json:"status"`
				Detail string `json:"detail"`
				Code   string `json:"code"`
			}
			err := json.Unmarshal(rec.Body.Bytes(), &problem)
			require.NoError(t, err)
			assert.Equal(t, 503, problem.Status)
			assert.Equal(t, promotion.ErrCodeSystemOverloadLoadShedding, problem.Code)
			assert.Contains(t, problem.Detail, "System overload detected")
		})
	}
}

func TestLoadShedding_ActiveLoadThreshold(t *testing.T) {
	// CPU is low (40%), but active load is 600 > 500 threshold
	monitor := middleware.NewAtomicCapacityMonitor(40.0, 600)
	mw := middleware.AdaptiveLoadShedding(middleware.LoadSheddingConfig{
		CPUThresholdPercent: 90.0,
		ActiveLoadThreshold: 500,
		RetryAfterSeconds:   5,
		Monitor:             monitor,
	})

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Equal(t, "5", rec.Header().Get("Retry-After"))
}
