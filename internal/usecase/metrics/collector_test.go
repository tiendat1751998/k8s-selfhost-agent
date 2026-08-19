package metrics

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/api/types/system"
	"go.uber.org/zap"
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
	collector := NewCollector(nil, nil, zap.NewNop(), WithThresholds(Thresholds{
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

	collector := NewCollector(mockDocker, nil, zap.NewNop())
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
}

func TestCollector_CollectOnce_NilClient(t *testing.T) {
	collector := NewCollector(nil, nil, zap.NewNop())
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
