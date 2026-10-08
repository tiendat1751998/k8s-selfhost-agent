package clickhouse_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/domain/logging"
	"github.com/datdt/k8sselfhost/internal/infrastructure/clickhouse"
)

func TestBatchWriter_BatchSizeFlush(t *testing.T) {
	var flushedCount int64
	var flushCalls int64

	flushFn := func(ctx context.Context, batch []logging.LogEntry) error {
		atomic.AddInt64(&flushedCount, int64(len(batch)))
		atomic.AddInt64(&flushCalls, 1)
		return nil
	}

	cfg := clickhouse.BatchWriterConfig{
		BatchSize:       10,
		FlushInterval:   10 * time.Second, // Long interval, should trigger on count
		ChannelCapacity: 100,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	writer := clickhouse.NewBatchWriterWithFn(ctx, nil, cfg, flushFn)

	for i := 0; i < 25; i++ {
		err := writer.Write(logging.LogEntry{
			TenantID:  "tenant-test",
			ClusterID: "cluster-test",
			Message:   "log line",
		})
		require.NoError(t, err)
	}

	// Wait briefly for 2 batches of 10 to flush
	require.Eventually(t, func() bool {
		return atomic.LoadInt64(&flushedCount) >= 20
	}, 2*time.Second, 20*time.Millisecond)

	// Close should flush the remaining 5 entries gracefully
	err := writer.Close()
	require.NoError(t, err)

	require.Equal(t, int64(25), atomic.LoadInt64(&flushedCount))
	require.Equal(t, int64(3), atomic.LoadInt64(&flushCalls))
}

func TestBatchWriter_IntervalFlush(t *testing.T) {
	var flushedCount int64

	flushFn := func(ctx context.Context, batch []logging.LogEntry) error {
		atomic.AddInt64(&flushedCount, int64(len(batch)))
		return nil
	}

	cfg := clickhouse.BatchWriterConfig{
		BatchSize:       100, // High batch size, will flush on interval
		FlushInterval:   50 * time.Millisecond,
		ChannelCapacity: 100,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	writer := clickhouse.NewBatchWriterWithFn(ctx, nil, cfg, flushFn)

	err := writer.Write(logging.LogEntry{
		TenantID:  "t1",
		ClusterID: "c1",
		Message:   "interval log",
	})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return atomic.LoadInt64(&flushedCount) == 1
	}, 1*time.Second, 10*time.Millisecond)

	err = writer.Close()
	require.NoError(t, err)
}

func TestBatchWriter_FlushDrainsPendingAndClearsError(t *testing.T) {
	var flushedCount int64

	flushFn := func(ctx context.Context, batch []logging.LogEntry) error {
		atomic.AddInt64(&flushedCount, int64(len(batch)))
		return nil
	}

	cfg := clickhouse.BatchWriterConfig{
		BatchSize:       1000,
		FlushInterval:   10 * time.Second,
		ChannelCapacity: 100,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	writer := clickhouse.NewBatchWriterWithFn(ctx, nil, cfg, flushFn)

	for i := 0; i < 5; i++ {
		err := writer.Write(logging.LogEntry{
			TenantID:  "t1",
			ClusterID: "c1",
			Message:   "flush test",
		})
		require.NoError(t, err)
	}

	// Flush should drain the 5 items immediately
	flushCtx, flushCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer flushCancel()
	err := writer.Flush(flushCtx)
	require.NoError(t, err)

	require.Equal(t, int64(5), atomic.LoadInt64(&flushedCount))

	err = writer.Close()
	require.NoError(t, err)
}

func TestBatchWriter_ConcurrentWritesAndClose(t *testing.T) {
	var flushedCount int64

	flushFn := func(ctx context.Context, batch []logging.LogEntry) error {
		atomic.AddInt64(&flushedCount, int64(len(batch)))
		return nil
	}

	cfg := clickhouse.BatchWriterConfig{
		BatchSize:       50,
		FlushInterval:   50 * time.Millisecond,
		ChannelCapacity: 1000,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	writer := clickhouse.NewBatchWriterWithFn(ctx, nil, cfg, flushFn)

	const goroutines = 10
	const logsPerGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < logsPerGoroutine; i++ {
				writeErr := writer.Write(logging.LogEntry{
					TenantID:  "tenant-concurrent",
					ClusterID: "cluster-concurrent",
					Message:   "concurrent message",
				})
				if writeErr != nil {
					t.Errorf("write error: %v", writeErr)
				}
			}
		}()
	}

	wg.Wait()
	err := writer.Close()
	require.NoError(t, err)

	require.Equal(t, int64(goroutines*logsPerGoroutine), atomic.LoadInt64(&flushedCount))
}

