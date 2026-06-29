// Package llm provides an OpenAI-compatible LLM client.
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

	"github.com/datdt/k8sselfhost/pkg/logger"
)

// OpenAIClient implements the Client interface for OpenAI-compatible APIs
// (OpenAI, Azure OpenAI, LiteLLM, Groq, Together AI, etc.).
type OpenAIClient struct {
	endpoint   string
	model      string
	apiKey     string
	httpClient *http.Client
}

// openaiChatRequest is the OpenAI Chat Completions API request format.
type openaiChatRequest struct {
	Model       string              `json:"model"`
	Messages    []openaiChatMessage `json:"messages"`
	Temperature float64             `json:"temperature,omitempty"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
	Stream      bool                `json:"stream"`
}

type openaiChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openaiChatResponse is the OpenAI Chat Completions API response format.
type openaiChatResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// openaiModelsResponse is the OpenAI Models API response format.
type openaiModelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// NewOpenAIClient creates a new OpenAI-compatible LLM client.
func NewOpenAIClient(endpoint, model, apiKey string) *OpenAIClient {
	return &OpenAIClient{
		endpoint: endpoint,
		model:    model,
		apiKey:   apiKey,
		httpClient: &http.Client{
			Timeout: 2 * time.Minute,
		},
	}
}

// Complete sends a prompt to the OpenAI-compatible API and returns the response.
func (c *OpenAIClient) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	log := logger.WithContext(ctx)

	model := req.Model
	if model == "" {
		model = c.model
	}

	messages := make([]openaiChatMessage, 0, 2)
	if req.System != "" {
		messages = append(messages, openaiChatMessage{Role: "system", Content: req.System})
	}
	messages = append(messages, openaiChatMessage{Role: "user", Content: req.Prompt})

	chatReq := openaiChatRequest{
		Model:       model,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      false,
	}

	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("marshaling OpenAI request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/chat/completions", c.endpoint)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	start := time.Now()
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("calling OpenAI API: %w", err)
	}
	defer httpResp.Body.Close()
	duration := time.Since(start)

	if httpResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("OpenAI API returned status %d: %s", httpResp.StatusCode, string(respBody))
	}

	var chatResp openaiChatResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("decoding OpenAI response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("OpenAI API returned no choices")
	}

	log.Debug("OpenAI completion finished",
		zap.String("model", model),
		zap.Int("prompt_tokens", chatResp.Usage.PromptTokens),
		zap.Int("response_tokens", chatResp.Usage.CompletionTokens),
		zap.Duration("duration", duration),
	)

	return &CompletionResponse{
		Content:        chatResp.Choices[0].Message.Content,
		Model:          chatResp.Model,
		PromptTokens:   chatResp.Usage.PromptTokens,
		ResponseTokens: chatResp.Usage.CompletionTokens,
		Duration:       duration,
	}, nil
}

// HealthCheck verifies the OpenAI-compatible service is reachable.
func (c *OpenAIClient) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/v1/models", c.endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating health check request: %w", err)
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("OpenAI health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("OpenAI returned status %d", resp.StatusCode)
	}

	return nil
}
