package http

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/automation"
)

// AutomationHandler provides HTTP handlers for automation rules API.
type AutomationHandler struct {
	repo automation.Repository
}

// NewAutomationHandler creates a new automation HTTP handler.
func NewAutomationHandler(repo automation.Repository) *AutomationHandler {
	return &AutomationHandler{repo: repo}
}

// RegisterRoutes registers automation routes.
func (h *AutomationHandler) RegisterRoutes(r chi.Router) {
	r.Get("/rules", h.ListRules)
	r.Post("/rules", h.CreateRule)
	r.Put("/rules/{id}", h.UpdateRule)
	r.Delete("/rules/{id}", h.DeleteRule)
	r.Put("/rules/{id}/toggle", h.ToggleRule)
	r.Get("/executions", h.ListExecutions)
}

// ListRules handles GET /api/v1/automation/rules
func (h *AutomationHandler) ListRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.repo.ListRules(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list rules", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": rules})
}

type createRuleRequest struct {
	automation.Rule
}

func (r *createRuleRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Name) == "" {
		ve.Add("name", "name is required")
	}
	if strings.TrimSpace(string(r.TriggerType)) == "" {
		ve.Add("trigger_type", "trigger_type is required")
	}
	if strings.TrimSpace(string(r.ActionType)) == "" {
		ve.Add("action_type", "action_type is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// CreateRule handles POST /api/v1/automation/rules
func (h *AutomationHandler) CreateRule(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[createRuleRequest](w, r)
	if !ok {
		return
	}
	if err := h.repo.CreateRule(r.Context(), &req.Rule); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create rule", err)
		return
	}
	writeJSON(w, http.StatusCreated, req.Rule)
}

type updateRuleRequest struct {
	automation.Rule
}

func (r *updateRuleRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Name) == "" {
		ve.Add("name", "name is required")
	}
	if strings.TrimSpace(string(r.TriggerType)) == "" {
		ve.Add("trigger_type", "trigger_type is required")
	}
	if strings.TrimSpace(string(r.ActionType)) == "" {
		ve.Add("action_type", "action_type is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// UpdateRule handles PUT /api/v1/automation/rules/{id}
func (h *AutomationHandler) UpdateRule(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "rule id is required", err)
		return
	}
	req, ok := decodeJSON[updateRuleRequest](w, r)
	if !ok {
		return
	}
	req.ID = id
	if err := h.repo.UpdateRule(r.Context(), &req.Rule); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update rule", err)
		return
	}
	writeJSON(w, http.StatusOK, req.Rule)
}

// DeleteRule handles DELETE /api/v1/automation/rules/{id}
func (h *AutomationHandler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "rule id is required", err)
		return
	}
	if err := h.repo.DeleteRule(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete rule", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

type toggleRuleRequest struct {
	Enabled bool `json:"enabled"`
}

// ToggleRule handles PUT /api/v1/automation/rules/{id}/toggle
func (h *AutomationHandler) ToggleRule(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "rule id is required", err)
		return
	}
	req, ok := decodeJSON[toggleRuleRequest](w, r)
	if !ok {
		return
	}
	if err := h.repo.ToggleRule(r.Context(), id, req.Enabled); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to toggle rule", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "enabled": req.Enabled})
}

// ListExecutions handles GET /api/v1/automation/executions
func (h *AutomationHandler) ListExecutions(w http.ResponseWriter, r *http.Request) {
	limit := parseIntParam(r, "limit", 50)
	offset := parseIntParam(r, "offset", 0)
	items, total, err := h.repo.ListExecutions(r.Context(), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list executions", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}
