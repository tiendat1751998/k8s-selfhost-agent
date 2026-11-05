package middleware

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

type flusherHijackerRecorder struct {
	*httptest.ResponseRecorder
	flushed  bool
	hijacked bool
}

func (f *flusherHijackerRecorder) Flush() {
	f.flushed = true
}

func (f *flusherHijackerRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	f.hijacked = true
	return nil, nil, nil
}

func TestResponseWriter_UnwrapFlusherHijacker(t *testing.T) {
	rec := &flusherHijackerRecorder{ResponseRecorder: httptest.NewRecorder()}
	rw := newResponseWriter(rec)

	unwrapped, ok := interface{}(rw).(interface{ Unwrap() http.ResponseWriter })
	if !ok {
		t.Fatal("responseWriter must implement Unwrap() http.ResponseWriter")
	}
	if unwrapped.Unwrap() != rec {
		t.Fatalf("expected Unwrap() to return inner recorder")
	}

	rw.Flush()
	if !rec.flushed {
		t.Errorf("expected underlying recorder to be flushed")
	}

	_, _, err := rw.Hijack()
	if err != nil {
		t.Errorf("unexpected error on Hijack: %v", err)
	}
	if !rec.hijacked {
		t.Errorf("expected underlying recorder to be hijacked")
	}
}

func TestStructuredLogger_CanonicalLogLineWithRequestMeta(t *testing.T) {
	called := false
	handler := StructuredLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		meta := tenancy.RequestMetaFromContext(r.Context())
		if meta == nil {
			t.Fatal("expected non-nil RequestMeta in context during StructuredLogger")
		}
		tenancy.SetRequestIdentity(r.Context(), "user-456", "tenant_admin", "tenant-xyz", "jwt")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected handler to be called")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
}

func TestRequestBodyLimitWithSkipPrefixes(t *testing.T) {
	limiter := RequestBodyLimitWithSkipPrefixes(10, "/api/v1/logs", "/skip")
	handler := limiter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))

	// Case 1: Skipped prefix /api/v1/logs allows payload > 10 bytes
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/logs/ingest", bytes.NewReader([]byte("this is a large payload beyond 10 bytes")))
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Errorf("expected 200 OK for skipped prefix, got %d", rec1.Code)
	}

	// Case 2: Non-skipped prefix enforces 10 byte limit
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/other", bytes.NewReader([]byte("this is a large payload beyond 10 bytes")))
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413 Request Entity Too Large for non-skipped prefix, got %d", rec2.Code)
	}
}
