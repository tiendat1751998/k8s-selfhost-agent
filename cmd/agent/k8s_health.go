package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// UnhealthyContainerInfo holds diagnostic details of a non-running or unhealthy container.
type UnhealthyContainerInfo struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Image        string `json:"image"`
	State        string `json:"state"`
	Status       string `json:"status"`
	RestartCount int    `json:"restart_count"`
}

// NodeK8sHealthResponse represents structured Kubernetes node-level diagnostics.
type NodeK8sHealthResponse struct {
	NodeName            string                   `json:"node_name"`
	KubeletHealthy      bool                     `json:"kubelet_healthy"`
	KubeletLatencyMs    float64                  `json:"kubelet_latency_ms"`
	PLEGHealthy         bool                     `json:"pleg_healthy"`
	ContainerdHealthy   bool                     `json:"containerd_healthy"`
	DockerHealthy       bool                     `json:"docker_healthy"`
	CRISocketPath       string                   `json:"cri_socket_path,omitempty"`
	DiskPressure        bool                     `json:"disk_pressure"`
	InodePressure       bool                     `json:"inode_pressure"`
	MemoryPressure      bool                     `json:"memory_pressure"`
	PIDPressure         bool                     `json:"pid_pressure"`
	DStateProcesses     int                      `json:"d_state_processes"`
	ZombieCount         int                      `json:"zombie_count"`
	ConntrackUsagePct   float64                  `json:"conntrack_usage_pct"`
	LastLeaseRenewUnix  int64                    `json:"last_lease_renew_unix,omitempty"`
	UnhealthyContainers []UnhealthyContainerInfo `json:"unhealthy_containers"`
	DetectedAnomalies   []string                 `json:"detected_anomalies"`
	CheckedAt           string                   `json:"checked_at"`
}

// RuntimeProber defines methods to probe local Kubelet and container runtime sockets.
type RuntimeProber interface {
	ProbeKubelet(ctx context.Context) (healthy bool, latencyMs float64, plegHealthy bool)
	ProbeSocket(sockPath string, timeout time.Duration) bool
}

// defaultRuntimeProber is the production implementation of RuntimeProber.
type defaultRuntimeProber struct {
	client *http.Client
}

func newDefaultRuntimeProber() *defaultRuntimeProber {
	return &defaultRuntimeProber{
		client: &http.Client{Timeout: 1 * time.Second},
	}
}

func (p *defaultRuntimeProber) ProbeKubelet(ctx context.Context) (bool, float64, bool) {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:10248/healthz", nil)
	if err == nil {
		resp, err := p.client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				lat := float64(time.Since(start).Microseconds()) / 1000.0
				return true, lat, lat < 5000.0
			}
		}
	}

	// Fallback to TCP probe on Kubelet HTTPS port 10250
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", "127.0.0.1:10250")
	if err == nil {
		_ = conn.Close()
		lat := float64(time.Since(start).Microseconds()) / 1000.0
		return true, lat, true
	}

	return false, 0, false
}

func (p *defaultRuntimeProber) ProbeSocket(sockPath string, timeout time.Duration) bool {
	conn, err := net.DialTimeout("unix", sockPath, timeout)
	if err == nil {
		_ = conn.Close()
		return true
	}
	return false
}

// K8sHealthChecker probes local host runtime health.
type K8sHealthChecker struct {
	collector *SystemCollector
	prober    RuntimeProber
	procPath  string
	runner    CommandRunner
}

// HealthCheckerOption configures K8sHealthChecker.
type HealthCheckerOption func(*K8sHealthChecker)

// WithRuntimeProber injects custom prober implementation.
func WithRuntimeProber(p RuntimeProber) HealthCheckerOption {
	return func(h *K8sHealthChecker) {
		if p != nil {
			h.prober = p
		}
	}
}

// WithK8sProcPath configures custom /proc path for scanning.
func WithK8sProcPath(p string) HealthCheckerOption {
	return func(h *K8sHealthChecker) {
		if p != "" {
			h.procPath = p
		}
	}
}

// WithHealthCommandRunner injects custom CommandRunner for container inspection.
func WithHealthCommandRunner(r CommandRunner) HealthCheckerOption {
	return func(h *K8sHealthChecker) {
		if r != nil {
			h.runner = r
		}
	}
}

// NewK8sHealthChecker constructs a new K8sHealthChecker.
func NewK8sHealthChecker(collector *SystemCollector, opts ...HealthCheckerOption) *K8sHealthChecker {
	checker := &K8sHealthChecker{
		collector: collector,
		prober:    newDefaultRuntimeProber(),
		procPath:  "/proc",
		runner:    &osCommandRunner{},
	}
	for _, opt := range opts {
		opt(checker)
	}
	return checker
}

