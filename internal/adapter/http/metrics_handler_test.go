package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/domain/nodemetrics"
	"github.com/datdt/k8sselfhost/internal/usecase/metrics"
)

func TestOverviewHandler_GetOverview_NilSnapshot(t *testing.T) {
	collector := metrics.NewCollector(nil, nil, nil, zap.NewNop())
	handler := NewOverviewHandler(collector, zap.NewNop())

	r := chi.NewRouter()
	r.Route("/overview", handler.RegisterRoutes)

	req := httptest.NewRequest(http.MethodGet, "/overview", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp metrics.SystemOverview
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Nodes == nil || resp.Containers == nil || resp.Alerts == nil {
		t.Errorf("expected non-nil empty slices in JSON response: %+v", resp)
	}
}

func TestOverviewHandler_GetOverview_WithData(t *testing.T) {
	collector := metrics.NewCollector(nil, nil, nil, zap.NewNop())
	_, _ = collector.CollectOnce(context.Background())

	handler := NewOverviewHandler(collector, zap.NewNop())

	r := chi.NewRouter()
	r.Route("/overview", handler.RegisterRoutes)

	req := httptest.NewRequest(http.MethodGet, "/overview", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp metrics.SystemOverview
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
}

func TestOverviewHandler_GetNodes_And_Alerts_And_Containers(t *testing.T) {
	collector := metrics.NewCollector(nil, nil, nil, zap.NewNop())
	handler := NewOverviewHandler(collector, zap.NewNop())

	r := chi.NewRouter()
	r.Route("/overview", handler.RegisterRoutes)

	// Test GET /overview/nodes
	{
		req := httptest.NewRequest(http.MethodGet, "/overview/nodes", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var resp struct {
			Data []metrics.NodeMetrics `json:"data"`
		}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode nodes response: %v", err)
		}
		if resp.Data == nil {
			t.Errorf("expected non-nil nodes array")
		}
	}

	// Test GET /overview/alerts
	{
		req := httptest.NewRequest(http.MethodGet, "/overview/alerts", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var resp struct {
			Data []metrics.MetricAlert `json:"data"`
		}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode alerts response: %v", err)
		}
		if resp.Data == nil {
			t.Errorf("expected non-nil alerts array")
		}
	}

	// Test GET /overview/containers
	{
		req := httptest.NewRequest(http.MethodGet, "/overview/containers", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var resp struct {
			Data []metrics.ContainerMetrics `json:"data"`
		}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode containers response: %v", err)
		}
		if resp.Data == nil {
			t.Errorf("expected non-nil containers array")
		}
	}
}

func TestMetricsHandler_AliasConstructor(t *testing.T) {
	collector := metrics.NewCollector(nil, nil, nil, zap.NewNop())
	handler := NewMetricsHandler(collector, zap.NewNop())
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestOverviewHandler_GetTPS(t *testing.T) {
	collector := metrics.NewCollector(nil, nil, nil, zap.NewNop())
	tpsCollector := metrics.NewTPSCollector(collector, nil, zap.NewNop())
	handler := NewOverviewHandler(collector, zap.NewNop(), tpsCollector)

	r := chi.NewRouter()
	r.Route("/overview", handler.RegisterRoutes)

	// 1. With configured TPS collector
	{
		req := httptest.NewRequest(http.MethodGet, "/overview/tps", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var resp metrics.TPSSnapshot
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode TPS response: %v", err)
		}

		if resp.PerNode == nil {
			t.Errorf("expected non-nil PerNode array")
		}
	}

	// 2. With nil TPS collector (fallback)
	{
		handlerWithoutTPS := NewOverviewHandler(collector, zap.NewNop())
		r2 := chi.NewRouter()
		r2.Route("/overview", handlerWithoutTPS.RegisterRoutes)

		req := httptest.NewRequest(http.MethodGet, "/overview/tps", nil)
		w := httptest.NewRecorder()
		r2.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var resp metrics.TPSSnapshot
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode fallback TPS response: %v", err)
		}

		if resp.PerNode == nil {
			t.Errorf("expected non-nil PerNode array")
		}
	}
}

func TestOverviewHandler_FiveContainers_K8sMaterEndpoints(t *testing.T) {
	collector := metrics.NewCollector(nil, nil, nil, zap.NewNop())
	collector.SetAgentMetric("host-k8smater", &metrics.AgentMetrics{
		Hostname: "k8smater",
		Status:   "online",
	})
	overview, _ := collector.CollectOnce(context.Background())
	// Inject 5 containers
	overview.Containers = []metrics.ContainerMetrics{
		{ContainerID: "c1", ContainerName: "tiki_traefik", ServiceName: "tiki_traefik", NodeID: "host-k8smater", NodeName: "k8smater", State: "running"},
		{ContainerID: "c2", ContainerName: "nats", ServiceName: "nats", NodeID: "host-k8smater", NodeName: "k8smater", State: "running"},
		{ContainerID: "c3", ContainerName: "tiki_redis", ServiceName: "tiki_redis", NodeID: "host-k8smater", NodeName: "k8smater", State: "running"},
		{ContainerID: "c4", ContainerName: "postgres_db", ServiceName: "postgres_db", NodeID: "host-k8smater", NodeName: "k8smater", State: "running"},
		{ContainerID: "c5", ContainerName: "registry", ServiceName: "registry", NodeID: "host-k8smater", NodeName: "k8smater", State: "running"},
	}
	overview.TotalContainers = 5
	overview.RunningContainers = 5
	if len(overview.Nodes) > 0 {
		overview.Nodes[0].ContainerCount = 5
		overview.Nodes[0].RunningCount = 5
	}
	collector.SetLastSnapshot(overview)

	tpsCollector := metrics.NewTPSCollector(collector, nil, zap.NewNop())
	_, _ = tpsCollector.Collect(context.Background())

	handler := NewOverviewHandler(collector, zap.NewNop(), tpsCollector)
	r := chi.NewRouter()
	r.Route("/overview", handler.RegisterRoutes)

	// 1. Verify GET /overview
	{
		req := httptest.NewRequest(http.MethodGet, "/overview", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("GET /overview returned status %d", w.Code)
		}

		var snap metrics.SystemOverview
		if err := json.NewDecoder(w.Body).Decode(&snap); err != nil {
			t.Fatalf("failed to decode overview: %v", err)
		}

		if snap.TotalContainers != 5 {
			t.Errorf("expected TotalContainers = 5, got %d", snap.TotalContainers)
		}
		if snap.RunningContainers != 5 {
			t.Errorf("expected RunningContainers = 5, got %d", snap.RunningContainers)
		}
		if len(snap.Containers) != 5 {
			t.Errorf("expected 5 containers, got %d", len(snap.Containers))
		}
	}

	// 2. Verify GET /overview/tps
	{
		req := httptest.NewRequest(http.MethodGet, "/overview/tps", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("GET /overview/tps returned status %d", w.Code)
		}

		var snap metrics.TPSSnapshot
		if err := json.NewDecoder(w.Body).Decode(&snap); err != nil {
			t.Fatalf("failed to decode tps: %v", err)
		}

		if len(snap.Services) != 5 {
			t.Errorf("expected 5 services, got %d", len(snap.Services))
		}
	}
}

type mockNodeMetricsRepoHttp struct {
	rollups   []nodemetrics.NodeMetricRollup
	lastQuery nodemetrics.NodeHistoryQuery
}

func (m *mockNodeMetricsRepoHttp) InsertBatch(ctx context.Context, rollups []nodemetrics.NodeMetricRollup) error {
	m.rollups = append(m.rollups, rollups...)
	return nil
}

func (m *mockNodeMetricsRepoHttp) QueryHistory(ctx context.Context, q nodemetrics.NodeHistoryQuery) ([]nodemetrics.NodeMetricRollup, error) {
	m.lastQuery = q
	return m.rollups, nil
}

func (m *mockNodeMetricsRepoHttp) GetSummary(ctx context.Context, q nodemetrics.NodeHistoryQuery) (*nodemetrics.NodeHistoricalSummary, error) {
	m.lastQuery = q
	return &nodemetrics.NodeHistoricalSummary{
		NodeID:         q.NodeID,
		AvgCPUPercent:  28.5,
		PeakCPUPercent: 62.0,
		TotalSamples:   len(m.rollups),
		WindowStart:    q.StartTime,
		WindowEnd:      q.EndTime,
	}, nil
}

func (m *mockNodeMetricsRepoHttp) DownsampleAndPrune(ctx context.Context, olderThan7Days, olderThan90Days time.Time) error {
	return nil
}

func TestOverviewHandler_GetNodeHistory(t *testing.T) {
	collector := metrics.NewCollector(nil, nil, nil, zap.NewNop())
	handler := NewOverviewHandler(collector, zap.NewNop())

	mockRepo := &mockNodeMetricsRepoHttp{
		rollups: []nodemetrics.NodeMetricRollup{
			{
				NodeID:     "k8smater",
				NodeName:   "k8smater",
				CPUPercent: 28.5,
				CPUPeak:    62.0,
				Status:     "online",
				Resolution: "1m",
				RecordedAt: time.Now().UTC(),
			},
		},
	}
	handler.SetNodeMetricsRepo(mockRepo)

	r := chi.NewRouter()
	r.Route("/overview", handler.RegisterRoutes)

	req := httptest.NewRequest(http.MethodGet, "/overview/nodes/k8smater/history?range=24h", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		NodeID     string                             `json:"node_id"`
		Range      string                             `json:"range"`
		Resolution string                             `json:"resolution"`
		Summary    *nodemetrics.NodeHistoricalSummary `json:"summary"`
		History    []nodemetrics.NodeMetricRollup     `json:"history"`
		Incidents  []*incident.Incident               `json:"incidents"`
	}

	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode history response: %v", err)
	}

	if resp.NodeID != "k8smater" {
		t.Errorf("expected node_id = k8smater, got %s", resp.NodeID)
	}
	if len(resp.History) != 1 {
		t.Errorf("expected 1 history record, got %d", len(resp.History))
	}
	if resp.Summary == nil || resp.Summary.PeakCPUPercent != 62.0 {
		t.Errorf("expected valid summary with PeakCPUPercent=62.0, got %+v", resp.Summary)
	}
	if resp.Incidents == nil {
		t.Errorf("expected non-nil incidents slice, got nil")
	}
}

func TestOverviewHandler_GetNodeHistory_UUID_And_Incidents(t *testing.T) {
	collector := metrics.NewCollector(nil, nil, nil, zap.NewNop())
	handler := NewOverviewHandler(collector, zap.NewNop())

	nodeUUID := "46adc366-b85b-45e7-8233-e69e6ceeb195"
	mockRepo := &mockNodeMetricsRepoHttp{
		rollups: []nodemetrics.NodeMetricRollup{
			{
				NodeID:     nodeUUID,
				NodeName:   "k8smater",
				CPUPercent: 42.0,
				CPUPeak:    85.0,
				Status:     "online",
				Resolution: "1m",
				RecordedAt: time.Now().UTC(),
			},
		},
	}
	handler.SetNodeMetricsRepo(mockRepo)

	mockIncRepo := newMockIncidentRepo()
	testInc, _ := incident.New("default-cluster", "infrastructure", "k8smater", incident.TypeNodeNotReady, incident.SeverityCritical, "Node went offline")
	testInc.AddRawData("node_id", nodeUUID)
	testInc.AddRawData("node_name", "k8smater")
	_ = mockIncRepo.Create(context.Background(), testInc)
	handler.SetIncidentRepo(mockIncRepo)

	r := chi.NewRouter()
	r.Route("/overview", handler.RegisterRoutes)

	req := httptest.NewRequest(http.MethodGet, "/overview/nodes/"+nodeUUID+"/history?range=24h", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		NodeID     string                             `json:"node_id"`
		Range      string                             `json:"range"`
		Resolution string                             `json:"resolution"`
		Summary    *nodemetrics.NodeHistoricalSummary `json:"summary"`
		History    []nodemetrics.NodeMetricRollup     `json:"history"`
		Incidents  []*incident.Incident               `json:"incidents"`
	}

	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.NodeID != nodeUUID {
		t.Errorf("expected node_id = %s, got %s", nodeUUID, resp.NodeID)
	}
	if resp.Range != "24h" {
		t.Errorf("expected range = 24h, got %s", resp.Range)
	}
	if resp.Resolution != "1m" {
		t.Errorf("expected resolution = 1m, got %s", resp.Resolution)
	}
	if len(resp.History) != 1 {
		t.Errorf("expected 1 history record, got %d", len(resp.History))
	}
	if resp.Summary == nil {
		t.Fatal("expected non-nil summary")
	}
	if len(resp.Incidents) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(resp.Incidents))
	}
	if resp.Incidents[0].RawData["node_id"] != nodeUUID {
		t.Errorf("expected incident node_id match, got %v", resp.Incidents[0].RawData)
	}
}

