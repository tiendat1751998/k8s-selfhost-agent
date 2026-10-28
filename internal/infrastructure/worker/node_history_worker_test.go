package worker_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/domain/nodemetrics"
	"github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/infrastructure/worker"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
	usecaseMetrics "github.com/datdt/k8sselfhost/internal/usecase/metrics"
)

type mockSnapshotProvider struct {
	snapshot *usecaseMetrics.SystemOverview
}

func (m *mockSnapshotProvider) GetLastSnapshot() *usecaseMetrics.SystemOverview {
	return m.snapshot
}

type mockNodeMetricsRepo struct {
	insertedRollups []nodemetrics.NodeMetricRollup
	downsampled     bool
	lastCtx         context.Context
}

func (m *mockNodeMetricsRepo) InsertBatch(ctx context.Context, rollups []nodemetrics.NodeMetricRollup) error {
	m.lastCtx = ctx
	m.insertedRollups = append(m.insertedRollups, rollups...)
	return nil
}

func (m *mockNodeMetricsRepo) QueryHistory(ctx context.Context, q nodemetrics.NodeHistoryQuery) ([]nodemetrics.NodeMetricRollup, error) {
	return nil, nil
}

func (m *mockNodeMetricsRepo) GetSummary(ctx context.Context, q nodemetrics.NodeHistoryQuery) (*nodemetrics.NodeHistoricalSummary, error) {
	return nil, nil
}

func (m *mockNodeMetricsRepo) DownsampleAndPrune(ctx context.Context, olderThan7Days, olderThan90Days time.Time) error {
	m.lastCtx = ctx
	m.downsampled = true
	return nil
}

type mockIncidentRepo struct {
	createdIncidents []*incident.Incident
	lastCtx          context.Context
}

func (m *mockIncidentRepo) Create(ctx context.Context, inc *incident.Incident) error {
	m.lastCtx = ctx
	m.createdIncidents = append(m.createdIncidents, inc)
	return nil
}

func (m *mockIncidentRepo) GetByID(ctx context.Context, id string) (*incident.Incident, error) {
	return nil, nil
}

func (m *mockIncidentRepo) List(ctx context.Context, filter incident.Filter) ([]*incident.Incident, int64, error) {
	return nil, 0, nil
}

func (m *mockIncidentRepo) Update(ctx context.Context, inc *incident.Incident) error {
	return nil
}

func (m *mockIncidentRepo) GetByPodAndType(ctx context.Context, namespace, podName string, incidentType incident.Type) (*incident.Incident, error) {
	return nil, nil
}

type mockComputeHostLister struct {
	hosts []docker.ComputeHost
}

func (m *mockComputeHostLister) ListAll(ctx context.Context) ([]docker.ComputeHost, error) {
	return m.hosts, nil
}

func TestNodeHistoryWorker_ActiveComputeHost_GeneratesIncidentWithPlatformAdmin(t *testing.T) {
	sp := &mockSnapshotProvider{}
	nmRepo := &mockNodeMetricsRepo{}
	incRepo := &mockIncidentRepo{}
	hostLister := &mockComputeHostLister{
		hosts: []docker.ComputeHost{
			{ID: "node-1", Name: "k8smasterdeb", Status: "connected"},
		},
	}

	w := worker.NewNodeHistoryWorker(sp, nmRepo, incRepo, zap.NewNop(),
		worker.WithComputeHostLister(hostLister),
	)

	// Cycle 1: Host is online
	sp.snapshot = &usecaseMetrics.SystemOverview{
		Nodes: []usecaseMetrics.NodeMetrics{
			{NodeID: "node-1", NodeName: "k8smasterdeb", Status: "ready", CPUPercent: 40.0, MemoryPercent: 50.0},
		},
	}
	err := w.RecordRollupOnce(context.Background())
	require.NoError(t, err)
	assert.Empty(t, incRepo.createdIncidents)

	// Cycle 2: Host transitions to offline
	sp.snapshot = &usecaseMetrics.SystemOverview{
		Nodes: []usecaseMetrics.NodeMetrics{
			{NodeID: "node-1", NodeName: "k8smasterdeb", Status: "offline", CPUPercent: 0.0, MemoryPercent: 0.0},
		},
	}
	err = w.RecordRollupOnce(context.Background())
	require.NoError(t, err)

	require.Len(t, incRepo.createdIncidents, 1)
	assert.Equal(t, "k8smasterdeb", incRepo.createdIncidents[0].PodName)
	assert.Equal(t, incident.SeverityCritical, incRepo.createdIncidents[0].Severity)

	// Verify tenancy context in incident creation
	assert.Equal(t, "platform_admin", tenancy.UserRoleFromContext(incRepo.lastCtx))
	assert.Equal(t, tenancy.DefaultTenantID, tenancy.TenantIDFromContext(incRepo.lastCtx))
}

