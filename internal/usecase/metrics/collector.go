package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/api/types/system"
	"go.uber.org/zap"
)

// NodeMetrics represents infrastructure metrics for a single node.
type NodeMetrics struct {
	NodeID         string    `json:"node_id"`
	NodeName       string    `json:"node_name"`
	Role           string    `json:"role"`   // manager, worker, standalone
	Status         string    `json:"status"` // ready, down, disconnected
	CPUPercent     float64   `json:"cpu_percent"`
	MemoryUsed     int64     `json:"memory_used"`   // bytes
	MemoryTotal    int64     `json:"memory_total"`  // bytes
	MemoryPercent  float64   `json:"memory_percent"`
	DiskUsed       int64     `json:"disk_used"`     // bytes
	DiskTotal      int64     `json:"disk_total"`    // bytes
	DiskPercent    float64   `json:"disk_percent"`
	NetworkRxBytes int64     `json:"network_rx_bytes"`
	NetworkTxBytes int64     `json:"network_tx_bytes"`
	ContainerCount int       `json:"container_count"`
	RunningCount   int       `json:"running_count"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ContainerMetrics represents resource stats for an individual container.
type ContainerMetrics struct {
	ContainerID   string  `json:"container_id"`
	ContainerName string  `json:"container_name"`
	NodeID        string  `json:"node_id"`
	Image         string  `json:"image"`
	State         string  `json:"state"`
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryUsed    int64   `json:"memory_used"`
	MemoryLimit   int64   `json:"memory_limit"`
	MemoryPercent float64 `json:"memory_percent"`
	NetworkRx     int64   `json:"network_rx"`
	NetworkTx     int64   `json:"network_tx"`
}

// SystemOverview aggregates high-level platform health, node and container metrics, and alerts.
type SystemOverview struct {
	Nodes             []NodeMetrics      `json:"nodes"`
	Containers        []ContainerMetrics `json:"containers"`
	TotalNodes        int                `json:"total_nodes"`
	HealthyNodes      int                `json:"healthy_nodes"`
	TotalContainers   int                `json:"total_containers"`
	RunningContainers int                `json:"running_containers"`
	TotalCPUPercent   float64            `json:"total_cpu_percent"`
	TotalMemPercent   float64            `json:"total_mem_percent"`
	TotalDiskPercent  float64            `json:"total_disk_percent"`
	RequestsPerSec    float64            `json:"requests_per_sec"`
	Alerts            []MetricAlert      `json:"alerts"`
	CollectedAt       time.Time          `json:"collected_at"`
}

// MetricAlert represents an active resource threshold or availability alert.
type MetricAlert struct {
	NodeID    string  `json:"node_id"`
	NodeName  string  `json:"node_name"`
	Type      string  `json:"type"` // cpu_high, memory_high, disk_high, node_down
	Message   string  `json:"message"`
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
}

// Thresholds configures warning limits for resource consumption.
type Thresholds struct {
	CPUWarning    float64 // default 80.0%
	MemoryWarning float64 // default 85.0%
	DiskWarning   float64 // default 90.0%
}

// Broadcaster is an interface for broadcasting messages across real-time channels (e.g. WebSockets).
type Broadcaster interface {
	Broadcast(msgType string, data interface{})
}

// DockerAPIClient defines the Docker API subset required for metrics polling.
type DockerAPIClient interface {
	ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	ContainerStats(ctx context.Context, containerID string, stream bool) (container.StatsResponseReader, error)
	NodeList(ctx context.Context, options swarm.NodeListOptions) ([]swarm.Node, error)
	Info(ctx context.Context) (system.Info, error)
	DiskUsage(ctx context.Context, options types.DiskUsageOptions) (types.DiskUsage, error)
}

// Collector periodically collects metrics from Docker daemon, tracks request rate, and broadcasts over WebSocket.
type Collector struct {
	dockerClient   DockerAPIClient
	broadcaster    Broadcaster
	logger         *zap.Logger
	interval       time.Duration
	thresholds     Thresholds
	requestCountFn func() int64

	lastSnapshot *SystemOverview
	lastReqCount int64
	lastReqTime  time.Time
	mu           sync.RWMutex
	stopCh       chan struct{}
}

// Option configures Collector behavior.
type Option func(*Collector)

// WithInterval sets the polling interval.
func WithInterval(d time.Duration) Option {
	return func(c *Collector) {
		if d > 0 {
			c.interval = d
		}
	}
}

// WithThresholds sets custom alerting thresholds.
func WithThresholds(t Thresholds) Option {
	return func(c *Collector) {
		c.thresholds = t
	}
}

// WithRequestCountFn sets the function to read total HTTP request count.
func WithRequestCountFn(fn func() int64) Option {
	return func(c *Collector) {
		c.requestCountFn = fn
	}
}

// NewCollector creates a new infrastructure metrics collector.
func NewCollector(dockerClient DockerAPIClient, broadcaster Broadcaster, logger *zap.Logger, opts ...Option) *Collector {
	if logger == nil {
		logger = zap.NewNop()
	}

	c := &Collector{
		dockerClient: dockerClient,
		broadcaster:  broadcaster,
		logger:       logger,
		interval:     5 * time.Second,
		thresholds: Thresholds{
			CPUWarning:    80.0,
			MemoryWarning: 85.0,
			DiskWarning:   90.0,
		},
		stopCh: make(chan struct{}),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Start runs the periodic metrics collection loop in a background goroutine until context is cancelled or Stop is called.
func (c *Collector) Start(ctx context.Context) {
	c.logger.Info("Starting Docker infrastructure metrics collector", zap.Duration("interval", c.interval))

	// Initial poll immediately
	c.runCollection(ctx)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("Stopping metrics collector (context cancelled)")
			return
		case <-c.stopCh:
			c.logger.Info("Stopping metrics collector (stop requested)")
			return
		case <-ticker.C:
			c.runCollection(ctx)
		}
	}
}

// Stop terminates the collector background loop.
func (c *Collector) Stop() {
	select {
	case <-c.stopCh:
	default:
		close(c.stopCh)
	}
}

// GetLastSnapshot returns the latest cached SystemOverview metrics snapshot.
func (c *Collector) GetLastSnapshot() *SystemOverview {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastSnapshot
}

func (c *Collector) runCollection(ctx context.Context) {
	timeout := c.interval - 500*time.Millisecond
	if timeout <= 0 {
		timeout = 4 * time.Second
	}
	collectCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	overview, err := c.CollectOnce(collectCtx)
	if err != nil {
		c.logger.Debug("Metrics collection completed with warnings", zap.Error(err))
	}

	if overview != nil {
		c.mu.Lock()
		c.lastSnapshot = overview
		c.mu.Unlock()

		if c.broadcaster != nil {
			c.broadcaster.Broadcast("metrics", overview)
		}
	}
}

// CollectOnce polls the Docker API once, builds the SystemOverview, and generates threshold alerts.
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

	// Graceful fallback if Docker client is unavailable
	if c.dockerClient == nil {
		overview := &SystemOverview{
			Nodes:             make([]NodeMetrics, 0),
			Containers:        make([]ContainerMetrics, 0),
			TotalNodes:        0,
			HealthyNodes:      0,
			TotalContainers:   0,
			RunningContainers: 0,
			TotalCPUPercent:   0,
			TotalMemPercent:   0,
			TotalDiskPercent:  0,
			RequestsPerSec:    math.Round(rps*100) / 100,
			Alerts:            make([]MetricAlert, 0),
			CollectedAt:       now,
		}
		return overview, nil
	}

	// 2. Fetch Docker Engine Info & Disk Usage
	info, infoErr := c.dockerClient.Info(ctx)
	if infoErr != nil {
		c.logger.Debug("Failed to fetch Docker daemon info", zap.Error(infoErr))
	}

	var diskUsed int64
	var diskTotal int64
	var diskPercent float64

	du, duErr := c.dockerClient.DiskUsage(ctx, types.DiskUsageOptions{})
	if duErr == nil {
		diskUsed += du.LayersSize
		for _, img := range du.Images {
			if img != nil {
				diskUsed += img.Size
			}
		}
		for _, cnt := range du.Containers {
			if cnt != nil {
				diskUsed += cnt.SizeRw
			}
		}
		for _, vol := range du.Volumes {
			if vol != nil && vol.UsageData != nil {
				diskUsed += vol.UsageData.Size
			}
		}
		for _, bc := range du.BuildCache {
			if bc != nil {
				diskUsed += bc.Size
			}
		}
	}

	if diskUsed > 0 {
		diskTotal = 100 * 1024 * 1024 * 1024 // 100 GiB baseline
		if diskUsed > diskTotal {
			diskTotal = diskUsed * 2
		}
		diskPercent = (float64(diskUsed) / float64(diskTotal)) * 100.0
	}

	// 3. Fetch Containers & Stats
	containersList, listErr := c.dockerClient.ContainerList(ctx, container.ListOptions{All: true})
	if listErr != nil {
		c.logger.Debug("Failed to list Docker containers", zap.Error(listErr))
	}

	containerMetricsList := make([]ContainerMetrics, 0, len(containersList))
	var totalContainerMemUsed int64
	var totalContainerCPUPercent float64
	var runningContainersCount int
	var totalNetRx, totalNetTx int64

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

			nodeID := ""
			if cSummary.Labels != nil {
				if nid, ok := cSummary.Labels["com.docker.swarm.node.id"]; ok {
					nodeID = nid
				}
			}
			if nodeID == "" {
				nodeID = info.ID
			}

			cm := ContainerMetrics{
				ContainerID:   cSummary.ID,
				ContainerName: name,
				NodeID:        nodeID,
				Image:         cSummary.Image,
				State:         string(cSummary.State),
			}

			if cSummary.State == "running" {
				statsReader, err := c.dockerClient.ContainerStats(ctx, cSummary.ID, false)
				if err == nil && statsReader.Body != nil {
					defer statsReader.Body.Close()
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
				}
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
		totalContainerMemUsed += r.cm.MemoryUsed
		totalContainerCPUPercent += r.cm.CPUPercent
		totalNetRx += r.cm.NetworkRx
		totalNetTx += r.cm.NetworkTx
	}

	// 4. Fetch Nodes (Swarm or Standalone fallback)
	nodeMetricsList := make([]NodeMetrics, 0)
	swarmNodes, nodeErr := c.dockerClient.NodeList(ctx, swarm.NodeListOptions{})

	if nodeErr == nil && len(swarmNodes) > 0 {
		for _, n := range swarmNodes {
			role := string(n.Spec.Role)
			status := string(n.Status.State)
			if status == "" {
				status = "ready"
			}

			memTotal := n.Description.Resources.MemoryBytes
			if memTotal == 0 && info.MemTotal > 0 {
				memTotal = info.MemTotal
			}

			var nodeContainerCount, nodeRunningCount int
			var nodeMemUsed int64
			var nodeCPUPercent float64
			var nodeNetRx, nodeNetTx int64

			for _, cm := range containerMetricsList {
				if cm.NodeID == n.ID || len(swarmNodes) == 1 {
					nodeContainerCount++
					if cm.State == "running" {
						nodeRunningCount++
					}
					nodeMemUsed += cm.MemoryUsed
					nodeCPUPercent += cm.CPUPercent
					nodeNetRx += cm.NetworkRx
					nodeNetTx += cm.NetworkTx
				}
			}

			var memPct float64
			if memTotal > 0 {
				memPct = (float64(nodeMemUsed) / float64(memTotal)) * 100.0
			}

			nodeMetricsList = append(nodeMetricsList, NodeMetrics{
				NodeID:         n.ID,
				NodeName:       n.Description.Hostname,
				Role:           role,
				Status:         status,
				CPUPercent:     math.Round(nodeCPUPercent*100) / 100,
				MemoryUsed:     nodeMemUsed,
				MemoryTotal:    memTotal,
				MemoryPercent:  math.Round(memPct*100) / 100,
				DiskUsed:       diskUsed,
				DiskTotal:      diskTotal,
				DiskPercent:    math.Round(diskPercent*100) / 100,
				NetworkRxBytes: nodeNetRx,
				NetworkTxBytes: nodeNetTx,
				ContainerCount: nodeContainerCount,
				RunningCount:   nodeRunningCount,
				UpdatedAt:      n.UpdatedAt,
			})
		}
	} else {
		// Standalone Docker Engine
		nodeName := info.Name
		if nodeName == "" {
			nodeName = "localhost"
		}
		nodeID := info.ID
		if nodeID == "" {
			nodeID = "local-node"
		}

		memTotal := info.MemTotal
		var memPct float64
		if memTotal > 0 {
			memPct = (float64(totalContainerMemUsed) / float64(memTotal)) * 100.0
		}

		nodeMetricsList = append(nodeMetricsList, NodeMetrics{
			NodeID:         nodeID,
			NodeName:       nodeName,
			Role:           "standalone",
			Status:         "ready",
			CPUPercent:     math.Round(totalContainerCPUPercent*100) / 100,
			MemoryUsed:     totalContainerMemUsed,
			MemoryTotal:    memTotal,
			MemoryPercent:  math.Round(memPct*100) / 100,
			DiskUsed:       diskUsed,
			DiskTotal:      diskTotal,
			DiskPercent:    math.Round(diskPercent*100) / 100,
			NetworkRxBytes: totalNetRx,
			NetworkTxBytes: totalNetTx,
			ContainerCount: len(containersList),
			RunningCount:   runningContainersCount,
			UpdatedAt:      now,
		})
	}

	// 5. Aggregate System Overview
	healthyNodesCount := 0
	var totalCPU float64
	var totalMemUsed, totalMemLimit int64

	for _, nm := range nodeMetricsList {
		if nm.Status == "ready" {
			healthyNodesCount++
		}
		totalCPU += nm.CPUPercent
		totalMemUsed += nm.MemoryUsed
		totalMemLimit += nm.MemoryTotal
	}

	var totalMemPct float64
	if totalMemLimit > 0 {
		totalMemPct = (float64(totalMemUsed) / float64(totalMemLimit)) * 100.0
	}

	avgCPU := totalCPU
	if len(nodeMetricsList) > 1 {
		avgCPU = totalCPU / float64(len(nodeMetricsList))
	}

	overview := &SystemOverview{
		Nodes:             nodeMetricsList,
		Containers:        containerMetricsList,
		TotalNodes:        len(nodeMetricsList),
		HealthyNodes:      healthyNodesCount,
		TotalContainers:   len(containersList),
		RunningContainers: runningContainersCount,
		TotalCPUPercent:   math.Round(avgCPU*100) / 100,
		TotalMemPercent:   math.Round(totalMemPct*100) / 100,
		TotalDiskPercent:  math.Round(diskPercent*100) / 100,
		RequestsPerSec:    math.Round(rps*100) / 100,
		Alerts:            make([]MetricAlert, 0),
		CollectedAt:       now,
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
