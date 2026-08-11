// Package llm provides a circuit breaker wrapper for LLM clients.
package llm

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

// CircuitState represents the state of the circuit breaker.
type CircuitState int

const (
	// CircuitClosed is the normal operating state — requests flow through.
	CircuitClosed CircuitState = iota
	// CircuitOpen means the circuit is tripped — all requests fail immediately.
	CircuitOpen
	// CircuitHalfOpen allows a single probe request to test recovery.
	CircuitHalfOpen
)

// String returns the string representation of a CircuitState.
func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig configures the circuit breaker behavior.
type CircuitBreakerConfig struct {
	// MaxFailures is the number of consecutive failures before opening the circuit.
	MaxFailures int
	// ResetTimeout is how long to wait before transitioning from open to half-open.
	ResetTimeout time.Duration
	// HalfOpenMaxCalls is the number of calls allowed in half-open state.
	HalfOpenMaxCalls int
	// RequestTimeout is how long to wait before timing out LLM requests.
	RequestTimeout time.Duration
}

// DefaultCircuitBreakerConfig returns sensible defaults for LLM circuit breaking.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		MaxFailures:      3,
		ResetTimeout:     30 * time.Second,
		HalfOpenMaxCalls: 1,
		RequestTimeout:   30 * time.Second,
	}
}

// CircuitBreakerClient wraps an LLM Client with circuit breaker logic.
type CircuitBreakerClient struct {
	inner          Client
	name           string
	cfg            CircuitBreakerConfig
	state          CircuitState
	failures       int
	lastFailure    time.Time
	halfOpenCalls  int
	mu             sync.Mutex
}

// NewCircuitBreakerClient wraps an existing Client with circuit breaker protection.
func NewCircuitBreakerClient(name string, inner Client, cfg CircuitBreakerConfig) *CircuitBreakerClient {
	return &CircuitBreakerClient{
		inner: inner,
		name:  name,
		cfg:   cfg,
		state: CircuitClosed,
	}
}

// Complete sends a prompt through the circuit breaker.
func (cb *CircuitBreakerClient) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	if err := cb.allowRequest(); err != nil {
		return nil, err
	}

	timeout := cb.cfg.RequestTimeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	resp, err := cb.inner.Complete(timeoutCtx, req)
	cb.recordResult(err)
	return resp, err
}

// HealthCheck delegates to the inner client, respecting circuit state.
func (cb *CircuitBreakerClient) HealthCheck(ctx context.Context) error {
	// Always allow health checks regardless of circuit state
	return cb.inner.HealthCheck(ctx)
}

// State returns the current circuit state.
func (cb *CircuitBreakerClient) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.currentState()
}

// allowRequest checks if a request is allowed through the circuit.
func (cb *CircuitBreakerClient) allowRequest() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.currentState() {
	case CircuitClosed:
		return nil
	case CircuitOpen:
		return fmt.Errorf("circuit breaker is OPEN for provider %q — failing fast (last failure: %s ago)",
			cb.name, time.Since(cb.lastFailure).Round(time.Second))
	case CircuitHalfOpen:
		if cb.halfOpenCalls >= cb.cfg.HalfOpenMaxCalls {
			return fmt.Errorf("circuit breaker is HALF-OPEN for provider %q — max probe calls reached", cb.name)
		}
		cb.halfOpenCalls++
		return nil
	}
	return nil
}

// recordResult records the outcome of a request and transitions state accordingly.
func (cb *CircuitBreakerClient) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	log := logger.Get()

	if err != nil {
		cb.failures++
		cb.lastFailure = time.Now()

		if cb.state == CircuitHalfOpen {
			// Probe failed — re-open the circuit
			cb.state = CircuitOpen
			log.Warn("circuit breaker re-opened (half-open probe failed)",
				zap.String("provider", cb.name),
				zap.Error(err),
			)
			return
		}

		if cb.failures >= cb.cfg.MaxFailures {
			cb.state = CircuitOpen
			log.Warn("circuit breaker opened",
				zap.String("provider", cb.name),
				zap.Int("consecutive_failures", cb.failures),
				zap.Error(err),
			)
		}
	} else {
		if cb.state == CircuitHalfOpen {
			// Probe succeeded — close the circuit
			log.Info("circuit breaker closed (probe succeeded)",
				zap.String("provider", cb.name),
			)
		}
		cb.state = CircuitClosed
		cb.failures = 0
		cb.halfOpenCalls = 0
	}
}

// currentState returns the effective state, considering timeout-based transitions.
func (cb *CircuitBreakerClient) currentState() CircuitState {
	if cb.state == CircuitOpen {
		if time.Since(cb.lastFailure) >= cb.cfg.ResetTimeout {
			cb.state = CircuitHalfOpen
			cb.halfOpenCalls = 0
			logger.Get().Info("circuit breaker transitioning to half-open",
				zap.String("provider", cb.name),
			)
		}
	}
	return cb.state
}
