package http_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	"github.com/datdt/k8sselfhost/internal/pkg/health"
)

func TestRouter_AuthRateLimiting(t *testing.T) {
	healthHandler := health.NewHandler(5 * time.Second)
	authHandler, _, _, _ := setupAuthTest(t)
	platform := &adapthttp.PlatformHandlers{
		Auth: authHandler,
	}

	router := adapthttp.NewRouterWithWS(healthHandler, nil, platform)

	endpoints := []struct {
		name string
		path string
		body map[string]string
	}{
		{
			name: "Login Endpoint",
			path: "/api/v1/auth/login",
			body: map[string]string{"email": "ratelimit@example.com", "password": "wrongpassword"},
		},
		{
			name: "MFA Verify Endpoint",
			path: "/api/v1/auth/verify-mfa",
			body: map[string]string{"partial_token": "dummy", "code": "123456"},
		},
		{
			name: "Recovery Verify Endpoint",
			path: "/api/v1/auth/recovery/verify",
			body: map[string]string{"partial_token": "dummy", "recovery_code": "recov1"},
		},
		{
			name: "Refresh Endpoint",
			path: "/api/v1/auth/refresh",
			body: map[string]string{},
		},
		{
			name: "Logout Endpoint",
			path: "/api/v1/auth/logout",
			body: map[string]string{},
		},
	}

	for idx, ep := range endpoints {
		t.Run(ep.name, func(t *testing.T) {
			clientIP := fmt.Sprintf("198.51.100.%d:9999", idx+10)
			bodyBytes, _ := json.Marshal(ep.body)

			// The rate limit configured on auth endpoints is 10 requests per minute.
			// The first 10 requests should reach the handler (returning 401/400 etc, NOT 429).
			for i := 0; i < 10; i++ {
				req := httptest.NewRequest(http.MethodPost, ep.path, bytes.NewReader(bodyBytes))
				req.Header.Set("Content-Type", "application/json")
				req.RemoteAddr = clientIP
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)

				if rec.Code == http.StatusTooManyRequests {
					t.Fatalf("[%s] request %d should not be rate limited yet, got 429", ep.name, i+1)
				}
			}

			// 11th request must be rate limited with 429
			req := httptest.NewRequest(http.MethodPost, ep.path, bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			req.RemoteAddr = clientIP
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusTooManyRequests {
				t.Fatalf("[%s] request 11 must be rate limited with 429, got %d: %s", ep.name, rec.Code, rec.Body.String())
			}

			if rec.Header().Get("Retry-After") == "" {
				t.Fatalf("[%s] expected Retry-After header on 429 response", ep.name)
			}
		})
	}

	t.Run("Shared Rate Limit Across Auth Endpoints", func(t *testing.T) {
		sharedIP := "198.51.100.99:9999"
		loginBody, _ := json.Marshal(map[string]string{"email": "shared@example.com", "password": "wrong"})
		refreshBody, _ := json.Marshal(map[string]string{})

		// 5 requests to /login
		for i := 0; i < 5; i++ {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
			req.Header.Set("Content-Type", "application/json")
			req.RemoteAddr = sharedIP
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code == http.StatusTooManyRequests {
				t.Fatalf("login request %d should not be rate limited yet", i+1)
			}
		}

		// 5 requests to /refresh
		for i := 0; i < 5; i++ {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader(refreshBody))
			req.Header.Set("Content-Type", "application/json")
			req.RemoteAddr = sharedIP
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code == http.StatusTooManyRequests {
				t.Fatalf("refresh request %d should not be rate limited yet", i+1)
			}
		}

		// 11th request to /verify-mfa from same IP must be rate limited
		mfaBody, _ := json.Marshal(map[string]string{"partial_token": "dummy", "code": "123456"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify-mfa", bytes.NewReader(mfaBody))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = sharedIP
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("11th request across auth endpoints must return 429, got %d", rec.Code)
		}
	})

	t.Run("SEC-HIGH-02: Spoofed XFF from untrusted peer does not bypass rate limiter", func(t *testing.T) {
		t.Setenv("TRUSTED_PROXIES", "127.0.0.1/32, ::1/128")
		untrustedPeer := "198.51.100.77:9999"
		loginBody, _ := json.Marshal(map[string]string{"email": "spoof@example.com", "password": "wrong"})

		// Attacker sends 10 requests from untrusted peer 198.51.100.77, rotating X-Forwarded-For
		for i := 0; i < 10; i++ {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Forwarded-For", fmt.Sprintf("10.0.0.%d", i+1))
			req.RemoteAddr = untrustedPeer
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code == http.StatusTooManyRequests {
				t.Fatalf("request %d should not be rate limited yet", i+1)
			}
		}

		// 11th request with a new spoofed X-Forwarded-For header MUST STILL BE RATE LIMITED!
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", "10.0.0.99")
		req.RemoteAddr = untrustedPeer
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("11th request with spoofed XFF MUST return 429, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}
