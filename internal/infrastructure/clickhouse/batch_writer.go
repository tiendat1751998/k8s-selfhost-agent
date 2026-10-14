package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/logging"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

// BatchConfig is an alias for BatchWriterConfig.
type BatchConfig = BatchWriterConfig

// BatchWriterConfig holds buffer and flush interval settings.
type BatchWriterConfig struct {
	BatchSize            int           `json:"batch_size"`
	FlushInterval        time.Duration `json:"flush_interval"`
	ChannelCapacity      int           `json:"channel_capacity"`
	BufferCap            int           `json:"buffer_cap,omitempty"`
	TableName            string        `json:"table_name"`
	RetryInitialInterval time.Duration `json:"retry_initial_interval,omitempty"`
}

// DefaultBatchWriterConfig returns standard high-throughput parameters: 5000 logs or 5 seconds.
func DefaultBatchWriterConfig() BatchWriterConfig {
	return BatchWriterConfig{
		BatchSize:            5000,
		FlushInterval:        5 * time.Second,
		ChannelCapacity:      10000,
		BufferCap:            10000,
		TableName:            "cluster_logs",
		RetryInitialInterval: 50 * time.Millisecond,
	}
}

var errChanPool = sync.Pool{
	New: func() any {
		return make(chan error, 1)
	},
}

type flushRequest struct {
	ctx   context.Context
	errCh chan error
}

// BatchWriterStats encapsulates runtime status metrics for the batch writer.
type BatchWriterStats struct {
	DroppedLogs int64 `json:"dropped_logs"`
	PendingLogs int   `json:"pending_logs"`
}

// BatchWriter manages asynchronous, zero-allocation batched writes to ClickHouse.
type BatchWriter struct {
	// Hot-path state padded to 64 bytes to eliminate false sharing / cache-line bouncing
	closed      atomic.Bool
	_           [63]byte
	droppedLogs atomic.Int64
	_           [56]byte
	retryCount  atomic.Int64
	_           [56]byte

	client       *Client
	cfg          BatchWriterConfig
	logCh        chan logging.LogEntry
	stopCh       chan struct{}
	flushReqCh   chan flushRequest
	doneCh       chan struct{}
	startOnce    sync.Once
	closeOnce    sync.Once
	started      atomic.Bool
	mu           sync.RWMutex
	flushFn      func(ctx context.Context, batch []logging.LogEntry) error
	lastFlushErr error
	buf          []logging.LogEntry
}

// NewBatchWriter constructs a new background BatchWriter attached to a ClickHouse Client.
func NewBatchWriter(client *Client, cfg BatchConfig) *BatchWriter {
	return NewBatchWriterWithFn(nil, client, cfg, nil)
}

// NewBatchWriterWithFn allows injecting a custom flush function for zero-mock unit testing.
func NewBatchWriterWithFn(
	ctx context.Context,
	client *Client,
	cfg BatchWriterConfig,
	flushFn func(ctx context.Context, batch []logging.LogEntry) error,
) *BatchWriter {
	if cfg.BatchSize <= 0 { cfg.BatchSize = 5000 }
	if cfg.FlushInterval <= 0 { cfg.FlushInterval = 5 * time.Second }
	if cfg.BufferCap > 0 && cfg.ChannelCapacity <= 0 { cfg.ChannelCapacity = cfg.BufferCap }
	if cfg.ChannelCapacity <= 0 { cfg.ChannelCapacity = 10000 }
	if cfg.BufferCap <= 0 { cfg.BufferCap = cfg.ChannelCapacity }
	if cfg.TableName == "" { cfg.TableName = "cluster_logs" }
	if cfg.RetryInitialInterval <= 0 { cfg.RetryInitialInterval = 50 * time.Millisecond }
	initialCap := cfg.BatchSize * 2
	if initialCap < 1024 { initialCap = 1024 }

	w := &BatchWriter{
		client:     client,
		cfg:        cfg,
		logCh:      make(chan logging.LogEntry, cfg.ChannelCapacity),
		stopCh:     make(chan struct{}),
		flushReqCh: make(chan flushRequest),
		doneCh:     make(chan struct{}),
		flushFn:    flushFn,
		buf:        make([]logging.LogEntry, 0, initialCap),
	}

	if ctx != nil {
		w.Start(ctx)
	}

	return w
}

// Start launches the background flusher loop if not already started.
func (w *BatchWriter) Start(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	w.startOnce.Do(func() {
		w.started.Store(true)
		go w.flusherLoop(ctx)
	})
}

// Stop gracefully shuts down the BatchWriter, flushing any pending logs.
func (w *BatchWriter) Stop() error {
	return w.Close()
}

// SetFlushFn overrides the flush execution function (useful for testing without real ClickHouse).
func (w *BatchWriter) SetFlushFn(fn func(ctx context.Context, batch []logging.LogEntry) error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.flushFn = fn
}

