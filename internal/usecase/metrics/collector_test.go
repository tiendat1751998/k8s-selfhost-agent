package metrics

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/system"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/domain/provider/docker"
)

type mockBroadcaster struct {
	mu       sync.Mutex
	messages []struct {
		msgType string
		data    interface{}
	}
}

func (m *mockBroadcaster) Broadcast(msgType string, data interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, struct {
		msgType string
		data    interface{}
	}{msgType: msgType, data: data})
}

func (m *mockBroadcaster) getMessages() []struct {
	msgType string
	data    interface{}
} {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]struct {
		msgType string
		data    interface{}
	}(nil), m.messages...)
}

type mockDockerAPI struct {
	containers []container.Summary
	statsMap   map[string]container.StatsResponse
	info       system.Info
	diskUsage  types.DiskUsage
}

func (m *mockDockerAPI) ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error) {
	return m.containers, nil
}

func (m *mockDockerAPI) ContainerStats(ctx context.Context, containerID string, stream bool) (container.StatsResponseReader, error) {
	stats, ok := m.statsMap[containerID]
	if !ok {
		stats = container.StatsResponse{}
	}
	bodyBytes, _ := json.Marshal(stats)
	return container.StatsResponseReader{
		Body:   io.NopCloser(bytes.NewReader(bodyBytes)),
		OSType: "linux",
	}, nil
}

func (m *mockDockerAPI) Info(ctx context.Context) (system.Info, error) {
	return m.info, nil
}

func (m *mockDockerAPI) DiskUsage(ctx context.Context, options types.DiskUsageOptions) (types.DiskUsage, error) {
	return m.diskUsage, nil
}

type mockComputeHostRepo struct {
	hosts         []docker.ComputeHost
	listAllCalled bool
	updatedStatus map[string]struct {
		status    string
		checkTime time.Time
	}
	mu sync.Mutex
}

func (m *mockComputeHostRepo) Create(ctx context.Context, host *docker.ComputeHost) error { return nil }
func (m *mockComputeHostRepo) GetByID(ctx context.Context, id string) (*docker.ComputeHost, error) {
	return nil, nil
}
func (m *mockComputeHostRepo) List(ctx context.Context, tenantID string) ([]docker.ComputeHost, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.hosts, nil
}
func (m *mockComputeHostRepo) ListAll(ctx context.Context) ([]docker.ComputeHost, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listAllCalled = true
	return m.hosts, nil
}
func (m *mockComputeHostRepo) Update(ctx context.Context, host *docker.ComputeHost) error { return nil }
func (m *mockComputeHostRepo) Delete(ctx context.Context, id string) error               { return nil }
func (m *mockComputeHostRepo) UpdateStatus(ctx context.Context, id string, status string, lastHealthCheck time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updatedStatus == nil {
		m.updatedStatus = make(map[string]struct {
			status    string
			checkTime time.Time
		})
	}
	m.updatedStatus[id] = struct {
		status    string
		checkTime time.Time
	}{status: status, checkTime: lastHealthCheck}
	return nil
}

