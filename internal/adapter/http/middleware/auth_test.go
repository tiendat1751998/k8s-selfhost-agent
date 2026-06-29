package middleware

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJWTAuth_MissingHeader(t *testing.T) {
	handler := JWTAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestJWTAuth_InvalidFormat(t *testing.T) {
	handler := JWTAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "invalid_token_format")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestJWTAuth_EmptyToken(t *testing.T) {
	handler := JWTAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer ")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestJWTAuth_ValidToken(t *testing.T) {
	handler := JWTAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok1 := r.Context().Value(UserIDKey).(string)
		userRole, ok2 := r.Context().Value(UserRoleKey).(string)
		if !ok1 || userID != "test-user-id" {
			t.Errorf("expected userID test-user-id, got %s", userID)
		}
		if !ok2 || userRole != "platform_admin" {
			t.Errorf("expected userRole platform_admin, got %s", userRole)
		}
		w.WriteHeader(http.StatusOK)
	}))

	header := `{"alg":"HS256","typ":"JWT"}`
	payload := `{"sub":"test-user-id","role":"platform_admin"}`
	encHeader := base64.RawURLEncoding.EncodeToString([]byte(header))
	encPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	token := encHeader + "." + encPayload + ".sig"

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestJWTAuth_DemoToken(t *testing.T) {
	handler := JWTAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value(UserIDKey).(string)
		userRole := r.Context().Value(UserRoleKey).(string)
		if userID != "admin-user-id" {
			t.Errorf("expected userID admin-user-id, got %s", userID)
		}
		if userRole != "platform_admin" {
			t.Errorf("expected userRole platform_admin, got %s", userRole)
		}
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer k8s-enterprise-demo-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestRBAC_RequiredRole_HasRole(t *testing.T) {
	handler := JWTAuthMiddleware(RBACMiddleware("platform_admin", "tenant_admin")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	payload := `{"sub":"user","role":"tenant_admin"}`
	token := "hdr." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".sig"

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestRBAC_RequiredRole_MissingRole(t *testing.T) {
	handler := JWTAuthMiddleware(RBACMiddleware("platform_admin")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	payload := `{"sub":"user","role":"viewer"}`
	token := "hdr." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".sig"

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}

func TestJWTAuth_QueryToken(t *testing.T) {
	handler := JWTAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := UserIDFromContext(r.Context())
		if userID != "test-user-id" {
			t.Errorf("expected userID test-user-id, got %s", userID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	payload := `{"sub":"test-user-id","role":"viewer"}`
	token := "hdr." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".sig"

	r := httptest.NewRequest(http.MethodGet, "/?token="+token, nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestJWTAuth_TenantClaim(t *testing.T) {
	handler := JWTAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := TenantIDFromContext(r.Context())
		if tenantID != "my-custom-tenant" {
			t.Errorf("expected tenantID my-custom-tenant, got %s", tenantID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	payload := `{"sub":"test-user-id","role":"viewer","tenant":"my-custom-tenant"}`
	token := "hdr." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".sig"

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}
