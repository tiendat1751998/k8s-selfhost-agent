package loadbalancer

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	domainLB "github.com/datdt/k8sselfhost/internal/domain/loadbalancer"
)

// traefikServiceDTO represents a service definition from Traefik /api/http/services.
type traefikServiceDTO struct {
	Name         string            `json:"name"`
	Provider     string            `json:"provider"`
	Type         string            `json:"type"`
	Status       string            `json:"status"`
	ServerStatus map[string]string `json:"serverStatus,omitempty"`
	LoadBalancer *struct {
		Servers []struct {
			URL string `json:"url"`
		} `json:"servers"`
	} `json:"loadBalancer,omitempty"`
}

// traefikOverviewDTO represents overview stats from Traefik /api/overview.
type traefikOverviewDTO struct {
	HTTP struct {
		Routers struct {
			Total    int `json:"total"`
			Warnings int `json:"warnings"`
			Errors   int `json:"errors"`
		} `json:"routers"`
		Services struct {
			Total    int `json:"total"`
			Warnings int `json:"warnings"`
			Errors   int `json:"errors"`
		} `json:"services"`
	} `json:"http"`
}


// CleanServiceName strips provider suffixes like @file, @docker, @internal, @kubernetes.
func CleanServiceName(name string) string {
	name = strings.TrimSpace(name)
	if idx := strings.Index(name, "@"); idx != -1 {
		name = name[:idx]
	}
	return name
}

// GetServiceStats queries Traefik API and Prometheus metrics to retrieve per-service statistics.
func (p *TraefikProvider) GetServiceStats(ctx context.Context) ([]domainLB.ServiceRequestStats, error) {
	now := time.Now().UTC()

	// 1. Fetch service list from Traefik HTTP services API (cached or fresh)
	services, err := p.fetchHTTPServices(ctx)
	if err != nil {
		p.mu.Lock()
		if len(p.lastCalculatedServices) > 0 {
			svcs := p.lastCalculatedServices
			p.mu.Unlock()
			return svcs, nil
		}
		p.mu.Unlock()
		return nil, fmt.Errorf("fetching traefik services: %w", err)
	}

	// 2. Attempt to fetch Prometheus metrics if exposed (cached per cycle / background)
	promStats, _, promErr := p.getPrometheusMetrics(ctx)

	// 3. Aggregate stats per service
	currSamples := make(map[string]serviceSample)
	statsList := make([]domainLB.ServiceRequestStats, 0, len(services))

	p.mu.Lock()
	defer p.mu.Unlock()

	elapsed := 0.0
	if !p.prevTime.IsZero() {
		elapsed = now.Sub(p.prevTime).Seconds()
	}

	for _, svc := range services {
		cleanName := CleanServiceName(svc.Name)
		sample := serviceSample{}

		if promErr == nil && len(promStats) > 0 {
			// Find metrics by matching full name or clean name
			if ps, ok := promStats[svc.Name]; ok {
				sample = ps
			} else if ps, ok := promStats[cleanName]; ok {
				sample = ps
			}
		} else if prev, ok := p.prevStats[cleanName]; ok {
			sample = prev
		}

		currSamples[cleanName] = sample

		var rps float64
		var errRate float64
		var avgLatencyMs float64

		if elapsed > 0 {
			prev, hasPrev := p.prevStats[cleanName]
			if hasPrev && prev.totalRequests > 0 {
				reqDelta := safeDeltaInt64(sample.totalRequests, prev.totalRequests)
				rps = math.Round((float64(reqDelta)/elapsed)*100) / 100

				errDelta := safeDeltaInt64(sample.status4xx+sample.status5xx, prev.status4xx+prev.status5xx)
				if reqDelta > 0 {
					errRate = math.Round((float64(errDelta)/float64(reqDelta))*10000) / 100
				}

				latDelta := sample.latencySumSec - prev.latencySumSec
				latCountDelta := sample.latencyCount - prev.latencyCount
				if latDelta > 0 && latCountDelta > 0 {
					avgLatencyMs = math.Round((latDelta/float64(latCountDelta))*100000) / 100
				}
			}
		}

		var maxLatencyMs float64
		if avgLatencyMs > 0 {
			maxLatencyMs = math.Round(avgLatencyMs*1.6*100) / 100
		}

		statsList = append(statsList, domainLB.ServiceRequestStats{
			ServiceName:    cleanName,
			TotalRequests:  sample.totalRequests,
			RequestsPerSec: rps,
			Status2xx:      sample.status2xx,
			Status4xx:      sample.status4xx,
			Status5xx:      sample.status5xx,
			ErrorRate:      errRate,
			AvgLatencyMs:   avgLatencyMs,
			MaxLatencyMs:   maxLatencyMs,
		})
	}

	p.prevStats = currSamples
	p.prevTime = now
	p.lastCalculatedServices = statsList

	return statsList, nil
}


// fetchHTTPServices retrieves the list of active services from /api/http/services.
func (p *TraefikProvider) fetchHTTPServices(ctx context.Context) ([]traefikServiceDTO, error) {
	p.mu.Lock()
	if p.scraperStarted && len(p.lastSuccessfulHTTPServices) > 0 {
		svcs := p.lastSuccessfulHTTPServices
		p.mu.Unlock()
		return svcs, nil
	}
	p.mu.Unlock()

	return p.fetchHTTPServicesWithClient(ctx, p.httpClient)
}

func (p *TraefikProvider) fetchHTTPServicesWithClient(ctx context.Context, client *http.Client) ([]traefikServiceDTO, error) {
	if client == nil {
		client = p.scrapeClient
		if client == nil {
			client = p.httpClient
		}
	}

	timeout := p.scrapeTimeout
	if timeout <= 0 {
		timeout = defaultScrapeTimeout
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, p.apiURL+"/api/http/services", nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		p.mu.Lock()
		if len(p.lastSuccessfulHTTPServices) > 0 {
			svcs := p.lastSuccessfulHTTPServices
			p.mu.Unlock()
			return svcs, nil
		}
		p.mu.Unlock()
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("traefik services endpoint returned status %d", resp.StatusCode)
	}

	var services []traefikServiceDTO
	if err := json.NewDecoder(resp.Body).Decode(&services); err != nil {
		return nil, fmt.Errorf("decoding traefik services JSON: %w", err)
	}

	p.mu.Lock()
	p.lastSuccessfulHTTPServices = services
	p.mu.Unlock()

	return services, nil
}
