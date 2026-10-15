//go:build linux
package metrics

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type platformHostProber struct {
	mu           sync.Mutex
	procPath     string
	prevActive   uint64
	prevTotal    uint64
	prevCPUUsage float64
}

func newPlatformHostProber() HostProber {
	proc := os.Getenv("HOST_PROC")
	if proc == "" {
		proc = "/proc"
	}
	return &platformHostProber{
		procPath: proc,
	}
}

func (p *platformHostProber) ProbeHost(ctx context.Context) HostTelemetry {
	hName, _ := os.Hostname()
	if hName == "" {
		hName = "linux-host"
	}

	totalMem, usedMem, memPercent := p.readMeminfo()
	cpuPercent := p.readCPUStat()
	uptime := p.readUptime()

	return HostTelemetry{
		Hostname:      hName,
		OS:            "linux",
		Arch:          runtime.GOARCH,
		OSDistro:      "Linux",
		CPUPercent:    cpuPercent,
		CPUCount:      runtime.NumCPU(),
		MemoryUsed:    usedMem,
		MemoryTotal:   totalMem,
		MemoryPercent: memPercent,
		Processes:     runtime.NumGoroutine(),
		UptimeSeconds: uptime,
	}
}

func (p *platformHostProber) readMeminfo() (int64, int64, float64) {
	file, err := os.Open(filepath.Join(p.procPath, "meminfo"))
	if err != nil {
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		return int64(ms.Sys), int64(ms.Alloc), 0.0
	}
	defer file.Close()

	var totalKB, freeKB, availKB, buffersKB, cachedKB int64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		fields := strings.Fields(parts[1])
		if len(fields) == 0 {
			continue
		}
		val, _ := strconv.ParseInt(fields[0], 10, 64)
		switch key {
		case "MemTotal":
			totalKB = val
		case "MemFree":
			freeKB = val
		case "MemAvailable":
			availKB = val
		case "Buffers":
			buffersKB = val
		case "Cached":
			cachedKB = val
		}
	}

	totalBytes := totalKB * 1024
	availBytes := availKB * 1024
	if availBytes == 0 && (freeKB > 0 || buffersKB > 0 || cachedKB > 0) {
		availBytes = (freeKB + buffersKB + cachedKB) * 1024
	}
	usedBytes := totalBytes - availBytes
	if usedBytes < 0 {
		usedBytes = 0
	}
	pct := 0.0
	if totalBytes > 0 {
		pct = (float64(usedBytes) / float64(totalBytes)) * 100.0
	}
	return totalBytes, usedBytes, pct
}

func (p *platformHostProber) readCPUStat() float64 {
	data, err := os.ReadFile(filepath.Join(p.procPath, "stat"))
	if err != nil {
		return 0.0
	}

	lines := strings.Split(string(data), "\n")
	var sum, idleTotal uint64
	for _, line := range lines {
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			if len(fields) >= 5 {
				for i := 1; i < len(fields); i++ {
					v, _ := strconv.ParseUint(fields[i], 10, 64)
					sum += v
				}
				idle, _ := strconv.ParseUint(fields[4], 10, 64)
				var iowait uint64
				if len(fields) >= 6 {
					iowait, _ = strconv.ParseUint(fields[5], 10, 64)
				}
				idleTotal = idle + iowait
				break
			}
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	var active uint64
	if sum >= idleTotal {
		active = sum - idleTotal
	}

	usage := p.prevCPUUsage
	if p.prevTotal > 0 && sum > p.prevTotal {
		deltaTotal := float64(sum - p.prevTotal)
		deltaActive := float64(active - p.prevActive)
		if deltaActive > 0 && deltaTotal > 0 {
			usage = (deltaActive / deltaTotal) * 100.0
		} else {
			usage = 0.0
		}
	}

	p.prevTotal = sum
	p.prevActive = active
	p.prevCPUUsage = usage
	return usage
}

func (p *platformHostProber) readUptime() int64 {
	data, err := os.ReadFile(filepath.Join(p.procPath, "uptime"))
	if err == nil {
		fields := strings.Fields(string(data))
		if len(fields) > 0 {
			if sec, parseErr := strconv.ParseFloat(fields[0], 64); parseErr == nil {
				return int64(sec)
			}
		}
	}
	return int64(time.Since(processStartTime).Seconds())
}
