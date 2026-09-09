package loadbalancer

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	domainLB "github.com/datdt/k8sselfhost/internal/domain/loadbalancer"
)

// entrypointSample tracks raw counters across entrypoints for delta RPS and error calculations.
type entrypointSample struct {
	totalRequests int64
	status2xx     int64
	status4xx     int64
	status5xx     int64
	latencySumSec float64
	latencyCount  int64
	openConns     int
}

// safeDeltaInt64 protects against negative deltas when counters reset across restarts.
func safeDeltaInt64(curr, prev int64) int64 {
	if curr < prev {
		// Counter was reset after service restart / container recreation
		return curr
	}
	return curr - prev
}


func isInternalEntrypoint(ep string) bool {
	ep = strings.ToLower(strings.TrimSpace(ep))
	return ep == "traefik" || ep == "dashboard" || ep == "internal" || ep == "metrics"
}


// GetAggregateStats queries Traefik Prometheus metrics to retrieve entrypoint aggregate throughput.
func (p *TraefikProvider) GetAggregateStats(ctx context.Context) (*domainLB.AggregateStats, error) {
	now := time.Now().UTC()

	// 1. Fetch Prometheus metrics from Traefik (cached per cycle / background)
	_, entrypoint, err := p.getPrometheusMetrics(ctx)
	if err != nil {
		p.mu.Lock()
		defer p.mu.Unlock()
		// If Traefik is temporarily unresponsive under extreme load, maintain the last calculated RPS with smooth EMA decay
		if !p.prevAggregateTime.IsZero() && p.prevSmoothedRPS > 0 {
			decayedRPS := (1.0 - defaultEMAAlpha*0.5) * p.prevSmoothedRPS
			if decayedRPS < 0.05 {
				decayedRPS = 0
			}
			p.prevSmoothedRPS = decayedRPS
			rps := math.Round(decayedRPS*100) / 100
			agg := &domainLB.AggregateStats{
				TotalRequests:       p.prevAggregateStats.totalRequests,
				TotalRequestsPerSec: rps,
				ActiveConnections:   p.prevAggregateStats.openConns,
				ErrorRate:           0,
				AvgLatencyMs:        0,
				MaxLatencyMs:        0,
			}
			if p.lastCalculatedAggregate != nil {
				agg.ErrorRate = p.lastCalculatedAggregate.ErrorRate
				agg.AvgLatencyMs = p.lastCalculatedAggregate.AvgLatencyMs
				agg.MaxLatencyMs = p.lastCalculatedAggregate.MaxLatencyMs
				if agg.ActiveConnections == 0 {
					agg.ActiveConnections = p.lastCalculatedAggregate.ActiveConnections
				}
			}
			p.lastCalculatedAggregate = agg
			return agg, nil
		}
		return nil, fmt.Errorf("fetching traefik prometheus metrics: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	var rps float64
	var errRate float64
	var avgLatencyMs float64
	var maxLatencyMs float64

	if !p.prevAggregateTime.IsZero() && p.prevAggregateStats.totalRequests > 0 {
		elapsed := now.Sub(p.prevAggregateTime).Seconds()
		if elapsed > 0 {
			reqDelta := safeDeltaInt64(entrypoint.totalRequests, p.prevAggregateStats.totalRequests)
			rawRPS := float64(reqDelta) / elapsed

			// Exponential Moving Average (EMA) smoothing for stable live reporting
			var smoothedRPS float64
			if reqDelta > 0 {
				p.zeroDeltaCount = 0
				if p.prevSmoothedRPS == 0 || math.Abs(rawRPS-p.prevSmoothedRPS) < 0.01 {
					smoothedRPS = rawRPS
				} else {
					smoothedRPS = defaultEMAAlpha*rawRPS + (1.0-defaultEMAAlpha)*p.prevSmoothedRPS
				}
			} else {
				p.zeroDeltaCount++
				if p.zeroDeltaCount >= 2 {
					// Decay smoothly to 0 after consecutive idle cycles
					smoothedRPS = (1.0 - defaultEMAAlpha) * p.prevSmoothedRPS
					if smoothedRPS < 0.05 {
						smoothedRPS = 0
					}
				} else {
					smoothedRPS = (1.0 - defaultEMAAlpha) * p.prevSmoothedRPS
					if smoothedRPS < 0.05 {
						smoothedRPS = 0
					}
				}
			}
			p.prevSmoothedRPS = smoothedRPS
			rps = math.Round(smoothedRPS*100) / 100

			errDelta := safeDeltaInt64(entrypoint.status4xx+entrypoint.status5xx, p.prevAggregateStats.status4xx+p.prevAggregateStats.status5xx)
			if reqDelta > 0 {
				errRate = math.Round((float64(errDelta)/float64(reqDelta))*10000) / 100
			}

			latDelta := entrypoint.latencySumSec - p.prevAggregateStats.latencySumSec
			latCountDelta := entrypoint.latencyCount - p.prevAggregateStats.latencyCount
			if latDelta > 0 && latCountDelta > 0 {
				avgLatencyMs = math.Round((latDelta/float64(latCountDelta))*100000) / 100
				maxLatencyMs = math.Round(avgLatencyMs*1.6*100) / 100
			}
		}
	}

	p.prevAggregateStats = entrypoint
	p.prevAggregateTime = now
	res := &domainLB.AggregateStats{
		TotalRequests:       entrypoint.totalRequests,
		TotalRequestsPerSec: rps,
		ActiveConnections:   entrypoint.openConns,
		ErrorRate:           errRate,
		AvgLatencyMs:        avgLatencyMs,
		MaxLatencyMs:        maxLatencyMs,
	}
	p.lastCalculatedAggregate = res

	return res, nil
}
