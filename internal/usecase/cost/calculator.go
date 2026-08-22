package cost

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	domainCost "github.com/datdt/k8sselfhost/internal/domain/cost"
	domainDocker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	usecaseMetrics "github.com/datdt/k8sselfhost/internal/usecase/metrics"
)

const (
	// Unit Pricing Model (Cloud-equivalent hourly rates)
	CPURatePerHour     = 0.035  // $0.035 / vCPU / hour
	RAMRatePerHour     = 0.0045 // $0.0045 / GB / hour
	StorageRatePerHour = 0.0001 // $0.0001 / GB / hour
	HoursPerMonth      = 720.0  // 24 hours * 30 days
)

// MetricsProvider supplies infrastructure and container runtime metrics.
type MetricsProvider interface {
	GetLastSnapshot() *usecaseMetrics.SystemOverview
	GetAgentMetrics() map[string]*usecaseMetrics.AgentMetrics
}

// ComputeHostProvider supplies compute host metadata.
type ComputeHostProvider interface {
	ListAll(ctx context.Context) ([]domainDocker.ComputeHost, error)
}

// Calculator dynamically calculates FinOps infrastructure and workload costs from live telemetry.
type Calculator struct {
	computeHostRepo ComputeHostProvider
	metricsProvider MetricsProvider
}

// NewCalculator creates a new dynamic CostCalculator.
func NewCalculator(computeHostRepo ComputeHostProvider, metricsProvider MetricsProvider) *Calculator {
	return &Calculator{
		computeHostRepo: computeHostRepo,
		metricsProvider: metricsProvider,
	}
}

// GetClusterCosts computes aggregated monthly cluster costs from registered compute hosts and node hardware specs.
func (c *Calculator) GetClusterCosts(ctx context.Context) ([]domainCost.ClusterCost, error) {
	var totalCPUCost, totalRAMCost, totalStorageCost float64
	var hostCount int

	var agentMetrics map[string]*usecaseMetrics.AgentMetrics
	if c.metricsProvider != nil {
		agentMetrics = c.metricsProvider.GetAgentMetrics()
	}

	// 1. Fetch registered compute hosts
	if c.computeHostRepo != nil {
		hosts, err := c.computeHostRepo.ListAll(ctx)
		if err == nil && len(hosts) > 0 {
			for _, host := range hosts {
				hostCPUs := 2.0
				hostRAMGB := 4.0
				hostStorageGB := 50.0

				if am, ok := agentMetrics[host.ID]; ok && am != nil {
					if am.CPUCount > 0 {
						hostCPUs = float64(am.CPUCount)
					}
					if am.MemTotal > 0 {
						hostRAMGB = float64(am.MemTotal) / (1024.0 * 1024.0 * 1024.0)
					}
					if am.DiskTotal > 0 {
						hostStorageGB = float64(am.DiskTotal) / (1024.0 * 1024.0 * 1024.0)
					}
				}

				cpuCost := hostCPUs * CPURatePerHour * HoursPerMonth
				ramCost := hostRAMGB * RAMRatePerHour * HoursPerMonth
				storageCost := hostStorageGB * StorageRatePerHour * HoursPerMonth

				totalCPUCost += cpuCost
				totalRAMCost += ramCost
				totalStorageCost += storageCost
				hostCount++
			}
		}
	}

	// 2. Fallback to snapshot nodes if no hosts were found via repository
	if hostCount == 0 && c.metricsProvider != nil {
		snapshot := c.metricsProvider.GetLastSnapshot()
		if snapshot != nil && len(snapshot.Nodes) > 0 {
			for _, node := range snapshot.Nodes {
				hostCPUs := 2.0
				hostRAMGB := 4.0
				hostStorageGB := 50.0

				if node.MemoryTotal > 0 {
					hostRAMGB = float64(node.MemoryTotal) / (1024.0 * 1024.0 * 1024.0)
				}
				if node.DiskTotal > 0 {
					hostStorageGB = float64(node.DiskTotal) / (1024.0 * 1024.0 * 1024.0)
				}

				cpuCost := hostCPUs * CPURatePerHour * HoursPerMonth
				ramCost := hostRAMGB * RAMRatePerHour * HoursPerMonth
				storageCost := hostStorageGB * StorageRatePerHour * HoursPerMonth

				totalCPUCost += cpuCost
				totalRAMCost += ramCost
				totalStorageCost += storageCost
				hostCount++
			}
		}
	}

	// Graceful fallback if no compute hosts or nodes are registered
	if hostCount == 0 {
		return []domainCost.ClusterCost{}, nil
	}

	totalMonthlyCost := totalCPUCost + totalRAMCost + totalStorageCost
	dailyCost := totalMonthlyCost / 30.0

	clusterCost := domainCost.ClusterCost{
		ID:          "cluster-fleet-primary",
		Name:        "fleet-primary",
		Provider:    "docker",
		MonthlyCost: int(math.Round(totalMonthlyCost)),
		DailyCost:   int(math.Round(dailyCost)),
		CPUCost:     int(math.Round(totalCPUCost)),
		MemoryCost:  int(math.Round(totalRAMCost)),
		StorageCost: int(math.Round(totalStorageCost)),
		NetworkCost: 0,
		Trend:       0,
		UpdatedAt:   time.Now().UTC(),
	}

	return []domainCost.ClusterCost{clusterCost}, nil
}

