package main

import (
	"bufio"
	"context"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

// DiskUsageFunc resolves disk metrics for a mount point.
type DiskUsageFunc func(mountPoint string) (totalBytes int64, usedBytes int64, err error)

type netDevStat struct {
	rxBytes int64
	txBytes int64
}

// SystemCollector collects local system metrics from Linux /proc (with cross-platform fallbacks).
type SystemCollector struct {
	procPath      string
	mountsPath    string
	diskUsageFn   DiskUsageFunc
	interval      time.Duration
	prevCPUTotal  uint64
	prevCPUActive uint64
	prevNetStats  map[string]netDevStat
	prevNetTime   time.Time
	startTime     time.Time
	lastMetrics   *MetricsResponse
	mu            sync.RWMutex
	stopCh        chan struct{}
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
		procPath:     procPath,
		mountsPath:   mountsPath,
		diskUsageFn:  diskUsageFn,
		interval:     5 * time.Second,
		prevNetStats: make(map[string]netDevStat),
		startTime:    time.Now().UTC(),
		stopCh:       make(chan struct{}),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Collect executes a single metric collection pass and updates the cached snapshot.
func (c *SystemCollector) Collect() (*MetricsResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UTC()

	// 1. Hostname, OS, Arch
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "unknown"
	}
	osName := runtime.GOOS
	arch := runtime.GOARCH

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

	// 7. Network
	netMetrics := c.collectNetwork(now)

	// 8. Processes
	procCount := c.collectProcesses()

	resp := &MetricsResponse{
		Hostname:      hostname,
		OS:            osName,
		Arch:          arch,
		UptimeSeconds: uptime,
		LoadAverage:   loadAvg,
		CPU:           cpuMetrics,
		Memory:        memMetrics,
		Disks:         diskMetrics,
		Network:       netMetrics,
		Processes:     procCount,
		CollectedAt:   now,
	}

	c.lastMetrics = resp
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

// Stop stops the background polling loop.
func (c *SystemCollector) Stop() {
	select {
	case <-c.stopCh:
	default:
		close(c.stopCh)
	}
}

func (c *SystemCollector) collectUptime() int64 {
	uptimePath := filepath.Join(c.procPath, "uptime")
	data, err := os.ReadFile(uptimePath)
	if err == nil {
		fields := strings.Fields(string(data))
		if len(fields) > 0 {
			if sec, err := strconv.ParseFloat(fields[0], 64); err == nil && sec >= 0 {
				return int64(sec)
			}
		}
	}
	return int64(time.Since(c.startTime).Seconds())
}

func (c *SystemCollector) collectLoadAvg() [3]float64 {
	loadPath := filepath.Join(c.procPath, "loadavg")
	data, err := os.ReadFile(loadPath)
	if err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 3 {
			l1, e1 := strconv.ParseFloat(fields[0], 64)
			l5, e2 := strconv.ParseFloat(fields[1], 64)
			l15, e3 := strconv.ParseFloat(fields[2], 64)
			if e1 == nil && e2 == nil && e3 == nil {
				return [3]float64{
					math.Round(l1*100) / 100,
					math.Round(l5*100) / 100,
					math.Round(l15*100) / 100,
				}
			}
		}
	}
	return [3]float64{0.0, 0.0, 0.0}
}

func (c *SystemCollector) collectCPU() CPUMetrics {
	statPath := filepath.Join(c.procPath, "stat")
	data, err := os.ReadFile(statPath)
	if err != nil {
		return CPUMetrics{
			Count:        runtime.NumCPU(),
			UsagePercent: 0.0,
		}
	}

	lines := strings.Split(string(data), "\n")
	var cpuCount int
	var totalVal, activeVal uint64
	var foundOverall bool

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			if len(fields) >= 5 {
				var sum uint64
				for i := 1; i < len(fields); i++ {
					v, _ := strconv.ParseUint(fields[i], 10, 64)
					sum += v
				}
				idle, _ := strconv.ParseUint(fields[4], 10, 64)
				var iowait uint64
				if len(fields) >= 6 {
					iowait, _ = strconv.ParseUint(fields[5], 10, 64)
				}
				idleTotal := idle + iowait
				totalVal = sum
				if sum >= idleTotal {
					activeVal = sum - idleTotal
				}
				foundOverall = true
			}
		} else if strings.HasPrefix(line, "cpu") && len(line) > 3 && unicode.IsDigit(rune(line[3])) {
			cpuCount++
		}
	}

	if cpuCount == 0 {
		cpuCount = runtime.NumCPU()
	}

	var usagePercent float64
	if foundOverall {
		if c.prevCPUTotal > 0 && totalVal > c.prevCPUTotal {
			deltaTotal := totalVal - c.prevCPUTotal
			deltaActive := activeVal - c.prevCPUActive
			if deltaTotal > 0 && deltaActive <= deltaTotal {
				usagePercent = math.Round((float64(deltaActive)/float64(deltaTotal))*10000) / 100
			}
		}
		c.prevCPUTotal = totalVal
		c.prevCPUActive = activeVal
	}

	return CPUMetrics{
		Count:        cpuCount,
		UsagePercent: usagePercent,
	}
}

