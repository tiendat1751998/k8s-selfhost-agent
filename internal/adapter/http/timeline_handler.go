package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/timeline"
)

// TimelineHandler provides HTTP handlers for the deployment timeline API.
type TimelineHandler struct {
	repo timeline.Repository
}

// NewTimelineHandler creates a new timeline HTTP handler.
func NewTimelineHandler(repo timeline.Repository) *TimelineHandler {
	return &TimelineHandler{repo: repo}
}

// RegisterRoutes registers timeline routes.
func (h *TimelineHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.ListEvents)
	r.Get("/{id}", h.GetEvent)
}

// ListEvents handles GET /api/v1/timeline
func (h *TimelineHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	limit := parseIntParam(r, "limit", 50)
	offset := parseIntParam(r, "offset", 0)

	var eventType *timeline.EventType
	if t := r.URL.Query().Get("type"); t != "" {
		et := timeline.EventType(t)
		eventType = &et
	}

	since := time.Now().AddDate(0, 0, -7) // default last 7 days
	if s := r.URL.Query().Get("range"); s != "" {
		switch s {
		case "24h":
			since = time.Now().Add(-24 * time.Hour)
		case "7d":
			since = time.Now().AddDate(0, 0, -7)
		case "30d":
			since = time.Now().AddDate(0, 0, -30)
		}
	}

	items, total, err := h.repo.List(r.Context(), eventType, since, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list events", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}

// GetEvent handles GET /api/v1/timeline/{id}
func (h *TimelineHandler) GetEvent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ev, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get event", err)
		return
	}
	writeJSON(w, http.StatusOK, ev)
}
