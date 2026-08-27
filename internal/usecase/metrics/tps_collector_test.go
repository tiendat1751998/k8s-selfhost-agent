package metrics

import (
	"context"
	"encoding/json"
	"errors"
	"math"
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
	if snap1.Services[0].TotalRxBytes != 200000 {
		t.Errorf("expected postgres_db TotalRxBytes = 200000, got %d", snap1.Services[0].TotalRxBytes)
	}
	if snap1.Services[0].TotalTxBytes != 100000 {
		t.Errorf("expected postgres_db TotalTxBytes = 100000, got %d", snap1.Services[0].TotalTxBytes)
	}
	if snap1.Services[1].NodeID != "host-1" {
		t.Errorf("expected tiki_cart NodeID = host-1, got %s", snap1.Services[1].NodeID)
	}
	if snap1.Services[1].TotalRxBytes != 100000 {
		t.Errorf("expected tiki_cart TotalRxBytes = 100000, got %d", snap1.Services[1].TotalRxBytes)
	}
	if snap1.Services[1].TotalTxBytes != 50000 {
		t.Errorf("expected tiki_cart TotalTxBytes = 50000, got %d", snap1.Services[1].TotalTxBytes)
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

func TestTPSCollector_FiveContainers_K8sMaterServices(t *testing.T) {
	provider := &mockMetricsProvider{
		snapshot: &SystemOverview{
			Nodes: []NodeMetrics{
				{
					NodeID:         "host-k8smater",
					NodeName:       "k8smater",
					Status:         "ready",
					ContainerCount: 5,
					RunningCount:   5,
				},
			},
			Containers: []ContainerMetrics{
				{
					ContainerID:   "c-traefik",
					ContainerName: "tiki_traefik",
					ServiceName:   "tiki_traefik",
					NodeID:        "host-k8smater",
					NodeName:      "k8smater",
					State:         "running",
					CPUPercent:    5.0,
					MemoryUsed:    128 * 1024 * 1024,
					MemoryLimit:   1024 * 1024 * 1024,
					NetworkRx:     1000,
					NetworkTx:     2000,
				},
				{
					ContainerID:   "c-nats",
					ContainerName: "nats",
					ServiceName:   "nats",
					NodeID:        "host-k8smater",
					NodeName:      "k8smater",
					State:         "running",
					CPUPercent:    3.0,
					MemoryUsed:    64 * 1024 * 1024,
					MemoryLimit:   1024 * 1024 * 1024,
					NetworkRx:     500,
					NetworkTx:     600,
				},
				{
					ContainerID:   "c-redis",
					ContainerName: "tiki_redis",
					ServiceName:   "tiki_redis",
					NodeID:        "host-k8smater",
					NodeName:      "k8smater",
					State:         "running",
					CPUPercent:    2.0,
					MemoryUsed:    32 * 1024 * 1024,
					MemoryLimit:   1024 * 1024 * 1024,
					NetworkRx:     300,
					NetworkTx:     400,
				},
				{
					ContainerID:   "c-postgres",
					ContainerName: "postgres_db",
					ServiceName:   "postgres_db",
					NodeID:        "host-k8smater",
					NodeName:      "k8smater",
					State:         "running",
					CPUPercent:    4.0,
					MemoryUsed:    256 * 1024 * 1024,
					MemoryLimit:   1024 * 1024 * 1024,
					NetworkRx:     800,
					NetworkTx:     900,
				},
				{
					ContainerID:   "c-registry",
					ContainerName: "registry",
					ServiceName:   "registry",
					NodeID:        "host-k8smater",
					NodeName:      "k8smater",
					State:         "running",
					CPUPercent:    1.0,
					MemoryUsed:    48 * 1024 * 1024,
					MemoryLimit:   1024 * 1024 * 1024,
					NetworkRx:     100,
					NetworkTx:     200,
				},
			},
		},
		agentMetrics: map[string]*AgentMetrics{
			"host-k8smater": {
				Hostname: "k8smater",
				Status:   "online",
			},
		},
	}

	collector := NewTPSCollector(provider, nil, zap.NewNop())
	snap, err := collector.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	if len(snap.Services) != 5 {
		t.Fatalf("expected 5 services in TPSSnapshot, got %d", len(snap.Services))
	}

	expectedServices := map[string]bool{
		"tiki_traefik": true,
		"nats":         true,
		"tiki_redis":   true,
		"postgres_db":  true,
		"registry":     true,
	}

	for _, svc := range snap.Services {
		if !expectedServices[svc.ServiceName] {
			t.Errorf("unexpected service name: %s", svc.ServiceName)
		}
		if svc.NodeID != "host-k8smater" || svc.NodeName != "k8smater" {
			t.Errorf("service %s node mismatch: NodeID=%q NodeName=%q, want host-k8smater/k8smater", svc.ServiceName, svc.NodeID, svc.NodeName)
		}
		if svc.Status != "healthy" {
			t.Errorf("service %s expected status healthy, got %s", svc.ServiceName, svc.Status)
		}
		if svc.ContainerCount != 1 {
			t.Errorf("service %s expected container count 1, got %d", svc.ServiceName, svc.ContainerCount)
		}
	}
}

func TestTPSCollector_EnrichServicesWithLBStats_TraefikAndBackends(t *testing.T) {
	provider := &mockMetricsProvider{
		snapshot: &SystemOverview{
			Containers: []ContainerMetrics{
				{
					ContainerID:   "c-traefik",
					ContainerName: "tiki_traefik",
					ServiceName:   "tiki_traefik",
					State:         "running",
					CPUPercent:    5.0,
					NetworkRx:     100000,
					NetworkTx:     200000,
				},
				{
					ContainerID:   "c-db",
					ContainerName: "postgres_db",
					ServiceName:   "postgres_db",
					State:         "running",
					CPUPercent:    4.0,
					NetworkRx:     50000,
					NetworkTx:     60000,
				},
				{
					ContainerID:   "c-nats",
					ContainerName: "nats",
					ServiceName:   "nats",
					State:         "running",
					CPUPercent:    3.0,
					NetworkRx:     30000,
					NetworkTx:     40000,
				},
				{
					ContainerID:   "c-redis",
					ContainerName: "tiki_redis",
					ServiceName:   "tiki_redis",
					State:         "running",
					CPUPercent:    2.0,
					NetworkRx:     20000,
					NetworkTx:     25000,
				},
			},
		},
		agentMetrics: map[string]*AgentMetrics{
			"host-1": {
				Hostname:  "k8smater",
				NetRxRate: 200000,
				NetTxRate: 325000,
			},
		},
	}

	lbMock := &mockLBProvider{
		stats: []domainLB.ServiceRequestStats{
			{
				ServiceName:    "web@file",
				RequestsPerSec: 55.4,
				ErrorRate:      1.2,
				AvgLatencyMs:   14.5,
			},
			{
				ServiceName:    "gateway@file",
				RequestsPerSec: 40.0,
				ErrorRate:      0.5,
				AvgLatencyMs:   8.0,
			},
		},
		aggStats: &domainLB.AggregateStats{
			TotalRequestsPerSec: 95.4,
			ActiveConnections:   35,
			ErrorRate:           0.9,
			AvgLatencyMs:        11.2,
		},
	}

	collector := NewTPSCollector(
		provider,
		nil,
		zap.NewNop(),
		WithLoadBalancerProvider(lbMock),
	)

	// Set baseline and simulate active DB, Messaging, and Container stats
	collector.prevDBStats = dbStatsRaw{
		commit: 1000,
	}
	collector.prevDBTime = time.Now().Add(-1 * time.Second)
	collector.prevNATSStats = natsStatsRaw{
		inMsgs:  5000,
		outMsgs: 10000,
	}
	collector.prevNATSTime = time.Now().Add(-1 * time.Second)
	collector.prevContainerStats = map[string]containerNetRaw{
		"c-traefik": {rxBytes: 50000, txBytes: 100000},
		"c-db":      {rxBytes: 25000, txBytes: 30000},
		"c-nats":    {rxBytes: 15000, txBytes: 20000},
		"c-redis":   {rxBytes: 10000, txBytes: 12000},
	}
	collector.prevContainerTime = time.Now().Add(-1 * time.Second)

	snap, err := collector.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	svcMap := make(map[string]ServiceTPS)
	for _, s := range snap.Services {
		svcMap[s.ServiceName] = s
	}

	// 1. Verify tiki_traefik receives aggregate Traefik stats
	traefik, ok := svcMap["tiki_traefik"]
	if !ok {
		t.Fatal("expected tiki_traefik in services")
	}
	if traefik.RequestsPerSec != 95.4 {
		t.Errorf("expected tiki_traefik RequestsPerSec = 95.4, got %f", traefik.RequestsPerSec)
	}
	if traefik.ErrorRate != 0.9 {
		t.Errorf("expected tiki_traefik ErrorRate = 0.9, got %f", traefik.ErrorRate)
	}
	if traefik.AvgLatencyMs != 11.2 {
		t.Errorf("expected tiki_traefik AvgLatencyMs = 11.2, got %f", traefik.AvgLatencyMs)
	}

	// 2. Direct test with enrichServicesWithLBStats directly supplying DB and Messaging TPS
	dbTPS := DatabaseTPS{TransactionsPerSec: 45.0}
	msgTPS := MessagingTPS{InMsgsPerSec: 100.0, OutMsgsPerSec: 150.0}
	httpTPS := HTTPTPS{RequestsPerSec: 95.4, ErrorRate: 0.9, AvgLatencyMs: 11.2}

	enriched := collector.enrichServicesWithLBStats(context.Background(), snap.Services, httpTPS, dbTPS, msgTPS)
	enrichedMap := make(map[string]ServiceTPS)
	for _, s := range enriched {
		enrichedMap[s.ServiceName] = s
	}

	// Verify postgres_db receives Database TPS
	dbSvc, ok := enrichedMap["postgres_db"]
	if !ok {
		t.Fatal("expected postgres_db in enriched services")
	}
	if dbSvc.RequestsPerSec != 45.0 {
		t.Errorf("expected postgres_db RequestsPerSec = 45.0, got %f", dbSvc.RequestsPerSec)
	}

	// Verify nats receives Messaging TPS (In + Out)
	natsSvc, ok := enrichedMap["nats"]
	if !ok {
		t.Fatal("expected nats in enriched services")
	}
	if natsSvc.RequestsPerSec != 250.0 {
		t.Errorf("expected nats RequestsPerSec = 250.0, got %f", natsSvc.RequestsPerSec)
	}

	// Verify tiki_redis receives proportional traffic when active
	redisSvc, ok := enrichedMap["tiki_redis"]
	if !ok {
		t.Fatal("expected tiki_redis in enriched services")
	}
	if redisSvc.RequestsPerSec <= 0 {
		t.Errorf("expected tiki_redis RequestsPerSec > 0, got %f", redisSvc.RequestsPerSec)
	}
}

func TestTPSCollector_EnrichServicesWithLBStats_TraefikServiceMatching(t *testing.T) {
	lbMock := &mockLBProvider{
		stats: []domainLB.ServiceRequestStats{
			{
				ServiceName:    "web@file",
				RequestsPerSec: 35.0,
				ErrorRate:      2.5,
				AvgLatencyMs:   18.0,
			},
		},
		aggStats: nil, // no aggregate stats, only per-service
	}

	provider := &mockMetricsProvider{
		snapshot: &SystemOverview{
			Containers: []ContainerMetrics{
				{
					ContainerID:   "c-traefik",
					ContainerName: "tiki_traefik",
					ServiceName:   "tiki_traefik",
					State:         "running",
					CPUPercent:    5.0,
					NetworkRx:     50000,
					NetworkTx:     100000,
				},
			},
		},
	}

	collector := NewTPSCollector(
		provider,
		nil,
		zap.NewNop(),
		WithLoadBalancerProvider(lbMock),
	)

	snap, err := collector.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	if len(snap.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(snap.Services))
	}

	traefik := snap.Services[0]
	if traefik.ServiceName != "tiki_traefik" {
		t.Errorf("expected service tiki_traefik, got %s", traefik.ServiceName)
	}
	if traefik.RequestsPerSec != 35.0 {
		t.Errorf("expected tiki_traefik RequestsPerSec = 35.0, got %f", traefik.RequestsPerSec)
	}
	if traefik.ErrorRate != 2.5 {
		t.Errorf("expected tiki_traefik ErrorRate = 2.5, got %f", traefik.ErrorRate)
	}
}

func TestIsServiceHelpers(t *testing.T) {
	traefikTests := []struct {
		name string
		want bool
	}{
		{"tiki_traefik", true},
		{"traefik", true},
		{"traefik_proxy", true},
		{"ingress", true},
		{"ingress-controller", true},
		{"k8s_traefik", true},
		{"tiki_redis", false},
		{"nats", false},
		{"postgres_db", false},
	}

	for _, tt := range traefikTests {
		got := isTraefikService(tt.name)
		if got != tt.want {
			t.Errorf("isTraefikService(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}

	dbTests := []struct {
		name string
		want bool
	}{
		{"postgres_db", true},
		{"db", true},
		{"database", true},
		{"tiki_db", true},
		{"postgres", true},
		{"mysql_db", true},
		{"tiki_redis", false},
		{"dashboard", false},
		{"tiki_traefik", false},
	}

	for _, tt := range dbTests {
		got := isDatabaseService(tt.name)
		if got != tt.want {
			t.Errorf("isDatabaseService(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}

	msgTests := []struct {
		name string
		want bool
	}{
		{"nats", true},
		{"tiki_nats", true},
		{"kafka", true},
		{"rabbitmq", true},
		{"message_broker", true},
		{"tiki_redis", false},
		{"tiki_traefik", false},
	}

	for _, tt := range msgTests {
		got := isMessagingService(tt.name)
		if got != tt.want {
			t.Errorf("isMessagingService(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}

	cacheTests := []struct {
		name string
		want bool
	}{
		{"tiki_redis", true},
		{"redis", true},
		{"memcached", true},
		{"tiki_traefik", false},
		{"postgres_db", false},
	}

	for _, tt := range cacheTests {
		got := isCacheService(tt.name)
		if got != tt.want {
			t.Errorf("isCacheService(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestTPSCollector_IntelligentContainerNetworkFallback(t *testing.T) {
	provider := &mockMetricsProvider{
		snapshot: &SystemOverview{
			Containers: []ContainerMetrics{
				{
					ContainerID:   "c-traefik",
					ContainerName: "tiki_traefik",
					ServiceName:   "tiki_traefik",
					State:         "running",
					CPUPercent:    85.0,
					NetworkRx:     12000000,
					NetworkRxRate: 1200000, // 1.2 MB/s = ~1000 RPS (1,200,000 / 1200)
					NetworkTx:     24000000,
					NetworkTxRate: 2400000,
				},
				{
					ContainerID:   "c-backend",
					ContainerName: "tiki_gateway",
					ServiceName:   "tiki_gateway",
					State:         "running",
					CPUPercent:    50.0,
					NetworkRx:     6000000,
					NetworkRxRate: 600000,
					NetworkTx:     6000000,
					NetworkTxRate: 600000,
				},
			},
		},
	}

	// Load balancer provider returns error (simulating timeout under 1000+ connections)
	lbMock := &mockLBProvider{
		err: errors.New("management port 8080 timed out under load"),
	}

	reqCount := int64(5) // Internal API request counter only reports 5
	collector := NewTPSCollector(
		provider,
		nil,
		zap.NewNop(),
		WithLoadBalancerProvider(lbMock),
		WithTPSRequestCountFn(func() int64 { return reqCount }),
	)

	snap, err := collector.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	// Throughput should estimate real traffic (1,200,000 / 1200 = 1000 RPS) rather than 5 req/s
	if snap.HTTP.RequestsPerSec != 1000.0 {
		t.Errorf("expected HTTP RequestsPerSec = 1000.0 via container network fallback, got %f", snap.HTTP.RequestsPerSec)
	}

	// Verify tiki_traefik in Services also reflects 1000 RPS
	var traefikSvc *ServiceTPS
	for i := range snap.Services {
		if snap.Services[i].ServiceName == "tiki_traefik" {
			traefikSvc = &snap.Services[i]
			break
		}
	}
	if traefikSvc == nil {
		t.Fatal("expected tiki_traefik service in snapshot")
	}
	if traefikSvc.RequestsPerSec != 1000.0 {
		t.Errorf("expected tiki_traefik RequestsPerSec = 1000.0, got %f", traefikSvc.RequestsPerSec)
	}
}

func TestTPSCollector_CounterResetProtection_AllSources(t *testing.T) {
	// NATS server
	natsIteration := 0
	natsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if natsIteration == 0 {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"in_msgs":     int64(50000),
				"out_msgs":    int64(60000),
				"in_bytes":    int64(50000000),
				"out_bytes":   int64(60000000),
				"in_rate":     0.0,
				"out_rate":    0.0,
				"connections": 10,
			})
		} else {
			// Counter reset
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"in_msgs":     int64(500),
				"out_msgs":    int64(600),
				"in_bytes":    int64(500000),
				"out_bytes":   int64(600000),
				"in_rate":     0.0,
				"out_rate":    0.0,
				"connections": 10,
			})
		}
	}))
	defer natsSrv.Close()

	// DB mock
	dbIteration := 0
	mockDB := &mockDBPool{
		rowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			dbIteration++
			if dbIteration == 1 {
				return &mockDBRow{
					values: []any{
						int64(100000), int64(5000), int64(500000), int64(300000),
						int64(20000), int64(15000), int64(2000), int64(10),
						int64(10000), int64(90000),
					},
				}
			}
			// DB restarted, counters reset to small values
			return &mockDBRow{
				values: []any{
					int64(50), int64(2), int64(300), int64(200),
					int64(10), int64(8), int64(1), int64(10),
					int64(10), int64(90),
				},
			}
		},
	}

	provider := &mockMetricsProvider{
		snapshot: &SystemOverview{
			Containers: []ContainerMetrics{
				{
					ContainerID:   "c-app",
					ContainerName: "my_app",
					ServiceName:   "my_app",
					State:         "running",
					NetworkRx:     10000000,
					NetworkTx:     5000000,
				},
			},
		},
	}

	httpReqs := int64(10000)
	collector := NewTPSCollector(
		provider,
		mockDB,
		zap.NewNop(),
		WithNATSMonitorURL(natsSrv.URL),
		WithTPSRequestCountFn(func() int64 { return httpReqs }),
	)

	// First baseline collection
	_, err := collector.Collect(context.Background())
	if err != nil {
		t.Fatalf("first collect failed: %v", err)
	}

	// Trigger counter resets
	time.Sleep(50 * time.Millisecond)
	natsIteration = 1
	httpReqs = 50
	provider.snapshot = &SystemOverview{
		Containers: []ContainerMetrics{
			{
				ContainerID:   "c-app",
				ContainerName: "my_app",
				ServiceName:   "my_app",
				State:         "running",
				NetworkRx:     1000,
				NetworkTx:     500,
			},
		},
	}

	snap2, err := collector.Collect(context.Background())
	if err != nil {
		t.Fatalf("second collect failed: %v", err)
	}

	if snap2.HTTP.RequestsPerSec <= 0 {
		t.Errorf("expected positive HTTP RequestsPerSec after reset, got %f", snap2.HTTP.RequestsPerSec)
	}
	if snap2.Database.TransactionsPerSec <= 0 {
		t.Errorf("expected positive DB TransactionsPerSec after reset, got %f", snap2.Database.TransactionsPerSec)
	}
	if snap2.Database.ReadsPerSec <= 0 {
		t.Errorf("expected positive DB ReadsPerSec after reset, got %f", snap2.Database.ReadsPerSec)
	}
	if snap2.Database.WritesPerSec <= 0 {
		t.Errorf("expected positive DB WritesPerSec after reset, got %f", snap2.Database.WritesPerSec)
	}
	if snap2.Messaging.InBytesPerSec <= 0 {
		t.Errorf("expected positive NATS InBytesPerSec after reset, got %d", snap2.Messaging.InBytesPerSec)
	}
	if snap2.Messaging.InMsgsPerSec <= 0 {
		t.Errorf("expected positive NATS InMsgsPerSec after reset, got %f", snap2.Messaging.InMsgsPerSec)
	}
}

func TestTPSCollector_DeltaDatabaseCacheHitRatio(t *testing.T) {
	dbStep := 0
	mockDB := &mockDBPool{
		rowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			dbStep++
			switch dbStep {
			case 1:
				// Baseline: 1000 read, 9000 hit (90%)
				return &mockDBRow{
					values: []any{
						int64(100), int64(0), int64(1000), int64(1000),
						int64(10), int64(10), int64(0), int64(5),
						int64(1000), int64(9000),
					},
				}
			case 2:
				// Active query step: +200 read (1200), +800 hit (9800) -> delta ratio = 800/(800+200) = 80%
				return &mockDBRow{
					values: []any{
						int64(200), int64(0), int64(2000), int64(2000),
						int64(20), int64(20), int64(0), int64(5),
						int64(1200), int64(9800),
					},
				}
			default:
				// Idle step: +0 read (1200), +0 hit (9800) -> delta=0, fallback to cumulative: 9800/(9800+1200) = 0.8909
				return &mockDBRow{
					values: []any{
						int64(200), int64(0), int64(2000), int64(2000),
						int64(20), int64(20), int64(0), int64(5),
						int64(1200), int64(9800),
					},
				}
			}
		},
	}

	collector := NewTPSCollector(nil, mockDB, zap.NewNop())

	// Step 1: Baseline
	snap1, err := collector.Collect(context.Background())
	if err != nil {
		t.Fatalf("step 1 failed: %v", err)
	}
	if snap1.Database.CacheHitRatio != 0.9000 {
		t.Errorf("step 1 expected cumulative 0.9000, got %f", snap1.Database.CacheHitRatio)
	}

	// Step 2: Active queries with 80% delta hit ratio
	time.Sleep(50 * time.Millisecond)
	snap2, err := collector.Collect(context.Background())
	if err != nil {
		t.Fatalf("step 2 failed: %v", err)
	}
	if snap2.Database.CacheHitRatio != 0.8000 {
		t.Errorf("step 2 expected delta 0.8000, got %f", snap2.Database.CacheHitRatio)
	}

	// Step 3: Idle step -> delta total blocks == 0 -> fallback to cumulative
	time.Sleep(50 * time.Millisecond)
	snap3, err := collector.Collect(context.Background())
	if err != nil {
		t.Fatalf("step 3 failed: %v", err)
	}
	expectedCumulative := 0.8909
	if math.Abs(snap3.Database.CacheHitRatio-expectedCumulative) > 0.001 {
		t.Errorf("step 3 expected fallback cumulative ~0.8909, got %f", snap3.Database.CacheHitRatio)
	}
}

func TestTPSCollector_MaxLatencyMs_Propagation(t *testing.T) {
	lbMock := &mockLBProvider{
		aggStats: &domainLB.AggregateStats{
			TotalRequests:       10000,
			TotalRequestsPerSec: 50.0,
			ActiveConnections:   10,
			ErrorRate:           0.5,
			AvgLatencyMs:        15.0,
			MaxLatencyMs:        24.0,
		},
		stats: []domainLB.ServiceRequestStats{
			{
				ServiceName:    "gateway",
				TotalRequests:  5000,
				RequestsPerSec: 25.0,
				ErrorRate:      0.2,
				AvgLatencyMs:   12.0,
				MaxLatencyMs:   19.2,
			},
		},
	}

	provider := &mockMetricsProvider{
		snapshot: &SystemOverview{
			Containers: []ContainerMetrics{
				{
					ContainerID:   "c1",
					ContainerName: "tiki_traefik",
					ServiceName:   "tiki_traefik",
					State:         "running",
					NetworkRx:     1000000,
					NetworkRxRate: 100000,
				},
				{
					ContainerID:   "c2",
					ContainerName: "gateway",
					ServiceName:   "gateway",
					State:         "running",
					NetworkRx:     500000,
					NetworkRxRate: 50000,
				},
				{
					ContainerID:   "c3",
					ContainerName: "tiki_orders",
					ServiceName:   "tiki_orders",
					State:         "running",
					NetworkRx:     500000,
					NetworkRxRate: 50000,
				},
			},
		},
	}

	collector := NewTPSCollector(
		provider,
		nil,
		zap.NewNop(),
		WithLoadBalancerProvider(lbMock),
	)

	snap, err := collector.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	// 1. HTTP TPS should reflect AggregateStats MaxLatencyMs
	if snap.HTTP.MaxLatencyMs != 24.0 {
		t.Errorf("expected HTTP MaxLatencyMs = 24.0, got %f", snap.HTTP.MaxLatencyMs)
	}

	// 2. Services
	svcMap := make(map[string]ServiceTPS)
	for _, s := range snap.Services {
		svcMap[s.ServiceName] = s
	}

	// Gateway should have matched LB MaxLatencyMs (19.2)
	if gw, ok := svcMap["gateway"]; !ok || gw.MaxLatencyMs != 19.2 {
		t.Errorf("expected gateway MaxLatencyMs = 19.2, got %f", gw.MaxLatencyMs)
	}

	// tiki_traefik should reflect HTTP MaxLatencyMs (24.0)
	if tr, ok := svcMap["tiki_traefik"]; !ok || tr.MaxLatencyMs != 24.0 {
		t.Errorf("expected tiki_traefik MaxLatencyMs = 24.0, got %f", tr.MaxLatencyMs)
	}

	// tiki_orders (fallback backend) should inherit HTTP MaxLatencyMs (24.0)
	if ord, ok := svcMap["tiki_orders"]; !ok || ord.MaxLatencyMs != 24.0 {
		t.Errorf("expected tiki_orders MaxLatencyMs = 24.0, got %f", ord.MaxLatencyMs)
	}
}

func TestTPSCollector_NoFabricatedRPS_WhenHTTPZero(t *testing.T) {
	collector := NewTPSCollector(nil, nil, zap.NewNop())

	services := []ServiceTPS{
		{
			ServiceName:   "postgres_db",
			RxBytesPerSec: 50000,
		},
		{
			ServiceName:   "nats",
			RxBytesPerSec: 30000,
		},
		{
			ServiceName:   "tiki_orders",
			RxBytesPerSec: 100000,
		},
	}

	httpTPS := HTTPTPS{
		RequestsPerSec: 0,
		ErrorRate:      0,
		AvgLatencyMs:   0,
	}

	dbTPS := DatabaseTPS{
		TransactionsPerSec: 15.5,
	}

	msgTPS := MessagingTPS{
		InMsgsPerSec:  10.0,
		OutMsgsPerSec: 20.0,
	}

	enriched := collector.enrichServicesWithLBStats(context.Background(), services, httpTPS, dbTPS, msgTPS)

	svcMap := make(map[string]ServiceTPS)
	for _, s := range enriched {
		svcMap[s.ServiceName] = s
	}

	// DB should be 15.5
	if db, ok := svcMap["postgres_db"]; !ok || db.RequestsPerSec != 15.5 {
		t.Errorf("expected postgres_db RequestsPerSec = 15.5, got %f", db.RequestsPerSec)
	}

	// NATS should be 30.0 (10+20)
	if nats, ok := svcMap["nats"]; !ok || nats.RequestsPerSec != 30.0 {
		t.Errorf("expected nats RequestsPerSec = 30.0, got %f", nats.RequestsPerSec)
	}

	// tiki_orders must be 0 (never fabricated when HTTP is 0)
	if ord, ok := svcMap["tiki_orders"]; !ok || ord.RequestsPerSec != 0 {
		t.Errorf("expected tiki_orders RequestsPerSec = 0 when HTTP is 0, got %f", ord.RequestsPerSec)
	}
}




