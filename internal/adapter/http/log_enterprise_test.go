package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	"github.com/datdt/k8sselfhost/internal/domain/logging"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

type mockEnterpriseLoggingService struct {
	lastFilter logging.LogFilter
	logs       []logging.LogEntry
}

func (m *mockEnterpriseLoggingService) Ingest(ctx context.Context, entries []logging.LogEntry) error {
	m.logs = append(m.logs, entries...)
	return nil
}

func (m *mockEnterpriseLoggingService) QueryLogs(ctx context.Context, filter logging.LogFilter) (*logging.LogSearchResult, error) {
	m.lastFilter = filter
	var matched []logging.LogEntry
	for _, e := range m.logs {
		if filter.TraceID != "" && e.TraceID != filter.TraceID {
			continue
		}
		matched = append(matched, e)
	}
	return &logging.LogSearchResult{
		Entries:    matched,
		TotalCount: int64(len(matched)),
	}, nil
}

func (m *mockEnterpriseLoggingService) GetHistogram(ctx context.Context, filter logging.LogFilter, intervalSeconds int) ([]logging.LogAggregationBucket, error) {
	return nil, nil
}

func (m *mockEnterpriseLoggingService) TailLogs(ctx context.Context, filter logging.LogFilter) (<-chan logging.LogEntry, error) {
	ch := make(chan logging.LogEntry)
	close(ch)
	return ch, nil
}

func (m *mockEnterpriseLoggingService) QuerySurroundingContext(ctx context.Context, service string, timestamp time.Time, window int) ([]logging.LogEntry, error) {
	var matched []logging.LogEntry
	for _, e := range m.logs {
		if service == "" || e.ContainerName == service || e.PodName == service {
			matched = append(matched, e)
		}
	}
	return matched, nil
}

func TestLogHandler_HandleSurroundingContext(t *testing.T) {
	svc := &mockEnterpriseLoggingService{
		logs: []logging.LogEntry{
			{Timestamp: time.Now().Add(-10 * time.Second), PodName: "order-svc", Message: "before log"},
			{Timestamp: time.Now(), PodName: "order-svc", Message: "at target log"},
			{Timestamp: time.Now().Add(10 * time.Second), PodName: "order-svc", Message: "after log"},
		},
	}
	handler := adapthttp.NewLogHandler(svc)

	r := chi.NewRouter()
	r.Get("/api/v1/logs/context", handler.HandleSurroundingContext)

	// 1. Missing timestamp -> 400
	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/context?service=order-svc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)

	// 2. Valid request -> 200 with entries
	req = httptest.NewRequest(http.MethodGet, "/api/v1/logs/context?service=order-svc&timestamp=2026-09-16T08:00:00Z&window=50", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Entries    []logging.LogEntry `json:"entries"`
		TotalCount int                `json:"total_count"`
		Window     int                `json:"window"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Len(t, resp.Entries, 3)
	require.Equal(t, 50, resp.Window)
}

func TestLogHandler_HandleTraceQuery(t *testing.T) {
	svc := &mockEnterpriseLoggingService{
		logs: []logging.LogEntry{
			{Timestamp: time.Now().Add(-5 * time.Second), TraceID: "trace-xyz-99", Message: "step 1"},
			{Timestamp: time.Now(), TraceID: "trace-xyz-99", Message: "step 2"},
			{Timestamp: time.Now(), TraceID: "other-trace", Message: "unrelated"},
		},
	}
	handler := adapthttp.NewLogHandler(svc)

	r := chi.NewRouter()
	r.Get("/api/v1/logs/trace/{traceId}", handler.HandleTraceQuery)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/trace/trace-xyz-99", nil)
	ctx := tenancy.WithTenantID(req.Context(), "default-tenant")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		TraceID    string             `json:"trace_id"`
		Entries    []logging.LogEntry `json:"entries"`
		TotalCount int                `json:"total_count"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Equal(t, "trace-xyz-99", resp.TraceID)
	require.Len(t, resp.Entries, 2)
}

func TestLogHandler_DefaultLimitIs500(t *testing.T) {
	svc := &mockEnterpriseLoggingService{}
	handler := adapthttp.NewLogHandler(svc)

	r := chi.NewRouter()
	r.Get("/api/v1/logs/search", handler.HandleSearch)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/search", nil)
	ctx := tenancy.WithTenantID(req.Context(), "default-tenant")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	require.Equal(t, 500, svc.lastFilter.Limit, "default limit must be 500")
}
