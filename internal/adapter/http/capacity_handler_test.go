package http

import (
	"bytes"
	"errors"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/capacity"
)

type mockCapacityForecaster struct {
	items     []capacity.Forecast
	headrooms []capacity.NodeHeadroom
	err       error
}

func (m *mockCapacityForecaster) List(ctx context.Context, cluster string) ([]capacity.Forecast, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.items, nil
}

func (m *mockCapacityForecaster) Record(ctx context.Context, f *capacity.Forecast) error {
	if m.err != nil {
		return m.err
	}
	f.ID = "fc-new-id"
	m.items = append(m.items, *f)
	return nil
}

func (m *mockCapacityForecaster) ListNodeHeadroom(ctx context.Context, cluster string) ([]capacity.NodeHeadroom, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.headrooms, nil
}

func TestCapacityHandler_ListForecasts(t *testing.T) {
	fc := &mockCapacityForecaster{
		items: []capacity.Forecast{
			{
				ID:           "cap-cpu-fleet-primary",
				Cluster:      "fleet-primary",
				ResourceType: capacity.ResourceCPU,
				CurrentUsage: 45.0,
				Forecast7d:   46.6,
				Forecast30d:  51.8,
				Forecast90d:  65.3,
				Status:       "healthy",
				RecordedAt:   time.Now().UTC(),
			},
		},
	}

	handler := NewCapacityHandler(fc)
	r := chi.NewRouter()
	r.Route("/capacity", handler.RegisterRoutes)

	req := httptest.NewRequest(http.MethodGet, "/capacity?cluster=fleet-primary", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		Data []capacity.Forecast `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode forecasts response: %v", err)
	}

	if len(resp.Data) != 1 || resp.Data[0].ResourceType != capacity.ResourceCPU {
		t.Errorf("unexpected forecasts response: %+v", resp.Data)
	}
}

func TestCapacityHandler_RecordForecast(t *testing.T) {
	fc := &mockCapacityForecaster{}
	handler := NewCapacityHandler(fc)
	r := chi.NewRouter()
	r.Route("/capacity", handler.RegisterRoutes)

	payload := map[string]interface{}{
		"cluster":       "fleet-primary",
		"resource_type": "cpu",
		"current_usage": 50.0,
		"forecast_7d":   52.0,
		"forecast_30d":  58.0,
		"forecast_90d":  70.0,
		"status":        "healthy",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/capacity", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCapacityHandler_ListNodeHeadroom(t *testing.T) {
	fc := &mockCapacityForecaster{
		headrooms: []capacity.NodeHeadroom{
			{
				ID:                "node-1",
				Name:              "k8smaster",
				Role:              "control-plane",
				CPUTotalCores:     4.0,
				CPUAllocatedCores: 1.0,
				CPUUsagePercent:   25.0,
				MemTotalGiB:       16.0,
				MemAllocatedGiB:   4.0,
				MemUsagePercent:   25.0,
				PodCount:          15,
				PodCapacity:       60,
				BinPackingScore:   25.0,
				Status:            "healthy",
				HeadroomPercent:   75.0,
			},
			{
				ID:                "node-2",
				Name:              "k8sworker-01",
				Role:              "worker",
				CPUTotalCores:     8.0,
				CPUAllocatedCores: 5.8,
				CPUUsagePercent:   72.0,
				MemTotalGiB:       32.0,
				MemAllocatedGiB:   24.0,
				MemUsagePercent:   75.0,
				PodCount:          35,
				PodCapacity:       70,
				BinPackingScore:   73.5,
				Status:            "warning",
				HeadroomPercent:   25.0,
			},
		},
	}

	handler := NewCapacityHandler(fc)
	r := chi.NewRouter()
	r.Route("/capacity", handler.RegisterRoutes)

	req := httptest.NewRequest(http.MethodGet, "/capacity/nodes?cluster=fleet-primary", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp []capacity.NodeHeadroom
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode node headroom response: %v", err)
	}

	if len(resp) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(resp))
	}
	if resp[0].Name != "k8smaster" || resp[0].Role != "control-plane" {
		t.Errorf("unexpected node 0: %+v", resp[0])
	}
	if resp[1].Name != "k8sworker-01" || resp[1].Role != "worker" {
		t.Errorf("unexpected node 1: %+v", resp[1])
	}
}

func TestCapacityHandler_ListNodeHeadroom_Error(t *testing.T) {
	fc := &mockCapacityForecaster{
		err: errors.New("metrics unavailable"),
	}

	handler := NewCapacityHandler(fc)
	r := chi.NewRouter()
	r.Route("/capacity", handler.RegisterRoutes)

	req := httptest.NewRequest(http.MethodGet, "/capacity/nodes", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}
}
