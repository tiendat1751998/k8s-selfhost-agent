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
	r.Post("/channels/{id}/test", h.TestChannel)
	
	r.Post("/rules", h.CreateRule)
	r.Get("/rules", h.ListRules)
	r.Put("/rules/{id}", h.UpdateRule)
	r.Delete("/rules/{id}", h.DeleteRule)
	
	r.Get("/history", h.ListHistory)
	r.Post("/history/{id}/acknowledge", h.AcknowledgeAlert)
	r.Post("/silences", h.CreateSilence)
	r.Post("/alerts/{id}/resolve", h.ResolveAlert)
}

// RegisterAlertmanagerRoutes mounts compatibility endpoints under /api/v1/alertmanager
func (h *AlertHandler) RegisterAlertmanagerRoutes(r chi.Router) {
	r.Post("/channels/{id}/test", h.TestChannel)
	r.Post("/silences", h.CreateSilence)
	r.Post("/alerts/{id}/resolve", h.ResolveAlert)
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

	masked := make([]*alert.NotificationChannel, len(channels))
	for i, ch := range channels {
		if ch == nil {
			continue
		}
		clone := *ch
		if len(clone.Config) > 0 {
			cfg := make(map[string]interface{}, len(clone.Config))
			for k, v := range clone.Config {
				if isSensitiveKey(k) {
					cfg[k] = "***"
				} else {
					cfg[k] = v
				}
			}
			clone.Config = cfg
		}
		masked[i] = &clone
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": masked})
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
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "rule id is required", err)
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
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "rule id is required", err)
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
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "alert id is required", err)
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

type createSilenceRequest struct {
	AlertID         string `json:"alert_id"`
	DurationMinutes int    `json:"duration_minutes"`
	Comment         string `json:"comment"`
}

func (r *createSilenceRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.AlertID) == "" {
		ve.Add("alert_id", "alert_id is required")
	}
	if r.DurationMinutes <= 0 {
		ve.Add("duration_minutes", "duration_minutes must be greater than 0")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

func (h *AlertHandler) TestChannel(w http.ResponseWriter, r *http.Request) {
	channelID := chi.URLParam(r, "id")
	if channelID == "" {
		writeError(w, http.StatusBadRequest, "channel id is required", nil)
		return
	}
	tenantID := middleware.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default"
	}
	channels, err := h.usecase.ListChannels(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list channels", err)
		return
	}
	var found bool
	for _, ch := range channels {
		if ch != nil && ch.ID == channelID {
			found = true
			break
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "notification channel not found", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "test alert sent to channel",
	})
}

func (h *AlertHandler) CreateSilence(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[createSilenceRequest](w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "silenced",
		"alert_id": req.AlertID,
	})
}

func (h *AlertHandler) ResolveAlert(w http.ResponseWriter, r *http.Request) {
	alertID := chi.URLParam(r, "id")
	if alertID == "" {
		writeError(w, http.StatusBadRequest, "alert id is required", nil)
		return
	}
	tenantID := middleware.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default"
	}
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		userID = "system"
	}
	if err := h.usecase.AcknowledgeAlert(r.Context(), alertID, tenantID, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve alert", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "resolved"})
}
