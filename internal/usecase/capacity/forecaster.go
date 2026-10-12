package capacity

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
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

	var dailyDeltaCPU, dailyDeltaMem, dailyDeltaDisk float64
	if f.repo != nil {
		snapshots, err := f.repo.ListSnapshots(ctx, cluster)
		if err == nil && len(snapshots) >= 2 {
			sort.Slice(snapshots, func(i, j int) bool {
				return snapshots[i].RecordedAt.Before(snapshots[j].RecordedAt)
			})
			t0 := snapshots[0].RecordedAt
			xs := make([]float64, len(snapshots))
			ysCPU := make([]float64, len(snapshots))
			ysMem := make([]float64, len(snapshots))
			ysDisk := make([]float64, len(snapshots))

			for i, s := range snapshots {
				xs[i] = s.RecordedAt.Sub(t0).Hours() / 24.0
				ysCPU[i] = s.CPU
				ysMem[i] = s.Memory
				ysDisk[i] = s.Storage
			}

			slopeCPU := CalculateLinearSlope(xs, ysCPU)
			slopeMem := CalculateLinearSlope(xs, ysMem)
			slopeDisk := CalculateLinearSlope(xs, ysDisk)

			if slopeCPU > 0 {
				dailyDeltaCPU = slopeCPU
			}
			if slopeMem > 0 {
				dailyDeltaMem = slopeMem
			}
			if slopeDisk > 0 {
				dailyDeltaDisk = slopeDisk
			}
		}
	}
	// If fewer than 2 snapshots exist, slope remains 0.0 (flat/unprojected), NEVER synthetic growth rate.

	// 1. CPU linear trend
	fc7dCPU, fc30dCPU, fc90dCPU, exhaustCPU, statusCPU := ComputeProjection(currCPU, dailyDeltaCPU, now)

	// 2. Memory linear trend
	fc7dMem, fc30dMem, fc90dMem, exhaustMem, statusMem := ComputeProjection(currMem, dailyDeltaMem, now)

	// 3. Storage linear trend
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

// ListSnapshots retrieves historical capacity snapshots if backing repository is configured.
func (f *Forecaster) ListSnapshots(ctx context.Context, cluster string) ([]domainCapacity.Snapshot, error) {
	if f.repo != nil {
		return f.repo.ListSnapshots(ctx, cluster)
	}
	return nil, nil
}



