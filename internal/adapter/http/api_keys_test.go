package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

func TestSettingsHandler_APIKeys(t *testing.T) {
	repo := newMockSettingsRepo()
	handler := NewSettingsHandler(repo, zap.NewNop())

	r := chi.NewRouter()
	r.Route("/settings", handler.RegisterRoutes)

	// 1. Initially empty
	req := httptest.NewRequest(http.MethodGet, "/settings/api-keys", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var keys []ApiKeyItem
	if err := json.Unmarshal(rr.Body.Bytes(), &keys); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("expected 0 keys, got %d", len(keys))
	}

	// 2. Create API Key
	payload := `{"name":"ci-token","role":"operator"}`
	req = httptest.NewRequest(http.MethodPost, "/settings/api-keys", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var created ApiKeyItem
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to decode created key: %v", err)
	}
	if created.Name != "ci-token" || created.Role != "operator" || created.ID == "" {
		t.Fatalf("unexpected created key: %+v", created)
	}
	if created.Token == "" || len(created.Token) < 32 {
		t.Fatalf("expected real cryptographic token on creation, got: %q", created.Token)
	}
	if created.Prefix == "" || len(created.Prefix) < 10 {
		t.Fatalf("expected real cryptographic prefix, got: %q", created.Prefix)
	}

	// 3. List contains created key (token must NOT be leaked in list)
	req = httptest.NewRequest(http.MethodGet, "/settings/api-keys", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &keys); err != nil {
		t.Fatalf("failed to decode list: %v", err)
	}
	if len(keys) != 1 || keys[0].ID != created.ID {
		t.Fatalf("expected 1 key with id %s, got %+v", created.ID, keys)
	}
	if keys[0].Token != "" {
		t.Fatalf("expected token to be omitted from list response, got: %q", keys[0].Token)
	}

	// 4. Delete API Key
	req = httptest.NewRequest(http.MethodDelete, "/settings/api-keys/"+created.ID, nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// 5. List is empty again
	req = httptest.NewRequest(http.MethodGet, "/settings/api-keys", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	keys = nil
	if err := json.Unmarshal(rr.Body.Bytes(), &keys); err != nil {
		t.Fatalf("failed to decode list: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("expected 0 keys after delete, got %d", len(keys))
	}
}

func TestSettingsHandler_APIKeys_AliasesAndNilSafety(t *testing.T) {
	t.Run("Nil repo returns empty array", func(t *testing.T) {
		handler := NewSettingsHandler(nil, nil)
		r := chi.NewRouter()
		r.Route("/settings", handler.RegisterRoutes)

		req := httptest.NewRequest(http.MethodGet, "/settings/api-keys", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 with nil repo, got %d", rr.Code)
		}
		var keys []ApiKeyItem
		if err := json.Unmarshal(rr.Body.Bytes(), &keys); err != nil {
			t.Fatalf("failed to decode: %v", err)
		}
		if len(keys) != 0 {
			t.Fatalf("expected 0 keys, got %d", len(keys))
		}
	})

	t.Run("/settings/keys alias routes work", func(t *testing.T) {
		repo := newMockSettingsRepo()
		handler := NewSettingsHandler(repo, zap.NewNop())
		r := chi.NewRouter()
		r.Route("/settings", handler.RegisterRoutes)

		// Create via /settings/keys
		payload := `{"name":"alias-token","role":"admin"}`
		req := httptest.NewRequest(http.MethodPost, "/settings/keys", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("expected 201 on /settings/keys, got %d: %s", rr.Code, rr.Body.String())
		}
		var created ApiKeyItem
		if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
			t.Fatalf("failed to decode: %v", err)
		}
		if created.Token == "" {
			t.Fatal("expected token on creation")
		}

		// List via /settings/keys
		req = httptest.NewRequest(http.MethodGet, "/settings/keys", nil)
		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 on /settings/keys list, got %d", rr.Code)
		}
		var keys []ApiKeyItem
		if err := json.Unmarshal(rr.Body.Bytes(), &keys); err != nil {
			t.Fatalf("failed to decode list: %v", err)
		}
		if len(keys) != 1 {
			t.Fatalf("expected 1 key, got %d", len(keys))
		}

		// Delete via /settings/keys/{id}
		req = httptest.NewRequest(http.MethodDelete, "/settings/keys/"+created.ID, nil)
		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 on delete /settings/keys/{id}, got %d", rr.Code)
		}
	})
}

