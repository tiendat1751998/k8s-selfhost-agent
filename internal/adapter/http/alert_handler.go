package http

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/alert"
	alertUsecase "github.com/datdt/k8sselfhost/internal/usecase/alert"
)

type AlertHandler struct {
	usecase *alertUsecase.Usecase
}

func NewAlertHandler(usecase *alertUsecase.Usecase) *AlertHandler {
	return &AlertHandler{usecase: usecase}
}

func (h *AlertHandler) RegisterRoutes(r chi.Router) {
	r.Post("/channels", h.CreateChannel)
	r.Get("/channels", h.ListChannels)
	
	r.Post("/rules", h.CreateRule)
	r.Get("/rules", h.ListRules)
	r.Put("/rules/{id}", h.UpdateRule)
	r.Delete("/rules/{id}", h.DeleteRule)
	
	r.Get("/history", h.ListHistory)
	r.Post("/history/{id}/acknowledge", h.AcknowledgeAlert)
}

type createAlertChannelRequest struct {
	Name    string                 `json:"name"`
	Type    string                 `json:"type"`
	Config  map[string]interface{} `json:"config"`
	Enabled bool                   `json:"enabled"`
}

func (r *createAlertChannelRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Name) == "" {
		ve.Add("name", "name is required")
	}
	if strings.TrimSpace(r.Type) == "" {
		ve.Add("type", "type is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

func (h *AlertHandler) CreateChannel(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[createAlertChannelRequest](w, r)
	if !ok {
		return
	}

	tenantID := middleware.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default"
	}

	channel := &alert.NotificationChannel{
		TenantID: tenantID,
		Name:     req.Name,
		Type:     req.Type,
		Config:   req.Config,
		Enabled:  req.Enabled,
	}

	if err := h.usecase.CreateChannel(r.Context(), channel); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create channel", err)
		return
	}
	writeJSON(w, http.StatusCreated, channel)
}

func (h *AlertHandler) ListChannels(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default"
	}

	channels, err := h.usecase.ListChannels(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list channels", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": channels})
}

type createAlertRuleRequest struct {
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	MetricName      string   `json:"metric_name"`
	Condition       string   `json:"condition"`
	Threshold       float64  `json:"threshold"`
	DurationSeconds int      `json:"duration_seconds"`
	Severity        string   `json:"severity"`
	ChannelIDs      []string `json:"channel_ids"`
	Enabled         bool     `json:"enabled"`
}

func (r *createAlertRuleRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Name) == "" {
		ve.Add("name", "name is required")
	}
	if strings.TrimSpace(r.MetricName) == "" {
		ve.Add("metric_name", "metric_name is required")
	}
	if strings.TrimSpace(r.Condition) == "" {
		ve.Add("condition", "condition is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

func (h *AlertHandler) CreateRule(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[createAlertRuleRequest](w, r)
	if !ok {
		return
	}

	tenantID := middleware.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default"
	}

	rule := &alert.AlertRule{
		TenantID:        tenantID,
		Name:            req.Name,
		Description:     req.Description,
		MetricName:      req.MetricName,
		Condition:       req.Condition,
		Threshold:       req.Threshold,
		DurationSeconds: req.DurationSeconds,
		Severity:        req.Severity,
		ChannelIDs:      req.ChannelIDs,
		Enabled:         req.Enabled,
	}

	if err := h.usecase.CreateRule(r.Context(), rule); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create rule", err)
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

func (h *AlertHandler) ListRules(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default"
	}

	rules, err := h.usecase.ListRules(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list rules", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": rules})
}

func (h *AlertHandler) UpdateRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "rule id is required", nil)
		return
	}

	req, ok := decodeJSON[createAlertRuleRequest](w, r)
	if !ok {
		return
	}

	tenantID := middleware.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default"
	}

	rule := &alert.AlertRule{
		ID:              id,
		TenantID:        tenantID,
		Name:            req.Name,
		Description:     req.Description,
		MetricName:      req.MetricName,
		Condition:       req.Condition,
		Threshold:       req.Threshold,
		DurationSeconds: req.DurationSeconds,
		Severity:        req.Severity,
		ChannelIDs:      req.ChannelIDs,
		Enabled:         req.Enabled,
	}

	if err := h.usecase.UpdateRule(r.Context(), rule); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update rule", err)
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (h *AlertHandler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "rule id is required", nil)
		return
	}

	tenantID := middleware.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default"
	}

	if err := h.usecase.DeleteRule(r.Context(), id, tenantID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete rule", err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func (h *AlertHandler) ListHistory(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default"
	}

	history, err := h.usecase.ListHistory(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list history", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": history})
}

func (h *AlertHandler) AcknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "alert id is required", nil)
		return
	}

	tenantID := middleware.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default"
	}

	userID := "system" // Normally from auth token

	if err := h.usecase.AcknowledgeAlert(r.Context(), id, tenantID, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to acknowledge alert", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "acknowledged"})
}
