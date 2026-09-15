package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/logging"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

// LoggingService defines the operations needed for log ingestion, querying, and streaming.
type LoggingService interface {
	Ingest(ctx context.Context, entries []logging.LogEntry) error
	QueryLogs(ctx context.Context, filter logging.LogFilter) (*logging.LogSearchResult, error)
	GetHistogram(ctx context.Context, filter logging.LogFilter, intervalSeconds int) ([]logging.LogAggregationBucket, error)
	TailLogs(ctx context.Context, filter logging.LogFilter) (<-chan logging.LogEntry, error)
}

// LogEngineStatus represents the engine status metadata.
type LogEngineStatus = logging.LogEngineStatus

// LogStatusProvider provides status metadata for the centralized logging engine.
type LogStatusProvider interface {
	GetStatus(ctx context.Context) (*LogEngineStatus, error)
}

// LogHandler provides HTTP and WebSocket endpoints for centralized logging.
type LogHandler struct {
	service        LoggingService
	statusProvider any
}

// NewLogHandler constructs a new LogHandler with optional status providers.
func NewLogHandler(service LoggingService, statusProviders ...any) *LogHandler {
	h := &LogHandler{service: service}
	if len(statusProviders) > 0 && statusProviders[0] != nil {
		h.statusProvider = statusProviders[0]
	}
	return h
}

// SetStatusProvider assigns a status provider to the handler.
func (h *LogHandler) SetStatusProvider(sp any) { h.statusProvider = sp }

// Ingest provides programmatic log ingestion into the underlying logging service.
func (h *LogHandler) Ingest(ctx context.Context, entries []logging.LogEntry) error {
	if h.service == nil {
		return errors.New("logging service unavailable")
	}
	return h.service.Ingest(ctx, entries)
}

func getContainerAliases(t string) []string {
	if s := strings.ToLower(strings.TrimSpace(t)); s == "postgres_db" || s == "postgres" {
		return []string{"postgres_db", "postgres"}
	}
	if t != "" {
		return []string{t}
	}
	return nil
}

// RegisterRoutes mounts the centralized log routes.
func (h *LogHandler) RegisterRoutes(r chi.Router) {
	r.Post("/ingest", h.HandleIngest)
	r.Get("/search", h.HandleSearch)
	r.Get("/histogram", h.HandleHistogram)
	r.Get("/stream", h.HandleStream)
	r.Get("/status", h.HandleStatus)
	r.Get("/services", h.HandleGetServices)
}

// HandleStatus returns metadata on the storage engine, latency, total records, and retention.
func (h *LogHandler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	if sp, ok := h.statusProvider.(LogStatusProvider); ok {
		if st, err := sp.GetStatus(r.Context()); err == nil {
			writeJSON(w, http.StatusOK, st)
			return
		}
	}
	type simpleStatusProvider interface{ GetStatus() any }
	if ssp, ok := h.statusProvider.(simpleStatusProvider); ok {
		writeJSON(w, http.StatusOK, ssp.GetStatus())
		return
	}
	if ssp, ok := h.service.(simpleStatusProvider); ok {
		writeJSON(w, http.StatusOK, ssp.GetStatus())
		return
	}
	writeJSON(w, http.StatusOK, LogEngineStatus{
		Engine: "In-Memory RingBuffer (Fallback)", Status: "fallback",
		LatencyMS: 0.1, TotalRecords: 0, RetentionDays: 30,
	})
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
	tenantID := resolveTenant(r.Context(), r)
	if tenantID == "" {
		writeError(w, http.StatusUnauthorized, "tenant context or X-Tenant-ID header required", nil)
		return
	}
	for i := range entries {
		entries[i].TenantID = tenantID
		if entries[i].ClusterID == "" {
			entries[i].ClusterID = "default"
		}
		if err := entries[i].Validate(); err != nil {
			writeError(w, http.StatusBadRequest, "invalid log entry", err)
			return
		}
	}
	if err := h.service.Ingest(r.Context(), entries); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to ingest logs", err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "accepted", "count": len(entries)})
}

