// Package http provides HTTP routing and handlers for the platform.
package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/ingestguard"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

// IngestLogRequest defines the payload for enqueuing logs.
type IngestLogRequest struct {
	Database string `json:"database,omitempty"`
	Table    string `json:"table,omitempty"`
	Record   any    `json:"record"`
}

// ValidateQueryRequest defines the payload for DSL inspection.
type ValidateQueryRequest struct {
	QueryDSL string `json:"query_dsl"`
}

// IngestGuardHandler provides REST API endpoints for ClickHouse parts safeguards and OpenSearch query auditing.
type IngestGuardHandler struct {
	protector ingestguard.AnalyticsIngestProtector
	logger    *zap.Logger
}

// NewIngestGuardHandler constructs an IngestGuardHandler with dependencies.
func NewIngestGuardHandler(protector ingestguard.AnalyticsIngestProtector, loggers ...*zap.Logger) *IngestGuardHandler {
	var l *zap.Logger
	if len(loggers) > 0 && loggers[0] != nil {
		l = loggers[0]
	} else {
		l = logger.Get()
	}
	return &IngestGuardHandler{
		protector: protector,
		logger:    l,
	}
}

// RegisterRoutes mounts the ingest safeguard endpoints on the given chi router.
func (h *IngestGuardHandler) RegisterRoutes(r chi.Router) {
	// Root canonical routes
	r.Post("/api/v1/analytics/ingest/log", h.HandleEnqueueLog)
	r.Post("/api/v1/analytics/ingest/flush", h.HandleFlushPendingBatches)
	r.Get("/api/v1/analytics/clickhouse/parts-health", h.HandleGetTablePartHealth)
	r.Post("/api/v1/analytics/opensearch/validate-query", h.HandleValidateQuery)

	// Relative subroute aliases (when router is mounted under /api/v1/analytics or similar)
	r.Post("/ingest/log", h.HandleEnqueueLog)
	r.Post("/ingest/flush", h.HandleFlushPendingBatches)
	r.Get("/clickhouse/parts-health", h.HandleGetTablePartHealth)
	r.Post("/opensearch/validate-query", h.HandleValidateQuery)
}

// HandleEnqueueLog enqueues a log record into the adaptive batch buffer.
// Returns HTTP 202 on success, or HTTP 503 RFC 7807 when active parts reach saturation limit.
func (h *IngestGuardHandler) HandleEnqueueLog(w http.ResponseWriter, r *http.Request) {
	if h.protector == nil {
		h.writeProblem(w, http.StatusServiceUnavailable, "Service Unavailable", "ingest protector unconfigured", "SERVICE_UNAVAILABLE", r.URL.Path)
		return
	}

	bodyBytes, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 10<<20))
	if err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "failed to read request body", "READ_ERROR", r.URL.Path)
		return
	}
	trimmed := strings.TrimSpace(string(bodyBytes))
	if trimmed == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "request body cannot be empty", "EMPTY_BODY", r.URL.Path)
		return
	}

	var req IngestLogRequest
	if err := json.Unmarshal([]byte(trimmed), &req); err == nil && req.Record != nil {
		// parsed as IngestLogRequest successfully
	} else {
		// Treat the entire JSON body as the log record
		var genericRecord any
		if err := json.Unmarshal([]byte(trimmed), &genericRecord); err != nil {
			h.writeProblem(w, http.StatusBadRequest, "Bad Request", "invalid json payload", "INVALID_JSON", r.URL.Path)
			return
		}
		req = IngestLogRequest{
			Database: "default",
			Table:    "cluster_logs",
			Record:   genericRecord,
		}
	}

	if req.Database == "" {
		req.Database = "default"
	}
	if req.Table == "" {
		req.Table = "cluster_logs"
	}

	if err := h.protector.EnqueueLog(r.Context(), req.Database, req.Table, req.Record); err != nil {
		if errors.Is(err, ingestguard.ErrClickHousePartsSaturation) {
			h.writeProblem(w, http.StatusServiceUnavailable, "Service Unavailable", err.Error(), "PARTS_SATURATION", r.URL.Path)
			return
		}
		if errors.Is(err, ingestguard.ErrBufferOverflow) {
			h.writeProblem(w, http.StatusServiceUnavailable, "Service Unavailable", err.Error(), "BUFFER_OVERFLOW", r.URL.Path)
			return
		}
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", err.Error(), "INGEST_FAILED", r.URL.Path)
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status": "enqueued",
	})
}

// HandleFlushPendingBatches forces an immediate flush of all in-memory batch buffers.
func (h *IngestGuardHandler) HandleFlushPendingBatches(w http.ResponseWriter, r *http.Request) {
	if h.protector == nil {
		h.writeProblem(w, http.StatusServiceUnavailable, "Service Unavailable", "ingest protector unconfigured", "SERVICE_UNAVAILABLE", r.URL.Path)
		return
	}

	if err := h.protector.FlushPendingBatches(r.Context()); err != nil {
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", err.Error(), "FLUSH_FAILED", r.URL.Path)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "flushed",
	})
}

// HandleGetTablePartHealth returns active parts count and merge status for a ClickHouse table.
func (h *IngestGuardHandler) HandleGetTablePartHealth(w http.ResponseWriter, r *http.Request) {
	if h.protector == nil {
		h.writeProblem(w, http.StatusServiceUnavailable, "Service Unavailable", "ingest protector unconfigured", "SERVICE_UNAVAILABLE", r.URL.Path)
		return
	}

	table := r.URL.Query().Get("table")
	if table == "" {
		table = "cluster_logs"
	}

	health, err := h.protector.GetTablePartHealth(r.Context(), table)
	if err != nil {
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", err.Error(), "PARTS_QUERY_FAILED", r.URL.Path)
		return
	}

	writeJSON(w, http.StatusOK, health)
}

// HandleValidateQuery checks OpenSearch / Elasticsearch DSL to prevent JVM heap exhaustion.
// Returns HTTP 200 if safe, or HTTP 400 with RFC 7807 problem details if unsafe or malformed.
func (h *IngestGuardHandler) HandleValidateQuery(w http.ResponseWriter, r *http.Request) {
	if h.protector == nil {
		h.writeProblem(w, http.StatusServiceUnavailable, "Service Unavailable", "ingest protector unconfigured", "SERVICE_UNAVAILABLE", r.URL.Path)
		return
	}

	bodyBytes, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2<<20))
	if err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "failed to read query body", "READ_ERROR", r.URL.Path)
		return
	}
	trimmed := strings.TrimSpace(string(bodyBytes))
	if trimmed == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "query body cannot be empty", "EMPTY_QUERY", r.URL.Path)
		return
	}

	queryDSL := trimmed
	var req ValidateQueryRequest
	if err := json.Unmarshal([]byte(trimmed), &req); err == nil && req.QueryDSL != "" {
		queryDSL = req.QueryDSL
	}

	valid, valErr := h.protector.ValidateSearchQuery(queryDSL)
	if valErr != nil {
		if errors.Is(valErr, ingestguard.ErrUnboundedAggQuery) {
			h.writeProblem(w, http.StatusBadRequest, "Bad Request", valErr.Error(), "UNBOUNDED_AGGREGATION_QUERY", r.URL.Path)
			return
		}
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", valErr.Error(), "INVALID_QUERY_SYNTAX", r.URL.Path)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"valid":   valid,
		"message": "query is safe",
	})
}

func (h *IngestGuardHandler) writeProblem(w http.ResponseWriter, status int, title, detail, code, instance string) {
	problem := ProblemDetails{
		Type:     "about:blank",
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: instance,
		Code:     code,
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem)
}
