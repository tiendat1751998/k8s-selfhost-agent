package http

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/domain/nodemetrics"
	"github.com/datdt/k8sselfhost/internal/usecase/metrics"
)

// OverviewHandler provides HTTP REST endpoints for retrieving platform infrastructure overview metrics.
type OverviewHandler struct {
	collector       *metrics.Collector
	tpsCollector    *metrics.TPSCollector
	nodeMetricsRepo nodemetrics.Repository
	incidentRepo    incident.Repository
	logger          *zap.Logger
}

// MetricsHandler is an alias for OverviewHandler for compatibility.
type MetricsHandler = OverviewHandler

// NewOverviewHandler creates a new OverviewHandler instance.
func NewOverviewHandler(collector *metrics.Collector, logger *zap.Logger, tpsCollector ...*metrics.TPSCollector) *OverviewHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	var tc *metrics.TPSCollector
	if len(tpsCollector) > 0 {
		tc = tpsCollector[0]
	}
	return &OverviewHandler{
		collector:    collector,
		tpsCollector: tc,
		logger:       logger,
	}
}

// NewMetricsHandler creates a new MetricsHandler (alias constructor for NewOverviewHandler).
func NewMetricsHandler(collector *metrics.Collector, logger *zap.Logger, tpsCollector ...*metrics.TPSCollector) *MetricsHandler {
	return NewOverviewHandler(collector, logger, tpsCollector...)
}

// SetTPSCollector sets or updates the TPSCollector instance.
func (h *OverviewHandler) SetTPSCollector(tc *metrics.TPSCollector) {
	h.tpsCollector = tc
}

// SetNodeMetricsRepo sets the node metrics repository for historical telemetry.
func (h *OverviewHandler) SetNodeMetricsRepo(repo nodemetrics.Repository) {
	h.nodeMetricsRepo = repo
}

// SetIncidentRepo sets the incident repository for node audit trails.
func (h *OverviewHandler) SetIncidentRepo(repo incident.Repository) {
	h.incidentRepo = repo
}

// RegisterRoutes registers the overview metrics routes on a chi.Router.
func (h *OverviewHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.GetOverview)
	r.Get("/nodes", h.GetNodeMetrics)
	r.Get("/nodes/{id}/history", h.GetNodeHistory)
	r.Get("/alerts", h.GetAlerts)
	r.Get("/containers", h.GetContainerMetrics)
	r.Get("/tps", h.GetTPS)
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

// GetTPS handles GET /api/v1/overview/tps
// Returns aggregated throughput metrics across network, HTTP, database, messaging, and containers.
func (h *OverviewHandler) GetTPS(w http.ResponseWriter, r *http.Request) {
	if h.tpsCollector != nil {
		snapshot := h.tpsCollector.GetLastSnapshot()
		if snapshot != nil {
			writeJSON(w, http.StatusOK, snapshot)
			return
		}
	}

	writeJSON(w, http.StatusOK, metrics.TPSSnapshot{
		PerNode:  make([]metrics.NodeTPS, 0),
		Services: make([]metrics.ServiceTPS, 0),
	})
}

// parseNodeHistoryTime parses a time string against multiple ISO/RFC formats or returns fallback.
func parseNodeHistoryTime(s string, fallback time.Time) time.Time {
	if s == "" {
		return fallback
	}
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t.UTC()
		}
	}
	return fallback
}

