package loadbalancer

import (
	"context"
	"net/http"
	"net/http/httptest"
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
