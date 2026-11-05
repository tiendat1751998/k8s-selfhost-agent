package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

func setTestIdentity(r *http.Request, userID, tenantID string) *http.Request {
	ctx, meta := tenancy.EnsureRequestMeta(r.Context())
	meta.SetIdentity(userID, "operator", tenantID, "jwt")
	return r.WithContext(ctx)
}

func TestGitHubRateLimiter_PrimaryHeadersAndPerIdentityIsolation(t *testing.T) {
	cfg := DefaultGitHubRateLimitConfig()
	cfg.PrimaryLimit = 5
	cfg.PrimaryWindow = time.Minute
	limiter := NewGitHubRateLimiter(cfg)
	defer limiter.Close()

	handler := limiter.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	// User 1 makes 5 requests
	for i := 1; i <= 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/resources", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		req = setTestIdentity(req, "user-1", "tenant-alpha")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("request %d expected 200, got %d", i, rec.Code)
		}
		if rec.Header().Get("X-RateLimit-Limit") != "5" {
			t.Fatalf("expected limit 5, got %q", rec.Header().Get("X-RateLimit-Limit"))
		}
		if rec.Header().Get("X-RateLimit-Resource") != "core" {
			t.Fatalf("expected resource core, got %q", rec.Header().Get("X-RateLimit-Resource"))
		}
	}

	// 6th request for user 1 is rate limited
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/resources", nil)
	req1.RemoteAddr = "10.0.0.1:1234"
	req1 = setTestIdentity(req1, "user-1", "tenant-alpha")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for user-1, got %d", rec1.Code)
	}

	// User 2 from the SAME IP is NOT rate limited (identity isolation)
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/resources", nil)
	req2.RemoteAddr = "10.0.0.1:1234"
	req2 = setTestIdentity(req2, "user-2", "tenant-alpha")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 for user-2 from same IP, got %d", rec2.Code)
	}
}

func TestGitHubRateLimiter_SecondaryMutationLimit(t *testing.T) {
	cfg := DefaultGitHubRateLimitConfig()
	cfg.MutationLimit = 3
	cfg.MutationWindow = time.Minute
	limiter := NewGitHubRateLimiter(cfg)
	defer limiter.Close()

	handler := limiter.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	// 3 mutating POSTs succeed
	for i := 1; i <= 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/deployments/create", nil)
		req = setTestIdentity(req, "actor-1", "tenant-beta")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("mutation %d expected 200, got %d", i, rec.Code)
		}
	}

	// 4th mutating POST exceeds mutation limit
	req4 := httptest.NewRequest(http.MethodPost, "/api/v1/deployments/create", nil)
	req4 = setTestIdentity(req4, "actor-1", "tenant-beta")
	rec4 := httptest.NewRecorder()
	handler.ServeHTTP(rec4, req4)

	if rec4.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests, got %d", rec4.Code)
	}
	if rec4.Header().Get("X-RateLimit-Resource") != "mutation" {
		t.Errorf("expected X-RateLimit-Resource mutation, got %q", rec4.Header().Get("X-RateLimit-Resource"))
	}
	if rec4.Header().Get("X-RateLimit-Limit") != "3" {
		t.Errorf("expected limit 3, got %q", rec4.Header().Get("X-RateLimit-Limit"))
	}

	var errBody map[string]interface{}
	if err := json.Unmarshal(rec4.Body.Bytes(), &errBody); err != nil {
		t.Fatalf("failed to decode JSON error: %v", err)
	}
	if errBody["error"] != "secondary rate limit exceeded: too many mutating requests" {
		t.Errorf("unexpected error string: %v", errBody["error"])
	}

	// Safe GET request is NOT blocked even when mutation limit breached
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/deployments", nil)
	reqGet = setTestIdentity(reqGet, "actor-1", "tenant-beta")
	recGet := httptest.NewRecorder()
	handler.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET even when mutation breached, got %d", recGet.Code)
	}

	// Read-only POST (/cluster-search) is exempt from mutation limit
	reqSearch := httptest.NewRequest(http.MethodPost, "/api/v1/cluster-search", nil)
	reqSearch = setTestIdentity(reqSearch, "actor-1", "tenant-beta")
	recSearch := httptest.NewRecorder()
	handler.ServeHTTP(recSearch, reqSearch)
	if recSearch.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for read-only POST, got %d", recSearch.Code)
	}
}

