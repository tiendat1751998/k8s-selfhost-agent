package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestCostHandler_OptimizationMutationsReturn501(t *testing.T) {
	calc := &mockCostCalculator{}
	handler := NewCostHandler(calc)
	r := chi.NewRouter()
	r.Route("/cost", handler.RegisterRoutes)

	testCases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{
			name:   "Optimize POST /cost/optimize",
			method: http.MethodPost,
			path:   "/cost/optimize",
			body:   `{"cluster":"fleet-primary","target":"rightsizing"}`,
		},
		{
			name:   "ApplyRecommendation POST /cost/recommendations/rec-123/apply",
			method: http.MethodPost,
			path:   "/cost/recommendations/rec-123/apply",
			body:   `{}`,
		},
		{
			name:   "SetBudget POST /cost/budget",
			method: http.MethodPost,
			path:   "/cost/budget",
			body:   `{"monthly_limit":1000}`,
		},
		{
			name:   "UpdateBudget PUT /cost/budget",
			method: http.MethodPut,
			path:   "/cost/budget",
			body:   `{"monthly_limit":1200}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var bodyReader *strings.Reader
			if tc.body != "" {
				bodyReader = strings.NewReader(tc.body)
			} else {
				bodyReader = strings.NewReader("")
			}

			req := httptest.NewRequest(tc.method, tc.path, bodyReader)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotImplemented {
				t.Fatalf("expected 501 Not Implemented, got %d: %s", rec.Code, rec.Body.String())
			}

			var resp map[string]interface{}
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode json response: %v", err)
			}

			expectedErr := "Cost optimization controller not configured in this edition"
			if errMsg, _ := resp["error"].(string); errMsg != expectedErr {
				t.Errorf("expected error %q, got %q", expectedErr, errMsg)
			}

			if code, _ := resp["code"].(float64); int(code) != http.StatusNotImplemented {
				t.Errorf("expected code 501, got %v", resp["code"])
			}
		})
	}
}
