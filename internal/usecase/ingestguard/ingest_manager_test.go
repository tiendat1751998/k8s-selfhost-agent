package ingestguard_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/domain/ingestguard"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/ingestguard"
)

type mockBulkWriter struct {
	mu           sync.Mutex
	flushBatches [][]any
	callCount    int32
}

func (m *mockBulkWriter) WriteBulk(ctx context.Context, database, table string, records []any) error {
	atomic.AddInt32(&m.callCount, 1)
	m.mu.Lock()
	defer m.mu.Unlock()
	batchCopy := make([]any, len(records))
	copy(batchCopy, records)
	m.flushBatches = append(m.flushBatches, batchCopy)
	return nil
}

type mockPartsInspector struct {
	parts           int64
	mergeInProgress bool
}

func (m *mockPartsInspector) InspectTableParts(ctx context.Context, database, table string) (int64, bool, error) {
	return m.parts, m.mergeInProgress, nil
}

type mockMergeAccelerator struct {
	scheduledCalls int32
}

func (m *mockMergeAccelerator) ScheduleMerge(ctx context.Context, database, table string) error {
	atomic.AddInt32(&m.scheduledCalls, 1)
	return nil
}

type mockSpillJournal struct {
	spilledBatches [][]any
	mu             sync.Mutex
}

func (m *mockSpillJournal) Spill(ctx context.Context, database, table string, records []any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]any, len(records))
	copy(cp, records)
	m.spilledBatches = append(m.spilledBatches, cp)
	return nil
}

func TestAdaptiveBatching_FlushesAtMaxBatchSize_NoSingleRow(t *testing.T) {
	writer := &mockBulkWriter{}
	cfg := ingestguard.BatchFlushConfig{
		MaxBatchSize:  10000,
		FlushInterval: 1 * time.Hour, // do not flush on timer
		BufferType:    ingestguard.BufferTypeRing,
	}

	mgr := usecase.NewIngestManager(cfg,
		usecase.WithBulkWriter(writer),
	)
	defer mgr.Close()

	ctx := context.Background()

	// Enqueue 9,999 records - should not trigger bulk flush yet
	for i := 0; i < 9999; i++ {
		err := mgr.EnqueueLog(ctx, "default", "cluster_logs", fmt.Sprintf("log-%d", i))
		require.NoError(t, err)
	}
	assert.Equal(t, int32(0), atomic.LoadInt32(&writer.callCount))

	// Enqueue 10,000th record - should immediately flush in one bulk chunk of 10,000 records
	err := mgr.EnqueueLog(ctx, "default", "cluster_logs", "log-9999")
	require.NoError(t, err)

	assert.Equal(t, int32(1), atomic.LoadInt32(&writer.callCount), "Must flush bulk chunk, zero single-row inserts")
	writer.mu.Lock()
	require.Len(t, writer.flushBatches, 1)
	assert.Len(t, writer.flushBatches[0], 10000)
	writer.mu.Unlock()
}

func TestAdaptiveBatching_FlushesOnIntervalOrManual(t *testing.T) {
	writer := &mockBulkWriter{}
	cfg := ingestguard.BatchFlushConfig{
		MaxBatchSize:  10000,
		FlushInterval: 50 * time.Millisecond,
		BufferType:    ingestguard.BufferTypeRing,
	}

	mgr := usecase.NewIngestManager(cfg,
		usecase.WithBulkWriter(writer),
	)
	defer mgr.Close()

	ctx := context.Background()

	for i := 0; i < 25; i++ {
		err := mgr.EnqueueLog(ctx, "default", "cluster_logs", fmt.Sprintf("item-%d", i))
		require.NoError(t, err)
	}

	// Manual flush pending batches
	err := mgr.FlushPendingBatches(ctx)
	require.NoError(t, err)

	assert.Equal(t, int32(1), atomic.LoadInt32(&writer.callCount))
	writer.mu.Lock()
	require.Len(t, writer.flushBatches, 1)
	assert.Len(t, writer.flushBatches[0], 25)
	writer.mu.Unlock()
}

func TestActivePartsWatcher_NormalThreshold(t *testing.T) {
	writer := &mockBulkWriter{}
	inspector := &mockPartsInspector{parts: 120, mergeInProgress: false}

	cfg := ingestguard.DefaultBatchFlushConfig()
	mgr := usecase.NewIngestManager(cfg,
		usecase.WithBulkWriter(writer),
		usecase.WithPartsInspector(inspector),
	)
	defer mgr.Close()

	ctx := context.Background()
	health, err := mgr.GetTablePartHealth(ctx, "cluster_logs")
	require.NoError(t, err)
	assert.Equal(t, ingestguard.HealthStatusHealthy, health.HealthStatus)
	assert.Equal(t, int64(120), health.ActiveParts)
	assert.False(t, health.ShouldThrottle())
	assert.False(t, health.IsSaturated())
}

