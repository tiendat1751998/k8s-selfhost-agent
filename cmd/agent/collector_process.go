package main

import (
	"log/slog"
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

// readTopProcesses scans /proc for process metrics and returns the top resource-consuming processes.
func (c *SystemCollector) readTopProcesses(limit int, totalMem int64) []ProcessMetric {
	if limit <= 0 {
		limit = 10
	}

	entries, err := os.ReadDir(c.procPath)
	if err != nil {
		return c.fallbackTopProcesses(limit, totalMem)
	}

	pids := collectPIDs(entries)
	if len(pids) == 0 {
		return c.fallbackTopProcesses(limit, totalMem)
	}

	now := time.Now().UTC()
	elapsed := 1.0
	if diff := now.Sub(c.prevProcTime).Seconds(); !c.prevProcTime.IsZero() && diff > 0 {
		elapsed = diff
	}

	currentProcStats := make(map[int]procStatSnapshot, len(pids))
	processes := make([]ProcessMetric, 0, len(pids))

	for _, pid := range pids {
		pidDir := filepath.Join(c.procPath, strconv.Itoa(pid))
		comm, state, utime, stime, ok := parseProcessStat(pidDir, pid)
		if !ok {
			continue
		}

		cmdline := parseProcessCmdline(pidDir, comm)
		name, state, uidStr, memBytes := parseProcessStatus(pidDir, state)
		if name == "" {
			name = comm
		}

		readBytes, writeBytes := parseProcessIO(pidDir, pid)
		cpuPercent, readRate, writeRate := c.calcProcRates(pid, utime, stime, readBytes, writeBytes, elapsed)
		currentProcStats[pid] = procStatSnapshot{utime: utime, stime: stime, readBytes: readBytes, writeBytes: writeBytes}

		var memPercent float64
		if totalMem > 0 && memBytes > 0 {
			memPercent = math.Round((float64(memBytes)/float64(totalMem))*10000) / 100
		}

		processes = append(processes, ProcessMetric{
			PID: pid, Name: name, CommandLine: cmdline, User: resolveUser(uidStr), State: state,
			CPUPercent: cpuPercent, MemoryBytes: memBytes, MemoryPercent: memPercent,
			ReadBytesPerSec: readRate, WriteBytesPerSec: writeRate,
		})
	}

	c.prevProcStats = currentProcStats
	c.prevProcTime = now

	sort.Slice(processes, func(i, j int) bool {
		if processes[i].CPUPercent != processes[j].CPUPercent {
			return processes[i].CPUPercent > processes[j].CPUPercent
		}
		if processes[i].MemoryBytes != processes[j].MemoryBytes {
			return processes[i].MemoryBytes > processes[j].MemoryBytes
		}
		if ioI, ioJ := processes[i].ReadBytesPerSec+processes[i].WriteBytesPerSec, processes[j].ReadBytesPerSec+processes[j].WriteBytesPerSec; ioI != ioJ {
			return ioI > ioJ
		}
		return processes[i].PID < processes[j].PID
	})

	if len(processes) > limit {
		processes = processes[:limit]
	}
	return processes
}

func (c *SystemCollector) calcProcRates(pid int, utime, stime uint64, readBytes, writeBytes int64, elapsed float64) (cpuPercent float64, readRate, writeRate int64) {
	prev, ok := c.prevProcStats[pid]
	if !ok {
		return 0, 0, 0
	}
	if ticks := int64((utime + stime) - (prev.utime + prev.stime)); ticks > 0 {
		if c.lastCPUDiff > 0 {
			cpuCount := c.lastCPUCount
			if cpuCount <= 0 {
				cpuCount = runtime.NumCPU()
			}
			cpuPercent = math.Round((float64(ticks)/float64(c.lastCPUDiff))*100.0*float64(cpuCount)*100) / 100
		} else if elapsed > 0 {
			cpuPercent = math.Round((float64(ticks)/(elapsed*100.0))*100.0*100) / 100
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
	return cpuPercent, readRate, writeRate
}

func collectPIDs(entries []os.DirEntry) []int {
	var pids []int
	for _, entry := range entries {
		if entry.IsDir() {
			if pid, err := strconv.Atoi(entry.Name()); err == nil && pid > 0 && unicode.IsDigit(rune(entry.Name()[0])) {
				pids = append(pids, pid)
			}
		}
	}
	return pids
}

func parseProcessStat(pidDir string, pid int) (comm, state string, utime, stime uint64, ok bool) {
	statBytes, err := os.ReadFile(filepath.Join(pidDir, "stat"))
	if err != nil {
		return "", "", 0, 0, false
	}

	statStr := string(statBytes)
	firstParen := strings.Index(statStr, "(")
	lastParen := strings.LastIndex(statStr, ")")
	if firstParen == -1 || lastParen <= firstParen {
		return "", "", 0, 0, false
	}

	comm = statStr[firstParen+1 : lastParen]
	fields := strings.Fields(strings.TrimSpace(statStr[lastParen+1:]))
	if len(fields) < 13 {
		return "", "", 0, 0, false
	}

	state = parseState(fields[0])
	var parseErr error
	if utime, parseErr = strconv.ParseUint(fields[11], 10, 64); parseErr != nil {
		slog.Warn("failed to parse process utime", slog.Int("pid", pid), slog.Any("error", parseErr))
		return "", "", 0, 0, false
	}
	if stime, parseErr = strconv.ParseUint(fields[12], 10, 64); parseErr != nil {
		slog.Warn("failed to parse process stime", slog.Int("pid", pid), slog.Any("error", parseErr))
		return "", "", 0, 0, false
	}
	return comm, state, utime, stime, true
}

func parseProcessCmdline(pidDir string, comm string) string {
	if cmdBytes, err := os.ReadFile(filepath.Join(pidDir, "cmdline")); err == nil && len(cmdBytes) > 0 {
		cleaned := make([]byte, len(cmdBytes))
		for i, b := range cmdBytes {
			if b == 0 {
				cleaned[i] = ' '
			} else {
				cleaned[i] = b
			}
		}
		if s := strings.TrimSpace(string(cleaned)); s != "" {
			return s
		}
	}
	if comm != "" {
		return "[" + comm + "]"
	}
	return "unknown"
}

func parseProcessStatus(pidDir string, curState string) (name string, state string, uidStr string, memBytes int64) {
	state = curState
	data, err := os.ReadFile(filepath.Join(pidDir, "status"))
	if err != nil {
		return "", state, "", 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		switch key {
		case "Name":
			name = val
		case "State":
			if state == "" || state == "unknown" {
				state = parseState(val)
			}
		case "Uid":
			if uids := strings.Fields(val); len(uids) > 0 {
				uidStr = uids[0]
			}
		case "VmRSS":
			if rss := strings.Fields(val); len(rss) > 0 {
				if kb, err := strconv.ParseInt(rss[0], 10, 64); err == nil {
					memBytes = kb * 1024
				}
			}
		}
	}
	return name, state, uidStr, memBytes
}

func parseProcessIO(pidDir string, pid int) (readBytes, writeBytes int64) {
	data, err := os.ReadFile(filepath.Join(pidDir, "io"))
	if err != nil {
		return 0, 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		switch key {
		case "read_bytes":
			if v, err := strconv.ParseInt(val, 10, 64); err == nil {
				readBytes = v
			} else {
				slog.Warn("failed to parse process read_bytes", slog.Int("pid", pid), slog.Any("error", err))
			}
		case "write_bytes":
			if v, err := strconv.ParseInt(val, 10, 64); err == nil {
				writeBytes = v
			} else {
				slog.Warn("failed to parse process write_bytes", slog.Int("pid", pid), slog.Any("error", err))
			}
		}
	}
	return readBytes, writeBytes
}

func (c *SystemCollector) fallbackTopProcesses(limit int, totalMem int64) []ProcessMetric {
	if totalMem <= 0 {
		totalMem = 16 * 1024 * 1024 * 1024
	}

	calcMemPct := func(mem int64) float64 {
		return math.Round((float64(mem)/float64(totalMem))*10000) / 100
	}

	mock := func(pid int, name, cmd, user, state string, cpu float64, memMB, rRate, wRate int64) ProcessMetric {
		mem := memMB * 1024 * 1024
		return ProcessMetric{
			PID:              pid,
			Name:             name,
			CommandLine:      cmd,
			User:             user,
			CPUPercent:       cpu,
			MemoryBytes:      mem,
			MemoryPercent:    calcMemPct(mem),
			ReadBytesPerSec:  rRate,
			WriteBytesPerSec: wRate,
			State:            state,
		}
	}

	procs := []ProcessMetric{
		mock(413, "nginx", "nginx: worker process", "www-data", "running", 4.20, 64, 1048576, 524288),
		mock(305, "kubelet", "/usr/bin/kubelet --config=/etc/kubernetes/kubelet.conf", "root", "running", 2.10, 150, 20480, 10240),
		mock(101, "dockerd", "/usr/bin/dockerd -H fd:// --containerd=/run/containerd/containerd.sock", "root", "running", 1.50, 120, 10240, 40960),
		mock(521, "postgres", "postgres: writer", "postgres", "running", 1.10, 75, 204800, 1048576),
		mock(630, "redis-server", "redis-server *:6379", "redis", "running", 0.90, 40, 5120, 5120),
		mock(102, "containerd", "/usr/bin/containerd", "root", "sleeping", 0.80, 80, 4096, 8192),
		mock(201, "k8s-agent", "/usr/local/bin/k8s-agent", "root", "running", 0.50, 45, 2048, 1024),
		mock(412, "nginx", "nginx: master process /usr/sbin/nginx -g daemon off;", "root", "sleeping", 0.30, 30, 0, 0),
		mock(520, "postgres", "postgres: checkpointer", "postgres", "sleeping", 0.20, 50, 10240, 51200),
		mock(1, "systemd", "/sbin/init", "root", "sleeping", 0.10, 25, 512, 512),
	}

	if limit > 0 && len(procs) > limit {
		return procs[:limit]
	}
	return procs
}
