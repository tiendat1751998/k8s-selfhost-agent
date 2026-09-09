package metrics

import (
	"context"
	"fmt"
	"math"
	"time"
)

func (c *Collector) CollectOnce(ctx context.Context) (*SystemOverview, error) {
	now := time.Now().UTC()

	// 1. Calculate Requests/Sec
	var rps float64
	if c.requestCountFn != nil {
		currReqs := c.requestCountFn()
		c.mu.Lock()
		if !c.lastReqTime.IsZero() {
			elapsed := now.Sub(c.lastReqTime).Seconds()
			if elapsed > 0 {
				rps = float64(currReqs-c.lastReqCount) / elapsed
				if rps < 0 {
					rps = 0
				}
			}
		}
		c.lastReqCount = currReqs
		c.lastReqTime = now
		c.mu.Unlock()
	}

	containerMetricsList := make([]ContainerMetrics, 0)
	var runningContainersCount int

	c.dockerDiskMu.RLock()
	diskPercent := c.dockerDiskPercent
	c.dockerDiskMu.RUnlock()


	// 2. Fetch Docker Containers & Stats if Docker client is available
	containerMetricsList, runningContainersCount = c.collectDockerContainers(ctx, now)

	// 3. Build Agent Nodes (pure k8s-agent server topology)
	nodeMetricsList := make([]NodeMetrics, 0)
	c.agentMu.RLock()
	for hostID, am := range c.agentMetrics {
		if am == nil {
			continue
		}
		status := "ready"
		if am.Status != "online" {
			status = "down"
		}

		// Count actual containers running on this node
		var nodeContainerCount, nodeRunningCount int
		for _, cm := range containerMetricsList {
			matches := false
			if (cm.NodeID != "" && cm.NodeID == hostID) ||
				(cm.NodeName != "" && (cm.NodeName == am.Hostname || cm.NodeName == hostID)) {
				matches = true
			} else if len(c.agentMetrics) == 1 {
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

		nodeMetricsList = append(nodeMetricsList, NodeMetrics{
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
		})
	}
	c.agentMu.RUnlock()

	// 4. Aggregate System Overview
	healthyNodesCount := 0
	var totalCPU float64
	var totalMemUsed, totalMemLimit int64
	var totalDiskUsed, totalDiskLimit int64
	var totalDiskReadBytesPerSec, totalDiskWriteBytesPerSec int64

	for _, nm := range nodeMetricsList {
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
		totalDiskPct = diskPercent
	}

	avgCPU := 0.0
	if len(nodeMetricsList) > 0 {
		avgCPU = totalCPU / float64(len(nodeMetricsList))
	}

	overview := &SystemOverview{
		Nodes:                     nodeMetricsList,
		Containers:                containerMetricsList,
		TotalNodes:                len(nodeMetricsList),
		HealthyNodes:              healthyNodesCount,
		TotalContainers:           len(containerMetricsList),
		RunningContainers:         runningContainersCount,
		TotalCPUPercent:           math.Round(avgCPU*100) / 100,
		TotalMemPercent:           math.Round(totalMemPct*100) / 100,
		TotalDiskPercent:          math.Round(totalDiskPct*100) / 100,
		TotalDiskReadBytesPerSec:  totalDiskReadBytesPerSec,
		TotalDiskWriteBytesPerSec: totalDiskWriteBytesPerSec,
		RequestsPerSec:            math.Round(rps*100) / 100,
		Alerts:                    make([]MetricAlert, 0),
		CollectedAt:               now,
	}

	overview.Alerts = c.generateAlerts(overview)

	return overview, nil
}


func (c *Collector) generateAlerts(overview *SystemOverview) []MetricAlert {
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
		if node.CPUPercent >= c.thresholds.CPUWarning && c.thresholds.CPUWarning > 0 {
			alerts = append(alerts, MetricAlert{
				NodeID:    node.NodeID,
				NodeName:  node.NodeName,
				Type:      "cpu_high",
				Message:   fmt.Sprintf("CPU usage high: %.1f%% (threshold: %.1f%%)", node.CPUPercent, c.thresholds.CPUWarning),
				Value:     node.CPUPercent,
				Threshold: c.thresholds.CPUWarning,
			})
		}

		// Memory warning
		if node.MemoryPercent >= c.thresholds.MemoryWarning && c.thresholds.MemoryWarning > 0 {
			alerts = append(alerts, MetricAlert{
				NodeID:    node.NodeID,
				NodeName:  node.NodeName,
				Type:      "memory_high",
				Message:   fmt.Sprintf("Memory usage high: %.1f%% (threshold: %.1f%%)", node.MemoryPercent, c.thresholds.MemoryWarning),
				Value:     node.MemoryPercent,
				Threshold: c.thresholds.MemoryWarning,
			})
		}

		// Disk warning
		if node.DiskPercent >= c.thresholds.DiskWarning && c.thresholds.DiskWarning > 0 {
			alerts = append(alerts, MetricAlert{
				NodeID:    node.NodeID,
				NodeName:  node.NodeName,
				Type:      "disk_high",
				Message:   fmt.Sprintf("Disk usage high: %.1f%% (threshold: %.1f%%)", node.DiskPercent, c.thresholds.DiskWarning),
				Value:     node.DiskPercent,
				Threshold: c.thresholds.DiskWarning,
			})
		}
	}

	return alerts
}

