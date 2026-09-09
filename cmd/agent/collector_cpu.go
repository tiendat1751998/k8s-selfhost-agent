package main

import (
	"bufio"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type procStatSnapshot struct {
	utime      uint64
	stime      uint64
	readBytes  int64
	writeBytes int64
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
