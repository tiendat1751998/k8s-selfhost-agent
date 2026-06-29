package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/healthcenter"
)

// HealthCenterHandler provides HTTP handlers for the platform health API.
type HealthCenterHandler struct {
	repo healthcenter.Repository
}

// NewHealthCenterHandler creates a new health center HTTP handler.
func NewHealthCenterHandler(repo healthcenter.Repository) *HealthCenterHandler {
	return &HealthCenterHandler{repo: repo}
}

// RegisterRoutes registers health center routes.
func (h *HealthCenterHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.GetHealthStatus)
	r.Post("/ping", h.PingComponent)
}

// GetHealthStatus handles GET /api/v1/health
func (h *HealthCenterHandler) GetHealthStatus(w http.ResponseWriter, r *http.Request) {
	items, err := h.repo.GetStatuses(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get health statuses", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": items})
}

// PingComponent handles POST /api/v1/health/ping
// This endpoint is used by components to report their own health status.
func (h *HealthCenterHandler) PingComponent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Component string `json:"component"`
		Status    string `json:"status"`
		Message   string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	cs := healthcenter.ComponentStatus{
		Component:     req.Component,
		Status:        req.Status,
		Message:       req.Message,
		LastCheckedAt: time.Now(),
	}

	if err := h.repo.UpdateStatus(r.Context(), &cs); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update health status", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
