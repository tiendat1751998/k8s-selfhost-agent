package http

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
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

type createChangeRequest struct {
	changes.ChangeRequest
}

func (r *createChangeRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Title) == "" {
		ve.Add("title", "title is required")
	}
	if strings.TrimSpace(r.Requester) == "" {
		ve.Add("requester", "requester is required")
	}
	if strings.TrimSpace(r.Cluster) == "" {
		ve.Add("cluster", "cluster is required")
	}
	if strings.TrimSpace(r.Namespace) == "" {
		ve.Add("namespace", "namespace is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// CreateChange handles POST /api/v1/changes
func (h *ChangeHandler) CreateChange(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[createChangeRequest](w, r)
	if !ok {
		return
	}
	if err := h.repo.CreateRequest(r.Context(), &req.ChangeRequest); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create change request", err)
		return
	}
	writeJSON(w, http.StatusCreated, req.ChangeRequest)
}

// ApproveChange handles PUT /api/v1/changes/{id}/approve
func (h *ChangeHandler) ApproveChange(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "change id is required", err)
		return
	}
	approver, _ := r.Context().Value(middleware.UserIDKey).(string)

	if err := h.repo.ApproveRequest(r.Context(), id, approver); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to approve change", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "approved"})
}

// RejectChange handles PUT /api/v1/changes/{id}/reject
func (h *ChangeHandler) RejectChange(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "change id is required", err)
		return
	}
	approver, _ := r.Context().Value(middleware.UserIDKey).(string)

	if err := h.repo.RejectRequest(r.Context(), id, approver); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reject change", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "rejected"})
}