func TestCalculateCPUPercent(t *testing.T) {
	tests := []struct {
		name     string
		stats    *container.StatsResponse
		expected float64
	}{
		{
			name:     "nil stats",
			stats:    nil,
			expected: 0.0,
		},
		{
			name: "normal cpu calculation with online cpus",
			stats: &container.StatsResponse{
				CPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage: 200000000,
					},
					SystemUsage: 2000000000,
					OnlineCPUs:  4,
				},
				PreCPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage: 100000000,
					},
					SystemUsage: 1000000000,
				},
			},
			// cpuDelta = 100000000, systemDelta = 1000000000, ratio = 0.1 * 4 cpus * 100 = 40.0
			expected: 40.0,
		},
		{
			name: "zero system delta",
			stats: &container.StatsResponse{
				CPUStats: container.CPUStats{
					CPUUsage:    container.CPUUsage{TotalUsage: 200},
					SystemUsage: 1000,
				},
				PreCPUStats: container.CPUStats{
					CPUUsage:    container.CPUUsage{TotalUsage: 100},
					SystemUsage: 1000,
				},
			},
			expected: 0.0,
		},
		{
			name: "fallback to percpu slice when online cpus is zero",
			stats: &container.StatsResponse{
				CPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage:  2000,
						PercpuUsage: []uint64{1000, 1000},
					},
					SystemUsage: 20000,
					OnlineCPUs:  0,
				},
				PreCPUStats: container.CPUStats{
					CPUUsage:    container.CPUUsage{TotalUsage: 1000},
					SystemUsage: 10000,
				},
			},
			// cpuDelta = 1000, sysDelta = 10000 -> 0.1 * 2 * 100 = 20.0
			expected: 20.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateCPUPercent(tt.stats)
			if got != tt.expected {
				t.Errorf("calculateCPUPercent() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCollector_GenerateAlerts(t *testing.T) {
	collector := NewCollector(nil, nil, nil, zap.NewNop(), WithThresholds(Thresholds{
		CPUWarning:    80.0,
		MemoryWarning: 85.0,
		DiskWarning:   90.0,
	}))

	overview := &SystemOverview{
		Nodes: []NodeMetrics{
			{
				NodeID:        "node-1",
				NodeName:      "manager-1",
				Status:        "ready",
				CPUPercent:    85.5,
				MemoryPercent: 70.0,
				DiskPercent:   60.0,
			},
			{
				NodeID:        "node-2",
				NodeName:      "worker-1",
				Status:        "down",
				CPUPercent:    10.0,
				MemoryPercent: 90.0,
				DiskPercent:   95.0,
			},
			{
				NodeID:        "node-3",
				NodeName:      "worker-2",
				Status:        "ready",
				CPUPercent:    40.0,
				MemoryPercent: 50.0,
				DiskPercent:   30.0,
			},
		},
	}

	alerts := collector.generateAlerts(overview)

	if len(alerts) != 4 {
		t.Fatalf("expected 4 alerts, got %d: %+v", len(alerts), alerts)
	}

	alertTypes := make(map[string]int)
	for _, a := range alerts {
		alertTypes[a.Type]++
	}

	if alertTypes["cpu_high"] != 1 {
		t.Errorf("expected 1 cpu_high alert, got %d", alertTypes["cpu_high"])
	}
	if alertTypes["node_down"] != 1 {
		t.Errorf("expected 1 node_down alert, got %d", alertTypes["node_down"])
	}
	if alertTypes["memory_high"] != 1 {
		t.Errorf("expected 1 memory_high alert, got %d", alertTypes["memory_high"])
	}
	if alertTypes["disk_high"] != 1 {
		t.Errorf("expected 1 disk_high alert, got %d", alertTypes["disk_high"])
	}
}

func TestCollector_CollectOnce_AgentTopology(t *testing.T) {
	mockDocker := &mockDockerAPI{
		info: system.Info{
			ID:       "engine-1",
			Name:     "master-server",
			MemTotal: 16 * 1024 * 1024 * 1024,
			NCPU:     8,
		},
		diskUsage: types.DiskUsage{
			LayersSize: 5 * 1024 * 1024 * 1024,
			Images: []*image.Summary{
				{Size: 3 * 1024 * 1024 * 1024},
			},
		},
		containers: []container.Summary{
			{
				ID:    "c1",
				Names: []string{"/web-app"},
				Image: "nginx:latest",
				State: "running",
			},
			{
				ID:    "c2",
				Names: []string{"/db-app"},
				Image: "postgres:16",
				State: "exited",
			},
		},
		statsMap: map[string]container.StatsResponse{
			"c1": {
				CPUStats: container.CPUStats{
					CPUUsage:    container.CPUUsage{TotalUsage: 500000000},
					SystemUsage: 1000000000,
					OnlineCPUs:  2,
				},
				PreCPUStats: container.CPUStats{
					CPUUsage:    container.CPUUsage{TotalUsage: 250000000},
					SystemUsage: 500000000,
				},
				MemoryStats: container.MemoryStats{
					Usage: 512 * 1024 * 1024,
					Limit: 1024 * 1024 * 1024,
				},
				Networks: map[string]container.NetworkStats{
					"eth0": {RxBytes: 1024, TxBytes: 2048},
				},
			},
		},
	}

	broadcaster := &mockBroadcaster{}
	var reqCount int64 = 100

	collector := NewCollector(
		mockDocker,
		nil,
		broadcaster,
		zap.NewNop(),
		WithInterval(100*time.Millisecond),
		WithRequestCountFn(func() int64 {
			return reqCount
		}),
	)

	collector.SetAgentMetric("agent-node-1", &AgentMetrics{
		Hostname:      "agent-server-1",
		OS:            "linux",
		Arch:          "amd64",
		OSDistro:      "Ubuntu 22.04.3 LTS",
		KernelVersion: "5.15.0-91-generic",
		CPUUsage:      85.0,
		CPUCount:      4,
		MemTotal:      8 * 1024 * 1024 * 1024,
		MemUsed:       4 * 1024 * 1024 * 1024,
		MemPercent:    50.0,
		DiskTotal:     100 * 1024 * 1024 * 1024,
		DiskUsed:      20 * 1024 * 1024 * 1024,
		DiskPercent:   20.0,
		NetRxRate:     1024,
		NetTxRate:     2048,
		Processes:     10,
		TopProcesses: []ProcessMetric{
			{
				PID:              413,
				Name:             "nginx",
				CommandLine:      "nginx: worker process",
				User:             "www-data",
				CPUPercent:       4.20,
				MemoryBytes:      64 * 1024 * 1024,
				MemoryPercent:    0.78,
				ReadBytesPerSec:  1048576,
				WriteBytesPerSec: 524288,
				State:            "running",
			},
		},
		Status:      "online",
		LastSeen:    time.Now().UTC(),
	})
	collector.SetAgentMetric("agent-node-2", &AgentMetrics{
		Hostname:      "agent-server-2",
		OS:            "linux",
		Arch:          "amd64",
		OSDistro:      "Debian 12",
		KernelVersion: "6.1.0-18-amd64",
		CPUUsage:      10.0,
		CPUCount:      4,
		MemTotal:      8 * 1024 * 1024 * 1024,
		MemUsed:       2 * 1024 * 1024 * 1024,
		MemPercent:    25.0,
		DiskTotal:     100 * 1024 * 1024 * 1024,
		DiskUsed:    10 * 1024 * 1024 * 1024,
		DiskPercent: 10.0,
		NetRxRate:   0,
		NetTxRate:   0,
		Processes:   5,
		Status:      "offline",
		LastSeen:    time.Now().UTC(),
	})

	overview, err := collector.CollectOnce(context.Background())
	if err != nil {
		t.Fatalf("CollectOnce failed: %v", err)
	}

	if overview.TotalNodes != 2 {
		t.Errorf("expected TotalNodes = 2, got %d", overview.TotalNodes)
	}
	if overview.HealthyNodes != 1 {
		t.Errorf("expected HealthyNodes = 1, got %d", overview.HealthyNodes)
	}
	if overview.TotalContainers != 2 {
		t.Errorf("expected TotalContainers = 2, got %d", overview.TotalContainers)
	}
	if overview.RunningContainers != 1 {
		t.Errorf("expected RunningContainers = 1, got %d", overview.RunningContainers)
	}
	if len(overview.Containers) != 2 {
		t.Errorf("expected 2 container metrics, got %d", len(overview.Containers))
	}
	if len(overview.Alerts) != 2 {
		t.Errorf("expected 2 alerts (1 cpu_high, 1 node_down), got %d: %+v", len(overview.Alerts), overview.Alerts)
	}
	if overview.Nodes[0].Source != "agent" {
		t.Errorf("expected node source 'agent', got '%s'", overview.Nodes[0].Source)
	}
	if overview.Nodes[0].Role != "agent" {
		t.Errorf("expected node role 'agent', got '%s'", overview.Nodes[0].Role)
	}

	var foundNode1 *NodeMetrics
	for i := range overview.Nodes {
		if overview.Nodes[i].NodeID == "agent-node-1" {
			foundNode1 = &overview.Nodes[i]
			break
		}
	}
	if foundNode1 == nil {
		t.Fatalf("expected agent-node-1 in overview.Nodes")
	}
	if len(foundNode1.TopProcesses) != 1 {
		t.Fatalf("expected 1 top process on agent-node-1, got %d", len(foundNode1.TopProcesses))
	}
	if foundNode1.TopProcesses[0].PID != 413 || foundNode1.TopProcesses[0].Name != "nginx" {
		t.Errorf("unexpected top process in node metrics: %+v", foundNode1.TopProcesses[0])
	}
	if foundNode1.OSDistro != "Ubuntu 22.04.3 LTS" {
		t.Errorf("expected OSDistro 'Ubuntu 22.04.3 LTS', got '%s'", foundNode1.OSDistro)
	}
	if foundNode1.KernelVersion != "5.15.0-91-generic" {
		t.Errorf("expected KernelVersion '5.15.0-91-generic', got '%s'", foundNode1.KernelVersion)
	}
}

func TestCollector_CollectOnce_Standalone(t *testing.T) {
	mockDocker := &mockDockerAPI{
		info: system.Info{
			ID:                "standalone-engine-1",
			Name:              "my-docker-host",
			MemTotal:          8 * 1024 * 1024 * 1024,
			Containers:        1,
			ContainersRunning: 1,
		},
		containers: []container.Summary{
			{
				ID:    "c-redis",
				Names: []string{"/redis-cache"},
				Image: "redis:alpine",
				State: "running",
			},
		},
		statsMap: map[string]container.StatsResponse{
			"c-redis": {
				CPUStats: container.CPUStats{
					CPUUsage:    container.CPUUsage{TotalUsage: 200000000},
					SystemUsage: 1000000000,
					OnlineCPUs:  1,
				},
				PreCPUStats: container.CPUStats{
					CPUUsage:    container.CPUUsage{TotalUsage: 100000000},
					SystemUsage: 500000000,
				},
				MemoryStats: container.MemoryStats{
					Usage: 128 * 1024 * 1024,
					Limit: 8 * 1024 * 1024 * 1024,
				},
			},
		},
	}

	collector := NewCollector(mockDocker, nil, nil, zap.NewNop())
	collector.SetAgentMetric("agent-1", &AgentMetrics{
		Hostname: "my-agent-host",
		Status:   "online",
		CPUUsage: 15.0,
	})

	overview, err := collector.CollectOnce(context.Background())
	if err != nil {
		t.Fatalf("CollectOnce failed: %v", err)
	}

	if overview.TotalNodes != 1 {
		t.Errorf("expected TotalNodes = 1, got %d", overview.TotalNodes)
	}
	if overview.Nodes[0].NodeName != "my-agent-host" {
		t.Errorf("expected NodeName = my-agent-host, got %s", overview.Nodes[0].NodeName)
	}
	if overview.Nodes[0].Role != "agent" {
		t.Errorf("expected Role = agent, got %s", overview.Nodes[0].Role)
	}
	if overview.Nodes[0].Source != "agent" {
		t.Errorf("expected Source = agent, got %s", overview.Nodes[0].Source)
	}
	if overview.TotalContainers != 1 {
		t.Errorf("expected TotalContainers = 1, got %d", overview.TotalContainers)
	}
	if overview.RunningContainers != 1 {
		t.Errorf("expected RunningContainers = 1, got %d", overview.RunningContainers)
	}
}

func TestCollector_CollectOnce_NilClient(t *testing.T) {
	collector := NewCollector(nil, nil, nil, zap.NewNop())
	overview, err := collector.CollectOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected error on nil client: %v", err)
	}

	if overview == nil {
		t.Fatal("expected non-nil overview")
	}
	if overview.TotalNodes != 0 || overview.TotalContainers != 0 {
		t.Errorf("expected 0 nodes and 0 containers, got %d nodes, %d containers", overview.TotalNodes, overview.TotalContainers)
	}
}