// HandleSearch queries stored logs with filtering, sorting, and pagination.
func (h *LogHandler) HandleSearch(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeError(w, http.StatusServiceUnavailable, "logging service unavailable", nil)
		return
	}
	tenantID := resolveTenant(r.Context(), r)
	if tenantID == "" {
		writeError(w, http.StatusUnauthorized, "tenant context required", nil)
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
		TenantID:      tenantID,
		ClusterID:     q.Get("cluster_id"),
		Namespace:     q.Get("namespace"),
		PodName:       firstNonEmpty(q.Get("pod_name"), q.Get("pod")),
		ContainerName: firstNonEmpty(q.Get("container_name"), q.Get("container"), q.Get("service")),
		ServiceName:   firstNonEmpty(q.Get("service"), q.Get("app"), q.Get("container_name"), q.Get("container")),
		Stream:        q.Get("stream"),
		LogLevel:      logLevel,
		SearchText:    firstNonEmpty(q.Get("query"), q.Get("search_text"), q.Get("q")),
		StartTime:     startTime,
		EndTime:       endTime,
		Limit:         parseIntParam(r, "limit", 50),
		Offset:        parseIntParam(r, "offset", 0),
	}
	filter.Sanitize()
	if nodeParam := firstNonEmpty(q.Get("node"), q.Get("node_name"), q.Get("host")); nodeParam != "" {
		if filter.Attributes == nil {
			filter.Attributes = make(map[string]string)
		}
		filter.Attributes["node"] = nodeParam
	}

	aliases := getContainerAliases(filter.ContainerName)
	if len(aliases) > 1 {
		var allEntries []logging.LogEntry
		seen := make(map[string]bool)
		for _, alias := range aliases {
			af := filter
			af.ContainerName, af.ServiceName = alias, alias
			subRes, subErr := h.service.QueryLogs(r.Context(), af)
			if subErr != nil || subRes == nil {
				continue
			}
			for _, e := range subRes.Entries {
				if strings.TrimSpace(e.Message) == "-- No entries --" {
					continue
				}
				key := fmt.Sprintf("%d|%s|%s", e.Timestamp.UnixNano(), e.ContainerName, e.Message)
				if !seen[key] {
					seen[key] = true
					allEntries = append(allEntries, e)
				}
			}
		}
		sort.SliceStable(allEntries, func(i, j int) bool { return allEntries[i].Timestamp.After(allEntries[j].Timestamp) })
		totalCount := int64(len(allEntries))
		offset, limit := filter.Offset, filter.Limit
		if limit <= 0 { limit = 50 }
		paged := []logging.LogEntry{}
		if offset < len(allEntries) {
			end := offset + limit
			if end > len(allEntries) { end = len(allEntries) }
			paged = allEntries[offset:end]
		}
		sort.SliceStable(paged, func(i, j int) bool {
			return paged[i].Timestamp.Before(paged[j].Timestamp)
		})
		writeJSON(w, http.StatusOK, &logging.LogSearchResult{
			Entries: paged, TotalCount: totalCount, HasMore: offset+len(paged) < len(allEntries),
		})
		return
	}

	res, err := h.service.QueryLogs(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query logs", err)
		return
	}
	filtered := make([]logging.LogEntry, 0, len(res.Entries))
	for _, e := range res.Entries {
		if strings.TrimSpace(e.Message) != "-- No entries --" {
			filtered = append(filtered, e)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.Before(filtered[j].Timestamp)
	})
	res.Entries = filtered
	res.TotalCount = int64(len(filtered))
	writeJSON(w, http.StatusOK, res)
}

// HandleHistogram aggregates log count buckets over time intervals.
func (h *LogHandler) HandleHistogram(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeError(w, http.StatusServiceUnavailable, "logging service unavailable", nil)
		return
	}
	tenantID := resolveTenant(r.Context(), r)
	if tenantID == "" {
		writeError(w, http.StatusUnauthorized, "tenant context required", nil)
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
	if intervalSeconds <= 0 { intervalSeconds = parseIntParam(r, "interval", 60) }
	filter := logging.LogFilter{
		TenantID:      tenantID,
		Namespace:     q.Get("namespace"),
		PodName:       firstNonEmpty(q.Get("pod_name"), q.Get("pod")),
		ContainerName: firstNonEmpty(q.Get("container_name"), q.Get("container"), q.Get("service")),
		ServiceName:   firstNonEmpty(q.Get("service"), q.Get("app"), q.Get("container_name"), q.Get("container")),
		LogLevel:      logLevel,
		SearchText:    firstNonEmpty(q.Get("query"), q.Get("search_text")),
		StartTime:     startTime,
		EndTime:       endTime,
	}
	filter.Sanitize()
	if nodeParam := firstNonEmpty(q.Get("node"), q.Get("node_name"), q.Get("host")); nodeParam != "" {
		if filter.Attributes == nil { filter.Attributes = make(map[string]string) }
		filter.Attributes["node"] = nodeParam
	}
	buckets, err := h.service.GetHistogram(r.Context(), filter, intervalSeconds)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get histogram", err)
		return
	}
	if buckets == nil { buckets = []logging.LogAggregationBucket{} }
	writeJSON(w, http.StatusOK, buckets)
}

