// Package loadbalancer provides load balancer provider implementations.
package loadbalancer

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	domainLB "github.com/datdt/k8sselfhost/internal/domain/loadbalancer"
)

// serviceSample tracks raw counters from previous collection for delta RPS and error calculations.
type serviceSample struct {
	totalRequests int64
	status2xx     int64
	status4xx     int64
	status5xx     int64
	latencySumSec float64
	latencyCount  int64
}

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

const (
	promMetricsCacheTTL = 800 * time.Millisecond
	defaultEMAAlpha     = 0.6
	defaultScrapeTimeout = 10 * time.Second
)

// TraefikProvider implements domainLB.Provider for Traefik edge reverse proxy.
type TraefikProvider struct {
	apiURL       string
	httpClient   *http.Client
	scrapeClient *http.Client

	scrapeInterval time.Duration
	scrapeTimeout  time.Duration

	mu                         sync.Mutex
	scraperStarted             bool
	stopCh                     chan struct{}
	prevStats                  map[string]serviceSample
	prevTime                   time.Time
	prevAggregateStats         entrypointSample
	prevAggregateTime          time.Time
	prevSmoothedRPS            float64
	zeroDeltaCount             int
	lastSuccessfulSample       entrypointSample
	lastSuccessfulServices     map[string]serviceSample
	lastSuccessfulHTTPServices []traefikServiceDTO
	lastScrapeTime             time.Time
	lastScrapeErr              error
	lastCalculatedAggregate    *domainLB.AggregateStats
	lastCalculatedServices     []domainLB.ServiceRequestStats

	// Cached Prometheus metrics to share across GetAggregateStats and GetServiceStats within the same cycle
	cachedPromStats      map[string]serviceSample
	cachedPromEntrypoint entrypointSample
	cachedPromTime       time.Time
	cachedPromErr        error
}

// Option configures TraefikProvider.
type Option func(*TraefikProvider)

// WithHTTPClient sets a custom HTTP client for Traefik API requests.
func WithHTTPClient(client *http.Client) Option {
	return func(p *TraefikProvider) {
		if client != nil {
			p.httpClient = client
			p.scrapeClient = client
		}
	}
}

// WithScrapeInterval sets the background metrics polling interval.
func WithScrapeInterval(d time.Duration) Option {
	return func(p *TraefikProvider) {
		if d > 0 {
			p.scrapeInterval = d
		}
	}
}

// WithScrapeTimeout sets the HTTP timeout for Traefik metrics scraping.
func WithScrapeTimeout(d time.Duration) Option {
	return func(p *TraefikProvider) {
		if d > 0 {
			p.scrapeTimeout = d
		}
	}
}

// NewTraefikProvider creates a new Traefik load balancer provider.
func NewTraefikProvider(apiURL string, opts ...Option) *TraefikProvider {
	trimmed := strings.TrimRight(strings.TrimSpace(apiURL), "/")
	if trimmed == "" {
		trimmed = "http://localhost:8080"
	}

	transport := &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
	}

	p := &TraefikProvider{
		apiURL: trimmed,
		httpClient: &http.Client{
			Timeout:   defaultScrapeTimeout,
			Transport: transport,
		},
		scrapeClient: &http.Client{
			Timeout:   defaultScrapeTimeout,
			Transport: transport,
		},
		scrapeInterval: 4 * time.Second,
		scrapeTimeout:  defaultScrapeTimeout,
		prevStats:      make(map[string]serviceSample),
		stopCh:         make(chan struct{}),
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// Name returns the provider name identifier.
func (p *TraefikProvider) Name() string {
	return "traefik"
}

// StartBackgroundScraper starts a continuous background scraping loop for Traefik metrics and services.
func (p *TraefikProvider) StartBackgroundScraper(ctx context.Context) {
	p.mu.Lock()
	if p.scraperStarted {
		p.mu.Unlock()
		return
	}
	p.scraperStarted = true
	stopCh := p.stopCh
	interval := p.scrapeInterval
	if interval <= 0 {
		interval = 4 * time.Second
	}
	p.mu.Unlock()

	// Initial immediate scrape
	p.scrapeOnce(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-stopCh:
			return
		case <-ticker.C:
			p.scrapeOnce(ctx)
		}
	}
}

