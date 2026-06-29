package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/changes"
)

// ChangeHandler provides HTTP handlers for the change management API.
type ChangeHandler struct {
	repo changes.Repository
}

// NewChangeHandler creates a new change management HTTP handler.
func NewChangeHandler(repo changes.Repository) *ChangeHandler {
	return &ChangeHandler{repo: repo}
}

// RegisterRoutes registers change management routes.
func (h *ChangeHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.ListChanges)
	r.Post("/", h.CreateChange)
	r.Put("/{id}/approve", h.ApproveChange)
	r.Put("/{id}/reject", h.RejectChange)
}

// ListChanges handles GET /api/v1/changes
func (h *ChangeHandler) ListChanges(w http.ResponseWriter, r *http.Request) {
	limit := parseIntParam(r, "limit", 50)
	offset := parseIntParam(r, "offset", 0)

	var st *changes.ChangeStatus
	if s := r.URL.Query().Get("status"); s != "" {
		ds := changes.ChangeStatus(s)
		st = &ds
	}

	items, total, err := h.repo.ListRequests(r.Context(), st, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list change requests", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}

// CreateChange handles POST /api/v1/changes
func (h *ChangeHandler) CreateChange(w http.ResponseWriter, r *http.Request) {
	var c changes.ChangeRequest
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	if err := h.repo.CreateRequest(r.Context(), &c); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create change request", err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

// ApproveChange handles PUT /api/v1/changes/{id}/approve
func (h *ChangeHandler) ApproveChange(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	approver := r.URL.Query().Get("approver")
	if approver == "" {
		approver = "system"
	}

	if err := h.repo.ApproveRequest(r.Context(), id, approver); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to approve change", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "approved"})
}

// RejectChange handles PUT /api/v1/changes/{id}/reject
func (h *ChangeHandler) RejectChange(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	approver := r.URL.Query().Get("approver")
	if approver == "" {
		approver = "system"
	}

	if err := h.repo.RejectRequest(r.Context(), id, approver); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reject change", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "rejected"})
}
