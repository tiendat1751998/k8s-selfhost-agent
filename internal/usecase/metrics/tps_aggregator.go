package metrics

import (
	"math"
	"sort"
	"strings"
	"time"
)

// collectNetworkAndContainerTPS aggregates node and container network rates and per-service statistics.
func (c *TPSCollector) collectNetworkAndContainerTPS(now time.Time) (NetworkTPS, []NodeTPS, []ServiceTPS) {
	perNode := make([]NodeTPS, 0)
	services := make([]ServiceTPS, 0)
	var totalRx, totalTx int64

	if c.metricsProvider == nil {
		return NetworkTPS{}, perNode, services
	}

	nodeNameMap := make(map[string]string)
	nodeIDByName := make(map[string]string)
	agentMap := c.metricsProvider.GetAgentMetrics()
	for hostID, am := range agentMap {
		if am == nil {
			continue
		}
		if am.Hostname != "" {
			nodeNameMap[hostID] = am.Hostname
			nodeIDByName[strings.ToLower(strings.TrimSpace(am.Hostname))] = hostID
			nodeIDByName[normalizeNodeName(am.Hostname)] = hostID
		}
		var diskRead, diskWrite int64
		for _, p := range am.TopProcesses {
			diskRead += p.ReadBytesPerSec
			diskWrite += p.WriteBytesPerSec
		}

		node := NodeTPS{
			NodeName:        am.Hostname,
			NodeID:          hostID,
			RxBytesPerSec:   am.NetRxRate,
			TxBytesPerSec:   am.NetTxRate,
			DiskReadPerSec:  diskRead,
			DiskWritePerSec: diskWrite,
			Processes:       am.Processes,
		}
		perNode = append(perNode, node)
		totalRx += am.NetRxRate
		totalTx += am.NetTxRate
	}

	snapshot := c.metricsProvider.GetLastSnapshot()
	if snapshot != nil {
		for _, nm := range snapshot.Nodes {
			if nm.NodeID != "" && nm.NodeName != "" {
				nodeNameMap[nm.NodeID] = nm.NodeName
				nodeIDByName[strings.ToLower(strings.TrimSpace(nm.NodeName))] = nm.NodeID
				nodeIDByName[normalizeNodeName(nm.NodeName)] = nm.NodeID
			}
		}

		// If agentMap was empty, populate nodes from SystemOverview snapshot
		if len(agentMap) == 0 {
			for _, nm := range snapshot.Nodes {
				var diskRead, diskWrite int64
				for _, p := range nm.TopProcesses {
					diskRead += p.ReadBytesPerSec
					diskWrite += p.WriteBytesPerSec
				}
				procCount := nm.Processes
				if procCount == 0 {
					procCount = nm.ContainerCount
				}
				node := NodeTPS{
					NodeName:        nm.NodeName,
					NodeID:          nm.NodeID,
					RxBytesPerSec:   nm.NetworkRxBytes,
					TxBytesPerSec:   nm.NetworkTxBytes,
					DiskReadPerSec:  diskRead,
					DiskWritePerSec: diskWrite,
					Processes:       procCount,
				}
				perNode = append(perNode, node)
				totalRx += nm.NetworkRxBytes
				totalTx += nm.NetworkTxBytes
			}
		}

		// Source 5: Aggregate Container Network delta stats & per-service rates
		c.mu.Lock()
		elapsed := 0.0
		if !c.prevContainerTime.IsZero() {
			elapsed = now.Sub(c.prevContainerTime).Seconds()
		}

		var containerRxDelta, containerTxDelta int64
		containerRxRates := make(map[string]int64, len(snapshot.Containers))
		containerTxRates := make(map[string]int64, len(snapshot.Containers))

		if elapsed > 0 {
			for _, cm := range snapshot.Containers {
				if prev, ok := c.prevContainerStats[cm.ContainerID]; ok && prev.rxBytes > 0 {
					rxDelta := safeDeltaInt64(cm.NetworkRx, prev.rxBytes)
					if rxDelta > 0 {
						rate := int64(float64(rxDelta) / elapsed)
						if rate < 10*1024*1024*1024 {
							containerRxDelta += rxDelta
							containerRxRates[cm.ContainerID] = rate
						}
					}
				}
				if prev, ok := c.prevContainerStats[cm.ContainerID]; ok && prev.txBytes > 0 {
					txDelta := safeDeltaInt64(cm.NetworkTx, prev.txBytes)
					if txDelta > 0 {
						rate := int64(float64(txDelta) / elapsed)
						if rate < 10*1024*1024*1024 {
							containerTxDelta += txDelta
							containerTxRates[cm.ContainerID] = rate
						}
					}
				}
			}
			// If no agent nodes reported, fallback to container network rates
			if len(agentMap) == 0 && len(snapshot.Nodes) == 0 {
				totalRx = int64(float64(containerRxDelta) / elapsed)
				totalTx = int64(float64(containerTxDelta) / elapsed)
			}
		}

		currMap := make(map[string]containerNetRaw, len(snapshot.Containers))
		for _, cm := range snapshot.Containers {
			currMap[cm.ContainerID] = containerNetRaw{
				rxBytes: cm.NetworkRx,
				txBytes: cm.NetworkTx,
			}
		}
		c.prevContainerStats = currMap
		c.prevContainerTime = now
		c.mu.Unlock()

		// Group containers by service and node
		if len(snapshot.Containers) > 0 {
			type serviceAcc struct {
				name          string
				nodeID        string
				nodeName      string
				totalCount    int
				runningCount  int
				totalCPU      float64
				totalMemBytes int64
				totalMemLimit int64
				rxRate        int64
				txRate        int64
				totalRxBytes  int64
				totalTxBytes  int64
			}

			serviceMap := make(map[string]*serviceAcc)
			serviceOrder := make([]string, 0)

			for _, cm := range snapshot.Containers {
				sName := extractServiceName(cm)
				if sName == "" {
					sName = "unknown"
				}

				nodeID := cm.NodeID
				nodeName := cm.NodeName

				if nodeID == "" && cm.Labels != nil {
					if nid, ok := cm.Labels["com.docker.swarm.node.id"]; ok {
						nodeID = nid
					}
				}

				if nodeName == "" && nodeID != "" {
					if name, ok := nodeNameMap[nodeID]; ok {
						nodeName = name
					}
				}

				if nodeName != "" {
					if id, ok := nodeIDByName[strings.ToLower(strings.TrimSpace(nodeName))]; ok {
						nodeID = id
					} else if id, ok := nodeIDByName[normalizeNodeName(nodeName)]; ok {
						nodeID = id
					}
				}

				if nodeName == "" && nodeID != "" {
					if id, ok := nodeIDByName[strings.ToLower(strings.TrimSpace(nodeID))]; ok {
						nodeName = nodeNameMap[id]
						nodeID = id
					} else if id, ok := nodeIDByName[normalizeNodeName(nodeID)]; ok {
						nodeName = nodeNameMap[id]
						nodeID = id
					}
				}

				if (nodeID == "" || nodeName == "") && len(snapshot.Nodes) == 1 {
					if nodeID == "" {
						nodeID = snapshot.Nodes[0].NodeID
					}
					if nodeName == "" {
						nodeName = snapshot.Nodes[0].NodeName
					}
				}

				if nodeName == "" && nodeID != "" {
					nodeName = nodeID
				}

				groupKey := sName + "|" + nodeID
				acc, ok := serviceMap[groupKey]
				if !ok {
					acc = &serviceAcc{
						name:     sName,
						nodeID:   nodeID,
						nodeName: nodeName,
					}
					serviceMap[groupKey] = acc
					serviceOrder = append(serviceOrder, groupKey)
				}

				acc.totalCount++
				if strings.EqualFold(cm.State, "running") {
					acc.runningCount++
				}
				acc.totalCPU += cm.CPUPercent
				acc.totalMemBytes += cm.MemoryUsed
				acc.totalMemLimit += cm.MemoryLimit

				rxRate := cm.NetworkRxRate
				if rxRate == 0 {
					rxRate = containerRxRates[cm.ContainerID]
				}
				txRate := cm.NetworkTxRate
				if txRate == 0 {
					txRate = containerTxRates[cm.ContainerID]
				}
				acc.rxRate += rxRate
				acc.txRate += txRate
				acc.totalRxBytes += cm.NetworkRx
				acc.totalTxBytes += cm.NetworkTx
			}

			for _, groupKey := range serviceOrder {
				acc := serviceMap[groupKey]
				var memPercent float64
				if acc.totalMemLimit > 0 {
					memPercent = math.Round((float64(acc.totalMemBytes)/float64(acc.totalMemLimit))*10000) / 100
				}

				status := "down"
				if acc.runningCount == acc.totalCount && acc.totalCount > 0 {
					status = "healthy"
				} else if acc.runningCount > 0 {
					status = "degraded"
				}

				services = append(services, ServiceTPS{
					ServiceName:    acc.name,
					NodeID:         acc.nodeID,
					NodeName:       acc.nodeName,
					ContainerCount: acc.totalCount,
					CPUPercent:     math.Round(acc.totalCPU*100) / 100,
					MemoryUsedMB:   math.Round((float64(acc.totalMemBytes)/(1024*1024))*100) / 100,
					MemoryPercent:  memPercent,
					RxBytesPerSec:  acc.rxRate,
					TxBytesPerSec:  acc.txRate,
					TotalRxBytes:   acc.totalRxBytes,
					TotalTxBytes:   acc.totalTxBytes,
					Status:         status,
				})
			}

			// Sort services by CPU percent descending, then MemoryUsedMB descending
			sort.Slice(services, func(i, j int) bool {
				if services[i].CPUPercent != services[j].CPUPercent {
					return services[i].CPUPercent > services[j].CPUPercent
				}
				return services[i].MemoryUsedMB > services[j].MemoryUsedMB
			})
		}
	}

	return NetworkTPS{
		TotalRxBytesPerSec: totalRx,
		TotalTxBytesPerSec: totalTx,
	}, perNode, services
}
