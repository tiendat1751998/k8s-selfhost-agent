package metrics

import (
	"fmt"
	"math"
	"strings"
	"time"
)

type nodeHostMapping struct {
	id       string
	name     string
	endpoint string
}

func normalizeNodeName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.TrimPrefix(name, "k8s")
	name = strings.TrimPrefix(name, "node-")
	name = strings.TrimPrefix(name, "host-")
	return name
}

func matchHost(hosts []nodeHostMapping, swarmHostname string, swarmAddr string, defaultName string) (string, string) {
	sHost := strings.ToLower(strings.TrimSpace(swarmHostname))
	sNorm := normalizeNodeName(swarmHostname)
	sAddr := strings.TrimSpace(swarmAddr)
	dHost := strings.ToLower(strings.TrimSpace(defaultName))
	dNorm := normalizeNodeName(defaultName)

	// 1. Exact match by name
	for _, h := range hosts {
		hLower := strings.ToLower(strings.TrimSpace(h.name))
		if (sHost != "" && hLower == sHost) || (dHost != "" && hLower == dHost) {
			return h.id, h.name
		}
	}

	// 2. Normalized name match (e.g. k8sworker3 <-> worker3)
	for _, h := range hosts {
		hNorm := normalizeNodeName(h.name)
		if (sNorm != "" && hNorm == sNorm) || (dNorm != "" && hNorm == dNorm) {
			return h.id, h.name
		}
		if sNorm != "" && len(sNorm) >= 3 && (strings.Contains(hNorm, sNorm) || strings.Contains(sNorm, hNorm)) {
			return h.id, h.name
		}
		if dNorm != "" && len(dNorm) >= 3 && (strings.Contains(hNorm, dNorm) || strings.Contains(dNorm, hNorm)) {
			return h.id, h.name
		}
	}

	// 3. IP / Endpoint match
	if sAddr != "" {
		for _, h := range hosts {
			if h.endpoint != "" && strings.Contains(h.endpoint, sAddr) {
				return h.id, h.name
			}
		}
	}

	return "", ""
}

// extractServiceNameFromContainerName derives a service name from standard container naming conventions.
func extractServiceNameFromContainerName(name string) string {
	name = strings.TrimPrefix(name, "/")
	if name == "" {
		return ""
	}
	// Check swarm container pattern: <service>.<slot>.<task_id>
	if parts := strings.Split(name, "."); len(parts) >= 3 {
		return parts[0]
	}
	return name
}
// GenerateAlerts evaluates system overview telemetry against warning thresholds and generates active alerts.
func GenerateAlerts(overview *SystemOverview, thresholds Thresholds) []MetricAlert {
	alerts := make([]MetricAlert, 0)
	if overview == nil {
		return alerts
	}

	for _, node := range overview.Nodes {
		// Node Down alert
		if node.Status != "ready" && node.Status != "active" {
			alerts = append(alerts, MetricAlert{
				NodeID:    node.NodeID,
				NodeName:  node.NodeName,
				Type:      "node_down",
				Message:   fmt.Sprintf("Node %s is %s", node.NodeName, node.Status),
				Value:     0,
				Threshold: 0,
			})
		}

		// CPU warning
		if node.CPUPercent >= thresholds.CPUWarning && thresholds.CPUWarning > 0 {
			alerts = append(alerts, MetricAlert{
				NodeID:    node.NodeID,
				NodeName:  node.NodeName,
				Type:      "cpu_high",
				Message:   fmt.Sprintf("CPU usage high: %.1f%% (threshold: %.1f%%)", node.CPUPercent, thresholds.CPUWarning),
				Value:     node.CPUPercent,
				Threshold: thresholds.CPUWarning,
			})
		}

		// Memory warning
		if node.MemoryPercent >= thresholds.MemoryWarning && thresholds.MemoryWarning > 0 {
			alerts = append(alerts, MetricAlert{
				NodeID:    node.NodeID,
				NodeName:  node.NodeName,
				Type:      "memory_high",
				Message:   fmt.Sprintf("Memory usage high: %.1f%% (threshold: %.1f%%)", node.MemoryPercent, thresholds.MemoryWarning),
				Value:     node.MemoryPercent,
				Threshold: thresholds.MemoryWarning,
			})
		}

		// Disk warning
		if node.DiskPercent >= thresholds.DiskWarning && thresholds.DiskWarning > 0 {
			alerts = append(alerts, MetricAlert{
				NodeID:    node.NodeID,
				NodeName:  node.NodeName,
				Type:      "disk_high",
				Message:   fmt.Sprintf("Disk usage high: %.1f%% (threshold: %.1f%%)", node.DiskPercent, thresholds.DiskWarning),
				Value:     node.DiskPercent,
				Threshold: thresholds.DiskWarning,
			})
		}
	}

	return alerts
}

