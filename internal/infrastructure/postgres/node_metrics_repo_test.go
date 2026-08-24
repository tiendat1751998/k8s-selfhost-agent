package postgres_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datdt/k8sselfhost/internal/domain/nodemetrics"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
)

func getNodeMetricsTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	host := os.Getenv("K8S_POSTGRES_HOST")
	if host == "" {
		host = "10.10.10.133"
	}
	user := os.Getenv("K8S_POSTGRES_USER")
	if user == "" {
		user = "myuser"
	}
	pass := os.Getenv("K8S_POSTGRES_PASSWORD")
	if pass == "" {
		pass = "mysecretpassword"
	}
	dbname := os.Getenv("K8S_POSTGRES_DBNAME")
	if dbname == "" {
		dbname = "mydatabase"
	}
	port := os.Getenv("K8S_POSTGRES_PORT")
	if port == "" {
		port = "5432"
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, pass, host, port, dbname)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("Skipping test: cannot connect to database: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("Skipping test: database ping failed: %v", err)
	}

	// Ensure table exists for testing
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS node_metric_rollups (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id UUID NOT NULL,
			node_id TEXT NOT NULL,
			node_name TEXT NOT NULL,
			cpu_percent DOUBLE PRECISION NOT NULL DEFAULT 0,
			cpu_peak DOUBLE PRECISION NOT NULL DEFAULT 0,
			mem_used_bytes BIGINT NOT NULL DEFAULT 0,
			mem_total_bytes BIGINT NOT NULL DEFAULT 0,
			mem_percent DOUBLE PRECISION NOT NULL DEFAULT 0,
			disk_used_bytes BIGINT NOT NULL DEFAULT 0,
			disk_total_bytes BIGINT NOT NULL DEFAULT 0,
			disk_percent DOUBLE PRECISION NOT NULL DEFAULT 0,
			rx_bytes_per_sec BIGINT NOT NULL DEFAULT 0,
			tx_bytes_per_sec BIGINT NOT NULL DEFAULT 0,
			process_count INT NOT NULL DEFAULT 0,
			container_count INT NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'online',
			resolution TEXT NOT NULL DEFAULT '1m',
			recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`
	_, _ = pool.Exec(ctx, createTableSQL)
	return pool
}

func TestNodeMetricsRepo_InsertAndQuery(t *testing.T) {
	pool := getNodeMetricsTestPool(t)
	defer pool.Close()

	repo := postgres.NewNodeMetricsRepo(pool)
	ctx := context.Background()
	testNodeID := "test-node-" + uuid.NewString()[:8]

	now := time.Now().UTC()
	samples := []nodemetrics.NodeMetricRollup{
		{
			NodeID:         testNodeID,
			NodeName:       "Test Master",
			CPUPercent:     25.5,
			CPUPeak:        45.0,
			MemUsedBytes:   4 * 1024 * 1024 * 1024,
			MemTotalBytes:  16 * 1024 * 1024 * 1024,
			MemPercent:     25.0,
			DiskUsedBytes:  50 * 1024 * 1024 * 1024,
			DiskTotalBytes: 200 * 1024 * 1024 * 1024,
			DiskPercent:    25.0,
			RxBytesPerSec:  1024000,
			TxBytesPerSec:  512000,
			ProcessCount:   150,
			ContainerCount: 5,
			Status:         "online",
			Resolution:     "1m",
			RecordedAt:     now.Add(-2 * time.Minute),
		},
		{
			NodeID:         testNodeID,
			NodeName:       "Test Master",
			CPUPercent:     30.0,
			CPUPeak:        55.0,
			MemUsedBytes:   5 * 1024 * 1024 * 1024,
			MemTotalBytes:  16 * 1024 * 1024 * 1024,
			MemPercent:     31.25,
			DiskUsedBytes:  50 * 1024 * 1024 * 1024,
			DiskTotalBytes: 200 * 1024 * 1024 * 1024,
			DiskPercent:    25.0,
			RxBytesPerSec:  2048000,
			TxBytesPerSec:  1024000,
			ProcessCount:   155,
			ContainerCount: 5,
			Status:         "online",
			Resolution:     "1m",
			RecordedAt:     now.Add(-1 * time.Minute),
		},
	}

	err := repo.InsertBatch(ctx, samples)
	if err != nil {
		t.Fatalf("InsertBatch failed: %v", err)
	}

	// Query History
	history, err := repo.QueryHistory(ctx, nodemetrics.NodeHistoryQuery{
		NodeID:     testNodeID,
		Resolution: "1m",
		StartTime:  now.Add(-10 * time.Minute),
		EndTime:    now.Add(1 * time.Minute),
	})
	if err != nil {
		t.Fatalf("QueryHistory failed: %v", err)
	}

	if len(history) < 2 {
		t.Errorf("expected at least 2 history records, got %d", len(history))
	}

	// Get Summary
	summary, err := repo.GetSummary(ctx, nodemetrics.NodeHistoryQuery{
		NodeID:    testNodeID,
		StartTime: now.Add(-10 * time.Minute),
		EndTime:   now.Add(1 * time.Minute),
	})
	if err != nil {
		t.Fatalf("GetSummary failed: %v", err)
	}

	if summary.PeakCPUPercent < 55.0 {
		t.Errorf("expected PeakCPUPercent >= 55.0, got %f", summary.PeakCPUPercent)
	}
	if summary.TotalSamples < 2 {
		t.Errorf("expected TotalSamples >= 2, got %d", summary.TotalSamples)
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM node_metric_rollups WHERE node_id = $1", testNodeID)
}

func TestNodeMetricsRepo_QueryHistory_24h_1440Points(t *testing.T) {
	pool := getNodeMetricsTestPool(t)
	defer pool.Close()

	repo := postgres.NewNodeMetricsRepo(pool)
	ctx := context.Background()
	testNodeID := "test-node-24h-" + uuid.NewString()[:8]

	now := time.Now().UTC().Truncate(time.Minute)
	var samples []nodemetrics.NodeMetricRollup
	// Generate 1440 1-minute samples (a full 24-hour day)
	for i := 1439; i >= 0; i-- {
		samples = append(samples, nodemetrics.NodeMetricRollup{
			NodeID:         testNodeID,
			NodeName:       "Test Full Day Master",
			CPUPercent:     20.0 + float64(i%50),
			CPUPeak:        30.0 + float64(i%50),
			MemUsedBytes:   8 * 1024 * 1024 * 1024,
			MemTotalBytes:  16 * 1024 * 1024 * 1024,
			MemPercent:     50.0,
			DiskUsedBytes:  50 * 1024 * 1024 * 1024,
			DiskTotalBytes: 200 * 1024 * 1024 * 1024,
			DiskPercent:    25.0,
			RxBytesPerSec:  1024000,
			TxBytesPerSec:  512000,
			ProcessCount:   150,
			ContainerCount: 5,
			Status:         "online",
			Resolution:     "1m",
			RecordedAt:     now.Add(-time.Duration(i) * time.Minute),
		})
	}

	// Insert in batches of 200 to keep transaction manageable
	batchSize := 200
	for i := 0; i < len(samples); i += batchSize {
		end := i + batchSize
		if end > len(samples) {
			end = len(samples)
		}
		if err := repo.InsertBatch(ctx, samples[i:end]); err != nil {
			t.Fatalf("InsertBatch failed at batch %d: %v", i, err)
		}
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM node_metric_rollups WHERE node_id = $1", testNodeID)
	}()

	// 1. Query with Limit=1500: should return all 1440 points
	history1500, err := repo.QueryHistory(ctx, nodemetrics.NodeHistoryQuery{
		NodeID:     testNodeID,
		Resolution: "1m",
		StartTime:  now.Add(-24 * time.Hour),
		EndTime:    now.Add(1 * time.Minute),
		Limit:      1500,
	})
	if err != nil {
		t.Fatalf("QueryHistory with limit=1500 failed: %v", err)
	}
	if len(history1500) != 1440 {
		t.Fatalf("expected 1440 points with limit=1500, got %d", len(history1500))
	}

	// 2. Query with Limit=0 (default limit=1500): should return all 1440 points without truncation
	historyDefault, err := repo.QueryHistory(ctx, nodemetrics.NodeHistoryQuery{
		NodeID:     testNodeID,
		Resolution: "1m",
		StartTime:  now.Add(-24 * time.Hour),
		EndTime:    now.Add(1 * time.Minute),
		Limit:      0,
	})
	if err != nil {
		t.Fatalf("QueryHistory with default limit failed: %v", err)
	}
	if len(historyDefault) != 1440 {
		t.Fatalf("expected 1440 points with default limit (1500), got %d (truncation bug still present!)", len(historyDefault))
	}

	// Verify points are in chronological order (ASC)
	for i := 1; i < len(historyDefault); i++ {
		if historyDefault[i].RecordedAt.Before(historyDefault[i-1].RecordedAt) {
			t.Fatalf("history records not in chronological order: index %d (%v) before index %d (%v)",
				i, historyDefault[i].RecordedAt, i-1, historyDefault[i-1].RecordedAt)
		}
	}
}
