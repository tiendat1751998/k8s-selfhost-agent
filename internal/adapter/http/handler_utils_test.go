package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"key": "value"})

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected content-type application/json, got %s", w.Header().Get("Content-Type"))
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusNotFound, "not found", nil)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestParseIntParam(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?limit=25&offset=10", nil)

	limit := parseIntParam(r, "limit", 50)
	if limit != 25 {
		t.Errorf("expected limit 25, got %d", limit)
	}

	offset := parseIntParam(r, "offset", 0)
	if offset != 10 {
		t.Errorf("expected offset 10, got %d", offset)
	}
}

func TestParseIntParam_Default(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	limit := parseIntParam(r, "limit", 50)
	if limit != 50 {
		t.Errorf("expected default limit 50, got %d", limit)
	}
}

func TestParseIntParam_Invalid(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?limit=abc", nil)

	limit := parseIntParam(r, "limit", 50)
	if limit != 50 {
		t.Errorf("expected default limit 50 for invalid input, got %d", limit)
	}
}

func TestAuthMiddleware_NoToken(t *testing.T) {
	handler := AuthMiddleware("")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 when no token configured, got %d", w.Code)
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	handler := AuthMiddleware("secret-token")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer secret-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with valid token, got %d", w.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	handler := AuthMiddleware("secret-token")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 with invalid token, got %d", w.Code)
	}
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	handler := AuthMiddleware("secret-token")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 with missing header, got %d", w.Code)
	}
}

func TestParseUUIDParam(t *testing.T) {
	t.Run("valid UUID", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/incidents/123e4567-e89b-12d3-a456-426614174000", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "123e4567-e89b-12d3-a456-426614174000")
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

		id, err := parseUUIDParam(r, "id")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != "123e4567-e89b-12d3-a456-426614174000" {
			t.Errorf("expected UUID, got %s", id)
		}
	})

	t.Run("valid custom string ID", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/services/service-auth-prod", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "service-auth-prod")
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

		id, err := parseUUIDParam(r, "id")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != "service-auth-prod" {
			t.Errorf("expected string ID, got %s", id)
		}
	})

	t.Run("missing parameter", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/incidents/", nil)
		rctx := chi.NewRouteContext()
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

		_, err := parseUUIDParam(r, "id")
		if err == nil {
			t.Fatalf("expected error for missing param, got nil")
		}
	})

	t.Run("parameter too long", func(t *testing.T) {
		longID := strings.Repeat("a", 129)
		r := httptest.NewRequest(http.MethodGet, "/incidents/"+longID, nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", longID)
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

		_, err := parseUUIDParam(r, "id")
		if err == nil {
			t.Fatalf("expected error for overly long param (>128 chars), got nil")
		}
	})
}