// CheckHealth runs an exhaustive local health inspection and returns NodeK8sHealthResponse.
func (h *K8sHealthChecker) CheckHealth(ctx context.Context) NodeK8sHealthResponse {
	resp := NodeK8sHealthResponse{
		CheckedAt:           time.Now().UTC().Format(time.RFC3339),
		UnhealthyContainers: make([]UnhealthyContainerInfo, 0),
		DetectedAnomalies:   make([]string, 0),
	}

	// 1. Host identity & resource metrics
	if h.collector != nil {
		metrics := h.collector.GetLastMetrics()
		if metrics != nil {
			resp.NodeName = metrics.Hostname
			for _, d := range metrics.Disks {
				if d.UsagePercent >= 85.0 {
					resp.DiskPressure = true
				}
				if d.InodesUsagePercent >= 85.0 {
					resp.InodePressure = true
				}
			}
			if metrics.Memory.UsagePercent >= 90.0 {
				resp.MemoryPressure = true
			}
			if metrics.Network.ConntrackMax > 0 {
				resp.ConntrackUsagePct = (float64(metrics.Network.ConntrackCount) / float64(metrics.Network.ConntrackMax)) * 100.0
			}
			if metrics.K8s.OOMKills > 0 {
				resp.DetectedAnomalies = append(resp.DetectedAnomalies, "OOM_KILLS_DETECTED")
			}
		}
	}

	// 2. Kubelet health probe
	kHealthy, kLat, plegHealthy := h.prober.ProbeKubelet(ctx)
	resp.KubeletHealthy = kHealthy
	resp.KubeletLatencyMs = kLat
	resp.PLEGHealthy = plegHealthy

	// 3. Container runtime socket probing
	containerdSock := "/run/containerd/containerd.sock"
	dockerSock := "/var/run/docker.sock"
	crioSock := "/var/run/crio/crio.sock"

	if h.prober.ProbeSocket(containerdSock, 1*time.Second) {
		resp.ContainerdHealthy = true
		resp.CRISocketPath = containerdSock
	} else if _, err := os.Stat(containerdSock); err == nil {
		resp.CRISocketPath = containerdSock
	}

	if h.prober.ProbeSocket(dockerSock, 1*time.Second) {
		resp.DockerHealthy = true
		if resp.CRISocketPath == "" {
			resp.CRISocketPath = dockerSock
		}
	}
	if !resp.ContainerdHealthy && h.prober.ProbeSocket(crioSock, 1*time.Second) {
		if resp.CRISocketPath == "" {
			resp.CRISocketPath = crioSock
		}
	}

	// Default to containerdSock if neither docker nor crio succeeded
	if resp.CRISocketPath == "" && !resp.DockerHealthy {
		resp.CRISocketPath = containerdSock
	}

	// 4. Scan /proc for D-state and Zombie processes
	dCount, zCount, totalPIDs := scanProcStates(h.procPath)
	resp.DStateProcesses = dCount
	resp.ZombieCount = zCount
	if totalPIDs >= 30000 {
		resp.PIDPressure = true
	}

	// 5. Detect anomalies
	if !resp.KubeletHealthy {
		resp.DetectedAnomalies = append(resp.DetectedAnomalies, "KUBELET_UNRESPONSIVE")
	}
	if !resp.ContainerdHealthy {
		resp.DetectedAnomalies = append(resp.DetectedAnomalies, "CONTAINERD_SOCKET_DOWN")
	}
	if resp.DiskPressure {
		resp.DetectedAnomalies = append(resp.DetectedAnomalies, "DISK_PRESSURE_HIGH")
	}
	if resp.InodePressure {
		resp.DetectedAnomalies = append(resp.DetectedAnomalies, "INODE_PRESSURE_HIGH")
	}
	if resp.MemoryPressure {
		resp.DetectedAnomalies = append(resp.DetectedAnomalies, "MEMORY_PRESSURE_HIGH")
	}
	if resp.ConntrackUsagePct >= 85.0 {
		resp.DetectedAnomalies = append(resp.DetectedAnomalies, "CONNTRACK_NEAR_FULL")
	}
	if resp.DStateProcesses >= 5 {
		resp.DetectedAnomalies = append(resp.DetectedAnomalies, "D_STATE_PROCESSES_STUCK")
	}
	if resp.ZombieCount > 0 {
		resp.DetectedAnomalies = append(resp.DetectedAnomalies, "ZOMBIE_PROCESSES_DETECTED")
	}

	// 6. Inspect local containers
	resp.UnhealthyContainers = detectUnhealthyContainers(ctx, h.runner)
	if len(resp.UnhealthyContainers) > 0 {
		resp.DetectedAnomalies = append(resp.DetectedAnomalies, "UNHEALTHY_CONTAINERS_DETECTED")
	}

	return resp
}

// scanProcStates counts D-state and Zombie processes with zero heap-alloc buffer reuse.
func scanProcStates(procPath string) (dCount int, zCount int, totalPIDs int) {
	entries, err := os.ReadDir(procPath)
	if err != nil {
		return 0, 0, 0
	}

	buf := make([]byte, 512)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if len(name) == 0 || name[0] < '0' || name[0] > '9' {
			continue
		}
		totalPIDs++

		statPath := filepath.Join(procPath, name, "stat")
		f, err := os.Open(statPath)
		if err != nil {
			continue
		}
		n, err := f.Read(buf)
		_ = f.Close()
		if err != nil || n <= 0 {
			continue
		}

		content := buf[:n]
		lastParen := -1
		for i := len(content) - 1; i >= 0; i-- {
			if content[i] == ')' {
				lastParen = i
				break
			}
		}
		if lastParen == -1 || lastParen+2 >= len(content) {
			continue
		}

		idx := lastParen + 1
		for idx < len(content) && content[idx] == ' ' {
			idx++
		}
		if idx < len(content) {
			switch content[idx] {
			case 'D':
				dCount++
			case 'Z':
				zCount++
			}
		}
	}
	return dCount, zCount, totalPIDs
}

// RegisterRoutes registers the /k8s/health endpoint to the provided ServeMux.
func (h *K8sHealthChecker) RegisterRoutes(mux *http.ServeMux, isAuthorized func(r *http.Request) bool) {
	mux.HandleFunc("/k8s/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
			return
		}
		if !isAuthorized(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}

		resp := h.CheckHealth(r.Context())
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})
}
