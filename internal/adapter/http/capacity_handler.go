package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/capacity"
)

// CapacityHandler provides HTTP handlers for the capacity API.
type CapacityHandler struct {
	repo capacity.Repository
}

// NewCapacityHandler creates a new capacity HTTP handler.
func NewCapacityHandler(repo capacity.Repository) *CapacityHandler {
	return &CapacityHandler{repo: repo}
}

// RegisterRoutes registers capacity routes.
func (h *CapacityHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.ListForecasts)
	r.Post("/", h.RecordForecast)
}

// ListForecasts handles GET /api/v1/capacity
func (h *CapacityHandler) ListForecasts(w http.ResponseWriter, r *http.Request) {
	cluster := r.URL.Query().Get("cluster")
	items, err := h.repo.List(r.Context(), cluster)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list capacity forecasts", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": items})
}

// RecordForecast handles POST /api/v1/capacity
func (h *CapacityHandler) RecordForecast(w http.ResponseWriter, r *http.Request) {
	var f capacity.Forecast
	if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	if err := h.repo.Record(r.Context(), &f); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record capacity forecast", err)
		return
	}
	writeJSON(w, http.StatusCreated, f)
}
