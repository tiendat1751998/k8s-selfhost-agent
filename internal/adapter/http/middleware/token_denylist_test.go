package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
)

func TestTokenDenylist_RevocationAndExpiration(t *testing.T) {
	dl := middleware.NewTokenDenylist(50 * time.Millisecond)
	defer dl.Close()

	rawToken := "test-sample-bearer-token-12345"

	// Initially not revoked
	if dl.IsRevoked(rawToken) {
		t.Fatal("token should not be revoked initially")
	}

	// Revoke with 100ms TTL
	dl.Revoke(rawToken, time.Now().Add(100*time.Millisecond))

	// Immediately should be revoked
	if !dl.IsRevoked(rawToken) {
		t.Fatal("token should be revoked after Revoke()")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// After expiration, should no longer be revoked
	if dl.IsRevoked(rawToken) {
		t.Fatal("token should not be revoked after its expiration")
	}
}

func TestTokenDenylist_JWTAuthMiddlewareRejectsRevokedToken(t *testing.T) {
	_ = middleware.SetJWTSecret("supersecret-must-be-at-least-32-characters-long")
	token, err := middleware.GenerateAccessToken("user-1", "operator", "tenant-1")
	if err != nil {
		t.Fatalf("failed to generate access token: %v", err)
	}

	handler := middleware.JWTAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 1. Valid request before revocation returns 200
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	req1.Header.Set("Authorization", "Bearer "+token)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 200 before revocation, got %d", rec1.Code)
	}

	// 2. Revoke token
	middleware.RevokeToken(token, time.Now().Add(time.Minute))

	// 3. Request after revocation returns 401 Unauthorized
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized after revocation, got %d: %s", rec2.Code, rec2.Body.String())
	}
}
