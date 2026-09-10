package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestRateLimit_UnderLimit(t *testing.T) {
	limiter := RateLimit(5, time.Minute)
	handler := limiter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req.RemoteAddr = "192.168.1.10:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200 OK, got %d", i+1, rec.Code)
		}
	}
}

func TestRateLimit_OverLimit_Returns429WithRetryAfter(t *testing.T) {
	limiter := RateLimit(3, 10*time.Second)
	handler := limiter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	// 3 requests within limit
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req.RemoteAddr = "10.0.0.5:8080"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200 OK, got %d", i+1, rec.Code)
		}
	}

	// 4th request must be rejected with 429
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req.RemoteAddr = "10.0.0.5:8080"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests, got %d: %s", rec.Code, rec.Body.String())
	}

	retryAfterHeader := rec.Header().Get("Retry-After")
	if retryAfterHeader == "" {
		t.Fatal("expected Retry-After header to be set")
	}
	retryAfterVal, err := strconv.Atoi(retryAfterHeader)
	if err != nil || retryAfterVal <= 0 {
		t.Fatalf("expected positive integer in Retry-After header, got %q", retryAfterHeader)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse 429 JSON response: %v", err)
	}
	if resp["error"] != "too many requests" {
		t.Fatalf("expected error message 'too many requests', got %v", resp["error"])
	}
	retryAfterBody, ok := resp["retry_after"].(float64)
	if !ok || int(retryAfterBody) <= 0 {
		t.Fatalf("expected positive retry_after in response body, got %v", resp["retry_after"])
	}
}

func TestRateLimit_MultipleIPs_Independent(t *testing.T) {
	limiter := RateLimit(2, time.Minute)
	handler := limiter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust IP 1
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req.RemoteAddr = "1.1.1.1:1111"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("IP1 req %d failed: %d", i+1, rec.Code)
		}
	}

	// IP 1 3rd request should fail
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req1.RemoteAddr = "1.1.1.1:1111"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusTooManyRequests {
		t.Fatalf("IP1 req 3 expected 429, got %d", rec1.Code)
	}

	// IP 2 should succeed
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req2.RemoteAddr = "2.2.2.2:2222"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("IP2 req 1 expected 200 OK, got %d", rec2.Code)
	}
}

func TestRateLimit_ConcurrentRequests(t *testing.T) {
	limit := 10
	limiter := RateLimit(limit, time.Minute)
	handler := limiter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	numIPs := 5
	requestsPerIP := 30
	var wg sync.WaitGroup

	successCounts := make([]int64, numIPs)
	rateLimitedCounts := make([]int64, numIPs)
	var mu sync.Mutex

	for ipIdx := 0; ipIdx < numIPs; ipIdx++ {
		ip := fmt.Sprintf("192.168.10.%d", ipIdx+1)
		for reqIdx := 0; reqIdx < requestsPerIP; reqIdx++ {
			wg.Add(1)
			go func(idx int, clientIP string) {
				defer wg.Done()
				req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
				req.RemoteAddr = clientIP + ":5555"
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				mu.Lock()
				defer mu.Unlock()
				if rec.Code == http.StatusOK {
					successCounts[idx]++
				} else if rec.Code == http.StatusTooManyRequests {
					rateLimitedCounts[idx]++
				}
			}(ipIdx, ip)
		}
	}

	wg.Wait()

	for i := 0; i < numIPs; i++ {
		if successCounts[i] != int64(limit) {
			t.Errorf("IP %d: expected exactly %d successes, got %d", i+1, limit, successCounts[i])
		}
		expectedRejections := int64(requestsPerIP - limit)
		if rateLimitedCounts[i] != expectedRejections {
			t.Errorf("IP %d: expected %d rate limited rejections, got %d", i+1, expectedRejections, rateLimitedCounts[i])
		}
	}
}

func TestRateLimiter_Janitor_PurgesStaleEntries(t *testing.T) {
	limiter := NewIPRateLimiter(5, 50*time.Millisecond, 20*time.Millisecond)
	defer limiter.Close()

	// Add an entry
	allowed, _ := limiter.Allow("172.16.0.1")
	if !allowed {
		t.Fatal("expected request to be allowed")
	}

	if limiter.ClientCount() != 1 {
		t.Fatalf("expected 1 client entry, got %d", limiter.ClientCount())
	}

	// Wait long enough for entry to become stale and janitor to run
	time.Sleep(150 * time.Millisecond)

	if limiter.ClientCount() != 0 {
		t.Fatalf("expected janitor to purge stale client entry, got count: %d", limiter.ClientCount())
	}
}

func TestRateLimit_WindowReset(t *testing.T) {
	limiter := RateLimit(2, 60*time.Millisecond)
	handler := limiter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ip := "10.10.10.10:4321"

	// 2 requests pass
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req.RemoteAddr = ip
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("req %d expected 200, got %d", i+1, rec.Code)
		}
	}

	// 3rd fails
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req3.RemoteAddr = ip
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusTooManyRequests {
		t.Fatalf("req 3 expected 429, got %d", rec3.Code)
	}

	// Wait for window to expire
	time.Sleep(80 * time.Millisecond)

	// 4th should now succeed
	req4 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req4.RemoteAddr = ip
	rec4 := httptest.NewRecorder()
	handler.ServeHTTP(rec4, req4)
	if rec4.Code != http.StatusOK {
		t.Fatalf("req 4 after window reset expected 200, got %d", rec4.Code)
	}
}