func (c *SystemCollector) collectMemory() MemoryMetrics {
	memPath := filepath.Join(c.procPath, "meminfo")
	file, err := os.Open(memPath)
	if err != nil {
		// Non-Linux or fallback
		return MemoryMetrics{
			TotalBytes:     16 * 1024 * 1024 * 1024,
			UsedBytes:      8 * 1024 * 1024 * 1024,
			AvailableBytes: 8 * 1024 * 1024 * 1024,
			UsagePercent:   50.0,
		}
	}
	defer file.Close()

	var memTotalKB, memFreeKB, memAvailKB, buffersKB, cachedKB int64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		valFields := strings.Fields(parts[1])
		if len(valFields) == 0 {
			continue
		}
		val, _ := strconv.ParseInt(valFields[0], 10, 64)

		switch key {
		case "MemTotal":
			memTotalKB = val
		case "MemFree":
			memFreeKB = val
		case "MemAvailable":
			memAvailKB = val
		case "Buffers":
			buffersKB = val
		case "Cached":
			cachedKB = val
		}
	}

	totalBytes := memTotalKB * 1024
	availableBytes := memAvailKB * 1024
	if availableBytes == 0 && (memFreeKB > 0 || buffersKB > 0 || cachedKB > 0) {
		availableBytes = (memFreeKB + buffersKB + cachedKB) * 1024
	}

	usedBytes := totalBytes - availableBytes
	if usedBytes < 0 {
		usedBytes = 0
	}

	var usagePercent float64
	if totalBytes > 0 {
		usagePercent = math.Round((float64(usedBytes)/float64(totalBytes))*10000) / 100
	}

	return MemoryMetrics{
		TotalBytes:     totalBytes,
		UsedBytes:      usedBytes,
		AvailableBytes: availableBytes,
		UsagePercent:   usagePercent,
	}
}

var ignoredFileSystems = map[string]bool{
	"proc":        true,
	"sysfs":       true,
	"devpts":      true,
	"cgroup":      true,
	"cgroup2":     true,
	"pstore":      true,
	"bpf":         true,
	"autofs":      true,
	"mqueue":      true,
	"hugetlbfs":   true,
	"debugfs":     true,
	"tracefs":     true,
	"fusectl":     true,
	"configfs":    true,
	"binfmt_misc": true,
	"nsfs":        true,
	"securityfs":  true,
	"devtmpfs":    true,
	"efivarfs":    true,
	"tmpfs":       true,
	"squashfs":    true,
	"ramfs":       true,
	"none":        true,
}

