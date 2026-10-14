package http_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
)

func computeHMACSHA256(payload []byte, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestGitOpsHandler_Webhook_FailClosedWhenSecretUnset(t *testing.T) {
	// Handler constructed without WithWebhookSecret -> secret is nil/empty
	handler := adapthttp.NewGitOpsHandler(nil)
	r := chi.NewRouter()
	r.Post("/api/v1/gitops/webhook", handler.HandleWebhook)

	payload := []byte(`{"action":"push","ref":"refs/heads/main"}`)

	// Case 1: No signature header sent -> must fail closed with 503
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/gitops/webhook", bytes.NewReader(payload))
	rec1 := httptest.NewRecorder()
	r.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 Service Unavailable when secret unset, got %d: %s", rec1.Code, rec1.Body.String())
	}

	// Case 2: Forged signature header sent -> MUST STILL fail closed with 503, not 200
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/gitops/webhook", bytes.NewReader(payload))
	req2.Header.Set("X-Hub-Signature-256", "sha256=abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890")
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 Service Unavailable even with signature header when secret unset, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

func TestGitOpsHandler_Webhook_ValidAndInvalidSignatures(t *testing.T) {
	secret := []byte("super-secure-webhook-secret-token")
	handler := adapthttp.NewGitOpsHandler(nil, adapthttp.WithWebhookSecret(secret))
	r := chi.NewRouter()
	r.Post("/api/v1/gitops/webhook", handler.HandleWebhook)

	payload := []byte(`{"action":"synchronize","pull_request":{"id":42}}`)

	// Case 1: Missing signature header -> 401
	reqMissing := httptest.NewRequest(http.MethodPost, "/api/v1/gitops/webhook", bytes.NewReader(payload))
	recMissing := httptest.NewRecorder()
	r.ServeHTTP(recMissing, reqMissing)
	if recMissing.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing signature, got %d: %s", recMissing.Code, recMissing.Body.String())
	}

	// Case 2: Invalid signature -> 401
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/gitops/webhook", bytes.NewReader(payload))
	reqInvalid.Header.Set("X-Hub-Signature-256", "sha256=0000000000000000000000000000000000000000000000000000000000000000")
	recInvalid := httptest.NewRecorder()
	r.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid signature, got %d: %s", recInvalid.Code, recInvalid.Body.String())
	}

	// Case 3: Valid GitHub signature -> 200
	validSig := computeHMACSHA256(payload, secret)
	reqValid := httptest.NewRequest(http.MethodPost, "/api/v1/gitops/webhook", bytes.NewReader(payload))
	reqValid.Header.Set("X-Hub-Signature-256", validSig)
	recValid := httptest.NewRecorder()
	r.ServeHTTP(recValid, reqValid)
	if recValid.Code != http.StatusOK {
		t.Errorf("expected 200 for valid GitHub signature, got %d: %s", recValid.Code, recValid.Body.String())
	}

	// Case 4: Valid GitLab token -> 200
	reqGitLab := httptest.NewRequest(http.MethodPost, "/api/v1/gitops/webhook", bytes.NewReader(payload))
	reqGitLab.Header.Set("X-Gitlab-Token", string(secret))
	recGitLab := httptest.NewRecorder()
	r.ServeHTTP(recGitLab, reqGitLab)
	if recGitLab.Code != http.StatusOK {
		t.Errorf("expected 200 for valid GitLab token, got %d: %s", recGitLab.Code, recGitLab.Body.String())
	}
}