// CalculateContainerNetworkRates updates real-time Rx/Tx rates for running containers and returns the current raw snapshot map.
func CalculateContainerNetworkRates(containerMetricsList []ContainerMetrics, prevStats map[string]containerNetRaw, elapsed float64) map[string]containerNetRaw {
	currMap := make(map[string]containerNetRaw, len(containerMetricsList))
	for i := range containerMetricsList {
		cm := &containerMetricsList[i]
		currMap[cm.ContainerID] = containerNetRaw{
			rxBytes: cm.NetworkRx,
			txBytes: cm.NetworkTx,
		}
		if elapsed > 0 && cm.State == "running" {
			if prev, ok := prevStats[cm.ContainerID]; ok && prev.rxBytes > 0 && cm.NetworkRx >= prev.rxBytes {
				rxDelta := cm.NetworkRx - prev.rxBytes
				if rxDelta > 0 {
					rate := int64(float64(rxDelta) / elapsed)
					if rate < 10*1024*1024*1024 {
						cm.NetworkRxRate = rate
					}
				}
			}
			if prev, ok := prevStats[cm.ContainerID]; ok && prev.txBytes > 0 && cm.NetworkTx >= prev.txBytes {
				txDelta := cm.NetworkTx - prev.txBytes
				if txDelta > 0 {
					rate := int64(float64(txDelta) / elapsed)
					if rate < 10*1024*1024*1024 {
						cm.NetworkTxRate = rate
					}
				}
			}
		}
	}
	return currMap
}

// BuildNodeMetricsFromAgent constructs a NodeMetrics record from scraped agent telemetry and matching containers.
func BuildNodeMetricsFromAgent(hostID string, am *AgentMetrics, containers []ContainerMetrics, totalAgents int) NodeMetrics {
	status := "ready"
	if am.Status != "online" {
		status = "down"
	}

	var nodeContainerCount, nodeRunningCount int
	for _, cm := range containers {
		matches := false
		if (cm.NodeID != "" && cm.NodeID == hostID) ||
			(cm.NodeName != "" && (cm.NodeName == am.Hostname || cm.NodeName == hostID)) {
			matches = true
		} else if totalAgents == 1 {
			matches = true
		}

		if matches {
			nodeContainerCount++
			if cm.State == "running" {
				nodeRunningCount++
			}
		}
	}

	osName := am.OS
	if osName == "" {
		osName = "linux"
	}
	arch := am.Arch
	if arch == "" {
		arch = "amd64"
	}
	distro := normalizeOSDistro(am.OSDistro, osName)
	kernel := normalizeKernelVersion(am.KernelVersion, osName)

	topProcs := am.TopProcesses
	if topProcs == nil {
		topProcs = make([]ProcessMetric, 0)
	}
	ifaces := am.NetworkInterfaces
	if ifaces == nil {
		ifaces = make([]NetworkInterface, 0)
	}
	diskDevices := am.DiskIO.Devices
	if diskDevices == nil {
		diskDevices = make([]DiskIOStats, 0)
	}

	return NodeMetrics{
		NodeID:               hostID,
		NodeName:             am.Hostname,
		Role:                 "agent",
		Status:               status,
		OS:                   osName,
		Arch:                 arch,
		OSDistro:             distro,
		KernelVersion:        kernel,
		CPUPercent:           am.CPUUsage,
		MemoryUsed:           am.MemUsed,
		MemoryTotal:          am.MemTotal,
		MemoryPercent:        am.MemPercent,
		DiskUsed:             am.DiskUsed,
		DiskTotal:            am.DiskTotal,
		DiskPercent:          am.DiskPercent,
		DiskReadBytesPerSec:  am.DiskIO.TotalReadBytesPerSec,
		DiskWriteBytesPerSec: am.DiskIO.TotalWriteBytesPerSec,
		DiskReadIOPS:         am.DiskIO.TotalReadIOPS,
		DiskWriteIOPS:        am.DiskIO.TotalWriteIOPS,
		DiskAvgAwaitMs:       am.DiskIO.AvgAwaitMs,
		DiskMaxIoUtilPct:     am.DiskIO.MaxIoUtilizationPct,
		DiskDevices:          diskDevices,
		NetworkRxBytes:       am.NetRxRate,
		NetworkTxBytes:       am.NetTxRate,
		NetworkInterfaces:    ifaces,
		ContainerCount:       nodeContainerCount,
		RunningCount:         nodeRunningCount,
		Processes:            am.Processes,
		UptimeSeconds:        am.Uptime,
		LoadAverage:          am.LoadAvg,
		TopProcesses:         topProcs,
		Source:               "agent",
		UpdatedAt:            am.LastSeen,
	}
}

