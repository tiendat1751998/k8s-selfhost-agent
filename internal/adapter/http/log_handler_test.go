package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/logging"
	"github.com/datdt/k8sselfhost/internal/pkg/health"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
	"github.com/gorilla/websocket"
)

type fakeLoggingService struct {
	mu           sync.RWMutex
	ingested     []logging.LogEntry
	lastFilter   logging.LogFilter
	lastInterval int
	searchResult *logging.LogSearchResult
	histogramRes []logging.LogAggregationBucket
	tailChan     chan logging.LogEntry
	ingestErr    error
	queryErr     error
	histErr      error
	tailErr      error
}

func (f *fakeLoggingService) IngestBatch(ctx context.Context, entries []logging.LogEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ingestErr != nil {
		return f.ingestErr
	}
	f.ingested = append(f.ingested, entries...)
	return nil
}

func (f *fakeLoggingService) QueryLogs(ctx context.Context, filter logging.LogFilter) (*logging.LogSearchResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastFilter = filter
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	if f.searchResult != nil {
		return f.searchResult, nil
	}
	return &logging.LogSearchResult{
		Entries:    []logging.LogEntry{},
		TotalCount: 0,
		HasMore:    false,
	}, nil
}

func (f *fakeLoggingService) GetHistogram(ctx context.Context, filter logging.LogFilter, intervalSeconds int) ([]logging.LogAggregationBucket, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastFilter = filter
	f.lastInterval = intervalSeconds
	if f.histErr != nil {
		return nil, f.histErr
	}
	return f.histogramRes, nil
}

func (f *fakeLoggingService) Tail(ctx context.Context, filter logging.LogFilter) (<-chan logging.LogEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastFilter = filter
	if f.tailErr != nil {
		return nil, f.tailErr
	}
	if f.tailChan == nil {
		f.tailChan = make(chan logging.LogEntry, 10)
	}
	return f.tailChan, nil
}

func withTenantContext(ctx context.Context, tenantID, role string) context.Context {
	ctx = context.WithValue(ctx, tenancy.TenantIDKey, tenantID)
	ctx = context.WithValue(ctx, tenancy.UserRoleKey, role)
	ctx = context.WithValue(ctx, tenancy.UserIDKey, "user-1")
	return ctx
}