func TestOverviewHandler_GetNodeHistory_NilRepo_ReturnsEmptySliceAndZeroSummary(t *testing.T) {
	collector := metrics.NewCollector(nil, nil, nil, zap.NewNop())
	handler := NewOverviewHandler(collector, zap.NewNop())
	// No repo set

	r := chi.NewRouter()
	r.Route("/overview", handler.RegisterRoutes)

	req := httptest.NewRequest(http.MethodGet, "/overview/nodes/46adc366-b85b-45e7-8233-e69e6ceeb195/history?range=1h", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		NodeID     string                             `json:"node_id"`
		Range      string                             `json:"range"`
		Resolution string                             `json:"resolution"`
		Summary    *nodemetrics.NodeHistoricalSummary `json:"summary"`
		History    []nodemetrics.NodeMetricRollup     `json:"history"`
		Incidents  []*incident.Incident               `json:"incidents"`
	}

	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.NodeID != "46adc366-b85b-45e7-8233-e69e6ceeb195" {
		t.Errorf("expected node_id = 46adc366-b85b-45e7-8233-e69e6ceeb195, got %s", resp.NodeID)
	}
	if resp.Range != "1h" {
		t.Errorf("expected range = 1h, got %s", resp.Range)
	}
	if resp.Resolution != "1m" {
		t.Errorf("expected resolution = 1m, got %s", resp.Resolution)
	}
	if resp.History == nil || len(resp.History) != 0 {
		t.Errorf("expected empty non-nil history slice, got %+v", resp.History)
	}
	if resp.Incidents == nil || len(resp.Incidents) != 0 {
		t.Errorf("expected empty non-nil incidents slice, got %+v", resp.Incidents)
	}
	if resp.Summary == nil {
		t.Fatal("expected non-nil zeroed summary, got nil")
	}
	if resp.Summary.NodeID != "46adc366-b85b-45e7-8233-e69e6ceeb195" {
		t.Errorf("expected summary node_id = 46adc366-b85b-45e7-8233-e69e6ceeb195, got %s", resp.Summary.NodeID)
	}
}

