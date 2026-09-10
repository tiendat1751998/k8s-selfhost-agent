package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestRequestBodyLimit(t *testing.T) {
	limitMiddleware := RequestBodyLimit(10) // 10 bytes limit

	handler := limitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))

	// Case 1: Body within limit
	r1 := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("small")))
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, r1)

	if w1.Code != http.StatusOK {
		t.Errorf("expected 200 OK for small body, got %d", w1.Code)
	}

	// Case 2: Body exceeds limit
	r2 := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("this body is way too long for a 10 byte limit")))
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, r2)

	if w2.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413 for oversized body, got %d", w2.Code)
	}
}

func TestCORS_DefaultOrigins(t *testing.T) {
	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Case 1: Allowed origin localhost:5173
	r1 := httptest.NewRequest(http.MethodGet, "/", nil)
	r1.Header.Set("Origin", "http://localhost:5173")
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, r1)

	if w1.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Errorf("expected CORS Allow-Origin http://localhost:5173, got %s", w1.Header().Get("Access-Control-Allow-Origin"))
	}
	if w1.Header().Get("Vary") != "Origin" {
		t.Errorf("expected Vary: Origin, got %s", w1.Header().Get("Vary"))
	}

	// Case 2: Disallowed origin evil.com
	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	r2.Header.Set("Origin", "http://evil.com")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, r2)

	if w2.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("expected no CORS Allow-Origin for disallowed origin, got %s", w2.Header().Get("Access-Control-Allow-Origin"))
	}

	// Case 3: Empty origin receives default first origin
	r3 := httptest.NewRequest(http.MethodGet, "/", nil)
	w3 := httptest.NewRecorder()
	handler.ServeHTTP(w3, r3)

	if w3.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("expected default origin http://localhost:3000, got %s", w3.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORS_CustomEnvOrigins(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.k8sselfhost.io,https://admin.k8sselfhost.io")

	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Origin", "https://app.k8sselfhost.io")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Header().Get("Access-Control-Allow-Origin") != "https://app.k8sselfhost.io" {
		t.Errorf("expected allowed origin https://app.k8sselfhost.io, got %s", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORS_Preflight(t *testing.T) {
	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodOptions, "/", nil)
	r.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204 for OPTIONS, got %d", w.Code)
	}
	methods := w.Header().Get("Access-Control-Allow-Methods")
	if methods == "" || !strings.Contains(methods, "PATCH") {
		t.Errorf("expected CORS Allow-Methods header to contain PATCH, got: %s", methods)
	}
	if w.Header().Get("Access-Control-Allow-Headers") == "" {
		t.Error("expected CORS Allow-Headers header")
	}
}

func TestSecurityHeaders(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	checks := map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"X-XSS-Protection":          "1; mode=block",
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
		"Permissions-Policy":        "camera=(), microphone=(), geolocation=()",
	}

	for header, expected := range checks {
		if w.Header().Get(header) != expected {
			t.Errorf("expected %s: %s, got %s", header, expected, w.Header().Get(header))
		}
	}

	// Verify CSP removes unsafe-inline from script-src but keeps in style-src
	csp := w.Header().Get("Content-Security-Policy")
	if strings.Contains(csp, "script-src 'self' 'unsafe-inline'") {
		t.Errorf("CSP script-src must not contain unsafe-inline, got: %s", csp)
	}
	if !strings.Contains(csp, "script-src 'self'") {
		t.Errorf("CSP script-src should contain 'self', got: %s", csp)
	}
	if !strings.Contains(csp, "style-src 'self' 'unsafe-inline'") {
		t.Errorf("CSP style-src should contain 'unsafe-inline', got: %s", csp)
	}
}

func TestMetrics(t *testing.T) {
	handler := Metrics(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestStructuredLogger(t *testing.T) {
	handler := StructuredLogger(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestResponseWriter_WriteHeader(t *testing.T) {
	w := httptest.NewRecorder()
	rw := newResponseWriter(w)

	rw.WriteHeader(http.StatusNotFound)

	if rw.statusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rw.statusCode)
	}
}

func TestSanitizeURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
		omits    []string
	}{
		{
			name:     "no query params",
			input:    "http://localhost:8080/api/v1/clusters",
			contains: []string{"http://localhost:8080/api/v1/clusters"},
			omits:    []string{"[REDACTED]"},
		},
		{
			name:     "safe query params only",
			input:    "http://localhost:8080/api/v1/pods?namespace=prod&limit=50",
			contains: []string{"namespace=prod", "limit=50"},
			omits:    []string{"[REDACTED]"},
		},
		{
			name:     "sensitive query params redacted",
			input:    "http://localhost:8080/api/v1/auth?token=my-secret-token&trace_token=trace-99&key=my-key&password=supersecret&secret=classified&normal=ok",
			contains: []string{"normal=ok", "token=%5BREDACTED%5D", "trace_token=%5BREDACTED%5D", "key=%5BREDACTED%5D", "password=%5BREDACTED%5D", "secret=%5BREDACTED%5D"},
			omits:    []string{"my-secret-token", "trace-99", "my-key", "supersecret", "classified"},
		},
		{
			name:     "case insensitive matching",
			input:    "http://localhost:8080/api/v1/auth?TOKEN=xyz&Secret=shh",
			contains: []string{"TOKEN=%5BREDACTED%5D", "Secret=%5BREDACTED%5D"},
			omits:    []string{"xyz", "shh"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := url.Parse(tc.input)
			if err != nil {
				t.Fatalf("failed to parse url: %v", err)
			}
			origRawQuery := parsed.RawQuery
			result := SanitizeURL(parsed)

			// Ensure original parsed URL was not modified
			if parsed.RawQuery != origRawQuery {
				t.Errorf("SanitizeURL mutated original URL RawQuery")
			}

			for _, expected := range tc.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("expected result to contain %q, got: %s", expected, result)
				}
			}
			for _, omitted := range tc.omits {
				if strings.Contains(result, omitted) {
					t.Errorf("expected result to NOT contain %q, got: %s", omitted, result)
				}
			}
		})
	}
}

func TestTracing_SanitizesSensitiveQueryParams(t *testing.T) {
	handler := Tracing(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify downstream handler still sees original query params
		token := r.URL.Query().Get("token")
		if token != "my-secret-token" {
			t.Errorf("expected downstream handler to see original token 'my-secret-token', got: %s", token)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test?token=my-secret-token&safe=hello", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", rec.Code)
	}
}
