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
	collector := metrics.NewCollector(nil, nil, nil, zap.NewNop())
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
	collector := metrics.NewCollector(nil, nil, nil, zap.NewNop())
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
	collector := metrics.NewCollector(nil, nil, nil, zap.NewNop())
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
	collector := metrics.NewCollector(nil, nil, nil, zap.NewNop())
	handler := NewMetricsHandler(collector, zap.NewNop())
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestOverviewHandler_GetTPS(t *testing.T) {
	collector := metrics.NewCollector(nil, nil, nil, zap.NewNop())
	tpsCollector := metrics.NewTPSCollector(collector, nil, zap.NewNop())
	handler := NewOverviewHandler(collector, zap.NewNop(), tpsCollector)

	r := chi.NewRouter()
	r.Route("/overview", handler.RegisterRoutes)

	// 1. With configured TPS collector
	{
		req := httptest.NewRequest(http.MethodGet, "/overview/tps", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var resp metrics.TPSSnapshot
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode TPS response: %v", err)
		}

		if resp.PerNode == nil {
			t.Errorf("expected non-nil PerNode array")
		}
	}

	// 2. With nil TPS collector (fallback)
	{
		handlerWithoutTPS := NewOverviewHandler(collector, zap.NewNop())
		r2 := chi.NewRouter()
		r2.Route("/overview", handlerWithoutTPS.RegisterRoutes)

		req := httptest.NewRequest(http.MethodGet, "/overview/tps", nil)
		w := httptest.NewRecorder()
		r2.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var resp metrics.TPSSnapshot
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode fallback TPS response: %v", err)
		}

		if resp.PerNode == nil {
			t.Errorf("expected non-nil PerNode array")
		}
	}
}

func TestOverviewHandler_FiveContainers_K8sMaterEndpoints(t *testing.T) {
	collector := metrics.NewCollector(nil, nil, nil, zap.NewNop())
	collector.SetAgentMetric("host-k8smater", &metrics.AgentMetrics{
		Hostname: "k8smater",
		Status:   "online",
	})
	overview, _ := collector.CollectOnce(context.Background())
	// Inject 5 containers
	overview.Containers = []metrics.ContainerMetrics{
		{ContainerID: "c1", ContainerName: "tiki_traefik", ServiceName: "tiki_traefik", NodeID: "host-k8smater", NodeName: "k8smater", State: "running"},
		{ContainerID: "c2", ContainerName: "nats", ServiceName: "nats", NodeID: "host-k8smater", NodeName: "k8smater", State: "running"},
		{ContainerID: "c3", ContainerName: "tiki_redis", ServiceName: "tiki_redis", NodeID: "host-k8smater", NodeName: "k8smater", State: "running"},
		{ContainerID: "c4", ContainerName: "postgres_db", ServiceName: "postgres_db", NodeID: "host-k8smater", NodeName: "k8smater", State: "running"},
		{ContainerID: "c5", ContainerName: "registry", ServiceName: "registry", NodeID: "host-k8smater", NodeName: "k8smater", State: "running"},
	}
	overview.TotalContainers = 5
	overview.RunningContainers = 5
	if len(overview.Nodes) > 0 {
		overview.Nodes[0].ContainerCount = 5
		overview.Nodes[0].RunningCount = 5
	}
	collector.SetLastSnapshot(overview)

	tpsCollector := metrics.NewTPSCollector(collector, nil, zap.NewNop())
	_, _ = tpsCollector.Collect(context.Background())

	handler := NewOverviewHandler(collector, zap.NewNop(), tpsCollector)
	r := chi.NewRouter()
	r.Route("/overview", handler.RegisterRoutes)

	// 1. Verify GET /overview
	{
		req := httptest.NewRequest(http.MethodGet, "/overview", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("GET /overview returned status %d", w.Code)
		}

		var snap metrics.SystemOverview
		if err := json.NewDecoder(w.Body).Decode(&snap); err != nil {
			t.Fatalf("failed to decode overview: %v", err)
		}

		if snap.TotalContainers != 5 {
			t.Errorf("expected TotalContainers = 5, got %d", snap.TotalContainers)
		}
		if snap.RunningContainers != 5 {
			t.Errorf("expected RunningContainers = 5, got %d", snap.RunningContainers)
		}
		if len(snap.Containers) != 5 {
			t.Errorf("expected 5 containers, got %d", len(snap.Containers))
		}
	}

	// 2. Verify GET /overview/tps
	{
		req := httptest.NewRequest(http.MethodGet, "/overview/tps", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("GET /overview/tps returned status %d", w.Code)
		}

		var snap metrics.TPSSnapshot
		if err := json.NewDecoder(w.Body).Decode(&snap); err != nil {
			t.Fatalf("failed to decode tps: %v", err)
		}

		if len(snap.Services) != 5 {
			t.Errorf("expected 5 services, got %d", len(snap.Services))
		}
	}
}