// AggregateSystemOverview aggregates platform totals, rates, and active alerts into SystemOverview.
func AggregateSystemOverview(nodes []NodeMetrics, containers []ContainerMetrics, rps float64, dockerDiskPct float64, collectedAt time.Time, thresholds Thresholds) *SystemOverview {
	healthyNodesCount := 0
	var totalCPU float64
	var totalMemUsed, totalMemLimit int64
	var totalDiskUsed, totalDiskLimit int64
	var totalDiskReadBytesPerSec, totalDiskWriteBytesPerSec int64
	var runningContainersCount int

	for _, cnt := range containers {
		if cnt.State == "running" {
			runningContainersCount++
		}
	}

	for _, nm := range nodes {
		if nm.Status == "ready" {
			healthyNodesCount++
			totalDiskReadBytesPerSec += nm.DiskReadBytesPerSec
			totalDiskWriteBytesPerSec += nm.DiskWriteBytesPerSec
		}
		totalCPU += nm.CPUPercent
		totalMemUsed += nm.MemoryUsed
		totalMemLimit += nm.MemoryTotal
		totalDiskUsed += nm.DiskUsed
		totalDiskLimit += nm.DiskTotal
	}

	var totalMemPct float64
	if totalMemLimit > 0 {
		totalMemPct = (float64(totalMemUsed) / float64(totalMemLimit)) * 100.0
	}

	var totalDiskPct float64
	if totalDiskLimit > 0 {
		totalDiskPct = (float64(totalDiskUsed) / float64(totalDiskLimit)) * 100.0
	} else {
		totalDiskPct = dockerDiskPct
	}

	avgCPU := 0.0
	if len(nodes) > 0 {
		avgCPU = totalCPU / float64(len(nodes))
	}

	overview := &SystemOverview{
		Nodes:                     nodes,
		Containers:                containers,
		TotalNodes:                len(nodes),
		HealthyNodes:              healthyNodesCount,
		TotalContainers:           len(containers),
		RunningContainers:         runningContainersCount,
		TotalCPUPercent:           math.Round(avgCPU*100) / 100,
		TotalMemPercent:           math.Round(totalMemPct*100) / 100,
		TotalDiskPercent:          math.Round(totalDiskPct*100) / 100,
		TotalDiskReadBytesPerSec:  totalDiskReadBytesPerSec,
		TotalDiskWriteBytesPerSec: totalDiskWriteBytesPerSec,
		RequestsPerSec:            math.Round(rps*100) / 100,
		Alerts:                    make([]MetricAlert, 0),
		CollectedAt:               collectedAt,
	}

	overview.Alerts = GenerateAlerts(overview, thresholds)
	return overview
}