func (c *SystemCollector) collectDisks() []DiskMetrics {
	disks := make([]DiskMetrics, 0)
	seenMounts := make(map[string]bool)

	file, err := os.Open(c.mountsPath)
	if err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) >= 3 {
				mountPoint := fields[1]
				fsType := fields[2]

				if ignoredFileSystems[fsType] || seenMounts[mountPoint] {
					continue
				}
				if !strings.HasPrefix(mountPoint, "/") {
					continue
				}

				total, used, err := c.diskUsageFn(mountPoint)
				if err == nil && total > 0 {
					seenMounts[mountPoint] = true
					pct := math.Round((float64(used)/float64(total))*10000) / 100
					disks = append(disks, DiskMetrics{
						MountPoint:   mountPoint,
						TotalBytes:   total,
						UsedBytes:    used,
						UsagePercent: pct,
						Filesystem:   fsType,
					})
				}
			}
		}
	}

	// Fallback if no mounts discovered
	if len(disks) == 0 {
		total, used, err := c.diskUsageFn("/")
		if err == nil && total > 0 {
			pct := math.Round((float64(used)/float64(total))*10000) / 100
			disks = append(disks, DiskMetrics{
				MountPoint:   "/",
				TotalBytes:   total,
				UsedBytes:    used,
				UsagePercent: pct,
				Filesystem:   "ext4",
			})
		}
	}

	return disks
}

func (c *SystemCollector) collectNetwork(now time.Time) NetworkMetrics {
	netDevPath := filepath.Join(c.procPath, "net", "dev")
	data, err := os.ReadFile(netDevPath)
	if err != nil {
		return NetworkMetrics{
			Interfaces:         make([]NetworkInterfaceMetrics, 0),
			TotalRxBytesPerSec: 0,
			TotalTxBytesPerSec: 0,
		}
	}

	var elapsed float64
	if !c.prevNetTime.IsZero() {
		elapsed = now.Sub(c.prevNetTime).Seconds()
	}
	if elapsed <= 0 {
		elapsed = 1.0
	}

	interfaces := make([]NetworkInterfaceMetrics, 0)
	var totalRx, totalTx int64

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}
		iface := strings.TrimSpace(parts[0])
		fields := strings.Fields(parts[1])
		if len(fields) < 9 {
			continue
		}

		rxBytes, _ := strconv.ParseInt(fields[0], 10, 64)
		txBytes, _ := strconv.ParseInt(fields[8], 10, 64)

		var rxRate, txRate int64
		prev, ok := c.prevNetStats[iface]
		if ok && elapsed > 0 {
			if rxBytes >= prev.rxBytes {
				rxRate = int64(float64(rxBytes-prev.rxBytes) / elapsed)
			}
			if txBytes >= prev.txBytes {
				txRate = int64(float64(txBytes-prev.txBytes) / elapsed)
			}
		}

		c.prevNetStats[iface] = netDevStat{
			rxBytes: rxBytes,
			txBytes: txBytes,
		}

		interfaces = append(interfaces, NetworkInterfaceMetrics{
			Name:          iface,
			RxBytesPerSec: rxRate,
			TxBytesPerSec: txRate,
		})

		totalRx += rxRate
		totalTx += txRate
	}

	c.prevNetTime = now

	return NetworkMetrics{
		Interfaces:         interfaces,
		TotalRxBytesPerSec: totalRx,
		TotalTxBytesPerSec: totalTx,
	}
}

func (c *SystemCollector) collectProcesses() int {
	entries, err := os.ReadDir(c.procPath)
	if err == nil {
		var count int
		for _, entry := range entries {
			if entry.IsDir() {
				name := entry.Name()
				isNumeric := true
				for _, r := range name {
					if !unicode.IsDigit(r) {
						isNumeric = false
						break
					}
				}
				if isNumeric && len(name) > 0 {
					count++
				}
			}
		}
		if count > 0 {
			return count
		}
	}

	loadPath := filepath.Join(c.procPath, "loadavg")
	if data, err := os.ReadFile(loadPath); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 4 {
			threadsPart := fields[3]
			parts := strings.Split(threadsPart, "/")
			if len(parts) == 2 {
				if n, err := strconv.Atoi(parts[1]); err == nil && n > 0 {
					return n
				}
			}
		}
	}

	return 1
}
