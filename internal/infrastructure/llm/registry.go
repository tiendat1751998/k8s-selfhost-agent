// Package llm provides the ProviderRegistry for managing multiple LLM backends.
package llm

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/ports"
	"github.com/datdt/k8sselfhost/internal/infrastructure/config"
	"github.com/datdt/k8sselfhost/internal/pkg/concurrency"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

// ProviderInfo describes a registered LLM provider's metadata and health status.
type ProviderInfo = ports.LLMProviderInfo

// ProviderRegistry manages multiple LLM provider clients with thread-safe access.
type ProviderRegistry struct {
	providers    map[string]Client
	metadata     map[string]ProviderInfo
	defaultName  string
	mu           sync.RWMutex
}

// NewProviderRegistry creates a new empty provider registry.
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]Client),
		metadata:  make(map[string]ProviderInfo),
	}
}

// Register adds a new provider client with its metadata.
// If info.Default is true, this provider becomes the default.
func (r *ProviderRegistry) Register(name string, client Client, info ProviderInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.providers[name] = client
	info.Name = name
	r.metadata[name] = info

	if info.Default || r.defaultName == "" {
		r.defaultName = name
	}

	logger.Get().Info("registered LLM provider",
		zap.String("name", name),
		zap.String("type", info.Type),
		zap.String("model", info.Model),
		zap.Bool("default", info.Default),
	)
}

// Unregister removes a provider by name.
func (r *ProviderRegistry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[name]; !exists {
		return fmt.Errorf("provider %q not found", name)
	}

	delete(r.providers, name)
	delete(r.metadata, name)

	if r.defaultName == name {
		r.defaultName = ""
		for n, info := range r.metadata {
			r.defaultName = n
			_ = info
			break
		}
	}

	return nil
}

// Get returns a provider client by name.
func (r *ProviderRegistry) Get(name string) (Client, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	client, exists := r.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider %q not registered", name)
	}
	return client, nil
}

// Default returns the default provider client, falling back to other healthy providers if the default's circuit is tripped.
func (r *ProviderRegistry) Default() (Client, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.defaultName == "" {
		return nil, fmt.Errorf("no default provider configured")
	}
	client, exists := r.providers[r.defaultName]
	if !exists {
		return nil, fmt.Errorf("default provider %q not found", r.defaultName)
	}

	// Check if default provider is circuit-broken
	if cb, ok := client.(*CircuitBreakerClient); ok {
		if cb.State() == CircuitOpen {
			logger.Get().Warn("default provider circuit is open, attempting fallback to other providers", zap.String("default_provider", r.defaultName))
			// Find another healthy provider
			for name, c := range r.providers {
				if name == r.defaultName {
					continue
				}
				if otherCb, ok := c.(*CircuitBreakerClient); ok {
					if otherCb.State() != CircuitOpen {
						logger.Get().Info("selected fallback provider", zap.String("fallback_provider", name))
						return c, nil
					}
				} else {
					// Client doesn't wrap circuit breaker, assume healthy
					logger.Get().Info("selected fallback provider (no circuit breaker)", zap.String("fallback_provider", name))
					return c, nil
				}
			}
		}
	}

	return client, nil
}

// DefaultName returns the name of the default provider.
func (r *ProviderRegistry) DefaultName() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.defaultName
}

// List returns metadata for all registered providers.
func (r *ProviderRegistry) List() []ProviderInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ProviderInfo, 0, len(r.metadata))
	for _, info := range r.metadata {
		result = append(result, info)
	}
	return result
}

// HealthCheckAll probes all registered providers and returns their health status.
func (r *ProviderRegistry) HealthCheckAll(ctx context.Context) map[string]ProviderHealthResult {
	r.mu.RLock()
	names := make([]string, 0, len(r.providers))
	clients := make([]Client, 0, len(r.providers))
	for name, client := range r.providers {
		names = append(names, name)
		clients = append(clients, client)
	}
	r.mu.RUnlock()

	results := make(map[string]ProviderHealthResult, len(names))
	var wg sync.WaitGroup

	type result struct {
		name   string
		health ProviderHealthResult
	}
	ch := make(chan result, len(names))

	for i, name := range names {
		wg.Add(1)
		nVal := name
		cVal := clients[i]
		concurrency.Go(logger.Get(), func() {
			defer wg.Done()
			start := time.Now()
			err := cVal.HealthCheck(ctx)
			latency := time.Since(start)

			hr := ProviderHealthResult{
				Latency: latency,
			}
			if err != nil {
				hr.Status = "down"
				hr.Error = err.Error()
			} else {
				hr.Status = "healthy"
			}

			ch <- result{name: nVal, health: hr}
		})
	}

	wg.Wait()
	close(ch)

	for res := range ch {
		results[res.name] = res.health

		// Update metadata with latest health status
		r.mu.Lock()
		if info, ok := r.metadata[res.name]; ok {
			info.Status = res.health.Status
			info.Latency = res.health.Latency.String()
			r.metadata[res.name] = info
		}
		r.mu.Unlock()
	}

	return results
}

// ProviderHealthResult holds the health check result for a single provider.
type ProviderHealthResult = ports.LLMProviderHealthResult

// Count returns the number of registered providers.
func (r *ProviderRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.providers)
}

// InitRegistry initializes and returns a ProviderRegistry from a configuration.
func InitRegistry(cfg config.LLMConfig) *ProviderRegistry {
	log := logger.Get()
	registry := NewProviderRegistry()

	for _, pCfg := range cfg.Providers {
		var client Client
		switch pCfg.Type {
		case "ollama":
			client = NewOllamaClientDynamic(OllamaClientConfig{
				Endpoint: pCfg.Endpoint,
				Model:    pCfg.Model,
			})
		case "openai":
			client = NewOpenAIClient(pCfg.Endpoint, pCfg.Model, pCfg.APIKey)
		case "vllm":
			client = NewVLLMClient(pCfg.Endpoint, pCfg.Model, pCfg.APIKey)
		default:
			log.Warn("unknown LLM provider type, skipping", zap.String("type", pCfg.Type))
			continue
		}

		cbClient := NewCircuitBreakerClient(pCfg.Name, client, DefaultCircuitBreakerConfig())
		registry.Register(pCfg.Name, cbClient, ProviderInfo{
			Type:     pCfg.Type,
			Model:    pCfg.Model,
			Endpoint: pCfg.Endpoint,
			Default:  pCfg.Default,
		})
	}

	// Register default fallback if empty
	if registry.Count() == 0 && cfg.Endpoint != "" {
		client := NewOllamaClientDynamic(OllamaClientConfig{
			Endpoint: cfg.Endpoint,
			Model:    cfg.Model,
		})
		cbClient := NewCircuitBreakerClient("default", client, DefaultCircuitBreakerConfig())
		registry.Register("default", cbClient, ProviderInfo{
			Type:     "ollama",
			Model:    cfg.Model,
			Endpoint: cfg.Endpoint,
			Default:  true,
		})
	}

	return registry
}
