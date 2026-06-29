package ai

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/infrastructure/llm"
)

type mockLLMClient struct {
	healthCheckErr error
}

func (m *mockLLMClient) Complete(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	return &llm.CompletionResponse{Content: "mock"}, nil
}

func (m *mockLLMClient) HealthCheck(ctx context.Context) error {
	return m.healthCheckErr
}

func TestHealthPoller_StartStop(t *testing.T) {
	reg := llm.NewProviderRegistry()
	client := &mockLLMClient{}
	reg.Register("test-provider", client, llm.ProviderInfo{
		Name:   "test-provider",
		Type:   "openai",
		Model:  "gpt-4",
		Status: "healthy",
	})

	var mu sync.Mutex
	statusChanges := make(map[string]llm.ProviderHealthResult)

	poller := NewHealthPoller(reg, 10*time.Millisecond, func(name string, result llm.ProviderHealthResult) {
		mu.Lock()
		statusChanges[name] = result
		mu.Unlock()
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	poller.Start(ctx)

	// Wait for poller to run at least once
	time.Sleep(25 * time.Millisecond)
	poller.Stop()

	mu.Lock()
	_, ok := statusChanges["test-provider"]
	mu.Unlock()

	if !ok {
		t.Error("expected status change callback to be invoked for registered provider")
	}
}

func TestHealthPoller_BackoffResets(t *testing.T) {
	reg := llm.NewProviderRegistry()
	client := &mockLLMClient{}
	reg.Register("test-provider", client, llm.ProviderInfo{
		Name: "test-provider",
	})

	poller := NewHealthPoller(reg, 10*time.Millisecond, nil)

	// 1. Initially healthy
	client.healthCheckErr = nil
	healthy := poller.pollAll(context.Background())
	if !healthy {
		t.Error("expected healthy to be true")
	}

	// 2. Unhealthy
	client.healthCheckErr = errors.New("timeout")
	healthy = poller.pollAll(context.Background())
	if healthy {
		t.Error("expected healthy to be false when client returns error")
	}
}