// StopBackgroundScraper stops the background scraping loop.
func (p *TraefikProvider) StopBackgroundScraper() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.scraperStarted {
		select {
		case <-p.stopCh:
		default:
			close(p.stopCh)
		}
		p.scraperStarted = false
		p.stopCh = make(chan struct{})
	}
}

// scrapeOnce executes a single scrape iteration of Prometheus metrics and HTTP services.
func (p *TraefikProvider) scrapeOnce(ctx context.Context) {
	timeout := p.scrapeTimeout
	if timeout <= 0 {
		timeout = defaultScrapeTimeout
	}
	scrapeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 1. Scrape Prometheus metrics
	promStats, entrypoint, promErr := p.fetchPrometheusMetricsWithClient(scrapeCtx, p.scrapeClient)

	// 2. Scrape HTTP services
	services, srvErr := p.fetchHTTPServicesWithClient(scrapeCtx, p.scrapeClient)

	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now().UTC()
	if promErr == nil {
		p.lastSuccessfulSample = entrypoint
		p.lastSuccessfulServices = promStats
		p.lastScrapeTime = now
		p.lastScrapeErr = nil

		p.cachedPromStats = promStats
		p.cachedPromEntrypoint = entrypoint
		p.cachedPromTime = now
		p.cachedPromErr = nil
	} else {
		p.lastScrapeErr = promErr
		p.cachedPromErr = promErr
	}

	if srvErr == nil {
		p.lastSuccessfulHTTPServices = services
	}
}

// HealthCheck verifies connectivity to Traefik API.
func (p *TraefikProvider) HealthCheck(ctx context.Context) error {
	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	url := p.apiURL + "/api/overview"
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating traefik healthcheck request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		// Try fallback to /api/http/services
		srvReq, srvErr := http.NewRequestWithContext(reqCtx, http.MethodGet, p.apiURL+"/api/http/services", nil)
		if srvErr != nil {
			return fmt.Errorf("traefik healthcheck unreachable at %s: %w", p.apiURL, err)
		}
		srvResp, srvDoErr := p.httpClient.Do(srvReq)
		if srvDoErr != nil {
			return fmt.Errorf("traefik healthcheck unreachable at %s: %w", p.apiURL, srvDoErr)
		}
		defer srvResp.Body.Close()
		if srvResp.StatusCode != http.StatusOK {
			return fmt.Errorf("traefik services endpoint returned HTTP %d", srvResp.StatusCode)
		}
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("traefik healthcheck returned HTTP %d", resp.StatusCode)
	}

	return nil
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

// getPrometheusMetrics returns cached Prometheus metrics if recent (< promMetricsCacheTTL or background scrape),
// or fetches fresh metrics from Traefik /metrics endpoint.
func (p *TraefikProvider) getPrometheusMetrics(ctx context.Context) (map[string]serviceSample, entrypointSample, error) {
	p.mu.Lock()
	if !p.cachedPromTime.IsZero() && time.Since(p.cachedPromTime) < promMetricsCacheTTL && p.cachedPromErr == nil {
		stats := p.cachedPromStats
		ep := p.cachedPromEntrypoint
		p.mu.Unlock()
		return stats, ep, nil
	}
	if p.scraperStarted {
		if !p.lastScrapeTime.IsZero() {
			stats := p.lastSuccessfulServices
			ep := p.lastSuccessfulSample
			p.mu.Unlock()
			return stats, ep, nil
		}
		if p.lastScrapeErr != nil {
			err := p.lastScrapeErr
			p.mu.Unlock()
			return nil, entrypointSample{}, err
		}
	}
	p.mu.Unlock()

	stats, ep, err := p.fetchPrometheusMetrics(ctx)

	p.mu.Lock()
	if err == nil {
		p.cachedPromStats = stats
		p.cachedPromEntrypoint = ep
		p.cachedPromTime = time.Now().UTC()
		p.cachedPromErr = nil
		p.lastSuccessfulSample = ep
		p.lastSuccessfulServices = stats
		p.lastScrapeTime = p.cachedPromTime
		p.lastScrapeErr = nil
	} else {
		p.cachedPromErr = err
	}
	p.mu.Unlock()

	return stats, ep, err
}

