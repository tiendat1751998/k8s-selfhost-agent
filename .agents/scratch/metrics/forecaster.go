package metrics

import (
	"math"
	"time"
)

// ForecastStatus represents capacity warning state.
type ForecastStatus string

const (
	ForecastStatusHealthy  ForecastStatus = "healthy"
	ForecastStatusWarning  ForecastStatus = "warning"
	ForecastStatusCritical ForecastStatus = "critical"
)

// ResourceForecast contains predictive utilization checkpoints and exhaustion estimates.
type ResourceForecast struct {
	ResourceType string     `json:"resource_type"`
	CurrentUsage float64    `json:"current_usage"`
	Forecast7d   float64    `json:"forecast_7d"`
	Forecast30d  float64    `json:"forecast_30d"`
	Forecast90d  float64    `json:"forecast_90d"`
	ExhaustionAt *time.Time `json:"exhaustion_at,omitempty"`
	Status       string     `json:"status"`
	ForecastedAt time.Time  `json:"forecasted_at"`
}

// NodeCapacityForecast represents per-node predictive capacity metrics.
type NodeCapacityForecast struct {
	NodeID   string           `json:"node_id"`
	NodeName string           `json:"node_name"`
	CPU      ResourceForecast `json:"cpu"`
	Memory   ResourceForecast `json:"memory"`
	Disk     ResourceForecast `json:"disk"`
}

// SystemCapacityForecast aggregates platform-level capacity forecasting.
type SystemCapacityForecast struct {
	CPU         ResourceForecast       `json:"cpu"`
	Memory      ResourceForecast       `json:"memory"`
	Disk        ResourceForecast       `json:"disk"`
	PerNode     []NodeCapacityForecast `json:"per_node"`
	GeneratedAt time.Time              `json:"generated_at"`
}

// LinearRegression computes ordinary least squares linear regression (slope and intercept).
func LinearRegression(data []float64) (slope float64, intercept float64) {
	n := float64(len(data))
	if n < 2 {
		if len(data) == 1 {
			return 0, data[0]
		}
		return 0, 0
	}

	var sumX, sumY, sumXY, sumXX float64
	for i, y := range data {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumXX += x * x
	}

	denominator := (n * sumXX) - (sumX * sumX)
	if math.Abs(denominator) < 1e-9 {
		return 0, sumY / n
	}

	slope = ((n * sumXY) - (sumX * sumY)) / denominator
	intercept = (sumY - (slope * sumX)) / n
	return slope, intercept
}

// ForecastLinearTrend projects future metric value using linear regression.
func ForecastLinearTrend(history []float64, stepsAhead int) float64 {
	if len(history) == 0 {
		return 0.0
	}
	if len(history) == 1 {
		return history[0]
	}
	slope, intercept := LinearRegression(history)
	targetStep := float64(len(history) - 1 + stepsAhead)
	projected := intercept + (slope * targetStep)
	if projected < 0 {
		return 0.0
	}
	if projected > 100.0 {
		return 100.0
	}
	return math.Round(projected*100) / 100
}
// ComputeCapacityHeadroom returns the remaining percentage headroom before resource limit.
func ComputeCapacityHeadroom(currUsage, maxLimit float64) float64 {
	if maxLimit <= 0 {
		maxLimit = 100.0
	}
	headroom := maxLimit - currUsage
	if headroom < 0 {
		return 0.0
	}
	return math.Round(headroom*100) / 100
}

// CalculateTimeToExhaustion computes duration until a resource hits exhaustion limit under a daily delta rate.
func CalculateTimeToExhaustion(currUsage, dailyRate, maxLimit float64) (time.Duration, bool) {
	if dailyRate <= 0 || currUsage >= maxLimit {
		return 0, false
	}
	days := (maxLimit - currUsage) / dailyRate
	if days <= 0 || days > 365.0 {
		return 0, false
	}
	return time.Duration(days * 24.0 * float64(time.Hour)), true
}

