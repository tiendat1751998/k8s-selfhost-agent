// Package ingestguard implements the application usecase layer for TSDB ingestion safeguards,
// adaptive batch buffering, ClickHouse active parts backpressure, and OpenSearch query validation.
package ingestguard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/ingestguard"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

// BulkWriter defines the contract for persisting bulk batches into storage.
type BulkWriter interface {
	WriteBulk(ctx context.Context, database, table string, records []any) error
}

// PartsInspector abstracts inspecting active parts on ClickHouse MergeTree tables.
type PartsInspector interface {
	InspectTableParts(ctx context.Context, database, table string) (activeParts int64, mergeInProgress bool, err error)
}

// MergeAccelerator abstracts scheduling OPTIMIZE TABLE ... FINAL background merges.
type MergeAccelerator interface {
	ScheduleMerge(ctx context.Context, database, table string) error
}

// SpillJournal abstracts emergency spilling of records onto NVMe disk journal when parts saturate.
type SpillJournal interface {
	Spill(ctx context.Context, database, table string, records []any) error
}

// IngestMetricsSnapshot provides an atomic snapshot of ingestion throughput and safety counters.
type IngestMetricsSnapshot struct {
	EnqueuedTotal  uint64 `json:"enqueued_total"`
	FlushedTotal   uint64 `json:"flushed_total"`
	DroppedTotal   uint64 `json:"dropped_total"`
	SpilledTotal   uint64 `json:"spilled_total"`
	ThrottledTotal uint64 `json:"throttled_total"`
}

type tableBuffer struct {
	mu          sync.Mutex
	items       []any
	lastFlushAt time.Time
}

// IngestManagerOption configures IngestManager.
type IngestManagerOption func(*IngestManager)

// WithBulkWriter configures the bulk storage writer.
func WithBulkWriter(bw BulkWriter) IngestManagerOption {
	return func(m *IngestManager) { m.writer = bw }
}

// WithPartsInspector configures the ClickHouse parts inspector.
func WithPartsInspector(pi PartsInspector) IngestManagerOption {
	return func(m *IngestManager) { m.inspector = pi }
}

// WithMergeAccelerator configures the background merge accelerator.
func WithMergeAccelerator(ma MergeAccelerator) IngestManagerOption {
	return func(m *IngestManager) { m.accelerator = ma }
}

// WithSpillJournal configures the NVMe disk journal for emergency saturation spill.
func WithSpillJournal(sj SpillJournal) IngestManagerOption {
	return func(m *IngestManager) { m.journal = sj }
}

