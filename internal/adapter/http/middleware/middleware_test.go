package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
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
	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("expected CORS Allow-Methods header")
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
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
		"Permissions-Policy":        "camera=(), microphone=(), geolocation=()",
	}

	for header, expected := range checks {
		if w.Header().Get(header) != expected {
			t.Errorf("expected %s: %s, got %s", header, expected, w.Header().Get(header))
		}
	}

	// Verify X-XSS-Protection is removed
	if xss := w.Header().Get("X-XSS-Protection"); xss != "" {
		t.Errorf("expected X-XSS-Protection to be absent, got %s", xss)
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