func TestOverviewHandler_GetNodeHistory_CustomRange(t *testing.T) {
	collector := metrics.NewCollector(nil, nil, nil, zap.NewNop())
	handler := NewOverviewHandler(collector, zap.NewNop())

	mockRepo := &mockNodeMetricsRepoHttp{
		rollups: []nodemetrics.NodeMetricRollup{
			{
				NodeID:     "node-1",
				NodeName:   "worker-1",
				CPUPercent: 45.0,
				Status:     "online",
				Resolution: "1m",
				RecordedAt: time.Date(2026, 8, 24, 11, 0, 0, 0, time.UTC),
			},
		},
	}
	handler.SetNodeMetricsRepo(mockRepo)

	r := chi.NewRouter()
	r.Route("/overview", handler.RegisterRoutes)

	req := httptest.NewRequest(http.MethodGet, "/overview/nodes/node-1/history?from=2026-08-24T10:00:00Z&to=2026-08-24T14:00:00Z", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		NodeID     string                             `json:"node_id"`
		Range      string                             `json:"range"`
		From       string                             `json:"from"`
		To         string                             `json:"to"`
		Resolution string                             `json:"resolution"`
		Summary    *nodemetrics.NodeHistoricalSummary `json:"summary"`
		History    []nodemetrics.NodeMetricRollup     `json:"history"`
		Incidents  []*incident.Incident               `json:"incidents"`
	}

	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.NodeID != "node-1" {
		t.Errorf("expected node_id = node-1, got %s", resp.NodeID)
	}
	if resp.Range != "custom" {
		t.Errorf("expected range = custom, got %s", resp.Range)
	}
	if resp.From != "2026-08-24T10:00:00Z" {
		t.Errorf("expected from = 2026-08-24T10:00:00Z, got %s", resp.From)
	}
	if resp.To != "2026-08-24T14:00:00Z" {
		t.Errorf("expected to = 2026-08-24T14:00:00Z, got %s", resp.To)
	}
	if resp.Resolution != "1m" {
		t.Errorf("expected resolution = 1m, got %s", resp.Resolution)
	}
	if len(resp.History) != 1 {
		t.Errorf("expected 1 history record, got %d", len(resp.History))
	}
}

