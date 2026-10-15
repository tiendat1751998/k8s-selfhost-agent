// Package ingestguard provides domain abstractions, hardware-aware safeguards,
// and buffer contracts for high-throughput TSDB ingestion and log analytics.
//
// ==============================================================================================
// ENGINEERING NOTE: PHYSICAL HARDWARE BOTTLENECKS & REAL-WORLD FAILURE MODES
// ==============================================================================================
//
// 1. PHYSICAL HARDWARE BOTTLENECKS:
//   - NVMe IOPS & Bandwidth Saturation:
//     ClickHouse MergeTree tables continuously execute background merges in asynchronous worker
//     threads. When incoming insert batches are small or frequent, ClickHouse creates many small
//     immutable parts. If parts creation outpaces background merges, disk I/O saturates NVMe queues
//     (especially on shared cloud block storage like AWS EBS gp3 or GCP pd-balanced), driving write
//     latencies from microseconds to tens of seconds.
//   - Linux Page Cache Writeback Stalls:
//     Unbuffered high-volume bulk log writes cause dirty pages in the Linux kernel page cache to
//     rapidly cross vm.dirty_background_ratio and vm.dirty_ratio. Once vm.dirty_ratio is hit,
//     user-space ingestion goroutines are throttled directly by kernel flusher threads (kswapd/bdi),
//     introducing catastrophic tail latency spikes (100ms - 5s).
//   - OpenSearch JVM Heap Garbage Collection Pauses:
//     Unbounded JSON aggregations (such as terms aggregations with size > 10,000 or deep nested
//     bucketing) allocate millions of bucket tracking objects in JVM young/tenured generation.
//     Even with modern collectors (ZGC or G1GC), extreme heap allocation rates trigger concurrent
//     mark failures and stop-the-world (STW) full GC pauses, frequently tripping OpenSearch's
//     parent/fielddata circuit breakers (`circuit_breaking_exception`).
//   - CPU Cache Contention:
//     High-throughput ingestion pipelines parsing JSON strings into runtime structs cause extensive
//     L1/L2/L3 CPU cache misses. Concurrent worker goroutines updating shared counters
//     benefit from atomic operations to ensure consistent telemetry updates across processor cores.
//
// 2. REAL-WORLD FAILURE MODES & MITIGATIONS:
//   - ClickHouse `DB::Exception: Too many parts in all data parts in table` Crash:
//     Occurs when active parts exceed max_parts_in_total (default 300). Single-row inserts or tiny
//     batches will swiftly trigger this exception, abruptly terminating table ingestion.
//     Mitigation: Strict adaptive in-memory batching (10,000 records or 5s timer), parts count
//     monitoring from system.parts, proactive throttling at >= 200 parts, and emergency backpressure
//     spill/rejection at >= 280 parts.
//   - OpenSearch JVM OutOfMemoryError / Circuit Breaker Trip:
//     Caused by uncontrolled queries scanning raw high-cardinality log text or leading-wildcard queries
//     (`*error*`). Mitigation: Upfront AST inspection rejecting unbounded aggregation sizes (> 10,000)
//     and leading wildcards prior to cluster execution.
//   - Log Pipeline Backpressure Causing Pod Memory Bloat:
//     When TSDB/Search downstream slows down, buffering logs in unbound memory queues leads to Linux
//     OOMKill of agent pods. Mitigation: Bounded ring buffers with explicit 64-byte cache-line
//     padding and graceful NVMe disk journaling.
// ==============================================================================================
package ingestguard

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"
)

// Sentinel domain errors for TSDB and OpenSearch safeguards.
var (
	// ErrClickHousePartsSaturation is returned when active table parts exceed the safety ceiling (>= 280),
	// signaling upstream callers to back off or spill to disk (maps to HTTP 503 Service Unavailable / RFC 7807).
	ErrClickHousePartsSaturation = errors.New("clickhouse active parts saturated: ingestion throttled or rejected")

	// ErrUnboundedAggQuery is returned when an OpenSearch query DSL contains unsafe unbounded aggregations
	// or unindexed wildcard filters that risk JVM heap exhaustion (maps to HTTP 400 Bad Request).
	ErrUnboundedAggQuery = errors.New("unbounded aggregation query rejected: risk of openSearch heap exhaustion")

	// ErrBufferOverflow is returned when the adaptive batching ring buffer capacity is exceeded.
	ErrBufferOverflow = errors.New("adaptive batch buffer overflow: ring buffer full")
)

// Operational part threshold constants for ClickHouse MergeTree management.
const (
	PartsNormalThreshold     int64 = 150
	PartsThrottleThreshold   int64 = 200
	PartsSaturationThreshold int64 = 280
	PartsMaxHardLimit        int64 = 300

	DefaultMaxBatchSize        = 10000
	DefaultFlushInterval       = 5 * time.Second
	DefaultMaxAggTermsSize     = 10000
	DefaultRingBufferCapacity  = 65536
)

