package metrics

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
	if snap1.Services[0].Status != "healthy" {
		t.Errorf("expected postgres_db status = healthy, got %s", snap1.Services[0].Status)
	}
	if snap1.Services[0].MemoryUsedMB != 256.0 {
		t.Errorf("expected postgres_db MemoryUsedMB = 256.0, got %f", snap1.Services[0].MemoryUsedMB)
	}
	if snap1.Database.ActiveConnections != 12 {
		t.Errorf("expected DB ActiveConnections = 12, got %d", snap1.Database.ActiveConnections)
	}
	if snap1.Database.CacheHitRatio != 0.99 {
		t.Errorf("expected DB CacheHitRatio = 0.99, got %f", snap1.Database.CacheHitRatio)
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

