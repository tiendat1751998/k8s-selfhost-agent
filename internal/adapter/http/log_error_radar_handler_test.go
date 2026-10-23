package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	domainLogging "github.com/datdt/k8sselfhost/internal/domain/logging"
	"github.com/datdt/k8sselfhost/internal/infrastructure/agent"
)

type mockSummarizer struct {
	lastFilter domainLogging.LogFilter
	groups     []agent.ErrorRadarGroup
	err        error
}

func (m *mockSummarizer) SummarizeErrors(ctx context.Context, filter domainLogging.LogFilter) ([]agent.ErrorRadarGroup, error) {
	m.lastFilter = filter
	return m.groups, m.err
}

func TestHandleErrorsRadar_WithSummarizer(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	summarizer := &mockSummarizer{
		groups: []agent.ErrorRadarGroup{
			{
				Node:          "worker-01",
				Service:       "order-service",
				Level:         "error",
				Fingerprint:   "err_nil_pointer_xyz",
				SampleMessage: "runtime error: invalid memory address or nil pointer dereference",
				Count:         42,
				FirstSeen:     now.Add(-10 * time.Minute),
				LastSeen:      now,
			},
			{
				Node:          "worker-02",
				Service:       "payment-gateway",
				Level:         "warn",
				Fingerprint:   "err_conn_timeout_abc",
				SampleMessage: "connection timeout contacting upstream gateway",
				Count:         7,
				FirstSeen:     now.Add(-2 * time.Minute),
				LastSeen:      now,
			},
		},
	}

	h := NewLogHandler(nil)
	h.SetStatusProvider(summarizer)

	req := httptest.NewRequest(http.MethodGet, "/errors?node=worker-01&service=order-service&level=error&limit=25", nil)
	w := httptest.NewRecorder()

	h.HandleErrorsRadar(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Groups           []agent.ErrorRadarGroup `json:"groups"`
		TotalGroups      int                     `json:"total_groups"`
		TotalOccurrences int                     `json:"total_occurrences"`
		TookMs           float64                 `json:"took_ms"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.TotalGroups != 2 {
		t.Errorf("expected 2 total groups, got %d", resp.TotalGroups)
	}
	if resp.TotalOccurrences != 49 {
		t.Errorf("expected 49 total occurrences, got %d", resp.TotalOccurrences)
	}
	if len(resp.Groups) != 2 {
		t.Fatalf("expected 2 groups in response, got %d", len(resp.Groups))
	}
	if resp.Groups[0].Fingerprint != "err_nil_pointer_xyz" || resp.Groups[0].Count != 42 {
		t.Errorf("unexpected first group: %+v", resp.Groups[0])
	}
	if resp.TookMs < 0 {
		t.Errorf("took_ms must be non-negative: %f", resp.TookMs)
	}

	// Verify filter was mapped properly
	if summarizer.lastFilter.ServiceName != "order-service" {
		t.Errorf("expected service filter order-service, got %s", summarizer.lastFilter.ServiceName)
	}
	if summarizer.lastFilter.Limit != 25 {
		t.Errorf("expected limit filter 25, got %d", summarizer.lastFilter.Limit)
	}
	if summarizer.lastFilter.Attributes["node"] != "worker-01" {
		t.Errorf("expected node attribute worker-01, got %v", summarizer.lastFilter.Attributes)
	}
}

func TestHandleErrorsRadar_FallbackLoggingService(t *testing.T) {
	now := time.Now().UTC()
	fake := &fakeLoggingService{
		searchResult: &domainLogging.LogSearchResult{
			Entries: []domainLogging.LogEntry{
				{
					ContainerName: "api-gateway",
					LogLevel:      domainLogging.LogLevelError,
					Message:       "dial tcp 10.0.0.1:5432: i/o timeout",
					Timestamp:     now.Add(-5 * time.Minute),
					Attributes:    map[string]string{"node_name": "node-alpha"},
				},
				{
					ContainerName: "api-gateway",
					LogLevel:      domainLogging.LogLevelError,
					Message:       "dial tcp 10.0.0.1:5432: i/o timeout",
					Timestamp:     now.Add(-1 * time.Minute),
					Attributes:    map[string]string{"node_name": "node-alpha"},
				},
				{
					ContainerName: "system.slice",
					LogLevel:      domainLogging.LogLevelError,
					Message:       "noise slice log",
					Timestamp:     now,
				},
				{
					ContainerName: "session-123.scope",
					LogLevel:      domainLogging.LogLevelError,
					Message:       "noise scope log",
					Timestamp:     now,
				},
			},
		},
	}

	h := NewLogHandler(fake)
	req := httptest.NewRequest(http.MethodGet, "/errors", nil)
	w := httptest.NewRecorder()

	h.HandleErrorsRadar(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Groups           []agent.ErrorRadarGroup `json:"groups"`
		TotalGroups      int                     `json:"total_groups"`
		TotalOccurrences int                     `json:"total_occurrences"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// system.slice and session-123.scope should be discarded
	if resp.TotalGroups != 1 {
		t.Fatalf("expected 1 group (noise filtered), got %d: %+v", resp.TotalGroups, resp.Groups)
	}
	if resp.TotalOccurrences != 2 {
		t.Errorf("expected 2 occurrences of timeout error, got %d", resp.TotalOccurrences)
	}
	if resp.Groups[0].Node != "node-alpha" || resp.Groups[0].Service != "api-gateway" {
		t.Errorf("unexpected group properties: %+v", resp.Groups[0])
	}
}

func TestHandleErrorsRadar_RouterMount(t *testing.T) {
	h := NewLogHandler(&fakeLoggingService{})
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/errors", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected router GET /errors to return 200, got %d", w.Code)
	}
}

func TestHandleErrorsRadar_Error(t *testing.T) {
	summarizer := &mockSummarizer{
		err: errors.New("cluster connection lost"),
	}
	h := NewLogHandler(nil)
	h.SetStatusProvider(summarizer)

	req := httptest.NewRequest(http.MethodGet, "/errors", nil)
	w := httptest.NewRecorder()
	h.HandleErrorsRadar(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected HTTP 500 on error, got %d", w.Code)
	}
}