func TestCollector_Start_Stop_Broadcast(t *testing.T) {
	mockDocker := &mockDockerAPI{
		info: system.Info{
			ID:       "engine-test",
			Name:     "test-host",
			MemTotal: 4 * 1024 * 1024 * 1024,
		},
	}
	broadcaster := &mockBroadcaster{}
	collector := NewCollector(
		mockDocker,
		nil,
		broadcaster,
		zap.NewNop(),
		WithInterval(20*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go collector.Start(ctx)

	// Wait for at least 1 collection cycle
	time.Sleep(60 * time.Millisecond)

	snapshot := collector.GetLastSnapshot()
	if snapshot == nil {
		t.Fatal("expected non-nil cached snapshot")
	}

	msgs := broadcaster.getMessages()
	if len(msgs) == 0 {
		t.Fatal("expected broadcast messages to be sent over WebSocket")
	}
	if msgs[0].msgType != "metrics" {
		t.Errorf("expected message type 'metrics', got %s", msgs[0].msgType)
	}

	collector.Stop()
}

func TestCollector_ScrapeAgent_Online(t *testing.T) {
	agentResp := map[string]interface{}{
		"hostname":       "db-server-01",
		"os":             "linux",
		"arch":           "amd64",
		"os_distro":      "Ubuntu 22.04.3 LTS",
		"kernel_version": "5.15.0-91-generic",
		"uptime_seconds": 1234567,
		"load_average":   []float64{0.5, 0.3, 0.2},
		"cpu": map[string]interface{}{
			"count":         8,
			"usage_percent": 23.5,
		},
		"memory": map[string]interface{}{
			"total_bytes":     int64(17179869184),
			"used_bytes":      int64(8589934592),
			"available_bytes": int64(8589934592),
			"usage_percent":   50.0,
		},
		"disks": []map[string]interface{}{
			{
				"mount_point":   "/",
				"total_bytes":   int64(107374182400),
				"used_bytes":    int64(64424509440),
				"usage_percent": 60.0,
				"filesystem":    "ext4",
			},
		},
		"network": map[string]interface{}{
			"interfaces": []map[string]interface{}{
				{
					"name":             "eth0",
					"rx_bytes_per_sec": 1024000,
					"tx_bytes_per_sec": 512000,
				},
			},
			"total_rx_bytes_per_sec": 1024000,
			"total_tx_bytes_per_sec": 512000,
		},
		"processes":    142,
		"top_processes": []map[string]interface{}{
			{
				"pid":                 101,
				"name":                "nginx",
				"command_line":        "nginx: worker process",
				"user":                "www-data",
				"cpu_percent":         12.5,
				"memory_bytes":        int64(67108864),
				"memory_percent":      0.39,
				"read_bytes_per_sec":  int64(1048576),
				"write_bytes_per_sec": int64(524288),
				"state":               "running",
			},
		},
		"collected_at": "2026-08-19T15:20:00Z",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/metrics" {
			http.NotFound(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer secret-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(agentResp)
	}))
	defer srv.Close()

	hostRepo := &mockComputeHostRepo{
		hosts: []docker.ComputeHost{
			{
				ID:       "host-db-01",
				Name:     "db-server-01",
				HostType: "agent",
				Endpoint: srv.URL,
				Labels: map[string]string{
					"auth_token": "secret-token",
				},
			},
		},
	}

	collector := NewCollector(nil, hostRepo, nil, zap.NewNop(), WithHTTPClient(srv.Client()))

	collector.ScrapeAgent(context.Background(), hostRepo.hosts[0])

	metricsMap := collector.GetAgentMetrics()
	m, ok := metricsMap["host-db-01"]
	if !ok {
		t.Fatalf("expected agent metrics for host-db-01")
	}

	if m.Status != "online" {
		t.Errorf("expected status 'online', got '%s'", m.Status)
	}
	if m.Hostname != "db-server-01" {
		t.Errorf("expected hostname 'db-server-01', got '%s'", m.Hostname)
	}
	if m.OSDistro != "Ubuntu 22.04.3 LTS" {
		t.Errorf("expected OSDistro 'Ubuntu 22.04.3 LTS', got '%s'", m.OSDistro)
	}
	if m.KernelVersion != "5.15.0-91-generic" {
		t.Errorf("expected KernelVersion '5.15.0-91-generic', got '%s'", m.KernelVersion)
	}
	if m.CPUUsage != 23.5 {
		t.Errorf("expected CPUUsage 23.5, got %f", m.CPUUsage)
	}
	if m.MemPercent != 50.0 {
		t.Errorf("expected MemPercent 50.0, got %f", m.MemPercent)
	}
	if m.DiskTotal != 107374182400 {
		t.Errorf("expected DiskTotal 107374182400, got %d", m.DiskTotal)
	}
	if m.NetRxRate != 1024000 {
		t.Errorf("expected NetRxRate 1024000, got %d", m.NetRxRate)
	}
	if len(m.TopProcesses) != 1 {
		t.Fatalf("expected 1 top process in agent metrics, got %d", len(m.TopProcesses))
	}
	if m.TopProcesses[0].PID != 101 || m.TopProcesses[0].Name != "nginx" || m.TopProcesses[0].CPUPercent != 12.5 {
		t.Errorf("unexpected top process parsed: %+v", m.TopProcesses[0])
	}

	hostRepo.mu.Lock()
	statusEntry, recorded := hostRepo.updatedStatus["host-db-01"]
	hostRepo.mu.Unlock()

	if !recorded || statusEntry.status != "connected" {
		t.Errorf("expected repo status 'connected', got recorded=%v status=%s", recorded, statusEntry.status)
	}
}

func TestCollector_ScrapeAgent_DiskFilteringAndDeduplication(t *testing.T) {
	agentResp := map[string]interface{}{
		"hostname":       "worker1",
		"os":             "linux",
		"arch":           "amd64",
		"uptime_seconds": 86400,
		"load_average":   [3]float64{0.5, 0.4, 0.3},
		"cpu": map[string]interface{}{
			"count":         4,
			"usage_percent": 15.0,
		},
		"memory": map[string]interface{}{
			"total_bytes":     int64(16000000000),
			"used_bytes":      int64(8000000000),
			"available_bytes": int64(8000000000),
			"usage_percent":   50.0,
		},
		"disks": []map[string]interface{}{
			{
				"mount_point":   "/",
				"total_bytes":   int64(102971269120),
				"used_bytes":    int64(31782694912),
				"usage_percent": 30.86,
				"filesystem":    "ext4",
			},
			{
				"mount_point":   "/boot",
				"total_bytes":   int64(2040373248),
				"used_bytes":    int64(204037324),
				"usage_percent": 10.0,
				"filesystem":    "ext4",
			},
			{
				"mount_point":   "/var/lib/docker/rootfs/overlayfs/1",
				"total_bytes":   int64(102971269120),
				"used_bytes":    int64(31782694912),
				"usage_percent": 30.86,
				"filesystem":    "overlay",
			},
			{
				"mount_point":   "/var/lib/docker/rootfs/overlayfs/2",
				"total_bytes":   int64(102971269120),
				"used_bytes":    int64(31782694912),
				"usage_percent": 30.86,
				"filesystem":    "overlay",
			},
			{
				"mount_point":   "/run",
				"total_bytes":   int64(102971269120),
				"used_bytes":    int64(1024),
				"usage_percent": 0.01,
				"filesystem":    "tmpfs",
			},
			{
				"mount_point":   "/snap/core/123",
				"total_bytes":   int64(50000000),
				"used_bytes":    int64(50000000),
				"usage_percent": 100.0,
				"filesystem":    "squashfs",
			},
			{
				"mount_point":   "/mnt/duplicate_root",
				"total_bytes":   int64(102971269120),
				"used_bytes":    int64(31782694912),
				"usage_percent": 30.86,
				"filesystem":    "ext4",
			},
		},
		"network": map[string]interface{}{
			"total_rx_bytes_per_sec": 1000,
			"total_tx_bytes_per_sec": 500,
		},
		"processes": 50,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(agentResp)
	}))
	defer srv.Close()

	hostRepo := &mockComputeHostRepo{
		hosts: []docker.ComputeHost{
			{
				ID:       "host-worker-1",
				Name:     "worker1",
				HostType: "agent",
				Endpoint: srv.URL,
			},
		},
	}

	collector := NewCollector(nil, hostRepo, nil, zap.NewNop(), WithHTTPClient(srv.Client()))
	collector.ScrapeAgent(context.Background(), hostRepo.hosts[0])

	metricsMap := collector.GetAgentMetrics()
	m, ok := metricsMap["host-worker-1"]
	if !ok {
		t.Fatalf("expected metrics for host-worker-1")
	}

	expectedTotal := int64(102971269120 + 2040373248)
	expectedUsed := int64(31782694912 + 204037324)

	if m.DiskTotal != expectedTotal {
		t.Errorf("expected DiskTotal %d, got %d", expectedTotal, m.DiskTotal)
	}
	if m.DiskUsed != expectedUsed {
		t.Errorf("expected DiskUsed %d, got %d", expectedUsed, m.DiskUsed)
	}

	expectedPct := math.Round((float64(expectedUsed)/float64(expectedTotal))*10000) / 100
	if m.DiskPercent != expectedPct {
		t.Errorf("expected DiskPercent %f, got %f", expectedPct, m.DiskPercent)
	}
}