// DroppedLogs returns the total number of logs evicted due to buffer cap overflow.
func (w *BatchWriter) DroppedLogs() int64 {
	return w.droppedLogs.Load()
}

// RetryCount returns total retry operations performed.
func (w *BatchWriter) RetryCount() int64 {
	return w.retryCount.Load()
}

// Stats returns a snapshot of current dropped and pending log counters.
func (w *BatchWriter) Stats() BatchWriterStats {
	return BatchWriterStats{
		DroppedLogs: w.droppedLogs.Load(),
		PendingLogs: len(w.logCh),
	}
}

// Write enqueues a log entry into the channel buffer lock-free.
func (w *BatchWriter) Write(entry logging.LogEntry) error {
	if w.closed.Load() {
		return errors.New("batch writer is closed")
	}

	select {
	case w.logCh <- entry:
		return nil
	default:
		// Bound memory under channel pressure: evict oldest entry to prevent lockups.
		select {
		case <-w.logCh:
			dropped := w.droppedLogs.Add(1)
			if dropped == 1 || dropped%10000 == 0 {
				logger.Get().Warn("clickhouse batch writer channel overflow: dropping oldest log",
					zap.Int64("total_dropped", dropped),
					zap.Int("channel_capacity", cap(w.logCh)),
				)
			}
		default:
		}

		select {
		case w.logCh <- entry:
			return nil
		default:
			return fmt.Errorf("%w: batch writer buffer channel is full", logging.ErrLogIngestionFailed)
		}
	}
}

// WriteBatch enqueues multiple log entries sequentially.
func (w *BatchWriter) WriteBatch(entries []logging.LogEntry) error {
	for i := range entries {
		if err := w.Write(entries[i]); err != nil {
			return err
		}
	}
	return nil
}

// Flush synchronously drains pending channel items and forces an immediate batch write honoring caller ctx.
func (w *BatchWriter) Flush(ctx context.Context) error {
	if w.closed.Load() {
		return errors.New("batch writer is closed")
	}

	errCh := errChanPool.Get().(chan error)
	req := flushRequest{
		ctx:   ctx,
		errCh: errCh,
	}

	select {
	case w.flushReqCh <- req:
	case <-ctx.Done():
		errChanPool.Put(errCh)
		return ctx.Err()
	case <-w.doneCh:
		errChanPool.Put(errCh)
		return errors.New("batch writer flusher already terminated")
	}

	select {
	case err := <-errCh:
		errChanPool.Put(errCh)
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close gracefully marks writer closed via CAS, closes stopCh without closing logCh, and waits for final flush.
func (w *BatchWriter) Close() error {
	w.closeOnce.Do(func() {
		if w.closed.CompareAndSwap(false, true) {
			close(w.stopCh)
		}
	})

	if !w.started.Load() {
		return nil
	}

	<-w.doneCh

	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.lastFlushErr
}

// shutdownContext creates a bounded 5s shutdown context preserving tracing values without cancellation.
func shutdownContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		return context.WithTimeout(context.Background(), 5*time.Second)
	}
	return context.WithTimeout(context.WithoutCancel(parent), 5*time.Second)
}

// enforceBufferCap evicts oldest entries from buf if it exceeds the configured BufferCap.
func (w *BatchWriter) enforceBufferCap(buf []logging.LogEntry) []logging.LogEntry {
	maxCap := w.cfg.BufferCap
	if maxCap <= 0 {
		return buf
	}
	overflow := len(buf) - maxCap
	if overflow <= 0 {
		return buf
	}

	w.droppedLogs.Add(int64(overflow))
	logger.Get().Error("clickhouse batch writer buffer overflow: dropping oldest logs",
		zap.Int("evicted", overflow),
		zap.Int64("total_dropped", w.droppedLogs.Load()),
		zap.Int("buffer_cap", maxCap),
	)

	copy(buf, buf[overflow:])
	for i := len(buf) - overflow; i < len(buf); i++ {
		buf[i] = logging.LogEntry{}
	}
	return buf[:len(buf)-overflow]
}

// flushWithRetry retries executeFlush up to 3 attempts with exponential backoff and jitter.
func (w *BatchWriter) flushWithRetry(ctx context.Context, entries []logging.LogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	// Zero-allocation fast-path for non-failing flushes
	if err := w.executeFlush(ctx, entries); err == nil {
		return nil
	}

	w.retryCount.Add(1)

	exp := backoff.NewExponentialBackOff()
	exp.InitialInterval = w.cfg.RetryInitialInterval
	exp.Multiplier = 1.5
	exp.MaxInterval = 500 * time.Millisecond

	_, err := backoff.Retry(ctx, func() (struct{}, error) {
		w.retryCount.Add(1)
		if flushErr := w.executeFlush(ctx, entries); flushErr != nil {
			return struct{}{}, flushErr
		}
		return struct{}{}, nil
	}, backoff.WithBackOff(exp), backoff.WithMaxTries(3))

	return err
}

