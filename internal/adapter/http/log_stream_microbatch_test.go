package http_test

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	mw "github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/infrastructure/logging"
	"github.com/datdt/k8sselfhost/internal/pkg/health"
)

func TestLogStreamHandler_MicroBatchingLive(t *testing.T) {
	aggregator := logging.NewLogAggregator(500)
	streamHandler := adapthttp.NewLogStreamHandler(aggregator)
	platform := &adapthttp.PlatformHandlers{LogStream: streamHandler}
	healthHandler := health.NewHandler(5 * time.Second)
	router := adapthttp.NewRouterWithWS(healthHandler, nil, platform)

	server := httptest.NewServer(router)
	defer server.Close()

	token, err := mw.GenerateJWT("test-user", "platform_admin", "default-tenant")
	require.NoError(t, err)

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/logs/stream?token=" + token + "&namespace=prod&batch=true"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	// Publish 75 log entries rapidly
	for i := 0; i < 75; i++ {
		aggregator.Ingest(logging.LogEntry{
			Timestamp: time.Now().UTC(),
			Namespace: "prod",
			Pod:       "order-service",
			Level:     "INFO",
			Message:   fmt.Sprintf("log event %d", i),
		})
	}

	// Read frames until all 75 entries are received across micro-batches
	totalReceived := 0
	frameCount := 0
	deadline := time.Now().Add(3 * time.Second)
	for totalReceived < 75 && time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		_, msgBytes, rErr := conn.ReadMessage()
		if rErr != nil {
			break
		}
		var batch []map[string]any
		uErr := json.Unmarshal(msgBytes, &batch)
		require.NoError(t, uErr, "every streaming frame must be a JSON array")
		require.LessOrEqual(t, len(batch), 50, "batch frame slice must not exceed 50 entries")
		totalReceived += len(batch)
		frameCount++
	}
	require.Equal(t, 75, totalReceived)
	require.GreaterOrEqual(t, frameCount, 2, "75 entries must be split across multiple micro-batches")
}

func TestLogStreamHandler_ReplayLinesQueryParam(t *testing.T) {
	aggregator := logging.NewLogAggregator(3000)
	// Prepopulate 1500 logs
	for i := 0; i < 1500; i++ {
		aggregator.Ingest(logging.LogEntry{
			Timestamp: time.Now().UTC().Add(time.Duration(i) * time.Millisecond),
			Namespace: "replay-ns",
			Pod:       "analytics",
			Level:     "INFO",
			Message:   fmt.Sprintf("historical log %d", i),
		})
	}

	streamHandler := adapthttp.NewLogStreamHandler(aggregator)
	platform := &adapthttp.PlatformHandlers{LogStream: streamHandler}
	healthHandler := health.NewHandler(5 * time.Second)
	router := adapthttp.NewRouterWithWS(healthHandler, nil, platform)

	server := httptest.NewServer(router)
	defer server.Close()

	token, err := mw.GenerateJWT("test-user", "platform_admin", "default-tenant")
	require.NoError(t, err)

	// Connect with replay_lines=2000
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/logs/stream?token=" + token + "&namespace=replay-ns&replay_lines=2000"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	// Collect all replayed historical entries across batches
	totalReplayed := 0
	for totalReplayed < 1500 {
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, msgBytes, err := conn.ReadMessage()
		require.NoError(t, err)

		var batch []map[string]any
		err = json.Unmarshal(msgBytes, &batch)
		require.NoError(t, err, "replay frame must be a JSON array")
		totalReplayed += len(batch)
	}
	require.Equal(t, 1500, totalReplayed)
}

func TestLogStreamHandler_BackpressureTelemetry(t *testing.T) {
	aggregator := logging.NewLogAggregator(100)
	streamHandler := adapthttp.NewLogStreamHandler(aggregator)
	platform := &adapthttp.PlatformHandlers{LogStream: streamHandler}
	healthHandler := health.NewHandler(5 * time.Second)
	router := adapthttp.NewRouterWithWS(healthHandler, nil, platform)

	server := httptest.NewServer(router)
	defer server.Close()

	token, err := mw.GenerateJWT("test-user", "platform_admin", "default-tenant")
	require.NoError(t, err)

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/logs/stream?token=" + token + "&namespace=prod&batch=true"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	// Flood aggregator to trigger client drops or congestion
	for i := 0; i < 2000; i++ {
		aggregator.Ingest(logging.LogEntry{
			Timestamp: time.Now().UTC(),
			Namespace: "prod",
			Pod:       "congested-service",
			Level:     "WARN",
			Message:   fmt.Sprintf("flood event %d", i),
		})
	}

	// Read messages until we encounter a stream_telemetry frame or timeout
	telemetryFound := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(time.Now().Add(1500 * time.Millisecond))
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var rawMap map[string]any
		if err := json.Unmarshal(msgBytes, &rawMap); err == nil {
			if rawMap["type"] == "stream_telemetry" {
				telemetryFound = true
				require.Contains(t, rawMap, "dropped")
				require.Contains(t, rawMap, "rate")
				break
			}
		}
	}
	require.True(t, telemetryFound, "expected backpressure telemetry frame with type stream_telemetry")
}