func TestOverviewHandler_GetNodeHistory_CustomRange_FormatsAndResolutions(t *testing.T) {
	collector := metrics.NewCollector(nil, nil, nil, zap.NewNop())
	handler := NewOverviewHandler(collector, zap.NewNop())

	mockRepo := &mockNodeMetricsRepoHttp{}
	handler.SetNodeMetricsRepo(mockRepo)

	r := chi.NewRouter()
	r.Route("/overview", handler.RegisterRoutes)

	tests := []struct {
		name               string
		url                string
		expectedRange      string
		expectedResolution string
		expectedLimit      int
		checkTimes         func(t *testing.T, from, to string, q nodemetrics.NodeHistoryQuery)
	}{
		{
			name:               "2 hours duration -> 1m resolution",
			url:                "/overview/nodes/node-1/history?from=2026-08-24T10:00:00Z&to=2026-08-24T12:00:00Z",
			expectedRange:      "custom",
			expectedResolution: "1m",
			expectedLimit:      1500,
		},
		{
			name:               "5 days duration -> 1h resolution",
			url:                "/overview/nodes/node-1/history?from=2026-08-01T00:00:00Z&to=2026-08-06T00:00:00Z",
			expectedRange:      "custom",
			expectedResolution: "1h",
			expectedLimit:      500,
		},
		{
			name:               "15 days duration -> 1h resolution, 1000 limit",
			url:                "/overview/nodes/node-1/history?from=2026-08-01&to=2026-08-16",
			expectedRange:      "custom",
			expectedResolution: "1h",
			expectedLimit:      1000,
		},
		{
			name:               "Swapped start and end times",
			url:                "/overview/nodes/node-1/history?from=2026-08-24T14:00:00Z&to=2026-08-24T10:00:00Z",
			expectedRange:      "custom",
			expectedResolution: "1m",
			expectedLimit:      1500,
			checkTimes: func(t *testing.T, from, to string, q nodemetrics.NodeHistoryQuery) {
				if from != "2026-08-24T10:00:00Z" || to != "2026-08-24T14:00:00Z" {
					t.Errorf("expected times swapped to 10:00 and 14:00, got from=%s to=%s", from, to)
				}
			},
		},
		{
			name:               "range=custom without from defaults to 24h fallback",
			url:                "/overview/nodes/node-1/history?range=custom",
			expectedRange:      "custom",
			expectedResolution: "1m",
			expectedLimit:      1500,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d", w.Code)
			}

			var resp struct {
				Range      string `json:"range"`
				From       string `json:"from"`
				To         string `json:"to"`
				Resolution string `json:"resolution"`
			}
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if resp.Range != tc.expectedRange {
				t.Errorf("expected range %s, got %s", tc.expectedRange, resp.Range)
			}
			if resp.Resolution != tc.expectedResolution {
				t.Errorf("expected resolution %s, got %s", tc.expectedResolution, resp.Resolution)
			}
			if mockRepo.lastQuery.Limit != tc.expectedLimit {
				t.Errorf("expected query limit %d, got %d", tc.expectedLimit, mockRepo.lastQuery.Limit)
			}
			if tc.checkTimes != nil {
				tc.checkTimes(t, resp.From, resp.To, mockRepo.lastQuery)
			}
		})
	}
}

