package clickhouse_test

import (
	"context"
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