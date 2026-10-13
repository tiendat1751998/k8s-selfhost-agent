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
	"github.com/datdt/k8sselfhost/internal/domain/apfguard"
)

type stubResilienceEngine struct {
	auditFunc    func(ctx context.Context) ([]apfguard.APFQueueStatus, error)
	mitigateFunc func(ctx context.Context, policy apfguard.WatchCacheCompactionPolicy) error
	hpaFunc      func(ctx context.Context, ns, dep string) (*apfguard.HPAAutoscaleGuardReport, error)
}

func (s *stubResilienceEngine) AuditAPFQueueStatus(ctx context.Context) ([]apfguard.APFQueueStatus, error) {
	if s.auditFunc != nil {
		return s.auditFunc(ctx)
	}
	return []apfguard.APFQueueStatus{}, nil
}

func (s *stubResilienceEngine) MitigateWatchCacheCompactionThunderingHerd(ctx context.Context, policy apfguard.WatchCacheCompactionPolicy) error {
	if s.mitigateFunc != nil {
		return s.mitigateFunc(ctx, policy)
	}
	return nil
}

func (s *stubResilienceEngine) CheckHPAHealthAndApplyFallback(ctx context.Context, ns, dep string) (*apfguard.HPAAutoscaleGuardReport, error) {
	if s.hpaFunc != nil {
		return s.hpaFunc(ctx, ns, dep)
	}
	return &apfguard.HPAAutoscaleGuardReport{
		MetricsAPIAvailable: true,
		LastSampleTime:      time.Now().UTC(),
		FallbackTriggered:   false,
		TargetReplicas:      3,
		CurrentReplicas:     3,
	}, nil
}

func setupAPFGuardRouter(engine apfguard.K8sControlPlaneResilienceEngine) *chi.Mux {
	r := chi.NewRouter()
	handler := adapthttp.NewAPFGuardHandler(engine, nil)
	handler.WireToRouter(r)
	return r
}

func TestAPFGuardHandler_AuditQueueStatus(t *testing.T) {
	engine := &stubResilienceEngine{
		auditFunc: func(ctx context.Context) ([]apfguard.APFQueueStatus, error) {
			return []apfguard.APFQueueStatus{
				apfguard.NewAPFQueueStatus("system-leader-election", 0, 5, 10, 0),
				apfguard.NewAPFQueueStatus("workload-high", 2, 20, 50, 0),
			}, nil
		},
	}

	router := setupAPFGuardRouter(engine)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/k8s/apf/queue-status", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var statuses []apfguard.APFQueueStatus
	if err := json.NewDecoder(rr.Body).Decode(&statuses); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}
}

func TestAPFGuardHandler_MitigateThunderingHerd(t *testing.T) {
	var capturedPolicy apfguard.WatchCacheCompactionPolicy
	engine := &stubResilienceEngine{
		mitigateFunc: func(ctx context.Context, policy apfguard.WatchCacheCompactionPolicy) error {
			capturedPolicy = policy
			return nil
		},
	}

	router := setupAPFGuardRouter(engine)

	reqBody := `{"max_jitter_window": "45s", "batch_size": 250, "compaction_age": "10m"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/k8s/watch-cache/mitigate-thundering-herd", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	if capturedPolicy.BatchSize != 250 {
		t.Fatalf("expected batch_size 250, got %d", capturedPolicy.BatchSize)
	}
	if capturedPolicy.MaxJitterWindow != 45*time.Second {
		t.Fatalf("expected max_jitter_window 45s, got %v", capturedPolicy.MaxJitterWindow)
	}
}

func TestAPFGuardHandler_CheckHPAFallback_Success(t *testing.T) {
	engine := &stubResilienceEngine{
		hpaFunc: func(ctx context.Context, ns, dep string) (*apfguard.HPAAutoscaleGuardReport, error) {
			return &apfguard.HPAAutoscaleGuardReport{
				MetricsAPIAvailable: false,
				LastSampleTime:      time.Now().UTC(),
				FallbackTriggered:   true,
				TargetReplicas:      6,
				CurrentReplicas:     4,
				Message:             "fallback triggered",
			}, nil
		},
	}

	router := setupAPFGuardRouter(engine)

	reqBody := `{"namespace": "prod", "deployment": "api-gateway"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/k8s/hpa/check-health-fallback", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var report apfguard.HPAAutoscaleGuardReport
	if err := json.NewDecoder(rr.Body).Decode(&report); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !report.FallbackTriggered || report.TargetReplicas != 6 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestAPFGuardHandler_CheckHPAFallback_InvalidParams(t *testing.T) {
	engine := &stubResilienceEngine{}
	router := setupAPFGuardRouter(engine)

	reqBody := `{"namespace": "", "deployment": ""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/k8s/hpa/check-health-fallback", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for empty params, got %d", rr.Code)
	}
}

func TestAPFGuardHandler_ErrorMapping_RFC7807(t *testing.T) {
	engine := &stubResilienceEngine{
		auditFunc: func(ctx context.Context) ([]apfguard.APFQueueStatus, error) {
			return nil, apfguard.ErrAPFQueueSaturated
		},
	}
	router := setupAPFGuardRouter(engine)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/k8s/apf/queue-status", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503 for ErrAPFQueueSaturated, got %d", rr.Code)
	}
	if rr.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf("expected Content-Type application/problem+json, got %s", rr.Header().Get("Content-Type"))
	}
}
