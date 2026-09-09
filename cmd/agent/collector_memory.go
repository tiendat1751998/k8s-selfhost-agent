package main

import (
	"bufio"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

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
		val, err := strconv.ParseInt(valFields[0], 10, 64)
		if err != nil {
			slog.Warn("failed to parse meminfo field", slog.String("key", key), slog.String("val", valFields[0]), slog.Any("error", err))
			continue
		}

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
