package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/usecase/metrics"
)

func TestOverviewHandler_GetOverview_NilSnapshot(t *testing.T) {
	collector := metrics.NewCollector(nil, nil, zap.NewNop())
	handler := NewOverviewHandler(collector, zap.NewNop())

	r := chi.NewRouter()
	r.Route("/overview", handler.RegisterRoutes)

	req := httptest.NewRequest(http.MethodGet, "/overview", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp metrics.SystemOverview
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Nodes == nil || resp.Containers == nil || resp.Alerts == nil {
		t.Errorf("expected non-nil empty slices in JSON response: %+v", resp)
	}
}

func TestOverviewHandler_GetOverview_WithData(t *testing.T) {
	collector := metrics.NewCollector(nil, nil, zap.NewNop())
	_, _ = collector.CollectOnce(context.Background())

	handler := NewOverviewHandler(collector, zap.NewNop())

	r := chi.NewRouter()
	r.Route("/overview", handler.RegisterRoutes)

	req := httptest.NewRequest(http.MethodGet, "/overview", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp metrics.SystemOverview
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
}

func TestOverviewHandler_GetNodes_And_Alerts_And_Containers(t *testing.T) {
	collector := metrics.NewCollector(nil, nil, zap.NewNop())
	handler := NewOverviewHandler(collector, zap.NewNop())

	r := chi.NewRouter()
	r.Route("/overview", handler.RegisterRoutes)

	// Test GET /overview/nodes
	{
		req := httptest.NewRequest(http.MethodGet, "/overview/nodes", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var resp struct {
			Data []metrics.NodeMetrics `json:"data"`
		}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode nodes response: %v", err)
		}
		if resp.Data == nil {
			t.Errorf("expected non-nil nodes array")
		}
	}

	// Test GET /overview/alerts
	{
		req := httptest.NewRequest(http.MethodGet, "/overview/alerts", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var resp struct {
			Data []metrics.MetricAlert `json:"data"`
		}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode alerts response: %v", err)
		}
		if resp.Data == nil {
			t.Errorf("expected non-nil alerts array")
		}
	}

	// Test GET /overview/containers
	{
		req := httptest.NewRequest(http.MethodGet, "/overview/containers", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var resp struct {
			Data []metrics.ContainerMetrics `json:"data"`
		}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode containers response: %v", err)
		}
		if resp.Data == nil {
			t.Errorf("expected non-nil containers array")
		}
	}
}

func TestMetricsHandler_AliasConstructor(t *testing.T) {
	collector := metrics.NewCollector(nil, nil, zap.NewNop())
	handler := NewMetricsHandler(collector, zap.NewNop())
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}
