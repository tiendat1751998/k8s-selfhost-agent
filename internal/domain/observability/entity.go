// Package observability provides domain entities for SLO, traces, and events.
package observability

import "time"

// SLODefinition represents a Service Level Objective definition.
type SLODefinition struct {
	ID             string    `json:"id"`
	Service        string    `json:"service"`
	Target         float64   `json:"target"`
	IndicatorType  string    `json:"indicator_type"` // availability | latency | error_rate | cache_hit_rate
	Window         string    `json:"window"`         // e.g. "7d", "14d", "30d", "90d"
	Query          string    `json:"query,omitempty"`
	AlertThreshold float64   `json:"alert_threshold,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// SLOSnapshot represents a point-in-time SLO compliance reading.
type SLOSnapshot struct {
	ID           string    `json:"id"`
	SLOID        string    `json:"slo_id"`
	Service      string    `json:"service"`
	Target       float64   `json:"target"`
	Actual       float64   `json:"actual"`
	BurnRate     float64   `json:"burn_rate"`
	ErrorBudget  float64   `json:"error_budget"`
	BudgetStatus string    `json:"budget_status"` // healthy | warning | critical
	RecordedAt   time.Time `json:"recorded_at"`
}
