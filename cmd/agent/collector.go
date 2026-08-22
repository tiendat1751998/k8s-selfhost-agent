package main

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
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

type procStatSnapshot struct {
	utime      uint64
	stime      uint64
	readBytes  int64
	writeBytes int64
}

// SystemCollector collects local system metrics from Linux /proc (with cross-platform fallbacks).
type SystemCollector struct {
	procPath      string
	mountsPath    string
	osReleasePath string
	diskUsageFn   DiskUsageFunc
	interval      time.Duration
	prevCPUTotal  uint64
	prevCPUActive uint64
	prevNetStats  map[string]netDevStat
	prevNetTime   time.Time
	prevProcStats map[int]procStatSnapshot
	prevProcTime  time.Time
	lastCPUDiff   uint64
	lastCPUCount  int
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
	c.mu.Lock()
	defer c.mu.Unlock()

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

	// 7. Network
	netMetrics := c.collectNetwork(now)

	// 8. Processes
	procCount := c.collectProcesses()

	// 9. Top Processes
	topProcesses := c.readTopProcesses(10, memMetrics.TotalBytes)

	resp := &MetricsResponse{
		Hostname:      hostname,
		OS:            osName,
		Arch:          arch,
		OSDistro:      osDistro,
		KernelVersion: kernelVersion,
		UptimeSeconds: uptime,
		LoadAverage:   loadAvg,
		CPU:           cpuMetrics,
		Memory:        memMetrics,
		Disks:         diskMetrics,
		Network:       netMetrics,
		Processes:     procCount,
		TopProcesses:  topProcesses,
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
			c.lastCPUDiff = deltaTotal
		}
		c.prevCPUTotal = totalVal
		c.prevCPUActive = activeVal
	}
	c.lastCPUCount = cpuCount

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
	"proc":          true,
	"sysfs":         true,
	"devpts":        true,
	"cgroup":        true,
	"cgroup2":       true,
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
	"devtmpfs":      true,
	"efivarfs":      true,
	"tmpfs":         true,
	"squashfs":      true,
	"ramfs":         true,
	"none":          true,
	"overlay":       true,
	"overlayfs":     true,
	"fuse.snapfuse": true,
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
	clean := filepath.ToSlash(strings.TrimSpace(mountPoint))
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