// InvalidateMetricsCache clears any cached Prometheus metrics snapshot.
func (p *TraefikProvider) InvalidateMetricsCache() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cachedPromTime = time.Time{}
	p.cachedPromStats = nil
	p.cachedPromEntrypoint = entrypointSample{}
	p.cachedPromErr = nil
	p.lastScrapeTime = time.Time{}
	p.lastSuccessfulSample = entrypointSample{}
	p.lastSuccessfulServices = nil
	p.lastScrapeErr = nil
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

// fetchPrometheusMetrics attempts to parse prometheus metrics from /metrics endpoint if present.
func (p *TraefikProvider) fetchPrometheusMetrics(ctx context.Context) (map[string]serviceSample, entrypointSample, error) {
	return p.fetchPrometheusMetricsWithClient(ctx, p.httpClient)
}

func (p *TraefikProvider) fetchPrometheusMetricsWithClient(ctx context.Context, client *http.Client) (map[string]serviceSample, entrypointSample, error) {
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

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, p.apiURL+"/metrics", nil)
	if err != nil {
		return nil, entrypointSample{}, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, entrypointSample{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, entrypointSample{}, fmt.Errorf("metrics returned status %d", resp.StatusCode)
	}

	return parsePrometheusMetrics(resp.Body)
}

// parsePrometheusMetrics parses Prometheus plain text format to extract Traefik per-service and entrypoint counters.
func parsePrometheusMetrics(r io.Reader) (map[string]serviceSample, entrypointSample, error) {
	stats := make(map[string]serviceSample)
	var entrypoint entrypointSample
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Service requests: traefik_service_requests_total{code="200",method="GET",protocol="http",service="gateway@file"} 142
		if strings.HasPrefix(line, "traefik_service_requests_total") {
			labels, val := parsePrometheusMetricLineData(line)
			service := labels["service"]
			if service != "" {
				code := parseCode(labels["code"])
				s := stats[service]
				s.totalRequests += int64(val)
				if code >= 200 && code < 300 {
					s.status2xx += int64(val)
				} else if code >= 400 && code < 500 {
					s.status4xx += int64(val)
				} else if code >= 500 && code < 600 {
					s.status5xx += int64(val)
				}
				stats[service] = s
			}
		}

		// Service latency sum: traefik_service_request_duration_seconds_sum{code="200",service="gateway@file"} 1.452
		if strings.HasPrefix(line, "traefik_service_request_duration_seconds_sum") {
			labels, val := parsePrometheusMetricLineData(line)
			service := labels["service"]
			if service != "" {
				s := stats[service]
				s.latencySumSec += val
				stats[service] = s
			}
		}

		// Service latency count: traefik_service_request_duration_seconds_count{code="200",service="gateway@file"} 142
		if strings.HasPrefix(line, "traefik_service_request_duration_seconds_count") {
			labels, val := parsePrometheusMetricLineData(line)
			service := labels["service"]
			if service != "" {
				s := stats[service]
				s.latencyCount += int64(val)
				stats[service] = s
			}
		}

		// Entrypoint requests: traefik_entrypoint_requests_total{code="200",entrypoint="web",method="GET",protocol="http"} 24324
		if strings.HasPrefix(line, "traefik_entrypoint_requests_total") {
			labels, val := parsePrometheusMetricLineData(line)
			ep := labels["entrypoint"]
			if !isInternalEntrypoint(ep) {
				code := parseCode(labels["code"])
				entrypoint.totalRequests += int64(val)
				if code >= 200 && code < 300 {
					entrypoint.status2xx += int64(val)
				} else if code >= 400 && code < 500 {
					entrypoint.status4xx += int64(val)
				} else if code >= 500 && code < 600 {
					entrypoint.status5xx += int64(val)
				}
			}
		}

		// Entrypoint latency sum: traefik_entrypoint_request_duration_seconds_sum{entrypoint="web"} 12.5
		if strings.HasPrefix(line, "traefik_entrypoint_request_duration_seconds_sum") {
			labels, val := parsePrometheusMetricLineData(line)
			ep := labels["entrypoint"]
			if !isInternalEntrypoint(ep) {
				entrypoint.latencySumSec += val
			}
		}

		// Entrypoint latency count: traefik_entrypoint_request_duration_seconds_count{entrypoint="web"} 24324
		if strings.HasPrefix(line, "traefik_entrypoint_request_duration_seconds_count") {
			labels, val := parsePrometheusMetricLineData(line)
			ep := labels["entrypoint"]
			if !isInternalEntrypoint(ep) {
				entrypoint.latencyCount += int64(val)
			}
		}

		// Active open connections: traefik_open_connections{entrypoint="web",protocol="TCP"} 5 or traefik_entrypoint_open_connections{entrypoint="web"} 5
		if strings.HasPrefix(line, "traefik_open_connections") || strings.HasPrefix(line, "traefik_entrypoint_open_connections") {
			labels, val := parsePrometheusMetricLineData(line)
			ep := labels["entrypoint"]
			if !isInternalEntrypoint(ep) {
				entrypoint.openConns += int(val)
			}
		}
	}

	return stats, entrypoint, scanner.Err()
}