func TestAPIKey_AuthenticationThroughJWTAuthMiddleware(t *testing.T) {
	_ = middleware.SetJWTSecret("test-secret-at-least-32-bytes-long-super-secret")
	repo := newMockSettingsRepo()
	handler := NewSettingsHandler(repo, zap.NewNop())
	validator := NewSettingsAPIKeyValidator(repo)
	middleware.SetAPIKeyValidator(validator)
	defer middleware.SetAPIKeyValidator(nil)

	// 1. Create an API key via handler
	payload := `{"name":"deploy-key","role":"operator"}`
	createReq := httptest.NewRequest(http.MethodPost, "/settings/api-keys", bytes.NewBufferString(payload))
	createReq.Header.Set("Content-Type", "application/json")
	createRr := httptest.NewRecorder()
	handler.CreateAPIKey(createRr, createReq)
	if createRr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", createRr.Code, createRr.Body.String())
	}
	var created ApiKeyItem
	if err := json.Unmarshal(createRr.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to decode created key: %v", err)
	}
	apiKeyToken := created.Token
	if apiKeyToken == "" {
		t.Fatal("expected non-empty token")
	}

	// Protected test endpoint using JWTAuthMiddleware
	protectedHandler := middleware.JWTAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := tenancy.UserIDFromContext(r.Context())
		role := tenancy.UserRoleFromContext(r.Context())
		tenantID := tenancy.TenantIDFromContext(r.Context())
		resp := map[string]string{
			"user_id":   userID,
			"role":      role,
			"tenant_id": tenantID,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))

	// 2. Success with created k8s_live_ token
	req := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	req.Header.Set("Authorization", "Bearer "+apiKeyToken)
	rr := httptest.NewRecorder()
	protectedHandler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var data map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &data); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if data["user_id"] != "apikey:"+created.ID {
		t.Fatalf("expected user_id apikey:%s, got %s", created.ID, data["user_id"])
	}
	if data["role"] != "operator" {
		t.Fatalf("expected role operator, got %s", data["role"])
	}

	// 3. Reject invalid k8s_live_ token
	req = httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	req.Header.Set("Authorization", "Bearer k8s_live_invalid_unrecognized_token")
	rr = httptest.NewRecorder()
	protectedHandler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid api key, got %d", rr.Code)
	}

	// 4. Reject when APIKeyValidator is nil
	middleware.SetAPIKeyValidator(nil)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	req.Header.Set("Authorization", "Bearer "+apiKeyToken)
	rr = httptest.NewRecorder()
	protectedHandler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when validator is nil, got %d", rr.Code)
	}
	middleware.SetAPIKeyValidator(validator)

	// 5. Delete key -> token becomes rejected
	delReq := httptest.NewRequest(http.MethodDelete, "/settings/api-keys/"+created.ID, nil)
	delRr := httptest.NewRecorder()
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", created.ID)
	delReq = delReq.WithContext(context.WithValue(delReq.Context(), chi.RouteCtxKey, rctx))
	handler.DeleteAPIKey(delRr, delReq)
	if delRr.Code != http.StatusOK {
		t.Fatalf("expected 200 on delete, got %d", delRr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	req.Header.Set("Authorization", "Bearer "+apiKeyToken)
	rr = httptest.NewRecorder()
	protectedHandler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after key deletion, got %d", rr.Code)
	}
}

