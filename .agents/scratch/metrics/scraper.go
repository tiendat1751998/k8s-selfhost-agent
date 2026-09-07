package metrics

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"

	"github.com/datdt/k8sselfhost/internal/domain/provider/docker"
)

// Allowed and ignored filesystem tables for physical disk filtering.
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

func formatAgentURL(endpoint string, tlsEnabled bool) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return ""
	}
	var u string
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		u = endpoint
	} else if tlsEnabled {
		u = "https://" + endpoint
	} else {
		u = "http://" + endpoint
	}
	if !strings.HasSuffix(u, "/metrics") {
		u = strings.TrimSuffix(u, "/") + "/metrics"
	}
	return u
}

func normalizeOSDistro(distro, osName string) string {
	d := strings.TrimSpace(distro)
	o := strings.TrimSpace(osName)
	if d != "" && !strings.EqualFold(d, "linux") && !strings.EqualFold(d, "unknown") {
		return d
	}
	if strings.EqualFold(o, "darwin") || strings.EqualFold(d, "darwin") {
		return "macOS"
	}
	if strings.EqualFold(o, "windows") || strings.EqualFold(d, "windows") {
		return "Windows"
	}
	return "Linux"
}

func normalizeKernelVersion(kernel, osName string) string {
	k := strings.TrimSpace(kernel)
	o := strings.TrimSpace(osName)
	if k != "" && !strings.EqualFold(k, "unknown") {
		return k
	}
	if strings.EqualFold(o, "darwin") {
		return "Darwin"
	}
	if strings.EqualFold(o, "windows") {
		return "Windows NT"
	}
	return "Linux"
}

// calculateCPUPercent calculates CPU usage percentage from Docker container stats.
func calculateCPUPercent(stats *container.StatsResponse) float64 {
	if stats == nil {
		return 0.0
	}

	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage) - float64(stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage) - float64(stats.PreCPUStats.SystemUsage)

	if systemDelta > 0.0 && cpuDelta > 0.0 {
		numCPUs := float64(stats.CPUStats.OnlineCPUs)
		if numCPUs == 0 {
			numCPUs = float64(len(stats.CPUStats.CPUUsage.PercpuUsage))
		}
		if numCPUs == 0 {
			numCPUs = 1.0
		}
		cpuPercent := (cpuDelta / systemDelta) * numCPUs * 100.0
		return math.Round(cpuPercent*100) / 100
	}

	return 0.0
}

// ScrapeContainerStats fetches container stats from Docker daemon.
func ScrapeContainerStats(ctx context.Context, client DockerAPIClient, containerID string) (*container.StatsResponse, error) {
	if client == nil {
		return nil, fmt.Errorf("nil docker client")
	}
	statCtx, statCancel := context.WithTimeout(ctx, 2*time.Second)
	defer statCancel()

	statsReader, err := client.ContainerStats(statCtx, containerID, false)
	if err != nil {
		return nil, err
	}
	if statsReader.Body == nil {
		return nil, fmt.Errorf("empty stats body")
	}
	defer statsReader.Body.Close()

	var stats container.StatsResponse
	if decodeErr := json.NewDecoder(statsReader.Body).Decode(&stats); decodeErr != nil {
		return nil, decodeErr
	}
	return &stats, nil
}

// ScrapeDockerDiskUsage calculates disk usage percentage across Docker layers, images, containers, volumes, and build cache.
func ScrapeDockerDiskUsage(ctx context.Context, client DockerAPIClient) (float64, error) {
	if client == nil {
		return 0, fmt.Errorf("nil docker client")
	}
	duCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	du, duErr := client.DiskUsage(duCtx, types.DiskUsageOptions{})
	if duErr != nil {
		return 0, duErr
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
		return pct, nil
	}

	return 0, nil
}
type agentDiskPayload struct {
	MountPoint   string  `json:"mount_point"`
	TotalBytes   int64   `json:"total_bytes"`
	UsedBytes    int64   `json:"used_bytes"`
	UsagePercent float64 `json:"usage_percent"`
	Filesystem   string  `json:"filesystem"`
}

type agentMetricsPayload struct {
	Hostname      string             `json:"hostname"`
	OS            string             `json:"os"`
	Arch          string             `json:"arch"`
	OSDistro      string             `json:"os_distro"`
	KernelVersion string             `json:"kernel_version"`
	UptimeSeconds int64              `json:"uptime_seconds"`
	LoadAverage   [3]float64         `json:"load_average"`
	CPU           struct {
		Count        int     `json:"count"`
		UsagePercent float64 `json:"usage_percent"`
	} `json:"cpu"`
	Memory struct {
		TotalBytes     int64   `json:"total_bytes"`
		UsedBytes      int64   `json:"used_bytes"`
		AvailableBytes int64   `json:"available_bytes"`
		UsagePercent   float64 `json:"usage_percent"`
	} `json:"memory"`
	Disks   []agentDiskPayload `json:"disks"`
	DiskIO  DiskIOMetrics      `json:"disk_io"`
	Network struct {
		Interfaces []struct {
			Name          string `json:"name"`
			RxBytesPerSec int64  `json:"rx_bytes_per_sec"`
			TxBytesPerSec int64  `json:"tx_bytes_per_sec"`
		} `json:"interfaces"`
		TotalRxBytesPerSec int64 `json:"total_rx_bytes_per_sec"`
		TotalTxBytesPerSec int64 `json:"total_tx_bytes_per_sec"`
	} `json:"network"`
	Processes    int             `json:"processes"`
	TopProcesses []ProcessMetric `json:"top_processes"`
	CollectedAt  time.Time       `json:"collected_at"`
}

