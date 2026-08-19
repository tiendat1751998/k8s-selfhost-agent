package http

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/correlation"
)

// CorrelationHandler provides HTTP handlers for the event correlation API.
type CorrelationHandler struct {
	repo correlation.Repository
}

// NewCorrelationHandler creates a new correlation HTTP handler.
func NewCorrelationHandler(repo correlation.Repository) *CorrelationHandler {
	return &CorrelationHandler{repo: repo}
}

// RegisterRoutes registers event correlation routes.
func (h *CorrelationHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.ListCorrelated)
	r.Post("/", h.CreateCorrelated)
	r.Put("/{id}/resolve", h.ResolveCorrelated)
}

// ListCorrelated handles GET /api/v1/correlation
func (h *CorrelationHandler) ListCorrelated(w http.ResponseWriter, r *http.Request) {
	limit := parseIntParam(r, "limit", 50)
	offset := parseIntParam(r, "offset", 0)
	status := r.URL.Query().Get("status")

	items, total, err := h.repo.ListCorrelated(r.Context(), status, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list correlated events", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}

type createCorrelatedRequest struct {
	correlation.CorrelatedEvent
}

func (r *createCorrelatedRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Title) == "" {
		ve.Add("title", "title is required")
	}
	if strings.TrimSpace(r.RootCause) == "" {
		ve.Add("root_cause", "root_cause is required")
	}
	if strings.TrimSpace(r.Severity) == "" {
		ve.Add("severity", "severity is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// CreateCorrelated handles POST /api/v1/correlation
func (h *CorrelationHandler) CreateCorrelated(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[createCorrelatedRequest](w, r)
	if !ok {
		return
	}
	if err := h.repo.Create(r.Context(), &req.CorrelatedEvent); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create correlated event", err)
		return
	}
	writeJSON(w, http.StatusCreated, req.CorrelatedEvent)
}

// ResolveCorrelated handles PUT /api/v1/correlation/{id}/resolve
func (h *CorrelationHandler) ResolveCorrelated(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "event id is required", err)
		return
	}
	if err := h.repo.Resolve(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve correlated event", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "resolved"})
}
