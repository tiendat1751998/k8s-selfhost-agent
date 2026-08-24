package metrics_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/domain/nodemetrics"
	"github.com/datdt/k8sselfhost/internal/usecase/metrics"
)

// MockNodeMetricsRepo for testing
type mockNodeMetricsRepo struct {
	mu      sync.Mutex
	rollups []nodemetrics.NodeMetricRollup
	pruned  bool
}

func (m *mockNodeMetricsRepo) InsertBatch(ctx context.Context, rollups []nodemetrics.NodeMetricRollup) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rollups = append(m.rollups, rollups...)
	return nil
}

func (m *mockNodeMetricsRepo) QueryHistory(ctx context.Context, q nodemetrics.NodeHistoryQuery) ([]nodemetrics.NodeMetricRollup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rollups, nil
}

func (m *mockNodeMetricsRepo) GetSummary(ctx context.Context, q nodemetrics.NodeHistoryQuery) (*nodemetrics.NodeHistoricalSummary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return &nodemetrics.NodeHistoricalSummary{
		NodeID:        q.NodeID,
		TotalSamples: len(m.rollups),
	}, nil
}

func (m *mockNodeMetricsRepo) DownsampleAndPrune(ctx context.Context, olderThan7Days, olderThan90Days time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruned = true
	return nil
}

// MockIncidentRepo for testing
type mockIncidentRepo struct {
	mu        sync.Mutex
	incidents []*incident.Incident
}

func (m *mockIncidentRepo) Create(ctx context.Context, inc *incident.Incident) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.incidents = append(m.incidents, inc)
	return nil
}

func (m *mockIncidentRepo) GetByID(ctx context.Context, id string) (*incident.Incident, error) {
	return nil, nil
}

func (m *mockIncidentRepo) Update(ctx context.Context, inc *incident.Incident) error {
	return nil
}

func (m *mockIncidentRepo) List(ctx context.Context, filter incident.Filter) ([]*incident.Incident, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.incidents, int64(len(m.incidents)), nil
}

func (m *mockIncidentRepo) GetByPodAndType(ctx context.Context, namespace, podName string, incidentType incident.Type) (*incident.Incident, error) {
	return nil, nil
}

// MockSnapshotProvider for testing
type mockSnapshotProvider struct {
	snapshot *metrics.SystemOverview
}

func (m *mockSnapshotProvider) GetLastSnapshot() *metrics.SystemOverview {
	return m.snapshot
}

func TestNodeHistoryWorker_RecordRollupAndIncident(t *testing.T) {
	sp := &mockSnapshotProvider{
		snapshot: &metrics.SystemOverview{
			Nodes: []metrics.NodeMetrics{
				{
					NodeID:         "node-master",
					NodeName:       "k8smater",
					CPUPercent:     45.5,
					MemoryUsed:     4 * 1024 * 1024 * 1024,
					MemoryTotal:    16 * 1024 * 1024 * 1024,
					MemoryPercent:  25.0,
					DiskUsed:       50 * 1024 * 1024 * 1024,
					DiskTotal:      200 * 1024 * 1024 * 1024,
					DiskPercent:    25.0,
					NetworkRxBytes: 102400,
					NetworkTxBytes: 51200,
					Processes:      120,
					ContainerCount: 5,
					Status:         "ready",
				},
			},
		},
	}

	nmRepo := &mockNodeMetricsRepo{}
	incRepo := &mockIncidentRepo{}

	worker := metrics.NewNodeHistoryWorker(
		sp,
		nmRepo,
		incRepo,
		zap.NewNop(),
		metrics.WithRollupInterval(100*time.Millisecond),
	)

	ctx := context.Background()

	// 1. Initial collection cycle (Node Online)
	err := worker.RecordRollupOnce(ctx)
	if err != nil {
		t.Fatalf("RecordRollupOnce failed: %v", err)
	}

	if len(nmRepo.rollups) != 1 {
		t.Fatalf("expected 1 rollup, got %d", len(nmRepo.rollups))
	}
	r := nmRepo.rollups[0]
	if r.NodeName != "k8smater" || r.CPUPercent != 45.5 {
		t.Errorf("unexpected rollup data: %+v", r)
	}
	if len(incRepo.incidents) != 0 {
		t.Errorf("expected 0 incidents while online, got %d", len(incRepo.incidents))
	}

	// 2. Second cycle: Node goes Down/Offline -> Sentinel must capture pre-crash incident
	sp.snapshot.Nodes[0].Status = "down"
	sp.snapshot.Nodes[0].CPUPercent = 95.0
	sp.snapshot.Nodes[0].TopProcesses = []metrics.ProcessMetric{
		{PID: 1024, Name: "postgres", CPUPercent: 80.5, MemoryBytes: 4 * 1024 * 1024 * 1024},
	}

	err = worker.RecordRollupOnce(ctx)
	if err != nil {
		t.Fatalf("second RecordRollupOnce failed: %v", err)
	}

	if len(incRepo.incidents) != 1 {
		t.Fatalf("expected 1 incident triggered when node went offline, got %d", len(incRepo.incidents))
	}
	inc := incRepo.incidents[0]
	if inc.Type != incident.TypeNodeNotReady {
		t.Errorf("expected TypeNodeNotReady, got %s", inc.Type)
	}
	if inc.RawData["node_name"] != "k8smater" {
		t.Errorf("expected node_name in incident raw data, got %v", inc.RawData)
	}
}
