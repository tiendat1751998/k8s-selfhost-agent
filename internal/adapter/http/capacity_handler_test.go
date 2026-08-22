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

	"github.com/datdt/k8sselfhost/internal/domain/capacity"
)

type mockCapacityForecaster struct {
	items []capacity.Forecast
	err   error
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
