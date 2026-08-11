// Package http provides the AI provider management HTTP handler.
package http

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/infrastructure/llm"
)

// AIHandler provides HTTP endpoints for managing AI/LLM providers.
type AIHandler struct {
	registry *llm.ProviderRegistry
}

// NewAIHandler creates a new AI handler with the given provider registry.
func NewAIHandler(registry *llm.ProviderRegistry) *AIHandler {
	return &AIHandler{registry: registry}
}

// RegisterRoutes registers AI provider API routes on the given chi router.
func (h *AIHandler) RegisterRoutes(r chi.Router) {
	r.Get("/providers", h.ListProviders)
	r.Get("/providers/{name}", h.GetProvider)
	r.Post("/providers", h.AddProvider)
	r.Delete("/providers/{name}", h.DeleteProvider)
	r.Post("/providers/{name}/health", h.HealthCheckProvider)
	r.Post("/test", h.TestPrompt)
}

// providerResponse is the API response for a provider.
type providerResponse struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Model    string `json:"model"`
	Endpoint string `json:"endpoint"`
	Status   string `json:"status"`
	Latency  string `json:"latency,omitempty"`
	Default  bool   `json:"default"`
}

// addProviderRequest is the request body for adding a new provider.
type addProviderRequest struct {
	Name     string `json:"name"`
	Type     string `json:"type"` // ollama | openai | vllm
	Endpoint string `json:"endpoint"`
	Model    string `json:"model"`
	APIKey   string `json:"api_key,omitempty"`
	Default  bool   `json:"default"`
}

func (r *addProviderRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Name) == "" {
		ve.Add("name", "name is required")
	}
	if strings.TrimSpace(r.Endpoint) == "" {
		ve.Add("endpoint", "endpoint is required")
	}
	if strings.TrimSpace(r.Model) == "" {
		ve.Add("model", "model is required")
	}
	switch r.Type {
	case "ollama", "openai", "vllm":
	default:
		ve.Add("type", "type must be one of: ollama, openai, vllm")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// testPromptRequest is the request body for testing a prompt.
type testPromptRequest struct {
	Provider string `json:"provider"`
	Prompt   string `json:"prompt"`
	System   string `json:"system,omitempty"`
}

func (r *testPromptRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Prompt) == "" {
		ve.Add("prompt", "prompt is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// testPromptResponse is the response for a test prompt.
type testPromptResponse struct {
	Content        string `json:"content"`
	Model          string `json:"model"`
	PromptTokens   int    `json:"prompt_tokens"`
	ResponseTokens int    `json:"response_tokens"`
	DurationMs     int64  `json:"duration_ms"`
}

// ListProviders returns all registered AI providers.
func (h *AIHandler) ListProviders(w http.ResponseWriter, r *http.Request) {
	providers := h.registry.List()
	result := make([]providerResponse, len(providers))
	for i, p := range providers {
		result[i] = providerResponse{
			Name:     p.Name,
			Type:     p.Type,
			Model:    p.Model,
			Endpoint: p.Endpoint,
			Status:   p.Status,
			Latency:  p.Latency,
			Default:  p.Default,
		}
	}
	writeJSON(w, http.StatusOK, result)
}

// GetProvider returns details for a single provider.
func (h *AIHandler) GetProvider(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	providers := h.registry.List()
	for _, p := range providers {
		if p.Name == name {
			writeJSON(w, http.StatusOK, providerResponse{
				Name:     p.Name,
				Type:     p.Type,
				Model:    p.Model,
				Endpoint: p.Endpoint,
				Status:   p.Status,
				Latency:  p.Latency,
				Default:  p.Default,
			})
			return
		}
	}

	writeError(w, http.StatusNotFound, "provider not found", nil)
}

// AddProvider registers a new LLM provider dynamically.
func (h *AIHandler) AddProvider(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[addProviderRequest](w, r)
	if !ok {
		return
	}

	// Create the appropriate client based on type
	var client llm.Client
	switch req.Type {
	case "ollama":
		client = llm.NewOllamaClientDynamic(llm.OllamaClientConfig{
			Endpoint: req.Endpoint,
			Model:    req.Model,
		})
	case "openai":
		client = llm.NewOpenAIClient(req.Endpoint, req.Model, req.APIKey)
	case "vllm":
		client = llm.NewVLLMClient(req.Endpoint, req.Model, req.APIKey)
	default:
		writeError(w, http.StatusBadRequest, "unsupported provider type: "+req.Type, nil)
		return
	}

	// Wrap with circuit breaker
	cbClient := llm.NewCircuitBreakerClient(req.Name, client, llm.DefaultCircuitBreakerConfig())

	info := llm.ProviderInfo{
		Name:     req.Name,
		Type:     req.Type,
		Model:    req.Model,
		Endpoint: req.Endpoint,
		Status:   "unknown",
		Default:  req.Default,
	}

	h.registry.Register(req.Name, cbClient, info)

	writeJSON(w, http.StatusCreated, info)
}

// DeleteProvider removes a registered provider.
func (h *AIHandler) DeleteProvider(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	if err := h.registry.Unregister(name); err != nil {
		writeError(w, http.StatusNotFound, "provider not found", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "name": name})
}

// HealthCheckProvider triggers a health check for a specific provider.
func (h *AIHandler) HealthCheckProvider(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	client, err := h.registry.Get(name)
	if err != nil {
		writeError(w, http.StatusNotFound, "provider not found", err)
		return
	}

	if err := client.HealthCheck(r.Context()); err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"name":   name,
			"status": "down",
			"error":  err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"name":   name,
		"status": "healthy",
	})
}

// TestPrompt sends a test prompt to a specified provider.
func (h *AIHandler) TestPrompt(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[testPromptRequest](w, r)
	if !ok {
		return
	}

	var client llm.Client
	var err error
	if req.Provider != "" {
		client, err = h.registry.Get(req.Provider)
	} else {
		client, err = h.registry.Default()
	}
	if err != nil {
		writeError(w, http.StatusNotFound, "provider not available", err)
		return
	}

	resp, err := client.Complete(r.Context(), llm.CompletionRequest{
		Prompt:      req.Prompt,
		System:      req.System,
		Temperature: 0.3,
		MaxTokens:   2048,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "LLM completion failed", err)
		return
	}

	writeJSON(w, http.StatusOK, testPromptResponse{
		Content:        resp.Content,
		Model:          resp.Model,
		PromptTokens:   resp.PromptTokens,
		ResponseTokens: resp.ResponseTokens,
		DurationMs:     resp.Duration.Milliseconds(),
	})
}