func TestOverviewHandler_GetNodeHistory_Ranges_3h_and_6h(t *testing.T) {
	collector := metrics.NewCollector(nil, nil, nil, zap.NewNop())
	handler := NewOverviewHandler(collector, zap.NewNop())

	mockRepo := &mockNodeMetricsRepoHttp{}
	handler.SetNodeMetricsRepo(mockRepo)

	r := chi.NewRouter()
	r.Route("/overview", handler.RegisterRoutes)

	tests := []struct {
		rangeParam         string
		expectedDuration   time.Duration
		expectedResolution string
		expectedLimit      int
	}{
		{
			rangeParam:         "3h",
			expectedDuration:   3 * time.Hour,
			expectedResolution: "1m",
			expectedLimit:      500,
		},
		{
			rangeParam:         "6h",
			expectedDuration:   6 * time.Hour,
			expectedResolution: "1m",
			expectedLimit:      500,
		},
	}

	for _, tc := range tests {
		t.Run("range="+tc.rangeParam, func(t *testing.T) {
			beforeReq := time.Now().UTC()
			req := httptest.NewRequest(http.MethodGet, "/overview/nodes/worker-node-1/history?range="+tc.rangeParam, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			afterReq := time.Now().UTC()

			if w.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d", w.Code)
			}

			var resp struct {
				NodeID     string `json:"node_id"`
				Range      string `json:"range"`
				From       string `json:"from"`
				To         string `json:"to"`
				Resolution string `json:"resolution"`
			}
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if resp.NodeID != "worker-node-1" {
				t.Errorf("expected node_id worker-node-1, got %s", resp.NodeID)
			}
			if resp.Range != tc.rangeParam {
				t.Errorf("expected range %s, got %s", tc.rangeParam, resp.Range)
			}
			if resp.Resolution != tc.expectedResolution {
				t.Errorf("expected resolution %s, got %s", tc.expectedResolution, resp.Resolution)
			}

			if mockRepo.lastQuery.Resolution != tc.expectedResolution {
				t.Errorf("expected repo query resolution %s, got %s", tc.expectedResolution, mockRepo.lastQuery.Resolution)
			}
			if mockRepo.lastQuery.Limit != tc.expectedLimit {
				t.Errorf("expected repo query limit %d, got %d", tc.expectedLimit, mockRepo.lastQuery.Limit)
			}

			queryDuration := mockRepo.lastQuery.EndTime.Sub(mockRepo.lastQuery.StartTime)
			if queryDuration != tc.expectedDuration {
				t.Errorf("expected query duration %v, got %v", tc.expectedDuration, queryDuration)
			}

			expectedStartMin := beforeReq.Add(-tc.expectedDuration)
			expectedStartMax := afterReq.Add(-tc.expectedDuration)
			if mockRepo.lastQuery.StartTime.Before(expectedStartMin) || mockRepo.lastQuery.StartTime.After(expectedStartMax) {
				t.Errorf("expected StartTime between %v and %v, got %v", expectedStartMin, expectedStartMax, mockRepo.lastQuery.StartTime)
			}
		})
	}
}

