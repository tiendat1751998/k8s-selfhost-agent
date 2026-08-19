package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/usecase/metrics"
)

// OverviewHandler provides HTTP REST endpoints for retrieving platform infrastructure overview metrics.
type OverviewHandler struct {
	collector *metrics.Collector
	logger    *zap.Logger
}

// MetricsHandler is an alias for OverviewHandler for compatibility.
type MetricsHandler = OverviewHandler

// NewOverviewHandler creates a new OverviewHandler instance.
func NewOverviewHandler(collector *metrics.Collector, logger *zap.Logger) *OverviewHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &OverviewHandler{
		collector: collector,
		logger:    logger,
	}
}

// NewMetricsHandler creates a new MetricsHandler (alias constructor for NewOverviewHandler).
func NewMetricsHandler(collector *metrics.Collector, logger *zap.Logger) *MetricsHandler {
	return NewOverviewHandler(collector, logger)
}

// RegisterRoutes registers the overview metrics routes on a chi.Router.
func (h *OverviewHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.GetOverview)
	r.Get("/nodes", h.GetNodeMetrics)
	r.Get("/alerts", h.GetAlerts)
	r.Get("/containers", h.GetContainerMetrics)
}

// GetOverview handles GET /api/v1/overview
// Returns the full cached SystemOverview including nodes, containers, totals, and alerts.
func (h *OverviewHandler) GetOverview(w http.ResponseWriter, r *http.Request) {
	var snapshot *metrics.SystemOverview
	if h.collector != nil {
		snapshot = h.collector.GetLastSnapshot()
	}
	if snapshot == nil {
		writeJSON(w, http.StatusOK, metrics.SystemOverview{
			Nodes:      make([]metrics.NodeMetrics, 0),
			Containers: make([]metrics.ContainerMetrics, 0),
			Alerts:     make([]metrics.MetricAlert, 0),
		})
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

// GetNodeMetrics handles GET /api/v1/overview/nodes
// Returns the array of node metrics from the latest snapshot.
func (h *OverviewHandler) GetNodeMetrics(w http.ResponseWriter, r *http.Request) {
	var snapshot *metrics.SystemOverview
	if h.collector != nil {
		snapshot = h.collector.GetLastSnapshot()
	}
	if snapshot == nil || snapshot.Nodes == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": make([]metrics.NodeMetrics, 0)})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": snapshot.Nodes})
}

// GetAlerts handles GET /api/v1/overview/alerts
// Returns the active infrastructure metric alerts from the latest snapshot.
func (h *OverviewHandler) GetAlerts(w http.ResponseWriter, r *http.Request) {
	var snapshot *metrics.SystemOverview
	if h.collector != nil {
		snapshot = h.collector.GetLastSnapshot()
	}
	if snapshot == nil || snapshot.Alerts == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": make([]metrics.MetricAlert, 0)})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": snapshot.Alerts})
}

// GetContainerMetrics handles GET /api/v1/overview/containers
// Returns the container metrics array from the latest snapshot.
func (h *OverviewHandler) GetContainerMetrics(w http.ResponseWriter, r *http.Request) {
	var snapshot *metrics.SystemOverview
	if h.collector != nil {
		snapshot = h.collector.GetLastSnapshot()
	}
	if snapshot == nil || snapshot.Containers == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": make([]metrics.ContainerMetrics, 0)})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": snapshot.Containers})
}
