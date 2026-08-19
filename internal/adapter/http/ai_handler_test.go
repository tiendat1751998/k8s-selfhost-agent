package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/infrastructure/llm"
)

func TestAIHandler_AddProvider_SSRFBlocked(t *testing.T) {
	registry := llm.NewProviderRegistry()
	handler := NewAIHandler(registry)

	router := chi.NewRouter()
	router.Route("/ai", handler.RegisterRoutes)

	body := map[string]interface{}{
		"name":     "local-ollama",
		"type":     "ollama",
		"endpoint": "http://127.0.0.1:11434",
		"model":    "llama3",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/ai/providers", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 Bad Request due to SSRF validation, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAIHandler_AddProvider_PublicAllowed(t *testing.T) {
	registry := llm.NewProviderRegistry()
	handler := NewAIHandler(registry)

	router := chi.NewRouter()
	router.Route("/ai", handler.RegisterRoutes)

	body := map[string]interface{}{
		"name":     "openai-prod",
		"type":     "openai",
		"endpoint": "https://8.8.8.8/v1",
		"model":    "gpt-4o",
		"api_key":  "sk-test",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/ai/providers", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201 Created for public IP, got %d: %s", rec.Code, rec.Body.String())
	}
}
