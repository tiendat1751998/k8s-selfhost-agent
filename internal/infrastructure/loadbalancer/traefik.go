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

// TraefikProvider implements domainLB.Provider for Traefik edge reverse proxy.
type TraefikProvider struct {
	apiURL     string
	httpClient *http.Client

	mu        sync.Mutex
	prevStats map[string]serviceSample
	prevTime  time.Time
}

// Option configures TraefikProvider.
type Option func(*TraefikProvider)

// WithHTTPClient sets a custom HTTP client for Traefik API requests.
func WithHTTPClient(client *http.Client) Option {
	return func(p *TraefikProvider) {
		if client != nil {
			p.httpClient = client
		}
	}
}

// NewTraefikProvider creates a new Traefik load balancer provider.
func NewTraefikProvider(apiURL string, opts ...Option) *TraefikProvider {
	trimmed := strings.TrimRight(strings.TrimSpace(apiURL), "/")
	if trimmed == "" {
		trimmed = "http://localhost:8080"
	}

	p := &TraefikProvider{
		apiURL: trimmed,
		httpClient: &http.Client{
			Timeout: 4 * time.Second,
		},
		prevStats: make(map[string]serviceSample),
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

	// 1. Fetch service list from Traefik HTTP services API
	services, err := p.fetchHTTPServices(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching traefik services: %w", err)
	}

	// 2. Attempt to fetch Prometheus metrics if exposed
	promStats, promErr := p.fetchPrometheusMetrics(ctx)

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
		}

		currSamples[cleanName] = sample

		var rps float64
		var errRate float64
		var avgLatencyMs float64

		if elapsed > 0 {
			prev, hasPrev := p.prevStats[cleanName]
			if hasPrev {
				reqDelta := sample.totalRequests - prev.totalRequests
				if reqDelta < 0 {
					reqDelta = 0
				}
				rps = math.Round((float64(reqDelta)/elapsed)*100) / 100

				errDelta := (sample.status4xx + sample.status5xx) - (prev.status4xx + prev.status5xx)
				if errDelta < 0 {
					errDelta = 0
				}
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

		statsList = append(statsList, domainLB.ServiceRequestStats{
			ServiceName:    cleanName,
			TotalRequests:  sample.totalRequests,
			RequestsPerSec: rps,
			Status2xx:      sample.status2xx,
			Status4xx:      sample.status4xx,
			Status5xx:      sample.status5xx,
			ErrorRate:      errRate,
			AvgLatencyMs:   avgLatencyMs,
		})
	}

	p.prevStats = currSamples
	p.prevTime = now

	return statsList, nil
}

// fetchHTTPServices retrieves the list of active services from /api/http/services.
func (p *TraefikProvider) fetchHTTPServices(ctx context.Context) ([]traefikServiceDTO, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, p.apiURL+"/api/http/services", nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
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

	return services, nil
}

// fetchPrometheusMetrics attempts to parse prometheus metrics from /metrics endpoint if present.
func (p *TraefikProvider) fetchPrometheusMetrics(ctx context.Context) (map[string]serviceSample, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, p.apiURL+"/metrics", nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("metrics returned status %d", resp.StatusCode)
	}

	return parsePrometheusMetrics(resp.Body)
}

// parsePrometheusMetrics parses Prometheus plain text format to extract Traefik per-service counters.
func parsePrometheusMetrics(r io.Reader) (map[string]serviceSample, error) {
	stats := make(map[string]serviceSample)
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse requests total: traefik_service_requests_total{code="200",method="GET",protocol="http",service="gateway@file"} 142
		if strings.HasPrefix(line, "traefik_service_requests_total") {
			service, code, val := parsePrometheusMetricLine(line)
			if service != "" {
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

		// Parse duration sum: traefik_service_request_duration_seconds_sum{code="200",service="gateway@file"} 1.452
		if strings.HasPrefix(line, "traefik_service_request_duration_seconds_sum") {
			service, _, val := parsePrometheusMetricLine(line)
			if service != "" {
				s := stats[service]
				s.latencySumSec += val
				stats[service] = s
			}
		}

		// Parse duration count: traefik_service_request_duration_seconds_count{code="200",service="gateway@file"} 142
		if strings.HasPrefix(line, "traefik_service_request_duration_seconds_count") {
			service, _, val := parsePrometheusMetricLine(line)
			if service != "" {
				s := stats[service]
				s.latencyCount += int64(val)
				stats[service] = s
			}
		}
	}

	return stats, scanner.Err()
}

// parsePrometheusMetricLine extracts service label, code label, and floating point value.
func parsePrometheusMetricLine(line string) (service string, code int, value float64) {
	// Sample line: traefik_service_requests_total{code="200",method="GET",protocol="http",service="gateway@file"} 142
	openIdx := strings.Index(line, "{")
	closeIdx := strings.LastIndex(line, "}")
	if openIdx == -1 || closeIdx == -1 || closeIdx <= openIdx {
		return "", 0, 0
	}

	labelsStr := line[openIdx+1 : closeIdx]
	valStr := strings.TrimSpace(line[closeIdx+1:])

	v, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return "", 0, 0
	}
	value = v

	labels := parseLabels(labelsStr)
	service = labels["service"]
	if cStr, ok := labels["code"]; ok {
		if c, err := strconv.Atoi(cStr); err == nil {
			code = c
		}
	}

	return service, code, value
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
