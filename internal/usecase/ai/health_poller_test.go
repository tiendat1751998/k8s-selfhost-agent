package ai

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/ports"
)

type mockLLMClient struct {
	healthCheckErr error
}

func (m *mockLLMClient) Complete(ctx context.Context, req ports.LLMCompletionRequest) (*ports.LLMCompletionResponse, error) {
	return &ports.LLMCompletionResponse{Content: "mock"}, nil
}

func (m *mockLLMClient) HealthCheck(ctx context.Context) error {
	return m.healthCheckErr
}

type mockLLMRegistry struct {
	mu        sync.RWMutex
	providers map[string]ports.LLMClient
	infos     map[string]ports.LLMProviderInfo
}

func newMockLLMRegistry() *mockLLMRegistry {
	return &mockLLMRegistry{
		providers: make(map[string]ports.LLMClient),
		infos:     make(map[string]ports.LLMProviderInfo),
	}
}

func (r *mockLLMRegistry) Register(name string, client ports.LLMClient, info ports.LLMProviderInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = client
	r.infos[name] = info
}

func (r *mockLLMRegistry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.providers, name)
	delete(r.infos, name)
	return nil
}

func (r *mockLLMRegistry) Get(name string) (ports.LLMClient, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.providers[name]
	if !ok {
		return nil, errors.New("provider not found")
	}
	return c, nil
}

func (r *mockLLMRegistry) Default() (ports.LLMClient, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, c := range r.providers {
		return c, nil
	}
	return nil, errors.New("no providers")
}

func (r *mockLLMRegistry) DefaultName() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for name := range r.providers {
		return name
	}
	return ""
}

func (r *mockLLMRegistry) List() []ports.LLMProviderInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []ports.LLMProviderInfo
	for _, info := range r.infos {
		list = append(list, info)
	}
	return list
}

func (r *mockLLMRegistry) HealthCheckAll(ctx context.Context) map[string]ports.LLMProviderHealthResult {
	r.mu.RLock()
	defer r.mu.RUnlock()
	results := make(map[string]ports.LLMProviderHealthResult)
	for name, client := range r.providers {
		start := time.Now()
		err := client.HealthCheck(ctx)
		latency := time.Since(start)
		if err != nil {
			results[name] = ports.LLMProviderHealthResult{
				Status:  "unhealthy",
				Latency: latency,
				Error:   err.Error(),
			}
		} else {
			results[name] = ports.LLMProviderHealthResult{
				Status:  "healthy",
				Latency: latency,
			}
		}
	}
	return results
}

func (r *mockLLMRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.providers)
}

func TestHealthPoller_StartStop(t *testing.T) {
	reg := newMockLLMRegistry()
	client := &mockLLMClient{}
	reg.Register("test-provider", client, ports.LLMProviderInfo{
		Name:   "test-provider",
		Type:   "openai",
		Model:  "gpt-4",
		Status: "healthy",
	})

	var mu sync.Mutex
	statusChanges := make(map[string]ports.LLMProviderHealthResult)

	poller := NewHealthPoller(reg, 10*time.Millisecond, func(name string, result ports.LLMProviderHealthResult) {
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
	reg := newMockLLMRegistry()
	client := &mockLLMClient{}
	reg.Register("test-provider", client, ports.LLMProviderInfo{
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
