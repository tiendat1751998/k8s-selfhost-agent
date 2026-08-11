package ports

import (
	"context"
	"time"
)

// LLMCompletionRequest holds the parameters for an LLM completion.
type LLMCompletionRequest struct {
	Model       string  `json:"model"`
	Prompt      string  `json:"prompt"`
	System      string  `json:"system,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
}

// LLMCompletionResponse holds the LLM response.
type LLMCompletionResponse struct {
	Content        string        `json:"content"`
	Model          string        `json:"model"`
	PromptTokens   int           `json:"prompt_tokens"`
	ResponseTokens int           `json:"response_tokens"`
	Duration       time.Duration `json:"duration"`
}

// LLMProviderHealthResult holds the health check result for a single provider.
type LLMProviderHealthResult struct {
	Status  string        `json:"status"`
	Latency time.Duration `json:"latency"`
	Error   string        `json:"error,omitempty"`
}

// LLMProviderInfo describes a registered LLM provider's metadata and health status.
type LLMProviderInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Model    string `json:"model"`
	Endpoint string `json:"endpoint"`
	Status   string `json:"status"`
	Latency  string `json:"latency,omitempty"`
	Default  bool   `json:"default"`
}

// LLMClient defines the interface for LLM interactions.
type LLMClient interface {
	Complete(ctx context.Context, req LLMCompletionRequest) (*LLMCompletionResponse, error)
	HealthCheck(ctx context.Context) error
}

// LLMRegistry defines the interface for managing multiple LLM provider clients.
type LLMRegistry interface {
	Register(name string, client LLMClient, info LLMProviderInfo)
	Unregister(name string) error
	Get(name string) (LLMClient, error)
	Default() (LLMClient, error)
	DefaultName() string
	List() []LLMProviderInfo
	HealthCheckAll(ctx context.Context) map[string]LLMProviderHealthResult
	Count() int
}
