package metrics

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/swarm"
	"go.uber.org/zap"
)

// calculateCPUPercent calculates CPU usage percentage from Docker container stats.
func calculateCPUPercent(stats *container.StatsResponse) float64 {
	if stats == nil {
		return 0.0
	}

	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage) - float64(stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage) - float64(stats.PreCPUStats.SystemUsage)

	if systemDelta > 0.0 && cpuDelta > 0.0 {
		numCPUs := float64(stats.CPUStats.OnlineCPUs)
		if numCPUs == 0 {
			numCPUs = float64(len(stats.CPUStats.CPUUsage.PercpuUsage))
		}
		if numCPUs == 0 {
			numCPUs = 1.0
		}
		cpuPercent := (cpuDelta / systemDelta) * numCPUs * 100.0
		return math.Round(cpuPercent*100) / 100
	}

	return 0.0
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


// collectDockerContainers queries Docker daemon for containers and extracts CPU/memory/network metrics.
func (c *Collector) collectDockerContainers(ctx context.Context, now time.Time) ([]ContainerMetrics, int) {
	containerMetricsList := make([]ContainerMetrics, 0)
	var runningContainersCount int

	if c.dockerClient != nil {
		info, infoErr := c.dockerClient.Info(ctx)
		if infoErr != nil {
			c.logger.Debug("Failed to fetch Docker daemon info", zap.Error(infoErr))
		}

		containersList, listErr := c.dockerClient.ContainerList(ctx, container.ListOptions{All: true})
		if listErr != nil {
			c.logger.Debug("Failed to list Docker containers", zap.Error(listErr))
		}

		if len(containersList) > 0 {
			// Build known hosts list from agent metrics and compute host repository
			var knownHosts []nodeHostMapping
			c.agentMu.RLock()
			for hID, am := range c.agentMetrics {
				if am != nil {
					knownHosts = append(knownHosts, nodeHostMapping{
						id:   hID,
						name: am.Hostname,
					})
				}
			}
			c.agentMu.RUnlock()

			if c.computeHostRepo != nil {
				if dbHosts, err := c.computeHostRepo.ListAll(ctx); err == nil {
					seen := make(map[string]bool)
					for _, kh := range knownHosts {
						seen[kh.id] = true
					}
					for _, dh := range dbHosts {
						if !seen[dh.ID] {
							knownHosts = append(knownHosts, nodeHostMapping{
								id:       dh.ID,
								name:     dh.Name,
								endpoint: dh.Endpoint,
							})
						} else {
							for i := range knownHosts {
								if knownHosts[i].id == dh.ID && knownHosts[i].endpoint == "" {
									knownHosts[i].endpoint = dh.Endpoint
								}
							}
						}
					}
				}
			}

			var swarmNodes []swarm.Node
			if lister, ok := c.dockerClient.(SwarmNodeLister); ok {
				if sn, err := lister.NodeList(ctx, swarm.NodeListOptions{}); err == nil {
					swarmNodes = sn
				}
			}

			swarmNodeMap := make(map[string]swarm.Node, len(swarmNodes))
			for _, sn := range swarmNodes {
				swarmNodeMap[sn.ID] = sn
			}

			type rawContainerStat struct {
				cm ContainerMetrics
			}

			statCh := make(chan rawContainerStat, len(containersList))
			var wg sync.WaitGroup

			for _, cnt := range containersList {
				wg.Add(1)
				go func(cSummary container.Summary) {
					defer wg.Done()

					name := cSummary.ID
					if len(cSummary.Names) > 0 {
						name = cSummary.Names[0]
						if len(name) > 0 && name[0] == '/' {
							name = name[1:]
						}
					}

					swarmNodeID := ""
					if cSummary.Labels != nil {
						if nid, ok := cSummary.Labels["com.docker.swarm.node.id"]; ok {
							swarmNodeID = nid
						}
					}

					var resolvedNodeID, resolvedNodeName string

					if swarmNodeID != "" {
						if sn, ok := swarmNodeMap[swarmNodeID]; ok {
							resolvedNodeID, resolvedNodeName = matchHost(knownHosts, sn.Description.Hostname, sn.Status.Addr, "")
							if resolvedNodeName == "" {
								resolvedNodeName = sn.Description.Hostname
							}
							if resolvedNodeID == "" {
								resolvedNodeID = swarmNodeID
							}
						} else if info.Swarm.NodeID != "" && swarmNodeID == info.Swarm.NodeID {
							resolvedNodeID, resolvedNodeName = matchHost(knownHosts, info.Name, "", info.Name)
							if resolvedNodeName == "" {
								resolvedNodeName = info.Name
							}
							if resolvedNodeID == "" {
								resolvedNodeID = swarmNodeID
							}
						} else {
							resolvedNodeID = swarmNodeID
						}
					}

					if resolvedNodeID == "" || resolvedNodeName == "" {
						hID, hName := matchHost(knownHosts, info.Name, "", info.Name)
						if resolvedNodeID == "" {
							if hID != "" {
								resolvedNodeID = hID
							} else if len(knownHosts) == 1 {
								resolvedNodeID = knownHosts[0].id
							} else if info.ID != "" {
								resolvedNodeID = info.ID
							} else if swarmNodeID != "" {
								resolvedNodeID = swarmNodeID
							}
						}
						if resolvedNodeName == "" {
							if hName != "" {
								resolvedNodeName = hName
							} else if len(knownHosts) == 1 {
								resolvedNodeName = knownHosts[0].name
							} else if info.Name != "" {
								resolvedNodeName = info.Name
							} else {
								resolvedNodeName = resolvedNodeID
							}
						}
					}

					serviceName := ""
					if cSummary.Labels != nil {
						if sn, ok := cSummary.Labels["com.docker.swarm.service.name"]; ok && sn != "" {
							serviceName = sn
						} else if sn, ok := cSummary.Labels["com.docker.compose.service"]; ok && sn != "" {
							serviceName = sn
						}
					}
					if serviceName == "" {
						serviceName = extractServiceNameFromContainerName(name)
					}

					cm := ContainerMetrics{
						ContainerID:   cSummary.ID,
						ContainerName: name,
						NodeID:        resolvedNodeID,
						NodeName:      resolvedNodeName,
						Image:         cSummary.Image,
						State:         string(cSummary.State),
						ServiceName:   serviceName,
						Labels:        cSummary.Labels,
					}

					if cSummary.State == "running" {
						statCtx, statCancel := context.WithTimeout(ctx, 2*time.Second)
						statsReader, err := c.dockerClient.ContainerStats(statCtx, cSummary.ID, false)
						if err == nil && statsReader.Body != nil {
							var stats container.StatsResponse
							if decodeErr := json.NewDecoder(statsReader.Body).Decode(&stats); decodeErr == nil {
								cm.CPUPercent = calculateCPUPercent(&stats)
								cm.MemoryUsed = int64(stats.MemoryStats.Usage)
								cm.MemoryLimit = int64(stats.MemoryStats.Limit)
								if cm.MemoryLimit > 0 {
									cm.MemoryPercent = math.Round((float64(cm.MemoryUsed)/float64(cm.MemoryLimit))*10000) / 100
								}
								var rx, tx int64
								for _, netStat := range stats.Networks {
									rx += int64(netStat.RxBytes)
									tx += int64(netStat.TxBytes)
								}
								cm.NetworkRx = rx
								cm.NetworkTx = tx
							}
							statsReader.Body.Close()
						}
						statCancel()
					}

					statCh <- rawContainerStat{cm: cm}
				}(cnt)
			}

			wg.Wait()
			close(statCh)

			for r := range statCh {
				containerMetricsList = append(containerMetricsList, r.cm)
				if r.cm.State == "running" {
					runningContainersCount++
				}
			}

			c.mu.Lock()
			elapsed := 0.0
			if !c.prevContainerTime.IsZero() {
				elapsed = now.Sub(c.prevContainerTime).Seconds()
			}

			currMap := make(map[string]containerNetRaw, len(containerMetricsList))
			for i := range containerMetricsList {
				cm := &containerMetricsList[i]
				currMap[cm.ContainerID] = containerNetRaw{
					rxBytes: cm.NetworkRx,
					txBytes: cm.NetworkTx,
				}
				if elapsed > 0 && cm.State == "running" {
					if prev, ok := c.prevContainerStats[cm.ContainerID]; ok && prev.rxBytes > 0 && cm.NetworkRx >= prev.rxBytes {
						rxDelta := cm.NetworkRx - prev.rxBytes
						if rxDelta > 0 {
							rate := int64(float64(rxDelta) / elapsed)
							if rate < 10*1024*1024*1024 {
								cm.NetworkRxRate = rate
							}
						}
					}
					if prev, ok := c.prevContainerStats[cm.ContainerID]; ok && prev.txBytes > 0 && cm.NetworkTx >= prev.txBytes {
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
			c.prevContainerStats = currMap
			c.prevContainerTime = now
			c.mu.Unlock()
		}
	}

	return containerMetricsList, runningContainersCount
}