func TestCollector_ScrapeAgent_Offline(t *testing.T) {
	hostRepo := &mockComputeHostRepo{
		hosts: []docker.ComputeHost{
			{
				ID:       "host-down",
				Name:     "offline-server",
				HostType: "agent",
				Endpoint: "http://127.0.0.1:59997",
			},
		},
	}

	collector := NewCollector(nil, hostRepo, nil, zap.NewNop())

	collector.ScrapeAgent(context.Background(), hostRepo.hosts[0])

	metricsMap := collector.GetAgentMetrics()
	m, ok := metricsMap["host-down"]
	if !ok {
		t.Fatalf("expected agent metrics for host-down")
	}

	if m.Status != "offline" {
		t.Errorf("expected status 'offline', got '%s'", m.Status)
	}

	hostRepo.mu.Lock()
	statusEntry, recorded := hostRepo.updatedStatus["host-down"]
	hostRepo.mu.Unlock()

	if !recorded || statusEntry.status != "disconnected" {
		t.Errorf("expected repo status 'disconnected', got recorded=%v status=%s", recorded, statusEntry.status)
	}
}

func TestCollector_AgentNodesAndContainers(t *testing.T) {
	mockDocker := &mockDockerAPI{
		info: system.Info{
			ID:                "docker-host-1",
			Name:              "docker-node",
			MemTotal:          8 * 1024 * 1024 * 1024,
			Containers:        1,
			ContainersRunning: 1,
		},
		containers: []container.Summary{
			{
				ID:    "c-app",
				Names: []string{"/app"},
				State: "running",
			},
		},
		statsMap: map[string]container.StatsResponse{
			"c-app": {
				CPUStats: container.CPUStats{
					CPUUsage:    container.CPUUsage{TotalUsage: 200000000},
					SystemUsage: 1000000000,
					OnlineCPUs:  1,
				},
				PreCPUStats: container.CPUStats{
					CPUUsage:    container.CPUUsage{TotalUsage: 100000000},
					SystemUsage: 500000000,
				},
				MemoryStats: container.MemoryStats{
					Usage: 2 * 1024 * 1024 * 1024,
					Limit: 8 * 1024 * 1024 * 1024,
				},
			},
		},
	}

	collector := NewCollector(mockDocker, nil, nil, zap.NewNop())

	// Set an agent metric
	collector.SetAgentMetric("agent-host-01", &AgentMetrics{
		Hostname:    "external-db",
		OS:          "linux",
		Arch:        "amd64",
		CPUUsage:    30.0,
		CPUCount:    4,
		MemTotal:    16 * 1024 * 1024 * 1024,
		MemUsed:     8 * 1024 * 1024 * 1024,
		MemPercent:  50.0,
		DiskTotal:   100 * 1024 * 1024 * 1024,
		DiskUsed:    40 * 1024 * 1024 * 1024,
		DiskPercent: 40.0,
		NetRxRate:   5000,
		NetTxRate:   2000,
		Processes:   45,
		Status:      "online",
		LastSeen:    time.Now().UTC(),
	})

	overview, err := collector.CollectOnce(context.Background())
	if err != nil {
		t.Fatalf("CollectOnce failed: %v", err)
	}

	if overview.TotalNodes != 1 {
		t.Fatalf("expected TotalNodes = 1, got %d", overview.TotalNodes)
	}
	if overview.HealthyNodes != 1 {
		t.Errorf("expected HealthyNodes = 1, got %d", overview.HealthyNodes)
	}
	if overview.TotalContainers != 1 {
		t.Errorf("expected TotalContainers = 1, got %d", overview.TotalContainers)
	}

	n := overview.Nodes[0]
	if n.Source != "agent" {
		t.Errorf("expected agent source, got '%s'", n.Source)
	}
	if n.NodeName != "external-db" {
		t.Errorf("expected agent node name 'external-db', got '%s'", n.NodeName)
	}
	if n.Role != "agent" {
		t.Errorf("expected agent role 'agent', got '%s'", n.Role)
	}
	if n.Status != "ready" {
		t.Errorf("expected agent status 'ready', got '%s'", n.Status)
	}
	if n.CPUPercent != 30.0 {
		t.Errorf("expected agent CPUPercent 30.0, got %f", n.CPUPercent)
	}
}

