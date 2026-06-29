package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/cost"
)

// CostHandler provides HTTP endpoints for Cost Management data.
type CostHandler struct {
	repo cost.Repository
}

// NewCostHandler creates a new CostHandler.
func NewCostHandler(repo cost.Repository) *CostHandler {
	return &CostHandler{repo: repo}
}

// RegisterRoutes registers cost management routes.
func (h *CostHandler) RegisterRoutes(r chi.Router) {
	r.Get("/summary", h.GetSummary)
	r.Get("/waste", h.GetWaste)
}

// GetSummary handles GET /api/v1/cost/summary
func (h *CostHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	clusters, err := h.repo.GetClusterCosts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get cluster costs", err)
		return
	}

	namespaces, err := h.repo.GetNamespaceCosts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get namespace costs", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"clusters":   clusters,
		"namespaces": namespaces,
	})
}

// GetWaste handles GET /api/v1/cost/waste
func (h *CostHandler) GetWaste(w http.ResponseWriter, r *http.Request) {
	waste, err := h.repo.GetResourceWaste(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get resource waste", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": waste,
	})
}
