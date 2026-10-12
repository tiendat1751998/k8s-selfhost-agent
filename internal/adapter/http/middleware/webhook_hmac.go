package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
)

// WebhookHMACVerifier defines the interface for verifying Git hosting provider webhooks.
type WebhookHMACVerifier interface {
	VerifyGitHubSignature(payload []byte, signatureHeader string, secretKey []byte) bool
	VerifyGitLabToken(tokenHeader string, expectedToken string) bool
}

type defaultWebhookVerifier struct{}

// NewWebhookVerifier creates a default WebhookHMACVerifier instance.
func NewWebhookVerifier() WebhookHMACVerifier {
	return &defaultWebhookVerifier{}
}

// VerifyGitHubSignature verifies that the given payload matches GitHub's HMAC-SHA256 signature.
func (v *defaultWebhookVerifier) VerifyGitHubSignature(payload []byte, signatureHeader string, secretKey []byte) bool {
	if len(payload) == 0 || len(secretKey) == 0 || signatureHeader == "" {
		return false
	}

	const prefix = "sha256="
	if !strings.HasPrefix(signatureHeader, prefix) {
		return false
	}

	sigHex := strings.TrimPrefix(signatureHeader, prefix)
	expectedSig, err := hex.DecodeString(sigHex)
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, secretKey)
	mac.Write(payload)
	actualSig := mac.Sum(nil)

	return subtle.ConstantTimeCompare(actualSig, expectedSig) == 1
}

// VerifyGitLabToken verifies that the GitLab webhook token matches the expected secret token.
func (v *defaultWebhookVerifier) VerifyGitLabToken(tokenHeader string, expectedToken string) bool {
	if tokenHeader == "" || expectedToken == "" {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(tokenHeader), []byte(expectedToken)) == 1
}

// GitHubWebhookAuth returns an HTTP middleware verifying the GitHub X-Hub-Signature-256 header.
func GitHubWebhookAuth(secretKey []byte, verifiers ...WebhookHMACVerifier) func(http.Handler) http.Handler {
	var verifier WebhookHMACVerifier
	if len(verifiers) > 0 && verifiers[0] != nil {
		verifier = verifiers[0]
	} else {
		verifier = NewWebhookVerifier()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sigHeader := r.Header.Get("X-Hub-Signature-256")
			if sigHeader == "" {
				http.Error(w, "Invalid X-Hub-Signature-256 HMAC signature.", http.StatusUnauthorized)
				return
			}

			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Failed to read request body", http.StatusBadRequest)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

			if !verifier.VerifyGitHubSignature(bodyBytes, sigHeader, secretKey) {
				http.Error(w, "Invalid X-Hub-Signature-256 HMAC signature.", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
