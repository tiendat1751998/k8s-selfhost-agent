package loadbalancer

import "context"

// ServiceRequestStats holds per-service request metrics from any LB.
type ServiceRequestStats struct {
	ServiceName    string  `json:"service_name"`
	TotalRequests  int64   `json:"total_requests"`
	RequestsPerSec float64 `json:"requests_per_sec"`
	Status2xx      int64   `json:"status_2xx"`
	Status4xx      int64   `json:"status_4xx"`
	Status5xx      int64   `json:"status_5xx"`
	ErrorRate      float64 `json:"error_rate"` // 0.0-100.0%
	AvgLatencyMs   float64 `json:"avg_latency_ms"`
}

// Provider abstracts any load balancer (Traefik, Nginx, HAProxy, etc.)
type Provider interface {
	GetServiceStats(ctx context.Context) ([]ServiceRequestStats, error)
	HealthCheck(ctx context.Context) error
	Name() string
}
