package llm

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mockClient struct {
	completeFn func(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
	healthFn   func(ctx context.Context) error
}

func (m *mockClient) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	if m.completeFn != nil {
		return m.completeFn(ctx, req)
	}
	return &CompletionResponse{Content: "mock"}, nil
}

func (m *mockClient) HealthCheck(ctx context.Context) error {
	if m.healthFn != nil {
		return m.healthFn(ctx)
	}
	return nil
}

func TestCircuitBreaker_Closed(t *testing.T) {
	inner := &mockClient{}
	cfg := CircuitBreakerConfig{
		MaxFailures:      3,
		ResetTimeout:     10 * time.Millisecond,
		HalfOpenMaxCalls: 1,
	}
	cb := NewCircuitBreakerClient("test", inner, cfg)

	if cb.State() != CircuitClosed {
		t.Errorf("expected state closed, got %v", cb.State())
	}

	_, err := cb.Complete(context.Background(), CompletionRequest{Prompt: "test"})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestCircuitBreaker_OpensOnFailures(t *testing.T) {
	calls := 0
	inner := &mockClient{
		completeFn: func(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
			calls++
			return nil, errors.New("llm error")
		},
	}
	cfg := CircuitBreakerConfig{
		MaxFailures:      2,
		ResetTimeout:     100 * time.Millisecond,
		HalfOpenMaxCalls: 1,
	}
	cb := NewCircuitBreakerClient("test", inner, cfg)

	// Call 1: failure
	_, err := cb.Complete(context.Background(), CompletionRequest{Prompt: "test"})
	if err == nil {
		t.Error("expected error, got nil")
	}
	if cb.State() != CircuitClosed {
		t.Errorf("expected state closed after 1 failure, got %v", cb.State())
	}

	// Call 2: failure -> trips circuit open
	_, err = cb.Complete(context.Background(), CompletionRequest{Prompt: "test"})
	if err == nil {
		t.Error("expected error, got nil")
	}
	if cb.State() != CircuitOpen {
		t.Errorf("expected state open after 2 failures, got %v", cb.State())
	}

	// Call 3: rejected immediately
	_, err = cb.Complete(context.Background(), CompletionRequest{Prompt: "test"})
	if err == nil {
		t.Error("expected fast fail error, got nil")
	}
	if calls != 2 {
		t.Errorf("expected only 2 calls to inner client, got %d", calls)
	}
}

func TestCircuitBreaker_HalfOpenAndRecovery(t *testing.T) {
	var errToReturn error = errors.New("fail")
	inner := &mockClient{
		completeFn: func(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
			return nil, errToReturn
		},
	}
	cfg := CircuitBreakerConfig{
		MaxFailures:      1,
		ResetTimeout:     10 * time.Millisecond,
		HalfOpenMaxCalls: 1,
	}
	cb := NewCircuitBreakerClient("test", inner, cfg)

	// Trip open
	_, _ = cb.Complete(context.Background(), CompletionRequest{Prompt: "test"})
	if cb.State() != CircuitOpen {
		t.Fatal("circuit breaker should be open")
	}

	// Wait cooldown
	time.Sleep(15 * time.Millisecond)

	// State should become half-open
	if cb.State() != CircuitHalfOpen {
		t.Errorf("expected half-open state, got %v", cb.State())
	}

	// Next request succeeds
	errToReturn = nil
	_, err := cb.Complete(context.Background(), CompletionRequest{Prompt: "test"})
	if err != nil {
		t.Errorf("expected successful probe request, got err: %v", err)
	}

	// Recovery back to closed
	if cb.State() != CircuitClosed {
		t.Errorf("expected recovery to closed state, got %v", cb.State())
	}
}
