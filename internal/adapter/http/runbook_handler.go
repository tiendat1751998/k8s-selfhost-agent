package http

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/audit"
	"github.com/datdt/k8sselfhost/internal/domain/runbook"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

// RunbookHandler provides HTTP handlers for the runbook API.
type RunbookHandler struct {
	repo      runbook.Repository
	auditRepo audit.Repository
}

// NewRunbookHandler creates a new runbook HTTP handler.
func NewRunbookHandler(repo runbook.Repository, auditRepo audit.Repository) *RunbookHandler {
	return &RunbookHandler{repo: repo, auditRepo: auditRepo}
}

// RegisterRoutes registers runbook routes.
func (h *RunbookHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Get("/{id}", h.GetByID)
	r.Put("/{id}", h.Update)
	r.Delete("/{id}", h.Delete)
}

// List handles GET /api/v1/runbooks
func (h *RunbookHandler) List(w http.ResponseWriter, r *http.Request) {
	limit := parseIntParam(r, "limit", 50)
	offset := parseIntParam(r, "offset", 0)
	category := r.URL.Query().Get("category")
	items, total, err := h.repo.List(r.Context(), category, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list runbooks", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}

// GetByID handles GET /api/v1/runbooks/{id}
func (h *RunbookHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "runbook id is required", err)
		return
	}
	rb, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "runbook not found", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get runbook", err)
		return
	}
	writeJSON(w, http.StatusOK, rb)
}

type createRunbookRequest struct {
	runbook.Runbook
}

func (r *createRunbookRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Title) == "" {
		ve.Add("title", "title is required")
	}
	if strings.TrimSpace(r.Category) == "" {
		ve.Add("category", "category is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// Create handles POST /api/v1/runbooks
func (h *RunbookHandler) Create(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[createRunbookRequest](w, r)
	if !ok {
		return
	}
	rb := req.Runbook

	err := h.repo.Create(r.Context(), &rb)
	result := "success"
	var details map[string]interface{}
	if err != nil {
		result = "failure"
		details = map[string]interface{}{"error": err.Error(), "title": rb.Title}
	} else {
		details = map[string]interface{}{"title": rb.Title, "category": rb.Category}
	}

	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		userID = "system"
	}
	if auditErr := h.auditRepo.RecordAction(r.Context(), userID, "create", "system", rb.ID, rb.Title, result, details, r.RemoteAddr, r.Header.Get("User-Agent")); auditErr != nil {
		logger.Get().Error("failed to record audit action for runbook creation", zap.Error(auditErr))
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create runbook", err)
		return
	}
	writeJSON(w, http.StatusCreated, rb)
}

type updateRunbookRequest struct {
	runbook.Runbook
}

func (r *updateRunbookRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Title) == "" {
		ve.Add("title", "title is required")
	}
	if strings.TrimSpace(r.Category) == "" {
		ve.Add("category", "category is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// Update handles PUT /api/v1/runbooks/{id}
func (h *RunbookHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "runbook id is required", err)
		return
	}
	req, ok := decodeJSON[updateRunbookRequest](w, r)
	if !ok {
		return
	}
	rb := req.Runbook
	rb.ID = id

	err = h.repo.Update(r.Context(), &rb)
	result := "success"
	var details map[string]interface{}
	if err != nil {
		result = "failure"
		details = map[string]interface{}{"error": err.Error(), "title": rb.Title}
	} else {
		details = map[string]interface{}{"title": rb.Title, "category": rb.Category}
	}

	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		userID = "system"
	}
	if auditErr := h.auditRepo.RecordAction(r.Context(), userID, "update", "system", rb.ID, rb.Title, result, details, r.RemoteAddr, r.Header.Get("User-Agent")); auditErr != nil {
		logger.Get().Error("failed to record audit action for runbook update", zap.Error(auditErr))
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update runbook", err)
		return
	}
	writeJSON(w, http.StatusOK, rb)
}

// Delete handles DELETE /api/v1/runbooks/{id}
func (h *RunbookHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "runbook id is required", err)
		return
	}

	err = h.repo.Delete(r.Context(), id)
	result := "success"
	var details map[string]interface{}
	if err != nil {
		result = "failure"
		details = map[string]interface{}{"error": err.Error()}
	} else {
		details = map[string]interface{}{}
	}

	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		userID = "system"
	}
	if auditErr := h.auditRepo.RecordAction(r.Context(), userID, "delete", "system", id, id, result, details, r.RemoteAddr, r.Header.Get("User-Agent")); auditErr != nil {
		logger.Get().Error("failed to record audit action for runbook deletion", zap.Error(auditErr))
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete runbook", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
