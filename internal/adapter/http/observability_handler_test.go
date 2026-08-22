package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/observability"
)


type mockObservabilityRepo struct {
	defs    map[string]observability.SLODefinition
	snaps   map[string]observability.SLOSnapshot
	samples []observability.HealthSample
}

func newMockObservabilityRepo() *mockObservabilityRepo {
	return &mockObservabilityRepo{
		defs:    make(map[string]observability.SLODefinition),
		snaps:   make(map[string]observability.SLOSnapshot),
		samples: make([]observability.HealthSample, 0),
	}
}

func (m *mockObservabilityRepo) ListSLODefinitions(ctx context.Context) ([]observability.SLODefinition, error) {
	var list []observability.SLODefinition
	for _, d := range m.defs {
		list = append(list, d)
	}
	return list, nil
}

func (m *mockObservabilityRepo) GetSLODefinition(ctx context.Context, id string) (*observability.SLODefinition, error) {
	if d, ok := m.defs[id]; ok {
		return &d, nil
	}
	return nil, nil
}

func (m *mockObservabilityRepo) CreateSLODefinition(ctx context.Context, d *observability.SLODefinition) error {
	m.defs[d.ID] = *d
	return nil
}

func (m *mockObservabilityRepo) UpdateSLODefinition(ctx context.Context, d *observability.SLODefinition) error {
	m.defs[d.ID] = *d
	return nil
}

func (m *mockObservabilityRepo) DeleteSLODefinition(ctx context.Context, id string) error {
	delete(m.defs, id)
	delete(m.snaps, id)
	return nil
}

func (m *mockObservabilityRepo) ListSLOSnapshots(ctx context.Context) ([]observability.SLOSnapshot, error) {
	var list []observability.SLOSnapshot
	for _, s := range m.snaps {
		list = append(list, s)
	}
	return list, nil
}

func (m *mockObservabilityRepo) GetSLOSnapshotBySLOID(ctx context.Context, sloID string) (*observability.SLOSnapshot, error) {
	if s, ok := m.snaps[sloID]; ok {
		return &s, nil
	}
	return nil, nil
}

func (m *mockObservabilityRepo) CreateSLOSnapshot(ctx context.Context, s *observability.SLOSnapshot) error {
	m.snaps[s.SLOID] = *s
	return nil
}

func (m *mockObservabilityRepo) UpdateSLOSnapshot(ctx context.Context, s *observability.SLOSnapshot) error {
	m.snaps[s.SLOID] = *s
	return nil
}

func (m *mockObservabilityRepo) DeleteSLOSnapshotBySLOID(ctx context.Context, sloID string) error {
	delete(m.snaps, sloID)
	return nil
}

func (m *mockObservabilityRepo) RecordHealthSample(ctx context.Context, tenantID string, serviceName string, desired int, running int, isHealthy bool, latencyMs *int) error {
	m.samples = append(m.samples, observability.HealthSample{
		ID:                   "sample-1",
		TenantID:             tenantID,
		ServiceName:          serviceName,
		DesiredReplicas:      desired,
		RunningReplicas:      running,
		IsHealthy:            isHealthy,
		HealthCheckLatencyMs: latencyMs,
	})
	return nil
}

func (m *mockObservabilityRepo) GetHealthSamples(ctx context.Context, serviceName string, since time.Time) ([]observability.HealthSample, error) {
	var res []observability.HealthSample
	for _, s := range m.samples {
		if s.ServiceName == serviceName {
			res = append(res, s)
		}
	}
	return res, nil
}

func (m *mockObservabilityRepo) ComputeSLI(ctx context.Context, serviceName string, window time.Duration) (float64, error) {
	total := 0
	healthy := 0
	for _, s := range m.samples {
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


func TestObservabilityHandler_CRUD(t *testing.T) {
	repo := newMockObservabilityRepo()
	handler := NewObservabilityHandler(repo)

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	// 1. Create SLO
	createPayload := createSLODefinitionRequest{
		Service:        "tiki_gateway",
		Target:         99.90,
		IndicatorType:  "availability",
		Window:         "30d",
		Query:          `sum(rate(http_requests_total{status=~"2..|3.."}[5m])) / sum(rate(http_requests_total[5m])) * 100`,
		AlertThreshold: 1.5,
	}
	body, _ := json.Marshal(createPayload)
	req := httptest.NewRequest(http.MethodPost, "/slo/definitions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var createResp struct {
		Data observability.SLODefinition `json:"data"`
	}
	_ = json.NewDecoder(w.Body).Decode(&createResp)
	createdID := createResp.Data.ID
	if createdID == "" || createResp.Data.Service != "tiki_gateway" {
		t.Fatalf("invalid created slo: %+v", createResp.Data)
	}

	// 2. List SLO Definitions
	req = httptest.NewRequest(http.MethodGet, "/slo/definitions", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	// 3. Update SLO
	updatePayload := updateSLODefinitionRequest{
		Target: 99.95,
	}
	body, _ = json.Marshal(updatePayload)
	req = httptest.NewRequest(http.MethodPut, "/slo/definitions/"+createdID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	// 4. Trigger Alert
	req = httptest.NewRequest(http.MethodPost, "/slo/definitions/"+createdID+"/trigger-alert", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	// 5. Delete SLO
	req = httptest.NewRequest(http.MethodDelete, "/slo/definitions/"+createdID, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}
}
