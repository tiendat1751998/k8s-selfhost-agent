package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	mw "github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/logging"
	infraLogging "github.com/datdt/k8sselfhost/internal/infrastructure/logging"
	"github.com/datdt/k8sselfhost/internal/pkg/health"
)

// TestRouter_LogsStream_UsesLogStreamHandlerWhenCentralizedLogsPresent verifies Acceptance Criterion 1:
// When both platform.CentralizedLogs and platform.LogStream are present, /logs/stream routes
// to LogStreamHandler (broadcasting live events and 1000 history items from logAggregator).
func TestRouter_LogsStream_UsesLogStreamHandlerWhenCentralizedLogsPresent(t *testing.T) {
	aggregator := infraLogging.NewLogAggregator(100)
	aggregator.Ingest(infraLogging.LogEntry{
		Timestamp: time.Now().UTC(),
		Namespace: "prod",
		Pod:       "postgres_db",
		Container: "postgres_db",
		Service:   "postgres_db",
		Level:     "INFO",
		Message:   "database system is ready to accept connections",
	})

	logStreamHandler := NewLogStreamHandler(aggregator)
	fakeCentralized := &fakeLoggingService{}
	centralizedLogs := NewLogHandler(fakeCentralized)

	platform := &PlatformHandlers{
		CentralizedLogs: centralizedLogs,
		LogStream:       logStreamHandler,
	}

	hh := health.NewHandler(5 * time.Second)
	router := NewRouterWithWS(hh, nil, platform)

	server := httptest.NewServer(router)
	defer server.Close()

	token, err := mw.GenerateJWT("test-user", "platform_admin", "default-tenant")
	if err != nil {
		t.Fatalf("failed to generate JWT token: %v", err)
	}

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/logs/stream?token=" + token + "&service=postgres_db"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		if resp != nil {
			t.Fatalf("failed to dial websocket (status %d): %v", resp.StatusCode, err)
		}
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msgBytes, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read message from /logs/stream: %v", err)
	}

	var histEntry struct {
		Service string `json:"service"`
		Message string `json:"message"`
		Msg     string `json:"msg"`
	}
	if err := json.Unmarshal(msgBytes, &histEntry); err != nil {
		t.Fatalf("failed to unmarshal log message: %v", err)
	}

	expectedMsg := "database system is ready to accept connections"
	if histEntry.Message != expectedMsg && histEntry.Msg != expectedMsg {
		t.Fatalf("expected replayed log from LogStreamHandler %q, got msg=%q message=%q", expectedMsg, histEntry.Msg, histEntry.Message)
	}
}

// aliasTrackingFakeService supports testing alias expansion for postgres_db (matches db, postgres).
type aliasTrackingFakeService struct {
	queries []logging.LogFilter
	entries []logging.LogEntry
}

func (s *aliasTrackingFakeService) Ingest(ctx context.Context, entries []logging.LogEntry) error {
	s.entries = append(s.entries, entries...)
	return nil
}

func (s *aliasTrackingFakeService) QueryLogs(ctx context.Context, filter logging.LogFilter) (*logging.LogSearchResult, error) {
	s.queries = append(s.queries, filter)
	var matched []logging.LogEntry
	for _, e := range s.entries {
		if filter.ContainerName != "" && e.ContainerName != filter.ContainerName {
			continue
		}
		matched = append(matched, e)
	}
	return &logging.LogSearchResult{
		Entries:    matched,
		TotalCount: int64(len(matched)),
		HasMore:    false,
	}, nil
}

func (s *aliasTrackingFakeService) GetHistogram(ctx context.Context, filter logging.LogFilter, intervalSeconds int) ([]logging.LogAggregationBucket, error) {
	return nil, nil
}

func (s *aliasTrackingFakeService) TailLogs(ctx context.Context, filter logging.LogFilter) (<-chan logging.LogEntry, error) {
	ch := make(chan logging.LogEntry, 10)
	return ch, nil
}

func (s *aliasTrackingFakeService) QuerySurroundingContext(ctx context.Context, service string, timestamp time.Time, window int) ([]logging.LogEntry, error) {
	return nil, nil
}

