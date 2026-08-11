// Package http provides HTTP handlers for platform features.
package http

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/notification"
)

// NotificationHandler provides HTTP handlers for the notification API.
type NotificationHandler struct {
	repo notification.Repository
}

// NewNotificationHandler creates a new notification HTTP handler.
func NewNotificationHandler(repo notification.Repository) *NotificationHandler {
	return &NotificationHandler{repo: repo}
}

// RegisterRoutes registers notification routes.
func (h *NotificationHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.ListNotifications)
	r.Put("/{id}/read", h.MarkRead)
	r.Put("/read-all", h.MarkAllRead)
	r.Get("/channels", h.ListChannels)
	r.Post("/channels", h.CreateChannel)
	r.Delete("/channels/{id}", h.DeleteChannel)
	r.Get("/history", h.ListHistory)
}

// ListNotifications handles GET /api/v1/notifications
func (h *NotificationHandler) ListNotifications(w http.ResponseWriter, r *http.Request) {
	limit := parseIntParam(r, "limit", 50)
	offset := parseIntParam(r, "offset", 0)
	items, total, err := h.repo.ListNotifications(r.Context(), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list notifications", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}

// MarkRead handles PUT /api/v1/notifications/{id}/read
func (h *NotificationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.repo.MarkRead(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to mark read", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// MarkAllRead handles PUT /api/v1/notifications/read-all
func (h *NotificationHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.MarkAllRead(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to mark all read", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ListChannels handles GET /api/v1/notifications/channels
func (h *NotificationHandler) ListChannels(w http.ResponseWriter, r *http.Request) {
	channels, err := h.repo.ListChannels(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list channels", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": channels})
}

type createChannelRequest struct {
	notification.Channel
}

func (r *createChannelRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Name) == "" {
		ve.Add("name", "name is required")
	}
	if strings.TrimSpace(string(r.Type)) == "" {
		ve.Add("type", "type is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// CreateChannel handles POST /api/v1/notifications/channels
func (h *NotificationHandler) CreateChannel(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[createChannelRequest](w, r)
	if !ok {
		return
	}
	if err := h.repo.CreateChannel(r.Context(), &req.Channel); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create channel", err)
		return
	}
	writeJSON(w, http.StatusCreated, req.Channel)
}

// DeleteChannel handles DELETE /api/v1/notifications/channels/{id}
func (h *NotificationHandler) DeleteChannel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.repo.DeleteChannel(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete channel", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ListHistory handles GET /api/v1/notifications/history
func (h *NotificationHandler) ListHistory(w http.ResponseWriter, r *http.Request) {
	limit := parseIntParam(r, "limit", 50)
	offset := parseIntParam(r, "offset", 0)
	items, total, err := h.repo.ListHistory(r.Context(), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list history", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}
