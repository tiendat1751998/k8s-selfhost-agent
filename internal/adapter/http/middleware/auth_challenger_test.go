package middleware

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestJWTAuth_SpoofedClaims verifies that modifying claims (e.g. role, sub)
// without a valid signature results in rejection (401 Unauthorized).
func TestJWTAuth_SpoofedClaims(t *testing.T) {
	// Generate a valid token first
	validToken, err := GenerateJWT("user-1", "viewer", "tenant-1")
	if err != nil {
		t.Fatalf("failed to generate valid token: %v", err)
	}

	parts := strings.Split(validToken, ".")
	if len(parts) != 3 {
		t.Fatalf("unexpected token format: %s", validToken)
	}

	// 1. Modify the payload to change the role to "platform_admin" but keep the original signature
	spoofedPayload := `{"sub":"user-1","role":"platform_admin","tenant":"tenant-1"}`
	encSpoofedPayload := base64.RawURLEncoding.EncodeToString([]byte(spoofedPayload))
	spoofedToken := parts[0] + "." + encSpoofedPayload + "." + parts[2]

	handler := JWTAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+spoofedToken)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected spoofed payload token to be rejected with 401, got %d", w.Code)
	}

	// 2. Modify signature to make it invalid
	invalidSigToken := parts[0] + "." + parts[1] + "." + "invalid_signature_bytes_here"
	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	r2.Header.Set("Authorization", "Bearer "+invalidSigToken)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, r2)

	if w2.Code != http.StatusUnauthorized {
		t.Errorf("Expected invalid signature token to be rejected with 401, got %d", w2.Code)
	}
}

// TestJWTAuth_NoneAlgorithmChecking verifies whether "none" algorithm tokens are rejected.
func TestJWTAuth_NoneAlgorithm(t *testing.T) {
	// Header specifying alg: none
	header := `{"alg":"none","typ":"JWT"}`
	payload := `{"sub":"attacker","role":"platform_admin","tenant":"default-tenant"}`

	encHeader := base64.RawURLEncoding.EncodeToString([]byte(header))
	encPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))

	// An alg:none token with no signature
	noneToken := encHeader + "." + encPayload + "."

	handler := JWTAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+noneToken)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected alg:none token to be rejected with 401, got %d", w.Code)
	}
}

// TestJWTAuth_BypassVulnerabilityRejected verifies that tokens starting with
// "k8s-enterprise-demo-" are properly rejected now that the backdoor is removed.
func TestJWTAuth_BypassVulnerabilityRejected(t *testing.T) {
	handler := JWTAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be reached for invalid demo token")
		w.WriteHeader(http.StatusOK)
	}))

	// A malicious token with the former backdoor prefix must now be rejected
	maliciousToken := "k8s-enterprise-demo-attacker-session"

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+maliciousToken)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 for demo-prefix token (backdoor removed), got %d", w.Code)
	}
}