// flusherLoop runs the background ticker and batch accumulator with zero slice reallocation.
func (w *BatchWriter) flusherLoop(ctx context.Context) {
	defer close(w.doneCh)

	ticker := time.NewTicker(w.cfg.FlushInterval)
	defer ticker.Stop()

	buf := w.buf
	var lastFailure time.Time
	retryCooldown := w.cfg.RetryInitialInterval * 2

	for {
		select {
		case <-w.stopCh:
			buf = w.drainRemaining(buf)
			buf = w.enforceBufferCap(buf)
			if len(buf) > 0 {
				flushCtx, cancel := shutdownContext(ctx)
				err := w.flushWithRetry(flushCtx, buf)
				w.recordFlushError(err)
				cancel()
				if err == nil {
					buf = buf[:0]
				}
			}
			return

		case <-ctx.Done():
			buf = w.drainRemaining(buf)
			buf = w.enforceBufferCap(buf)
			if len(buf) > 0 {
				flushCtx, cancel := shutdownContext(ctx)
				err := w.flushWithRetry(flushCtx, buf)
				w.recordFlushError(err)
				cancel()
				if err == nil {
					buf = buf[:0]
				}
			}
			return

		case req := <-w.flushReqCh:
			buf = w.drainRemaining(buf)
			buf = w.enforceBufferCap(buf)
			var err error
			if len(buf) > 0 {
				err = w.flushWithRetry(req.ctx, buf)
				w.recordFlushError(err)
				if err == nil {
					buf = buf[:0]
					lastFailure = time.Time{}
				} else {
					lastFailure = time.Now()
				}
			}
			req.errCh <- err

		case entry := <-w.logCh:
			buf = append(buf, entry)
			buf = w.enforceBufferCap(buf)
			if len(buf) >= w.cfg.BatchSize {
				if lastFailure.IsZero() || time.Since(lastFailure) >= retryCooldown {
					err := w.flushWithRetry(ctx, buf)
					w.recordFlushError(err)
					if err == nil {
						buf = buf[:0]
						lastFailure = time.Time{}
					} else {
						lastFailure = time.Now()
					}
				}
			}

		case <-ticker.C:
			if len(buf) > 0 {
				if lastFailure.IsZero() || time.Since(lastFailure) >= retryCooldown {
					err := w.flushWithRetry(ctx, buf)
					w.recordFlushError(err)
					if err == nil {
						buf = buf[:0]
						lastFailure = time.Time{}
					} else {
						lastFailure = time.Now()
					}
				}
			}
		}
	}
}

// drainRemaining collects any items left in the channel without blocking.
func (w *BatchWriter) drainRemaining(buf []logging.LogEntry) []logging.LogEntry {
	for {
		select {
		case entry := <-w.logCh:
			buf = append(buf, entry)
		default:
			return buf
		}
	}
}

func (w *BatchWriter) recordFlushError(err error) {
	w.mu.Lock()
	w.lastFlushErr = err
	w.mu.Unlock()
}

// executeFlush writes entries using either injected flushFn or native ClickHouse batch insert.
func (w *BatchWriter) executeFlush(ctx context.Context, entries []logging.LogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	w.mu.RLock()
	fn := w.flushFn
	w.mu.RUnlock()

	if fn != nil {
		return fn(ctx, entries)
	}

	if w.client == nil {
		return errors.New("clickhouse client is nil in batch writer")
	}

	conn, err := w.client.Conn(ctx)
	if err != nil {
		return fmt.Errorf("getting connection for batch insert: %w", err)
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (timestamp, tenant_id, cluster_id, namespace, pod_name, container_name, stream, log_level, message, attributes, trace_id, span_id, error_fingerprint)",
		w.cfg.TableName,
	)

	batch, err := conn.PrepareBatch(ctx, query)
	if err != nil {
		return fmt.Errorf("preparing native batch insert: %w", err)
	}

	for i := range entries {
		entry := &entries[i]
		if appendErr := batch.Append(
			entry.Timestamp,
			entry.TenantID,
			entry.ClusterID,
			entry.Namespace,
			entry.PodName,
			entry.ContainerName,
			entry.Stream,
			string(entry.LogLevel),
			entry.Message,
			entry.Attributes,
			entry.TraceID,
			entry.SpanID,
			entry.ErrorFingerprint,
		); appendErr != nil {
			return fmt.Errorf("appending entry to batch: %w", appendErr)
		}
	}

	if sendErr := batch.Send(); sendErr != nil {
		return fmt.Errorf("sending native batch insert: %w", sendErr)
	}

	return nil
}
