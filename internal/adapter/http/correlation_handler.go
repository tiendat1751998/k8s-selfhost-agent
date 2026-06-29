package http

import (
	"encoding/json"
	"net/http"

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

// CreateCorrelated handles POST /api/v1/correlation
func (h *CorrelationHandler) CreateCorrelated(w http.ResponseWriter, r *http.Request) {
	var c correlation.CorrelatedEvent
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	if err := h.repo.Create(r.Context(), &c); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create correlated event", err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

// ResolveCorrelated handles PUT /api/v1/correlation/{id}/resolve
func (h *CorrelationHandler) ResolveCorrelated(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.repo.Resolve(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve correlated event", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "resolved"})
}
