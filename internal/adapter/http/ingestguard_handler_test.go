package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	adapterHttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	"github.com/datdt/k8sselfhost/internal/domain/ingestguard"
)

type mockProtector struct {
	enqueueFunc   func(ctx context.Context, database, table string, record any) error
	flushFunc     func(ctx context.Context) error
	partsFunc     func(ctx context.Context, table string) (*ingestguard.ClickHouseTableHealth, error)
	validateFunc  func(queryDSL string) (bool, error)
}

func (m *mockProtector) EnqueueLog(ctx context.Context, database, table string, record any) error {
	if m.enqueueFunc != nil {
		return m.enqueueFunc(ctx, database, table, record)
	}
	return nil
}

func (m *mockProtector) FlushPendingBatches(ctx context.Context) error {
	if m.flushFunc != nil {
		return m.flushFunc(ctx)
	}
	return nil
}

func (m *mockProtector) GetTablePartHealth(ctx context.Context, table string) (*ingestguard.ClickHouseTableHealth, error) {
	if m.partsFunc != nil {
		return m.partsFunc(ctx, table)
	}
	h := ingestguard.NewClickHouseTableHealth("default", table, 100, false, time.Now())
	return &h, nil
}

func (m *mockProtector) ValidateSearchQuery(queryDSL string) (bool, error) {
	if m.validateFunc != nil {
		return m.validateFunc(queryDSL)
	}
	return true, nil
}

func setupIngestGuardRouter(p ingestguard.AnalyticsIngestProtector) chi.Router {
	r := chi.NewRouter()
	h := adapterHttp.NewIngestGuardHandler(p)
	h.RegisterRoutes(r)
	return r
}

func TestIngestGuardHandler_EnqueueLog_Success(t *testing.T) {
	var capturedRecord any
	p := &mockProtector{
		enqueueFunc: func(ctx context.Context, db, tbl string, record any) error {
			capturedRecord = record
			return nil
		},
	}
	router := setupIngestGuardRouter(p)

	payload := map[string]any{
		"database": "default",
		"table":    "cluster_logs",
		"record": map[string]any{
			"message": "application started",
			"level":   "info",
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/analytics/ingest/log", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusAccepted, rec.Code)
	var resp map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "enqueued", resp["status"])
	assert.NotNil(t, capturedRecord)
}

func TestIngestGuardHandler_EnqueueLog_PartsSaturation_503_RFC7807(t *testing.T) {
	p := &mockProtector{
		enqueueFunc: func(ctx context.Context, db, tbl string, record any) error {
			return ingestguard.ErrClickHousePartsSaturation
		},
	}
	router := setupIngestGuardRouter(p)

	payload := map[string]any{
		"database": "default",
		"table":    "cluster_logs",
		"record":   map[string]any{"msg": "overflow"},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/analytics/ingest/log", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/problem+json")

	var prob adapterHttp.ProblemDetails
	err := json.Unmarshal(rec.Body.Bytes(), &prob)
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, prob.Status)
	assert.Equal(t, "PARTS_SATURATION", prob.Code)
	assert.Contains(t, prob.Detail, "saturated")
}

func TestIngestGuardHandler_FlushPendingBatches_Success(t *testing.T) {
	flushed := false
	p := &mockProtector{
		flushFunc: func(ctx context.Context) error {
			flushed = true
			return nil
		},
	}
	router := setupIngestGuardRouter(p)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/analytics/ingest/flush", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, flushed)
	var resp map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "flushed", resp["status"])
}

func TestIngestGuardHandler_GetTablePartHealth_Success(t *testing.T) {
	p := &mockProtector{
		partsFunc: func(ctx context.Context, table string) (*ingestguard.ClickHouseTableHealth, error) {
			h := ingestguard.NewClickHouseTableHealth("default", table, 215, true, time.Now())
			return &h, nil
		},
	}
	router := setupIngestGuardRouter(p)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/clickhouse/parts-health?table=cluster_logs", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var health ingestguard.ClickHouseTableHealth
	err := json.Unmarshal(rec.Body.Bytes(), &health)
	require.NoError(t, err)
	assert.Equal(t, int64(215), health.ActiveParts)
	assert.Equal(t, ingestguard.HealthStatusDegraded, health.HealthStatus)
	assert.True(t, health.MergeInProgress)
}

func TestIngestGuardHandler_ValidateQuery_Success(t *testing.T) {
	p := &mockProtector{
		validateFunc: func(queryDSL string) (bool, error) {
			return true, nil
		},
	}
	router := setupIngestGuardRouter(p)

	payload := map[string]any{
		"query_dsl": `{"query": {"match_all": {}}}`,
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/analytics/opensearch/validate-query", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, true, resp["valid"])
}

func TestIngestGuardHandler_ValidateQuery_Unbounded_400_RFC7807(t *testing.T) {
	p := &mockProtector{
		validateFunc: func(queryDSL string) (bool, error) {
			return false, ingestguard.ErrUnboundedAggQuery
		},
	}
	router := setupIngestGuardRouter(p)

	payload := map[string]any{
		"query_dsl": `{"aggs": {"terms": {"size": 25000}}}`,
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/analytics/opensearch/validate-query", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/problem+json")

	var prob adapterHttp.ProblemDetails
	err := json.Unmarshal(rec.Body.Bytes(), &prob)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, prob.Status)
	assert.Equal(t, "UNBOUNDED_AGGREGATION_QUERY", prob.Code)
}