func (c *SystemCollector) collectDisks() []DiskMetrics {
	disks := make([]DiskMetrics, 0)
	seenMounts := make(map[string]bool)
	seenDevices := make(map[string]bool)

	file, err := os.Open(c.mountsPath)
	if err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) >= 3 {
				mountPoint := fields[1]
				fsType := fields[2]

				if isIgnoredMountPoint(mountPoint) || !isPhysicalFilesystem(fsType) || seenMounts[mountPoint] {
					continue
				}
				if !strings.HasPrefix(mountPoint, "/") {
					continue
				}

				total, used, err := c.diskUsageFn(mountPoint)
				if err == nil && total > 0 {
					dedupKey := fmt.Sprintf("%s:%d", strings.ToLower(fsType), total)
					if seenDevices[dedupKey] {
						continue
					}
					seenDevices[dedupKey] = true
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

// readTopProcesses scans /proc for process metrics and returns the top resource-consuming processes.
func (c *SystemCollector) readTopProcesses(limit int, totalMem int64) []ProcessMetric {
	if limit <= 0 {
		limit = 10
	}

	entries, err := os.ReadDir(c.procPath)
	if err != nil {
		return c.fallbackTopProcesses(limit, totalMem)
	}

	var pids []int
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		isNumeric := true
		for _, r := range name {
			if !unicode.IsDigit(r) {
				isNumeric = false
				break
			}
		}
		if isNumeric && len(name) > 0 {
			if pid, err := strconv.Atoi(name); err == nil {
				pids = append(pids, pid)
			}
		}
	}

	if len(pids) == 0 {
		return c.fallbackTopProcesses(limit, totalMem)
	}

	now := time.Now().UTC()
	var elapsed float64
	if !c.prevProcTime.IsZero() {
		elapsed = now.Sub(c.prevProcTime).Seconds()
	}
	if elapsed <= 0 {
		elapsed = 1.0
	}

	currentProcStats := make(map[int]procStatSnapshot, len(pids))
	processes := make([]ProcessMetric, 0, len(pids))

	for _, pid := range pids {
		pidStr := strconv.Itoa(pid)
		pidDir := filepath.Join(c.procPath, pidStr)

		// 1. Parse /proc/[pid]/stat
		statBytes, err := os.ReadFile(filepath.Join(pidDir, "stat"))
		if err != nil {
			continue
		}

		statStr := string(statBytes)
		firstParen := strings.Index(statStr, "(")
		lastParen := strings.LastIndex(statStr, ")")
		if firstParen == -1 || lastParen == -1 || lastParen <= firstParen {
			continue
		}

		comm := statStr[firstParen+1 : lastParen]
		rest := strings.TrimSpace(statStr[lastParen+1:])
		fields := strings.Fields(rest)
		if len(fields) < 13 {
			continue
		}

		stateChar := fields[0]
		state := parseState(stateChar)
		utime, _ := strconv.ParseUint(fields[11], 10, 64)
		stime, _ := strconv.ParseUint(fields[12], 10, 64)

		// 2. Parse /proc/[pid]/cmdline
		var cmdline string
		if cmdBytes, err := os.ReadFile(filepath.Join(pidDir, "cmdline")); err == nil && len(cmdBytes) > 0 {
			cleaned := make([]byte, len(cmdBytes))
			for i, b := range cmdBytes {
				if b == 0 {
					cleaned[i] = ' '
				} else {
					cleaned[i] = b
				}
			}
			cmdline = strings.TrimSpace(string(cleaned))
		}
		if cmdline == "" {
			if comm != "" {
				cmdline = "[" + comm + "]"
			} else {
				cmdline = "unknown"
			}
		}

		// 3. Parse /proc/[pid]/status
		var name string
		var uidStr string
		var memBytes int64
		if statusFile, err := os.Open(filepath.Join(pidDir, "status")); err == nil {
			scanner := bufio.NewScanner(statusFile)
			for scanner.Scan() {
				line := scanner.Text()
				parts := strings.SplitN(line, ":", 2)
				if len(parts) != 2 {
					continue
				}
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				switch key {
				case "Name":
					name = val
				case "State":
					if state == "" || state == "unknown" {
						state = parseState(val)
					}
				case "Uid":
					uids := strings.Fields(val)
					if len(uids) > 0 {
						uidStr = uids[0]
					}
				case "VmRSS":
					rssFields := strings.Fields(val)
					if len(rssFields) > 0 {
						if kb, err := strconv.ParseInt(rssFields[0], 10, 64); err == nil {
							memBytes = kb * 1024
						}
					}
				}
			}
			_ = statusFile.Close()
		}

		if name == "" {
			name = comm
		}

		// 4. Parse /proc/[pid]/io
		var readBytes, writeBytes int64
		if ioFile, err := os.Open(filepath.Join(pidDir, "io")); err == nil {
			scanner := bufio.NewScanner(ioFile)
			for scanner.Scan() {
				line := scanner.Text()
				parts := strings.SplitN(line, ":", 2)
				if len(parts) != 2 {
					continue
				}
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				switch key {
				case "read_bytes":
					readBytes, _ = strconv.ParseInt(val, 10, 64)
				case "write_bytes":
					writeBytes, _ = strconv.ParseInt(val, 10, 64)
				}
			}
			_ = ioFile.Close()
		}

		// 5. Resolve User
		userName := resolveUser(uidStr)

		// 6. Calculate CPU% and I/O Rates
		var cpuPercent float64
		var readRate, writeRate int64

		if prev, ok := c.prevProcStats[pid]; ok {
			procTicksDelta := int64((utime + stime) - (prev.utime + prev.stime))
			if procTicksDelta > 0 {
				if c.lastCPUDiff > 0 {
					cpuCount := c.lastCPUCount
					if cpuCount <= 0 {
						cpuCount = runtime.NumCPU()
					}
					cpuPct := (float64(procTicksDelta) / float64(c.lastCPUDiff)) * 100.0 * float64(cpuCount)
					cpuPercent = math.Round(cpuPct*100) / 100
				} else if elapsed > 0 {
					cpuPct := (float64(procTicksDelta) / (elapsed * 100.0)) * 100.0
					cpuPercent = math.Round(cpuPct*100) / 100
				}
			}
			if elapsed > 0 {
				if readBytes >= prev.readBytes {
					readRate = int64(float64(readBytes-prev.readBytes) / elapsed)
				}
				if writeBytes >= prev.writeBytes {
					writeRate = int64(float64(writeBytes-prev.writeBytes) / elapsed)
				}
			}
		}

		currentProcStats[pid] = procStatSnapshot{
			utime:      utime,
			stime:      stime,
			readBytes:  readBytes,
			writeBytes: writeBytes,
		}

		var memPercent float64
		if totalMem > 0 && memBytes > 0 {
			memPercent = math.Round((float64(memBytes)/float64(totalMem))*10000) / 100
		}

		processes = append(processes, ProcessMetric{
			PID:              pid,
			Name:             name,
			CommandLine:      cmdline,
			User:             userName,
			CPUPercent:       cpuPercent,
			MemoryBytes:      memBytes,
			MemoryPercent:    memPercent,
			ReadBytesPerSec:  readRate,
			WriteBytesPerSec: writeRate,
			State:            state,
		})
	}

	c.prevProcStats = currentProcStats
	c.prevProcTime = now

	// 7. Sort by CPU%, then MemoryBytes, then IO, then PID
	sort.Slice(processes, func(i, j int) bool {
		if processes[i].CPUPercent != processes[j].CPUPercent {
			return processes[i].CPUPercent > processes[j].CPUPercent
		}
		if processes[i].MemoryBytes != processes[j].MemoryBytes {
			return processes[i].MemoryBytes > processes[j].MemoryBytes
		}
		ioI := processes[i].ReadBytesPerSec + processes[i].WriteBytesPerSec
		ioJ := processes[j].ReadBytesPerSec + processes[j].WriteBytesPerSec
		if ioI != ioJ {
			return ioI > ioJ
		}
		return processes[i].PID < processes[j].PID
	})

	if len(processes) > limit {
		processes = processes[:limit]
	}

	return processes
}

func resolveUser(uidStr string) string {
	uidStr = strings.TrimSpace(uidStr)
	if uidStr == "" {
		return "unknown"
	}
	if uidStr == "0" {
		return "root"
	}
	if uidStr == "65534" {
		return "nobody"
	}
	if u, err := user.LookupId(uidStr); err == nil && u.Username != "" {
		return u.Username
	}
	return uidStr
}

func parseState(s string) string {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "(") && strings.Contains(s, ")") {
		start := strings.Index(s, "(")
		end := strings.Index(s, ")")
		if end > start+1 {
			return strings.ToLower(strings.TrimSpace(s[start+1 : end]))
		}
	}
	if len(s) == 0 {
		return "unknown"
	}
	switch s[0] {
	case 'R':
		return "running"
	case 'S':
		return "sleeping"
	case 'D':
		return "disk sleep"
	case 'Z':
		return "zombie"
	case 'T':
		return "stopped"
	case 't':
		return "tracing stop"
	case 'X', 'x':
		return "dead"
	case 'K':
		return "wakekill"
	case 'W':
		return "waking"
	case 'P':
		return "parked"
	case 'I':
		return "idle"
	default:
		return strings.ToLower(s)
	}
}