func TestLogHandler_Ingest_Batch(t *testing.T) {
	fake := &fakeLoggingService{}
	h := NewLogHandler(fake)

	now := time.Now().UTC()
	entries := []logging.LogEntry{
		{
			Timestamp:     now,
			ClusterID:     "cluster-1",
			Namespace:     "prod",
			PodName:       "api-pod-1",
			ContainerName: "api",
			Stream:        "stdout",
			LogLevel:      logging.LogLevelInfo,
			Message:       "batch entry 1",
		},
		{
			Timestamp:     now,
			ClusterID:     "cluster-1",
			Namespace:     "prod",
			PodName:       "api-pod-2",
			ContainerName: "api",
			Stream:        "stderr",
			LogLevel:      logging.LogLevelError,
			Message:       "batch entry 2",
		},
	}

	body, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("failed to marshal entries: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/logs/ingest", bytes.NewReader(body))
	req = req.WithContext(withTenantContext(req.Context(), "tenant-test", "operator"))
	w := httptest.NewRecorder()

	h.HandleIngest(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected status 202 Accepted, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if count, ok := resp["count"].(float64); !ok || int(count) != 2 {
		t.Errorf("expected count 2, got %v", resp["count"])
	}

	fake.mu.RLock()
	defer fake.mu.RUnlock()
	if len(fake.ingested) != 2 {
		t.Fatalf("expected 2 ingested logs, got %d", len(fake.ingested))
	}
	if fake.ingested[0].TenantID != "tenant-test" {
		t.Errorf("expected tenant_id tenant-test, got %s", fake.ingested[0].TenantID)
	}
}

func TestLogHandler_Ingest_Single(t *testing.T) {
	fake := &fakeLoggingService{}
	h := NewLogHandler(fake)

	entry := logging.LogEntry{
		Timestamp:     time.Now().UTC(),
		ClusterID:     "cluster-1",
		Namespace:     "kube-system",
		PodName:       "coredns-xyz",
		ContainerName: "coredns",
		Stream:        "stdout",
		LogLevel:      logging.LogLevelWarn,
		Message:       "dns query timeout",
	}

	body, _ := json.Marshal(entry)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/logs/ingest", bytes.NewReader(body))
	req = req.WithContext(withTenantContext(req.Context(), "tenant-single", "platform_admin"))
	w := httptest.NewRecorder()

	h.HandleIngest(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected status 202 Accepted, got %d. Body: %s", w.Code, w.Body.String())
	}

	fake.mu.RLock()
	defer fake.mu.RUnlock()
	if len(fake.ingested) != 1 {
		t.Fatalf("expected 1 ingested log, got %d", len(fake.ingested))
	}
	if fake.ingested[0].TenantID != "tenant-single" {
		t.Errorf("expected tenant-single, got %s", fake.ingested[0].TenantID)
	}
}

func TestLogHandler_Ingest_PayloadTooLarge(t *testing.T) {
	fake := &fakeLoggingService{}
	h := NewLogHandler(fake)

	hugePayload := bytes.Repeat([]byte("a"), 10<<20+1024)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/logs/ingest", bytes.NewReader(hugePayload))
	req = req.WithContext(withTenantContext(req.Context(), "tenant-large", "operator"))
	w := httptest.NewRecorder()

	h.HandleIngest(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 StatusRequestEntityTooLarge, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestLogHandler_Search_Success(t *testing.T) {
	fake := &fakeLoggingService{
		searchResult: &logging.LogSearchResult{
			Entries: []logging.LogEntry{
				{
					TenantID:  "tenant-search",
					Namespace: "default",
					PodName:   "nginx-1",
					LogLevel:  logging.LogLevelError,
					Message:   "502 Bad Gateway",
				},
			},
			TotalCount: 1,
			HasMore:    false,
		},
	}
	h := NewLogHandler(fake)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/search?query=Gateway&namespace=default&pod_name=nginx-1&log_level=error&limit=10&offset=0", nil)
	req = req.WithContext(withTenantContext(req.Context(), "tenant-search", "viewer"))
	w := httptest.NewRecorder()

	h.HandleSearch(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
	}

	var result logging.LogSearchResult
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode search result: %v", err)
	}

	if result.TotalCount != 1 || len(result.Entries) != 1 {
		t.Errorf("expected 1 result, got %d", result.TotalCount)
	}

	fake.mu.RLock()
	defer fake.mu.RUnlock()
	if fake.lastFilter.TenantID != "tenant-search" {
		t.Errorf("expected filter tenant tenant-search, got %s", fake.lastFilter.TenantID)
	}
	if fake.lastFilter.SearchText != "Gateway" {
		t.Errorf("expected search text Gateway, got %s", fake.lastFilter.SearchText)
	}
	if fake.lastFilter.LogLevel != logging.LogLevelError {
		t.Errorf("expected log level error, got %s", fake.lastFilter.LogLevel)
	}
}

func TestLogHandler_Search_TenantIsolation(t *testing.T) {
	fake := &fakeLoggingService{}
	h := NewLogHandler(fake)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/search?tenant_id=another-tenant&query=secret", nil)
	req = req.WithContext(withTenantContext(req.Context(), "legit-tenant", "viewer"))
	w := httptest.NewRecorder()

	h.HandleSearch(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	fake.mu.RLock()
	defer fake.mu.RUnlock()
	if fake.lastFilter.TenantID != "legit-tenant" {
		t.Fatalf("tenant isolation violation: expected legit-tenant, got %s", fake.lastFilter.TenantID)
	}
}

func TestLogHandler_Histogram_Success(t *testing.T) {
	bucketTime := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	fake := &fakeLoggingService{
		histogramRes: []logging.LogAggregationBucket{
			{
				TimeBucket: bucketTime,
				TotalCount: 42,
				LevelCount: map[string]uint64{"info": 40, "error": 2},
			},
		},
	}
	h := NewLogHandler(fake)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/histogram?interval_seconds=60&start_time=2026-09-10T11:00:00Z&end_time=2026-09-10T12:00:00Z", nil)
	req = req.WithContext(withTenantContext(req.Context(), "tenant-hist", "viewer"))
	w := httptest.NewRecorder()

	h.HandleHistogram(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
	}

	var buckets []logging.LogAggregationBucket
	if err := json.NewDecoder(w.Body).Decode(&buckets); err != nil {
		t.Fatalf("failed to decode histogram buckets: %v", err)
	}

	if len(buckets) != 1 || buckets[0].TotalCount != 42 {
		t.Errorf("unexpected buckets: %+v", buckets)
	}

	fake.mu.RLock()
	defer fake.mu.RUnlock()
	if fake.lastInterval != 60 {
		t.Errorf("expected interval 60, got %d", fake.lastInterval)
	}
	if fake.lastFilter.TenantID != "tenant-hist" {
		t.Errorf("expected tenant-hist, got %s", fake.lastFilter.TenantID)
	}
}

func TestLogHandler_Stream_WebSocket(t *testing.T) {
	fake := &fakeLoggingService{
		tailChan: make(chan logging.LogEntry, 5),
	}
	h := NewLogHandler(fake)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(withTenantContext(r.Context(), "tenant-stream", "viewer"))
		h.HandleStream(w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?namespace=prod"
	header := http.Header{}
	header.Set("Origin", server.URL)

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("expected 101 Switching Protocols, got %d", resp.StatusCode)
	}

	testEntry := logging.LogEntry{
		TenantID:  "tenant-stream",
		Namespace: "prod",
		PodName:   "worker-1",
		LogLevel:  logging.LogLevelInfo,
		Message:   "websocket streamed log line",
	}

	fake.tailChan <- testEntry

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var received logging.LogEntry
	if err := conn.ReadJSON(&received); err != nil {
		t.Fatalf("failed to read streamed log: %v", err)
	}

	if received.Message != testEntry.Message {
		t.Errorf("expected message %q, got %q", testEntry.Message, received.Message)
	}
}

func TestLogHandler_Router_RBAC_And_Auth(t *testing.T) {
	fake := &fakeLoggingService{}
	logH := NewLogHandler(fake)
	platform := &PlatformHandlers{
		CentralizedLogs: logH,
	}

	hh := health.NewHandler(5 * time.Second)
	router := NewRouterWithWS(hh, nil, platform)

	// 1. Unauthenticated request to /api/v1/logs/search should be rejected (401)
	unauthReq := httptest.NewRequest(http.MethodGet, "/api/v1/logs/search", nil)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, unauthReq)
	if w1.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated search, got %d", w1.Code)
	}

	// 2. Viewer token: GET /search allowed (200), POST /ingest forbidden (403)
	viewerToken, err := middleware.GenerateAccessToken("user-v", "viewer", "tenant-1")
	if err != nil {
		t.Fatalf("failed to generate viewer token: %v", err)
	}

	searchReq := httptest.NewRequest(http.MethodGet, "/api/v1/logs/search", nil)
	searchReq.Header.Set("Authorization", "Bearer "+viewerToken)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, searchReq)
	if w2.Code != http.StatusOK {
		t.Errorf("expected 200 for viewer search, got %d", w2.Code)
	}

	ingestBody := []byte(`{"message":"viewer test","pod_name":"pod-1"}`)
	viewerIngestReq := httptest.NewRequest(http.MethodPost, "/api/v1/logs/ingest", bytes.NewReader(ingestBody))
	viewerIngestReq.Header.Set("Authorization", "Bearer "+viewerToken)
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, viewerIngestReq)
	if w3.Code != http.StatusForbidden {
		t.Errorf("expected 403 for viewer ingest mutation, got %d", w3.Code)
	}

	// 3. Operator token: POST /ingest allowed (202)
	opToken, err := middleware.GenerateAccessToken("user-op", "operator", "tenant-1")
	if err != nil {
		t.Fatalf("failed to generate operator token: %v", err)
	}

	opIngestReq := httptest.NewRequest(http.MethodPost, "/api/v1/logs/ingest", bytes.NewReader(ingestBody))
	opIngestReq.Header.Set("Authorization", "Bearer "+opToken)
	w4 := httptest.NewRecorder()
	router.ServeHTTP(w4, opIngestReq)
	if w4.Code != http.StatusAccepted {
		t.Errorf("expected 202 for operator ingest mutation, got %d. Body: %s", w4.Code, w4.Body.String())
	}

	// 4. logs:write token: POST /ingest allowed (202)
	writeToken, err := middleware.GenerateAccessToken("user-w", "logs:write", "tenant-1")
	if err != nil {
		t.Fatalf("failed to generate logs:write token: %v", err)
	}

	writeIngestReq := httptest.NewRequest(http.MethodPost, "/api/v1/logs/ingest", bytes.NewReader(ingestBody))
	writeIngestReq.Header.Set("Authorization", "Bearer "+writeToken)
	w5 := httptest.NewRecorder()
	router.ServeHTTP(w5, writeIngestReq)
	if w5.Code != http.StatusAccepted {
		t.Errorf("expected 202 for logs:write ingest mutation, got %d. Body: %s", w5.Code, w5.Body.String())
	}
}