func TestCollector_ScrapeAllAgents_UsesListAll(t *testing.T) {
	agentResp := map[string]interface{}{
		"hostname":     "agent-server-1",
		"os":           "linux",
		"arch":         "amd64",
		"cpu_usage":    15.0,
		"cpu_count":    4,
		"mem_total":    8589934592,
		"mem_used":     4294967296,
		"mem_percent":  50.0,
		"disk_total":   107374182400,
		"disk_used":    53687091200,
		"disk_percent": 50.0,
		"network": map[string]interface{}{
			"total_rx_bytes_per_sec": 1000,
			"total_tx_bytes_per_sec": 2000,
		},
		"processes":    50,
		"collected_at": "2026-08-19T15:20:00Z",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(agentResp)
	}))
	defer srv.Close()

	hostRepo := &mockComputeHostRepo{
		hosts: []docker.ComputeHost{
			{
				ID:       "host-listall-01",
				Name:     "agent-server-1",
				HostType: "agent",
				Endpoint: srv.URL,
			},
		},
	}

	collector := NewCollector(nil, hostRepo, nil, zap.NewNop(), WithHTTPClient(srv.Client()))

	// Calling scrapeAllAgents with context.Background() should invoke ListAll
	collector.scrapeAllAgents(context.Background())

	hostRepo.mu.Lock()
	called := hostRepo.listAllCalled
	hostRepo.mu.Unlock()

	if !called {
		t.Errorf("expected scrapeAllAgents to call ListAll on compute host repo")
	}

	metricsMap := collector.GetAgentMetrics()
	if _, ok := metricsMap["host-listall-01"]; !ok {
		t.Errorf("expected host-listall-01 to be scraped")
	}
}

