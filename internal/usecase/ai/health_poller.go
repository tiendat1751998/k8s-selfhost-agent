// Package ai provides background services for AI provider management.
package ai

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/ports"
	"github.com/datdt/k8sselfhost/internal/pkg/concurrency"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

// HealthPoller periodically checks the health of all registered AI providers
// and updates their status in the registry. It also broadcasts status changes
// via the provided callback.
type HealthPoller struct {
	registry     ports.LLMRegistry
	interval     time.Duration
	onStatusChange func(name string, result ports.LLMProviderHealthResult)
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// NewHealthPoller creates a new health poller.
// onStatusChange is called when a provider's status is updated after a health check.
func NewHealthPoller(
	registry ports.LLMRegistry,
	interval time.Duration,
	onStatusChange func(name string, result ports.LLMProviderHealthResult),
) *HealthPoller {
	return &HealthPoller{
		registry:     registry,
		interval:     interval,
		onStatusChange: onStatusChange,
	}
}

// Start begins the background health polling loop.
func (hp *HealthPoller) Start(ctx context.Context) {
	ctx, hp.cancel = context.WithCancel(ctx)
	hp.wg.Add(1)

	concurrency.Go(logger.Get(), func() {
		defer hp.wg.Done()
		log := logger.WithContext(ctx)

		log.Info("AI health poller started",
			zap.Duration("interval", hp.interval),
		)

		// Run immediately on start
		hp.pollAll(ctx)

		currentInterval := hp.interval
		maxInterval := hp.interval * 10 // Max backoff up to 10x the base interval when all are healthy
		
		timer := time.NewTimer(currentInterval)
		defer timer.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Info("AI health poller stopped")
				return
			case <-timer.C:
				allHealthy := hp.pollAll(ctx)

				// Backoff polling frequency (increase interval) when all are healthy to save API costs.
				// Reset to short base interval when any provider is down to quickly detect recovery.
				if allHealthy {
					currentInterval = time.Duration(float64(currentInterval) * 1.5)
					if currentInterval > maxInterval {
						currentInterval = maxInterval
					}
				} else {
					currentInterval = hp.interval
				}
				
				timer.Reset(currentInterval)
			}
		}
	})
}

// Stop gracefully stops the health poller and waits for completion.
func (hp *HealthPoller) Stop() {
	if hp.cancel != nil {
		hp.cancel()
	}
	hp.wg.Wait()
}

// pollAll checks all registered providers and broadcasts results.
// Returns true if all registered providers are healthy.
func (hp *HealthPoller) pollAll(ctx context.Context) bool {
	log := logger.WithContext(ctx)

	if hp.registry.Count() == 0 {
		return true
	}

	results := hp.registry.HealthCheckAll(ctx)
	allHealthy := true

	for name, result := range results {
		log.Debug("AI provider health check",
			zap.String("provider", name),
			zap.String("status", result.Status),
			zap.Duration("latency", result.Latency),
		)

		if result.Status != "healthy" {
			allHealthy = false
		}

		if hp.onStatusChange != nil {
			hp.onStatusChange(name, result)
		}
	}
	return allHealthy
}
