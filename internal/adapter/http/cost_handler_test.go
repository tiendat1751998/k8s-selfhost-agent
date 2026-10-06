package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/cost"
)

type mockCostCalculator struct {
	clusters   []cost.ClusterCost
	namespaces []cost.NamespaceCost
	waste      []cost.ResourceWaste
	err        error
}

func (m *mockCostCalculator) GetClusterCosts(ctx context.Context) ([]cost.ClusterCost, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.clusters, nil
}

func (m *mockCostCalculator) GetNamespaceCosts(ctx context.Context) ([]cost.NamespaceCost, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.namespaces, nil
}

func (m *mockCostCalculator) GetResourceWaste(ctx context.Context) ([]cost.ResourceWaste, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.waste, nil
}

func TestCostHandler_GetSummary(t *testing.T) {
	calc := &mockCostCalculator{
		clusters: []cost.ClusterCost{
			{
				ID:          "cluster-fleet-primary",
				Name:        "fleet-primary",
				Provider:    "docker",
				MonthlyCost: 480,
				DailyCost:   16,
				CPUCost:     302,
				MemoryCost:  156,
				StorageCost: 22,
				UpdatedAt:   time.Now().UTC(),
			},
		},
		namespaces: []cost.NamespaceCost{
			{
				ID:              "ns-default",
				Namespace:       "default",
				Cluster:         "fleet-primary",
				CPURequested:    "2.00 vCPU",
				MemoryRequested: "4.00 GB",
				MonthlyCost:     63,
				Utilization:     45,
				UpdatedAt:       time.Now().UTC(),
			},
		},
	}

	handler := NewCostHandler(calc)
	r := chi.NewRouter()
	r.Route("/cost", handler.RegisterRoutes)

	req := httptest.NewRequest(http.MethodGet, "/cost/summary", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		Clusters   []cost.ClusterCost   `json:"clusters"`
		Namespaces []cost.NamespaceCost `json:"namespaces"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode summary response: %v", err)
	}

	if len(resp.Clusters) != 1 || resp.Clusters[0].MonthlyCost != 480 {
		t.Errorf("unexpected clusters response: %+v", resp.Clusters)
	}
	if len(resp.Namespaces) != 1 || resp.Namespaces[0].Namespace != "default" {
		t.Errorf("unexpected namespaces response: %+v", resp.Namespaces)
	}
}

func TestCostHandler_GetWaste(t *testing.T) {
	cpuUtil := 2
	memUtil := 4
	calc := &mockCostCalculator{
		waste: []cost.ResourceWaste{
			{
				ID:         "waste-c123",
				Type:       "idle_workload",
				Resource:   "monitoring-prometheus",
				Namespace:  "monitoring",
				Cluster:    "fleet-primary",
				CPUUtil:    &cpuUtil,
				MemUtil:    &memUtil,
				WastedCost: 15,
				Severity:   "critical",
				UpdatedAt:  time.Now().UTC(),
			},
		},
	}

	handler := NewCostHandler(calc)
	r := chi.NewRouter()
	r.Route("/cost", handler.RegisterRoutes)

	req := httptest.NewRequest(http.MethodGet, "/cost/waste", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		Data []cost.ResourceWaste `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode waste response: %v", err)
	}

	if len(resp.Data) != 1 || resp.Data[0].Resource != "monitoring-prometheus" {
		t.Errorf("unexpected waste response: %+v", resp.Data)
	}
}