// ScrapeAgentPayload executes an HTTP scrape against a compute host agent and decodes telemetry.
func ScrapeAgentPayload(ctx context.Context, httpClient *http.Client, host docker.ComputeHost) (*AgentMetrics, error) {
	url := formatAgentURL(host.Endpoint, host.TLSEnabled)
	if url == "" {
		return nil, fmt.Errorf("empty agent endpoint")
	}

	reqCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating http request: %w", err)
	}

	if host.Labels != nil {
		if token, ok := host.Labels["auth_token"]; ok && token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		} else if token, ok := host.Labels["token"]; ok && token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}

	client := httpClient
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP status: %d", resp.StatusCode)
	}

	var payload agentMetricsPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decoding agent response: %w", err)
	}

	var diskTotal, diskUsed int64
	seenDisks := make(map[string]bool)
	for _, d := range payload.Disks {
		if isIgnoredMountPoint(d.MountPoint) || !isPhysicalFilesystem(d.Filesystem) {
			continue
		}
		dedupKey := fmt.Sprintf("%s:%d", strings.ToLower(d.Filesystem), d.TotalBytes)
		if seenDisks[dedupKey] {
			continue
		}
		seenDisks[dedupKey] = true
		diskTotal += d.TotalBytes
		diskUsed += d.UsedBytes
	}
	var diskPercent float64
	if diskTotal > 0 {
		diskPercent = math.Round((float64(diskUsed)/float64(diskTotal))*10000) / 100
	}

	now := time.Now().UTC()
	collectedAt := payload.CollectedAt
	if collectedAt.IsZero() {
		collectedAt = now
	}

	hostname := payload.Hostname
	if hostname == "" {
		hostname = host.Name
	}
	osName := payload.OS
	if osName == "" {
		osName = "linux"
	}
	arch := payload.Arch
	if arch == "" {
		arch = "amd64"
	}

	distro := normalizeOSDistro(payload.OSDistro, osName)
	kernel := normalizeKernelVersion(payload.KernelVersion, osName)

	var ifaces []NetworkInterface
	for _, iface := range payload.Network.Interfaces {
		ifaces = append(ifaces, NetworkInterface{
			Name:          iface.Name,
			RxBytesPerSec: iface.RxBytesPerSec,
			TxBytesPerSec: iface.TxBytesPerSec,
		})
	}
	if ifaces == nil {
		ifaces = make([]NetworkInterface, 0)
	}

	topProcs := payload.TopProcesses
	if topProcs == nil {
		topProcs = make([]ProcessMetric, 0)
	}

	diskIODevices := payload.DiskIO.Devices
	if diskIODevices == nil {
		diskIODevices = make([]DiskIOStats, 0)
	}
	payload.DiskIO.Devices = diskIODevices

	return &AgentMetrics{
		Hostname:          hostname,
		OS:                osName,
		Arch:              arch,
		OSDistro:          distro,
		KernelVersion:     kernel,
		CPUUsage:          payload.CPU.UsagePercent,
		CPUCount:          payload.CPU.Count,
		MemTotal:          payload.Memory.TotalBytes,
		MemUsed:           payload.Memory.UsedBytes,
		MemPercent:        payload.Memory.UsagePercent,
		DiskTotal:         diskTotal,
		DiskUsed:          diskUsed,
		DiskPercent:       diskPercent,
		DiskIO:            payload.DiskIO,
		NetRxRate:         payload.Network.TotalRxBytesPerSec,
		NetTxRate:         payload.Network.TotalTxBytesPerSec,
		NetworkInterfaces: ifaces,
		Uptime:            payload.UptimeSeconds,
		LoadAvg:           payload.LoadAverage,
		Processes:         payload.Processes,
		TopProcesses:      topProcs,
		Status:            "online",
		LastSeen:          collectedAt,
	}, nil
}

// PrometheusMetric represents a single parsed Prometheus exposition line.
type PrometheusMetric struct {
	Name      string            `json:"name"`
	Labels    map[string]string `json:"labels"`
	Value     float64           `json:"value"`
	Timestamp time.Time         `json:"timestamp"`
}

// ParsePrometheusMetrics parses standard Prometheus text exposition format.
func ParsePrometheusMetrics(r io.Reader) ([]PrometheusMetric, error) {
	var results []PrometheusMetric
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		nameAndLabels := parts[0]
		valStr := parts[1]

		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			continue
		}

		name := nameAndLabels
		labels := make(map[string]string)

		if idx := strings.Index(nameAndLabels, "{"); idx != -1 && strings.HasSuffix(nameAndLabels, "}") {
			name = nameAndLabels[:idx]
			rawLabels := nameAndLabels[idx+1 : len(nameAndLabels)-1]
			for _, labelPair := range strings.Split(rawLabels, ",") {
				kv := strings.SplitN(labelPair, "=", 2)
				if len(kv) == 2 {
					k := strings.TrimSpace(kv[0])
					v := strings.Trim(strings.TrimSpace(kv[1]), "\"")
					labels[k] = v
				}
			}
		}

		results = append(results, PrometheusMetric{
			Name:      name,
			Labels:    labels,
			Value:     val,
			Timestamp: time.Now().UTC(),
		})
	}

	return results, scanner.Err()
}
