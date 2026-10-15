package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func parsePSIFile(path string) (PSIResourceStats, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return PSIResourceStats{}, err
	}
	return parsePSIContent(string(data)), nil
}

func parsePSIContent(content string) PSIResourceStats {
	var stats PSIResourceStats
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "some ") {
			fields := strings.Fields(line[5:])
			for _, f := range fields {
				parts := strings.SplitN(f, "=", 2)
				if len(parts) != 2 {
					continue
				}
				switch parts[0] {
				case "avg10":
					val, _ := strconv.ParseFloat(parts[1], 64)
					stats.Avg10 = val
				case "avg60":
					val, _ := strconv.ParseFloat(parts[1], 64)
					stats.Avg60 = val
				case "avg300":
					val, _ := strconv.ParseFloat(parts[1], 64)
					stats.Avg300 = val
				case "total":
					tVal, _ := strconv.ParseUint(parts[1], 10, 64)
					stats.TotalMicros = tVal
				}
			}
			break
		}
	}
	return stats
}

func (c *SystemCollector) collectPSI() PSIMetrics {
	var psi PSIMetrics
	pressureDir := filepath.Join(c.procPath, "pressure")
	psi.CPU, _ = parsePSIFile(filepath.Join(pressureDir, "cpu"))
	psi.Memory, _ = parsePSIFile(filepath.Join(pressureDir, "memory"))
	psi.IO, _ = parsePSIFile(filepath.Join(pressureDir, "io"))
	return psi
}

func (c *SystemCollector) collectK8sMetrics() K8sNodeMetrics {
	var metrics K8sNodeMetrics

	// 1. PodCount: count directories in /var/log/pods or /sys/fs/cgroup/kubepods.slice
	podDir := "/var/log/pods"
	if entries, err := os.ReadDir(podDir); err == nil {
		count := 0
		for _, e := range entries {
			if e.IsDir() {
				count++
			}
		}
		metrics.PodCount = count
	} else if entries, err := os.ReadDir("/sys/fs/cgroup/kubepods.slice"); err == nil {
		count := 0
		for _, e := range entries {
			if e.IsDir() && strings.HasPrefix(e.Name(), "kubepods-") {
				count++
			}
		}
		metrics.PodCount = count
	}

	// 2. OOMKills: check /proc/vmstat oom_kill or cgroup memory.events
	vmstatPath := filepath.Join(c.procPath, "vmstat")
	if data, err := os.ReadFile(vmstatPath); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "oom_kill ") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					if v, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
						metrics.OOMKills = v
					}
				}
				break
			}
		}
	}
	if metrics.OOMKills == 0 {
		cgroupMemEvents := filepath.Join(c.procPath, "sys", "fs", "cgroup", "kubepods.slice", "memory.events")
		if _, err := os.Stat(cgroupMemEvents); err != nil {
			cgroupMemEvents = "/sys/fs/cgroup/kubepods.slice/memory.events"
		}
		if data, err := os.ReadFile(cgroupMemEvents); err == nil {
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "oom_kill ") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						if v, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
							metrics.OOMKills = v
						}
					}
					break
				}
			}
		}
	}

	// 3. CPUThrottledPeriods: /sys/fs/cgroup/kubepods.slice/cpu.stat line nr_throttled
	cpuStatPath := filepath.Join(c.procPath, "sys", "fs", "cgroup", "kubepods.slice", "cpu.stat")
	if _, err := os.Stat(cpuStatPath); err != nil {
		cpuStatPath = "/sys/fs/cgroup/kubepods.slice/cpu.stat"
	}
	if data, err := os.ReadFile(cpuStatPath); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "nr_throttled ") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					if v, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
						metrics.CPUThrottledPeriods = v
					}
				}
				break
			}
		}
	}

	return metrics
}
