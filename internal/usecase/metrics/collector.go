package metrics

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/api/types/system"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/domain/provider/docker"
)

// ProcessMetric holds resource consumption details for a single OS process.
type ProcessMetric struct {
	PID              int     `json:"pid"`
	Name             string  `json:"name"`
	CommandLine      string  `json:"command_line"`
	User             string  `json:"user"`
	CPUPercent       float64 `json:"cpu_percent"`
	MemoryBytes      int64   `json:"memory_bytes"`
	MemoryPercent    float64 `json:"memory_percent"`
	ReadBytesPerSec  int64   `json:"read_bytes_per_sec"`
	WriteBytesPerSec int64   `json:"write_bytes_per_sec"`
	State            string  `json:"state"`
}

// AgentMetrics represents metrics from a k8s-agent instance.
type AgentMetrics struct {
	Hostname          string             `json:"hostname"`
	OS                string             `json:"os"`
	Arch              string             `json:"arch"`
	OSDistro          string             `json:"os_distro"`
	KernelVersion     string             `json:"kernel_version"`
	CPUUsage          float64            `json:"cpu_usage"`
	CPUCount          int                `json:"cpu_count"`
	MemTotal          int64              `json:"mem_total"`
	MemUsed           int64              `json:"mem_used"`
	MemPercent        float64            `json:"mem_percent"`
	DiskTotal         int64              `json:"disk_total"`
	DiskUsed          int64              `json:"disk_used"`
	DiskPercent       float64            `json:"disk_percent"`
	DiskIO            DiskIOMetrics      `json:"disk_io"`
	NetRxRate         int64              `json:"net_rx_rate"`
	NetTxRate         int64              `json:"net_tx_rate"`
	NetworkInterfaces []NetworkInterface `json:"network_interfaces,omitempty"`
	Uptime            int64              `json:"uptime"`
	LoadAvg           [3]float64         `json:"load_avg"`
	Processes         int                `json:"processes"`
	TopProcesses      []ProcessMetric    `json:"top_processes"`
	Status            string             `json:"status"` // "online", "offline", "error"
	LastSeen          time.Time          `json:"last_seen"`
}

// NodeMetrics represents infrastructure metrics for a single node.
type NodeMetrics struct {
	NodeID               string             `json:"node_id"`
	NodeName             string             `json:"node_name"`
	Role                 string             `json:"role"`   // manager, worker, standalone, agent
	Status               string             `json:"status"` // ready, down, disconnected
	OS                   string             `json:"os"`
	Arch                 string             `json:"arch"`
	OSDistro             string             `json:"os_distro"`
	KernelVersion        string             `json:"kernel_version"`
	CPUPercent           float64            `json:"cpu_percent"`
	MemoryUsed           int64              `json:"memory_used"`   // bytes
	MemoryTotal          int64              `json:"memory_total"`  // bytes
	MemoryPercent        float64            `json:"memory_percent"`
	DiskUsed             int64              `json:"disk_used"`     // bytes
	DiskTotal            int64              `json:"disk_total"`    // bytes
	DiskPercent          float64            `json:"disk_percent"`
	DiskReadBytesPerSec  int64              `json:"disk_read_bytes_per_sec"`
	DiskWriteBytesPerSec int64              `json:"disk_write_bytes_per_sec"`
	DiskReadIOPS         float64            `json:"disk_read_iops"`
	DiskWriteIOPS        float64            `json:"disk_write_iops"`
	DiskAvgAwaitMs       float64            `json:"disk_avg_await_ms"`
	DiskMaxIoUtilPct     float64            `json:"disk_max_io_util_pct"`
	DiskDevices          []DiskIOStats      `json:"disk_devices,omitempty"`
	NetworkRxBytes       int64              `json:"network_rx_bytes"`
	NetworkTxBytes       int64              `json:"network_tx_bytes"`
	NetworkInterfaces    []NetworkInterface `json:"network_interfaces,omitempty"`
	ContainerCount       int                `json:"container_count"`
	RunningCount         int                `json:"running_count"`
	Processes            int                `json:"processes,omitempty"`
	UptimeSeconds        int64              `json:"uptime_seconds,omitempty"`
	LoadAverage          [3]float64         `json:"load_average,omitempty"`
	TopProcesses         []ProcessMetric    `json:"top_processes"`
	Source               string             `json:"source"` // "docker" or "agent"
	UpdatedAt            time.Time          `json:"updated_at"`
}

