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

func TestLogStreamHandler_CheckOrigin_127001(t *testing.T) {
	aggregator := logging.NewLogAggregator(10)
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

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/logs/stream?token=" + token

	dialer := websocket.DefaultDialer
	headers := http.Header{}
	headers.Set("Origin", "http://127.0.0.1:5173")

	conn, resp, err := dialer.Dial(wsURL, headers)
	if err != nil {
		if resp != nil {
			t.Fatalf("failed to dial websocket with 127.0.0.1:5173 origin (status %d): %v", resp.StatusCode, err)
		}
		t.Fatalf("failed to dial websocket with 127.0.0.1:5173 origin: %v", err)
	}
	defer conn.Close()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("expected status 101, got %d", resp.StatusCode)
	}
}

func TestLogStreamHandler_MultiDimensionalQueryParamForwarding(t *testing.T) {
	aggregator := logging.NewLogAggregator(100)

	// Ingest matching historical log
	aggregator.Ingest(logging.LogEntry{
		Timestamp: time.Now().UTC().Add(-2 * time.Second),
		Namespace: "prod",
		Pod:       "postgres-worker1",
		Container: "main",
		Node:      "worker1",
		Service:   "postgres",
		Level:     "WARN",
		Message:   "detected deadlock on table orders",
	})

	// Ingest non-matching logs
	aggregator.Ingest(logging.LogEntry{
		Timestamp: time.Now().UTC().Add(-1 * time.Second),
		Namespace: "prod",
		Pod:       "postgres-worker2",
		Container: "main",
		Node:      "worker2", // wrong node
		Service:   "postgres",
		Level:     "WARN",
		Message:   "detected deadlock on table orders",
	})
	aggregator.Ingest(logging.LogEntry{
		Timestamp: time.Now().UTC().Add(-1 * time.Second),
		Namespace: "prod",
		Pod:       "nats-worker1",
		Container: "main",
		Node:      "worker1",
		Service:   "nats", // wrong service
		Level:     "WARN",
		Message:   "detected deadlock on table orders",
	})
	aggregator.Ingest(logging.LogEntry{
		Timestamp: time.Now().UTC().Add(-1 * time.Second),
		Namespace: "prod",
		Pod:       "postgres-worker1",
		Container: "main",
		Node:      "worker1",
		Service:   "postgres",
		Level:     "INFO", // wrong level
		Message:   "detected deadlock on table orders",
	})
	aggregator.Ingest(logging.LogEntry{
		Timestamp: time.Now().UTC().Add(-1 * time.Second),
		Namespace: "prod",
		Pod:       "postgres-worker1",
		Container: "main",
		Node:      "worker1",
		Service:   "postgres",
		Level:     "WARN",
		Message:   "normal checkpoint completed", // wrong keyword
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

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") +
		"/api/v1/logs/stream?token=" + token +
		"&node=worker1&service=postgres&container=main&level=WARN&keyword=deadlock"

	dialer := websocket.DefaultDialer
	conn, resp, err := dialer.Dial(wsURL, nil)
	if err != nil {
		if resp != nil {
			t.Fatalf("failed to dial websocket (status %d): %v", resp.StatusCode, err)
		}
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer conn.Close()

	// 1. Should receive only the matching historical log
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, histBytes, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read matching historical log: %v", err)
	}

	var histEntry struct {
		Node    string `json:"node"`
		Service string `json:"service"`
		Level   string `json:"level"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(histBytes, &histEntry); err != nil {
		t.Fatalf("failed to unmarshal log entry: %v", err)
	}
	if histEntry.Node != "worker1" || histEntry.Service != "postgres" || histEntry.Level != "WARN" || !strings.Contains(histEntry.Message, "deadlock") {
		t.Fatalf("unexpected historical entry received: %+v", histEntry)
	}

	// 2. Publish live logs: one non-matching (wrong service), then one matching
	go func() {
		time.Sleep(50 * time.Millisecond)
		// Non-matching
		aggregator.Ingest(logging.LogEntry{
			Timestamp: time.Now().UTC(),
			Namespace: "prod",
			Pod:       "nats-worker1",
			Container: "main",
			Node:      "worker1",
			Service:   "nats",
			Level:     "WARN",
			Message:   "deadlock in queue",
		})

		// Matching
		aggregator.Ingest(logging.LogEntry{
			Timestamp: time.Now().UTC(),
			Namespace: "prod",
			Pod:       "postgres-worker1",
			Container: "main",
			Node:      "worker1",
			Service:   "postgres",
			Level:     "WARN",
			Message:   "second deadlock event detected",
		})
	}()

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, liveBytes, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read live matching log: %v", err)
	}

	var liveEntry struct {
		Node    string `json:"node"`
		Service string `json:"service"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(liveBytes, &liveEntry); err != nil {
		t.Fatalf("failed to unmarshal live log: %v", err)
	}
	if liveEntry.Message != "second deadlock event detected" {
		t.Fatalf("expected second deadlock event, got: %s", liveEntry.Message)
	}
}

func TestLogStreamHandler_EmptyHistory_NodeFilter_EmitsHandshake(t *testing.T) {
	aggregator := logging.NewLogAggregator(10)
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

	// Connect targeting node worker2 with 0 historical logs
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/logs/stream?token=" + token + "&node=worker2"

	dialer := websocket.DefaultDialer
	conn, resp, err := dialer.Dial(wsURL, nil)
	if err != nil {
		if resp != nil {
			t.Fatalf("failed to dial websocket (status %d): %v", resp.StatusCode, err)
		}
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msgBytes, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read handshake log: %v", err)
	}

	var entry struct {
		Node    string `json:"node"`
		Service string `json:"service"`
		Level   string `json:"level"`
		Message string `json:"message"`
		Msg     string `json:"msg"`
	}
	if err := json.Unmarshal(msgBytes, &entry); err != nil {
		t.Fatalf("failed to unmarshal handshake message: %v", err)
	}

	expectedMsg := "[INFO] Real-time log stream opened for target (node: worker2, service: ). Waiting for live log events..."
	if entry.Message != expectedMsg && entry.Msg != expectedMsg {
		t.Fatalf("expected handshake msg %q, got msg=%q message=%q", expectedMsg, entry.Msg, entry.Message)
	}
	if entry.Node != "worker2" {
		t.Fatalf("expected node 'worker2', got %q", entry.Node)
	}
	if entry.Level != "INFO" {
		t.Fatalf("expected level 'INFO', got %q", entry.Level)
	}

	// Ensure no subsequent fake logs are emitted (offline node stays quiet)
	_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Fatal("expected no further log events for inactive node, but received data")
	}
}

func TestLogStreamHandler_EmptyHistory_ServiceFilter_EmitsHandshake(t *testing.T) {
	aggregator := logging.NewLogAggregator(10)
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

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/logs/stream?token=" + token + "&service=postgres"

	dialer := websocket.DefaultDialer
	conn, resp, err := dialer.Dial(wsURL, nil)
	if err != nil {
		if resp != nil {
			t.Fatalf("failed to dial websocket (status %d): %v", resp.StatusCode, err)
		}
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msgBytes, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read handshake log: %v", err)
	}

	var entry struct {
		Service string `json:"service"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(msgBytes, &entry); err != nil {
		t.Fatalf("failed to unmarshal handshake message: %v", err)
	}

	expectedMsg := "[INFO] Real-time log stream opened for target (node: , service: postgres). Waiting for live log events..."
	if entry.Message != expectedMsg {
		t.Fatalf("expected handshake msg %q, got message=%q", expectedMsg, entry.Message)
	}
	if entry.Service != "postgres" {
		t.Fatalf("expected service 'postgres', got %q", entry.Service)
	}
}