func TestNodeHistoryWorker_PhantomLegacyHost_BlockedFromIncidentGeneration(t *testing.T) {
	sp := &mockSnapshotProvider{}
	nmRepo := &mockNodeMetricsRepo{}
	incRepo := &mockIncidentRepo{}
	hostLister := &mockComputeHostLister{
		hosts: []docker.ComputeHost{
			{ID: "node-1", Name: "k8smasterdeb", Status: "connected"},
		},
	}

	w := worker.NewNodeHistoryWorker(sp, nmRepo, incRepo, zap.NewNop(),
		worker.WithComputeHostLister(hostLister),
	)

	phantomHosts := []string{"masterdb", "k8sworker3", "worker2", "worker1", "workerdb1", "test-qa-incident"}
	for _, phantom := range phantomHosts {
		// Cycle 1: Online
		sp.snapshot = &usecaseMetrics.SystemOverview{
			Nodes: []usecaseMetrics.NodeMetrics{
				{NodeID: phantom, NodeName: phantom, Status: "ready", CPUPercent: 20.0},
			},
		}
		_ = w.RecordRollupOnce(context.Background())

		// Cycle 2: Goes offline
		sp.snapshot = &usecaseMetrics.SystemOverview{
			Nodes: []usecaseMetrics.NodeMetrics{
				{NodeID: phantom, NodeName: phantom, Status: "offline", CPUPercent: 0.0},
			},
		}
		_ = w.RecordRollupOnce(context.Background())
	}

	// ZERO incidents generated for phantom legacy hosts
	assert.Empty(t, incRepo.createdIncidents, "phantom hosts must never trigger anomaly incidents")
}

func TestNodeHistoryWorker_UnmanagedHostNotInComputeHosts_Blocked(t *testing.T) {
	sp := &mockSnapshotProvider{}
	nmRepo := &mockNodeMetricsRepo{}
	incRepo := &mockIncidentRepo{}
	hostLister := &mockComputeHostLister{
		hosts: []docker.ComputeHost{
			{ID: "node-real", Name: "real-server", Status: "connected"},
		},
	}

	w := worker.NewNodeHistoryWorker(sp, nmRepo, incRepo, zap.NewNop(),
		worker.WithComputeHostLister(hostLister),
	)

	// Cycle 1: Unknown host online
	sp.snapshot = &usecaseMetrics.SystemOverview{
		Nodes: []usecaseMetrics.NodeMetrics{
			{NodeID: "unknown-99", NodeName: "rogue-box", Status: "ready", CPUPercent: 30.0},
		},
	}
	_ = w.RecordRollupOnce(context.Background())

	// Cycle 2: Unknown host degraded
	sp.snapshot = &usecaseMetrics.SystemOverview{
		Nodes: []usecaseMetrics.NodeMetrics{
			{NodeID: "unknown-99", NodeName: "rogue-box", Status: "degraded", CPUPercent: 95.0},
		},
	}
	_ = w.RecordRollupOnce(context.Background())

	assert.Empty(t, incRepo.createdIncidents, "unregistered compute host must not trigger anomaly incident")
}
