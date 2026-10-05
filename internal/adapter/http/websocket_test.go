package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebSocket_CheckOrigin(t *testing.T) {
	tests := []struct {
		name        string
		origin      string
		requestHost string
		envOrigins  string
		expectAllow bool
	}{
		{
			name:        "Empty origin (same-origin request)",
			origin:      "",
			requestHost: "localhost:8080",
			expectAllow: true,
		},
		{
			name:        "Same host",
			origin:      "http://api.k8sselfhost.local",
			requestHost: "api.k8sselfhost.local",
			expectAllow: true,
		},
		{
			name:        "Unconfigured 127.0.0.1 rejected when not same-host",
			origin:      "http://127.0.0.1:5173",
			requestHost: "localhost:8080",
			expectAllow: false,
		},
		{
			name:        "Unconfigured localhost rejected when not same-host",
			origin:      "http://localhost:5173",
			requestHost: "127.0.0.1:8080",
			expectAllow: false,
		},
		{
			name:        "Configured allowed origin via env",
			origin:      "https://portal.mycompany.com",
			requestHost: "api.mycompany.com",
			envOrigins:  "https://portal.mycompany.com,https://admin.mycompany.com",
			expectAllow: true,
		},
		{
			name:        "Configured localhost dev origin via env",
			origin:      "http://localhost:5173",
			requestHost: "api.mycompany.com",
			envOrigins:  "http://localhost:5173",
			expectAllow: true,
		},
		{
			name:        "Wildcard origin in env does not trust arbitrary origin",
			origin:      "https://anything.io",
			requestHost: "api.anything.io",
			envOrigins:  "*",
			expectAllow: false,
		},
		{
			name:        "Disallowed external origin",
			origin:      "http://malicious.com",
			requestHost: "localhost:8080",
			expectAllow: false,
		},
		{
			name:        "Spoofed localhost subdomain",
			origin:      "http://localhost.evil.com",
			requestHost: "localhost:8080",
			expectAllow: false,
		},
		{
			name:        "Spoofed 127.0.0.1 subdomain",
			origin:      "http://127.0.0.1.evil.com",
			requestHost: "localhost:8080",
			expectAllow: false,
		},
		{
			name:        "Malformed origin URL",
			origin:      "http://[::1:not-valid",
			requestHost: "localhost:8080",
			expectAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envOrigins != "" {
				t.Setenv("CORS_ALLOWED_ORIGINS", tt.envOrigins)
			} else {
				t.Setenv("CORS_ALLOWED_ORIGINS", "")
			}

			req := httptest.NewRequest(http.MethodGet, "/ws", nil)
			req.Host = tt.requestHost
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			got := upgrader.CheckOrigin(req)
			if got != tt.expectAllow {
				t.Errorf("CheckOrigin() = %v, expected %v (origin=%q, host=%q, env=%q)",
					got, tt.expectAllow, tt.origin, tt.requestHost, tt.envOrigins)
			}
		})
	}
}
