package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/pkg/logengine"
)

func TestLogServer_HandleEngineStatus(t *testing.T) {
	tempDir := t.TempDir()
	engineSrc, err := NewEngineLogSource(tempDir)
	if err != nil {
		t.Fatalf("failed to create engine source: %v", err)
	}
	defer engineSrc.Close()

	_ = engineSrc.Writer().Write(logengine.Entry{
		Timestamp: time.Now().UTC(),
		Service:   "test-svc",
		Level:     "info",
		Message:   "Engine status test log",
	})
	_ = engineSrc.Writer().Flush()

	logServer := NewLogServer(WithLogSource(engineSrc))
	collector := NewSystemCollector("", "", nil)
	const testToken = "test-log-token"
	rawStatusHandler := setupHandler(collector, testToken, logServer)
	statusHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("Authorization", "Bearer "+testToken)
		rawStatusHandler.ServeHTTP(w, r)
	})

	// Test GET /logs/status
	req := httptest.NewRequest(http.MethodGet, "/logs/status", nil)
	rec := httptest.NewRecorder()
	statusHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /logs/status, got %d: %s", rec.Code, rec.Body.String())
	}

	var statusResp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&statusResp); err != nil {
		t.Fatalf("failed to decode /logs/status response: %v", err)
	}

	if statusResp["engine"] != "k8s-agent embedded logengine" {
		t.Errorf("expected engine 'k8s-agent embedded logengine', got '%v'", statusResp["engine"])
	}
	if statusResp["status"] != "healthy" {
		t.Errorf("expected status 'healthy', got '%v'", statusResp["status"])
	}
	if blocks, ok := statusResp["blocks"].(float64); !ok || blocks < 1 {
		t.Errorf("expected blocks >= 1, got %v", statusResp["blocks"])
	}
}

type recordingTailLogSource struct {
	lastTail int
}

func (r *recordingTailLogSource) ListServices(ctx context.Context) ([]string, error) {
	return []string{"app"}, nil
}

func (r *recordingTailLogSource) GetLogs(ctx context.Context, app string, tail int, sinceTime, untilTime *time.Time, query string, level string) ([]LogEntry, error) {
	r.lastTail = tail
	return []LogEntry{{Message: "msg"}}, nil
}

func TestLogServer_HandleGetLogs_TailParsing(t *testing.T) {
	src := &recordingTailLogSource{}
	srv := NewLogServer(WithLogSource(src))

	tests := []struct {
		url          string
		expectedTail int
	}{
		{url: "/logs?app=app", expectedTail: 100},
		{url: "/logs?app=app&tail=", expectedTail: 100},
		{url: "/logs?app=app&tail=all", expectedTail: -1},
		{url: "/logs?app=app&tail=-1", expectedTail: -1},
		{url: "/logs?app=app&tail=0", expectedTail: 100},
		{url: "/logs?app=app&tail=42", expectedTail: 42},
		{url: "/logs?app=app&tail=invalid", expectedTail: 100},
		{url: "/logs?app=app&tail=-5", expectedTail: 100},
	}

	for _, tc := range tests {
		t.Run(tc.url, func(t *testing.T) {
			src.lastTail = 9999
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			rec := httptest.NewRecorder()
			srv.HandleGetLogs(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", rec.Code)
			}
			if src.lastTail != tc.expectedTail {
				t.Errorf("for %s, expected tail %d, got %d", tc.url, tc.expectedTail, src.lastTail)
			}
		})
	}
}

func TestLogServer_HandleTailLogs_StreamsFromEngine(t *testing.T) {
	tempDir := t.TempDir()
	engineSrc, err := NewEngineLogSource(tempDir)
	if err != nil {
		t.Fatalf("failed to create engine source: %v", err)
	}
	defer engineSrc.Close()

	_ = engineSrc.Writer().Write(logengine.Entry{
		Timestamp: time.Now().UTC(),
		Service:   "user@1000.service",
		Level:     "info",
		Message:   "Engine stream tail test",
	})
	_ = engineSrc.Writer().Flush()

	logServer := NewLogServer(WithLogSource(engineSrc))

	t.Run("streams SSE for specific service from engine", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		req := httptest.NewRequest(http.MethodGet, "/logs/tail?service=user@1000.service&tail=10", nil).WithContext(ctx)
		rec := httptest.NewRecorder()

		// Cancel after short delay to break polling loop
		time.AfterFunc(100*time.Millisecond, cancel)
		logServer.HandleTailLogs(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}
		if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
			t.Errorf("expected Content-Type text/event-stream, got %q", ct)
		}
		body := rec.Body.String()
		if !contains(body, "user@1000.service") || !contains(body, "Engine stream tail test") {
			t.Errorf("expected body to contain service and message, got: %s", body)
		}
	})

	t.Run("streams SSE for empty service from engine", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		req := httptest.NewRequest(http.MethodGet, "/logs/tail?tail=10", nil).WithContext(ctx)
		rec := httptest.NewRecorder()

		time.AfterFunc(100*time.Millisecond, cancel)
		logServer.HandleTailLogs(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		if !contains(body, "Engine stream tail test") {
			t.Errorf("expected body to contain message, got: %s", body)
		}
	})
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func TestLogServer_HandleLogsErrors(t *testing.T) {
	tempDir := t.TempDir()
	engineSrc, err := NewEngineLogSource(tempDir)
	if err != nil {
		t.Fatalf("failed to create engine source: %v", err)
	}
	defer engineSrc.Close()

	_ = engineSrc.Writer().Write(logengine.Entry{
		Timestamp: time.Now().UTC(),
		Service:   "payment-svc",
		Level:     "error",
		Message:   "connection timeout after 150ms to 10.0.0.1:5432",
	})
	_ = engineSrc.Writer().Write(logengine.Entry{
		Timestamp: time.Now().UTC(),
		Service:   "payment-svc",
		Level:     "error",
		Message:   "connection timeout after 300ms to 10.0.0.2:5432",
	})
	_ = engineSrc.Writer().Flush()

	logServer := NewLogServer(WithLogSource(engineSrc))
	collector := NewSystemCollector("", "", nil)
	const testToken = "test-log-token"
	handler := setupHandler(collector, testToken, logServer)

	req := httptest.NewRequest(http.MethodGet, "/logs/errors", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /logs/errors, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Groups []logengine.ErrorGroup `json:"groups"`
		Count  int                    `json:"count"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Count != 1 || len(resp.Groups) != 1 {
		t.Fatalf("expected 1 error group, got count=%d, len=%d", resp.Count, len(resp.Groups))
	}
	if resp.Groups[0].Count != 2 {
		t.Errorf("expected group count=2, got %d", resp.Groups[0].Count)
	}
	if resp.Groups[0].Service != "payment-svc" {
		t.Errorf("expected service 'payment-svc', got %q", resp.Groups[0].Service)
	}
}