func TestCollector_RemoveAgentMetric(t *testing.T) {
	collector := NewCollector(nil, nil, nil, zap.NewNop())

	collector.SetAgentMetric("host-to-remove", &AgentMetrics{
		Hostname: "test-host",
		Status:   "online",
	})

	metricsMap := collector.GetAgentMetrics()
	if _, ok := metricsMap["host-to-remove"]; !ok {
		t.Fatalf("expected host-to-remove to be present")
	}

	collector.RemoveAgentMetric("host-to-remove")

	metricsMapAfter := collector.GetAgentMetrics()
	if _, ok := metricsMapAfter["host-to-remove"]; !ok {
		// good
	} else {
		t.Errorf("expected host-to-remove to be deleted, but still present")
	}
}

func TestCollector_ScrapeAllAgents_EvictsDeletedHosts(t *testing.T) {
	agentResp := map[string]interface{}{
		"hostname":     "active-host",
		"os":           "linux",
		"arch":         "amd64",
		"cpu_usage":    10.0,
		"cpu_count":    2,
		"mem_total":    4294967296,
		"mem_used":     2147483648,
		"mem_percent":  50.0,
		"disk_total":   53687091200,
		"disk_used":    26843545600,
		"disk_percent": 50.0,
		"network": map[string]interface{}{
			"total_rx_bytes_per_sec": 500,
			"total_tx_bytes_per_sec": 1000,
		},
		"processes":    20,
		"collected_at": "2026-08-19T15:30:00Z",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(agentResp)
	}))
	defer srv.Close()

	hostRepo := &mockComputeHostRepo{
		hosts: []docker.ComputeHost{
			{
				ID:       "host-active",
				Name:     "active-host",
				HostType: "agent",
				Endpoint: srv.URL,
			},
		},
	}

	collector := NewCollector(nil, hostRepo, nil, zap.NewNop(), WithHTTPClient(srv.Client()))

	// Pre-populate with a stale deleted host and an active host
	collector.SetAgentMetric("host-deleted", &AgentMetrics{
		Hostname: "deleted-host",
		Status:   "offline",
	})
	collector.SetAgentMetric("host-active", &AgentMetrics{
		Hostname: "active-host",
		Status:   "online",
	})

	// Run scrapeAllAgents
	collector.scrapeAllAgents(context.Background())

	metricsMap := collector.GetAgentMetrics()

	if _, ok := metricsMap["host-active"]; !ok {
		t.Errorf("expected host-active to be present in agentMetrics")
	}

	if _, ok := metricsMap["host-deleted"]; ok {
		t.Errorf("expected host-deleted to be evicted from agentMetrics")
	}
}

type mockIncidentRepo struct {
	mu        sync.Mutex
	incidents map[string]*incident.Incident
	created   []*incident.Incident
	updated   []*incident.Incident
}

func newMockIncidentRepo() *mockIncidentRepo {
	return &mockIncidentRepo{
		incidents: make(map[string]*incident.Incident),
	}
}

func (m *mockIncidentRepo) Create(ctx context.Context, inc *incident.Incident) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if inc.ID == "" {
		inc.ID = "inc-test-1"
	}
	m.incidents[inc.Namespace+"/"+inc.PodName+"/"+string(inc.Type)] = inc
	m.created = append(m.created, inc)
	return nil
}

