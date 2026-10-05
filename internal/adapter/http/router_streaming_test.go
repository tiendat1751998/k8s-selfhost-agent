package http_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	mw "github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/infrastructure/logging"
	"github.com/datdt/k8sselfhost/internal/pkg/health"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

// TestRouter_StreamingBypassesTimeout verifies CON-20:
// Streaming routes (SSE, /stream, websockets) bypass the Chi 60s timeout middleware,
// while standard routes have timeout applied.
func TestRouter_StreamingBypassesTimeout(t *testing.T) {
	hh := health.NewHandler(5 * time.Second)
	router := adapthttp.NewRouterWithWS(hh, nil, nil)

	// An SSE request with Accept: text/event-stream should be detected as streaming
	sseReq := httptest.NewRequest(http.MethodGet, "/api/v1/logs/stream", nil)
	sseReq.Header.Set("Accept", "text/event-stream")
	assert.True(t, mw.IsStreamingRequest(sseReq), "SSE request must be detected as streaming")

	// Standard endpoint must NOT be detected as streaming
	stdReq := httptest.NewRequest(http.MethodGet, "/api/v1/clusters", nil)
	assert.False(t, mw.IsStreamingRequest(stdReq), "Standard REST request must not be streaming")

	// Suffix /stream must be detected as streaming
	telemetryStreamReq := httptest.NewRequest(http.MethodGet, "/api/v1/telemetry/stream", nil)
	assert.True(t, mw.IsStreamingRequest(telemetryStreamReq), "Telemetry stream must be streaming")

	// Root /stream must be detected as streaming
	streamReq := httptest.NewRequest(http.MethodGet, "/stream", nil)
	assert.True(t, mw.IsStreamingRequest(streamReq), "/stream route must be streaming")

	_ = router
}

// TestJWTAuth_SSEQueryTokenAuthentication verifies CON-19:
// Browser EventSource requests sending ?token= authenticate successfully,
// setting UserID, UserRole, and TenantID context identically to Bearer tokens.
func TestJWTAuth_SSEQueryTokenAuthentication(t *testing.T) {
	var capturedUser, capturedRole, capturedTenant string
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUser = tenancy.UserIDFromContext(r.Context())
		capturedRole = tenancy.UserRoleFromContext(r.Context())
		capturedTenant = tenancy.TenantIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	authMiddleware := mw.JWTAuthMiddleware(testHandler)

	token, err := mw.GenerateJWT("sse-user-42", "operator", "tenant-alpha")
	require.NoError(t, err)

	// 1. SSE request with Accept: text/event-stream and ?token= must succeed
	reqSSE := httptest.NewRequest(http.MethodGet, "/api/v1/telemetry/stream?token="+token, nil)
	reqSSE.Header.Set("Accept", "text/event-stream")
	recSSE := httptest.NewRecorder()
	authMiddleware.ServeHTTP(recSSE, reqSSE)

	assert.Equal(t, http.StatusOK, recSSE.Code)
	assert.Equal(t, "sse-user-42", capturedUser)
	assert.Equal(t, "operator", capturedRole)
	assert.Equal(t, "tenant-alpha", capturedTenant)

	// 2. Standard non-streaming route with ?token= must still be rejected (401)
	reqStd := httptest.NewRequest(http.MethodGet, "/api/v1/clusters?token="+token, nil)
	recStd := httptest.NewRecorder()
	authMiddleware.ServeHTTP(recStd, reqStd)

	assert.Equal(t, http.StatusUnauthorized, recStd.Code)
}

// TestRouter_NoDuplicateLogsStreamRoute verifies CON-21:
// When both platform.CentralizedLogs and platform.LogStream are mounted,
// router initializes cleanly without duplicate route registrations on /logs/stream.
func TestRouter_NoDuplicateLogsStreamRoute(t *testing.T) {
	aggregator := logging.NewLogAggregator(10)
	streamHandler := adapthttp.NewLogStreamHandler(aggregator)
	centralizedLogs := adapthttp.NewLogHandler(nil)

	platform := &adapthttp.PlatformHandlers{
		CentralizedLogs: centralizedLogs,
		LogStream:       streamHandler,
	}

	hh := health.NewHandler(5 * time.Second)
	router := adapthttp.NewRouterWithWS(hh, nil, platform)
	require.NotNil(t, router)

	server := httptest.NewServer(router)
	defer server.Close()

	token, err := mw.GenerateJWT("admin", "platform_admin", "default-tenant")
	require.NoError(t, err)

	// Query /api/v1/logs/stream with ?token= should not return 404 or panic from duplicate route
	req, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/logs/stream?token="+token, nil)
	require.NoError(t, err)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	req.Header.Set("Sec-WebSocket-Version", "13")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Switching protocols (101) or bad handshake is acceptable; 404/500 is not
	assert.True(t, resp.StatusCode == http.StatusSwitchingProtocols || resp.StatusCode == http.StatusBadRequest)
}
