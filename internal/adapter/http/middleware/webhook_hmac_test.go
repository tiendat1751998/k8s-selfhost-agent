package middleware_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
)

func computeGitHubSig(payload []byte, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestWebhookHMACVerifier_GitHubSignature(t *testing.T) {
	verifier := middleware.NewWebhookVerifier()
	secret := []byte("super-secret-webhook-key-12345")
	payload := []byte(`{"ref":"refs/heads/main","repository":{"name":"k8sselfhost"}}`)

	t.Run("valid signature returns true", func(t *testing.T) {
		sig := computeGitHubSig(payload, secret)
		assert.True(t, verifier.VerifyGitHubSignature(payload, sig, secret))
	})

	t.Run("invalid signature returns false", func(t *testing.T) {
		invalidSig := "sha256=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
		assert.False(t, verifier.VerifyGitHubSignature(payload, invalidSig, secret))
	})

	t.Run("missing sha256 prefix returns false", func(t *testing.T) {
		mac := hmac.New(sha256.New, secret)
		mac.Write(payload)
		rawHex := hex.EncodeToString(mac.Sum(nil))
		assert.False(t, verifier.VerifyGitHubSignature(payload, rawHex, secret))
	})

	t.Run("empty signature header returns false", func(t *testing.T) {
		assert.False(t, verifier.VerifyGitHubSignature(payload, "", secret))
	})

	t.Run("empty payload returns false", func(t *testing.T) {
		sig := computeGitHubSig(payload, secret)
		assert.False(t, verifier.VerifyGitHubSignature(nil, sig, secret))
	})

	t.Run("empty secret returns false", func(t *testing.T) {
		sig := computeGitHubSig(payload, secret)
		assert.False(t, verifier.VerifyGitHubSignature(payload, sig, nil))
	})

	t.Run("tampered payload returns false", func(t *testing.T) {
		sig := computeGitHubSig(payload, secret)
		tampered := []byte(`{"ref":"refs/heads/main","repository":{"name":"tampered"}}`)
		assert.False(t, verifier.VerifyGitHubSignature(tampered, sig, secret))
	})
}

func TestWebhookHMACVerifier_GitLabToken(t *testing.T) {
	verifier := middleware.NewWebhookVerifier()
	expectedToken := "gitlab-secret-token-token-abc"

	t.Run("valid token returns true", func(t *testing.T) {
		assert.True(t, verifier.VerifyGitLabToken(expectedToken, expectedToken))
	})

	t.Run("invalid token returns false", func(t *testing.T) {
		assert.False(t, verifier.VerifyGitLabToken("wrong-token", expectedToken))
	})

	t.Run("empty token returns false", func(t *testing.T) {
		assert.False(t, verifier.VerifyGitLabToken("", expectedToken))
	})

	t.Run("empty expected token returns false", func(t *testing.T) {
		assert.False(t, verifier.VerifyGitLabToken(expectedToken, ""))
	})
}

func TestGitHubWebhookAuthMiddleware(t *testing.T) {
	secret := []byte("webhook-signing-secret-key-32bytes")
	mw := middleware.GitHubWebhookAuth(secret)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	payload := []byte(`{"action":"opened","pull_request":{"id":42}}`)

	t.Run("valid signature proceeds to next handler with 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(payload))
		req.Header.Set("X-Hub-Signature-256", computeGitHubSig(payload, secret))
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "ok", rr.Body.String())
	})

	t.Run("missing signature header returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(payload))
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("invalid signature header returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(payload))
		req.Header.Set("X-Hub-Signature-256", "sha256=invalid")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}
