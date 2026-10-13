package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	domain "github.com/datdt/k8sselfhost/internal/domain/spiffeguard"
)

// fakeWorkloadIdentityManager implements domain.WorkloadIdentityManager for testing
type fakeWorkloadIdentityManager struct {
	inspectSVIDFunc       func(ctx context.Context, socketPath string) (*domain.SVIDStatus, error)
	renewSVIDFunc         func(ctx context.Context, socketPath string, maxJitter time.Duration) (*domain.SVIDStatus, error)
	watchTokenFunc        func(ctx context.Context, tokenPath string, leeway time.Duration) (*domain.TokenWatchStatus, error)
	validateClockSkewFunc func(ctx context.Context, tokenIAT time.Time, allowedSkew time.Duration) error
}

func (f *fakeWorkloadIdentityManager) InspectSVID(ctx context.Context, socketPath string) (*domain.SVIDStatus, error) {
	if f.inspectSVIDFunc != nil {
		return f.inspectSVIDFunc(ctx, socketPath)
	}
	return domain.NewSVIDStatus("spiffe://k8scontrol.prod/ns/default/sa/test", time.Now().Add(1*time.Hour), 1*time.Hour, "CERT-1234"), nil
}

func (f *fakeWorkloadIdentityManager) RenewSVIDWithJitter(ctx context.Context, socketPath string, maxJitter time.Duration) (*domain.SVIDStatus, error) {
	if f.renewSVIDFunc != nil {
		return f.renewSVIDFunc(ctx, socketPath, maxJitter)
	}
	return domain.NewSVIDStatus("spiffe://k8scontrol.prod/ns/default/sa/test", time.Now().Add(1*time.Hour), 1*time.Hour, "CERT-RENEWED"), nil
}

func (f *fakeWorkloadIdentityManager) WatchProjectedToken(ctx context.Context, tokenPath string, leeway time.Duration) (*domain.TokenWatchStatus, error) {
	if f.watchTokenFunc != nil {
		return f.watchTokenFunc(ctx, tokenPath, leeway)
	}
	return domain.NewTokenWatchStatus("/var/run/secrets/token", time.Now(), time.Now().Add(1*time.Hour), 0.2), nil
}

func (f *fakeWorkloadIdentityManager) ValidateTokenClockSkew(ctx context.Context, tokenIAT time.Time, allowedSkew time.Duration) error {
	if f.validateClockSkewFunc != nil {
		return f.validateClockSkewFunc(ctx, tokenIAT, allowedSkew)
	}
	return nil
}

func setupTestRouter(handler *SPIFFEGuardHandler) *chi.Mux {
	r := chi.NewRouter()
	handler.Mount(r)
	return r
}

func TestGetSVIDStatus_Success(t *testing.T) {
	fakeMgr := &fakeWorkloadIdentityManager{}
	h := NewSPIFFEGuardHandler(fakeMgr)
	router := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/identity/spiffe/svid-status?socket_path=/tmp/spire.sock", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	var status domain.SVIDStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if status.SPIFFEID != "spiffe://k8scontrol.prod/ns/default/sa/test" {
		t.Errorf("unexpected SPIFFE ID: %s", status.SPIFFEID)
	}
}

func TestGetSVIDStatus_BacklogSaturated(t *testing.T) {
	fakeMgr := &fakeWorkloadIdentityManager{
		inspectSVIDFunc: func(ctx context.Context, socketPath string) (*domain.SVIDStatus, error) {
			return nil, domain.ErrSVIDBacklogSaturated
		},
	}
	h := NewSPIFFEGuardHandler(fakeMgr)
	router := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/identity/spiffe/svid-status", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rec.Code)
	}

	var prob ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &prob); err != nil {
		t.Fatalf("failed to decode problem json: %v", err)
	}
	if prob.Code != "ERR_SVID_BACKLOG_SATURATED" {
		t.Errorf("expected code ERR_SVID_BACKLOG_SATURATED, got %s", prob.Code)
	}
}

func TestRenewSVIDJittered_Success(t *testing.T) {
	fakeMgr := &fakeWorkloadIdentityManager{}
	h := NewSPIFFEGuardHandler(fakeMgr)
	router := setupTestRouter(h)

	body := []byte(`{"socket_path": "/tmp/spire.sock", "max_jitter_ms": 200}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/identity/spiffe/renew-jittered", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", rec.Code, rec.Body.String())
	}
}

func TestRenewSVIDJittered_BacklogSaturated(t *testing.T) {
	fakeMgr := &fakeWorkloadIdentityManager{
		renewSVIDFunc: func(ctx context.Context, socketPath string, maxJitter time.Duration) (*domain.SVIDStatus, error) {
			return nil, domain.ErrSVIDBacklogSaturated
		},
	}
	h := NewSPIFFEGuardHandler(fakeMgr)
	router := setupTestRouter(h)

	body := []byte(`{"socket_path": "/tmp/spire.sock", "max_jitter_ms": 100}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/identity/spiffe/renew-jittered", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rec.Code)
	}
}

func TestGetProjectedTokenStatus_Success(t *testing.T) {
	fakeMgr := &fakeWorkloadIdentityManager{}
	h := NewSPIFFEGuardHandler(fakeMgr)
	router := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/identity/token/projected-status", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", rec.Code, rec.Body.String())
	}
}

func TestGetProjectedTokenStatus_StaleExpired(t *testing.T) {
	fakeMgr := &fakeWorkloadIdentityManager{
		watchTokenFunc: func(ctx context.Context, tokenPath string, leeway time.Duration) (*domain.TokenWatchStatus, error) {
			return nil, domain.ErrTokenStaleExpired
		},
	}
	h := NewSPIFFEGuardHandler(fakeMgr)
	router := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/identity/token/projected-status", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}

	var prob ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &prob); err != nil {
		t.Fatalf("failed to decode problem json: %v", err)
	}
	if prob.Code != "ERR_TOKEN_STALE_EXPIRED" {
		t.Errorf("expected code ERR_TOKEN_STALE_EXPIRED, got %s", prob.Code)
	}
}

func TestValidateClockSkew_Success(t *testing.T) {
	fakeMgr := &fakeWorkloadIdentityManager{}
	h := NewSPIFFEGuardHandler(fakeMgr)
	router := setupTestRouter(h)

	body := []byte(`{"token_iat": 1728800000, "allowed_skew_secs": 60}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/identity/token/validate-clock-skew", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", rec.Code, rec.Body.String())
	}
}

func TestValidateClockSkew_Exceeded(t *testing.T) {
	fakeMgr := &fakeWorkloadIdentityManager{
		validateClockSkewFunc: func(ctx context.Context, tokenIAT time.Time, allowedSkew time.Duration) error {
			return domain.ErrTokenClockSkewExceeded
		},
	}
	h := NewSPIFFEGuardHandler(fakeMgr)
	router := setupTestRouter(h)

	body := []byte(`{"token_iat": 1728800000, "allowed_skew_secs": 60}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/identity/token/validate-clock-skew", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}

	var prob ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &prob); err != nil {
		t.Fatalf("failed to decode problem json: %v", err)
	}
	if prob.Code != "ERR_TOKEN_CLOCK_SKEW_EXCEEDED" {
		t.Errorf("expected code ERR_TOKEN_CLOCK_SKEW_EXCEEDED, got %s", prob.Code)
	}
}

func TestValidateClockSkew_InvalidBody(t *testing.T) {
	fakeMgr := &fakeWorkloadIdentityManager{}
	h := NewSPIFFEGuardHandler(fakeMgr)
	router := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/identity/token/validate-clock-skew", bytes.NewReader([]byte(`invalid-json`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}
