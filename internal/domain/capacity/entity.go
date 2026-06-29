// Package capacity provides domain entities for capacity planning.
package capacity

import "time"

// ResourceType represents a capacity resource type.
type ResourceType string

const (
	ResourceCPU     ResourceType = "cpu"
	ResourceMemory  ResourceType = "memory"
	ResourceStorage ResourceType = "storage"
)

// Forecast represents a capacity forecast record.
type Forecast struct {
	ID           string       `json:"id"`
	Cluster      string       `json:"cluster"`
	ResourceType ResourceType `json:"resource_type"`
	CurrentUsage float64      `json:"current_usage"`  // percentage
	Forecast7d   float64      `json:"forecast_7d"`
	Forecast30d  float64      `json:"forecast_30d"`
	Forecast90d  float64      `json:"forecast_90d"`
	ExhaustionAt *time.Time   `json:"exhaustion_at,omitempty"`
	Status       string       `json:"status"` // healthy | warning | critical
	RecordedAt   time.Time    `json:"recorded_at"`
}
