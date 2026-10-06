package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	_ = SetJWTSecret("unit-test-secret-key-must-be-at-least-32-chars-long!")
	os.Exit(m.Run())
}

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

func TestJWTAuth_QueryTokenDisallowedOnStandardRoutes(t *testing.T) {
	handler := JWTAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	token, _ := GenerateJWT("test-user-id", "viewer", "")

	// Normal endpoints must reject ?token=
	r := httptest.NewRequest(http.MethodGet, "/api/v1/clusters?token="+token, nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 for query token on non-ws route, got %d", w.Code)
	}

	rRoot := httptest.NewRequest(http.MethodGet, "/?token="+token, nil)
	wRoot := httptest.NewRecorder()
	handler.ServeHTTP(wRoot, rRoot)

	if wRoot.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 for query token on root route, got %d", wRoot.Code)
	}
}

func TestJWTAuth_WebSocketQueryTokenAllowed(t *testing.T) {
	handler := JWTAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := UserIDFromContext(r.Context())
		if userID != "test-user-id" {
			t.Errorf("expected userID test-user-id, got %s", userID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	token, _ := GenerateJWT("test-user-id", "viewer", "")

	// /ws with query param token should be allowed
	rWS := httptest.NewRequest(http.MethodGet, "/ws?token="+token, nil)
	wWS := httptest.NewRecorder()
	handler.ServeHTTP(wWS, rWS)

	if wWS.Code != http.StatusOK {
		t.Errorf("expected status 200 for query token on /ws, got %d", wWS.Code)
	}

	// /ws/events with query param token should also be allowed
	rWSSub := httptest.NewRequest(http.MethodGet, "/ws/events?token="+token, nil)
	wWSSub := httptest.NewRecorder()
	handler.ServeHTTP(wWSSub, rWSSub)

	if wWSSub.Code != http.StatusOK {
		t.Errorf("expected status 200 for query token on /ws/events, got %d", wWSSub.Code)
	}

	// /api/v1/logs/stream with query param token should also be allowed
	rLogStream := httptest.NewRequest(http.MethodGet, "/api/v1/logs/stream?token="+token, nil)
	wLogStream := httptest.NewRecorder()
	handler.ServeHTTP(wLogStream, rLogStream)

	if wLogStream.Code != http.StatusOK {
		t.Errorf("expected status 200 for query token on /api/v1/logs/stream, got %d", wLogStream.Code)
	}

	// Upgrade: websocket header with query param token on arbitrary path should be allowed
	rWSHeader := httptest.NewRequest(http.MethodGet, "/api/v1/k8s/exec?token="+token, nil)
	rWSHeader.Header.Set("Upgrade", "websocket")
	wWSHeader := httptest.NewRecorder()
	handler.ServeHTTP(wWSHeader, rWSHeader)

	if wWSHeader.Code != http.StatusOK {
		t.Errorf("expected status 200 for query token with Upgrade: websocket header, got %d", wWSHeader.Code)
	}

	// Connection: upgrade header with query param token on arbitrary path should be allowed
	rConnHeader := httptest.NewRequest(http.MethodGet, "/api/v1/terminal?token="+token, nil)
	rConnHeader.Header.Set("Connection", "Upgrade")
	wConnHeader := httptest.NewRecorder()
	handler.ServeHTTP(wConnHeader, rConnHeader)

	if wConnHeader.Code != http.StatusOK {
		t.Errorf("expected status 200 for query token with Connection: Upgrade header, got %d", wConnHeader.Code)
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
	if err := SetJWTSecret(newSecret); err != nil {
		t.Fatalf("unexpected error setting valid secret: %v", err)
	}

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

func TestMetricsAuthMiddleware(t *testing.T) {
	handler := MetricsAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("No token provided returns 401 when METRICS_TOKEN unset", func(t *testing.T) {
		t.Setenv("METRICS_TOKEN", "")
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("Valid JWT allows request when METRICS_TOKEN unset", func(t *testing.T) {
		t.Setenv("METRICS_TOKEN", "")
		token, err := GenerateAccessToken("user-1", "platform_admin", "default-tenant")
		if err != nil {
			t.Fatalf("failed to generate access token: %v", err)
		}
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 for valid JWT, got %d", rec.Code)
		}
	})

	t.Run("METRICS_TOKEN set rejects request with missing token", func(t *testing.T) {
		t.Setenv("METRICS_TOKEN", "my-secret-metrics-token-12345")
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("METRICS_TOKEN set rejects request with invalid token", func(t *testing.T) {
		t.Setenv("METRICS_TOKEN", "my-secret-metrics-token-12345")
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		req.Header.Set("Authorization", "Bearer wrong-token")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("METRICS_TOKEN set accepts request with valid token", func(t *testing.T) {
		t.Setenv("METRICS_TOKEN", "my-secret-metrics-token-12345")
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		req.Header.Set("Authorization", "Bearer my-secret-metrics-token-12345")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})
}

func TestValidateJWTSecret(t *testing.T) {
	origSecret := jwtSecret
	defer func() {
		jwtSecret = origSecret
	}()

	jwtSecret = nil
	if err := ValidateJWTSecret(); !errors.Is(err, ErrJWTSecretMissing) {
		t.Fatalf("expected ErrJWTSecretMissing when unset, got: %v", err)
	}

	jwtSecret = []byte("too-short")
	if err := ValidateJWTSecret(); !errors.Is(err, ErrJWTSecretMissing) {
		t.Fatalf("expected ErrJWTSecretMissing when < 32 chars, got: %v", err)
	}

	jwtSecret = []byte("a-sufficiently-long-secret-key-at-least-32-bytes")
	if err := ValidateJWTSecret(); err != nil {
		t.Fatalf("expected nil error for valid secret, got: %v", err)
	}
}

func TestSetJWTSecret_ErrorWhenShort(t *testing.T) {
	err := SetJWTSecret("short")
	if !errors.Is(err, ErrJWTSecretMissing) {
		t.Fatalf("expected ErrJWTSecretMissing, got: %v", err)
	}
}

func TestJWTAuth_UnconfiguredSecret(t *testing.T) {
	origSecret := jwtSecret
	defer func() {
		jwtSecret = origSecret
	}()

	jwtSecret = nil
	handler := JWTAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer valid-format-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
	expectedBody := `{"error":"JWT authentication not configured"}`
	if w.Body.String() != expectedBody {
		t.Errorf("expected body %s, got %s", expectedBody, w.Body.String())
	}
}

func TestTokenFunctions_UnconfiguredSecret(t *testing.T) {
	origSecret := jwtSecret
	defer func() {
		jwtSecret = origSecret
	}()

	jwtSecret = nil
	if _, err := GenerateToken("user", "admin", "tenant"); !errors.Is(err, ErrJWTSecretMissing) {
		t.Errorf("expected ErrJWTSecretMissing from GenerateToken, got: %v", err)
	}
	if _, err := ValidateToken("some.fake.token"); !errors.Is(err, ErrJWTSecretMissing) {
		t.Errorf("expected ErrJWTSecretMissing from ValidateToken, got: %v", err)
	}
	if _, err := ValidateTokenClaims("some.fake.token"); !errors.Is(err, ErrJWTSecretMissing) {
		t.Errorf("expected ErrJWTSecretMissing from ValidateTokenClaims, got: %v", err)
	}
}

func TestValidateTokenClaims_And_PartialToken(t *testing.T) {
	token, err := GenerateAccessToken("u1", "admin", "t1")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	claims, err := ValidateTokenClaims(token)
	if err != nil || claims.Sub != "u1" || claims.Role != "admin" || claims.Tenant != "t1" {
		t.Fatalf("invalid claims: %+v, err: %v", claims, err)
	}
	if _, err := ValidateTokenClaims("bad.token"); err == nil {
		t.Error("expected error for bad token")
	}

	partial, err := GeneratePartialToken("u1")
	if err != nil {
		t.Fatalf("failed to generate partial token: %v", err)
	}
	pClaims, err := ValidatePartialToken(partial)
	if err != nil || pClaims.Type != TokenTypePartial {
		t.Fatalf("invalid partial claims: %+v, err: %v", pClaims, err)
	}
	if _, err := ValidatePartialToken(token); err == nil {
		t.Error("expected error validating access token as partial token")
	}

	handler := JWTAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+partial)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for partial token on API, got %d", rec.Code)
	}
}

