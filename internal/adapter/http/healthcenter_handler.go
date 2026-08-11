package http

import (
	"net/http"
	"strings"
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

type pingComponentRequest struct {
	Component string `json:"component"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

func (r *pingComponentRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Component) == "" {
		ve.Add("component", "component is required")
	}
	if strings.TrimSpace(r.Status) == "" {
		ve.Add("status", "status is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// PingComponent handles POST /api/v1/health/ping
// This endpoint is used by components to report their own health status.
func (h *HealthCenterHandler) PingComponent(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[pingComponentRequest](w, r)
	if !ok {
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