// GetNodeHistory handles GET /api/v1/overview/nodes/{id}/history?range=1h|3h|6h|24h|7d|30d|custom&from=...&to=...
// Returns time-series telemetry rollups, summary statistics, and related node incidents.
func (h *OverviewHandler) GetNodeHistory(w http.ResponseWriter, r *http.Request) {
	nodeID := chi.URLParam(r, "id")
	if nodeID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "node id is required"})
		return
	}

	fromParam := strings.TrimSpace(r.URL.Query().Get("from"))
	toParam := strings.TrimSpace(r.URL.Query().Get("to"))
	rangeParam := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("range")))

	now := time.Now().UTC()
	var startTime, endTime time.Time
	var resolution string
	limit := 500

	if fromParam != "" || rangeParam == "custom" {
		rangeParam = "custom"
		startTime = parseNodeHistoryTime(fromParam, now.Add(-24*time.Hour))
		endTime = parseNodeHistoryTime(toParam, now)
		if startTime.After(endTime) {
			startTime, endTime = endTime, startTime
		}
		duration := endTime.Sub(startTime)
		if duration <= 2*time.Hour {
			resolution = "1m"
			limit = 500
		} else if duration <= 24*time.Hour {
			resolution = "1m"
			limit = 500
		} else if duration <= 7*24*time.Hour {
			resolution = "1h"
			limit = 500
		} else {
			resolution = "1h"
			limit = 1000
		}
	} else {
		endTime = now
		switch rangeParam {
		case "1h":
			startTime = now.Add(-1 * time.Hour)
			resolution = "1m"
			limit = 500
		case "3h":
			startTime = now.Add(-3 * time.Hour)
			resolution = "1m"
			limit = 500
		case "6h":
			startTime = now.Add(-6 * time.Hour)
			resolution = "1m"
			limit = 500
		case "7d":
			startTime = now.Add(-7 * 24 * time.Hour)
			resolution = "1h"
			limit = 500
		case "30d":
			startTime = now.Add(-30 * 24 * time.Hour)
			resolution = "1h"
			limit = 1000
		case "24h":
			fallthrough
		default:
			rangeParam = "24h"
			startTime = now.Add(-24 * time.Hour)
			resolution = "1m"
			limit = 500
		}
	}

	ctx := r.Context()
	var history []nodemetrics.NodeMetricRollup
	var summary *nodemetrics.NodeHistoricalSummary

	if h.nodeMetricsRepo != nil {
		q := nodemetrics.NodeHistoryQuery{
			NodeID:     nodeID,
			Resolution: resolution,
			StartTime:  startTime,
			EndTime:    endTime,
			Limit:      limit,
		}
		var err error
		history, err = h.nodeMetricsRepo.QueryHistory(ctx, q)
		if err != nil {
			h.logger.Warn("Failed to query node history", zap.String("node_id", nodeID), zap.Error(err))
		}
		// If 1h query returned empty for 7d/30d/custom, fallback to 1m resolution
		if len(history) == 0 && resolution == "1h" {
			q.Resolution = "1m"
			history, _ = h.nodeMetricsRepo.QueryHistory(ctx, q)
		}

		summary, err = h.nodeMetricsRepo.GetSummary(ctx, q)
		if err != nil {
			h.logger.Warn("Failed to get node summary", zap.String("node_id", nodeID), zap.Error(err))
		}
	}

	if history == nil {
		history = make([]nodemetrics.NodeMetricRollup, 0)
	}

	if summary == nil {
		summary = &nodemetrics.NodeHistoricalSummary{
			NodeID:      nodeID,
			NodeName:    nodeID,
			WindowStart: startTime,
			WindowEnd:   endTime,
		}
	}

	// Fetch related incidents for this node if incident repo is available
	var incidentsList []*incident.Incident
	if h.incidentRepo != nil {
		allIncs, _, err := h.incidentRepo.List(ctx, incident.Filter{
			Limit: 30,
		})
		if err == nil {
			for _, inc := range allIncs {
				if inc == nil {
					continue
				}
				if strings.EqualFold(inc.PodName, nodeID) ||
					(inc.RawData != nil && (strings.EqualFold(inc.RawData["node_id"], nodeID) || strings.EqualFold(inc.RawData["node_name"], nodeID))) {
					incidentsList = append(incidentsList, inc)
				}
			}
		}
	}
	if incidentsList == nil {
		incidentsList = make([]*incident.Incident, 0)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"node_id":    nodeID,
		"range":      rangeParam,
		"from":       startTime.Format(time.RFC3339),
		"to":         endTime.Format(time.RFC3339),
		"resolution": resolution,
		"summary":    summary,
		"history":    history,
		"incidents":  incidentsList,
	})
}