func TestBatchWriter_ContextCancelGracefulFlush(t *testing.T) {
	var flushedCount int64

	flushFn := func(ctx context.Context, batch []logging.LogEntry) error {
		atomic.AddInt64(&flushedCount, int64(len(batch)))
		return nil
	}

	cfg := clickhouse.BatchWriterConfig{
		BatchSize:       100,
		FlushInterval:   10 * time.Second,
		ChannelCapacity: 100,
	}

	ctx, cancel := context.WithCancel(context.Background())

	writer := clickhouse.NewBatchWriterWithFn(ctx, nil, cfg, flushFn)

	for i := 0; i < 7; i++ {
		err := writer.Write(logging.LogEntry{
			TenantID:  "t1",
			ClusterID: "c1",
			Message:   "cancel test",
		})
		require.NoError(t, err)
	}

	// Cancel context - flusher should exit and flush remaining 7 items
	cancel()

	require.Eventually(t, func() bool {
		return atomic.LoadInt64(&flushedCount) == 7
	}, 2*time.Second, 20*time.Millisecond)

	err := writer.Close()
	require.NoError(t, err)
}

func TestBatchWriter_StartStopAndBufferCap(t *testing.T) {
	cfg := clickhouse.BatchConfig{
		BatchSize:     5000,
		FlushInterval: 2 * time.Second,
		BufferCap:     50000,
	}

	writer := clickhouse.NewBatchWriter(nil, cfg)
	require.NotNil(t, writer)

	var flushed int64
	writer.SetFlushFn(func(ctx context.Context, batch []logging.LogEntry) error {
		atomic.AddInt64(&flushed, int64(len(batch)))
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	writer.Start(ctx)

	err := writer.Write(logging.LogEntry{
		TenantID:  "tenant-1",
		ClusterID: "cluster-1",
		Message:   "batch writer test",
	})
	require.NoError(t, err)

	err = writer.Stop()
	require.NoError(t, err)
	require.Equal(t, int64(1), atomic.LoadInt64(&flushed))
}

func TestBatchWriter_CloseUnstarted(t *testing.T) {
	cfg := clickhouse.BatchWriterConfig{
		BatchSize:       10,
		FlushInterval:   10 * time.Second,
		ChannelCapacity: 100,
	}

	// Create writer without starting it (nil ctx)
	writer := clickhouse.NewBatchWriterWithFn(nil, nil, cfg, nil)

	done := make(chan struct{})
	go func() {
		defer close(done)
		err := writer.Close()
		require.NoError(t, err)
	}()

	select {
	case <-done:
		// Succeeded
	case <-time.After(1 * time.Second):
		t.Fatal("BatchWriter.Close() deadlocked on unstarted writer")
	}
}

func BenchmarkBatchWriter_Write(b *testing.B) {
	cfg := clickhouse.BatchWriterConfig{
		BatchSize:       1000,
		FlushInterval:   10 * time.Millisecond,
		ChannelCapacity: 1000000,
	}
	flushFn := func(ctx context.Context, batch []logging.LogEntry) error {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	writer := clickhouse.NewBatchWriterWithFn(ctx, nil, cfg, flushFn)
	defer writer.Close()

	entry := logging.LogEntry{
		TenantID:  "tenant-bench",
		ClusterID: "cluster-bench",
		Message:   "benchmark log line payload",
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = writer.Write(entry)
		if i%1000 == 0 {
			_ = writer.Flush(ctx)
		}
	}
}

func BenchmarkBatchWriter_FlushRetry(b *testing.B) {
	cfg := clickhouse.BatchWriterConfig{
		BatchSize:            10,
		FlushInterval:        10 * time.Second,
		ChannelCapacity:      10000,
		RetryInitialInterval: 1 * time.Millisecond,
	}
	var callCount atomic.Int64
	flushFn := func(ctx context.Context, batch []logging.LogEntry) error {
		if callCount.Add(1)%2 == 1 {
			return errors.New("transient flush failure")
		}
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	writer := clickhouse.NewBatchWriterWithFn(ctx, nil, cfg, flushFn)
	defer writer.Close()

	entry := logging.LogEntry{
		TenantID:  "tenant-bench",
		ClusterID: "cluster-bench",
		Message:   "benchmark retry payload",
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = writer.Write(entry)
		if i%10 == 0 {
			_ = writer.Flush(ctx)
		}
	}
}

func TestBatchWriter_FlushFailureRetriesZeroLoss(t *testing.T) {
	var collectedLogs []logging.LogEntry
	var mu sync.Mutex
	var attemptCount atomic.Int64

	// Fail first 2 attempts, succeed on the 3rd
	flushFn := func(ctx context.Context, batch []logging.LogEntry) error {
		attempt := attemptCount.Add(1)
		if attempt <= 2 {
			return errors.New("temporary clickhouse outage")
		}
		mu.Lock()
		collectedLogs = append(collectedLogs, batch...)
		mu.Unlock()
		return nil
	}

	cfg := clickhouse.BatchWriterConfig{
		BatchSize:       5,
		FlushInterval:   100 * time.Millisecond,
		ChannelCapacity: 50,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	writer := clickhouse.NewBatchWriterWithFn(ctx, nil, cfg, flushFn)

	for i := 0; i < 5; i++ {
		err := writer.Write(logging.LogEntry{
			TenantID:  "tenant-zero-loss",
			ClusterID: "cluster-zero-loss",
			Message:   "message zero loss",
		})
		require.NoError(t, err)
	}

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(collectedLogs) == 5
	}, 3*time.Second, 20*time.Millisecond)

	err := writer.Close()
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 5, len(collectedLogs))
	require.GreaterOrEqual(t, attemptCount.Load(), int64(3))
}

func TestBatchWriter_MemoryBoundedUnderPersistentFailure(t *testing.T) {
	persistentErr := errors.New("clickhouse connection refused")
	flushFn := func(ctx context.Context, batch []logging.LogEntry) error {
		return persistentErr
	}

	cfg := clickhouse.BatchWriterConfig{
		BatchSize:       5,
		FlushInterval:   30 * time.Millisecond,
		BufferCap:       10,
		ChannelCapacity: 100,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	writer := clickhouse.NewBatchWriterWithFn(ctx, nil, cfg, flushFn)

	// Write 25 entries. BufferCap is 10, so 15 should be evicted.
	for i := 0; i < 25; i++ {
		err := writer.Write(logging.LogEntry{
			TenantID:  "tenant-bound",
			ClusterID: "cluster-bound",
			Message:   "bounded log entry",
		})
		require.NoError(t, err)
	}

	require.Eventually(t, func() bool {
		return writer.DroppedLogs() == 15
	}, 3*time.Second, 20*time.Millisecond)

	err := writer.Close()
	require.Error(t, err)
	require.Equal(t, int64(15), writer.DroppedLogs())
}

func TestBatchWriter_ShutdownSurfacesFlushError(t *testing.T) {
	flushFn := func(ctx context.Context, batch []logging.LogEntry) error {
		return errors.New("clickhouse disk full")
	}

	cfg := clickhouse.BatchWriterConfig{
		BatchSize:       10,
		FlushInterval:   10 * time.Second,
		ChannelCapacity: 50,
	}

	ctx, cancel := context.WithCancel(context.Background())

	writer := clickhouse.NewBatchWriterWithFn(ctx, nil, cfg, flushFn)

	for i := 0; i < 5; i++ {
		err := writer.Write(logging.LogEntry{
			TenantID:  "tenant-shutdown",
			ClusterID: "cluster-shutdown",
			Message:   "shutdown log entry",
		})
		require.NoError(t, err)
	}

	cancel() // Cancel context to test graceful shutdown flush error capture
	err := writer.Close()
	require.Error(t, err)
	require.Contains(t, err.Error(), "clickhouse disk full")
}



