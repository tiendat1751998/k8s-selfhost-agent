package http

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

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
	r.Delete("/channels/{id}", h.DeleteChannel)
	r.Post("/channels/{id}/test", h.TestChannel)

	r.Post("/rules", h.CreateRule)
	r.Get("/rules", h.ListRules)
	r.Put("/rules/{id}", h.UpdateRule)
	r.Delete("/rules/{id}", h.DeleteRule)

	r.Get("/history", h.ListHistory)
	r.Post("/history/{id}/acknowledge", h.AcknowledgeAlert)
	r.Post("/silences", h.CreateSilence)
	r.Get("/silences", h.ListSilences)
	r.Post("/alerts/{id}/resolve", h.ResolveAlert)
}

// RegisterAlertmanagerRoutes mounts compatibility endpoints under /api/v1/alertmanager
func (h *AlertHandler) RegisterAlertmanagerRoutes(r chi.Router) {
	r.Post("/channels/{id}/test", h.TestChannel)
	r.Post("/silences", h.CreateSilence)
	r.Get("/silences", h.ListSilences)
	r.Post("/alerts/{id}/resolve", h.ResolveAlert)
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

func (h *AlertHandler) DeleteChannel(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "channel id is required", err)
		return
	}

	tenantID := middleware.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default"
	}

	if err := h.usecase.DeleteChannel(r.Context(), id, tenantID); err != nil {
		if errors.Is(err, alert.ErrNotFound) || strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "notification channel not found", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete channel", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
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

	req, ok := decodeJSON[updateAlertRuleRequest](w, r)
	if !ok {
		return
	}

	tenantID := middleware.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default"
	}

	rules, err := h.usecase.ListRules(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load existing alert rules", err)
		return
	}

	var existing *alert.AlertRule
	for _, rule := range rules {
		if rule != nil && rule.ID == id {
			existing = rule
			break
		}
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "alert rule not found", nil)
		return
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Description != nil {
		existing.Description = *req.Description
	}
	if req.MetricName != nil {
		existing.MetricName = *req.MetricName
	}
	if req.Condition != nil {
		existing.Condition = *req.Condition
	}
	if req.Threshold != nil {
		existing.Threshold = *req.Threshold
	}
	if req.DurationSeconds != nil {
		existing.DurationSeconds = *req.DurationSeconds
	}
	if req.Severity != nil {
		existing.Severity = *req.Severity
	}
	if req.ChannelIDs != nil {
		existing.ChannelIDs = *req.ChannelIDs
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}

	if err := h.usecase.UpdateRule(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update rule", err)
		return
	}
	writeJSON(w, http.StatusOK, existing)
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
	var targetCh *alert.NotificationChannel
	for _, ch := range channels {
		if ch != nil && ch.ID == channelID {
			targetCh = ch
			break
		}
	}
	if targetCh == nil {
		writeError(w, http.StatusNotFound, "notification channel not found", nil)
		return
	}

	result, err := testNotificationChannel(r.Context(), targetCh)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *AlertHandler) CreateSilence(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[createSilenceRequest](w, r)
	if !ok {
		return
	}
	tenantID := middleware.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default"
	}
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		userID = "operator"
	}
	now := time.Now().UTC()
	dur := req.DurationMinutes
	if dur <= 0 {
		dur = 60
	}
	silence := &alert.Silence{
		ID:              uuid.NewString(),
		TenantID:        tenantID,
		AlertID:         req.AlertID,
		Comment:         req.Comment,
		CreatedBy:       userID,
		DurationMinutes: dur,
		StartsAt:        now,
		EndsAt:          now.Add(time.Duration(dur) * time.Minute),
		CreatedAt:       now,
	}
	if err := h.usecase.CreateSilence(r.Context(), silence); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create silence", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "silenced",
		"id":       silence.ID,
		"alert_id": silence.AlertID,
		"ends_at":  silence.EndsAt,
	})
}

func (h *AlertHandler) ListSilences(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default"
	}
	silences, err := h.usecase.ListSilences(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list silences", err)
		return
	}
	if silences == nil {
		silences = make([]*alert.Silence, 0)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": silences,
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
	if err := h.usecase.ResolveAlert(r.Context(), alertID, tenantID, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve alert", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "resolved"})
}

