package http

import (
	"net/http"
	"strings"

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

type createDriftRequest struct {
	drift.DriftRecord
}

func (r *createDriftRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Cluster) == "" {
		ve.Add("cluster", "cluster is required")
	}
	if strings.TrimSpace(r.Resource) == "" {
		ve.Add("resource", "resource is required")
	}
	if strings.TrimSpace(r.ResourceKind) == "" {
		ve.Add("resource_kind", "resource_kind is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// CreateDrift handles POST /api/v1/drift
func (h *DriftHandler) CreateDrift(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[createDriftRequest](w, r)
	if !ok {
		return
	}
	if err := h.repo.Create(r.Context(), &req.DriftRecord); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create drift record", err)
		return
	}
	writeJSON(w, http.StatusCreated, req.DriftRecord)
}

// ResolveDrift handles PUT /api/v1/drift/{id}/resolve
func (h *DriftHandler) ResolveDrift(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "drift id is required", err)
		return
	}
	if err := h.repo.Resolve(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve drift", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "resolved"})
}
