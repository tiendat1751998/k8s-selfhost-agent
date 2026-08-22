package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
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
	Hostname      string          `json:"hostname"`
	OS            string          `json:"os"`
	Arch          string          `json:"arch"`
	OSDistro      string          `json:"os_distro,omitempty"`
	KernelVersion string          `json:"kernel_version,omitempty"`
	CPUUsage      float64         `json:"cpu_usage"`
	CPUCount      int             `json:"cpu_count"`
	MemTotal      int64           `json:"mem_total"`
	MemUsed       int64           `json:"mem_used"`
	MemPercent    float64         `json:"mem_percent"`
	DiskTotal     int64           `json:"disk_total"`
	DiskUsed      int64           `json:"disk_used"`
	DiskPercent   float64         `json:"disk_percent"`
	NetRxRate     int64           `json:"net_rx_rate"`
	NetTxRate     int64           `json:"net_tx_rate"`
	Uptime        int64           `json:"uptime"`
	LoadAvg       [3]float64      `json:"load_avg"`
	Processes     int             `json:"processes"`
	TopProcesses  []ProcessMetric `json:"top_processes"`
	Status        string          `json:"status"` // "online", "offline", "error"
	LastSeen      time.Time       `json:"last_seen"`
}

// NodeMetrics represents infrastructure metrics for a single node.
type NodeMetrics struct {
	NodeID         string          `json:"node_id"`
	NodeName       string          `json:"node_name"`
	Role           string          `json:"role"`   // manager, worker, standalone, agent
	Status         string          `json:"status"` // ready, down, disconnected
	OS             string          `json:"os,omitempty"`
	Arch           string          `json:"arch,omitempty"`
	OSDistro       string          `json:"os_distro,omitempty"`
	KernelVersion  string          `json:"kernel_version,omitempty"`
	CPUPercent     float64         `json:"cpu_percent"`
	MemoryUsed     int64           `json:"memory_used"`   // bytes
	MemoryTotal    int64           `json:"memory_total"`  // bytes
	MemoryPercent  float64         `json:"memory_percent"`
	DiskUsed       int64           `json:"disk_used"`     // bytes
	DiskTotal      int64           `json:"disk_total"`    // bytes
	DiskPercent    float64         `json:"disk_percent"`
	NetworkRxBytes int64           `json:"network_rx_bytes"`
	NetworkTxBytes int64           `json:"network_tx_bytes"`
	ContainerCount int             `json:"container_count"`
	RunningCount   int             `json:"running_count"`
	UptimeSeconds  int64           `json:"uptime_seconds,omitempty"`
	LoadAverage    [3]float64      `json:"load_average,omitempty"`
	TopProcesses   []ProcessMetric `json:"top_processes"`
	Source         string          `json:"source"` // "docker" or "agent"
	UpdatedAt      time.Time       `json:"updated_at"`
}

