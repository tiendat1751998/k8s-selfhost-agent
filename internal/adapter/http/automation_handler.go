package http

import (
	"encoding/json"
	"net/http"

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

// CreateRule handles POST /api/v1/automation/rules
func (h *AutomationHandler) CreateRule(w http.ResponseWriter, r *http.Request) {
	var rule automation.Rule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	if err := h.repo.CreateRule(r.Context(), &rule); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create rule", err)
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

// UpdateRule handles PUT /api/v1/automation/rules/{id}
func (h *AutomationHandler) UpdateRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var rule automation.Rule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	rule.ID = id
	if err := h.repo.UpdateRule(r.Context(), &rule); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update rule", err)
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

// DeleteRule handles DELETE /api/v1/automation/rules/{id}
func (h *AutomationHandler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.repo.DeleteRule(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete rule", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ToggleRule handles PUT /api/v1/automation/rules/{id}/toggle
func (h *AutomationHandler) ToggleRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	if err := h.repo.ToggleRule(r.Context(), id, body.Enabled); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to toggle rule", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "enabled": body.Enabled})
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
