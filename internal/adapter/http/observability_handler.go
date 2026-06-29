package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/observability"
)

// ObservabilityHandler provides HTTP handlers for the observability API.
type ObservabilityHandler struct {
	repo observability.Repository
}

// NewObservabilityHandler creates a new observability HTTP handler.
func NewObservabilityHandler(repo observability.Repository) *ObservabilityHandler {
	return &ObservabilityHandler{repo: repo}
}

// RegisterRoutes registers observability routes.
func (h *ObservabilityHandler) RegisterRoutes(r chi.Router) {
	r.Get("/slo", h.ListSLOSnapshots)
	r.Get("/slo/definitions", h.ListSLODefinitions)
}

// ListSLODefinitions handles GET /api/v1/observability/slo/definitions
func (h *ObservabilityHandler) ListSLODefinitions(w http.ResponseWriter, r *http.Request) {
	defs, err := h.repo.ListSLODefinitions(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list SLO definitions", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": defs})
}

// ListSLOSnapshots handles GET /api/v1/observability/slo
func (h *ObservabilityHandler) ListSLOSnapshots(w http.ResponseWriter, r *http.Request) {
	snapshots, err := h.repo.ListSLOSnapshots(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list SLO snapshots", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": snapshots})
}