// ContainerMetrics represents resource stats for an individual container.
type ContainerMetrics struct {
	ContainerID   string            `json:"container_id"`
	ContainerName string            `json:"container_name"`
	NodeID        string            `json:"node_id"`
	Image         string            `json:"image"`
	State         string            `json:"state"`
	CPUPercent    float64           `json:"cpu_percent"`
	MemoryUsed    int64             `json:"memory_used"`
	MemoryLimit   int64             `json:"memory_limit"`
	MemoryPercent float64           `json:"memory_percent"`
	NetworkRx     int64             `json:"network_rx"`
	NetworkTx     int64             `json:"network_tx"`
	ServiceName   string            `json:"service_name,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
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
		dockerClient:    dockerClient,
		computeHostRepo: computeHostRepo,
		httpClient:      &http.Client{Timeout: 5 * time.Second},
		agentMetrics:    make(map[string]*AgentMetrics),
		broadcaster:     broadcaster,
		logger:          logger,
		interval:        5 * time.Second,
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

	// Initial poll of agents and docker immediately
	go c.pollAgentHosts(ctx)
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

// pollAgentHosts periodically polls all registered compute host agents.
func (c *Collector) pollAgentHosts(ctx context.Context) {
	if c.computeHostRepo == nil {
		return
	}

	c.scrapeAllAgents(ctx)

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

// ScrapeAgent scrapes a single registered compute host agent.
func (c *Collector) ScrapeAgent(ctx context.Context, host docker.ComputeHost) {
	url := formatAgentURL(host.Endpoint, host.TLSEnabled)
	if url == "" {
		return
	}

	reqCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		c.recordAgentFailure(ctx, host, err)
		return
	}

	if host.Labels != nil {
		if token, ok := host.Labels["auth_token"]; ok && token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		} else if token, ok := host.Labels["token"]; ok && token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}

	client := c.httpClient
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		c.recordAgentFailure(ctx, host, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.recordAgentFailure(ctx, host, fmt.Errorf("unexpected HTTP status: %d", resp.StatusCode))
		return
	}

	type agentDiskPayload struct {
		MountPoint   string  `json:"mount_point"`
		TotalBytes   int64   `json:"total_bytes"`
		UsedBytes    int64   `json:"used_bytes"`
		UsagePercent float64 `json:"usage_percent"`
		Filesystem   string  `json:"filesystem"`
	}

	type agentMetricsPayload struct {
		Hostname      string             `json:"hostname"`
		OS            string             `json:"os"`
		Arch          string             `json:"arch"`
		OSDistro      string             `json:"os_distro"`
		KernelVersion string             `json:"kernel_version"`
		UptimeSeconds int64              `json:"uptime_seconds"`
		LoadAverage   [3]float64         `json:"load_average"`
		CPU           struct {
			Count        int     `json:"count"`
			UsagePercent float64 `json:"usage_percent"`
		} `json:"cpu"`
		Memory struct {
			TotalBytes     int64   `json:"total_bytes"`
			UsedBytes      int64   `json:"used_bytes"`
			AvailableBytes int64   `json:"available_bytes"`
			UsagePercent   float64 `json:"usage_percent"`
		} `json:"memory"`
		Disks   []agentDiskPayload `json:"disks"`
		Network struct {
			TotalRxBytesPerSec int64 `json:"total_rx_bytes_per_sec"`
			TotalTxBytesPerSec int64 `json:"total_tx_bytes_per_sec"`
		} `json:"network"`
		Processes    int             `json:"processes"`
		TopProcesses []ProcessMetric `json:"top_processes"`
		CollectedAt  time.Time       `json:"collected_at"`
	}

	var payload agentMetricsPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		c.recordAgentFailure(ctx, host, fmt.Errorf("decoding agent response: %w", err))
		return
	}

	var diskTotal, diskUsed int64
	seenDisks := make(map[string]bool)
	for _, d := range payload.Disks {
		if isIgnoredMountPoint(d.MountPoint) || !isPhysicalFilesystem(d.Filesystem) {
			continue
		}
		dedupKey := fmt.Sprintf("%s:%d", strings.ToLower(d.Filesystem), d.TotalBytes)
		if seenDisks[dedupKey] {
			continue
		}
		seenDisks[dedupKey] = true
		diskTotal += d.TotalBytes
		diskUsed += d.UsedBytes
	}
	var diskPercent float64
	if diskTotal > 0 {
		diskPercent = math.Round((float64(diskUsed)/float64(diskTotal))*10000) / 100
	}

	now := time.Now().UTC()
	collectedAt := payload.CollectedAt
	if collectedAt.IsZero() {
		collectedAt = now
	}

	hostname := payload.Hostname
	if hostname == "" {
		hostname = host.Name
	}

	am := &AgentMetrics{
		Hostname:      hostname,
		OS:            payload.OS,
		Arch:          payload.Arch,
		OSDistro:      payload.OSDistro,
		KernelVersion: payload.KernelVersion,
		CPUUsage:      payload.CPU.UsagePercent,
		CPUCount:      payload.CPU.Count,
		MemTotal:      payload.Memory.TotalBytes,
		MemUsed:       payload.Memory.UsedBytes,
		MemPercent:    payload.Memory.UsagePercent,
		DiskTotal:     diskTotal,
		DiskUsed:      diskUsed,
		DiskPercent:   diskPercent,
		NetRxRate:     payload.Network.TotalRxBytesPerSec,
		NetTxRate:     payload.Network.TotalTxBytesPerSec,
		Uptime:        payload.UptimeSeconds,
		LoadAvg:       payload.LoadAverage,
		Processes:     payload.Processes,
		TopProcesses:  payload.TopProcesses,
		Status:        "online",
		LastSeen:      collectedAt,
	}

	c.agentMu.Lock()
	c.agentMetrics[host.ID] = am
	c.agentMu.Unlock()

	if c.computeHostRepo != nil {
		_ = c.computeHostRepo.UpdateStatus(ctx, host.ID, "connected", now)
	}

	if c.incRepo != nil {
		activeInc, getErr := c.incRepo.GetByPodAndType(ctx, "infrastructure", host.Name, incident.TypeNodeNotReady)
		if getErr != nil {
			c.logger.Warn("Failed to query active incident on agent recovery", zap.String("host", host.Name), zap.Error(getErr))
		} else if activeInc != nil {
			if activeInc.Status == incident.StatusDetected || activeInc.Status == incident.StatusAnalyzing || activeInc.Status == incident.StatusRemediating {
				if resErr := activeInc.MarkResolved(); resErr == nil {
					if updateErr := c.incRepo.Update(ctx, activeInc); updateErr == nil {
						if c.broadcaster != nil {
							c.broadcaster.Broadcast("incident_resolved", activeInc)
						}
					} else {
						c.logger.Error("Failed to update resolved incident on agent recovery", zap.String("host", host.Name), zap.Error(updateErr))
					}
				}
			}
		}
	}
}

func (c *Collector) recordAgentFailure(ctx context.Context, host docker.ComputeHost, err error) {
	c.logger.Debug("Agent scrape failed", zap.String("host_id", host.ID), zap.String("host_name", host.Name), zap.Error(err))

	now := time.Now().UTC()
	c.agentMu.Lock()
	am, exists := c.agentMetrics[host.ID]
	if !exists {
		am = &AgentMetrics{
			Hostname: host.Name,
			Status:   "offline",
			LastSeen: now,
		}
		c.agentMetrics[host.ID] = am
	} else {
		am.Status = "offline"
		am.LastSeen = now
	}
	c.agentMu.Unlock()

	if c.computeHostRepo != nil {
		_ = c.computeHostRepo.UpdateStatus(ctx, host.ID, "disconnected", now)
	}

	if c.incRepo != nil {
		activeInc, getErr := c.incRepo.GetByPodAndType(ctx, "infrastructure", host.Name, incident.TypeNodeNotReady)
		if getErr != nil {
			c.logger.Warn("Failed to query existing incident on agent failure", zap.String("host", host.Name), zap.Error(getErr))
		}
		if activeInc == nil || activeInc.Status == incident.StatusResolved || activeInc.Status == incident.StatusFailed {
			newInc, newErr := incident.New("fleet-primary", "infrastructure", host.Name, incident.TypeNodeNotReady, incident.SeverityCritical, fmt.Sprintf("Infrastructure host '%s' is unreachable: agent at %s is down (%v)", host.Name, host.Endpoint, err))
			if newErr == nil {
				if saveErr := c.incRepo.Create(ctx, newInc); saveErr == nil {
					if c.broadcaster != nil {
						c.broadcaster.Broadcast("incident", newInc)
					}
				} else {
					c.logger.Error("Failed to create incident on agent failure", zap.String("host", host.Name), zap.Error(saveErr))
				}
			}
		}
	}
}

// GetAgentMetrics returns a copy of the current agent metrics.
func (c *Collector) GetAgentMetrics() map[string]*AgentMetrics {
	c.agentMu.RLock()
	defer c.agentMu.RUnlock()
	res := make(map[string]*AgentMetrics, len(c.agentMetrics))
	for k, v := range c.agentMetrics {
		if v != nil {
			metricCopy := *v
			if v.TopProcesses != nil {
				metricCopy.TopProcesses = append([]ProcessMetric(nil), v.TopProcesses...)
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

var allowedFileSystems = map[string]bool{
	"ext4":    true,
	"ext3":    true,
	"ext2":    true,
	"xfs":     true,
	"btrfs":   true,
	"zfs":     true,
	"ntfs":    true,
	"vfat":    true,
	"fat32":   true,
	"exfat":   true,
	"apfs":    true,
	"hfsplus": true,
}

var ignoredFileSystems = map[string]bool{
	"overlay":       true,
	"overlayfs":     true,
	"tmpfs":         true,
	"devtmpfs":      true,
	"squashfs":      true,
	"proc":          true,
	"sysfs":         true,
	"cgroup":        true,
	"cgroup2":       true,
	"fuse.snapfuse": true,
	"devpts":        true,
	"pstore":        true,
	"bpf":           true,
	"autofs":        true,
	"mqueue":        true,
	"hugetlbfs":     true,
	"debugfs":       true,
	"tracefs":       true,
	"fusectl":       true,
	"configfs":      true,
	"binfmt_misc":   true,
	"nsfs":          true,
	"securityfs":    true,
	"efivarfs":      true,
	"ramfs":         true,
	"none":          true,
}

var ignoredMountPrefixes = []string{
	"/var/lib/docker/",
	"/snap/",
	"/sys/",
	"/proc/",
	"/dev/",
	"/run/",
}

func isIgnoredMountPoint(mountPoint string) bool {
	clean := strings.ReplaceAll(strings.TrimSpace(mountPoint), "\\", "/")
	if clean == "" {
		return true
	}
	for _, prefix := range ignoredMountPrefixes {
		if strings.HasPrefix(clean, prefix) || clean == strings.TrimSuffix(prefix, "/") {
			return true
		}
	}
	return false
}

func isPhysicalFilesystem(fsType string) bool {
	fs := strings.ToLower(strings.TrimSpace(fsType))
	if fs == "" {
		return true
	}
	if ignoredFileSystems[fs] {
		return false
	}
	return allowedFileSystems[fs]
}

func formatAgentURL(endpoint string, tlsEnabled bool) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return ""
	}
	var u string
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		u = endpoint
	} else if tlsEnabled {
		u = "https://" + endpoint
	} else {
		u = "http://" + endpoint
	}
	if !strings.HasSuffix(u, "/metrics") {
		u = strings.TrimSuffix(u, "/") + "/metrics"
	}
	return u
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

// CollectOnce polls Docker containers, merges agent host metrics, builds SystemOverview, and generates alerts.
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
	var diskPercent float64

	// 2. Fetch Docker Containers & Stats if Docker client is available
	if c.dockerClient != nil {
		info, infoErr := c.dockerClient.Info(ctx)
		if infoErr != nil {
			c.logger.Debug("Failed to fetch Docker daemon info", zap.Error(infoErr))
		}

		var diskUsed int64
		var diskTotal int64
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

		containersList, listErr := c.dockerClient.ContainerList(ctx, container.ListOptions{All: true})
		if listErr != nil {
			c.logger.Debug("Failed to list Docker containers", zap.Error(listErr))
		}

		if len(containersList) > 0 {
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
						NodeID:        nodeID,
						Image:         cSummary.Image,
						State:         string(cSummary.State),
						ServiceName:   serviceName,
						Labels:        cSummary.Labels,
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
			}
		}
	}

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

		nodeMetricsList = append(nodeMetricsList, NodeMetrics{
			NodeID:         hostID,
			NodeName:       am.Hostname,
			Role:           "agent",
			Status:         status,
			OS:             am.OS,
			Arch:           am.Arch,
			OSDistro:       am.OSDistro,
			KernelVersion:  am.KernelVersion,
			CPUPercent:     am.CPUUsage,
			MemoryUsed:     am.MemUsed,
			MemoryTotal:    am.MemTotal,
			MemoryPercent:  am.MemPercent,
			DiskUsed:       am.DiskUsed,
			DiskTotal:      am.DiskTotal,
			DiskPercent:    am.DiskPercent,
			NetworkRxBytes: am.NetRxRate,
			NetworkTxBytes: am.NetTxRate,
			ContainerCount: am.Processes,
			RunningCount:   am.Processes,
			UptimeSeconds:  am.Uptime,
			LoadAverage:    am.LoadAvg,
			TopProcesses:   am.TopProcesses,
			Source:         "agent",
			UpdatedAt:      am.LastSeen,
		})
	}
	c.agentMu.RUnlock()

	// 4. Aggregate System Overview
	healthyNodesCount := 0
	var totalCPU float64
	var totalMemUsed, totalMemLimit int64
	var totalDiskUsed, totalDiskLimit int64

	for _, nm := range nodeMetricsList {
		if nm.Status == "ready" {
			healthyNodesCount++
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
		Nodes:             nodeMetricsList,
		Containers:        containerMetricsList,
		TotalNodes:        len(nodeMetricsList),
		HealthyNodes:      healthyNodesCount,
		TotalContainers:   len(containerMetricsList),
		RunningContainers: runningContainersCount,
		TotalCPUPercent:   math.Round(avgCPU*100) / 100,
		TotalMemPercent:   math.Round(totalMemPct*100) / 100,
		TotalDiskPercent:  math.Round(totalDiskPct*100) / 100,
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

