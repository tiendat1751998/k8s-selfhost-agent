package logging_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	domainLogging "github.com/datdt/k8sselfhost/internal/domain/logging"
	infraLogging "github.com/datdt/k8sselfhost/internal/infrastructure/logging"
)

func TestMemoryLogRepo_Ingest_And_Query(t *testing.T) {
	ctx := context.Background()
	repo := infraLogging.NewMemoryLogRepo(50)

	baseTime := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	entries := []domainLogging.LogEntry{
		{
			TenantID:      "tenant-1",
			ClusterID:     "cluster-a",
			Namespace:     "prod",
			PodName:       "api-1",
			ContainerName: "app",
			LogLevel:      domainLogging.LogLevelInfo,
			Message:       "Server listening on port 8080",
			Timestamp:     baseTime,
		},
		{
			TenantID:      "tenant-1",
			ClusterID:     "cluster-a",
			Namespace:     "prod",
			PodName:       "api-1",
			ContainerName: "app",
			LogLevel:      domainLogging.LogLevelWarn,
			Message:       "High memory usage warning",
			Timestamp:     baseTime.Add(1 * time.Minute),
		},
		{
			TenantID:      "tenant-1",
			ClusterID:     "cluster-a",
			Namespace:     "prod",
			PodName:       "worker-1",
			ContainerName: "worker",
			LogLevel:      domainLogging.LogLevelError,
			Message:       "Database connection timeout",
			Timestamp:     baseTime.Add(2 * time.Minute),
		},
		{
			TenantID:      "tenant-2",
			ClusterID:     "cluster-b",
			Namespace:     "dev",
			PodName:       "api-2",
			ContainerName: "app",
			LogLevel:      domainLogging.LogLevelInfo,
			Message:       "Dev server ready",
			Timestamp:     baseTime.Add(3 * time.Minute),
		},
	}

	if err := repo.IngestBatch(ctx, entries); err != nil {
		t.Fatalf("unexpected ingest error: %v", err)
	}

	// 1. Query with Tenant isolation
	res, err := repo.QueryLogs(ctx, domainLogging.LogFilter{
		TenantID: "tenant-1",
	})
	if err != nil {
		t.Fatalf("unexpected query error: %v", err)
	}
	if res.TotalCount != 3 {
		t.Fatalf("expected 3 entries for tenant-1, got %d", res.TotalCount)
	}
	// Check newest first ordering
	if res.Entries[0].Message != "Database connection timeout" {
		t.Errorf("expected newest first, got message %q", res.Entries[0].Message)
	}

	// 2. Query with search_text
	resSearch, err := repo.QueryLogs(ctx, domainLogging.LogFilter{
		TenantID:   "tenant-1",
		SearchText: "memory",
	})
	if err != nil {
		t.Fatalf("unexpected search error: %v", err)
	}
	if len(resSearch.Entries) != 1 || resSearch.Entries[0].LogLevel != domainLogging.LogLevelWarn {
		t.Errorf("expected 1 search match with warn level, got %d", len(resSearch.Entries))
	}

	// 3. Query with log_level
	resErr, err := repo.QueryLogs(ctx, domainLogging.LogFilter{
		TenantID: "tenant-1",
		LogLevel: domainLogging.LogLevelError,
	})
	if err != nil {
		t.Fatalf("unexpected level query error: %v", err)
	}
	if len(resErr.Entries) != 1 || resErr.Entries[0].PodName != "worker-1" {
		t.Errorf("expected worker-1 error log, got %v", resErr.Entries)
	}

	// 4. Query with time window
	resTime, err := repo.QueryLogs(ctx, domainLogging.LogFilter{
		TenantID:  "tenant-1",
		StartTime: baseTime,
		EndTime:   baseTime.Add(90 * time.Second),
	})
	if err != nil {
		t.Fatalf("unexpected time query error: %v", err)
	}
	if len(resTime.Entries) != 2 {
		t.Errorf("expected 2 entries in time window, got %d", len(resTime.Entries))
	}

	// 5. Pagination: limit & offset
	resPage, err := repo.QueryLogs(ctx, domainLogging.LogFilter{
		TenantID: "tenant-1",
		Limit:    1,
		Offset:   1,
	})
	if err != nil {
		t.Fatalf("unexpected page query error: %v", err)
	}
	if len(resPage.Entries) != 1 || resPage.Entries[0].Message != "High memory usage warning" {
		t.Errorf("expected offset 1 item to be memory warning, got %v", resPage.Entries)
	}
	if !resPage.HasMore {
		t.Errorf("expected HasMore to be true")
	}
}

func TestMemoryLogRepo_RingBuffer_Capacity(t *testing.T) {
	ctx := context.Background()
	repo := infraLogging.NewMemoryLogRepo(5)

	baseTime := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		err := repo.IngestBatch(ctx, []domainLogging.LogEntry{
			{
				TenantID:  "tenant-ring",
				ClusterID: "c1",
				Namespace: "default",
				LogLevel:  domainLogging.LogLevelInfo,
				Message:   fmt.Sprintf("log message %d", i),
				Timestamp: baseTime.Add(time.Duration(i) * time.Second),
			},
		})
		if err != nil {
			t.Fatalf("ingest error at %d: %v", i, err)
		}
	}

	res, err := repo.QueryLogs(ctx, domainLogging.LogFilter{
		TenantID: "tenant-ring",
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("query error: %v", err)
	}

	if res.TotalCount != 5 {
		t.Fatalf("expected exactly 5 entries retained in ring buffer, got %d", res.TotalCount)
	}

	if res.Entries[0].Message != "log message 9" {
		t.Errorf("expected newest entry to be 9, got %s", res.Entries[0].Message)
	}
	if res.Entries[4].Message != "log message 5" {
		t.Errorf("expected oldest entry in ring to be 5, got %s", res.Entries[4].Message)
	}
}

