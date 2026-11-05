package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJSONRecoverer_NormalRequest(t *testing.T) {
	handler := JSONRecoverer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
}

func TestJSONRecoverer_RecoversPanic_ReturnsJSON500(t *testing.T) {
	handler := JSONRecoverer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("database connection unexpected failure")
	}))

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 Internal Server Error, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected valid JSON body: %v", err)
	}
	if body["error"] != "internal server error" {
		t.Fatalf("expected {\"error\":\"internal server error\"}, got %v", body)
	}
}

func TestJSONRecoverer_ErrAbortHandler_RePanics(t *testing.T) {
	handler := JSONRecoverer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	}))

	req := httptest.NewRequest(http.MethodGet, "/abort", nil)
	rec := httptest.NewRecorder()

	defer func() {
		rvr := recover()
		if rvr == nil {
			t.Fatal("expected http.ErrAbortHandler to be re-panicked, but was swallowed")
		}
		if rvr != http.ErrAbortHandler {
			t.Fatalf("expected re-panic of http.ErrAbortHandler, got %v", rvr)
		}
	}()

	handler.ServeHTTP(rec, req)
}

func TestJSONRecoverer_HeadersAlreadyWritten(t *testing.T) {
	handler := JSONRecoverer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("partial response"))
		panic("panic after write")
	}))

	req := httptest.NewRequest(http.MethodGet, "/partial", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected original status 202 Accepted to remain, got %d", rec.Code)
	}
}