// ContainerMetrics represents resource stats for an individual container.
type ContainerMetrics struct {
	ContainerID   string            `json:"container_id"`
	ContainerName string            `json:"container_name"`
	NodeID        string            `json:"node_id"`
	NodeName      string            `json:"node_name,omitempty"`
	Image         string            `json:"image"`
	State         string            `json:"state"`
	CPUPercent    float64           `json:"cpu_percent"`
	MemoryUsed    int64             `json:"memory_used"`
	MemoryLimit   int64             `json:"memory_limit"`
	MemoryPercent float64           `json:"memory_percent"`
	NetworkRx     int64             `json:"network_rx"`
	NetworkTx     int64             `json:"network_tx"`
	NetworkRxRate int64             `json:"network_rx_rate,omitempty"` // Real-time Rx rate in Bytes/sec
	NetworkTxRate int64             `json:"network_tx_rate,omitempty"` // Real-time Tx rate in Bytes/sec
	ServiceName   string            `json:"service_name,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
}

// SystemOverview aggregates high-level platform health, node and container metrics, and alerts.
type SystemOverview struct {
	Nodes                     []NodeMetrics      `json:"nodes"`
	Containers                []ContainerMetrics `json:"containers"`
	TotalNodes                int                `json:"total_nodes"`
	HealthyNodes              int                `json:"healthy_nodes"`
	TotalContainers           int                `json:"total_containers"`
	RunningContainers         int                `json:"running_containers"`
	TotalCPUPercent           float64            `json:"total_cpu_percent"`
	TotalMemPercent           float64            `json:"total_mem_percent"`
	TotalDiskPercent          float64            `json:"total_disk_percent"`
	TotalDiskReadBytesPerSec  int64              `json:"total_disk_read_bytes_per_sec"`
	TotalDiskWriteBytesPerSec int64              `json:"total_disk_write_bytes_per_sec"`
	RequestsPerSec            float64            `json:"requests_per_sec"`
	Alerts                    []MetricAlert      `json:"alerts"`
	CollectedAt               time.Time          `json:"collected_at"`
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

// SwarmNodeLister defines optional Swarm node listing support on Docker client.
type SwarmNodeLister interface {
	NodeList(ctx context.Context, options swarm.NodeListOptions) ([]swarm.Node, error)
}

// DockerAPIClient defines the Docker API subset required for metrics polling.
type DockerAPIClient interface {
	ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	ContainerStats(ctx context.Context, containerID string, stream bool) (container.StatsResponseReader, error)
	Info(ctx context.Context) (system.Info, error)
	DiskUsage(ctx context.Context, options types.DiskUsageOptions) (types.DiskUsage, error)
}

// Collector periodically collects metrics from Docker daemon, tracks request rate, and broadcasts over WebSocket.
type Collector struct {
	dockerClient    DockerAPIClient
	computeHostRepo docker.ComputeHostRepository
	incRepo         incident.Repository
	httpClient      *http.Client
	agentMetrics    map[string]*AgentMetrics
	agentMu         sync.RWMutex
	broadcaster     Broadcaster
	logger          *zap.Logger
	interval        time.Duration
	thresholds      Thresholds
	requestCountFn  func() int64

	dockerDiskPercent float64
	dockerDiskMu      sync.RWMutex

	lastSnapshot       *SystemOverview
	lastReqCount       int64
	lastReqTime        time.Time
	prevContainerStats map[string]containerNetRaw
	prevContainerTime  time.Time
	mu                 sync.RWMutex
	stopCh             chan struct{}
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

// WithHTTPClient sets a custom HTTP client for agent scraping.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Collector) {
		if client != nil {
			c.httpClient = client
		}
	}
}

// WithComputeHostRepo sets the compute host repository.
func WithComputeHostRepo(repo docker.ComputeHostRepository) Option {
	return func(c *Collector) {
		c.computeHostRepo = repo
	}
}

// WithIncidentRepo sets the incident repository for auto-incident management on agent failure and recovery.
func WithIncidentRepo(repo incident.Repository) Option {
	return func(c *Collector) {
		c.incRepo = repo
	}
}

// NewCollector creates a new infrastructure metrics collector.
func NewCollector(dockerClient DockerAPIClient, computeHostRepo docker.ComputeHostRepository, broadcaster Broadcaster, logger *zap.Logger, opts ...Option) *Collector {
	if logger == nil {
		logger = zap.NewNop()
	}

	c := &Collector{
		dockerClient:       dockerClient,
		computeHostRepo:    computeHostRepo,
		httpClient:         &http.Client{Timeout: 5 * time.Second},
		agentMetrics:       make(map[string]*AgentMetrics),
		broadcaster:        broadcaster,
		logger:             logger,
		interval:           5 * time.Second,
		prevContainerStats: make(map[string]containerNetRaw),
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

	// Initial scrape of agents before running initial collection snapshot
	if c.computeHostRepo != nil {
		c.scrapeAllAgents(ctx)
	}

	if c.dockerClient != nil {
		go c.pollDockerDiskUsage(ctx)
	}

	c.runCollection(ctx)

	go c.pollAgentHosts(ctx)

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

// SetLastSnapshot sets the cached SystemOverview metrics snapshot (useful for testing or initial hydration).
func (c *Collector) SetLastSnapshot(s *SystemOverview) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastSnapshot = s
}


// pollAgentHosts periodically polls all registered compute host agents.
func (c *Collector) pollAgentHosts(ctx context.Context) {
	if c.computeHostRepo == nil {
		return
	}

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.scrapeAllAgents(ctx)
			c.runCollection(ctx)
		}
	}
}

func (c *Collector) scrapeAllAgents(ctx context.Context) {
	if c.computeHostRepo == nil {
		return
	}

	hosts, err := c.computeHostRepo.ListAll(ctx)
	if err != nil {
		c.logger.Debug("Failed to list compute hosts for agent scraping", zap.Error(err))
		return
	}

	currentHostIDs := make(map[string]bool, len(hosts))
	for _, host := range hosts {
		currentHostIDs[host.ID] = true
	}

	var wg sync.WaitGroup
	for _, host := range hosts {
		wg.Add(1)
		go func(h docker.ComputeHost) {
			defer wg.Done()
			c.ScrapeAgent(ctx, h)
		}(host)
	}
	wg.Wait()

	c.agentMu.Lock()
	for hostID := range c.agentMetrics {
		if !currentHostIDs[hostID] {
			delete(c.agentMetrics, hostID)
		}
	}
	c.agentMu.Unlock()
}

// GetAgentMetrics returns a copy of the current agent metrics.
func (c *Collector) GetAgentMetrics() map[string]*AgentMetrics {
	c.agentMu.RLock()
	defer c.agentMu.RUnlock()
	res := make(map[string]*AgentMetrics, len(c.agentMetrics))
	for k, v := range c.agentMetrics {
		if v != nil {
			metricCopy := *v
			if len(v.TopProcesses) > 0 {
				metricCopy.TopProcesses = append([]ProcessMetric(nil), v.TopProcesses...)
			} else {
				metricCopy.TopProcesses = make([]ProcessMetric, 0)
			}
			if len(v.NetworkInterfaces) > 0 {
				metricCopy.NetworkInterfaces = append([]NetworkInterface(nil), v.NetworkInterfaces...)
			} else {
				metricCopy.NetworkInterfaces = make([]NetworkInterface, 0)
			}
			if len(v.DiskIO.Devices) > 0 {
				metricCopy.DiskIO.Devices = append([]DiskIOStats(nil), v.DiskIO.Devices...)
			} else {
				metricCopy.DiskIO.Devices = make([]DiskIOStats, 0)
			}
			res[k] = &metricCopy
		}
	}
	return res
}

// SetAgentMetric sets an agent metric entry (useful for tests).
func (c *Collector) SetAgentMetric(hostID string, m *AgentMetrics) {
	c.agentMu.Lock()
	defer c.agentMu.Unlock()
	c.agentMetrics[hostID] = m
}

// RemoveAgentMetric removes an agent metric entry by hostID.
func (c *Collector) RemoveAgentMetric(hostID string) {
	c.agentMu.Lock()
	defer c.agentMu.Unlock()
	delete(c.agentMetrics, hostID)
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


