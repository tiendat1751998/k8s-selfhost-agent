package loadbalancer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestCleanServiceName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"gateway@file", "gateway"},
		{"web@docker", "web"},
		{"api@internal", "api"},
		{"dashboard@internal", "dashboard"},
		{"myapp@kubernetes", "myapp"},
		{"plain-service", "plain-service"},
	}

	for _, tt := range tests {
		got := CleanServiceName(tt.input)
		if got != tt.want {
			t.Errorf("CleanServiceName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTraefikProvider_HealthCheck(t *testing.T) {
	t.Run("success overview", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/overview" {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"http":{"routers":{"total":1}}}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer ts.Close()

		provider := NewTraefikProvider(ts.URL, WithHTTPClient(ts.Client()))
		if err := provider.HealthCheck(context.Background()); err != nil {
			t.Fatalf("unexpected healthcheck error: %v", err)
		}
		if provider.Name() != "traefik" {
			t.Errorf("expected provider name 'traefik', got %q", provider.Name())
		}
	})

	t.Run("server down", func(t *testing.T) {
		provider := NewTraefikProvider("http://127.0.0.1:59999", WithHTTPClient(&http.Client{Timeout: 100 * time.Millisecond}))
		if err := provider.HealthCheck(context.Background()); err == nil {
			t.Fatal("expected healthcheck error for unreachable host, got nil")
		}
	})
}

func TestTraefikProvider_GetServiceStats(t *testing.T) {
	servicesJSON := `[
		{
			"name": "gateway@file",
			"provider": "file",
			"type": "loadbalancer",
			"status": "enabled",
			"serverStatus": {"http://tiki_gateway:3579": "UP"}
		},
		{
			"name": "web@file",
			"provider": "file",
			"type": "loadbalancer",
			"status": "enabled",
			"serverStatus": {"http://tiki_web:3000": "UP"}
		}
	]`

	promMetrics1 := strings.Join([]string{
		`# HELP traefik_service_requests_total How many HTTP requests processed on a service, partitioned by status code, protocol, and method.`,
		`# TYPE traefik_service_requests_total counter`,
		`traefik_service_requests_total{code="200",method="GET",protocol="http",service="gateway@file"} 100`,
		`traefik_service_requests_total{code="500",method="POST",protocol="http",service="gateway@file"} 5`,
		`traefik_service_requests_total{code="200",method="GET",protocol="http",service="web@file"} 50`,
		`traefik_service_request_duration_seconds_sum{code="200",service="gateway@file"} 2.1`,
		`traefik_service_request_duration_seconds_count{code="200",service="gateway@file"} 105`,
	}, "\n")

	promMetrics2 := strings.Join([]string{
		`traefik_service_requests_total{code="200",method="GET",protocol="http",service="gateway@file"} 120`,
		`traefik_service_requests_total{code="500",method="POST",protocol="http",service="gateway@file"} 10`,
		`traefik_service_requests_total{code="200",method="GET",protocol="http",service="web@file"} 80`,
		`traefik_service_request_duration_seconds_sum{code="200",service="gateway@file"} 2.7`,
		`traefik_service_request_duration_seconds_count{code="200",service="gateway@file"} 130`,
	}, "\n")

	iteration := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/http/services":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(servicesJSON))
		case "/metrics":
			w.Header().Set("Content-Type", "text/plain")
			if iteration == 0 {
				w.Write([]byte(promMetrics1))
			} else {
				w.Write([]byte(promMetrics2))
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	provider := NewTraefikProvider(ts.URL, WithHTTPClient(ts.Client()))

	// First collection
	stats1, err := provider.GetServiceStats(context.Background())
	if err != nil {
		t.Fatalf("first GetServiceStats failed: %v", err)
	}

	if len(stats1) != 2 {
		t.Fatalf("expected 2 services, got %d", len(stats1))
	}

	// Gateway should have total = 105
	var gwStat, webStat ServiceRequestStatsTest
	for _, s := range stats1 {
		if s.ServiceName == "gateway" {
			gwStat = ServiceRequestStatsTest{s.ServiceName, s.TotalRequests, s.RequestsPerSec, s.Status2xx, s.Status5xx, s.ErrorRate}
		}
		if s.ServiceName == "web" {
			webStat = ServiceRequestStatsTest{s.ServiceName, s.TotalRequests, s.RequestsPerSec, s.Status2xx, s.Status5xx, s.ErrorRate}
		}
	}

	if gwStat.TotalRequests != 105 {
		t.Errorf("gateway TotalRequests = %d, want 105", gwStat.TotalRequests)
	}
	if gwStat.Status2xx != 100 || gwStat.Status5xx != 5 {
		t.Errorf("gateway status codes: 2xx=%d 5xx=%d, want 100/5", gwStat.Status2xx, gwStat.Status5xx)
	}
	if webStat.TotalRequests != 50 {
		t.Errorf("web TotalRequests = %d, want 50", webStat.TotalRequests)
	}

	// Advance iteration and simulate elapsed time
	iteration = 1
	provider.InvalidateMetricsCache()
	provider.mu.Lock()
	provider.prevTime = time.Now().UTC().Add(-2 * time.Second)
	provider.mu.Unlock()

	// Second collection
	stats2, err := provider.GetServiceStats(context.Background())
	if err != nil {
		t.Fatalf("second GetServiceStats failed: %v", err)
	}

	for _, s := range stats2 {
		if s.ServiceName == "gateway" {
			// Delta reqs: 130 - 105 = 25 reqs in ~2s -> ~12.5 RPS
			if s.RequestsPerSec <= 0 {
				t.Errorf("expected gateway RequestsPerSec > 0, got %f", s.RequestsPerSec)
			}
			// Delta error: 10 - 5 = 5 errors out of 25 delta reqs -> 20.0%
			if s.ErrorRate <= 0 {
				t.Errorf("expected gateway ErrorRate > 0, got %f", s.ErrorRate)
			}
		}
	}
}

type ServiceRequestStatsTest struct {
	ServiceName    string
	TotalRequests  int64
	RequestsPerSec float64
	Status2xx      int64
	Status5xx      int64
	ErrorRate      float64
}

func TestTraefikProvider_GetAggregateStats(t *testing.T) {
	promMetrics1 := strings.Join([]string{
		`# HELP traefik_entrypoint_requests_total How many HTTP requests processed on an entrypoint, partitioned by status code, protocol, and method.`,
		`# TYPE traefik_entrypoint_requests_total counter`,
		`traefik_entrypoint_requests_total{code="200",entrypoint="web",method="GET",protocol="http"} 1000`,
		`traefik_entrypoint_requests_total{code="404",entrypoint="web",method="GET",protocol="http"} 20`,
		`traefik_entrypoint_requests_total{code="503",entrypoint="web",method="GET",protocol="http"} 80`,
		`traefik_entrypoint_requests_total{code="200",entrypoint="traefik",method="GET",protocol="http"} 500`,
		`traefik_open_connections{entrypoint="web",protocol="TCP"} 15`,
		`traefik_open_connections{entrypoint="traefik",protocol="TCP"} 2`,
		`traefik_entrypoint_request_duration_seconds_sum{entrypoint="web"} 5.5`,
		`traefik_entrypoint_request_duration_seconds_count{entrypoint="web"} 1100`,
	}, "\n")

	promMetrics2 := strings.Join([]string{
		`traefik_entrypoint_requests_total{code="200",entrypoint="web",method="GET",protocol="http"} 2000`,
		`traefik_entrypoint_requests_total{code="404",entrypoint="web",method="GET",protocol="http"} 30`,
		`traefik_entrypoint_requests_total{code="503",entrypoint="web",method="GET",protocol="http"} 170`,
		`traefik_entrypoint_requests_total{code="200",entrypoint="traefik",method="GET",protocol="http"} 600`,
		`traefik_open_connections{entrypoint="web",protocol="TCP"} 25`,
		`traefik_open_connections{entrypoint="traefik",protocol="TCP"} 2`,
		`traefik_entrypoint_request_duration_seconds_sum{entrypoint="web"} 11.0`,
		`traefik_entrypoint_request_duration_seconds_count{entrypoint="web"} 2200`,
	}, "\n")

	iteration := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			w.Header().Set("Content-Type", "text/plain")
			if iteration == 0 {
				w.Write([]byte(promMetrics1))
			} else {
				w.Write([]byte(promMetrics2))
			}
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	provider := NewTraefikProvider(ts.URL, WithHTTPClient(ts.Client()))

	// First collection (initializes baseline)
	agg1, err := provider.GetAggregateStats(context.Background())
	if err != nil {
		t.Fatalf("first GetAggregateStats failed: %v", err)
	}

	// Entrypoint "web" total = 1000 + 20 + 80 = 1100 (ignoring "traefik" entrypoint 500)
	if agg1.TotalRequests != 1100 {
		t.Errorf("expected TotalRequests = 1100, got %d", agg1.TotalRequests)
	}
	// Active connections should be 15 (ignoring "traefik" entrypoint 2)
	if agg1.ActiveConnections != 15 {
		t.Errorf("expected ActiveConnections = 15, got %d", agg1.ActiveConnections)
	}
	if agg1.TotalRequestsPerSec != 0 {
		t.Errorf("expected initial TotalRequestsPerSec = 0, got %f", agg1.TotalRequestsPerSec)
	}

	// Advance iteration and simulate 2 seconds elapsed
	iteration = 1
	provider.InvalidateMetricsCache()
	provider.mu.Lock()
	provider.prevAggregateTime = time.Now().UTC().Add(-2 * time.Second)
	provider.mu.Unlock()

	// Second collection
	agg2, err := provider.GetAggregateStats(context.Background())
	if err != nil {
		t.Fatalf("second GetAggregateStats failed: %v", err)
	}

	// Total requests: 2000 + 30 + 170 = 2200
	if agg2.TotalRequests != 2200 {
		t.Errorf("expected TotalRequests = 2200, got %d", agg2.TotalRequests)
	}
	// Delta requests: 2200 - 1100 = 1100 in 2s -> ~550 RPS
	if agg2.TotalRequestsPerSec < 500 || agg2.TotalRequestsPerSec > 600 {
		t.Errorf("expected TotalRequestsPerSec ~ 550, got %f", agg2.TotalRequestsPerSec)
	}
	// Active connections = 25
	if agg2.ActiveConnections != 25 {
		t.Errorf("expected ActiveConnections = 25, got %d", agg2.ActiveConnections)
	}
	// Delta errors: (30+170) - (20+80) = 200 - 100 = 100 errors out of 1100 delta reqs -> ~9.09%
	if agg2.ErrorRate < 9.0 || agg2.ErrorRate > 9.2 {
		t.Errorf("expected ErrorRate ~ 9.09%%, got %f", agg2.ErrorRate)
	}
	// Delta latency: (11.0 - 5.5) / (2200 - 1100) = 5.5 / 1100 = 0.005s = 5ms
	if agg2.AvgLatencyMs < 4.9 || agg2.AvgLatencyMs > 5.1 {
		t.Errorf("expected AvgLatencyMs ~ 5.0ms, got %f", agg2.AvgLatencyMs)
	}
}

func TestTraefikProvider_EMASmoothing(t *testing.T) {
	currentTotal := 1000
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			w.Header().Set("Content-Type", "text/plain")
			line := strings.Join([]string{
				`traefik_entrypoint_requests_total{code="200",entrypoint="web",method="GET",protocol="http"} ` + strconv.Itoa(currentTotal),
				`traefik_open_connections{entrypoint="web",protocol="TCP"} 5`,
			}, "\n")
			w.Write([]byte(line))
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	provider := NewTraefikProvider(ts.URL, WithHTTPClient(ts.Client()))

	// Cycle 1: Baseline
	agg, err := provider.GetAggregateStats(context.Background())
	if err != nil || agg.TotalRequestsPerSec != 0 {
		t.Fatalf("expected initial 0 RPS, got %f, err: %v", agg.TotalRequestsPerSec, err)
	}

	// Cycle 2: +100 reqs in 1s -> rawRPS = 100 -> initial smoothed = 100
	currentTotal += 100
	provider.InvalidateMetricsCache()
	provider.mu.Lock()
	provider.prevAggregateTime = time.Now().UTC().Add(-1 * time.Second)
	provider.mu.Unlock()

	agg, err = provider.GetAggregateStats(context.Background())
	if err != nil || agg.TotalRequestsPerSec != 100 {
		t.Fatalf("expected initial smoothed RPS = 100, got %f", agg.TotalRequestsPerSec)
	}

	// Cycle 3: +200 reqs in 1s -> rawRPS = 200 -> smoothed = 0.6*200 + 0.4*100 = 120 + 40 = 160
	currentTotal += 200
	provider.InvalidateMetricsCache()
	provider.mu.Lock()
	provider.prevAggregateTime = time.Now().UTC().Add(-1 * time.Second)
	provider.mu.Unlock()

	agg, err = provider.GetAggregateStats(context.Background())
	if err != nil || agg.TotalRequestsPerSec != 160 {
		t.Fatalf("expected smoothed RPS = 160, got %f", agg.TotalRequestsPerSec)
	}

	// Cycle 4: 0 reqs in 1s -> 1st idle cycle -> smoothed = 0.4 * 160 = 64
	provider.InvalidateMetricsCache()
	provider.mu.Lock()
	provider.prevAggregateTime = time.Now().UTC().Add(-1 * time.Second)
	provider.mu.Unlock()

	agg, err = provider.GetAggregateStats(context.Background())
	if err != nil || agg.TotalRequestsPerSec != 64 {
		t.Fatalf("expected 1st idle cycle smoothed RPS = 64, got %f", agg.TotalRequestsPerSec)
	}

	// Cycle 5: 0 reqs in 1s -> 2nd consecutive idle cycle -> smoothed = 0.4 * 64 = 25.6
	provider.InvalidateMetricsCache()
	provider.mu.Lock()
	provider.prevAggregateTime = time.Now().UTC().Add(-1 * time.Second)
	provider.mu.Unlock()

	agg, err = provider.GetAggregateStats(context.Background())
	if err != nil || agg.TotalRequestsPerSec != 25.6 {
		t.Fatalf("expected 2nd idle cycle smoothed RPS = 25.6, got %f", agg.TotalRequestsPerSec)
	}
}

func TestTraefikProvider_MetricsCycleCaching(t *testing.T) {
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			requestCount++
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte(`traefik_entrypoint_requests_total{code="200",entrypoint="web",method="GET",protocol="http"} 500`))
			return
		}
		if r.URL.Path == "/api/http/services" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`[]`))
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	provider := NewTraefikProvider(ts.URL, WithHTTPClient(ts.Client()))

	// First call to GetAggregateStats: fetches /metrics
	_, err := provider.GetAggregateStats(context.Background())
	if err != nil {
		t.Fatalf("GetAggregateStats failed: %v", err)
	}

	// Immediate call to GetServiceStats: should use cached metrics snapshot, requestCount still 1
	_, err = provider.GetServiceStats(context.Background())
	if err != nil {
		t.Fatalf("GetServiceStats failed: %v", err)
	}

	if requestCount != 1 {
		t.Fatalf("expected 1 scrape to /metrics within cycle cache window, got %d", requestCount)
	}
}
