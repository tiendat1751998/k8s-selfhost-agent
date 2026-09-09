package metrics

import (
	"context"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"go.uber.org/zap"
)

// DiskIOStats represents real-time I/O performance and latency metrics for a block device.
type DiskIOStats struct {
	DeviceName        string  `json:"device_name"`         // e.g. "sda", "nvme0n1", "dm-0"
	ReadBytesPerSec   int64   `json:"read_bytes_per_sec"`  // Read throughput (Bytes/s)
	WriteBytesPerSec  int64   `json:"write_bytes_per_sec"` // Write throughput (Bytes/s)
	ReadIOPS          float64 `json:"read_iops"`           // Read operations per second
	WriteIOPS         float64 `json:"write_iops"`          // Write operations per second
	AvgWaitMs         float64 `json:"avg_wait_ms"`         // await (Average I/O wait time in ms)
	AvgRequestSizeKB  float64 `json:"avg_req_size_kb"`     // avgrq-sz (Average request size in KB)
	CurrentQueueDepth int64   `json:"current_queue_depth"` // in_flight requests currently queued
	IoUtilizationPct  float64 `json:"io_utilization_pct"`  // %util (Disk busy time %)
	IsRootDevice      bool    `json:"is_root_device"`      // true for sda, nvme0n1, false for partitions sda1
}

// DiskIOMetrics aggregates host-level disk I/O metrics across block devices.
type DiskIOMetrics struct {
	TotalReadBytesPerSec  int64         `json:"total_read_bytes_per_sec"`
	TotalWriteBytesPerSec int64         `json:"total_write_bytes_per_sec"`
	TotalReadIOPS         float64       `json:"total_read_iops"`
	TotalWriteIOPS        float64       `json:"total_write_iops"`
	AvgAwaitMs            float64       `json:"avg_await_ms"`
	MaxIoUtilizationPct   float64       `json:"max_io_util_pct"`
	Devices               []DiskIOStats `json:"devices"`
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

// pollDockerDiskUsage periodically updates Docker daemon layer disk usage in the background (every 60s)
// to avoid blocking the fast 5s CollectOnce tick.
func (c *Collector) pollDockerDiskUsage(ctx context.Context) {
	if c.dockerClient == nil {
		return
	}
	c.updateDockerDiskUsage(ctx)

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.updateDockerDiskUsage(ctx)
		}
	}
}

func (c *Collector) updateDockerDiskUsage(ctx context.Context) {
	if c.dockerClient == nil {
		return
	}
	duCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	du, duErr := c.dockerClient.DiskUsage(duCtx, types.DiskUsageOptions{})
	if duErr != nil {
		c.logger.Debug("Failed to fetch Docker disk usage", zap.Error(duErr))
		return
	}

	var diskUsed int64
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

	if diskUsed > 0 {
		diskTotal := int64(100 * 1024 * 1024 * 1024) // 100 GiB baseline
		if diskUsed > diskTotal {
			diskTotal = diskUsed * 2
		}
		pct := (float64(diskUsed) / float64(diskTotal)) * 100.0
		c.dockerDiskMu.Lock()
		c.dockerDiskPercent = pct
		c.dockerDiskMu.Unlock()
	}
}

