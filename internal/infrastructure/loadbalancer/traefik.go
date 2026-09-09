// Package loadbalancer provides load balancer provider implementations.
package loadbalancer

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	domainLB "github.com/datdt/k8sselfhost/internal/domain/loadbalancer"
)

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
