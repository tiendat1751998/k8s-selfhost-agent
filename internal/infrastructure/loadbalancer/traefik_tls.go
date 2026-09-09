package loadbalancer

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
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