func TestMemoryLogRepo_GetHistogram(t *testing.T) {
	ctx := context.Background()
	repo := infraLogging.NewMemoryLogRepo(100)

	baseTime := time.Date(2026, 9, 10, 14, 0, 10, 0, time.UTC)
	entries := []domainLogging.LogEntry{
		{
			TenantID:  "tenant-hist",
			ClusterID: "c1",
			LogLevel:  domainLogging.LogLevelInfo,
			Timestamp: baseTime,
		},
		{
			TenantID:  "tenant-hist",
			ClusterID: "c1",
			LogLevel:  domainLogging.LogLevelError,
			Timestamp: baseTime.Add(5 * time.Second),
		},
		{
			TenantID:  "tenant-hist",
			ClusterID: "c1",
			LogLevel:  domainLogging.LogLevelInfo,
			Timestamp: baseTime.Add(70 * time.Second),
		},
	}

	if err := repo.IngestBatch(ctx, entries); err != nil {
		t.Fatalf("ingest failed: %v", err)
	}

	buckets, err := repo.GetHistogram(ctx, domainLogging.LogFilter{
		TenantID: "tenant-hist",
	}, 60)
	if err != nil {
		t.Fatalf("GetHistogram failed: %v", err)
	}

	if len(buckets) != 2 {
		t.Fatalf("expected 2 histogram buckets, got %d", len(buckets))
	}

	if buckets[0].TotalCount != 2 {
		t.Errorf("expected first bucket total 2, got %d", buckets[0].TotalCount)
	}
	if buckets[0].LevelCount["info"] != 1 || buckets[0].LevelCount["error"] != 1 {
		t.Errorf("unexpected level count in first bucket: %+v", buckets[0].LevelCount)
	}

	if buckets[1].TotalCount != 1 || buckets[1].LevelCount["info"] != 1 {
		t.Errorf("unexpected second bucket: %+v", buckets[1])
	}
}

func TestMemoryLogRepo_TailLogs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	repo := infraLogging.NewMemoryLogRepo(100)
	ch, err := repo.TailLogs(ctx, domainLogging.LogFilter{
		TenantID: "tenant-tail",
		LogLevel: domainLogging.LogLevelError,
	})
	if err != nil {
		t.Fatalf("TailLogs failed: %v", err)
	}

	go func() {
		time.Sleep(20 * time.Millisecond)
		_ = repo.IngestBatch(context.Background(), []domainLogging.LogEntry{
			{
				TenantID:  "tenant-tail",
				ClusterID: "c1",
				LogLevel:  domainLogging.LogLevelInfo,
				Message:   "ignore this",
			},
			{
				TenantID:  "tenant-tail",
				ClusterID: "c1",
				LogLevel:  domainLogging.LogLevelError,
				Message:   "critical failure event",
			},
		})
	}()

	select {
	case entry, ok := <-ch:
		if !ok {
			t.Fatal("channel closed unexpectedly")
		}
		if entry.Message != "critical failure event" {
			t.Errorf("expected critical failure event, got %s", entry.Message)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for tail log entry")
	}

	cancel()
	select {
	case _, ok := <-ch:
		if ok {
			for range ch {
			}
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("channel not closed after context cancel")
	}
}

func TestMemoryLogRepo_GetStatus(t *testing.T) {
	ctx := context.Background()
	repo := infraLogging.NewMemoryLogRepo(100)

	_ = repo.IngestBatch(ctx, []domainLogging.LogEntry{
		{
			TenantID:  "t1",
			ClusterID: "c1",
			LogLevel:  domainLogging.LogLevelInfo,
			Message:   "m1",
		},
		{
			TenantID:  "t1",
			ClusterID: "c1",
			LogLevel:  domainLogging.LogLevelWarn,
			Message:   "m2",
		},
	})

	st := repo.GetStatus()
	if st.Engine != "In-Memory RingBuffer (Fallback)" {
		t.Errorf("expected engine In-Memory RingBuffer (Fallback), got %s", st.Engine)
	}
	if st.Status != "fallback" {
		t.Errorf("expected status fallback, got %s", st.Status)
	}
	if st.TotalRecords != 2 {
		t.Errorf("expected total records 2, got %d", st.TotalRecords)
	}
	if st.RetentionDays <= 0 {
		t.Errorf("expected positive retention days, got %d", st.RetentionDays)
	}
}

func TestMemoryLogRepo_Concurrency(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	repo := infraLogging.NewMemoryLogRepo(200)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < 30; j++ {
				_ = repo.IngestBatch(ctx, []domainLogging.LogEntry{
					{
						TenantID:  fmt.Sprintf("tenant-%d", workerID%2),
						ClusterID: "c1",
						LogLevel:  domainLogging.LogLevelInfo,
						Message:   fmt.Sprintf("worker %d log %d", workerID, j),
						Timestamp: time.Now().UTC(),
					},
				})
				_, _ = repo.QueryLogs(ctx, domainLogging.LogFilter{
					TenantID: fmt.Sprintf("tenant-%d", workerID%2),
					Limit:    10,
				})
				_, _ = repo.GetHistogram(ctx, domainLogging.LogFilter{
					TenantID: fmt.Sprintf("tenant-%d", workerID%2),
				}, 60)
			}
		}(i)
	}

	wg.Wait()
	st := repo.GetStatus()
	if st.TotalRecords <= 0 {
		t.Errorf("expected > 0 records after concurrent ingestion, got %d", st.TotalRecords)
	}
}
