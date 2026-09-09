package metrics

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
)

func generateDummyOverview(numNodes, numContainers int) *SystemOverview {
	overview := &SystemOverview{
		TotalNodes:                numNodes,
		HealthyNodes:              numNodes - 1,
		TotalContainers:           numContainers,
		RunningContainers:         numContainers - 5,
		TotalCPUPercent:           52.4,
		TotalMemPercent:           64.8,
		TotalDiskPercent:          41.2,
		TotalDiskReadBytesPerSec:  int64(numNodes * 1024 * 1024 * 10),
		TotalDiskWriteBytesPerSec: int64(numNodes * 1024 * 1024 * 5),
		RequestsPerSec:            1450.5,
		CollectedAt:               time.Now().UTC(),
		Nodes:                     make([]NodeMetrics, 0, numNodes),
		Containers:                make([]ContainerMetrics, 0, numContainers),
	}

	for i := 0; i < numNodes; i++ {
		node := NodeMetrics{
			NodeID:               fmt.Sprintf("node-%d", i),
			NodeName:             fmt.Sprintf("worker-%d", i),
			Role:                 "worker",
			Status:               "ready",
			CPUPercent:           45.5 + float64(i%40),
			MemoryUsed:           8 * 1024 * 1024 * 1024,
			MemoryTotal:          16 * 1024 * 1024 * 1024,
			MemoryPercent:        50.0,
			DiskUsed:             200 * 1024 * 1024 * 1024,
			DiskTotal:            500 * 1024 * 1024 * 1024,
			DiskPercent:          40.0,
			DiskReadBytesPerSec:  1024 * 1024 * 10,
			DiskWriteBytesPerSec: 1024 * 1024 * 5,
			ContainerCount:       numContainers / numNodes,
			RunningCount:         numContainers / numNodes,
			TopProcesses: []ProcessMetric{
				{PID: 1001, Name: "dockerd", CPUPercent: 5.2, MemoryBytes: 512 * 1024 * 1024},
				{PID: 1002, Name: "kubelet", CPUPercent: 3.1, MemoryBytes: 256 * 1024 * 1024},
				{PID: 1003, Name: "postgres", CPUPercent: 12.4, MemoryBytes: 1024 * 1024 * 1024},
			},
			UpdatedAt: time.Now().UTC(),
		}
		overview.Nodes = append(overview.Nodes, node)
	}

	for i := 0; i < numContainers; i++ {
		cm := ContainerMetrics{
			ContainerID:   fmt.Sprintf("container-%d-hash", i),
			ContainerName: fmt.Sprintf("/k8s_app-service-%d.1.taskid123", i%20),
			NodeID:        fmt.Sprintf("node-%d", i%numNodes),
			Image:         "registry.internal/app:v1.2.3",
			State:         "running",
			CPUPercent:    2.5,
			MemoryUsed:    128 * 1024 * 1024,
			MemoryLimit:   512 * 1024 * 1024,
			MemoryPercent: 25.0,
			ServiceName:   fmt.Sprintf("app-service-%d", i%20),
		}
		overview.Containers = append(overview.Containers, cm)
	}

	return overview
}

func BenchmarkGenerateAlerts_50Nodes_200Containers(b *testing.B) {
	c := &Collector{
		thresholds: Thresholds{
			CPUWarning:    80.0,
			MemoryWarning: 85.0,
			DiskWarning:   90.0,
		},
	}
	overview := generateDummyOverview(50, 200)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		alerts := c.generateAlerts(overview)
		_ = alerts
	}
}

func BenchmarkCalculateCPUPercent(b *testing.B) {
	stats := &container.StatsResponse{
		CPUStats: container.CPUStats{
			CPUUsage: container.CPUUsage{
				TotalUsage:        2000000000,
				PercpuUsage:       []uint64{1000000000, 1000000000},
				UsageInKernelmode: 500000000,
				UsageInUsermode:   1500000000,
			},
			SystemUsage: 10000000000,
			OnlineCPUs:  2,
		},
		PreCPUStats: container.CPUStats{
			CPUUsage: container.CPUUsage{
				TotalUsage:        1800000000,
				PercpuUsage:       []uint64{900000000, 900000000},
				UsageInKernelmode: 450000000,
				UsageInUsermode:   1350000000,
			},
			SystemUsage: 9000000000,
			OnlineCPUs:  2,
		},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cpu := calculateCPUPercent(stats)
		_ = cpu
	}
}

func BenchmarkExtractServiceName(b *testing.B) {
	cm := ContainerMetrics{
		ContainerName: "/prod-stack_web-service.1.0123456789abcdef",
		Labels: map[string]string{
			"com.docker.swarm.service.name": "prod-stack_web-service",
		},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		name := extractServiceName(cm)
		_ = name
	}
}

func BenchmarkNormalizeNodeName(b *testing.B) {
	name := "swarm-worker-node-prod-east-1.internal.lan"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res := normalizeNodeName(name)
		_ = res
	}
}

func BenchmarkSafeDeltaInt64(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d := safeDeltaInt64(int64(i*1000), int64((i-1)*1000))
		_ = d
	}
}

func BenchmarkSystemOverview_JSONMarshal(b *testing.B) {
	overview := generateDummyOverview(50, 200)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, err := json.Marshal(overview)
		if err != nil {
			b.Fatal(err)
		}
		_ = data
	}
}

func BenchmarkSystemOverview_JSONUnmarshal(b *testing.B) {
	overview := generateDummyOverview(50, 200)
	data, err := json.Marshal(overview)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var target SystemOverview
		if err := json.Unmarshal(data, &target); err != nil {
			b.Fatal(err)
		}
	}
}
