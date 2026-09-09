package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type diskDevSnapshot struct {
	readsCompleted   uint64
	sectorsRead      uint64
	readTimeMs       uint64
	writesCompleted  uint64
	sectorsWritten   uint64
	writeTimeMs      uint64
	ioInProgress     int64
	ioTimeMs         uint64
	weightedIoTimeMs uint64
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

var (
	diskRootSdRegex   = regexp.MustCompile(`^(sd|vd|xvd|hd)[a-z]+$`)
	diskRootNvmeRegex = regexp.MustCompile(`^nvme\d+n\d+$`)
	diskRootMmcRegex  = regexp.MustCompile(`^mmcblk\d+$`)
	diskRootDmRegex   = regexp.MustCompile(`^dm-\d+$`)
	diskRootMdRegex   = regexp.MustCompile(`^md\d+$`)
)

func isIgnoredDiskDevice(devName string) bool {
	devName = strings.TrimSpace(devName)
	if devName == "" {
		return true
	}
	if strings.HasPrefix(devName, "loop") ||
		strings.HasPrefix(devName, "ram") ||
		strings.HasPrefix(devName, "zram") ||
		strings.HasPrefix(devName, "sr") {
		return true
	}
	return false
}

func isRootBlockDevice(devName string) bool {
	devName = strings.TrimSpace(devName)
	if isIgnoredDiskDevice(devName) {
		return false
	}
	if diskRootSdRegex.MatchString(devName) ||
		diskRootNvmeRegex.MatchString(devName) ||
		diskRootMmcRegex.MatchString(devName) ||
		diskRootDmRegex.MatchString(devName) ||
		diskRootMdRegex.MatchString(devName) {
		return true
	}
	return false
}

func (c *SystemCollector) collectDiskIO(now time.Time) DiskIOMetrics {
	diskstatsPath := filepath.Join(c.procPath, "diskstats")
	data, err := os.ReadFile(diskstatsPath)
	if err != nil {
		return DiskIOMetrics{
			Devices: make([]DiskIOStats, 0),
		}
	}

	var deltaSec float64
	hasPrevTime := !c.prevDiskTime.IsZero()
	if hasPrevTime {
		deltaSec = now.Sub(c.prevDiskTime).Seconds()
	}

	newSnap := make(map[string]diskDevSnapshot)
	devices := make([]DiskIOStats, 0)

	var totalReadBytesPerSec, totalWriteBytesPerSec int64
	var totalReadIOPS, totalWriteIOPS float64
	var maxIoUtil float64
	var totalRootDeltaReads, totalRootDeltaWrites uint64
	var totalRootDeltaReadTimeMs, totalRootDeltaWriteTimeMs uint64

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 14 {
			continue
		}

		devName := fields[2]
		if isIgnoredDiskDevice(devName) {
			continue
		}

		readsCompleted, _ := strconv.ParseUint(fields[3], 10, 64)
		sectorsRead, _ := strconv.ParseUint(fields[5], 10, 64)
		readTimeMs, _ := strconv.ParseUint(fields[6], 10, 64)
		writesCompleted, _ := strconv.ParseUint(fields[7], 10, 64)
		sectorsWritten, _ := strconv.ParseUint(fields[9], 10, 64)
		writeTimeMs, _ := strconv.ParseUint(fields[10], 10, 64)
		ioInProgress, _ := strconv.ParseInt(fields[11], 10, 64)
		ioTimeMs, _ := strconv.ParseUint(fields[12], 10, 64)
		weightedIoTimeMs, _ := strconv.ParseUint(fields[13], 10, 64)

		curr := diskDevSnapshot{
			readsCompleted:   readsCompleted,
			sectorsRead:      sectorsRead,
			readTimeMs:       readTimeMs,
			writesCompleted:  writesCompleted,
			sectorsWritten:   sectorsWritten,
			writeTimeMs:      writeTimeMs,
			ioInProgress:     ioInProgress,
			ioTimeMs:         ioTimeMs,
			weightedIoTimeMs: weightedIoTimeMs,
		}
		newSnap[devName] = curr

		isRoot := isRootBlockDevice(devName)

		var readBytesPerSec, writeBytesPerSec int64
		var readIOPS, writeIOPS float64
		var avgWaitMs, avgReqSizeKB, ioUtilPct float64

		if hasPrevTime && deltaSec >= 0.1 {
			prev, ok := c.prevDiskStats[devName]
			if ok {
				var deltaReads, deltaSectorsRead, deltaReadTimeMs uint64
				var deltaWrites, deltaSectorsWritten, deltaWriteTimeMs uint64
				var deltaIoTimeMs uint64

				if curr.readsCompleted >= prev.readsCompleted {
					deltaReads = curr.readsCompleted - prev.readsCompleted
				}
				if curr.sectorsRead >= prev.sectorsRead {
					deltaSectorsRead = curr.sectorsRead - prev.sectorsRead
				}
				if curr.readTimeMs >= prev.readTimeMs {
					deltaReadTimeMs = curr.readTimeMs - prev.readTimeMs
				}
				if curr.writesCompleted >= prev.writesCompleted {
					deltaWrites = curr.writesCompleted - prev.writesCompleted
				}
				if curr.sectorsWritten >= prev.sectorsWritten {
					deltaSectorsWritten = curr.sectorsWritten - prev.sectorsWritten
				}
				if curr.writeTimeMs >= prev.writeTimeMs {
					deltaWriteTimeMs = curr.writeTimeMs - prev.writeTimeMs
				}
				if curr.ioTimeMs >= prev.ioTimeMs {
					deltaIoTimeMs = curr.ioTimeMs - prev.ioTimeMs
				}

				readBytesPerSec = int64(float64(deltaSectorsRead*512) / deltaSec)
				writeBytesPerSec = int64(float64(deltaSectorsWritten*512) / deltaSec)
				readIOPS = math.Round((float64(deltaReads)/deltaSec)*100) / 100
				writeIOPS = math.Round((float64(deltaWrites)/deltaSec)*100) / 100

				totalIOs := deltaReads + deltaWrites
				if totalIOs > 0 {
					avgWaitMs = math.Round((float64(deltaReadTimeMs+deltaWriteTimeMs)/float64(totalIOs))*100) / 100
					avgReqSizeKB = math.Round((float64((deltaSectorsRead+deltaSectorsWritten)*512)/float64(totalIOs)/1024)*100) / 100
				}

				ioUtilPct = math.Min(100.0, math.Round((float64(deltaIoTimeMs)/(deltaSec*1000.0)*100.0)*100)/100)

				if isRoot {
					totalReadBytesPerSec += readBytesPerSec
					totalWriteBytesPerSec += writeBytesPerSec
					totalReadIOPS += readIOPS
					totalWriteIOPS += writeIOPS
					totalRootDeltaReads += deltaReads
					totalRootDeltaWrites += deltaWrites
					totalRootDeltaReadTimeMs += deltaReadTimeMs
					totalRootDeltaWriteTimeMs += deltaWriteTimeMs
					if ioUtilPct > maxIoUtil {
						maxIoUtil = ioUtilPct
					}
				}
			}
		}

		devices = append(devices, DiskIOStats{
			DeviceName:        devName,
			ReadBytesPerSec:   readBytesPerSec,
			WriteBytesPerSec:  writeBytesPerSec,
			ReadIOPS:          readIOPS,
			WriteIOPS:         writeIOPS,
			AvgWaitMs:         avgWaitMs,
			AvgRequestSizeKB:  avgReqSizeKB,
			CurrentQueueDepth: ioInProgress,
			IoUtilizationPct:  ioUtilPct,
			IsRootDevice:      isRoot,
		})
	}

	c.prevDiskStats = newSnap
	c.prevDiskTime = now

	// Robust Fallback: If no root devices were matched or root devices reported 0 while sub-devices had active I/O,
	// sum active non-ignored block devices
	if totalReadBytesPerSec == 0 && totalWriteBytesPerSec == 0 && totalReadIOPS == 0 && totalWriteIOPS == 0 {
		for _, dev := range devices {
			if !isIgnoredDiskDevice(dev.DeviceName) {
				totalReadBytesPerSec += dev.ReadBytesPerSec
				totalWriteBytesPerSec += dev.WriteBytesPerSec
				totalReadIOPS += dev.ReadIOPS
				totalWriteIOPS += dev.WriteIOPS
				if dev.IoUtilizationPct > maxIoUtil {
					maxIoUtil = dev.IoUtilizationPct
				}
			}
		}
	}

	var avgAwaitMs float64
	totalRootIOs := totalRootDeltaReads + totalRootDeltaWrites
	if totalRootIOs > 0 {
		avgAwaitMs = math.Round((float64(totalRootDeltaReadTimeMs+totalRootDeltaWriteTimeMs)/float64(totalRootIOs))*100) / 100
	} else if len(devices) > 0 {
		var waitSum float64
		var waitCount int
		for _, dev := range devices {
			if dev.AvgWaitMs > 0 {
				waitSum += dev.AvgWaitMs
				waitCount++
			}
		}
		if waitCount > 0 {
			avgAwaitMs = math.Round((waitSum/float64(waitCount))*100) / 100
		}
	}

	return DiskIOMetrics{
		TotalReadBytesPerSec:  totalReadBytesPerSec,
		TotalWriteBytesPerSec: totalWriteBytesPerSec,
		TotalReadIOPS:         math.Round(totalReadIOPS*100) / 100,
		TotalWriteIOPS:        math.Round(totalWriteIOPS*100) / 100,
		AvgAwaitMs:            avgAwaitMs,
		MaxIoUtilizationPct:   math.Round(maxIoUtil*100) / 100,
		Devices:               devices,
	}
}