func (m *mockIncidentRepo) GetByID(ctx context.Context, id string) (*incident.Incident, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, inc := range m.incidents {
		if inc.ID == id {
			return inc, nil
		}
	}
	return nil, nil
}

func (m *mockIncidentRepo) Update(ctx context.Context, inc *incident.Incident) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.incidents[inc.Namespace+"/"+inc.PodName+"/"+string(inc.Type)] = inc
	m.updated = append(m.updated, inc)
	return nil
}

func (m *mockIncidentRepo) List(ctx context.Context, filter incident.Filter) ([]*incident.Incident, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var res []*incident.Incident
	for _, inc := range m.incidents {
		res = append(res, inc)
	}
	return res, int64(len(res)), nil
}

func (m *mockIncidentRepo) GetByPodAndType(ctx context.Context, namespace, podName string, incidentType incident.Type) (*incident.Incident, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	inc, ok := m.incidents[namespace+"/"+podName+"/"+string(incidentType)]
	if !ok {
		return nil, nil
	}
	return inc, nil
}

func TestCollector_ScrapeAgent_Failure_CreatesIncidentAndBroadcasts(t *testing.T) {
	hostRepo := &mockComputeHostRepo{
		hosts: []docker.ComputeHost{
			{
				ID:       "host-failing",
				Name:     "worker-srv-01",
				HostType: "agent",
				Endpoint: "http://127.0.0.1:59998",
			},
		},
	}

	incRepo := newMockIncidentRepo()
	broadcaster := &mockBroadcaster{}

	collector := NewCollector(
		nil,
		hostRepo,
		broadcaster,
		zap.NewNop(),
		WithIncidentRepo(incRepo),
	)

	// Scrape failing agent
	collector.ScrapeAgent(context.Background(), hostRepo.hosts[0])

	incRepo.mu.Lock()
	createdCount := len(incRepo.created)
	incRepo.mu.Unlock()

	if createdCount != 1 {
		t.Fatalf("expected 1 incident to be created, got %d", createdCount)
	}

	incRepo.mu.Lock()
	createdInc := incRepo.created[0]
	incRepo.mu.Unlock()

	if createdInc.Type != incident.TypeNodeNotReady {
		t.Errorf("expected incident type %s, got %s", incident.TypeNodeNotReady, createdInc.Type)
	}
	if createdInc.Namespace != "infrastructure" {
		t.Errorf("expected incident namespace 'infrastructure', got '%s'", createdInc.Namespace)
	}
	if createdInc.PodName != "worker-srv-01" {
		t.Errorf("expected incident pod_name 'worker-srv-01', got '%s'", createdInc.PodName)
	}
	if createdInc.Severity != incident.SeverityCritical {
		t.Errorf("expected incident severity %s, got %s", incident.SeverityCritical, createdInc.Severity)
	}

	msgs := broadcaster.getMessages()
	foundIncidentBroadcast := false
	for _, msg := range msgs {
		if msg.msgType == "incident" {
			foundIncidentBroadcast = true
			break
		}
	}
	if !foundIncidentBroadcast {
		t.Errorf("expected broadcast message of type 'incident'")
	}

	// Consecutive scrape failure should NOT create duplicate incident
	collector.ScrapeAgent(context.Background(), hostRepo.hosts[0])

	incRepo.mu.Lock()
	createdCountAfter := len(incRepo.created)
	incRepo.mu.Unlock()

	if createdCountAfter != 1 {
		t.Errorf("expected still 1 incident (no duplicates), got %d", createdCountAfter)
	}
}

func TestCollector_ScrapeAgent_Recovery_ResolvesIncidentAndBroadcasts(t *testing.T) {
	agentResp := map[string]interface{}{
		"hostname":       "worker-srv-02",
		"os":             "linux",
		"arch":           "amd64",
		"uptime_seconds": 3600,
		"load_average":   []float64{0.1, 0.1, 0.1},
		"cpu": map[string]interface{}{
			"count":         4,
			"usage_percent": 10.0,
		},
		"memory": map[string]interface{}{
			"total_bytes":     int64(8589934592),
			"used_bytes":      int64(2147483648),
			"available_bytes": int64(6442450944),
			"usage_percent":   25.0,
		},
		"disks": []map[string]interface{}{
			{
				"mount_point":   "/",
				"total_bytes":   int64(53687091200),
				"used_bytes":    int64(10737418240),
				"usage_percent": 20.0,
				"filesystem":    "ext4",
			},
		},
		"network": map[string]interface{}{
			"total_rx_bytes_per_sec": 100,
			"total_tx_bytes_per_sec": 200,
		},
		"processes":    20,
		"collected_at": "2026-08-19T16:00:00Z",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(agentResp)
	}))
	defer srv.Close()

	hostRepo := &mockComputeHostRepo{
		hosts: []docker.ComputeHost{
			{
				ID:       "host-recovering",
				Name:     "worker-srv-02",
				HostType: "agent",
				Endpoint: srv.URL,
			},
		},
	}

	incRepo := newMockIncidentRepo()
	activeInc, _ := incident.New("fleet-primary", "infrastructure", "worker-srv-02", incident.TypeNodeNotReady, incident.SeverityCritical, "host down")
	activeInc.ID = "inc-active-02"
	_ = incRepo.Create(context.Background(), activeInc)

	broadcaster := &mockBroadcaster{}

	collector := NewCollector(
		nil,
		hostRepo,
		broadcaster,
		zap.NewNop(),
		WithHTTPClient(srv.Client()),
		WithIncidentRepo(incRepo),
	)

	// Scrape online agent — should resolve incident
	collector.ScrapeAgent(context.Background(), hostRepo.hosts[0])

	incRepo.mu.Lock()
	updatedCount := len(incRepo.updated)
	incRepo.mu.Unlock()

	if updatedCount != 1 {
		t.Fatalf("expected 1 incident update (resolved), got %d", updatedCount)
	}

	incRepo.mu.Lock()
	resolvedInc := incRepo.updated[0]
	incRepo.mu.Unlock()

	if resolvedInc.Status != incident.StatusResolved {
		t.Errorf("expected incident status %s, got %s", incident.StatusResolved, resolvedInc.Status)
	}
	if resolvedInc.ResolvedAt == nil {
		t.Errorf("expected non-nil ResolvedAt timestamp")
	}

	msgs := broadcaster.getMessages()
	foundResolvedBroadcast := false
	for _, msg := range msgs {
		if msg.msgType == "incident_resolved" {
			foundResolvedBroadcast = true
			break
		}
	}
	if !foundResolvedBroadcast {
		t.Errorf("expected broadcast message of type 'incident_resolved'")
	}
}

