package http

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/audit"
	"github.com/datdt/k8sselfhost/internal/domain/runbook"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

// RunbookHandlerOption configures RunbookHandler.
type RunbookHandlerOption func(*RunbookHandler)

// WithCustomRunbookExecutor sets custom RunbookExecutor.
func WithCustomRunbookExecutor(exec RunbookExecutor) RunbookHandlerOption {
	return func(h *RunbookHandler) {
		h.executor = exec
	}
}

// RunbookHandler provides HTTP handlers for the runbook API.
type RunbookHandler struct {
	repo      runbook.Repository
	auditRepo audit.Repository
	executor  RunbookExecutor
}

// NewRunbookHandler creates a new runbook HTTP handler.
func NewRunbookHandler(repo runbook.Repository, auditRepo audit.Repository, opts ...RunbookHandlerOption) *RunbookHandler {
	h := &RunbookHandler{repo: repo, auditRepo: auditRepo}
	for _, opt := range opts {
		opt(h)
	}
	if h.executor == nil {
		var db postgres.DBTX
		if dbp, ok := repo.(interface{ DB() postgres.DBTX }); ok && dbp.DB() != nil {
			db = dbp.DB()
		}
		h.executor = NewDefaultRunbookExecutor(WithRunbookExecutorDB(db))
	}
	return h
}

// RegisterRoutes registers runbook routes.
func (h *RunbookHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Get("/{id}", h.GetByID)
	r.Put("/{id}", h.Update)
	r.Delete("/{id}", h.Delete)
	r.Post("/{id}/execute", h.Execute)
	r.Post("/{id}/steps/{stepIndex}/execute", h.ExecuteStep)
}

// List handles GET /api/v1/runbooks
func (h *RunbookHandler) List(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": []runbook.Runbook{}, "total": 0})
		return
	}
	limit := parseIntParam(r, "limit", 50)
	offset := parseIntParam(r, "offset", 0)
	category := r.URL.Query().Get("category")
	items, total, err := h.repo.List(r.Context(), category, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list runbooks", err)
		return
	}
	if items == nil {
		items = []runbook.Runbook{}
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

// Execute handles POST /api/v1/runbooks/{id}/execute
func (h *RunbookHandler) Execute(w http.ResponseWriter, r *http.Request) {
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
	now := time.Now().UTC()
	rb.LastUsedAt = &now
	if err := h.repo.Update(r.Context(), rb); err != nil {
		logger.Get().Warn("failed to update runbook execution timestamp", zap.Error(err), zap.String("runbook_id", rb.ID))
	}

	summary, err := h.executor.ExecuteAll(r.Context(), rb)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to execute runbook steps", err)
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		userID = "platform_admin"
	}
	details := map[string]interface{}{
		"title":           rb.Title,
		"category":        rb.Category,
		"steps_count":     summary.TotalSteps,
		"completed_steps": summary.CompletedSteps,
		"status":          summary.Status,
		"duration_ms":     summary.DurationMs,
	}
	if h.auditRepo != nil {
		if auditErr := h.auditRepo.RecordAction(r.Context(), userID, "execute", "runbook", rb.ID, rb.Title, summary.Status, details, r.RemoteAddr, r.Header.Get("User-Agent")); auditErr != nil {
			logger.Get().Error("failed to record audit action for runbook execution", zap.Error(auditErr), zap.String("runbook_id", rb.ID))
		}
	}

	writeJSON(w, http.StatusOK, summary)
}

// ExecuteStep handles POST /api/v1/runbooks/{id}/steps/{stepIndex}/execute
func (h *RunbookHandler) ExecuteStep(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "runbook id is required", err)
		return
	}
	stepIndexParam := chi.URLParam(r, "stepIndex")
	if stepIndexParam == "" {
		stepIndexParam = chi.URLParam(r, "step_index")
	}
	stepIndex, err := strconv.Atoi(stepIndexParam)
	if err != nil || stepIndex < 0 {
		writeError(w, http.StatusBadRequest, "valid non-negative step index is required", err)
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

	res, err := h.executor.ExecuteStep(r.Context(), rb, stepIndex)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to execute runbook step", err)
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		userID = "platform_admin"
	}
	details := map[string]interface{}{
		"title":       rb.Title,
		"step_index":  res.StepIndex,
		"step_title":  res.StepTitle,
		"command":     res.Command,
		"status":      res.Status,
		"duration_ms": res.DurationMs,
	}
	if h.auditRepo != nil {
		if auditErr := h.auditRepo.RecordAction(r.Context(), userID, "execute_step", "runbook", rb.ID, rb.Title, res.Status, details, r.RemoteAddr, r.Header.Get("User-Agent")); auditErr != nil {
			logger.Get().Error("failed to record audit action for step execution", zap.Error(auditErr), zap.String("runbook_id", rb.ID))
		}
	}

	writeJSON(w, http.StatusOK, res)
}

