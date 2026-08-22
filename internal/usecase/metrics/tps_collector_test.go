package metrics

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	domainLB "github.com/datdt/k8sselfhost/internal/domain/loadbalancer"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type mockMetricsProvider struct {
	snapshot     *SystemOverview
	agentMetrics map[string]*AgentMetrics
}

func (m *mockMetricsProvider) GetLastSnapshot() *SystemOverview {
	return m.snapshot
}

func (m *mockMetricsProvider) GetAgentMetrics() map[string]*AgentMetrics {
	return m.agentMetrics
}

type mockDBRow struct {
	values []any
	err    error
}

func (r *mockDBRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return errors.New("mismatched scan destination length")
	}
	for i, d := range dest {
		switch ptr := d.(type) {
		case *int64:
			*ptr = r.values[i].(int64)
		case *int:
			*ptr = r.values[i].(int)
		case *float64:
			*ptr = r.values[i].(float64)
		default:
			return errors.New("unsupported scan type")
		}
	}
	return nil
}

type mockDBPool struct {
	rowFn func(ctx context.Context, sql string, args ...any) pgx.Row
}

func (m *mockDBPool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.rowFn != nil {
		return m.rowFn(ctx, sql, args...)
	}
	return &mockDBRow{err: errors.New("no row configured")}
}

