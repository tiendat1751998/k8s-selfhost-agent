package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/drift"
)

// DriftHandler provides HTTP handlers for the drift detection API.
type DriftHandler struct {
	repo drift.Repository
}

// NewDriftHandler creates a new drift HTTP handler.
func NewDriftHandler(repo drift.Repository) *DriftHandler {
	return &DriftHandler{repo: repo}
}

// RegisterRoutes registers drift routes.
func (h *DriftHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.ListDrifts)
	r.Post("/", h.CreateDrift)
	r.Put("/{id}/resolve", h.ResolveDrift)
}

// ListDrifts handles GET /api/v1/drift
func (h *DriftHandler) ListDrifts(w http.ResponseWriter, r *http.Request) {
	limit := parseIntParam(r, "limit", 50)
	offset := parseIntParam(r, "offset", 0)

	var st *drift.DriftStatus
	if s := r.URL.Query().Get("status"); s != "" {
		ds := drift.DriftStatus(s)
		st = &ds
	}

	cluster := r.URL.Query().Get("cluster")

	items, total, err := h.repo.List(r.Context(), cluster, st, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list drift records", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}

// CreateDrift handles POST /api/v1/drift
func (h *DriftHandler) CreateDrift(w http.ResponseWriter, r *http.Request) {
	var d drift.DriftRecord
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	if err := h.repo.Create(r.Context(), &d); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create drift record", err)
		return
	}
	writeJSON(w, http.StatusCreated, d)
}

// ResolveDrift handles PUT /api/v1/drift/{id}/resolve
func (h *DriftHandler) ResolveDrift(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.repo.Resolve(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve drift", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "resolved"})
}
