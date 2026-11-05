package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGitHubRateLimiter_SpoofedStreamingHeadersRejected(t *testing.T) {
	cfg := DefaultGitHubRateLimitConfig()
	cfg.PrimaryLimit = 2
	cfg.MutationLimit = 2
	cfg.PrimaryWindow = time.Minute
	cfg.MutationWindow = time.Minute
	limiter := NewGitHubRateLimiter(cfg)
	defer limiter.Close()

	handler := limiter.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	t.Run("GET /logs/search with spoofed Upgrade: websocket does NOT bypass primary quota", func(t *testing.T) {
		for i := 1; i <= 2; i++ {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/search", nil)
			req.Header.Set("Upgrade", "websocket")
			req.Header.Set("Connection", "Upgrade")
			req = setTestIdentity(req, "attacker-ws", "tenant-test")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("request %d expected 200, got %d", i, rec.Code)
			}
			if rec.Header().Get("X-RateLimit-Limit") != "2" {
				t.Fatalf("expected X-RateLimit-Limit header to be present, got empty (spoofed WS header suppressed it)")
			}
		}

		// 3rd request must be 429 rate limited
		req3 := httptest.NewRequest(http.MethodGet, "/api/v1/logs/search", nil)
		req3.Header.Set("Upgrade", "websocket")
		req3.Header.Set("Connection", "Upgrade")
		req3 = setTestIdentity(req3, "attacker-ws", "tenant-test")
		rec3 := httptest.NewRecorder()
		handler.ServeHTTP(rec3, req3)

		if rec3.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429 for 3rd request with spoofed Upgrade: websocket, got %d (rate limit was bypassed!)", rec3.Code)
		}
	})

	t.Run("GET /logs/search with spoofed Accept: text/event-stream does NOT bypass primary quota", func(t *testing.T) {
		for i := 1; i <= 2; i++ {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/search", nil)
			req.Header.Set("Accept", "text/event-stream")
			req = setTestIdentity(req, "attacker-sse", "tenant-test")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("request %d expected 200, got %d", i, rec.Code)
			}
		}

		// 3rd request must be 429 rate limited
		req3 := httptest.NewRequest(http.MethodGet, "/api/v1/logs/search", nil)
		req3.Header.Set("Accept", "text/event-stream")
		req3 = setTestIdentity(req3, "attacker-sse", "tenant-test")
		rec3 := httptest.NewRecorder()
		handler.ServeHTTP(rec3, req3)

		if rec3.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429 for 3rd request with spoofed Accept: text/event-stream, got %d (rate limit was bypassed!)", rec3.Code)
		}
	})

	t.Run("POST /deployments with spoofed Upgrade: websocket does NOT bypass mutation limit", func(t *testing.T) {
		for i := 1; i <= 2; i++ {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/deployments", nil)
			req.Header.Set("Upgrade", "websocket")
			req.Header.Set("Connection", "Upgrade")
			req = setTestIdentity(req, "attacker-mut", "tenant-test")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("mutation %d expected 200, got %d", i, rec.Code)
			}
		}

		// 3rd request must be 429 rate limited on mutation
		req3 := httptest.NewRequest(http.MethodPost, "/api/v1/deployments", nil)
		req3.Header.Set("Upgrade", "websocket")
		req3.Header.Set("Connection", "Upgrade")
		req3 = setTestIdentity(req3, "attacker-mut", "tenant-test")
		rec3 := httptest.NewRecorder()
		handler.ServeHTTP(rec3, req3)

		if rec3.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429 for 3rd mutation with spoofed Upgrade: websocket, got %d (mutation limit was bypassed!)", rec3.Code)
		}
	})

	t.Run("POST /auth/login with spoofed Upgrade: websocket does NOT bypass mutation limit", func(t *testing.T) {
		for i := 1; i <= 2; i++ {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
			req.Header.Set("Upgrade", "websocket")
			req.Header.Set("Connection", "Upgrade")
			req = setTestIdentity(req, "attacker-login", "tenant-test")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("login %d expected 200, got %d", i, rec.Code)
			}
		}

		// 3rd request must be 429 rate limited
		req3 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req3.Header.Set("Upgrade", "websocket")
		req3.Header.Set("Connection", "Upgrade")
		req3 = setTestIdentity(req3, "attacker-login", "tenant-test")
		rec3 := httptest.NewRecorder()
		handler.ServeHTTP(rec3, req3)

		if rec3.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429 for 3rd login with spoofed Upgrade: websocket, got %d", rec3.Code)
		}
	})

	t.Run("POST /alerts/rules with spoofed Accept: text/event-stream does NOT bypass mutation limit", func(t *testing.T) {
		for i := 1; i <= 2; i++ {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts/rules", nil)
			req.Header.Set("Accept", "text/event-stream")
			req = setTestIdentity(req, "attacker-alert", "tenant-test")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("alert %d expected 200, got %d", i, rec.Code)
			}
		}

		// 3rd request must be 429 rate limited
		req3 := httptest.NewRequest(http.MethodPost, "/api/v1/alerts/rules", nil)
		req3.Header.Set("Accept", "text/event-stream")
		req3 = setTestIdentity(req3, "attacker-alert", "tenant-test")
		rec3 := httptest.NewRecorder()
		handler.ServeHTTP(rec3, req3)

		if rec3.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429 for 3rd alert rule with spoofed Accept: text/event-stream, got %d", rec3.Code)
		}
	})

	t.Run("Genuine GET /logs/stream with Upgrade: websocket DOES bypass primary quota and suppresses headers", func(t *testing.T) {
		for i := 1; i <= 5; i++ {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/stream", nil)
			req.Header.Set("Upgrade", "websocket")
			req.Header.Set("Connection", "Upgrade")
			req = setTestIdentity(req, "genuine-stream-user", "tenant-test")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("stream %d expected 200, got %d", i, rec.Code)
			}
			if h := rec.Header().Get("X-RateLimit-Limit"); h != "" {
				t.Fatalf("expected no X-RateLimit headers on genuine WS upgrade, got %q", h)
			}
		}
	})
}