func TestActivePartsWatcher_ThrottlesAt200Parts(t *testing.T) {
	writer := &mockBulkWriter{}
	inspector := &mockPartsInspector{parts: 220, mergeInProgress: false}
	accelerator := &mockMergeAccelerator{}

	cfg := ingestguard.DefaultBatchFlushConfig()
	mgr := usecase.NewIngestManager(cfg,
		usecase.WithBulkWriter(writer),
		usecase.WithPartsInspector(inspector),
		usecase.WithMergeAccelerator(accelerator),
	)
	defer mgr.Close()

	ctx := context.Background()
	health, err := mgr.GetTablePartHealth(ctx, "cluster_logs")
	require.NoError(t, err)
	assert.Equal(t, ingestguard.HealthStatusDegraded, health.HealthStatus)
	assert.True(t, health.ShouldThrottle())

	// Enqueuing should trigger merge acceleration schedule
	err = mgr.EnqueueLog(ctx, "default", "cluster_logs", "test-log")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, atomic.LoadInt32(&accelerator.scheduledCalls), int32(1))
}

func TestActivePartsWatcher_RejectsOrSpillsAt280Parts(t *testing.T) {
	writer := &mockBulkWriter{}
	inspector := &mockPartsInspector{parts: 285, mergeInProgress: true}

	cfg := ingestguard.DefaultBatchFlushConfig()

	// Case 1: Without journal, rejects with ErrClickHousePartsSaturation
	mgrWithoutJournal := usecase.NewIngestManager(cfg,
		usecase.WithBulkWriter(writer),
		usecase.WithPartsInspector(inspector),
	)
	defer mgrWithoutJournal.Close()

	ctx := context.Background()
	err := mgrWithoutJournal.EnqueueLog(ctx, "default", "cluster_logs", "test-record")
	assert.ErrorIs(t, err, ingestguard.ErrClickHousePartsSaturation)

	// Case 2: With journal, spills to NVMe disk journal
	journal := &mockSpillJournal{}
	mgrWithJournal := usecase.NewIngestManager(cfg,
		usecase.WithBulkWriter(writer),
		usecase.WithPartsInspector(inspector),
		usecase.WithSpillJournal(journal),
	)
	defer mgrWithJournal.Close()

	err = mgrWithJournal.EnqueueLog(ctx, "default", "cluster_logs", "test-record")
	require.NoError(t, err)

	journal.mu.Lock()
	assert.Len(t, journal.spilledBatches, 1)
	journal.mu.Unlock()
}

func TestOpenSearchSafeguard_Validation(t *testing.T) {
	cfg := ingestguard.DefaultBatchFlushConfig()
	mgr := usecase.NewIngestManager(cfg)
	defer mgr.Close()

	// Safe query
	safeDSL := `{
		"query": { "term": { "service": "api-gateway" } },
		"aggs": {
			"by_status": {
				"terms": { "field": "status", "size": 100 }
			}
		}
	}`
	valid, err := mgr.ValidateSearchQuery(safeDSL)
	require.NoError(t, err)
	assert.True(t, valid)

	// Unsafe query 1: Terms size > 10,000
	unsafeTermsDSL := `{
		"aggs": {
			"massive_terms": {
				"terms": { "field": "user_id", "size": 15000 }
			}
		}
	}`
	valid, err = mgr.ValidateSearchQuery(unsafeTermsDSL)
	assert.ErrorIs(t, err, ingestguard.ErrUnboundedAggQuery)
	assert.False(t, valid)

	// Unsafe query 2: Leading wildcard
	unsafeWildcardDSL := `{
		"query": {
			"wildcard": { "message": "*oom-killed*" }
		}
	}`
	valid, err = mgr.ValidateSearchQuery(unsafeWildcardDSL)
	assert.ErrorIs(t, err, ingestguard.ErrUnboundedAggQuery)
	assert.False(t, valid)

	// Unsafe query 3: Plain asterisk wildcard
	unsafeAsteriskDSL := `{
		"query": {
			"wildcard": { "message": "*" }
		}
	}`
	valid, err = mgr.ValidateSearchQuery(unsafeAsteriskDSL)
	assert.ErrorIs(t, err, ingestguard.ErrUnboundedAggQuery)
	assert.False(t, valid)

	// Invalid JSON DSL
	valid, err = mgr.ValidateSearchQuery(`{ invalid json `)
	assert.Error(t, err)
	assert.False(t, valid)
}

func TestConcurrentIngestion_CacheLinePaddedSafety(t *testing.T) {
	writer := &mockBulkWriter{}
	cfg := ingestguard.BatchFlushConfig{
		MaxBatchSize:  500,
		FlushInterval: 100 * time.Millisecond,
		BufferType:    ingestguard.BufferTypeRing,
	}

	mgr := usecase.NewIngestManager(cfg,
		usecase.WithBulkWriter(writer),
	)
	defer mgr.Close()

	ctx := context.Background()
	const numWorkers = 20
	const recordsPerWorker = 500

	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < recordsPerWorker; i++ {
				e := mgr.EnqueueLog(ctx, "default", "cluster_logs", fmt.Sprintf("worker-%d-item-%d", workerID, i))
				assert.NoError(t, e)
			}
		}(w)
	}

	wg.Wait()

	// Flush remaining records
	err := mgr.FlushPendingBatches(ctx)
	require.NoError(t, err)

	metrics := mgr.GetMetrics()
	assert.Equal(t, uint64(numWorkers*recordsPerWorker), metrics.EnqueuedTotal)
	assert.Equal(t, uint64(numWorkers*recordsPerWorker), metrics.FlushedTotal)
	assert.Equal(t, uint64(0), metrics.DroppedTotal)
}