func TestOverviewHandler_GetNodeHistory_AllRanges_LimitAndResolution(t *testing.T) {
	collector := metrics.NewCollector(nil, nil, nil, zap.NewNop())
	handler := NewOverviewHandler(collector, zap.NewNop())

	mockRepo := &mockNodeMetricsRepoHttp{
		rollups: []nodemetrics.NodeMetricRollup{
			{
				NodeID:     "node-test",
				NodeName:   "node-test",
				CPUPercent: 20.0,
				Status:     "online",
				Resolution: "1h",
				RecordedAt: time.Now().UTC(),
			},
		},
	}
	handler.SetNodeMetricsRepo(mockRepo)

	r := chi.NewRouter()
	r.Route("/overview", handler.RegisterRoutes)

	tests := []struct {
		rangeParam         string
		expectedResolution string
		expectedLimit      int
	}{
		{rangeParam: "1h", expectedResolution: "1m", expectedLimit: 500},
		{rangeParam: "3h", expectedResolution: "1m", expectedLimit: 500},
		{rangeParam: "6h", expectedResolution: "1m", expectedLimit: 500},
		{rangeParam: "24h", expectedResolution: "1m", expectedLimit: 1500},
		{rangeParam: "", expectedResolution: "1m", expectedLimit: 1500}, // default is 24h
		{rangeParam: "7d", expectedResolution: "1h", expectedLimit: 500},
		{rangeParam: "30d", expectedResolution: "1h", expectedLimit: 1000},
	}

	for _, tc := range tests {
		url := "/overview/nodes/node-test/history"
		if tc.rangeParam != "" {
			url += "?range=" + tc.rangeParam
		}
		t.Run("range="+tc.rangeParam, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, url, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d", w.Code)
			}

			if mockRepo.lastQuery.Resolution != tc.expectedResolution {
				t.Errorf("expected resolution %s, got %s", tc.expectedResolution, mockRepo.lastQuery.Resolution)
			}
			if mockRepo.lastQuery.Limit != tc.expectedLimit {
				t.Errorf("expected limit %d, got %d", tc.expectedLimit, mockRepo.lastQuery.Limit)
			}
		})
	}
}