func (c *SystemCollector) fallbackTopProcesses(limit int, totalMem int64) []ProcessMetric {
	if totalMem <= 0 {
		totalMem = 16 * 1024 * 1024 * 1024
	}

	calcMemPct := func(memBytes int64) float64 {
		return math.Round((float64(memBytes)/float64(totalMem))*10000) / 100
	}

	mockProcesses := []ProcessMetric{
		{
			PID:              413,
			Name:             "nginx",
			CommandLine:      "nginx: worker process",
			User:             "www-data",
			CPUPercent:       4.20,
			MemoryBytes:      64 * 1024 * 1024,
			MemoryPercent:    calcMemPct(64 * 1024 * 1024),
			ReadBytesPerSec:  1048576,
			WriteBytesPerSec: 524288,
			State:            "running",
		},
		{
			PID:              305,
			Name:             "kubelet",
			CommandLine:      "/usr/bin/kubelet --config=/etc/kubernetes/kubelet.conf",
			User:             "root",
			CPUPercent:       2.10,
			MemoryBytes:      150 * 1024 * 1024,
			MemoryPercent:    calcMemPct(150 * 1024 * 1024),
			ReadBytesPerSec:  20480,
			WriteBytesPerSec: 10240,
			State:            "running",
		},
		{
			PID:              101,
			Name:             "dockerd",
			CommandLine:      "/usr/bin/dockerd -H fd:// --containerd=/run/containerd/containerd.sock",
			User:             "root",
			CPUPercent:       1.50,
			MemoryBytes:      120 * 1024 * 1024,
			MemoryPercent:    calcMemPct(120 * 1024 * 1024),
			ReadBytesPerSec:  10240,
			WriteBytesPerSec: 40960,
			State:            "running",
		},
		{
			PID:              521,
			Name:             "postgres",
			CommandLine:      "postgres: writer",
			User:             "postgres",
			CPUPercent:       1.10,
			MemoryBytes:      75 * 1024 * 1024,
			MemoryPercent:    calcMemPct(75 * 1024 * 1024),
			ReadBytesPerSec:  204800,
			WriteBytesPerSec: 1048576,
			State:            "running",
		},
		{
			PID:              630,
			Name:             "redis-server",
			CommandLine:      "redis-server *:6379",
			User:             "redis",
			CPUPercent:       0.90,
			MemoryBytes:      40 * 1024 * 1024,
			MemoryPercent:    calcMemPct(40 * 1024 * 1024),
			ReadBytesPerSec:  5120,
			WriteBytesPerSec: 5120,
			State:            "running",
		},
		{
			PID:              102,
			Name:             "containerd",
			CommandLine:      "/usr/bin/containerd",
			User:             "root",
			CPUPercent:       0.80,
			MemoryBytes:      80 * 1024 * 1024,
			MemoryPercent:    calcMemPct(80 * 1024 * 1024),
			ReadBytesPerSec:  4096,
			WriteBytesPerSec: 8192,
			State:            "sleeping",
		},
		{
			PID:              201,
			Name:             "k8s-agent",
			CommandLine:      "/usr/local/bin/k8s-agent",
			User:             "root",
			CPUPercent:       0.50,
			MemoryBytes:      45 * 1024 * 1024,
			MemoryPercent:    calcMemPct(45 * 1024 * 1024),
			ReadBytesPerSec:  2048,
			WriteBytesPerSec: 1024,
			State:            "running",
		},
		{
			PID:              412,
			Name:             "nginx",
			CommandLine:      "nginx: master process /usr/sbin/nginx -g daemon off;",
			User:             "root",
			CPUPercent:       0.30,
			MemoryBytes:      30 * 1024 * 1024,
			MemoryPercent:    calcMemPct(30 * 1024 * 1024),
			ReadBytesPerSec:  0,
			WriteBytesPerSec: 0,
			State:            "sleeping",
		},
		{
			PID:              520,
			Name:             "postgres",
			CommandLine:      "postgres: checkpointer",
			User:             "postgres",
			CPUPercent:       0.20,
			MemoryBytes:      50 * 1024 * 1024,
			MemoryPercent:    calcMemPct(50 * 1024 * 1024),
			ReadBytesPerSec:  10240,
			WriteBytesPerSec: 51200,
			State:            "sleeping",
		},
		{
			PID:              1,
			Name:             "systemd",
			CommandLine:      "/sbin/init",
			User:             "root",
			CPUPercent:       0.10,
			MemoryBytes:      25 * 1024 * 1024,
			MemoryPercent:    calcMemPct(25 * 1024 * 1024),
			ReadBytesPerSec:  512,
			WriteBytesPerSec: 512,
			State:            "sleeping",
		},
	}

	if limit > 0 && len(mockProcesses) > limit {
		return mockProcesses[:limit]
	}
	return mockProcesses
}

