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

	adapterHttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	domain "github.com/datdt/k8sselfhost/internal/domain/secretguard"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/secretguard"
)

// stubDetector implements domain.DriftDetector for HTTP tests.
type stubDetector struct {
	policies          []domain.ExternalSecretPolicy
	mutatedIntervals  map[string]time.Duration
	rolloutsTriggered []string
	backupMap         map[string]bool
	auditErr          error
	mutateErr         error
	rolloutErr        error
}

func newStubDetector() *stubDetector {
	return &stubDetector{
		mutatedIntervals: make(map[string]time.Duration),
		backupMap:        make(map[string]bool),
	}
}

func (s *stubDetector) AuditExternalSecrets(_ context.Context, namespace string) ([]domain.ExternalSecretPolicy, error) {
	if s.auditErr != nil {
		return nil, s.auditErr
	}
	if namespace == "" {
		return s.policies, nil
	}
	var filtered []domain.ExternalSecretPolicy
	for _, p := range s.policies {
		if p.Namespace == namespace {
			filtered = append(filtered, p)
		}
	}
	return filtered, nil
}

func (s *stubDetector) MutateRefreshInterval(_ context.Context, ns, name string, minInterval time.Duration) error {
	if s.mutateErr != nil {
		return s.mutateErr
	}
	if minInterval < domain.MinSafetyInterval {
		return domain.ErrSecretPollingRateTooHigh
	}
	s.mutatedIntervals[ns+"/"+name] = minInterval
	return nil
}

func (s *stubDetector) TriggerGracefulRollout(_ context.Context, targetDeployment string) error {
	if s.rolloutErr != nil {
		return s.rolloutErr
	}
	s.rolloutsTriggered = append(s.rolloutsTriggered, targetDeployment)
	return nil
}

func (s *stubDetector) VerifySealedKeyBackup(_ context.Context, kmsKeyURI string) (bool, error) {
	return s.backupMap[kmsKeyURI], nil
}

type stubHealthChecker struct {
	rate float64
}

func (h *stubHealthChecker) GetWorkloadErrorRate(_ context.Context, _ string) (float64, error) {
	return h.rate, nil
}

func setupSecretDriftRouter(handler *adapterHttp.SecretDriftHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Route("/api/v1/secrets/drift", func(sub chi.Router) {
		handler.RegisterRoutes(sub)
	})
	return r
}

func TestSecretDriftHandler_Audit(t *testing.T) {
	t.Parallel()

	detector := newStubDetector()
	detector.policies = []domain.ExternalSecretPolicy{
		{
			Namespace:          "prod",
			Name:               "db-pass",
			Provider:           domain.ProviderVault,
			ConfiguredInterval: 10 * time.Second,
		},
		{
			Namespace:          "prod",
			Name:               "api-key",
			Provider:           domain.ProviderAWS,
			ConfiguredInterval: 2 * time.Hour,
		},
	}

	reconciler := usecase.NewDriftReconciler(detector, nil)
	handler := adapterHttp.NewSecretDriftHandler(reconciler, detector)
	router := setupSecretDriftRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/secrets/drift/audit?namespace=prod", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		AuditedCount int                            `json:"audited_count"`
		Policies     []domain.ExternalSecretPolicy `json:"policies"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.AuditedCount != 2 {
		t.Errorf("expected 2 audited policies, got %d", resp.AuditedCount)
	}
}

func TestSecretDriftHandler_Mutate_Success(t *testing.T) {
	t.Parallel()

	detector := newStubDetector()
	reconciler := usecase.NewDriftReconciler(detector, nil)
	handler := adapterHttp.NewSecretDriftHandler(reconciler, detector)
	router := setupSecretDriftRouter(handler)

	payload := map[string]string{
		"namespace": "prod",
		"name":      "db-pass",
		"interval":  "1h",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/secrets/drift/mutate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSecretDriftHandler_Mutate_TooHighRate_Returns400(t *testing.T) {
	t.Parallel()

	detector := newStubDetector()
	reconciler := usecase.NewDriftReconciler(detector, nil)
	handler := adapterHttp.NewSecretDriftHandler(reconciler, detector)
	router := setupSecretDriftRouter(handler)

	// An interval of 10s violates the 1h minimum safety floor
	payload := map[string]string{
		"namespace": "prod",
		"name":      "db-pass",
		"interval":  "10s",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/secrets/drift/mutate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400 for aggressive interval, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSecretDriftHandler_Rollout_Success(t *testing.T) {
	t.Parallel()

	detector := newStubDetector()
	healthChecker := &stubHealthChecker{rate: 0.01} // 1% error rate
	reconciler := usecase.NewDriftReconciler(detector, healthChecker)
	handler := adapterHttp.NewSecretDriftHandler(reconciler, detector)
	router := setupSecretDriftRouter(handler)

	payload := map[string]interface{}{
		"namespace":  "prod",
		"name":       "db-secret",
		"deployment": "billing-api",
		"hash":       "new-hash-v2",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/secrets/drift/rollout", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSecretDriftHandler_Rollout_Degraded_HaltsAndReturnsError(t *testing.T) {
	t.Parallel()

	detector := newStubDetector()
	healthChecker := &stubHealthChecker{rate: 0.09} // 9% error rate (> 5% safety threshold)
	reconciler := usecase.NewDriftReconciler(detector, healthChecker)
	handler := adapterHttp.NewSecretDriftHandler(reconciler, detector)
	router := setupSecretDriftRouter(handler)

	// Baseline established first
	_ = reconciler.TrackSecretBaseline(domain.ExternalSecretPolicy{
		Namespace:      "prod",
		Name:           "db-secret",
		Provider:       domain.ProviderVault,
		SecretDataHash: "old-hash",
	})

	payload := map[string]interface{}{
		"namespace":  "prod",
		"name":       "db-secret",
		"deployment": "billing-api",
		"hash":       "new-hash-v2",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/secrets/drift/rollout", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity && rec.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 422 or 400 for degraded rollout, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSecretDriftHandler_BackupVerify(t *testing.T) {
	t.Parallel()

	detector := newStubDetector()
	detector.backupMap["kms://aws:keys/sealed-secrets-master"] = true
	reconciler := usecase.NewDriftReconciler(detector, nil)
	handler := adapterHttp.NewSecretDriftHandler(reconciler, detector)
	router := setupSecretDriftRouter(handler)

	// 1. Success case
	req := httptest.NewRequest(http.MethodGet, "/api/v1/secrets/drift/backup-verify?kms_key_uri=kms://aws:keys/sealed-secrets-master", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// 2. Missing backup case
	reqMissing := httptest.NewRequest(http.MethodGet, "/api/v1/secrets/drift/backup-verify?kms_key_uri=kms://aws:keys/unbacked-key", nil)
	recMissing := httptest.NewRecorder()
	router.ServeHTTP(recMissing, reqMissing)

	if recMissing.Code != http.StatusNotFound && recMissing.Code != http.StatusConflict {
		t.Fatalf("expected HTTP 404 or 409 for missing backup, got %d: %s", recMissing.Code, recMissing.Body.String())
	}

	// 3. Missing query param
	reqNoParam := httptest.NewRequest(http.MethodGet, "/api/v1/secrets/drift/backup-verify", nil)
	recNoParam := httptest.NewRecorder()
	router.ServeHTTP(recNoParam, reqNoParam)

	if recNoParam.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400 when kms_key_uri is missing, got %d", recNoParam.Code)
	}
}