func TestCollector_TopProcesses_ScrapeAndTransfer(t *testing.T) {
	agentResp := map[string]interface{}{
		"hostname":       "worker-node-highcpu",
		"os":             "linux",
		"arch":           "amd64",
		"uptime_seconds": 7200,
		"load_average":   []float64{4.5, 3.2, 2.1},
		"cpu": map[string]interface{}{
			"count":         4,
			"usage_percent": 91.5,
		},
		"memory": map[string]interface{}{
			"total_bytes":     int64(16 * 1024 * 1024 * 1024),
			"used_bytes":      int64(12 * 1024 * 1024 * 1024),
			"available_bytes": int64(4 * 1024 * 1024 * 1024),
			"usage_percent":   75.0,
		},
		"disks": []map[string]interface{}{
			{
				"mount_point":   "/",
				"total_bytes":   int64(100 * 1024 * 1024 * 1024),
				"used_bytes":    int64(40 * 1024 * 1024 * 1024),
				"usage_percent": 40.0,
				"filesystem":    "ext4",
			},
		},
		"network": map[string]interface{}{
			"total_rx_bytes_per_sec": 5000000,
			"total_tx_bytes_per_sec": 2000000,
		},
		"processes": 150,
		"top_processes": []map[string]interface{}{
			{
				"pid":                 8912,
				"name":                "crypto-miner",
				"command_line":        "/usr/local/bin/worker-task --threads=8",
				"user":                "appuser",
				"cpu_percent":         85.2,
				"memory_bytes":        int64(4 * 1024 * 1024 * 1024),
				"memory_percent":      25.0,
				"read_bytes_per_sec":  int64(10485760),
				"write_bytes_per_sec": int64(5242880),
				"state":               "running",
			},
			{
				"pid":                 1204,
				"name":                "dockerd",
				"command_line":        "/usr/bin/dockerd",
				"user":                "root",
				"cpu_percent":         3.5,
				"memory_bytes":        int64(512 * 1024 * 1024),
				"memory_percent":      3.12,
				"read_bytes_per_sec":  int64(10240),
				"write_bytes_per_sec": int64(40960),
				"state":               "running",
			},
		},
		"collected_at": "2026-08-19T16:30:00Z",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(agentResp)
	}))
	defer srv.Close()

	hostRepo := &mockComputeHostRepo{
		hosts: []docker.ComputeHost{
			{
				ID:       "worker1",
				Name:     "worker-node-highcpu",
				HostType: "agent",
				Endpoint: srv.URL,
			},
		},
	}

	collector := NewCollector(nil, hostRepo, nil, zap.NewNop(), WithHTTPClient(srv.Client()))

	// Scrape the agent
	collector.ScrapeAgent(context.Background(), hostRepo.hosts[0])

	// Check agent metrics
	agentMetricsMap := collector.GetAgentMetrics()
	am, ok := agentMetricsMap["worker1"]
	if !ok {
		t.Fatalf("expected agent metrics for worker1")
	}
	if len(am.TopProcesses) != 2 {
		t.Fatalf("expected 2 top processes in agent metrics, got %d", len(am.TopProcesses))
	}
	if am.TopProcesses[0].PID != 8912 || am.TopProcesses[0].Name != "crypto-miner" || am.TopProcesses[0].CPUPercent != 85.2 {
		t.Errorf("unexpected top process in agent metrics: %+v", am.TopProcesses[0])
	}

	// Run CollectOnce to merge into NodeMetrics
	overview, err := collector.CollectOnce(context.Background())
	if err != nil {
		t.Fatalf("CollectOnce failed: %v", err)
	}

	if len(overview.Nodes) != 1 {
		t.Fatalf("expected 1 node in overview, got %d", len(overview.Nodes))
	}

	node := overview.Nodes[0]
	if node.NodeID != "worker1" {
		t.Errorf("expected node ID 'worker1', got '%s'", node.NodeID)
	}
	if len(node.TopProcesses) != 2 {
		t.Fatalf("expected 2 top processes transferred to node metrics, got %d", len(node.TopProcesses))
	}

	top1 := node.TopProcesses[0]
	if top1.PID != 8912 || top1.Name != "crypto-miner" || top1.CPUPercent != 85.2 || top1.MemoryPercent != 25.0 || top1.User != "appuser" {
		t.Errorf("unexpected top1 process details: %+v", top1)
	}

	// Verify JSON marshaling includes top_processes
	jsonData, err := json.Marshal(overview)
	if err != nil {
		t.Fatalf("failed to marshal overview: %v", err)
	}
	var unmarshaled SystemOverview
	if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal overview: %v", err)
	}
	if len(unmarshaled.Nodes[0].TopProcesses) != 2 {
		t.Fatalf("expected 2 top processes after JSON roundtrip, got %d", len(unmarshaled.Nodes[0].TopProcesses))
	}
}


