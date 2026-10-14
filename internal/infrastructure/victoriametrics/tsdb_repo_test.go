package victoriametrics

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTSDBRepository_WriteBatch_Success(t *testing.T) {
	var receivedBody string
	var receivedPath string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	repo := NewTSDBRepository(TSDBConfig{
		BaseURL: ts.URL,
		Timeout: 2 * time.Second,
	})

	samples := []MetricSample{
		{
			NodeID:      "node-1",
			ClusterID:   "prod-us-east-1",
			TenantID:    "tenant-alpha",
			TimestampMs: 1697274000000,
			CPUPercent:  45.2,
			MemPercent:  62.8,
			DiskPercent: 78.1,
			NetRxBytes:  1048576,
			NetTxBytes:  2097152,
		},
	}

	ctx := context.Background()
	err := repo.WriteBatch(ctx, samples)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedPath != "/api/v1/import/prometheus" {
		t.Errorf("expected path /api/v1/import/prometheus, got %s", receivedPath)
	}

	// Verify all 5 metrics are present with tenant_id isolation
	expectedSubstrings := []string{
		`node_cpu_percent{tenant_id="tenant-alpha",cluster_id="prod-us-east-1",node_id="node-1"} 45.20 1697274000000`,
		`node_mem_percent{tenant_id="tenant-alpha",cluster_id="prod-us-east-1",node_id="node-1"} 62.80 1697274000000`,
		`node_disk_percent{tenant_id="tenant-alpha",cluster_id="prod-us-east-1",node_id="node-1"} 78.10 1697274000000`,
		`node_net_rx_bytes{tenant_id="tenant-alpha",cluster_id="prod-us-east-1",node_id="node-1"} 1048576 1697274000000`,
		`node_net_tx_bytes{tenant_id="tenant-alpha",cluster_id="prod-us-east-1",node_id="node-1"} 2097152 1697274000000`,
	}

	for _, sub := range expectedSubstrings {
		if !strings.Contains(receivedBody, sub) {
			t.Errorf("missing expected line: %s\nreceived body:\n%s", sub, receivedBody)
		}
	}
}

func TestTSDBRepository_WriteBatch_TenantIsolation(t *testing.T) {
	repo := NewTSDBRepository(TSDBConfig{BaseURL: "http://127.0.0.1:8428"})

	samples := []MetricSample{
		{
			NodeID:      "node-1",
			ClusterID:   "prod",
			TenantID:    "", // Missing tenant
			TimestampMs: 1697274000000,
			CPUPercent:  10.0,
		},
	}

	err := repo.WriteBatch(context.Background(), samples)
	if err == nil {
		t.Fatal("expected error for empty tenant ID, got nil")
	}
	if !strings.Contains(err.Error(), "tenant_id is required") {
		t.Errorf("expected tenant isolation error, got %v", err)
	}
}

func TestTSDBRepository_WriteBatch_Empty(t *testing.T) {
	repo := NewTSDBRepository(TSDBConfig{BaseURL: "http://127.0.0.1:8428"})
	err := repo.WriteBatch(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected nil for empty batch, got %v", err)
	}
}

func TestTSDBRepository_QueryRange_Success(t *testing.T) {
	mockResponse := vmQueryResponse{
		Status: "success",
		Data: vmData{
			ResultType: "matrix",
			Result: []vmResult{
				{
					Metric: map[string]string{
						"__name__":   "node_cpu_percent",
						"tenant_id":  "tenant-beta",
						"cluster_id": "cluster-1",
						"node_id":    "node-42",
					},
					Values: [][]interface{}{
						{float64(1697274000), "42.5"},
						{float64(1697274060), "45.0"},
					},
				},
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		q := query.Get("query")
		if !strings.Contains(q, `tenant_id="tenant-beta"`) {
			http.Error(w, "missing tenant_id in query", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer ts.Close()

	repo := NewTSDBRepository(TSDBConfig{
		BaseURL: ts.URL,
		Timeout: 2 * time.Second,
	})

	series, err := repo.QueryRange(context.Background(), "tenant-beta", "cluster-1", "node-42", 1697274000000, 1697274120000)
	if err != nil {
		t.Fatalf("unexpected query error: %v", err)
	}

	if len(series) != 1 {
		t.Fatalf("expected 1 series, got %d", len(series))
	}

	s := series[0]
	if s.Metric["node_id"] != "node-42" {
		t.Errorf("expected node-42, got %s", s.Metric["node_id"])
	}
	if len(s.Points) != 2 {
		t.Fatalf("expected 2 points, got %d", len(s.Points))
	}
	if s.Points[0].Value != 42.5 || s.Points[0].TimestampMs != 1697274000000 {
		t.Errorf("unexpected first point: %+v", s.Points[0])
	}
	if s.Points[1].Value != 45.0 || s.Points[1].TimestampMs != 1697274060000 {
		t.Errorf("unexpected second point: %+v", s.Points[1])
	}
}

func TestTSDBRepository_QueryRange_TenantIsolation(t *testing.T) {
	repo := NewTSDBRepository(TSDBConfig{BaseURL: "http://127.0.0.1:8428"})
	_, err := repo.QueryRange(context.Background(), "", "cluster-1", "node-1", 100, 200)
	if err == nil {
		t.Fatal("expected error for empty tenantID, got nil")
	}
}

func BenchmarkTSDBRepository_WriteBatch(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	repo := NewTSDBRepository(TSDBConfig{
		BaseURL: ts.URL,
		Timeout: 5 * time.Second,
	})

	samples := make([]MetricSample, 1000)
	for i := 0; i < 1000; i++ {
		samples[i] = MetricSample{
			NodeID:      "node-benchmark",
			ClusterID:   "perf-cluster",
			TenantID:    "tenant-benchmark",
			TimestampMs: 1697274000000 + int64(i*1000),
			CPUPercent:  25.5,
			MemPercent:  55.0,
			DiskPercent: 40.2,
			NetRxBytes:  1024,
			NetTxBytes:  2048,
		}
	}

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := repo.WriteBatch(ctx, samples); err != nil {
			b.Fatalf("WriteBatch error: %v", err)
		}
	}
}
