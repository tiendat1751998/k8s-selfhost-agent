package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// DiskUsageFunc resolves disk metrics and inode stats for a mount point.
type DiskUsageFunc func(mountPoint string) (totalBytes, usedBytes, inodesTotal, inodesFree int64, err error)

// SystemCollector collects local system metrics from Linux /proc (with cross-platform fallbacks).
type SystemCollector struct {
	procPath      string
	mountsPath    string
	osReleasePath string
	diskUsageFn   DiskUsageFunc
	interval      time.Duration
	prevCPUTotal  uint64
	prevCPUActive uint64
	prevDiskStats map[string]diskDevSnapshot
	prevDiskTime  time.Time
	prevNetStats  map[string]netDevStat
	prevNetTime   time.Time
	prevProcStats map[int]procStatSnapshot
	prevProcTime  time.Time
	lastCPUDiff   uint64
	lastCPUCount  int
	startTime     time.Time
	lastMetrics   *MetricsResponse
	collectMu     sync.Mutex
	mu            sync.RWMutex
	stopCh        chan struct{}
	stopOnce      sync.Once
}

// CollectorOption configures SystemCollector.
type CollectorOption func(*SystemCollector)

// WithProcPath configures the /proc root directory (useful for testing with fixtures).
func WithProcPath(p string) CollectorOption {
	return func(c *SystemCollector) {
		if p != "" {
			c.procPath = p
		}
	}
}

// WithMountsPath configures the mounts file path (useful for testing).
func WithMountsPath(m string) CollectorOption {
	return func(c *SystemCollector) {
		if m != "" {
			c.mountsPath = m
		}
	}
}

// WithOSReleasePath configures the os-release file path (useful for testing).
func WithOSReleasePath(p string) CollectorOption {
	return func(c *SystemCollector) {
		if p != "" {
			c.osReleasePath = p
		}
	}
}

// WithDiskUsageFunc overrides the disk stat retrieval function.
func WithDiskUsageFunc(fn DiskUsageFunc) CollectorOption {
	return func(c *SystemCollector) {
		if fn != nil {
			c.diskUsageFn = fn
		}
	}
}

// WithCollectionInterval configures background polling frequency.
func WithCollectionInterval(d time.Duration) CollectorOption {
	return func(c *SystemCollector) {
		if d > 0 {
			c.interval = d
		}
	}
}

// NewSystemCollector initializes a new SystemCollector.
func NewSystemCollector(procPath, mountsPath string, diskUsageFn DiskUsageFunc, opts ...CollectorOption) *SystemCollector {
	if procPath == "" {
		procPath = "/proc"
	}
	if mountsPath == "" {
		mountsPath = filepath.Join(procPath, "mounts")
	}
	if diskUsageFn == nil {
		diskUsageFn = getDiskUsage
	}

	c := &SystemCollector{
		procPath:      procPath,
		mountsPath:    mountsPath,
		osReleasePath: "/etc/os-release",
		diskUsageFn:   diskUsageFn,
		interval:      5 * time.Second,
		prevDiskStats: make(map[string]diskDevSnapshot),
		prevNetStats:  make(map[string]netDevStat),
		prevProcStats: make(map[int]procStatSnapshot),
		startTime:     time.Now().UTC(),
		stopCh:        make(chan struct{}),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Collect executes a single metric collection pass and updates the cached snapshot.
func (c *SystemCollector) Collect() (*MetricsResponse, error) {
	c.collectMu.Lock()
	defer c.collectMu.Unlock()

	now := time.Now().UTC()

	// 1. Hostname, OS, Arch, Distro, Kernel
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "unknown"
	}
	osName := runtime.GOOS
	arch := runtime.GOARCH
	osDistro, kernelVersion := c.collectOSInfo()

	// 2. Uptime
	uptime := c.collectUptime()

	// 3. Load average
	loadAvg := c.collectLoadAvg()

	// 4. CPU
	cpuMetrics := c.collectCPU()

	// 5. Memory
	memMetrics := c.collectMemory()

	// 6. Disks
	diskMetrics := c.collectDisks()

	// 6b. Disk I/O
	diskIOMetrics := c.collectDiskIO(now)

	// 7. Network
	netMetrics := c.collectNetwork(now)

	// 8. Processes
	procCount := c.collectProcesses()

	// 9. Top Processes
	topProcesses := c.readTopProcesses(10, memMetrics.TotalBytes)

	// 10. Runtime environment and host role detection
	runtimeEnv := detectRuntimeEnvironment()
	hostRole, detectedServices := detectHostRoleAndServices(topProcesses)

	resp := &MetricsResponse{
		Hostname:           hostname,
		OS:                 osName,
		Arch:               arch,
		OSDistro:           osDistro,
		KernelVersion:      kernelVersion,
		RuntimeEnvironment: runtimeEnv,
		HostRole:           hostRole,
		DetectedServices:   detectedServices,
		UptimeSeconds:      uptime,
		LoadAverage:        loadAvg,
		CPU:                cpuMetrics,
		Memory:             memMetrics,
		Disks:              diskMetrics,
		DiskIO:             diskIOMetrics,
		Network:            netMetrics,
		Processes:          procCount,
		TopProcesses:       topProcesses,
		PSI:                c.collectPSI(),
		K8s:                c.collectK8sMetrics(),
		CollectedAt:        now,
	}

	c.mu.Lock()
	c.lastMetrics = resp
	c.mu.Unlock()

	return resp, nil
}

// GetLastMetrics returns the most recently collected system metrics.
func (c *SystemCollector) GetLastMetrics() *MetricsResponse {
	c.mu.RLock()
	metrics := c.lastMetrics
	c.mu.RUnlock()

	if metrics == nil {
		resp, _ := c.Collect()
		return resp
	}
	return metrics
}

// Start runs background periodic collection until context is cancelled or Stop is called.
func (c *SystemCollector) Start(ctx context.Context) {
	// Initial collection
	_, _ = c.Collect()

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			_, _ = c.Collect()
		}
	}
}

// Stop stops the background polling loop safely using sync.Once to prevent double-close panics.
func (c *SystemCollector) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
}