func TestGitHubRateLimiter_SecondaryConcurrencyLimit(t *testing.T) {
	cfg := DefaultGitHubRateLimitConfig()
	cfg.MaxConcurrent = 2
	limiter := NewGitHubRateLimiter(cfg)
	defer limiter.Close()

	started := make(chan struct{})
	block := make(chan struct{})

	handler := limiter.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		<-block
		w.WriteHeader(http.StatusOK)
	}))

	var wg sync.WaitGroup
	// Start 2 concurrent requests
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/work", nil)
			req = setTestIdentity(req, "concurrent-user", "tenant-gamma")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}()
		<-started
	}

	// 3rd concurrent request breaches limit
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/work", nil)
	req3 = setTestIdentity(req3, "concurrent-user", "tenant-gamma")
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)

	if rec3.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for concurrent limit breach, got %d", rec3.Code)
	}
	if rec3.Header().Get("Retry-After") != "1" {
		t.Errorf("expected Retry-After 1, got %q", rec3.Header().Get("Retry-After"))
	}

	// Release blocked requests
	close(block)
	wg.Wait()
}

func TestGitHubRateLimiter_CapacityExceeded_FailClosed(t *testing.T) {
	cfg := DefaultGitHubRateLimitConfig()
	cfg.MaxTrackedKeys = 2
	limiter := NewGitHubRateLimiter(cfg)
	defer limiter.Close()

	handler := limiter.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Identity 1 succeeds
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/work", nil)
	req1 = setTestIdentity(req1, "user-1", "tenant-1")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 200 for user-1, got %d", rec1.Code)
	}

	// Identity 2 succeeds
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/work", nil)
	req2 = setTestIdentity(req2, "user-2", "tenant-2")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 for user-2, got %d", rec2.Code)
	}

	// Identity 3 should fail closed with 429 and "rate limit capacity exceeded"
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/work", nil)
	req3 = setTestIdentity(req3, "user-3", "tenant-3")
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for user-3, got %d", rec3.Code)
	}
	var resp map[string]interface{}
	_ = json.Unmarshal(rec3.Body.Bytes(), &resp)
	if resp["error"] != "rate limit capacity exceeded" {
		t.Fatalf("expected 'rate limit capacity exceeded', got %v", resp["error"])
	}
}

func TestGitHubRateLimiter_MutationPaths_NeverExemptNonPostOrParamSuffix(t *testing.T) {
	cfg := DefaultGitHubRateLimitConfig()
	cfg.MutationLimit = 2
	cfg.MutationWindow = time.Minute
	limiter := NewGitHubRateLimiter(cfg)
	defer limiter.Close()

	handler := limiter.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 1. DELETE /api/v1/deployments/test (must be counted as mutation)
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/v1/deployments/test", nil)
	reqDel = setTestIdentity(reqDel, "actor-x", "tenant-x")
	recDel := httptest.NewRecorder()
	handler.ServeHTTP(recDel, reqDel)
	if recDel.Code != http.StatusOK {
		t.Fatalf("expected 200 for 1st mutation (DELETE), got %d", recDel.Code)
	}

	// 2. PUT /api/v1/organizations/test/quota (must be counted as mutation)
	reqPut := httptest.NewRequest(http.MethodPut, "/api/v1/organizations/test/quota", nil)
	reqPut = setTestIdentity(reqPut, "actor-x", "tenant-x")
	recPut := httptest.NewRecorder()
	handler.ServeHTTP(recPut, reqPut)
	if recPut.Code != http.StatusOK {
		t.Fatalf("expected 200 for 2nd mutation (PUT), got %d", recPut.Code)
	}

	// 3. POST /api/v1/namespaces/test/deployments (must be counted as mutation, exceeding limit 2)
	reqPost := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces/test/deployments", nil)
	reqPost = setTestIdentity(reqPost, "actor-x", "tenant-x")
	recPost := httptest.NewRecorder()
	handler.ServeHTTP(recPost, reqPost)
	if recPost.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for 3rd mutation (POST), got %d", recPost.Code)
	}
	if recPost.Header().Get("X-RateLimit-Resource") != "mutation" {
		t.Errorf("expected X-RateLimit-Resource mutation, got %q", recPost.Header().Get("X-RateLimit-Resource"))
	}
}

