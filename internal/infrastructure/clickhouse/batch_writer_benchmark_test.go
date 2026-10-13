package clickhouse_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/domain/logging"
	"github.com/datdt/k8sselfhost/internal/infrastructure/clickhouse"
)

func TestBatchWriter_ChannelOverflow_RateLimited(t *testing.T) {
	cfg := clickhouse.BatchWriterConfig{
		BatchSize:       10000,
		FlushInterval:   10 * time.Second,
		ChannelCapacity: 10,
	}
	// Unstarted flusher guarantees channel fills and stays full to test overflow throttling
	writer := clickhouse.NewBatchWriterWithFn(nil, nil, cfg, nil)
	defer writer.Close()

	entry := logging.LogEntry{
		TenantID:  "tenant-overflow",
		ClusterID: "cluster-overflow",
		Message:   "overflow log message",
	}

	// Channel capacity is 10. Write 2050 entries without draining.
	// Drops must be accurately counted and stdout locking prevented.
	for i := 0; i < 2050; i++ {
		_ = writer.Write(entry)
	}

	stats := writer.Stats()
	require.GreaterOrEqual(t, stats.DroppedLogs, int64(2000))
}

func BenchmarkBatchWriter_Enqueue(b *testing.B) {
	cfg := clickhouse.BatchWriterConfig{
		BatchSize:       10000,
		FlushInterval:   10 * time.Second,
		ChannelCapacity: 100,
		BufferCap:       1000000,
	}
	// Keep channel full to benchmark saturated channel overflow eviction throughput
	writer := clickhouse.NewBatchWriterWithFn(nil, nil, cfg, nil)
	defer writer.Close()

	entry := logging.LogEntry{
		TenantID:  "tenant-bench",
		ClusterID: "cluster-bench",
		Message:   "benchmark enqueue payload",
	}

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = writer.Write(entry)
		}
	})
}
