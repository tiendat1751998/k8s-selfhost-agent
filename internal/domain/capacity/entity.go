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
	CurrentUsage float64      `json:"current_usage"` // percentage
	Forecast7d   float64      `json:"forecast_7d"`
	Forecast30d  float64      `json:"forecast_30d"`
	Forecast90d  float64      `json:"forecast_90d"`
	ExhaustionAt *time.Time   `json:"exhaustion_at,omitempty"`
	Status       string       `json:"status"` // healthy | warning | critical
	RecordedAt   time.Time    `json:"recorded_at"`
}


// NodeHeadroom represents computed real-time headroom and capacity for a single compute node.
type NodeHeadroom struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	Role              string  `json:"role"` // "worker" | "control-plane"
	CPUTotalCores     float64 `json:"cpu_total_cores"`
	CPUAllocatedCores float64 `json:"cpu_allocated_cores"`
	CPUUsagePercent   float64 `json:"cpu_usage_percent"`
	MemTotalGiB       float64 `json:"mem_total_gib"`
	MemAllocatedGiB   float64 `json:"mem_allocated_gib"`
	MemUsagePercent   float64 `json:"mem_usage_percent"`
	PodCount          int     `json:"pod_count"`
	PodCapacity       int     `json:"pod_capacity"`
	BinPackingScore   float64 `json:"bin_packing_score"`
	Status            string  `json:"status"` // "healthy" | "warning" | "critical"
	HeadroomPercent   float64 `json:"headroom_percent"`
}
