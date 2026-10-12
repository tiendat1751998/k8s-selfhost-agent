package middleware

import (
	"strings"
)

// extractTokenFromSecWebSocketProtocol extracts JWT token from Sec-WebSocket-Protocol header.
// Supported formats:
// 1. "bearer, <token>" or "<token>, bearer" (standard browser WebSocket subprotocol array)
// 2. "bearer <token>" or "Bearer <token>"
func extractTokenFromSecWebSocketProtocol(header string) string {
	if header == "" {
		return ""
	}
	if strings.Contains(header, ",") {
		parts := strings.Split(header, ",")
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if strings.HasPrefix(strings.ToLower(trimmed), "bearer ") {
				return strings.TrimSpace(trimmed[7:])
			}
			if !strings.EqualFold(trimmed, "bearer") && trimmed != "" {
				return trimmed
			}
		}
	}
	if strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return strings.TrimSpace(header[7:])
	}
	return ""
}
