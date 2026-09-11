package main

import (
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"unicode"
)

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
				var parseErr bool
				for i := 1; i < len(fields); i++ {
					v, err := strconv.ParseUint(fields[i], 10, 64)
					if err != nil {
						slog.Warn("failed to parse cpu metric field", slog.String("field", fields[i]), slog.Any("error", err))
						parseErr = true
						break
					}
					sum += v
				}
				if parseErr {
					continue
				}
				idle, err := strconv.ParseUint(fields[4], 10, 64)
				if err != nil {
					slog.Warn("failed to parse cpu idle field", slog.String("field", fields[4]), slog.Any("error", err))
					continue
				}
				var iowait uint64
				if len(fields) >= 6 {
					var err error
					iowait, err = strconv.ParseUint(fields[5], 10, 64)
					if err != nil {
						slog.Warn("failed to parse cpu iowait field", slog.String("field", fields[5]), slog.Any("error", err))
						continue
					}
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
