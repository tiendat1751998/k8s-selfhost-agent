package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
)

func TestTrustedProxyRealIP_UntrustedPeerIgnoresSpoofedXFF(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1/32, ::1/128")
	mw := middleware.TrustedProxyRealIP()

	var recordedRemoteAddr string
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recordedRemoteAddr = r.RemoteAddr
		w.WriteHeader(http.StatusOK)
	}))

	// Untrusted peer 198.51.100.2 attempts to spoof X-Forwarded-For as 10.0.0.1
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "198.51.100.2:45678"
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 1.2.3.4")
	req.Header.Set("X-Real-IP", "10.0.0.1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Since peer is untrusted, RemoteAddr MUST remain untrusted peer IP, NOT spoofed XFF
	expectedHost := "198.51.100.2:45678"
	if recordedRemoteAddr != expectedHost {
		t.Fatalf("expected remote addr %s, got %s (spoofed header was trusted!)", expectedHost, recordedRemoteAddr)
	}
}

func TestTrustedProxyRealIP_TrustedPeerUsesXFF(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1/32, 10.0.0.0/8")
	mw := middleware.TrustedProxyRealIP()

	var recordedRemoteAddr string
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recordedRemoteAddr = r.RemoteAddr
		w.WriteHeader(http.StatusOK)
	}))

	// Trusted proxy 127.0.0.1 forwards real client 203.0.113.195
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	req.Header.Set("X-Forwarded-For", "203.0.113.195")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	expectedHost := "203.0.113.195:54321"
	if recordedRemoteAddr != expectedHost {
		t.Fatalf("expected remote addr %s, got %s", expectedHost, recordedRemoteAddr)
	}
}

func TestTrustedProxyRealIP_DefaultLoopbackWhenUnset(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "")
	mw := middleware.TrustedProxyRealIP()

	var recordedRemoteAddr string
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recordedRemoteAddr = r.RemoteAddr
		w.WriteHeader(http.StatusOK)
	}))

	// Untrusted peer
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.50:12345"
	req.Header.Set("X-Forwarded-For", "8.8.8.8")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if recordedRemoteAddr != "192.168.1.50:12345" {
		t.Fatalf("expected default loopback to reject 192.168.1.50, got %s", recordedRemoteAddr)
	}

	// Loopback peer
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "127.0.0.1:12345"
	req2.Header.Set("X-Real-IP", "8.8.8.8")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if recordedRemoteAddr != "8.8.8.8:12345" {
		t.Fatalf("expected loopback to be trusted by default, got %s", recordedRemoteAddr)
	}
}
