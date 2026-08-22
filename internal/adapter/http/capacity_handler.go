package http

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/capacity"
)

// CapacityForecaster defines the interface for capacity forecasting.
type CapacityForecaster interface {
	List(ctx context.Context, cluster string) ([]capacity.Forecast, error)
	Record(ctx context.Context, f *capacity.Forecast) error
}

// CapacityHandler provides HTTP handlers for the capacity API.
type CapacityHandler struct {
	forecaster CapacityForecaster
}

// NewCapacityHandler creates a new capacity HTTP handler.
func NewCapacityHandler(forecaster CapacityForecaster) *CapacityHandler {
	return &CapacityHandler{forecaster: forecaster}
}

// RegisterRoutes registers capacity routes.
func (h *CapacityHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.ListForecasts)
	r.Post("/", h.RecordForecast)
}

// ListForecasts handles GET /api/v1/capacity
func (h *CapacityHandler) ListForecasts(w http.ResponseWriter, r *http.Request) {
	cluster := r.URL.Query().Get("cluster")
	items, err := h.forecaster.List(r.Context(), cluster)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list capacity forecasts", err)
		return
	}
	if items == nil {
		items = []capacity.Forecast{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": items})
}

type recordForecastRequest struct {
	capacity.Forecast
}

func (r *recordForecastRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Cluster) == "" {
		ve.Add("cluster", "cluster is required")
	}
	if strings.TrimSpace(string(r.ResourceType)) == "" {
		ve.Add("resource_type", "resource_type is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// RecordForecast handles POST /api/v1/capacity
func (h *CapacityHandler) RecordForecast(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[recordForecastRequest](w, r)
	if !ok {
		return
	}
	if err := h.forecaster.Record(r.Context(), &req.Forecast); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record capacity forecast", err)
		return
	}
	writeJSON(w, http.StatusCreated, &req.Forecast)
}
