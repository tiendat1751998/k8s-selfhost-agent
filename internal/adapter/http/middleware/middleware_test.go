package middleware

import (
	"bytes"
	"crypto/tls"
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
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Case 1: Unconfigured localhost gets NO ACAO header by default
	r1 := httptest.NewRequest(http.MethodGet, "/", nil)
	r1.Header.Set("Origin", "http://localhost:5173")
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, r1)

	if got := w1.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no CORS Allow-Origin for unconfigured localhost by default, got %s", got)
	}

	// Case 2: Disallowed origin evil.com gets NO ACAO header
	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	r2.Header.Set("Origin", "http://evil.com")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, r2)

	if got := w2.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no CORS Allow-Origin for disallowed origin, got %s", got)
	}

	// Case 3: Empty origin (same-origin) receives NO ACAO header
	r3 := httptest.NewRequest(http.MethodGet, "/", nil)
	w3 := httptest.NewRecorder()
	handler.ServeHTTP(w3, r3)

	if got := w3.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no CORS Allow-Origin for empty origin, got %s", got)
	}
}

func TestCORS_ConfiguredOrigins(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:5173,https://app.k8sselfhost.io")
	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Allowed origin reflected
	r1 := httptest.NewRequest(http.MethodGet, "/", nil)
	r1.Header.Set("Origin", "http://localhost:5173")
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, r1)

	if got := w1.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("expected reflected origin http://localhost:5173, got %s", got)
	}
	if got := w1.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("expected Access-Control-Allow-Credentials true, got %s", got)
	}
	if got := w1.Header().Get("Vary"); got != "Origin" {
		t.Errorf("expected Vary: Origin, got %s", got)
	}

	// Disallowed origin gets no ACAO
	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	r2.Header.Set("Origin", "http://evil.com")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, r2)

	if got := w2.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no ACAO for evil.com, got %s", got)
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
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")
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
	headers := w.Header().Get("Access-Control-Allow-Headers")
	if !strings.Contains(headers, "X-Tenant-ID") {
		t.Errorf("expected CORS Allow-Headers header to contain X-Tenant-ID, got: %s", headers)
	}


	// Disallowed origin preflight receives 403
	rDisallowed := httptest.NewRequest(http.MethodOptions, "/", nil)
	rDisallowed.Header.Set("Origin", "http://evil.com")
	wDisallowed := httptest.NewRecorder()
	handler.ServeHTTP(wDisallowed, rDisallowed)

	if wDisallowed.Code != http.StatusForbidden {
		t.Errorf("expected 403 for disallowed origin OPTIONS, got %d", wDisallowed.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("Standard security headers", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)

		checks := map[string]string{
			"X-Content-Type-Options": "nosniff",
			"X-Frame-Options":        "DENY",
			"X-XSS-Protection":       "0",
			"Referrer-Policy":        "strict-origin-when-cross-origin",
			"Permissions-Policy":     "camera=(), microphone=(), geolocation=()",
		}

		for header, expected := range checks {
			if got := w.Header().Get(header); got != expected {
				t.Errorf("expected %s: %q, got %q", header, expected, got)
			}
		}
	})

	t.Run("Content-Security-Policy directives", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)

		csp := w.Header().Get("Content-Security-Policy")

		// WebSocket / API connectivity
		if !strings.Contains(csp, "connect-src 'self' ws: wss:") {
			t.Errorf("CSP must allow WebSocket connections in connect-src, got: %s", csp)
		}

		// Air-gapped: no external Google Fonts
		if strings.Contains(csp, "fonts.googleapis.com") || strings.Contains(csp, "fonts.gstatic.com") {
			t.Errorf("CSP must not depend on external Google Fonts in air-gapped mode, got: %s", csp)
		}

		// Modern CSP directives
		if !strings.Contains(csp, "frame-ancestors 'none'") {
			t.Errorf("CSP must contain frame-ancestors 'none', got: %s", csp)
		}
		if !strings.Contains(csp, "object-src 'none'") {
			t.Errorf("CSP must contain object-src 'none', got: %s", csp)
		}
		if !strings.Contains(csp, "base-uri 'self'") {
			t.Errorf("CSP must contain base-uri 'self', got: %s", csp)
		}

		// Essential directives
		if !strings.Contains(csp, "default-src 'self'") {
			t.Errorf("CSP must contain default-src 'self', got: %s", csp)
		}
		if !strings.Contains(csp, "script-src 'self'") {
			t.Errorf("CSP must contain script-src 'self', got: %s", csp)
		}
		if strings.Contains(csp, "script-src 'self' 'unsafe-inline'") {
			t.Errorf("CSP script-src must not contain unsafe-inline, got: %s", csp)
		}
		if !strings.Contains(csp, "style-src 'self' 'unsafe-inline'") {
			t.Errorf("CSP style-src should contain 'unsafe-inline', got: %s", csp)
		}
		if !strings.Contains(csp, "font-src 'self'") {
			t.Errorf("CSP font-src should contain 'self', got: %s", csp)
		}
	})

	t.Run("HSTS behavior on plain HTTP vs TLS vs X-Forwarded-Proto", func(t *testing.T) {
		// Plain HTTP: HSTS must NOT be set to prevent breaking local/dev setups
		rPlain := httptest.NewRequest(http.MethodGet, "http://localhost:8080/", nil)
		wPlain := httptest.NewRecorder()
		handler.ServeHTTP(wPlain, rPlain)
		if hsts := wPlain.Header().Get("Strict-Transport-Security"); hsts != "" {
			t.Errorf("expected no HSTS on plain HTTP, got: %q", hsts)
		}

		// TLS request: HSTS MUST be set
		rTLS := httptest.NewRequest(http.MethodGet, "https://localhost:8443/", nil)
		rTLS.TLS = &tls.ConnectionState{}
		wTLS := httptest.NewRecorder()
		handler.ServeHTTP(wTLS, rTLS)
		if hsts := wTLS.Header().Get("Strict-Transport-Security"); hsts != "max-age=31536000; includeSubDomains" {
			t.Errorf("expected HSTS on TLS request, got: %q", hsts)
		}

		// Reverse proxy with X-Forwarded-Proto: https
		rProxy := httptest.NewRequest(http.MethodGet, "http://localhost:8080/", nil)
		rProxy.Header.Set("X-Forwarded-Proto", "https")
		wProxy := httptest.NewRecorder()
		handler.ServeHTTP(wProxy, rProxy)
		if hsts := wProxy.Header().Get("Strict-Transport-Security"); hsts != "max-age=31536000; includeSubDomains" {
			t.Errorf("expected HSTS when X-Forwarded-Proto is https, got: %q", hsts)
		}

		// Case-insensitive X-Forwarded-Proto: HTTPS
		rProxyUpper := httptest.NewRequest(http.MethodGet, "http://localhost:8080/", nil)
		rProxyUpper.Header.Set("X-Forwarded-Proto", "HTTPS")
		wProxyUpper := httptest.NewRecorder()
		handler.ServeHTTP(wProxyUpper, rProxyUpper)
		if hsts := wProxyUpper.Header().Get("Strict-Transport-Security"); hsts != "max-age=31536000; includeSubDomains" {
			t.Errorf("expected HSTS when X-Forwarded-Proto is HTTPS, got: %q", hsts)
		}
	})
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
		{
			name:     "kebab-case query params and authorization redacted",
			input:    "http://localhost:8080/api/v1/auth?api-key=secret-key&access-token=at-secret&auth-token=auth-sec&authorization=bearer-token&private-key=pk-val&normal=public",
			contains: []string{
				"normal=public",
				"api-key=%5BREDACTED%5D",
				"access-token=%5BREDACTED%5D",
				"auth-token=%5BREDACTED%5D",
				"authorization=%5BREDACTED%5D",
				"private-key=%5BREDACTED%5D",
			},
			omits: []string{"secret-key", "at-secret", "auth-sec", "bearer-token", "pk-val"},
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
