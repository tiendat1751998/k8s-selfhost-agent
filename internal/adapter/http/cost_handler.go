package http

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/cost"
)

// CostCalculator defines data access and dynamic computation interfaces for resource cost and waste metrics.
type CostCalculator interface {
	GetClusterCosts(ctx context.Context) ([]cost.ClusterCost, error)
	GetNamespaceCosts(ctx context.Context) ([]cost.NamespaceCost, error)
	GetResourceWaste(ctx context.Context) ([]cost.ResourceWaste, error)
}

// CostHandler provides HTTP endpoints for Cost Management data.
type CostHandler struct {
	calculator CostCalculator
}

// NewCostHandler creates a new CostHandler.
func NewCostHandler(calculator CostCalculator) *CostHandler {
	return &CostHandler{calculator: calculator}
}

// RegisterRoutes registers cost management routes.
func (h *CostHandler) RegisterRoutes(r chi.Router) {
	r.Get("/summary", h.GetSummary)
	r.Get("/waste", h.GetWaste)
}

// GetSummary handles GET /api/v1/cost/summary
func (h *CostHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	clusters, err := h.calculator.GetClusterCosts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get cluster costs", err)
		return
	}
	if clusters == nil {
		clusters = []cost.ClusterCost{}
	}

	namespaces, err := h.calculator.GetNamespaceCosts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get namespace costs", err)
		return
	}
	if namespaces == nil {
		namespaces = []cost.NamespaceCost{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"clusters":   clusters,
		"namespaces": namespaces,
	})
}

// GetWaste handles GET /api/v1/cost/waste
func (h *CostHandler) GetWaste(w http.ResponseWriter, r *http.Request) {
	waste, err := h.calculator.GetResourceWaste(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get resource waste", err)
		return
	}
	if waste == nil {
		waste = []cost.ResourceWaste{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": waste,
	})
}
