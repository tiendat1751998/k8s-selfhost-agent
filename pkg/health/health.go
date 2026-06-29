// Package health provides health check HTTP handlers for service dependencies.
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Status represents the health status of a component.
type Status string

const (
	StatusUp   Status = "up"
	StatusDown Status = "down"
)

// ComponentHealth represents the health of a single dependency.
type ComponentHealth struct {
	Status  Status `json:"status"`
	Latency string `json:"latency,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Response is the full health check response.
type Response struct {
	Status     Status                     `json:"status"`
	Components map[string]ComponentHealth `json:"components"`
	Timestamp  string                     `json:"timestamp"`
}

// Checker is a function that checks the health of a dependency.
// It returns an error if the dependency is unhealthy.
type Checker func(ctx context.Context) error

// Handler manages health check registrations and serves HTTP responses.
type Handler struct {
	mu       sync.RWMutex
	checkers map[string]Checker
	timeout  time.Duration
}

// NewHandler creates a new health check handler with the given timeout per check.
func NewHandler(timeout time.Duration) *Handler {
	return &Handler{
		checkers: make(map[string]Checker),
		timeout:  timeout,
	}
}

// Register adds a named health checker.
func (h *Handler) Register(name string, checker Checker) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checkers[name] = checker
}

// ServeHTTP implements http.Handler and runs all registered health checks.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	checkers := make(map[string]Checker, len(h.checkers))
	for k, v := range h.checkers {
		checkers[k] = v
	}
	h.mu.RUnlock()

	resp := Response{
		Status:     StatusUp,
		Components: make(map[string]ComponentHealth, len(checkers)),
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for name, checker := range checkers {
		wg.Add(1)
		go func(name string, checker Checker) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
			defer cancel()

			start := time.Now()
			err := checker(ctx)
			latency := time.Since(start)

			ch := ComponentHealth{
				Status:  StatusUp,
				Latency: latency.String(),
			}

			if err != nil {
				ch.Status = StatusDown
				ch.Error = err.Error()
			}

			mu.Lock()
			resp.Components[name] = ch
			if ch.Status == StatusDown {
				resp.Status = StatusDown
			}
			mu.Unlock()
		}(name, checker)
	}

	wg.Wait()

	w.Header().Set("Content-Type", "application/json")
	if resp.Status == StatusDown {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	_ = json.NewEncoder(w).Encode(resp)
}

// LivenessHandler returns a simple liveness probe that always returns 200 OK.
func LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "alive",
		})
	}
}