// ListNodeHeadroom calculates factual node-level capacity headroom from live telemetry.
func (f *Forecaster) ListNodeHeadroom(ctx context.Context, cluster string) ([]domainCapacity.NodeHeadroom, error) {
	if f.metricsProvider == nil {
		return []domainCapacity.NodeHeadroom{}, nil
	}

	snapshot := f.metricsProvider.GetLastSnapshot()
	if snapshot == nil || len(snapshot.Nodes) == 0 {
		return []domainCapacity.NodeHeadroom{}, nil
	}

	agentMetrics := f.metricsProvider.GetAgentMetrics()

	// Sort nodes deterministically: ready nodes first, then alphabetical by name
	nodes := append([]usecaseMetrics.NodeMetrics(nil), snapshot.Nodes...)
	sort.SliceStable(nodes, func(i, j int) bool {
		iReady := strings.ToLower(nodes[i].Status) == "ready"
		jReady := strings.ToLower(nodes[j].Status) == "ready"
		if iReady != jReady {
			return iReady
		}
		nameI := nodes[i].NodeName
		if nameI == "" {
			nameI = nodes[i].NodeID
		}
		nameJ := nodes[j].NodeName
		if nameJ == "" {
			nameJ = nodes[j].NodeID
		}
		return nameI < nameJ
	})

	headrooms := make([]domainCapacity.NodeHeadroom, 0, len(nodes))
	for _, node := range nodes {
		nodeID := node.NodeID
		if nodeID == "" {
			nodeID = node.NodeName
		}
		nodeName := node.NodeName
		if nodeName == "" {
			nodeName = node.NodeID
		}

		// Role determination: control-plane vs worker
		roleLower := strings.ToLower(node.Role)
		role := "worker"
		if strings.Contains(roleLower, "master") || strings.Contains(roleLower, "manager") || strings.Contains(roleLower, "control") {
			role = "control-plane"
		}

		// Lookup agent metrics for CPU core count
		var cpuCount int
		if agentMetrics != nil {
			if am, ok := agentMetrics[node.NodeID]; ok && am != nil && am.CPUCount > 0 {
				cpuCount = am.CPUCount
			} else if am, ok := agentMetrics[node.NodeName]; ok && am != nil && am.CPUCount > 0 {
				cpuCount = am.CPUCount
			} else {
				for _, am := range agentMetrics {
					if am != nil && (am.Hostname == node.NodeName || am.Hostname == node.NodeID) && am.CPUCount > 0 {
						cpuCount = am.CPUCount
						break
					}
				}
			}
		}

		cpuTotalCores := 4.0
		if cpuCount > 0 {
			cpuTotalCores = float64(cpuCount)
		}

		cpuUsagePercent := math.Round(node.CPUPercent*10) / 10
		cpuAllocatedCores := math.Round(cpuTotalCores*(node.CPUPercent/100.0)*10) / 10

		memTotalGiB := math.Round(float64(node.MemoryTotal)/(1024*1024*1024)*10) / 10
		if memTotalGiB == 0 {
			memTotalGiB = 8.0
		}
		memAllocatedGiB := math.Round(float64(node.MemoryUsed)/(1024*1024*1024)*10) / 10
		memUsagePercent := math.Round(node.MemoryPercent*10) / 10

		podCount := node.ContainerCount
		if podCount == 0 && node.RunningCount > 0 {
			podCount = node.RunningCount
		}
		podCapacity := max(60, podCount*2)

		binPackingScore := math.Round(((node.CPUPercent+node.MemoryPercent)/2.0)*10) / 10
		headroomPercent := math.Max(0, math.Round((100.0-math.Max(node.CPUPercent, node.MemoryPercent))*10)/10)

		nodeStatus := strings.ToLower(node.Status)
		isDown := nodeStatus == "down" || nodeStatus == "disconnected" || nodeStatus == "offline" || nodeStatus == "error"

		status := "healthy"
		if isDown || headroomPercent < 15.0 {
			status = "critical"
		} else if headroomPercent < 30.0 {
			status = "warning"
		}

		headrooms = append(headrooms, domainCapacity.NodeHeadroom{
			ID:                nodeID,
			Name:              nodeName,
			Role:              role,
			CPUTotalCores:     cpuTotalCores,
			CPUAllocatedCores: cpuAllocatedCores,
			CPUUsagePercent:   cpuUsagePercent,
			MemTotalGiB:       memTotalGiB,
			MemAllocatedGiB:   memAllocatedGiB,
			MemUsagePercent:   memUsagePercent,
			PodCount:          podCount,
			PodCapacity:       podCapacity,
			BinPackingScore:   binPackingScore,
			Status:            status,
			HeadroomPercent:   headroomPercent,
		})
	}

	return headrooms, nil
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

// CalculateLinearSlope computes linear regression slope m from points (x, y):
// slope = (N*sum(x*y) - sum(x)*sum(y)) / (N*sum(x^2) - (sum(x))^2)
// If fewer than 2 snapshots exist or denominator is zero, slope must be 0.0.
func CalculateLinearSlope(xs, ys []float64) float64 {
	n := float64(len(xs))
	if n < 2 || len(xs) != len(ys) {
		return 0.0
	}

	var sumX, sumY, sumXY, sumX2 float64
	for i := 0; i < len(xs); i++ {
		sumX += xs[i]
		sumY += ys[i]
		sumXY += xs[i] * ys[i]
		sumX2 += xs[i] * xs[i]
	}

	denom := n*sumX2 - sumX*sumX
	if math.Abs(denom) < 1e-9 {
		return 0.0
	}

	return (n*sumXY - sumX*sumY) / denom
}

