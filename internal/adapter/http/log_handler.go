package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/logging"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

// LoggingService defines the operations needed for log ingestion, querying, and streaming.
type LoggingService interface {
	IngestBatch(ctx context.Context, entries []logging.LogEntry) error
	QueryLogs(ctx context.Context, filter logging.LogFilter) (*logging.LogSearchResult, error)
	GetHistogram(ctx context.Context, filter logging.LogFilter, intervalSeconds int) ([]logging.LogAggregationBucket, error)
	Tail(ctx context.Context, filter logging.LogFilter) (<-chan logging.LogEntry, error)
}

// LogHandler provides HTTP and WebSocket endpoints for centralized logging.
type LogHandler struct {
	service LoggingService
}

// NewLogHandler constructs a new LogHandler.
func NewLogHandler(service LoggingService) *LogHandler {
	return &LogHandler{service: service}
}

// RegisterRoutes mounts the centralized log routes.
func (h *LogHandler) RegisterRoutes(r chi.Router) {
	r.Post("/ingest", h.HandleIngest)
	r.Get("/search", h.HandleSearch)
	r.Get("/histogram", h.HandleHistogram)
	r.Get("/stream", h.HandleStream)
}

// HandleIngest accepts a batch or single LogEntry and persists them into storage.
func (h *LogHandler) HandleIngest(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeError(w, http.StatusServiceUnavailable, "logging service unavailable", nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) || strings.Contains(err.Error(), "request body too large") {
			writeError(w, http.StatusRequestEntityTooLarge, "payload too large (max 10MB)", err)
			return
		}
		writeError(w, http.StatusBadRequest, "failed to read request body", err)
		return
	}

	trimmed := bytes.TrimSpace(bodyBytes)
	if len(trimmed) == 0 {
		writeError(w, http.StatusBadRequest, "request body cannot be empty", nil)
		return
	}

	var entries []logging.LogEntry
	if trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &entries); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json batch payload", err)
			return
		}
	} else if trimmed[0] == '{' {
		var single logging.LogEntry
		if err := json.Unmarshal(trimmed, &single); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json log payload", err)
			return
		}
		entries = []logging.LogEntry{single}
	} else {
		writeError(w, http.StatusBadRequest, "payload must be a JSON object or array", nil)
		return
	}

	tenantID := tenancy.TenantIDFromContext(r.Context())
	now := time.Now().UTC()
	for i := range entries {
		if tenantID != "" {
			entries[i].TenantID = tenantID
		}
		if entries[i].ClusterID == "" {
			entries[i].ClusterID = "default"
		}
		if entries[i].Timestamp.IsZero() {
			entries[i].Timestamp = now
		}
		if entries[i].Stream == "" {
			entries[i].Stream = "stdout"
		}
		if entries[i].LogLevel == "" {
			entries[i].LogLevel = logging.LogLevelInfo
		}
	}

	if err := h.service.IngestBatch(r.Context(), entries); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to ingest logs", err)
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"status": "accepted",
		"count":  len(entries),
	})
}

// HandleSearch queries stored logs with filtering, sorting, and pagination.
func (h *LogHandler) HandleSearch(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeError(w, http.StatusServiceUnavailable, "logging service unavailable", nil)
		return
	}

	q := r.URL.Query()
	logLevel, err := parseLogLevelParam(q, "log_level")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid log_level", err)
		return
	}

	startTime, endTime, err := parseTimeRangeParams(q)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	filter := logging.LogFilter{
		TenantID:      resolveTenant(r.Context()),
		ClusterID:     q.Get("cluster_id"),
		Namespace:     q.Get("namespace"),
		PodName:       firstNonEmpty(q.Get("pod_name"), q.Get("pod")),
		ContainerName: firstNonEmpty(q.Get("container_name"), q.Get("container")),
		Stream:        q.Get("stream"),
		LogLevel:      logLevel,
		SearchText:    firstNonEmpty(q.Get("query"), q.Get("search_text"), q.Get("q")),
		StartTime:     startTime,
		EndTime:       endTime,
		Limit:         parseIntParam(r, "limit", 50),
		Offset:        parseIntParam(r, "offset", 0),
	}
	filter.Sanitize()

	res, err := h.service.QueryLogs(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query logs", err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// HandleHistogram aggregates log count buckets over time intervals.
func (h *LogHandler) HandleHistogram(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeError(w, http.StatusServiceUnavailable, "logging service unavailable", nil)
		return
	}

	q := r.URL.Query()
	startTime, endTime, err := parseTimeRangeParams(q)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	logLevel, _ := parseLogLevelParam(q, "log_level")
	intervalSeconds := parseIntParam(r, "interval_seconds", 0)
	if intervalSeconds <= 0 {
		intervalSeconds = parseIntParam(r, "interval", 60)
	}

	filter := logging.LogFilter{
		TenantID:      resolveTenant(r.Context()),
		Namespace:     q.Get("namespace"),
		PodName:       firstNonEmpty(q.Get("pod_name"), q.Get("pod")),
		ContainerName: firstNonEmpty(q.Get("container_name"), q.Get("container")),
		LogLevel:      logLevel,
		SearchText:    firstNonEmpty(q.Get("query"), q.Get("search_text")),
		StartTime:     startTime,
		EndTime:       endTime,
	}
	filter.Sanitize()

	buckets, err := h.service.GetHistogram(r.Context(), filter, intervalSeconds)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get histogram", err)
		return
	}
	if buckets == nil {
		buckets = []logging.LogAggregationBucket{}
	}
	writeJSON(w, http.StatusOK, buckets)
}

// HandleStream provides a WebSocket live tail for matching log events.
func (h *LogHandler) HandleStream(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeError(w, http.StatusServiceUnavailable, "logging service unavailable", nil)
		return
	}

	q := r.URL.Query()
	logLevel, _ := parseLogLevelParam(q, "log_level")

	filter := logging.LogFilter{
		TenantID:      resolveTenant(r.Context()),
		Namespace:     q.Get("namespace"),
		PodName:       firstNonEmpty(q.Get("pod_name"), q.Get("pod")),
		ContainerName: firstNonEmpty(q.Get("container_name"), q.Get("container")),
		LogLevel:      logLevel,
		SearchText:    firstNonEmpty(q.Get("query"), q.Get("search_text")),
	}

	ch, err := h.service.Tail(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to start tail stream", err)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Get().Error("failed to upgrade websocket connection", zap.Error(err))
		return
	}
	defer conn.Close()

	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-clientDone:
			return
		case entry, ok := <-ch:
			if !ok {
				return
			}
			if err := conn.WriteJSON(entry); err != nil {
				return
			}
		}
	}
}

func resolveTenant(ctx context.Context) string {
	if tenantID := tenancy.TenantIDFromContext(ctx); strings.TrimSpace(tenantID) != "" {
		return tenantID
	}
	return "default-tenant"
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func parseLogLevelParam(q url.Values, key string) (logging.LogLevel, error) {
	s := firstNonEmpty(q.Get(key), q.Get("level"))
	if s != "" {
		return logging.ParseLogLevel(s)
	}
	return "", nil
}

func parseTimeRangeParams(q url.Values) (time.Time, time.Time, error) {
	var startTime, endTime time.Time
	if s := q.Get("start_time"); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("invalid start_time format (RFC3339 required)")
		}
		startTime = t
	}
	if s := q.Get("end_time"); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("invalid end_time format (RFC3339 required)")
		}
		endTime = t
	}
	return startTime, endTime, nil
}
