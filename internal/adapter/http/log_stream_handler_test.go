package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	mw "github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/infrastructure/logging"
	"github.com/datdt/k8sselfhost/internal/pkg/health"
)

func TestLogStreamHandler_NilAggregator(t *testing.T) {
	handler := NewLogStreamHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/stream", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rec.Code)
	}
}

func TestLogStreamHandler_WebSocketIntegration(t *testing.T) {
	aggregator := logging.NewLogAggregator(100)

	// Pre-populate with historical logs
	aggregator.Ingest(logging.LogEntry{
		Timestamp: time.Now().UTC(),
		Namespace: "prod",
		Pod:       "api-gateway",
		Container: "main",
		Level:     "INFO",
		Message:   "Gateway initialized",
	})
	aggregator.Ingest(logging.LogEntry{
		Timestamp: time.Now().UTC(),
		Namespace: "staging",
		Pod:       "auth-service",
		Container: "main",
		Level:     "DEBUG",
		Message:   "Auth debug event",
	})

	logStreamHandler := NewLogStreamHandler(aggregator)
	platform := &PlatformHandlers{
		LogStream: logStreamHandler,
	}

	healthHandler := health.NewHandler(5 * time.Second)
	router := NewRouterWithWS(healthHandler, nil, platform)

	server := httptest.NewServer(router)
	defer server.Close()

	token, err := mw.GenerateJWT("test-user", "platform_admin", "default-tenant")
	if err != nil {
		t.Fatalf("failed to generate JWT token: %v", err)
	}

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/logs/stream?token=" + token + "&namespace=prod"

	dialer := websocket.DefaultDialer
	conn, resp, err := dialer.Dial(wsURL, nil)
	if err != nil {
		if resp != nil {
			t.Fatalf("failed to dial websocket (status %d): %v", resp.StatusCode, err)
		}
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer conn.Close()

	// 1. First message should be historical log for "prod" namespace
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msgBytes, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read historical log: %v", err)
	}

	var histEntry struct {
		Namespace string `json:"namespace"`
		Pod       string `json:"pod"`
		Message   string `json:"message"`
		Time      string `json:"time"`
		Msg       string `json:"msg"`
	}
	if err := json.Unmarshal(msgBytes, &histEntry); err != nil {
		t.Fatalf("failed to unmarshal log entry: %v", err)
	}

	if histEntry.Namespace != "prod" || histEntry.Pod != "api-gateway" {
		t.Fatalf("unexpected historical log: %+v", histEntry)
	}
	if histEntry.Msg != "Gateway initialized" {
		t.Fatalf("expected Msg 'Gateway initialized', got '%s'", histEntry.Msg)
	}

	// 2. Publish a live log matching filter
	go func() {
		time.Sleep(50 * time.Millisecond)
		aggregator.Ingest(logging.LogEntry{
			Timestamp: time.Now().UTC(),
			Namespace: "prod",
			Pod:       "payment-service",
			Container: "main",
			Level:     "INFO",
			Message:   "Transaction processed successfully",
		})
	}()

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, liveBytes, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read live log: %v", err)
	}

	var liveEntry struct {
		Namespace string `json:"namespace"`
		Pod       string `json:"pod"`
		Message   string `json:"message"`
	}
	if err := json.Unmarshal(liveBytes, &liveEntry); err != nil {
		t.Fatalf("failed to unmarshal live log: %v", err)
	}

	if liveEntry.Namespace != "prod" || liveEntry.Pod != "payment-service" || liveEntry.Message != "Transaction processed successfully" {
		t.Fatalf("unexpected live log received: %+v", liveEntry)
	}
}
