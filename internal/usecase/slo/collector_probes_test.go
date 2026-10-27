package slo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/observability"
	"github.com/datdt/k8sselfhost/internal/domain/provider/docker"
)

type mockSLORepo struct {
	recordedSamples []observability.HealthSample
}

func (m *mockSLORepo) RecordHealthSample(ctx context.Context, tenantID, service string, desired, running int, isHealthy bool, latencyMs *int) error {
	m.recordedSamples = append(m.recordedSamples, observability.HealthSample{
		TenantID:             tenantID,
		ServiceName:          service,
		DesiredReplicas:      desired,
		RunningReplicas:      running,
		IsHealthy:            isHealthy,
		HealthCheckLatencyMs: latencyMs,
	})
	return nil
}

func (m *mockSLORepo) GetHealthSamples(ctx context.Context, serviceName string, since time.Time) ([]observability.HealthSample, error) {
	return nil, nil
}
func (m *mockSLORepo) ListSLODefinitions(ctx context.Context) ([]observability.SLODefinition, error) {
	return nil, nil
}
func (m *mockSLORepo) GetSLODefinition(ctx context.Context, id string) (*observability.SLODefinition, error) {
	return nil, nil
}
func (m *mockSLORepo) CreateSLODefinition(ctx context.Context, d *observability.SLODefinition) error {
	return nil
}
func (m *mockSLORepo) UpdateSLODefinition(ctx context.Context, d *observability.SLODefinition) error {
	return nil
}
func (m *mockSLORepo) DeleteSLODefinition(ctx context.Context, id string) error {
	return nil
}
func (m *mockSLORepo) ListSLOSnapshots(ctx context.Context) ([]observability.SLOSnapshot, error) {
	return nil, nil
}
func (m *mockSLORepo) GetSLOSnapshotBySLOID(ctx context.Context, sloID string) (*observability.SLOSnapshot, error) {
	return nil, nil
}
func (m *mockSLORepo) CreateSLOSnapshot(ctx context.Context, s *observability.SLOSnapshot) error {
	return nil
}
func (m *mockSLORepo) UpdateSLOSnapshot(ctx context.Context, s *observability.SLOSnapshot) error {
	return nil
}
func (m *mockSLORepo) DeleteSLOSnapshotBySLOID(ctx context.Context, sloID string) error {
	return nil
}
func (m *mockSLORepo) ComputeSLI(ctx context.Context, service string, window time.Duration) (float64, error) {
	return 100.0, nil
}

func TestSLOCollector_BuiltinProbes_AllowLAN(t *testing.T) {
	// Built-in probes must continue probing admin-registered compute hosts on LAN
	mockRepo := &mockSLORepo{}
	collector := NewSLOCollector(nil, mockRepo, zap.NewNop(), WithCollectorTenantID("tenant-test"))

	// Create test server simulating LAN host
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer ts.Close()

	hosts := []docker.ComputeHost{
		{
			ID:       "host-1",
			Name:     "lan-node-1",
			Endpoint: ts.URL,
		},
	}

	collector.probeK8sAgentFleet(context.Background(), ts.Client(), hosts)
	assert.NotEmpty(t, mockRepo.recordedSamples)
	last := mockRepo.recordedSamples[len(mockRepo.recordedSamples)-1]
	assert.Equal(t, "k8s-agent-fleet", last.ServiceName)
	assert.True(t, last.IsHealthy)
}

func TestSLOCollector_CustomSLO_SSRF_Blocked(t *testing.T) {
	mockRepo := &mockSLORepo{}
	collector := NewSLOCollector(nil, mockRepo, zap.NewNop(), WithCollectorTenantID("tenant-test"))

	// Even if a local server is running, probeCustomSLO MUST block dialing loopback/private/metadata
	blockedQueries := []string{
		"http://127.0.0.1:8080/health",
		"http://10.10.10.60:9100/health",
		"http://169.254.169.254/latest/meta-data",
		"tcp://127.0.0.1:6443",
		"tcp://10.0.0.1:5432",
		"tcp://169.254.169.254:80",
	}

	for _, q := range blockedQueries {
		def := observability.SLODefinition{
			ID:      "custom-1",
			Service: "custom-probe",
			Query:   q,
		}
		collector.probeCustomSLO(context.Background(), http.DefaultClient, def)
		assert.NotEmpty(t, mockRepo.recordedSamples)
		last := mockRepo.recordedSamples[len(mockRepo.recordedSamples)-1]
		assert.Equal(t, "custom-probe", last.ServiceName)
		assert.False(t, last.IsHealthy, "query %s should be recorded as unhealthy/blocked", q)
		assert.Equal(t, 0, last.RunningReplicas, "query %s running count should be 0", q)
	}
}