// HandleStream provides a WebSocket live tail for matching log events with keepalive and cancellation.
func (h *LogHandler) HandleStream(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeError(w, http.StatusServiceUnavailable, "logging service unavailable", nil)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Get().Error("failed to upgrade websocket connection", zap.Error(err))
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	q := r.URL.Query()
	logLevel, _ := parseLogLevelParam(q, "log_level")
	filter := logging.LogFilter{
		TenantID:      resolveTenant(r.Context(), r),
		Namespace:     q.Get("namespace"),
		PodName:       firstNonEmpty(q.Get("pod_name"), q.Get("pod")),
		ContainerName: firstNonEmpty(q.Get("container_name"), q.Get("container"), q.Get("service")),
		ServiceName:   firstNonEmpty(q.Get("service"), q.Get("app"), q.Get("container_name"), q.Get("container")),
		LogLevel:      logLevel,
		SearchText:    firstNonEmpty(q.Get("query"), q.Get("search_text")),
	}

	aliases := getContainerAliases(filter.ContainerName)
	var ch <-chan logging.LogEntry
	if len(aliases) > 1 {
		mergedCh := make(chan logging.LogEntry, 100)
		var tailWg sync.WaitGroup
		for _, alias := range aliases {
			af := filter
			af.ContainerName, af.ServiceName = alias, alias
			ach, tErr := h.service.TailLogs(ctx, af)
			if tErr != nil { continue }
			tailWg.Add(1)
			go func(c <-chan logging.LogEntry) {
				defer tailWg.Done()
				for {
					select {
					case <-ctx.Done(): return
					case e, ok := <-c:
						if !ok { return }
						select {
						case <-ctx.Done(): return
						case mergedCh <- e:
						}
					}
				}
			}(ach)
		}
		go func() { tailWg.Wait(); close(mergedCh) }()
		ch = mergedCh
	} else {
		var tErr error
		ch, tErr = h.service.TailLogs(ctx, filter)
		if tErr != nil {
			_ = conn.WriteJSON(map[string]string{"error": "failed to start tail stream"})
			return
		}
	}

	conn.SetReadLimit(512)
	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	go func() {
		defer cancel()
		for {
			if _, _, err := conn.ReadMessage(); err != nil { return }
		}
	}()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done(): return
		case <-ticker.C:
			if err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(5*time.Second)); err != nil { return }
		case entry, ok := <-ch:
			if !ok { return }
			if strings.TrimSpace(entry.Message) == "-- No entries --" { continue }
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := conn.WriteJSON(entry); err != nil { return }
		}
	}
}

func resolveTenant(ctx context.Context, r *http.Request) string {
	if tenantID := tenancy.TenantIDFromContext(ctx); strings.TrimSpace(tenantID) != "" {
		return strings.TrimSpace(tenantID)
	}
	if r != nil {
		if hTenant := strings.TrimSpace(r.Header.Get("X-Tenant-ID")); hTenant != "" {
			return hTenant
		}
	}
	return ""
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

// HandleGetServices returns a deduplicated list of discovered services across compute hosts.
func (h *LogHandler) HandleGetServices(w http.ResponseWriter, r *http.Request) {
	type serviceLister interface {
		ListServices(ctx context.Context) ([]string, error)
	}
	var (
		services []string
		err      error
	)
	if sl, ok := h.service.(serviceLister); ok {
		services, err = sl.ListServices(r.Context())
	} else if sl, ok := h.statusProvider.(serviceLister); ok {
		services, err = sl.ListServices(r.Context())
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list services", err)
		return
	}
	if services == nil {
		services = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"services": services})
}
