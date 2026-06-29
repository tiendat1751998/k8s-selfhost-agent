// Package llm provides a vLLM client using the OpenAI-compatible API that vLLM exposes.
package llm

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// VLLMClient implements the Client interface for vLLM servers.
// vLLM exposes an OpenAI-compatible API, so this delegates to OpenAIClient
// with vLLM-specific health checking.
type VLLMClient struct {
	inner      *OpenAIClient
	endpoint   string
	httpClient *http.Client
}

// NewVLLMClient creates a new vLLM client.
func NewVLLMClient(endpoint, model, apiKey string) *VLLMClient {
	return &VLLMClient{
		inner:    NewOpenAIClient(endpoint, model, apiKey),
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Complete delegates to the underlying OpenAI-compatible client.
func (c *VLLMClient) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	return c.inner.Complete(ctx, req)
}

// HealthCheck verifies the vLLM service is reachable using the vLLM-specific health endpoint.
func (c *VLLMClient) HealthCheck(ctx context.Context) error {
	// vLLM provides a /health endpoint in addition to OpenAI-compatible /v1/models
	url := fmt.Sprintf("%s/health", c.endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating vLLM health check request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("vLLM health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("vLLM returned status %d", resp.StatusCode)
	}

	return nil
}
