package middleware

import (
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

	token, _ := GenerateJWT("test-user-id", "platform_admin", "")

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
		t.Error("handler should not be called for invalid demo token")
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer test-token-for-unit-tests")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestRBAC_RequiredRole_HasRole(t *testing.T) {
	handler := JWTAuthMiddleware(RBACMiddleware("platform_admin", "tenant_admin")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	token, _ := GenerateJWT("user", "tenant_admin", "")

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

	token, _ := GenerateJWT("user", "viewer", "")

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

	token, _ := GenerateJWT("test-user-id", "viewer", "")

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

	token, _ := GenerateJWT("test-user-id", "viewer", "my-custom-tenant")

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestRequireRolesForMutations_ReadOnlyAllowed(t *testing.T) {
	handler := JWTAuthMiddleware(RequireRolesForMutations("platform_admin", "operator")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	token, _ := GenerateJWT("user", "viewer", "")

	// GET is read-only -> viewer should be allowed
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for GET with viewer role, got %d", w.Code)
	}
}

func TestRequireRolesForMutations_MutationDeniedForViewer(t *testing.T) {
	handler := JWTAuthMiddleware(RequireRolesForMutations("platform_admin", "tenant_admin", "operator")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	token, _ := GenerateJWT("user", "viewer", "")

	methods := []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}
	for _, method := range methods {
		r := httptest.NewRequest(method, "/", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected status 403 for %s with viewer role, got %d", method, w.Code)
		}
	}
}

func TestRequireRolesForMutations_MutationAllowedForOperator(t *testing.T) {
	handler := JWTAuthMiddleware(RequireRolesForMutations("platform_admin", "tenant_admin", "operator")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	token, _ := GenerateJWT("user", "operator", "")

	methods := []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}
	for _, method := range methods {
		r := httptest.NewRequest(method, "/", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200 for %s with operator role, got %d", method, w.Code)
		}
	}
}

func TestSetJWTSecret_ValidAndSign(t *testing.T) {
	newSecret := "a-very-long-custom-jwt-secret-key-32chars!"
	SetJWTSecret(newSecret)

	token, err := GenerateJWT("user-custom", "platform_admin", "tenant-custom")
	if err != nil {
		t.Fatalf("unexpected error generating token with new secret: %v", err)
	}

	payloadStr, err := ValidateJWT(token)
	if err != nil {
		t.Fatalf("unexpected error validating token: %v", err)
	}
	if payloadStr == "" {
		t.Error("expected non-empty payload")
	}
}