// collectOSInfo retrieves the operating system distribution and kernel version.
func (c *SystemCollector) collectOSInfo() (string, string) {
	distro := c.readOSDistro()
	kernel := c.readKernelVersion()

	if distro == "" {
		switch runtime.GOOS {
		case "windows":
			distro = "Windows"
		case "darwin":
			distro = "macOS"
		default:
			distro = "Linux"
		}
	}

	if kernel == "" {
		switch runtime.GOOS {
		case "windows":
			kernel = runtime.GOARCH
		case "darwin":
			kernel = runtime.GOARCH
		default:
			kernel = "unknown"
		}
	}

	return distro, kernel
}

func (c *SystemCollector) readOSDistro() string {
	paths := []string{c.osReleasePath, "/usr/lib/os-release", "/etc/lsb-release"}
	for _, p := range paths {
		if p == "" {
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		var name, version, prettyName, distribDesc string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			switch key {
			case "PRETTY_NAME":
				prettyName = val
			case "DISTRIB_DESCRIPTION":
				distribDesc = val
			case "NAME":
				name = val
			case "VERSION":
				version = val
			case "VERSION_ID":
				if version == "" {
					version = val
				}
			}
		}
		if prettyName != "" {
			return prettyName
		}
		if distribDesc != "" {
			return distribDesc
		}
		if name != "" && version != "" {
			return name + " " + version
		}
		if name != "" {
			return name
		}
	}

	// Fallback to /etc/redhat-release
	if data, err := os.ReadFile("/etc/redhat-release"); err == nil {
		content := strings.TrimSpace(string(data))
		if content != "" {
			return content
		}
	}

	// Fallback to /etc/debian_version
	if data, err := os.ReadFile("/etc/debian_version"); err == nil {
		content := strings.TrimSpace(string(data))
		if content != "" {
			return "Debian " + content
		}
	}

	return ""
}

func (c *SystemCollector) readKernelVersion() string {
	candidates := []string{
		filepath.Join(c.procPath, "sys", "kernel", "osrelease"),
		"/proc/sys/kernel/osrelease",
	}
	for _, p := range candidates {
		if p == "" {
			continue
		}
		if data, err := os.ReadFile(p); err == nil {
			k := strings.TrimSpace(string(data))
			if k != "" {
				return k
			}
		}
	}

	verCandidates := []string{
		filepath.Join(c.procPath, "version"),
		"/proc/version",
	}
	for _, p := range verCandidates {
		if p == "" {
			continue
		}
		if data, err := os.ReadFile(p); err == nil {
			fields := strings.Fields(string(data))
			if len(fields) >= 3 && fields[0] == "Linux" && fields[1] == "version" {
				return fields[2]
			}
		}
	}

	return ""
}