// GetNamespaceCosts computes departmental and namespace resource cost allocations from container metrics.
func (c *Calculator) GetNamespaceCosts(ctx context.Context) ([]domainCost.NamespaceCost, error) {
	if c.metricsProvider == nil {
		return []domainCost.NamespaceCost{}, nil
	}

	snapshot := c.metricsProvider.GetLastSnapshot()
	if snapshot == nil || len(snapshot.Containers) == 0 {
		return []domainCost.NamespaceCost{}, nil
	}

	type nsAgg struct {
		namespace      string
		containerCount int
		cpuPercentSum  float64
		memBytesSum    int64
		utilSum        float64
	}

	nsMap := make(map[string]*nsAgg)

	for _, cnt := range snapshot.Containers {
		ns := ExtractNamespace(cnt.ContainerName)
		agg, ok := nsMap[ns]
		if !ok {
			agg = &nsAgg{namespace: ns}
			nsMap[ns] = agg
		}
		agg.containerCount++
		agg.cpuPercentSum += cnt.CPUPercent
		agg.memBytesSum += cnt.MemoryUsed
		agg.utilSum += (cnt.CPUPercent + cnt.MemoryPercent) / 2.0
	}

	var results []domainCost.NamespaceCost
	for ns, agg := range nsMap {
		if agg.containerCount == 0 {
			continue
		}

		cpuVCPU := agg.cpuPercentSum / 100.0
		if cpuVCPU < 0.1 {
			cpuVCPU = 0.1 * float64(agg.containerCount)
		}

		memGB := float64(agg.memBytesSum) / (1024.0 * 1024.0 * 1024.0)
		if memGB < 0.2 {
			memGB = 0.25 * float64(agg.containerCount)
		}

		cpuCost := cpuVCPU * CPURatePerHour * HoursPerMonth
		memCost := memGB * RAMRatePerHour * HoursPerMonth
		monthlyCost := cpuCost + memCost
		if monthlyCost < 1.0 {
			monthlyCost = 1.0 * float64(agg.containerCount)
		}

		avgUtil := agg.utilSum / float64(agg.containerCount)
		if avgUtil > 100.0 {
			avgUtil = 100.0
		} else if avgUtil < 1.0 {
			avgUtil = 1.0
		}

		results = append(results, domainCost.NamespaceCost{
			ID:              "ns-" + ns,
			Namespace:       ns,
			Cluster:         "fleet-primary",
			CPURequested:    fmt.Sprintf("%.2f vCPU", cpuVCPU),
			MemoryRequested: fmt.Sprintf("%.2f GB", memGB),
			MonthlyCost:     int(math.Round(monthlyCost)),
			Utilization:     int(math.Round(avgUtil)),
			UpdatedAt:       time.Now().UTC(),
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].MonthlyCost > results[j].MonthlyCost
	})

	return results, nil
}

