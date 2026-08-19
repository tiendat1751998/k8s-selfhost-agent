package metrics

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/api/types/system"
	"go.uber.org/zap"

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
	nodes      []swarm.Node
	info       system.Info
	diskUsage  types.DiskUsage
	nodeErr    error
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

func (m *mockDockerAPI) NodeList(ctx context.Context, options swarm.NodeListOptions) ([]swarm.Node, error) {
	if m.nodeErr != nil {
		return nil, m.nodeErr
	}
	return m.nodes, nil
}

func (m *mockDockerAPI) Info(ctx context.Context) (system.Info, error) {
	return m.info, nil
}

func (m *mockDockerAPI) DiskUsage(ctx context.Context, options types.DiskUsageOptions) (types.DiskUsage, error) {
	return m.diskUsage, nil
}

type mockComputeHostRepo struct {
	hosts         []docker.ComputeHost
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

func TestCollector_CollectOnce_Swarm(t *testing.T) {
	mockDocker := &mockDockerAPI{
		info: system.Info{
			ID:       "engine-1",
			Name:     "swarm-manager",
			MemTotal: 16 * 1024 * 1024 * 1024,
			NCPU:     8,
		},
		diskUsage: types.DiskUsage{
			LayersSize: 5 * 1024 * 1024 * 1024,
			Images: []*image.Summary{
				{Size: 3 * 1024 * 1024 * 1024},
			},
		},
		nodes: []swarm.Node{
			{
				ID: "node-1",
				Description: swarm.NodeDescription{
					Hostname: "swarm-node-1",
					Resources: swarm.Resources{
						MemoryBytes: 8 * 1024 * 1024 * 1024,
					},
				},
				Spec: swarm.NodeSpec{
					Role: swarm.NodeRoleManager,
				},
				Status: swarm.NodeStatus{
					State: swarm.NodeStateReady,
				},
				Meta: swarm.Meta{
					UpdatedAt: time.Now().Add(-1 * time.Hour),
				},
			},
			{
				ID: "node-2",
				Description: swarm.NodeDescription{
					Hostname: "swarm-node-2",
					Resources: swarm.Resources{
						MemoryBytes: 8 * 1024 * 1024 * 1024,
					},
				},
				Spec: swarm.NodeSpec{
					Role: swarm.NodeRoleWorker,
				},
				Status: swarm.NodeStatus{
					State: swarm.NodeStateDown,
				},
				Meta: swarm.Meta{
					UpdatedAt: time.Now().Add(-1 * time.Hour),
				},
			},
		},
		containers: []container.Summary{
			{
				ID:    "c1",
				Names: []string{"/web-app"},
				Image: "nginx:latest",
				State: "running",
				Labels: map[string]string{
					"com.docker.swarm.node.id": "node-1",
				},
			},
			{
				ID:    "c2",
				Names: []string{"/db-app"},
				Image: "postgres:16",
				State: "exited",
				Labels: map[string]string{
					"com.docker.swarm.node.id": "node-2",
				},
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
	if overview.Nodes[0].Source != "docker" {
		t.Errorf("expected node source 'docker', got '%s'", overview.Nodes[0].Source)
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
		nodeErr: context.Canceled, // Swarm not available
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
	overview, err := collector.CollectOnce(context.Background())
	if err != nil {
		t.Fatalf("CollectOnce failed: %v", err)
	}

	if overview.TotalNodes != 1 {
		t.Errorf("expected TotalNodes = 1, got %d", overview.TotalNodes)
	}
	if overview.Nodes[0].NodeName != "my-docker-host" {
		t.Errorf("expected NodeName = my-docker-host, got %s", overview.Nodes[0].NodeName)
	}
	if overview.Nodes[0].Role != "standalone" {
		t.Errorf("expected Role = standalone, got %s", overview.Nodes[0].Role)
	}
	if overview.Nodes[0].Source != "docker" {
		t.Errorf("expected Source = docker, got %s", overview.Nodes[0].Source)
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

	hostRepo.mu.Lock()
	statusEntry, recorded := hostRepo.updatedStatus["host-db-01"]
	hostRepo.mu.Unlock()

	if !recorded || statusEntry.status != "connected" {
		t.Errorf("expected repo status 'connected', got recorded=%v status=%s", recorded, statusEntry.status)
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

func TestCollector_MergeDockerAndAgentNodes(t *testing.T) {
	mockDocker := &mockDockerAPI{
		info: system.Info{
			ID:                "docker-host-1",
			Name:              "docker-node",
			MemTotal:          8 * 1024 * 1024 * 1024,
			Containers:        1,
			ContainersRunning: 1,
		},
		nodeErr: context.Canceled, // standalone docker
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

	if overview.TotalNodes != 2 {
		t.Fatalf("expected TotalNodes = 2, got %d", overview.TotalNodes)
	}
	if overview.HealthyNodes != 2 {
		t.Errorf("expected HealthyNodes = 2, got %d", overview.HealthyNodes)
	}

	var foundDocker, foundAgent bool
	for _, n := range overview.Nodes {
		if n.Source == "docker" {
			foundDocker = true
			if n.NodeName != "docker-node" {
				t.Errorf("expected docker node name 'docker-node', got '%s'", n.NodeName)
			}
		}
		if n.Source == "agent" {
			foundAgent = true
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
	}

	if !foundDocker {
		t.Errorf("docker node missing from snapshot")
	}
	if !foundAgent {
		t.Errorf("agent node missing from snapshot")
	}
}
