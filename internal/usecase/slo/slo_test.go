package slo

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/swarm"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/observability"
)

type mockDockerClient struct {
	services   []swarm.Service
	tasks      []swarm.Task
	containers []container.Summary
}

func (m *mockDockerClient) ServiceList(ctx context.Context, options swarm.ServiceListOptions) ([]swarm.Service, error) {
	return m.services, nil
}

func (m *mockDockerClient) TaskList(ctx context.Context, options swarm.TaskListOptions) ([]swarm.Task, error) {
	return m.tasks, nil
}

func (m *mockDockerClient) ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error) {
	return m.containers, nil
}

type mockObsRepo struct {
	samples []observability.HealthSample
	defs    []observability.SLODefinition
	snaps   map[string]*observability.SLOSnapshot
}

func newMockObsRepo() *mockObsRepo {
	return &mockObsRepo{
		samples: make([]observability.HealthSample, 0),
		defs:    make([]observability.SLODefinition, 0),
		snaps:   make(map[string]*observability.SLOSnapshot),
	}
}

func (r *mockObsRepo) ListSLODefinitions(ctx context.Context) ([]observability.SLODefinition, error) {
	return r.defs, nil
}

func (r *mockObsRepo) GetSLODefinition(ctx context.Context, id string) (*observability.SLODefinition, error) {
	for _, d := range r.defs {
		if d.ID == id {
			return &d, nil
		}
	}
	return nil, nil
}

func (r *mockObsRepo) CreateSLODefinition(ctx context.Context, d *observability.SLODefinition) error {
	r.defs = append(r.defs, *d)
	return nil
}

func (r *mockObsRepo) UpdateSLODefinition(ctx context.Context, d *observability.SLODefinition) error {
	for i, def := range r.defs {
		if def.ID == d.ID {
			r.defs[i] = *d
			return nil
		}
	}
	return nil
}

func (r *mockObsRepo) DeleteSLODefinition(ctx context.Context, id string) error {
	var newDefs []observability.SLODefinition
	for _, def := range r.defs {
		if def.ID != id {
			newDefs = append(newDefs, def)
		}
	}
	r.defs = newDefs
	return nil
}

func (r *mockObsRepo) ListSLOSnapshots(ctx context.Context) ([]observability.SLOSnapshot, error) {
	var list []observability.SLOSnapshot
	for _, s := range r.snaps {
		list = append(list, *s)
	}
	return list, nil
}

func (r *mockObsRepo) GetSLOSnapshotBySLOID(ctx context.Context, sloID string) (*observability.SLOSnapshot, error) {
	if s, ok := r.snaps[sloID]; ok {
		return s, nil
	}
	return nil, nil
}

func (r *mockObsRepo) CreateSLOSnapshot(ctx context.Context, s *observability.SLOSnapshot) error {
	r.snaps[s.SLOID] = s
	return nil
}

func (r *mockObsRepo) UpdateSLOSnapshot(ctx context.Context, s *observability.SLOSnapshot) error {
	r.snaps[s.SLOID] = s
	return nil
}

func (r *mockObsRepo) DeleteSLOSnapshotBySLOID(ctx context.Context, sloID string) error {
	delete(r.snaps, sloID)
	return nil
}

func (r *mockObsRepo) RecordHealthSample(ctx context.Context, tenantID string, serviceName string, desired int, running int, isHealthy bool, latencyMs *int) error {
	r.samples = append(r.samples, observability.HealthSample{
		ID:                   "sample-1",
		TenantID:             tenantID,
		ServiceName:          serviceName,
		DesiredReplicas:      desired,
		RunningReplicas:      running,
		IsHealthy:            isHealthy,
		HealthCheckLatencyMs: latencyMs,
		RecordedAt:           time.Now().UTC(),
	})
	return nil
}

func (r *mockObsRepo) GetHealthSamples(ctx context.Context, serviceName string, since time.Time) ([]observability.HealthSample, error) {
	var res []observability.HealthSample
	for _, s := range r.samples {
		if s.ServiceName == serviceName {
			res = append(res, s)
		}
	}
	return res, nil
}