func TestGitHubRateLimiter_StreamingExemptionAndNoWSHeaders(t *testing.T) {
	cfg := DefaultGitHubRateLimitConfig()
	cfg.PrimaryLimit = 2
	cfg.PrimaryWindow = time.Minute
	limiter := NewGitHubRateLimiter(cfg)
	defer limiter.Close()

	handler := limiter.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	// 1. WebSocket request: must succeed and NOT have any X-RateLimit-* headers
	wsReq := httptest.NewRequest(http.MethodGet, "/api/v1/logs/stream", nil)
	wsReq.Header.Set("Upgrade", "websocket")
	wsReq.Header.Set("Connection", "Upgrade")
	wsReq = setTestIdentity(wsReq, "stream-user", "tenant-stream")
	wsRec := httptest.NewRecorder()
	handler.ServeHTTP(wsRec, wsReq)

	if wsRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for WS request, got %d", wsRec.Code)
	}
	if h := wsRec.Header().Get("X-RateLimit-Limit"); h != "" {
		t.Errorf("expected no X-RateLimit-Limit header on WS upgrade, got %q", h)
	}
	if h := wsRec.Header().Get("X-RateLimit-Resource"); h != "" {
		t.Errorf("expected no X-RateLimit-Resource header on WS upgrade, got %q", h)
	}

	// 2. Perform 5 SSE streaming requests: should not decrement primary core limit
	for i := 0; i < 5; i++ {
		sseReq := httptest.NewRequest(http.MethodGet, "/api/v1/logs/stream?follow=true", nil)
		sseReq.Header.Set("Accept", "text/event-stream")
		sseReq = setTestIdentity(sseReq, "stream-user", "tenant-stream")
		sseRec := httptest.NewRecorder()
		handler.ServeHTTP(sseRec, sseReq)
		if sseRec.Code != http.StatusOK {
			t.Fatalf("expected 200 for SSE request %d, got %d", i, sseRec.Code)
		}
	}

	// 3. Normal request for same user: quota should still have all 2 requests remaining
	normReq := httptest.NewRequest(http.MethodGet, "/api/v1/logs/search", nil)
	normReq = setTestIdentity(normReq, "stream-user", "tenant-stream")
	normRec := httptest.NewRecorder()
	handler.ServeHTTP(normRec, normReq)
	if normRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for normal request, got %d", normRec.Code)
	}
	if rem := normRec.Header().Get("X-RateLimit-Remaining"); rem != "1" {
		t.Errorf("expected 1 remaining after 1 normal request (streaming was exempt), got %q", rem)
	}
}

func TestGitHubRateLimiter_IngestionExemptFromCoreQuotaAndConcurrencyCap(t *testing.T) {
	cfg := DefaultGitHubRateLimitConfig()
	cfg.PrimaryLimit = 2
	cfg.MaxConcurrent = 1
	limiter := NewGitHubRateLimiter(cfg)
	defer limiter.Close()

	started := make(chan struct{})
	blockIngest := make(chan struct{})
	handler := limiter.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/ingest") && r.Header.Get("X-Block") == "true" {
			close(started)
			<-blockIngest
		}
		w.WriteHeader(http.StatusOK)
	}))

	// 1. Launch a blocking ingestion request to occupy concurrency
	go func() {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/logs/ingest", nil)
		req.Header.Set("X-Block", "true")
		req = setTestIdentity(req, "edge-agent", "default-tenant")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}()

	<-started

	// 2. Second concurrent ingestion request to /api/v1/logs/ingest must succeed (exempt from MaxConcurrent: 1)
	reqIngest2 := httptest.NewRequest(http.MethodPost, "/api/v1/logs/ingest", nil)
	reqIngest2 = setTestIdentity(reqIngest2, "edge-agent", "default-tenant")
	recIngest2 := httptest.NewRecorder()
	handler.ServeHTTP(recIngest2, reqIngest2)
	if recIngest2.Code != http.StatusOK {
		t.Fatalf("expected 200 for concurrent ingestion, got %d", recIngest2.Code)
	}

	// 3. Metric ingestion /api/v1/metrics/ingest also succeeds
	reqMetrics := httptest.NewRequest(http.MethodPost, "/api/v1/metrics/ingest", nil)
	reqMetrics = setTestIdentity(reqMetrics, "edge-agent", "default-tenant")
	recMetrics := httptest.NewRecorder()
	handler.ServeHTTP(recMetrics, reqMetrics)
	if recMetrics.Code != http.StatusOK {
		t.Fatalf("expected 200 for metrics ingestion, got %d", recMetrics.Code)
	}

	close(blockIngest)

	// 4. Send 10 ingestion requests to exceed PrimaryLimit (2)
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/logs/ingest", nil)
		req = setTestIdentity(req, "edge-agent", "default-tenant")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("ingestion %d expected 200, got %d", i, rec.Code)
		}
	}

	// 5. Normal UI search request is NOT starved by the 10+ ingestion requests
	searchReq := httptest.NewRequest(http.MethodGet, "/api/v1/logs/search", nil)
	searchReq = setTestIdentity(searchReq, "edge-agent", "default-tenant")
	searchRec := httptest.NewRecorder()
	handler.ServeHTTP(searchRec, searchReq)
	if searchRec.Code != http.StatusOK {
		t.Fatalf("UI search request expected 200 (not starved), got %d", searchRec.Code)
	}
}

