package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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