// ComputeResourceProjections projects future resource utilization and determines capacity warning status.
func ComputeResourceProjections(resourceType string, currUsage, dailyDelta float64, now time.Time) ResourceForecast {
	fc7d := math.Min(100.0, math.Round((currUsage+dailyDelta*7.0)*10.0)/10.0)
	fc30d := math.Min(100.0, math.Round((currUsage+dailyDelta*30.0)*10.0)/10.0)
	fc90d := math.Min(100.0, math.Round((currUsage+dailyDelta*90.0)*10.0)/10.0)

	var exhaustAt *time.Time
	if duration, ok := CalculateTimeToExhaustion(currUsage, dailyDelta, 100.0); ok {
		t := now.Add(duration)
		exhaustAt = &t
	}

	effectiveUsage := math.Max(currUsage, fc30d)
	status := string(ForecastStatusHealthy)
	if effectiveUsage >= 85.0 {
		status = string(ForecastStatusCritical)
	} else if effectiveUsage >= 70.0 {
		status = string(ForecastStatusWarning)
	}

	return ResourceForecast{
		ResourceType: resourceType,
		CurrentUsage: math.Round(currUsage*10.0) / 10.0,
		Forecast7d:   fc7d,
		Forecast30d:  fc30d,
		Forecast90d:  fc90d,
		ExhaustionAt: exhaustAt,
		Status:       status,
		ForecastedAt: now,
	}
}

// ResourceForecaster calculates predictive capacity forecasts from real-time snapshots.
type ResourceForecaster struct{}

// NewResourceForecaster creates a new resource capacity forecaster.
func NewResourceForecaster() *ResourceForecaster {
	return &ResourceForecaster{}
}

// ForecastNode calculates capacity projections for an individual node.
func (f *ResourceForecaster) ForecastNode(node *NodeMetrics) NodeCapacityForecast {
	if node == nil {
		return NodeCapacityForecast{}
	}
	now := time.Now().UTC()

	dailyDeltaCPU := math.Max(0.05, node.CPUPercent*0.005)
	dailyDeltaMem := math.Max(0.04, node.MemoryPercent*0.004)
	dailyDeltaDisk := math.Max(0.03, node.DiskPercent*0.003)

	return NodeCapacityForecast{
		NodeID:   node.NodeID,
		NodeName: node.NodeName,
		CPU:      ComputeResourceProjections("cpu", node.CPUPercent, dailyDeltaCPU, now),
		Memory:   ComputeResourceProjections("memory", node.MemoryPercent, dailyDeltaMem, now),
		Disk:     ComputeResourceProjections("disk", node.DiskPercent, dailyDeltaDisk, now),
	}
}

// ForecastSystem calculates platform-wide capacity forecasts and per-node breakdowns.
func (f *ResourceForecaster) ForecastSystem(overview *SystemOverview) *SystemCapacityForecast {
	if overview == nil {
		return nil
	}
	now := time.Now().UTC()

	dailyDeltaCPU := math.Max(0.05, overview.TotalCPUPercent*0.005)
	dailyDeltaMem := math.Max(0.04, overview.TotalMemPercent*0.004)
	dailyDeltaDisk := math.Max(0.03, overview.TotalDiskPercent*0.003)

	perNode := make([]NodeCapacityForecast, 0, len(overview.Nodes))
	for i := range overview.Nodes {
		perNode = append(perNode, f.ForecastNode(&overview.Nodes[i]))
	}

	return &SystemCapacityForecast{
		CPU:         ComputeResourceProjections("cpu", overview.TotalCPUPercent, dailyDeltaCPU, now),
		Memory:      ComputeResourceProjections("memory", overview.TotalMemPercent, dailyDeltaMem, now),
		Disk:        ComputeResourceProjections("disk", overview.TotalDiskPercent, dailyDeltaDisk, now),
		PerNode:     perNode,
		GeneratedAt: now,
	}
}