// BufferType defines the in-memory batching mechanism.
type BufferType string

const (
	BufferTypeRing    BufferType = "ring"
	BufferTypeChannel BufferType = "channel"
)

// BatchFlushConfig specifies adaptive batching settings for bulk ingestion.
type BatchFlushConfig struct {
	MaxBatchSize  int           `json:"max_batch_size"`
	FlushInterval time.Duration `json:"flush_interval"`
	BufferType    BufferType    `json:"buffer_type"`
}

// DefaultBatchFlushConfig returns production-hardened defaults: 10,000 records or 5 seconds.
func DefaultBatchFlushConfig() BatchFlushConfig {
	return BatchFlushConfig{
		MaxBatchSize:  DefaultMaxBatchSize,
		FlushInterval: DefaultFlushInterval,
		BufferType:    BufferTypeRing,
	}
}

// Validate checks configuration invariants.
func (c *BatchFlushConfig) Validate() error {
	if c.MaxBatchSize <= 0 {
		return errors.New("max_batch_size must be positive")
	}
	if c.FlushInterval <= 0 {
		return errors.New("flush_interval must be positive")
	}
	if c.BufferType != BufferTypeRing && c.BufferType != BufferTypeChannel {
		return fmt.Errorf("unsupported buffer_type '%s'", c.BufferType)
	}
	return nil
}

// HealthStatus categorizes table merge health.
type HealthStatus string

const (
	HealthStatusHealthy  HealthStatus = "healthy"
	HealthStatusDegraded HealthStatus = "degraded"
	HealthStatusCritical HealthStatus = "critical"
)

// ClickHouseTableHealth encapsulates runtime active parts and merge status for a specific table.
type ClickHouseTableHealth struct {
	Database        string       `json:"database"`
	Table           string       `json:"table"`
	ActiveParts     int64        `json:"active_parts"`
	MaxPartsLimit   int64        `json:"max_parts_limit"`
	MergeInProgress bool         `json:"merge_in_progress"`
	HealthStatus    HealthStatus `json:"health_status"`
	LastCheckedAt   time.Time    `json:"last_checked_at"`
}

// NewClickHouseTableHealth constructs and evaluates table health against physical part limits.
func NewClickHouseTableHealth(db, table string, activeParts int64, mergeInProgress bool, checkedAt time.Time) ClickHouseTableHealth {
	status := HealthStatusHealthy
	if activeParts >= PartsSaturationThreshold {
		status = HealthStatusCritical
	} else if activeParts >= PartsThrottleThreshold {
		status = HealthStatusDegraded
	}

	return ClickHouseTableHealth{
		Database:        db,
		Table:           table,
		ActiveParts:     activeParts,
		MaxPartsLimit:   PartsMaxHardLimit,
		MergeInProgress: mergeInProgress,
		HealthStatus:    status,
		LastCheckedAt:   checkedAt,
	}
}

// ShouldThrottle returns true when parts exceed the throttle threshold (>= 200).
func (h ClickHouseTableHealth) ShouldThrottle() bool {
	return h.ActiveParts >= PartsThrottleThreshold
}

// IsSaturated returns true when parts reach or exceed the saturation threshold (>= 280).
func (h ClickHouseTableHealth) IsSaturated() bool {
	return h.ActiveParts >= PartsSaturationThreshold
}

// PaddedCounter is an atomic uint64 counter.
type PaddedCounter struct {
	value uint64
}

// Load atomically retrieves the current counter value.
func (c *PaddedCounter) Load() uint64 {
	return atomic.LoadUint64(&c.value)
}

// Inc atomically increments the counter by 1.
func (c *PaddedCounter) Inc() uint64 {
	return atomic.AddUint64(&c.value, 1)
}

// Add atomically increments the counter by delta.
func (c *PaddedCounter) Add(delta uint64) uint64 {
	return atomic.AddUint64(&c.value, delta)
}

// Store atomically replaces the counter value.
func (c *PaddedCounter) Store(val uint64) {
	atomic.StoreUint64(&c.value, val)
}

// Reset atomically resets the counter to 0.
func (c *PaddedCounter) Reset() {
	atomic.StoreUint64(&c.value, 0)
}

// AnalyticsIngestProtector defines the contract for safe TSDB ingestion and query validation.
type AnalyticsIngestProtector interface {
	EnqueueLog(ctx context.Context, database, table string, record any) error
	FlushPendingBatches(ctx context.Context) error
	GetTablePartHealth(ctx context.Context, table string) (*ClickHouseTableHealth, error)
	ValidateSearchQuery(queryDSL string) (bool, error)
}
