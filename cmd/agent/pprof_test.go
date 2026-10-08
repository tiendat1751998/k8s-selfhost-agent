package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"
)

func TestAgent_Pprof_DisabledByDefault(t *testing.T) {
	// Ensure AGENT_ENABLE_PPROF is not set
	t.Setenv("AGENT_ENABLE_PPROF", "")

	collector := NewSystemCollector("", "", nil)
	authToken := "test-secret-token"
	handler := setupHandler(collector, authToken)

	endpoints := []string{
		"/debug/pprof",
		"/debug/pprof/",
		"/debug/pprof/cmdline",
		"/debug/pprof/profile?seconds=1",
		"/debug/pprof/symbol",
		"/debug/pprof/trace?seconds=1",
		"/debug/pprof/goroutine",
		"/debug/pprof/heap",
		"/debug/pprof/allocs",
		"/debug/pprof/block",
		"/debug/pprof/mutex",
		"/debug/pprof/threadcreate",
	}

	for _, ep := range endpoints {
		t.Run("default_off_"+ep, func(t *testing.T) {
			// Even with valid auth token, pprof must return 404 when disabled
			req := httptest.NewRequest(http.MethodGet, ep, nil)
			req.Header.Set("Authorization", "Bearer "+authToken)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("expected 404 Not Found for %s when pprof is disabled, got %d", ep, rec.Code)
			}
		})
	}

	// Also test explicitly set to "false"
	t.Setenv("AGENT_ENABLE_PPROF", "false")
	handlerFalse := setupHandler(collector, authToken)
	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	req.Header.Set("Authorization", "Bearer "+authToken)
	rec := httptest.NewRecorder()
	handlerFalse.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found for /debug/pprof/ when AGENT_ENABLE_PPROF=false, got %d", rec.Code)
	}
}

func TestAgent_Pprof_Enabled_RequiresAuth(t *testing.T) {
	t.Setenv("AGENT_ENABLE_PPROF", "true")

	collector := NewSystemCollector("", "", nil)
	authToken := "test-secret-token"
	handler := setupHandler(collector, authToken)

	endpoints := []string{
		"/debug/pprof",
		"/debug/pprof/",
		"/debug/pprof/cmdline",
		"/debug/pprof/goroutine",
		"/debug/pprof/heap",
	}

	for _, ep := range endpoints {
		t.Run("no_token_"+ep, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, ep, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401 Unauthorized for %s without token, got %d", ep, rec.Code)
			}
		})

		t.Run("wrong_token_"+ep, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, ep, nil)
			req.Header.Set("Authorization", "Bearer wrong-token")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401 Unauthorized for %s with wrong token, got %d", ep, rec.Code)
			}
		})
	}
}

func TestAgent_Pprof_Enabled_Authorized(t *testing.T) {
	t.Setenv("AGENT_ENABLE_PPROF", "true")

	collector := NewSystemCollector("", "", nil)
	authToken := "test-secret-token"
	handler := setupHandler(collector, authToken)

	endpoints := []string{
		"/debug/pprof",
		"/debug/pprof/",
		"/debug/pprof/cmdline",
		"/debug/pprof/goroutine",
		"/debug/pprof/heap",
		"/debug/pprof/allocs",
		"/debug/pprof/threadcreate",
	}

	for _, ep := range endpoints {
		t.Run("authorized_"+ep, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, ep, nil)
			req.Header.Set("Authorization", "Bearer "+authToken)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200 OK for %s with valid token, got %d", ep, rec.Code)
			}
		})
	}
}

func TestAgent_GoroutineLeak_BaselineReturn(t *testing.T) {
	t.Setenv("AGENT_ENABLE_PPROF", "true")

	// Allow existing runtime goroutines to settle and capture baseline
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	// Perform operations that spin up and tear down goroutines
	collector := NewSystemCollector("", "", nil, WithCollectionInterval(20*time.Millisecond))
	authToken := "test-secret-token"
	handler := setupHandler(collector, authToken)

	ctx, cancel := context.WithCancel(context.Background())
	collectorStopped := make(chan struct{})
	go func() {
		collector.Start(ctx)
		close(collectorStopped)
	}()

	// Perform multiple concurrent HTTP requests to authenticated pprof and metrics endpoints
	for i := 0; i < 20; i++ {
		req := httptest.NewRequest(http.MethodGet, "/debug/pprof/goroutine", nil)
		req.Header.Set("Authorization", "Bearer "+authToken)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
	}

	// Trigger stop
	cancel()
	collector.Stop()
	select {
	case <-collectorStopped:
	case <-time.After(2 * time.Second):
		t.Fatal("collector failed to stop within timeout")
	}

	// Baseline polling with deadline
	deadline := time.Now().Add(3 * time.Second)
	var finalCount int
	leakResolved := false

	for time.Now().Before(deadline) {
		runtime.Gosched()
		runtime.GC()
		finalCount = runtime.NumGoroutine()
		if finalCount <= baseline {
			leakResolved = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !leakResolved {
		t.Fatalf("goroutine leak detected: baseline=%d, final=%d (leaked=%d)", baseline, finalCount, finalCount-baseline)
	}
}

func TestAgent_GoroutineLeak_HTTPServer_BaselineReturn(t *testing.T) {
	t.Setenv("AGENT_ENABLE_PPROF", "true")

	// Allow existing runtime goroutines to settle and capture baseline
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	collector := NewSystemCollector("", "", nil)
	authToken := "test-secret-token"
	handler := setupHandler(collector, authToken)

	server := httptest.NewServer(handler)

	client := &http.Client{Timeout: 2 * time.Second}
	for i := 0; i < 20; i++ {
		req, err := http.NewRequest(http.MethodGet, server.URL+"/debug/pprof/goroutine", nil)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+authToken)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
		}
	}

	server.Close()
	client.CloseIdleConnections()

	deadline := time.Now().Add(3 * time.Second)
	var finalCount int
	leakResolved := false

	for time.Now().Before(deadline) {
		runtime.Gosched()
		runtime.GC()
		finalCount = runtime.NumGoroutine()
		if finalCount <= baseline {
			leakResolved = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !leakResolved {
		t.Fatalf("goroutine leak detected after server close: baseline=%d, final=%d (leaked=%d)", baseline, finalCount, finalCount-baseline)
	}
}