func TestOverviewHandler_GetNodeHistory_24h_1440Points_NoTruncation(t *testing.T) {
	collector := metrics.NewCollector(nil, nil, nil, zap.NewNop())
	handler := NewOverviewHandler(collector, zap.NewNop())

	now := time.Now().UTC()
	var mockPoints []nodemetrics.NodeMetricRollup
	for i := 1439; i >= 0; i-- {
		mockPoints = append(mockPoints, nodemetrics.NodeMetricRollup{
			NodeID:     "node-full-day",
			NodeName:   "worker-full-day",
			CPUPercent: float64(10 + (i % 50)),
			CPUPeak:    float64(20 + (i % 50)),
			Status:     "online",
			Resolution: "1m",
			RecordedAt: now.Add(-time.Duration(i) * time.Minute),
		})
	}

	mockRepo := &mockNodeMetricsRepoHttp{
		rollups: mockPoints,
	}
	handler.SetNodeMetricsRepo(mockRepo)

	r := chi.NewRouter()
	r.Route("/overview", handler.RegisterRoutes)

	req := httptest.NewRequest(http.MethodGet, "/overview/nodes/node-full-day/history?range=24h", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		NodeID     string                             `json:"node_id"`
		Range      string                             `json:"range"`
		Resolution string                             `json:"resolution"`
		Summary    *nodemetrics.NodeHistoricalSummary `json:"summary"`
		History    []nodemetrics.NodeMetricRollup     `json:"history"`
	}

	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if mockRepo.lastQuery.Limit != 1500 {
		t.Errorf("expected repo limit to be 1500 for 24h, got %d", mockRepo.lastQuery.Limit)
	}
	if len(resp.History) != 1440 {
		t.Errorf("expected 1440 samples returned without truncation, got %d", len(resp.History))
	}
}