func (r *mockObsRepo) ComputeSLI(ctx context.Context, serviceName string, window time.Duration) (float64, error) {
	total := 0
	healthy := 0
	for _, s := range r.samples {
		if s.ServiceName == serviceName {
			total++
			if s.IsHealthy {
				healthy++
			}
		}
	}
	if total == 0 {
		return 100.0, nil
	}
	return (float64(healthy) / float64(total)) * 100.0, nil
}

func TestSLOCollector_CollectOnce(t *testing.T) {
	replicas := uint64(2)
	dockerMock := &mockDockerClient{
		services: []swarm.Service{
			{
				ID: "svc-1",
				Spec: swarm.ServiceSpec{
					Annotations: swarm.Annotations{Name: "tiki_gateway"},
					Mode: swarm.ServiceMode{
						Replicated: &swarm.ReplicatedService{Replicas: &replicas},
					},
				},
			},
		},
		tasks: []swarm.Task{
			{
				ServiceID: "svc-1",
				Status:    swarm.TaskStatus{State: swarm.TaskStateRunning},
			},
			{
				ServiceID: "svc-1",
				Status:    swarm.TaskStatus{State: swarm.TaskStateRunning},
			},
		},
		containers: []container.Summary{
			{
				ID:     "cnt-1",
				Names:  []string{"/standalone-redis"},
				State:  "running",
				Status: "Up 2 hours (healthy)",
			},
		},
	}

	repoMock := newMockObsRepo()
	collector := NewSLOCollector(dockerMock, repoMock, zap.NewNop(), WithCollectorInterval(10*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	collector.CollectOnce(ctx)

	if len(repoMock.samples) != 2 {
		t.Fatalf("expected 2 health samples recorded, got %d", len(repoMock.samples))
	}

	// Verify swarm service sample
	gatewaySample := repoMock.samples[0]
	if gatewaySample.ServiceName != "tiki_gateway" || !gatewaySample.IsHealthy || gatewaySample.RunningReplicas != 2 {
		t.Errorf("unexpected gateway sample: %+v", gatewaySample)
	}

	// Verify standalone container sample
	redisSample := repoMock.samples[1]
	if redisSample.ServiceName != "standalone-redis" || !redisSample.IsHealthy || redisSample.RunningReplicas != 1 {
		t.Errorf("unexpected redis sample: %+v", redisSample)
	}
}

func TestSLOSnapshotUpdater_UpdateOnce(t *testing.T) {
	repoMock := newMockObsRepo()
	repoMock.defs = []observability.SLODefinition{
		{
			ID:             "def-1",
			Service:        "tiki_gateway",
			Target:         99.9,
			IndicatorType:  "availability",
			Window:         "30d",
			AlertThreshold: 1.5,
		},
	}

	// Record 99 healthy samples and 1 unhealthy sample (99.0% actual SLI)
	for i := 0; i < 99; i++ {
		_ = repoMock.RecordHealthSample(context.Background(), "t1", "tiki_gateway", 1, 1, true, nil)
	}
	_ = repoMock.RecordHealthSample(context.Background(), "t1", "tiki_gateway", 1, 0, false, nil)

	updater := NewSLOSnapshotUpdater(repoMock, zap.NewNop(), WithUpdaterInterval(10*time.Millisecond))
	updater.UpdateOnce(context.Background())

	snap := repoMock.snaps["def-1"]
	if snap == nil {
		t.Fatalf("expected snapshot created for def-1")
	}

	if snap.Actual != 99.0 {
		t.Errorf("expected actual SLI 99.0, got %f", snap.Actual)
	}
	// Target = 99.9, allowed unhealthy = 0.1, actual unhealthy = 1.0 -> burn rate = 10.0 -> critical
	if snap.BurnRate < 9.0 {
		t.Errorf("expected fast burn rate >= 9.0, got %f", snap.BurnRate)
	}
	if snap.BudgetStatus != "critical" {
		t.Errorf("expected critical status, got %s", snap.BudgetStatus)
	}
}
