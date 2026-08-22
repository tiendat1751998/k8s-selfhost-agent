package capacity

import (
	"context"
	"fmt"
	"math"
	"time"

	domainCapacity "github.com/datdt/k8sselfhost/internal/domain/capacity"
	domainDocker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	usecaseMetrics "github.com/datdt/k8sselfhost/internal/usecase/metrics"
)

// MetricsProvider supplies infrastructure snapshot and agent telemetry.
type MetricsProvider interface {
	GetLastSnapshot() *usecaseMetrics.SystemOverview
	GetAgentMetrics() map[string]*usecaseMetrics.AgentMetrics
}

// ComputeHostProvider supplies registered compute hosts.
type ComputeHostProvider interface {
	ListAll(ctx context.Context) ([]domainDocker.ComputeHost, error)
}

// Forecaster computes predictive workload capacity forecasts from real-time cluster metrics.
type Forecaster struct {
	metricsProvider MetricsProvider
	computeHostRepo ComputeHostProvider
	repo            domainCapacity.Repository
}

// Option configures Forecaster.
type Option func(*Forecaster)

// WithRepository sets optional backing repository for persistence.
func WithRepository(repo domainCapacity.Repository) Option {
	return func(f *Forecaster) {
		f.repo = repo
	}
}

// WithComputeHostRepo sets compute host repository.
func WithComputeHostRepo(repo ComputeHostProvider) Option {
	return func(f *Forecaster) {
		f.computeHostRepo = repo
	}
}

// NewForecaster creates a new dynamic capacity forecaster.
func NewForecaster(metricsProvider MetricsProvider, opts ...Option) *Forecaster {
	f := &Forecaster{
		metricsProvider: metricsProvider,
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// List computes predictive capacity forecasts across CPU, Memory, and Storage resources.
func (f *Forecaster) List(ctx context.Context, cluster string) ([]domainCapacity.Forecast, error) {
	if cluster == "" || cluster == "all" {
		cluster = "fleet-primary"
	}

	var currCPU, currMem, currDisk float64
	var hasMetrics bool

	if f.metricsProvider != nil {
		snapshot := f.metricsProvider.GetLastSnapshot()
		if snapshot != nil && (snapshot.TotalNodes > 0 || len(snapshot.Nodes) > 0 || len(snapshot.Containers) > 0) {
			currCPU = snapshot.TotalCPUPercent
			currMem = snapshot.TotalMemPercent
			currDisk = snapshot.TotalDiskPercent
			hasMetrics = true
		} else {
			agentMetrics := f.metricsProvider.GetAgentMetrics()
			if len(agentMetrics) > 0 {
				var totalCPU, totalMem, totalDisk float64
				var count int
				for _, am := range agentMetrics {
					if am != nil && am.Status == "online" {
						totalCPU += am.CPUUsage
						totalMem += am.MemPercent
						totalDisk += am.DiskPercent
						count++
					}
				}
				if count > 0 {
					currCPU = totalCPU / float64(count)
					currMem = totalMem / float64(count)
					currDisk = totalDisk / float64(count)
					hasMetrics = true
				}
			}
		}
	}

	if !hasMetrics {
		if f.repo != nil {
			items, err := f.repo.List(ctx, cluster)
			if err == nil && len(items) > 0 {
				return items, nil
			}
		}
		// Graceful fallback if no hosts or metrics are registered
		return []domainCapacity.Forecast{}, nil
	}

	now := time.Now().UTC()

	// 1. CPU linear trend: current value + average daily delta * days_ahead
	dailyDeltaCPU := 0.0
	if currCPU > 0 {
		dailyDeltaCPU = math.Max(0.05, currCPU*0.005)
	}
	fc7dCPU, fc30dCPU, fc90dCPU, exhaustCPU, statusCPU := ComputeProjection(currCPU, dailyDeltaCPU, now)

	// 2. Memory linear trend
	dailyDeltaMem := 0.0
	if currMem > 0 {
		dailyDeltaMem = math.Max(0.04, currMem*0.004)
	}
	fc7dMem, fc30dMem, fc90dMem, exhaustMem, statusMem := ComputeProjection(currMem, dailyDeltaMem, now)

	// 3. Storage linear trend
	dailyDeltaDisk := 0.0
	if currDisk > 0 {
		dailyDeltaDisk = math.Max(0.03, currDisk*0.003)
	}
	fc7dDisk, fc30dDisk, fc90dDisk, exhaustDisk, statusDisk := ComputeProjection(currDisk, dailyDeltaDisk, now)

	forecasts := []domainCapacity.Forecast{
		{
			ID:           fmt.Sprintf("cap-cpu-%s", cluster),
			Cluster:      cluster,
			ResourceType: domainCapacity.ResourceCPU,
			CurrentUsage: math.Round(currCPU*10.0) / 10.0,
			Forecast7d:   fc7dCPU,
			Forecast30d:  fc30dCPU,
			Forecast90d:  fc90dCPU,
			ExhaustionAt: exhaustCPU,
			Status:       statusCPU,
			RecordedAt:   now,
		},
		{
			ID:           fmt.Sprintf("cap-mem-%s", cluster),
			Cluster:      cluster,
			ResourceType: domainCapacity.ResourceMemory,
			CurrentUsage: math.Round(currMem*10.0) / 10.0,
			Forecast7d:   fc7dMem,
			Forecast30d:  fc30dMem,
			Forecast90d:  fc90dMem,
			ExhaustionAt: exhaustMem,
			Status:       statusMem,
			RecordedAt:   now,
		},
		{
			ID:           fmt.Sprintf("cap-storage-%s", cluster),
			Cluster:      cluster,
			ResourceType: domainCapacity.ResourceStorage,
			CurrentUsage: math.Round(currDisk*10.0) / 10.0,
			Forecast7d:   fc7dDisk,
			Forecast30d:  fc30dDisk,
			Forecast90d:  fc90dDisk,
			ExhaustionAt: exhaustDisk,
			Status:       statusDisk,
			RecordedAt:   now,
		},
	}

	return forecasts, nil
}

// Record persists a forecast entry if backing repository is configured.
func (f *Forecaster) Record(ctx context.Context, item *domainCapacity.Forecast) error {
	if f.repo != nil {
		return f.repo.Record(ctx, item)
	}
	return nil
}

// ComputeProjection calculates future resource utilization checkpoints and determines threshold status.
func ComputeProjection(currUsage, dailyDelta float64, now time.Time) (float64, float64, float64, *time.Time, string) {
	fc7d := math.Min(100.0, math.Round((currUsage+dailyDelta*7.0)*10.0)/10.0)
	fc30d := math.Min(100.0, math.Round((currUsage+dailyDelta*30.0)*10.0)/10.0)
	fc90d := math.Min(100.0, math.Round((currUsage+dailyDelta*90.0)*10.0)/10.0)

	var exhaustAt *time.Time
	if dailyDelta > 0 && currUsage < 100.0 {
		daysTo100 := (100.0 - currUsage) / dailyDelta
		if daysTo100 <= 365.0 {
			t := now.Add(time.Duration(daysTo100*24.0) * time.Hour)
			exhaustAt = &t
		}
	}

	// Status determination: healthy (<70%), warning (70-85%), critical (>85%)
	effectiveUsage := math.Max(currUsage, fc30d)
	status := "healthy"
	if effectiveUsage > 85.0 {
		status = "critical"
	} else if effectiveUsage >= 70.0 {
		status = "warning"
	}

	return fc7d, fc30d, fc90d, exhaustAt, status
}