// IngestManager implements domain.AnalyticsIngestProtector.
type IngestManager struct {
	cfg         ingestguard.BatchFlushConfig
	writer      BulkWriter
	inspector   PartsInspector
	accelerator MergeAccelerator
	journal     SpillJournal
	logger      *zap.Logger

	// Cache-line padded counters (64 bytes each) eliminating MESI/MOESI false sharing
	enqueuedCount  ingestguard.PaddedCounter
	flushedCount   ingestguard.PaddedCounter
	droppedCount   ingestguard.PaddedCounter
	spilledCount   ingestguard.PaddedCounter
	throttledCount ingestguard.PaddedCounter

	bufferMu sync.RWMutex
	buffers  map[string]*tableBuffer

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewIngestManager constructs and initializes an IngestManager with background timer flusher.
func NewIngestManager(cfg ingestguard.BatchFlushConfig, opts ...IngestManagerOption) *IngestManager {
	if cfg.MaxBatchSize <= 0 {
		cfg.MaxBatchSize = ingestguard.DefaultMaxBatchSize
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = ingestguard.DefaultFlushInterval
	}
	if cfg.BufferType == "" {
		cfg.BufferType = ingestguard.BufferTypeRing
	}

	mgr := &IngestManager{
		cfg:     cfg,
		buffers: make(map[string]*tableBuffer),
		stopCh:  make(chan struct{}),
		logger:  logger.Get(),
	}

	for _, opt := range opts {
		opt(mgr)
	}

	mgr.wg.Add(1)
	go mgr.backgroundFlushLoop()

	return mgr
}

// Close stops background routines and flushes remaining batches.
func (m *IngestManager) Close() error {
	select {
	case <-m.stopCh:
		return nil
	default:
		close(m.stopCh)
	}
	m.wg.Wait()
	return m.FlushPendingBatches(context.Background())
}

// GetMetrics returns a point-in-time snapshot of padded metrics.
func (m *IngestManager) GetMetrics() IngestMetricsSnapshot {
	return IngestMetricsSnapshot{
		EnqueuedTotal:  m.enqueuedCount.Load(),
		FlushedTotal:   m.flushedCount.Load(),
		DroppedTotal:   m.droppedCount.Load(),
		SpilledTotal:   m.spilledCount.Load(),
		ThrottledTotal: m.throttledCount.Load(),
	}
}

func (m *IngestManager) getOrCreateBuffer(key string) *tableBuffer {
	m.bufferMu.RLock()
	buf, exists := m.buffers[key]
	m.bufferMu.RUnlock()
	if exists {
		return buf
	}

	m.bufferMu.Lock()
	defer m.bufferMu.Unlock()
	if buf, exists = m.buffers[key]; exists {
		return buf
	}
	buf = &tableBuffer{
		items:       make([]any, 0, m.cfg.MaxBatchSize),
		lastFlushAt: time.Now(),
	}
	m.buffers[key] = buf
	return buf
}

// GetTablePartHealth queries ClickHouse parts status and evaluates saturation limits.
func (m *IngestManager) GetTablePartHealth(ctx context.Context, table string) (*ingestguard.ClickHouseTableHealth, error) {
	if table == "" {
		table = "cluster_logs"
	}
	database := "default"
	var activeParts int64 = 0
	var mergeInProgress bool = false

	if m.inspector != nil {
		var err error
		activeParts, mergeInProgress, err = m.inspector.InspectTableParts(ctx, database, table)
		if err != nil {
			return nil, fmt.Errorf("inspecting table parts for %s: %w", table, err)
		}
	}

	health := ingestguard.NewClickHouseTableHealth(database, table, activeParts, mergeInProgress, time.Now())
	return &health, nil
}

// EnqueueLog validates table health and buffers a record for bulk insertion.
func (m *IngestManager) EnqueueLog(ctx context.Context, database, table string, record any) error {
	if table == "" {
		table = "cluster_logs"
	}
	if database == "" {
		database = "default"
	}

	// Step 1: Active parts watcher & backpressure check
	if m.inspector != nil {
		activeParts, _, err := m.inspector.InspectTableParts(ctx, database, table)
		if err == nil {
			if activeParts >= ingestguard.PartsSaturationThreshold {
				m.throttledCount.Inc()
				// Emergency saturation: parts >= 280. Spill to NVMe disk journal or reject with 503
				if m.journal != nil {
					m.spilledCount.Inc()
					return m.journal.Spill(ctx, database, table, []any{record})
				}
				m.droppedCount.Inc()
				return ingestguard.ErrClickHousePartsSaturation
			}

			if activeParts >= ingestguard.PartsThrottleThreshold {
				m.throttledCount.Inc()
				if m.accelerator != nil {
					_ = m.accelerator.ScheduleMerge(ctx, database, table)
				}
			}
		}
	}

	// Step 2: Adaptive batch buffer
	key := fmt.Sprintf("%s.%s", database, table)
	buf := m.getOrCreateBuffer(key)

	var toFlush []any
	buf.mu.Lock()
	buf.items = append(buf.items, record)
	m.enqueuedCount.Inc()

	if len(buf.items) >= m.cfg.MaxBatchSize {
		toFlush = buf.items
		buf.items = make([]any, 0, m.cfg.MaxBatchSize)
		buf.lastFlushAt = time.Now()
	}
	buf.mu.Unlock()

	// Zero single-row inserts: bulk flush only when batch threshold reached
	if len(toFlush) > 0 {
		return m.executeBulkFlush(ctx, database, table, toFlush)
	}

	return nil
}

func (m *IngestManager) executeBulkFlush(ctx context.Context, database, table string, records []any) error {
	if len(records) == 0 {
		return nil
	}
	if m.writer != nil {
		if err := m.writer.WriteBulk(ctx, database, table, records); err != nil {
			m.droppedCount.Add(uint64(len(records)))
			return fmt.Errorf("bulk writing %d records to %s.%s: %w", len(records), database, table, err)
		}
	}
	m.flushedCount.Add(uint64(len(records)))
	return nil
}

// FlushPendingBatches flushes all unwritten records across all table buffers.
func (m *IngestManager) FlushPendingBatches(ctx context.Context) error {
	m.bufferMu.RLock()
	tableKeys := make([]string, 0, len(m.buffers))
	for k := range m.buffers {
		tableKeys = append(tableKeys, k)
	}
	m.bufferMu.RUnlock()

	var errs []error
	for _, key := range tableKeys {
		buf := m.getOrCreateBuffer(key)
		buf.mu.Lock()
		if len(buf.items) == 0 {
			buf.mu.Unlock()
			continue
		}
		toFlush := buf.items
		buf.items = make([]any, 0, m.cfg.MaxBatchSize)
		buf.lastFlushAt = time.Now()
		buf.mu.Unlock()

		parts := strings.SplitN(key, ".", 2)
		db, tbl := parts[0], parts[1]
		if err := m.executeBulkFlush(ctx, db, tbl, toFlush); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (m *IngestManager) backgroundFlushLoop() {
	defer m.wg.Done()
	ticker := time.NewTicker(m.cfg.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			_ = m.flushTimedBatches()
		}
	}
}

func (m *IngestManager) flushTimedBatches() error {
	m.bufferMu.RLock()
	tableKeys := make([]string, 0, len(m.buffers))
	for k := range m.buffers {
		tableKeys = append(tableKeys, k)
	}
	m.bufferMu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, key := range tableKeys {
		buf := m.getOrCreateBuffer(key)
		buf.mu.Lock()
		if len(buf.items) == 0 {
			buf.mu.Unlock()
			continue
		}
		if time.Since(buf.lastFlushAt) >= m.cfg.FlushInterval {
			toFlush := buf.items
			buf.items = make([]any, 0, m.cfg.MaxBatchSize)
			buf.lastFlushAt = time.Now()
			buf.mu.Unlock()

			parts := strings.SplitN(key, ".", 2)
			_ = m.executeBulkFlush(ctx, parts[0], parts[1], toFlush)
			continue
		}
		buf.mu.Unlock()
	}
	return nil
}

// ValidateSearchQuery analyzes OpenSearch / Elasticsearch DSL to reject queries risking JVM OOM.
func (m *IngestManager) ValidateSearchQuery(queryDSL string) (bool, error) {
	trimmed := strings.TrimSpace(queryDSL)
	if trimmed == "" {
		return false, fmt.Errorf("%w: empty query DSL", ingestguard.ErrUnboundedAggQuery)
	}

	var dsl map[string]any
	if err := json.Unmarshal([]byte(trimmed), &dsl); err != nil {
		return false, fmt.Errorf("%w: invalid JSON syntax: %v", ingestguard.ErrUnboundedAggQuery, err)
	}

	if err := m.inspectQueryNode(dsl); err != nil {
		return false, err
	}

	return true, nil
}

func (m *IngestManager) inspectQueryNode(node any) error {
	switch v := node.(type) {
	case map[string]any:
		for key, child := range v {
			lowerKey := strings.ToLower(key)
			if lowerKey == "aggs" || lowerKey == "aggregations" {
				if err := m.inspectAggregations(child); err != nil {
					return err
				}
			}
			if lowerKey == "wildcard" {
				if err := m.inspectWildcard(child); err != nil {
					return err
				}
			}
			if err := m.inspectQueryNode(child); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range v {
			if err := m.inspectQueryNode(child); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *IngestManager) inspectAggregations(aggNode any) error {
	aggsMap, ok := aggNode.(map[string]any)
	if !ok {
		return nil
	}

	for _, body := range aggsMap {
		bodyMap, ok := body.(map[string]any)
		if !ok {
			continue
		}

		if termsVal, hasTerms := bodyMap["terms"]; hasTerms {
			if termsMap, ok := termsVal.(map[string]any); ok {
				if sizeVal, hasSize := termsMap["size"]; hasSize {
					sizeInt := toInt(sizeVal)
					if sizeInt > ingestguard.DefaultMaxAggTermsSize {
						return fmt.Errorf("%w: terms aggregation size %d exceeds limit of %d",
							ingestguard.ErrUnboundedAggQuery, sizeInt, ingestguard.DefaultMaxAggTermsSize)
					}
				}
			}
		}
	}
	return nil
}

func (m *IngestManager) inspectWildcard(wildcardNode any) error {
	switch wc := wildcardNode.(type) {
	case map[string]any:
		for _, val := range wc {
			switch strVal := val.(type) {
			case string:
				if isUnsafeWildcard(strVal) {
					return fmt.Errorf("%w: unsafe wildcard pattern '%s'", ingestguard.ErrUnboundedAggQuery, strVal)
				}
			case map[string]any:
				if v, ok := strVal["value"].(string); ok && isUnsafeWildcard(v) {
					return fmt.Errorf("%w: unsafe wildcard pattern '%s'", ingestguard.ErrUnboundedAggQuery, v)
				}
				if v, ok := strVal["wildcard"].(string); ok && isUnsafeWildcard(v) {
					return fmt.Errorf("%w: unsafe wildcard pattern '%s'", ingestguard.ErrUnboundedAggQuery, v)
				}
			}
		}
	case string:
		if isUnsafeWildcard(wc) {
			return fmt.Errorf("%w: unsafe wildcard pattern '%s'", ingestguard.ErrUnboundedAggQuery, wc)
		}
	}
	return nil
}

func isUnsafeWildcard(pattern string) bool {
	p := strings.TrimSpace(pattern)
	return p == "*" || p == "*.*" || strings.HasPrefix(p, "*") || strings.HasPrefix(p, "?")
}

func toInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	}
	return 0
}
