package middleware

import (
	"net/http"
	"strings"
)

// isVerifiedStreamingEndpoint strictly validates that a request is a genuine
// streaming endpoint (WebSocket, SSE, or follow log stream) and not a standard
// REST endpoint with spoofed headers.
func isVerifiedStreamingEndpoint(r *http.Request) bool {
	if r == nil || r.Method != http.MethodGet {
		return false
	}

	path := r.URL.Path
	isStreamRoute := path == "/api/v1/logs/stream" || path == "/logs/stream" ||
		path == "/api/v1/ws" || path == "/ws" || strings.HasPrefix(path, "/ws/") ||
		path == "/stream" || strings.HasSuffix(path, "/stream") ||
		path == "/api/v1/events" || path == "/events" || strings.HasSuffix(path, "/events") ||
		path == "/api/v1/exec" || path == "/exec" || strings.HasSuffix(path, "/exec") ||
		path == "/api/v1/portforward" || path == "/portforward" || strings.HasSuffix(path, "/portforward") ||
		((strings.Contains(path, "/logs") || strings.Contains(path, "/pods/")) &&
			(r.URL.Query().Get("follow") == "true" || r.URL.Query().Get("follow") == "1"))

	if !isStreamRoute {
		return false
	}

	isWS := isWebSocketUpgrade(r)
	isSSE := strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/event-stream")
	isFollow := r.URL.Query().Get("follow") == "true" || r.URL.Query().Get("follow") == "1"

	if isWS || isSSE || isFollow {
		return true
	}

	// Dedicated streaming endpoints (such as /logs/stream, /stream, /ws) invoked with GET
	return path == "/api/v1/logs/stream" || path == "/logs/stream" ||
		path == "/api/v1/ws" || path == "/ws" || strings.HasPrefix(path, "/ws/") ||
		path == "/stream" || strings.HasSuffix(path, "/stream")
}

// IsStreamingRequest determines if an incoming HTTP request is a streaming connection
// (WebSocket, SSE / EventSource, log tail, or interactive exec).
func IsStreamingRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	if strings.ToLower(r.Header.Get("Upgrade")) == "websocket" ||
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") ||
		strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/event-stream") {
		return true
	}
	path := r.URL.Path
	if path == "/ws" || strings.HasPrefix(path, "/ws/") ||
		path == "/stream" || strings.HasSuffix(path, "/stream") || strings.Contains(path, "/stream/") || strings.Contains(path, "/stream") ||
		strings.Contains(path, "/exec") {
		return true
	}
	if (strings.Contains(path, "/logs") || strings.Contains(path, "/pods/")) &&
		(r.URL.Query().Get("follow") == "true" || r.URL.Query().Get("follow") == "1") {
		return true
	}
	return false
}
