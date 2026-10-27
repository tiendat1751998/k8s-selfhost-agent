package metrics

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestCollectHTTPTPS_ConfigSyntaxErrorsDoNotInflateHTTPErrorRate(t *testing.T) {
	// Mock Traefik overview endpoint reporting router and service config syntax errors
	traefikSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/overview":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"http": map[string]interface{}{
					"routers": map[string]interface{}{
						"total":    10,
						"warnings": 2,
						"errors":   5, // 5 syntax configuration errors in router rules
					},
					"services": map[string]interface{}{
						"total":    8,
						"warnings": 1,
						"errors":   3, // 3 syntax configuration errors in service definitions
					},
					"middlewares": map[string]interface{}{
						"total": 4,
					},
				},
			})
		case "/api/http/services":
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"status": "enabled",
					"serverStatus": map[string]string{
						"http://10.0.0.1:8080": "UP",
						"http://10.0.0.2:8080": "UP",
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer traefikSrv.Close()

	totalRequests := int64(100)
	collector := NewTPSCollector(
		nil,
		nil,
		zap.NewNop(),
		WithTraefikURL(traefikSrv.URL),
		WithTPSRequestCountFn(func() int64 { return totalRequests }),
	)

	// First collection to establish baseline
	t0 := time.Now().Add(-1 * time.Second)
	_ = collector.collectHTTPTPS(context.Background(), t0)

	// Second collection after 1s with 50 new requests processed
	totalRequests = 150
	t1 := t0.Add(1 * time.Second)
	tps := collector.collectHTTPTPS(context.Background(), t1)

	// Assertions:
	// 1. RequestsPerSec should reflect 50 req / 1s = 50 RPS
	if tps.RequestsPerSec != 50.0 {
		t.Errorf("expected RequestsPerSec = 50.0, got %f", tps.RequestsPerSec)
	}

	// 2. Traefik config syntax errors (5 router errors + 3 service errors = 8 config errors)
	// MUST NOT be treated as HTTP 5xx errors or divided by total requests.
	// ErrorRate must strictly remain 0.
	if tps.ErrorRate != 0 {
		t.Errorf("expected ErrorRate = 0 (config syntax errors must not be treated as runtime 5xx HTTP errors), got %f", tps.ErrorRate)
	}

	// 3. Active connections should reflect upstream UP servers (2)
	if tps.ActiveConnections != 2 {
		t.Errorf("expected ActiveConnections = 2, got %d", tps.ActiveConnections)
	}
}

func TestDeriveTraefikURL_EdgeCases(t *testing.T) {
	os.Unsetenv("TRAEFIK_API_URL")

	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"unix:///var/run/docker.sock", ""},
		{"tcp://0.0.0.0:2375", "http://localhost:8080"},
		{"tcp://192.168.1.50:2375", "http://192.168.1.50:8080"},
		{"http://custom-traefik:9000", "http://custom-traefik:9000"},
		{"https://secure-traefik:8443", "https://secure-traefik:8443"},
	}

	for _, tc := range tests {
		got := DeriveTraefikURL(tc.input)
		if got != tc.expected {
			t.Errorf("DeriveTraefikURL(%q) = %q, expected %q", tc.input, got, tc.expected)
		}
	}

	// Test TRAEFIK_API_URL env var override
	os.Setenv("TRAEFIK_API_URL", "http://traefik.prod:8000")
	defer os.Unsetenv("TRAEFIK_API_URL")
	if got := DeriveTraefikURL(""); got != "http://traefik.prod:8000" {
		t.Errorf("expected http://traefik.prod:8000 from TRAEFIK_API_URL, got %q", got)
	}
}

func TestIsSelfServerURL(t *testing.T) {
	tests := []struct {
		url        string
		serverPort int
		expected   bool
	}{
		{"http://localhost:8080", 8080, true},
		{"http://127.0.0.1:8080", 8080, true},
		{"http://0.0.0.0:8080", 8080, true},
		{"http://[::1]:8080", 8080, true},
		{"localhost:8080", 8080, true},
		{"127.0.0.1:8080", 8080, true},
		{"http://localhost:8081", 8080, false},
		{"http://127.0.0.1:8081", 8080, false},
		{"http://traefik.prod:8080", 8080, false},
		{"http://192.168.1.50:8080", 8080, false},
		{"", 8080, false},
		{"http://localhost", 80, true},
		{"http://localhost:8080", 0, true}, // defaults to 8080 when serverPort <= 0
	}

	for _, tc := range tests {
		got := IsSelfServerURL(tc.url, tc.serverPort)
		if got != tc.expected {
			t.Errorf("IsSelfServerURL(%q, %d) = %v, expected %v", tc.url, tc.serverPort, got, tc.expected)
		}
	}
}
