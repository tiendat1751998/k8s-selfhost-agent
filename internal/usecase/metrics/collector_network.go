package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)


// NetworkInterface holds throughput metrics for a specific network interface.
type NetworkInterface struct {
	Name          string `json:"name"`
	RxBytesPerSec int64  `json:"rx_bytes_per_sec"`
	TxBytesPerSec int64  `json:"tx_bytes_per_sec"`
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

// ScrapeAgent scrapes a single registered compute host agent.
func (c *Collector) ScrapeAgent(ctx context.Context, host docker.ComputeHost) {
	url := formatAgentURL(host.Endpoint, host.TLSEnabled)
	if url == "" {
		return
	}

	reqCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		c.recordAgentFailure(ctx, host, err)
		return
	}

	if host.Labels != nil {
		if token, ok := host.Labels["auth_token"]; ok && token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		} else if token, ok := host.Labels["token"]; ok && token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}

	client := c.httpClient
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		c.recordAgentFailure(ctx, host, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.recordAgentFailure(ctx, host, fmt.Errorf("unexpected HTTP status: %d", resp.StatusCode))
		return
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

	var payload agentMetricsPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		c.recordAgentFailure(ctx, host, fmt.Errorf("decoding agent response: %w", err))
		return
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

	am := &AgentMetrics{
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
	}

	c.recordAgentSuccess(ctx, host, am, now)
}

func (c *Collector) recordAgentSuccess(ctx context.Context, host docker.ComputeHost, am *AgentMetrics, now time.Time) {
	c.agentMu.Lock()
	c.agentMetrics[host.ID] = am
	c.agentMu.Unlock()

	repoCtx := ctx
	if tenancy.TenantIDFromContext(repoCtx) == "" {
		repoCtx = tenancy.WithTenantID(repoCtx, "default-tenant")
	}

	if c.computeHostRepo != nil {
		_ = c.computeHostRepo.UpdateStatus(repoCtx, host.ID, "connected", now)
	}

	if c.incRepo != nil {
		activeInc, getErr := c.incRepo.GetByPodAndType(repoCtx, "infrastructure", host.Name, incident.TypeNodeNotReady)
		if getErr != nil {
			c.logger.Warn("Failed to query active incident on agent recovery", zap.String("host", host.Name), zap.Error(getErr))
		} else if activeInc != nil {
			if activeInc.Status == incident.StatusDetected || activeInc.Status == incident.StatusAnalyzing || activeInc.Status == incident.StatusRemediating || activeInc.Status == incident.StatusFailed {
				if resErr := activeInc.MarkResolved(); resErr == nil {
					if updateErr := c.incRepo.Update(repoCtx, activeInc); updateErr == nil {
						if c.broadcaster != nil {
							c.broadcaster.Broadcast("incident_resolved", activeInc)
						}
					} else {
						c.logger.Error("Failed to update resolved incident on agent recovery", zap.String("host", host.Name), zap.Error(updateErr))
					}
				}
			}
		}
	}
}

func (c *Collector) recordAgentFailure(ctx context.Context, host docker.ComputeHost, err error) {
	c.logger.Debug("Agent scrape failed", zap.String("host_id", host.ID), zap.String("host_name", host.Name), zap.Error(err))

	now := time.Now().UTC()
	c.agentMu.Lock()
	am, exists := c.agentMetrics[host.ID]
	if !exists {
		am = &AgentMetrics{
			Hostname:          host.Name,
			OS:                "linux",
			Arch:              "amd64",
			OSDistro:          "Linux",
			KernelVersion:     "Linux",
			TopProcesses:      make([]ProcessMetric, 0),
			NetworkInterfaces: make([]NetworkInterface, 0),
			DiskIO:            DiskIOMetrics{Devices: make([]DiskIOStats, 0)},
			Status:            "offline",
			LastSeen:          now,
		}
		c.agentMetrics[host.ID] = am
	} else {
		am.Status = "offline"
		am.LastSeen = now
		if am.OSDistro == "" {
			am.OSDistro = normalizeOSDistro(am.OSDistro, am.OS)
		}
		if am.KernelVersion == "" {
			am.KernelVersion = normalizeKernelVersion(am.KernelVersion, am.OS)
		}
		if am.TopProcesses == nil {
			am.TopProcesses = make([]ProcessMetric, 0)
		}
		if am.NetworkInterfaces == nil {
			am.NetworkInterfaces = make([]NetworkInterface, 0)
		}
		if am.DiskIO.Devices == nil {
			am.DiskIO.Devices = make([]DiskIOStats, 0)
		}
	}
	c.agentMu.Unlock()

	repoCtx := ctx
	if tenancy.TenantIDFromContext(repoCtx) == "" {
		repoCtx = tenancy.WithTenantID(repoCtx, "default-tenant")
	}

	if c.computeHostRepo != nil {
		_ = c.computeHostRepo.UpdateStatus(repoCtx, host.ID, "disconnected", now)
	}

	if c.incRepo != nil {
		activeInc, getErr := c.incRepo.GetByPodAndType(repoCtx, "infrastructure", host.Name, incident.TypeNodeNotReady)
		if getErr != nil {
			c.logger.Warn("Failed to query existing incident on agent failure", zap.String("host", host.Name), zap.Error(getErr))
		}
		if activeInc == nil || activeInc.Status == incident.StatusResolved {
			newInc, newErr := incident.New("fleet-primary", "infrastructure", host.Name, incident.TypeNodeNotReady, incident.SeverityCritical, fmt.Sprintf("Infrastructure host '%s' is unreachable: agent at %s is down (%v)", host.Name, host.Endpoint, err))
			if newErr == nil {
				if saveErr := c.incRepo.Create(repoCtx, newInc); saveErr == nil {
					if c.broadcaster != nil {
						c.broadcaster.Broadcast("incident", newInc)
					}
				} else {
					c.logger.Error("Failed to create incident on agent failure", zap.String("host", host.Name), zap.Error(saveErr))
				}
			}
		}
	}
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

