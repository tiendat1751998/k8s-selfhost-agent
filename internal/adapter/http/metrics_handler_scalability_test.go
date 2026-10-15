package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/usecase/metrics"
)

func createTestOverviewWithNodes(count int, processesPerNode int) *metrics.SystemOverview {
	nodes := make([]metrics.NodeMetrics, count)
	for i := 0; i < count; i++ {
		procs := make([]metrics.ProcessMetric, processesPerNode)
		for j := 0; j < processesPerNode; j++ {
			procs[j] = metrics.ProcessMetric{
				PID:         1000 + j,
				Name:        fmt.Sprintf("process-%d", j),
				CPUPercent:  1.5,
				MemoryBytes: 1024 * 1024 * 100,
			}
		}
		nodes[i] = metrics.NodeMetrics{
			NodeID:       fmt.Sprintf("node-%04d", i),
			NodeName:     fmt.Sprintf("worker-%04d", i),
			Status:       "ready",
			TopProcesses: procs,
		}
	}

	return &metrics.SystemOverview{
		TotalNodes: count,
		Nodes:      nodes,
		Containers: make([]metrics.ContainerMetrics, 0),
		Alerts:     make([]metrics.MetricAlert, 0),
	}
}

func TestOverviewHandler_GetOverview_AutoStripTopProcesses_WhenMoreThan100Nodes(t *testing.T) {
	collector := metrics.NewCollector(nil, nil, nil, zap.NewNop())
	overview := createTestOverviewWithNodes(105, 3)
	collector.SetLastSnapshot(overview)

	handler := NewOverviewHandler(collector, zap.NewNop())
	r := chi.NewRouter()
	r.Route("/overview", handler.RegisterRoutes)

	// 1. Default request with >100 nodes: TopProcesses must be stripped
	{
		req := httptest.NewRequest(http.MethodGet, "/overview", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp metrics.SystemOverview
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode: %v", err)
		}

		if len(resp.Nodes) != 105 {
			t.Fatalf("expected 105 nodes, got %d", len(resp.Nodes))
		}
		for i, n := range resp.Nodes {
			if len(n.TopProcesses) != 0 {
				t.Fatalf("expected empty TopProcesses on node %d, got %d processes", i, len(n.TopProcesses))
			}
		}
	}

	// 2. Request with ?full=true: TopProcesses must be retained even with >100 nodes
	{
		req := httptest.NewRequest(http.MethodGet, "/overview?full=true", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp metrics.SystemOverview
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode: %v", err)
		}

		if len(resp.Nodes) != 105 {
			t.Fatalf("expected 105 nodes, got %d", len(resp.Nodes))
		}
		for i, n := range resp.Nodes {
			if len(n.TopProcesses) != 3 {
				t.Fatalf("expected 3 TopProcesses on node %d with ?full=true, got %d", i, len(n.TopProcesses))
			}
		}
	}
}

func TestOverviewHandler_GetOverview_SummaryParam_WhenLessThanOrEqual100Nodes(t *testing.T) {
	collector := metrics.NewCollector(nil, nil, nil, zap.NewNop())
	overview := createTestOverviewWithNodes(10, 3)
	collector.SetLastSnapshot(overview)

	handler := NewOverviewHandler(collector, zap.NewNop())
	r := chi.NewRouter()
	r.Route("/overview", handler.RegisterRoutes)

	// 1. Default request with <= 100 nodes: TopProcesses retained
	{
		req := httptest.NewRequest(http.MethodGet, "/overview", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp metrics.SystemOverview
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode: %v", err)
		}

		if len(resp.Nodes) != 10 {
			t.Fatalf("expected 10 nodes, got %d", len(resp.Nodes))
		}
		for i, n := range resp.Nodes {
			if len(n.TopProcesses) != 3 {
				t.Fatalf("expected 3 TopProcesses on node %d, got %d", i, len(n.TopProcesses))
			}
		}
	}

	// 2. Request with ?summary=true: TopProcesses stripped even with <= 100 nodes
	{
		req := httptest.NewRequest(http.MethodGet, "/overview?summary=true", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp metrics.SystemOverview
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode: %v", err)
		}

		if len(resp.Nodes) != 10 {
			t.Fatalf("expected 10 nodes, got %d", len(resp.Nodes))
		}
		for i, n := range resp.Nodes {
			if len(n.TopProcesses) != 0 {
				t.Fatalf("expected empty TopProcesses on node %d with ?summary=true, got %d", i, len(n.TopProcesses))
			}
		}
	}
}