// GetResourceWaste identifies underutilized containers (<10% CPU or RAM) and computes wasted spend.
func (c *Calculator) GetResourceWaste(ctx context.Context) ([]domainCost.ResourceWaste, error) {
	if c.metricsProvider == nil {
		return []domainCost.ResourceWaste{}, nil
	}

	snapshot := c.metricsProvider.GetLastSnapshot()
	if snapshot == nil || len(snapshot.Containers) == 0 {
		return []domainCost.ResourceWaste{}, nil
	}

	var wasteItems []domainCost.ResourceWaste

	for _, cnt := range snapshot.Containers {
		if cnt.State != "running" && cnt.State != "" {
			continue
		}

		// Waste criteria: using < 10% of allocated CPU or RAM
		if cnt.CPUPercent < 10.0 || cnt.MemoryPercent < 10.0 {
			var wasteType string
			if cnt.CPUPercent < 10.0 && cnt.MemoryPercent < 10.0 {
				wasteType = "idle_workload"
			} else if cnt.CPUPercent < 10.0 {
				wasteType = "low_cpu_utilization"
			} else {
				wasteType = "low_memory_utilization"
			}

			var severity string
			if cnt.CPUPercent < 3.0 && cnt.MemoryPercent < 3.0 {
				severity = "critical"
			} else if cnt.CPUPercent < 5.0 || cnt.MemoryPercent < 5.0 {
				severity = "high"
			} else if cnt.CPUPercent < 10.0 && cnt.MemoryPercent < 10.0 {
				severity = "medium"
			} else {
				severity = "low"
			}

			memGB := float64(cnt.MemoryUsed) / (1024.0 * 1024.0 * 1024.0)
			if memGB <= 0 {
				memGB = 0.25
			}
			cpuVCPU := cnt.CPUPercent / 100.0
			if cpuVCPU <= 0 {
				cpuVCPU = 0.05
			}

			baseCost := (cpuVCPU*CPURatePerHour + memGB*RAMRatePerHour) * HoursPerMonth
			idleFactor := (100.0 - math.Max(cnt.CPUPercent, cnt.MemoryPercent)) / 100.0
			if idleFactor < 0.1 {
				idleFactor = 0.1
			}
			wastedCost := int(math.Round(math.Max(1.0, baseCost*idleFactor)))

			cID := cnt.ContainerID
			if len(cID) > 12 {
				cID = cID[:12]
			}

			cpuVal := int(math.Round(cnt.CPUPercent))
			memVal := int(math.Round(cnt.MemoryPercent))

			wasteItems = append(wasteItems, domainCost.ResourceWaste{
				ID:         "waste-" + cID,
				Type:       wasteType,
				Resource:   cnt.ContainerName,
				Namespace:  ExtractNamespace(cnt.ContainerName),
				Cluster:    "fleet-primary",
				CPUUtil:    &cpuVal,
				MemUtil:    &memVal,
				WastedCost: wastedCost,
				Severity:   severity,
				UpdatedAt:  time.Now().UTC(),
			})
		}
	}

	sort.Slice(wasteItems, func(i, j int) bool {
		return wasteItems[i].WastedCost > wasteItems[j].WastedCost
	})

	return wasteItems, nil
}

// ExtractNamespace extracts a logical namespace grouping from container names.
func ExtractNamespace(containerName string) string {
	name := strings.ToLower(strings.TrimPrefix(containerName, "/"))
	switch {
	case strings.HasPrefix(name, "k8s_"):
		parts := strings.Split(name, "_")
		if len(parts) >= 3 && parts[2] != "" {
			return parts[2]
		}
		return "k8s-system"
	case strings.Contains(name, "monitor") || strings.Contains(name, "prom") || strings.Contains(name, "grafana") || strings.Contains(name, "loki") || strings.Contains(name, "tempo"):
		return "monitoring"
	case strings.Contains(name, "ingress") || strings.Contains(name, "traefik") || strings.Contains(name, "nginx") || strings.Contains(name, "envoy") || strings.Contains(name, "proxy"):
		return "ingress"
	case strings.Contains(name, "postgres") || strings.Contains(name, "mysql") || strings.Contains(name, "redis") || strings.Contains(name, "db") || strings.Contains(name, "mongo"):
		return "database"
	case strings.Contains(name, "agent") || strings.Contains(name, "k8s") || strings.Contains(name, "daemon") || strings.Contains(name, "system"):
		return "system"
	case strings.Contains(name, "sec") || strings.Contains(name, "vault") || strings.Contains(name, "cert"):
		return "security"
	default:
		return "default"
	}
}