// parsePrometheusMetricLineData extracts labels map and floating point metric value.
func parsePrometheusMetricLineData(line string) (map[string]string, float64) {
	openIdx := strings.Index(line, "{")
	closeIdx := strings.LastIndex(line, "}")
	if openIdx != -1 && closeIdx != -1 && closeIdx > openIdx {
		labelsStr := line[openIdx+1 : closeIdx]
		valStr := strings.TrimSpace(line[closeIdx+1:])
		v, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			return nil, 0
		}
		return parseLabels(labelsStr), v
	}

	parts := strings.Fields(line)
	if len(parts) >= 2 {
		v, err := strconv.ParseFloat(parts[1], 64)
		if err == nil {
			return map[string]string{}, v
		}
	}
	return nil, 0
}

func parseCode(codeStr string) int {
	if codeStr == "" {
		return 0
	}
	c, _ := strconv.Atoi(codeStr)
	return c
}

func isInternalEntrypoint(ep string) bool {
	ep = strings.ToLower(strings.TrimSpace(ep))
	return ep == "traefik" || ep == "dashboard" || ep == "internal" || ep == "metrics"
}

// parseLabels parses comma-separated key="value" pairs.
func parseLabels(labelsStr string) map[string]string {
	result := make(map[string]string)
	pairs := strings.Split(labelsStr, ",")
	for _, p := range pairs {
		p = strings.TrimSpace(p)
		if eqIdx := strings.Index(p, "="); eqIdx != -1 {
			k := strings.TrimSpace(p[:eqIdx])
			v := strings.Trim(strings.TrimSpace(p[eqIdx+1:]), `"`)
			result[k] = v
		}
	}
	return result
}

// CleanServiceName strips provider suffixes like @file, @docker, @internal, @kubernetes.
func CleanServiceName(name string) string {
	name = strings.TrimSpace(name)
	if idx := strings.Index(name, "@"); idx != -1 {
		name = name[:idx]
	}
	return name
}
