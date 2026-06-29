package cost

import "time"

// ClusterCost represents cost aggregates for a cluster.
type ClusterCost struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Provider    string    `json:"provider"`
	MonthlyCost int       `json:"monthly_cost"`
	DailyCost   int       `json:"daily_cost"`
	CPUCost     int       `json:"cpu_cost"`
	MemoryCost  int       `json:"memory_cost"`
	StorageCost int       `json:"storage_cost"`
	NetworkCost int       `json:"network_cost"`
	Trend       int       `json:"trend"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// NamespaceCost represents resource cost allocations for a specific namespace.
type NamespaceCost struct {
	ID              string    `json:"id"`
	Namespace       string    `json:"namespace"`
	Cluster         string    `json:"cluster"`
	CPURequested    string    `json:"cpu_requested"`
	MemoryRequested string    `json:"memory_requested"`
	MonthlyCost     int       `json:"monthly_cost"`
	Utilization     int       `json:"utilization"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ResourceWaste represents detected underutilized or orphaned resource waste.
type ResourceWaste struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	Resource   string    `json:"resource"`
	Namespace  string    `json:"namespace"`
	Cluster    string    `json:"cluster"`
	CPUUtil    *int      `json:"cpu_util"`
	MemUtil    *int      `json:"mem_util"`
	WastedCost int       `json:"wasted_cost"`
	Severity   string    `json:"severity"`
	UpdatedAt  time.Time `json:"updated_at"`
}