// TestLogHandler_Search_ServiceParamAndContainerAliases verifies Acceptance Criterion 2:
// - q.Get("service") is parsed into filter.ContainerName and filter.ServiceName.
// - When searching for postgres_db, aliases "db" and "postgres" are also queried and merged.
// - Dummy "-- No entries --" messages are filtered out.
func TestLogHandler_Search_ServiceParamAndContainerAliases(t *testing.T) {
	fake := &aliasTrackingFakeService{
		entries: []logging.LogEntry{
			{Timestamp: time.Now().Add(-3 * time.Minute), ContainerName: "postgres_db", Message: "pg ready"},
			{Timestamp: time.Now().Add(-2 * time.Minute), ContainerName: "db", Message: "checkpoint complete"},
			{Timestamp: time.Now().Add(-1 * time.Minute), ContainerName: "postgres", Message: "autovacuum running"},
			{Timestamp: time.Now().Add(-30 * time.Second), ContainerName: "postgres", Message: "-- No entries --"},
			{Timestamp: time.Now().Add(-10 * time.Second), ContainerName: "redis", Message: "redis pong"},
		},
	}
	h := NewLogHandler(fake)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/search?service=postgres_db", nil)
	req = req.WithContext(withTenantContext(req.Context(), "default-tenant", "viewer"))
	w := httptest.NewRecorder()

	h.HandleSearch(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var res logging.LogSearchResult
	if err := json.NewDecoder(w.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// 1. Should have merged postgres_db and postgres entries (2 valid entries, bare "db" excluded)
	// 2. "-- No entries --" MUST be filtered out
	// 3. "db" and "redis" must NOT be included
	if len(res.Entries) != 2 {
		t.Fatalf("expected 2 entries from database aliases (postgres_db, postgres), got %d: %+v", len(res.Entries), res.Entries)
	}

	for _, e := range res.Entries {
		if e.Message == "-- No entries --" {
			t.Fatalf("dummy message '-- No entries --' was not filtered out: %+v", e)
		}
		if e.ContainerName != "postgres_db" && e.ContainerName != "postgres" {
			t.Fatalf("unexpected container %s in results: %+v", e.ContainerName, e)
		}
	}

	// Verify query filter parsed ContainerName and ServiceName
	foundService := false
	for _, q := range fake.queries {
		if q.ServiceName == "postgres_db" || q.ContainerName == "postgres_db" {
			foundService = true
			break
		}
	}
	if !foundService {
		t.Fatalf("expected at least one query with ServiceName or ContainerName == 'postgres_db'")
	}
}
func (s *aliasTrackingFakeService) TailLogsWithChan(ctx context.Context, filter logging.LogFilter, ch chan logging.LogEntry) (<-chan logging.LogEntry, error) {
	s.queries = append(s.queries, filter)
	return ch, nil
}

// TestLogHandler_Stream_ServiceParamAndContainerAliases verifies Acceptance Criterion 2 for HandleStream:
// - q.Get("service") parsed into filter.ContainerName and filter.ServiceName
// - If target is postgres_db, aliases (db, postgres) are tailed
// - Dummy "-- No entries --" messages are filtered out
func TestLogHandler_Stream_ServiceParamAndContainerAliases(t *testing.T) {
	dbCh := make(chan logging.LogEntry, 5)
	pgCh := make(chan logging.LogEntry, 5)
	pgDbCh := make(chan logging.LogEntry, 5)

	multiChService := &multiChanFakeService{
		channels: map[string]chan logging.LogEntry{
			"db":          dbCh,
			"postgres":    pgCh,
			"postgres_db": pgDbCh,
		},
	}
	h := NewLogHandler(multiChService)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(withTenantContext(r.Context(), "default-tenant", "viewer"))
		h.HandleStream(w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?service=postgres_db"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		if resp != nil {
			t.Fatalf("failed to dial websocket (status %d): %v", resp.StatusCode, err)
		}
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer conn.Close()

	// Push 1 entry to "postgres_db", 1 dummy to "postgres", 1 real entry to "postgres"
	pgDbCh <- logging.LogEntry{ContainerName: "postgres_db", Message: "pg ready"}
	pgCh <- logging.LogEntry{ContainerName: "postgres", Message: "-- No entries --"}
	pgCh <- logging.LogEntry{ContainerName: "postgres", Message: "vacuum started"}

	received := make(map[string]bool)
	for i := 0; i < 2; i++ {
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		var msg logging.LogEntry
		if err := conn.ReadJSON(&msg); err != nil {
			t.Fatalf("failed to read message %d: %v", i+1, err)
		}
		if msg.Message == "-- No entries --" {
			t.Fatalf("dummy message '-- No entries --' was streamed over websocket")
		}
		received[msg.Message] = true
	}

	if !received["pg ready"] || !received["vacuum started"] {
		t.Fatalf("expected messages 'pg ready' and 'vacuum started' from aliases, got %+v", received)
	}
}

type multiChanFakeService struct {
	mu       sync.Mutex
	channels map[string]chan logging.LogEntry
	queries  []logging.LogFilter
}

func (m *multiChanFakeService) Ingest(ctx context.Context, entries []logging.LogEntry) error { return nil }
func (m *multiChanFakeService) QueryLogs(ctx context.Context, filter logging.LogFilter) (*logging.LogSearchResult, error) {
	return nil, nil
}
func (m *multiChanFakeService) GetHistogram(ctx context.Context, filter logging.LogFilter, intervalSeconds int) ([]logging.LogAggregationBucket, error) {
	return nil, nil
}
func (m *multiChanFakeService) TailLogs(ctx context.Context, filter logging.LogFilter) (<-chan logging.LogEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queries = append(m.queries, filter)
	if ch, ok := m.channels[filter.ContainerName]; ok {
		return ch, nil
	}
	ch := make(chan logging.LogEntry, 10)
	return ch, nil
}

func (m *multiChanFakeService) QuerySurroundingContext(ctx context.Context, service string, timestamp time.Time, window int) ([]logging.LogEntry, error) {
	return nil, nil
}