func TestDeriveURLs(t *testing.T) {
	// Test Traefik URL derivation
	traefikTests := []struct {
		input    string
		expected string
	}{
		{"tcp://10.10.10.133:2375", "http://10.10.10.133:8080"},
		{"unix:///var/run/docker.sock", "http://localhost:8080"},
		{"", "http://localhost:8080"},
		{"http://10.10.10.133:8080", "http://10.10.10.133:8080"},
		{"https://proxy.example.com:8443", "https://proxy.example.com:8443"},
		{"tcp://0.0.0.0:2375", "http://localhost:8080"},
	}

	for _, tt := range traefikTests {
		got := DeriveTraefikURL(tt.input)
		if got != tt.expected {
			t.Errorf("DeriveTraefikURL(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}

	// Test NATS Monitor URL derivation
	natsTests := []struct {
		input    string
		expected string
	}{
		{"nats://10.10.10.133:4222", "http://10.10.10.133:8222/varz"},
		{"nats://localhost:4222", "http://localhost:8222/varz"},
		{"", "http://localhost:8222/varz"},
		{"http://10.10.10.133:8222/varz", "http://10.10.10.133:8222/varz"},
		{"10.10.10.133:4222", "http://10.10.10.133:8222/varz"},
	}

	for _, tt := range natsTests {
		got := DeriveNATSMonitorURL(tt.input)
		if got != tt.expected {
			t.Errorf("DeriveNATSMonitorURL(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestTPSCollector_Collect_AllSources(t *testing.T) {
	// 1. Mock Traefik API server
	traefikSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/overview" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"http": map[string]interface{}{
					"routers": map[string]interface{}{
						"total":    7,
						"warnings": 0,
						"errors":   1,
					},
					"services": map[string]interface{}{
						"total":    6,
						"warnings": 0,
						"errors":   0,
					},
					"middlewares": map[string]interface{}{
						"total": 2,
					},
				},
			})
			return
		}
		if r.URL.Path == "/api/http/services" {
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"name":   "gateway@file",
					"status": "enabled",
					"serverStatus": map[string]string{
						"http://backend:3579": "UP",
					},
				},
				{
					"name":   "web@file",
					"status": "enabled",
					"serverStatus": map[string]string{
						"http://web:3000": "UP",
					},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer traefikSrv.Close()

	// 2. Mock NATS varz server
	natsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/varz" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"in_msgs":     int64(10000),
				"out_msgs":    int64(20000),
				"in_bytes":    int64(5000000),
				"out_bytes":   int64(10000000),
				"in_rate":     150.5,
				"out_rate":    300.25,
				"connections": 42,
				"jetstream": map[string]interface{}{
					"stats": map[string]interface{}{
						"streams": 5,
					},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer natsSrv.Close()

	// 3. Mock Postgres DB pool
	dbCallCount := 0
	mockDB := &mockDBPool{
		rowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			dbCallCount++
			if dbCallCount == 1 {
				return &mockDBRow{
					values: []any{
						int64(1000), // xact_commit
						int64(50),   // xact_rollback
						int64(5000), // tup_returned
						int64(3000), // tup_fetched
						int64(200),  // tup_inserted
						int64(150),  // tup_updated
						int64(20),   // tup_deleted
						int64(12),   // numbackends
						int64(100),  // blks_read
						int64(9900), // blks_hit
					},
				}
			}
			return &mockDBRow{
				values: []any{
					int64(1200),  // +200 commit
					int64(55),    // +5 rollback
					int64(6000),  // +1000 returned
					int64(3500),  // +500 fetched
					int64(250),   // +50 inserted
					int64(180),   // +30 updated
					int64(25),    // +5 deleted
					int64(15),    // numbackends
					int64(120),   // blks_read
					int64(11880), // blks_hit
				},
			}
		},
	}

	// 4. Mock Metrics Provider (Agent + Container)
	provider := &mockMetricsProvider{
		agentMetrics: map[string]*AgentMetrics{
			"host-1": {
				Hostname:   "worker-node-1",
				NetRxRate:  512000,
				NetTxRate:  256000,
				Processes:  25,
				Status:     "online",
				TopProcesses: []ProcessMetric{
					{
						PID:              101,
						Name:             "postgres",
						ReadBytesPerSec:  1024,
						WriteBytesPerSec: 2048,
					},
				},
			},
			"host-2": {
				Hostname:   "worker-node-2",
				NetRxRate:  1024000,
				NetTxRate:  512000,
				Processes:  30,
				Status:     "online",
				TopProcesses: []ProcessMetric{
					{
						PID:              202,
						Name:             "nginx",
						ReadBytesPerSec:  4096,
						WriteBytesPerSec: 8192,
					},
				},
			},
		},
		snapshot: &SystemOverview{
			Containers: []ContainerMetrics{
				{
					ContainerID:   "c1",
					ContainerName: "tiki_cart.1.uwafkc3p667y4fvhdw2zw6es9",
					ServiceName:   "tiki_cart",
					NodeID:        "host-1",
					State:         "running",
					CPUPercent:    2.5,
					MemoryUsed:    134217728, // 128 MB
					MemoryLimit:   536870912, // 512 MB
					NetworkRx:     100000,
					NetworkTx:     50000,
				},
				{
					ContainerID:   "c2",
					ContainerName: "postgres_db",
					ServiceName:   "postgres_db",
					NodeID:        "host-1",
					State:         "running",
					CPUPercent:    5.0,
					MemoryUsed:    268435456, // 256 MB
					MemoryLimit:   1073741824, // 1 GB
					NetworkRx:     200000,
					NetworkTx:     100000,
				},
			},
		},
	}

	reqCount := int64(500)
	collector := NewTPSCollector(
		provider,
		mockDB,
		zap.NewNop(),
		WithTraefikURL(traefikSrv.URL),
		WithNATSMonitorURL(natsSrv.URL+"/varz"),
		WithTPSRequestCountFn(func() int64 { return reqCount }),
		WithTPSHTTPClient(traefikSrv.Client()),
	)

	// First collection (initializes baseline stats)
	snap1, err := collector.Collect(context.Background())
	if err != nil {
		t.Fatalf("first Collect failed: %v", err)
	}

	if snap1.Network.TotalRxBytesPerSec != 1536000 {
		t.Errorf("expected TotalRxBytesPerSec = 1536000, got %d", snap1.Network.TotalRxBytesPerSec)
	}
	if snap1.Network.TotalTxBytesPerSec != 768000 {
		t.Errorf("expected TotalTxBytesPerSec = 768000, got %d", snap1.Network.TotalTxBytesPerSec)
	}
	if len(snap1.PerNode) != 2 {
		t.Errorf("expected 2 PerNode entries, got %d", len(snap1.PerNode))
	}
	if len(snap1.Services) != 2 {
		t.Fatalf("expected 2 Services entries, got %d", len(snap1.Services))
	}
	// Sorted by CPU descending: postgres_db (5.0%) first, tiki_cart (2.5%) second
	if snap1.Services[0].ServiceName != "postgres_db" {
		t.Errorf("expected first service = postgres_db, got %s", snap1.Services[0].ServiceName)
	}
	if snap1.Services[0].NodeID != "host-1" {
		t.Errorf("expected postgres_db NodeID = host-1, got %s", snap1.Services[0].NodeID)
	}
	if snap1.Services[0].NodeName != "worker-node-1" {
		t.Errorf("expected postgres_db NodeName = worker-node-1, got %s", snap1.Services[0].NodeName)
	}
	if snap1.Services[0].Status != "healthy" {
		t.Errorf("expected postgres_db status = healthy, got %s", snap1.Services[0].Status)
	}
	if snap1.Services[0].MemoryUsedMB != 256.0 {
		t.Errorf("expected postgres_db MemoryUsedMB = 256.0, got %f", snap1.Services[0].MemoryUsedMB)
	}
	if snap1.Services[1].NodeID != "host-1" {
		t.Errorf("expected tiki_cart NodeID = host-1, got %s", snap1.Services[1].NodeID)
	}
	if snap1.Database.ActiveConnections != 12 {
		t.Errorf("expected DB ActiveConnections = 12, got %d", snap1.Database.ActiveConnections)
	}
	if snap1.Database.CacheHitRatio != 0.99 {
		t.Errorf("expected DB CacheHitRatio = 0.99, got %f", snap1.Database.CacheHitRatio)
	}
	if snap1.HTTP.ActiveConnections != 2 {
		t.Errorf("expected HTTP ActiveConnections = 2, got %d", snap1.HTTP.ActiveConnections)
	}
	if snap1.HTTP.QueuedRequests != 0 {
		t.Errorf("expected HTTP QueuedRequests = 0, got %d", snap1.HTTP.QueuedRequests)
	}
	if snap1.Messaging.InMsgsPerSec != 150.5 {
		t.Errorf("expected NATS InMsgsPerSec = 150.5, got %f", snap1.Messaging.InMsgsPerSec)
	}
	if snap1.Messaging.OutMsgsPerSec != 300.25 {
		t.Errorf("expected NATS OutMsgsPerSec = 300.25, got %f", snap1.Messaging.OutMsgsPerSec)
	}
	if snap1.Messaging.Connections != 42 {
		t.Errorf("expected NATS Connections = 42, got %d", snap1.Messaging.Connections)
	}
	if snap1.Messaging.Streams != 5 {
		t.Errorf("expected NATS Streams = 5, got %d", snap1.Messaging.Streams)
	}

	// Advance time and perform second collection
	time.Sleep(50 * time.Millisecond)
	reqCount = 600

	snap2, err := collector.Collect(context.Background())
	if err != nil {
		t.Fatalf("second Collect failed: %v", err)
	}

	if snap2.Database.TransactionsPerSec <= 0 {
		t.Errorf("expected positive DB TransactionsPerSec on second collect, got %f", snap2.Database.TransactionsPerSec)
	}
	if snap2.Database.ReadsPerSec <= 0 {
		t.Errorf("expected positive DB ReadsPerSec on second collect, got %f", snap2.Database.ReadsPerSec)
	}
	if snap2.Database.WritesPerSec <= 0 {
		t.Errorf("expected positive DB WritesPerSec on second collect, got %f", snap2.Database.WritesPerSec)
	}
	if snap2.HTTP.RequestsPerSec <= 0 {
		t.Errorf("expected positive HTTP RequestsPerSec on second collect, got %f", snap2.HTTP.RequestsPerSec)
	}

	cachedSnap := collector.GetLastSnapshot()
	if cachedSnap == nil {
		t.Fatal("expected non-nil cached snapshot from GetLastSnapshot()")
	}
}

func TestTPSCollector_GracefulDegradation(t *testing.T) {
	// All services unreachable or nil
	collector := NewTPSCollector(
		nil,
		nil,
		zap.NewNop(),
		WithTraefikURL("http://127.0.0.1:59999"),
		WithNATSMonitorURL("http://127.0.0.1:59998/varz"),
	)

	snap, err := collector.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect should succeed gracefully when dependencies are down, got error: %v", err)
	}

	if snap == nil {
		t.Fatal("expected non-nil snapshot on graceful degradation")
	}

	if snap.PerNode == nil {
		t.Errorf("expected non-nil PerNode slice")
	}
	if snap.Services == nil {
		t.Errorf("expected non-nil Services slice")
	}
	if snap.Database.ActiveConnections != 0 {
		t.Errorf("expected 0 active DB connections, got %d", snap.Database.ActiveConnections)
	}
	if snap.Messaging.Connections != 0 {
		t.Errorf("expected 0 NATS connections, got %d", snap.Messaging.Connections)
	}
	if snap.HTTP.RequestsPerSec != 0 {
		t.Errorf("expected 0 HTTP requests/sec, got %f", snap.HTTP.RequestsPerSec)
	}
}

func TestExtractServiceName(t *testing.T) {
	tests := []struct {
		name     string
		input    ContainerMetrics
		expected string
	}{
		{
			name: "explicit service_name",
			input: ContainerMetrics{
				ServiceName:   "custom_svc",
				ContainerName: "custom_cnt",
			},
			expected: "custom_svc",
		},
		{
			name: "swarm label",
			input: ContainerMetrics{
				ContainerName: "tiki_cart.1.uwafkc3p667y4fvhdw2zw6es9",
				Labels: map[string]string{
					"com.docker.swarm.service.name": "tiki_cart",
				},
			},
			expected: "tiki_cart",
		},
		{
			name: "compose label",
			input: ContainerMetrics{
				ContainerName: "datdt_nats_1",
				Labels: map[string]string{
					"com.docker.compose.service": "nats",
				},
			},
			expected: "nats",
		},
		{
			name: "swarm name pattern fallback",
			input: ContainerMetrics{
				ContainerName: "/tiki_redis.1.xyz12345",
			},
			expected: "tiki_redis",
		},
		{
			name: "standalone container name",
			input: ContainerMetrics{
				ContainerName: "/postgres_db",
			},
			expected: "postgres_db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractServiceName(tt.input)
			if got != tt.expected {
				t.Errorf("extractServiceName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestTPSCollector_Start_Stop(t *testing.T) {
	collector := NewTPSCollector(
		nil,
		nil,
		zap.NewNop(),
		WithTPSInterval(20*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go collector.Start(ctx)
	time.Sleep(50 * time.Millisecond)

	snap := collector.GetLastSnapshot()
	if snap == nil {
		t.Fatal("expected non-nil snapshot after background start")
	}

	collector.Stop()
}

func TestTPSCollector_NodeMapping(t *testing.T) {
	provider := &mockMetricsProvider{
		agentMetrics: map[string]*AgentMetrics{
			"host-uuid-k8smater": {
				Hostname: "k8smater",
			},
			"host-uuid-worker1": {
				Hostname: "worker1",
			},
		},
		snapshot: &SystemOverview{
			Nodes: []NodeMetrics{
				{
					NodeID:   "host-uuid-k8smater",
					NodeName: "k8smater",
				},
				{
					NodeID:   "host-uuid-worker1",
					NodeName: "worker1",
				},
			},
			Containers: []ContainerMetrics{
				{
					ContainerID:   "c1",
					ContainerName: "tiki_redis.1.123",
					ServiceName:   "tiki_redis",
					NodeID:        "host-uuid-k8smater",
					NodeName:      "k8smater",
					State:         "running",
					CPUPercent:    2.5,
					MemoryUsed:    1024 * 1024 * 50,
					MemoryLimit:   1024 * 1024 * 500,
				},
				{
					ContainerID:   "c2",
					ContainerName: "nats",
					ServiceName:   "nats",
					NodeID:        "",
					NodeName:      "",
					Labels: map[string]string{
						"com.docker.swarm.node.id": "swarm-node-worker1",
					},
					State:       "running",
					CPUPercent:  1.0,
					MemoryUsed:  1024 * 1024 * 20,
					MemoryLimit: 1024 * 1024 * 200,
				},
			},
		},
	}

	collector := NewTPSCollector(provider, nil, zap.NewNop())
	snap, err := collector.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	if len(snap.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(snap.Services))
	}

	svcMap := make(map[string]ServiceTPS)
	for _, s := range snap.Services {
		svcMap[s.ServiceName] = s
	}

	redis, ok := svcMap["tiki_redis"]
	if !ok {
		t.Fatal("expected tiki_redis service in snapshot")
	}
	if redis.NodeID != "host-uuid-k8smater" || redis.NodeName != "k8smater" {
		t.Errorf("tiki_redis node mismatch: NodeID=%q NodeName=%q, want host-uuid-k8smater/k8smater", redis.NodeID, redis.NodeName)
	}
}

type mockLBProvider struct {
	stats    []domainLB.ServiceRequestStats
	aggStats *domainLB.AggregateStats
	err      error
}

func (m *mockLBProvider) GetServiceStats(ctx context.Context) ([]domainLB.ServiceRequestStats, error) {
	return m.stats, m.err
}

func (m *mockLBProvider) GetAggregateStats(ctx context.Context) (*domainLB.AggregateStats, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.aggStats, nil
}

func (m *mockLBProvider) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *mockLBProvider) Name() string {
	return "mock-traefik"
}

func TestTPSCollector_WithLoadBalancerProvider(t *testing.T) {
	provider := &mockMetricsProvider{
		snapshot: &SystemOverview{
			Containers: []ContainerMetrics{
				{
					ContainerID:   "c-gateway",
					ContainerName: "tiki_gateway",
					ServiceName:   "tiki_gateway",
					State:         "running",
					CPUPercent:    3.2,
					MemoryUsed:    1024 * 1024 * 64,
					MemoryLimit:   1024 * 1024 * 512,
					NetworkRx:     100000,
					NetworkTx:     50000,
				},
				{
					ContainerID:   "c-web",
					ContainerName: "tiki_web",
					ServiceName:   "tiki_web",
					State:         "running",
					CPUPercent:    1.5,
					MemoryUsed:    1024 * 1024 * 32,
					MemoryLimit:   1024 * 1024 * 256,
					NetworkRx:     20000,
					NetworkTx:     10000,
				},
			},
		},
	}

	lbMock := &mockLBProvider{
		stats: []domainLB.ServiceRequestStats{
			{
				ServiceName:    "gateway@file",
				TotalRequests:  1500,
				RequestsPerSec: 42.5,
				Status2xx:      1450,
				Status5xx:      10,
				ErrorRate:      0.67,
				AvgLatencyMs:   12.4,
			},
			{
				ServiceName:    "web@file",
				TotalRequests:  800,
				RequestsPerSec: 18.2,
				Status2xx:      790,
				Status4xx:      10,
				ErrorRate:      1.25,
				AvgLatencyMs:   5.1,
			},
		},
		aggStats: &domainLB.AggregateStats{
			TotalRequests:       2300,
			TotalRequestsPerSec: 60.7,
			ActiveConnections:   20,
			ErrorRate:           0.87,
			AvgLatencyMs:        8.5,
		},
	}

	collector := NewTPSCollector(
		provider,
		nil,
		zap.NewNop(),
		WithLoadBalancerProvider(lbMock),
		WithTPSRequestCountFn(func() int64 { return 100 }),
	)

	snap, err := collector.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	// HTTP TPS should be overridden by LB AggregateStats
	if snap.HTTP.RequestsPerSec != 60.7 {
		t.Errorf("expected HTTP RequestsPerSec = 60.7, got %f", snap.HTTP.RequestsPerSec)
	}
	if snap.HTTP.ActiveConnections != 20 {
		t.Errorf("expected HTTP ActiveConnections = 20, got %d", snap.HTTP.ActiveConnections)
	}
	if snap.HTTP.ErrorRate != 0.87 {
		t.Errorf("expected HTTP ErrorRate = 0.87, got %f", snap.HTTP.ErrorRate)
	}
	if snap.HTTP.AvgLatencyMs != 8.5 {
		t.Errorf("expected HTTP AvgLatencyMs = 8.5, got %f", snap.HTTP.AvgLatencyMs)
	}
	if snap.HTTP.TotalRequests != 2300 {
		t.Errorf("expected HTTP TotalRequests = 2300, got %d", snap.HTTP.TotalRequests)
	}

	if len(snap.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(snap.Services))
	}

	svcMap := make(map[string]ServiceTPS)
	for _, s := range snap.Services {
		svcMap[s.ServiceName] = s
	}

	gw, ok := svcMap["tiki_gateway"]
	if !ok {
		t.Fatal("expected tiki_gateway in services")
	}
	if gw.RequestsPerSec != 42.5 {
		t.Errorf("expected gateway RequestsPerSec = 42.5, got %f", gw.RequestsPerSec)
	}
	if gw.ErrorRate != 0.67 {
		t.Errorf("expected gateway ErrorRate = 0.67, got %f", gw.ErrorRate)
	}
	if gw.AvgLatencyMs != 12.4 {
		t.Errorf("expected gateway AvgLatencyMs = 12.4, got %f", gw.AvgLatencyMs)
	}

	web, ok := svcMap["tiki_web"]
	if !ok {
		t.Fatal("expected tiki_web in services")
	}
	if web.RequestsPerSec != 18.2 {
		t.Errorf("expected web RequestsPerSec = 18.2, got %f", web.RequestsPerSec)
	}
}

func TestTPSCollector_FallbackWhenLBProviderFails(t *testing.T) {
	lbFailing := &mockLBProvider{
		err: errors.New("connection refused to loadbalancer"),
	}

	reqCount := int64(100)
	collector := NewTPSCollector(
		nil,
		nil,
		zap.NewNop(),
		WithLoadBalancerProvider(lbFailing),
		WithTPSRequestCountFn(func() int64 { return reqCount }),
	)

	// First collection
	snap1, err := collector.Collect(context.Background())
	if err != nil {
		t.Fatalf("expected Collect to succeed gracefully on LB error: %v", err)
	}
	if snap1.HTTP.RequestsPerSec != 0 {
		t.Errorf("expected initial RequestsPerSec = 0, got %f", snap1.HTTP.RequestsPerSec)
	}

	// Advance time and req count
	time.Sleep(50 * time.Millisecond)
	reqCount = 200

	// Second collection falls back to requestCountFn
	snap2, err := collector.Collect(context.Background())
	if err != nil {
		t.Fatalf("second Collect failed: %v", err)
	}
	if snap2.HTTP.RequestsPerSec <= 0 {
		t.Errorf("expected fallback RequestsPerSec > 0, got %f", snap2.HTTP.RequestsPerSec)
	}
}

func TestNormalizeServiceNameForLB(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"gateway@file", "gateway"},
		{"tiki_gateway", "gateway"},
		{"/tiki_web", "web"},
		{"k8s_api@internal", "api"},
		{"tiki-auth", "auth"},
	}

	for _, tt := range tests {
		got := normalizeServiceNameForLB(tt.input)
		if got != tt.want {
			t.Errorf("normalizeServiceNameForLB(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

