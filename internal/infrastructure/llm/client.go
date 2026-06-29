// Package llm provides the LLM client interface and Ollama implementation.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/infrastructure/config"
	"github.com/datdt/k8sselfhost/pkg/logger"
)

// Client defines the interface for LLM interactions.
type Client interface {
	// Complete sends a prompt to the LLM and returns the generated response.
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
	// HealthCheck verifies the LLM service is reachable.
	HealthCheck(ctx context.Context) error
}

// CompletionRequest holds the parameters for an LLM completion.
type CompletionRequest struct {
	Model       string  `json:"model"`
	Prompt      string  `json:"prompt"`
	System      string  `json:"system,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
}

// CompletionResponse holds the LLM response.
type CompletionResponse struct {
	Content        string `json:"content"`
	Model          string `json:"model"`
	PromptTokens   int    `json:"prompt_tokens"`
	ResponseTokens int    `json:"response_tokens"`
	Duration       time.Duration `json:"duration"`
}

// OllamaClient implements the Client interface using the Ollama API.
type OllamaClient struct {
	endpoint   string
	model      string
	httpClient *http.Client
}

// ollamaRequest is the Ollama API request format.
type ollamaRequest struct {
	Model       string  `json:"model"`
	Prompt      string  `json:"prompt"`
	System      string  `json:"system,omitempty"`
	Stream      bool    `json:"stream"`
	Options     *ollamaOptions `json:"options,omitempty"`
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

// ollamaResponse is the Ollama API response format.
type ollamaResponse struct {
	Model              string `json:"model"`
	Response           string `json:"response"`
	Done               bool   `json:"done"`
	TotalDuration      int64  `json:"total_duration"`
	PromptEvalCount    int    `json:"prompt_eval_count"`
	EvalCount          int    `json:"eval_count"`
}

// NewOllamaClient creates a new Ollama LLM client from application config.
func NewOllamaClient(cfg config.LLMConfig) *OllamaClient {
	return &OllamaClient{
		endpoint: cfg.Endpoint,
		model:    cfg.Model,
		httpClient: &http.Client{
			Timeout: 2 * time.Minute,
		},
	}
}

// OllamaClientConfig holds minimal config for creating an Ollama client dynamically.
type OllamaClientConfig struct {
	Endpoint string
	Model    string
}

// NewOllamaClientDynamic creates a new Ollama LLM client from a lightweight config.
// Use this when creating clients dynamically (e.g., from the AI handler) without
// importing the full config package.
func NewOllamaClientDynamic(cfg OllamaClientConfig) *OllamaClient {
	return &OllamaClient{
		endpoint: cfg.Endpoint,
		model:    cfg.Model,
		httpClient: &http.Client{
			Timeout: 2 * time.Minute,
		},
	}
}

// Complete sends a prompt to Ollama and returns the response.
func (c *OllamaClient) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	log := logger.WithContext(ctx)

	model := req.Model
	if model == "" {
		model = c.model
	}

	ollamaReq := ollamaRequest{
		Model:  model,
		Prompt: req.Prompt,
		System: req.System,
		Stream: false,
	}

	if req.Temperature > 0 || req.MaxTokens > 0 {
		ollamaReq.Options = &ollamaOptions{
			Temperature: req.Temperature,
			NumPredict:  req.MaxTokens,
		}
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("marshaling ollama request: %w", err)
	}

	url := fmt.Sprintf("%s/api/generate", c.endpoint)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	start := time.Now()
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("calling Ollama API: %w", err)
	}
	defer httpResp.Body.Close()
	duration := time.Since(start)

	if httpResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("ollama API returned status %d: %s", httpResp.StatusCode, string(respBody))
	}

	var ollamaResp ollamaResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("decoding ollama response: %w", err)
	}

	log.Debug("LLM completion finished",
		zap.String("model", model),
		zap.Int("prompt_tokens", ollamaResp.PromptEvalCount),
		zap.Int("response_tokens", ollamaResp.EvalCount),
		zap.Duration("duration", duration),
	)

	return &CompletionResponse{
		Content:        ollamaResp.Response,
		Model:          ollamaResp.Model,
		PromptTokens:   ollamaResp.PromptEvalCount,
		ResponseTokens: ollamaResp.EvalCount,
		Duration:       duration,
	}, nil
}

// HealthCheck verifies the Ollama service is reachable.
func (c *OllamaClient) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/tags", c.endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating health check request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ollama health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	return nil
}
