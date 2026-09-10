package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/logging"
)

// BatchWriterConfig holds buffer and flush interval settings.
type BatchWriterConfig struct {
	BatchSize       int           `json:"batch_size"`
	FlushInterval   time.Duration `json:"flush_interval"`
	ChannelCapacity int           `json:"channel_capacity"`
	TableName       string        `json:"table_name"`
}

// DefaultBatchWriterConfig returns standard high-throughput parameters: 5000 logs or 5 seconds.
func DefaultBatchWriterConfig() BatchWriterConfig {
	return BatchWriterConfig{
		BatchSize:       5000,
		FlushInterval:   5 * time.Second,
		ChannelCapacity: 10000,
		TableName:       "cluster_logs",
	}
}

type flushRequest struct {
	errCh chan error
}

// BatchWriter manages asynchronous, zero-allocation batched writes to ClickHouse.
type BatchWriter struct {
	client       *Client
	cfg          BatchWriterConfig
	logCh        chan logging.LogEntry
	flushReqCh   chan flushRequest
	doneCh       chan struct{}
	closeOnce    sync.Once
	mu           sync.RWMutex
	closed       bool
	flushFn      func(ctx context.Context, batch []logging.LogEntry) error
	lastFlushErr error
}

// NewBatchWriter constructs a new background BatchWriter attached to a ClickHouse Client.
func NewBatchWriter(ctx context.Context, client *Client, cfg BatchWriterConfig) *BatchWriter {
	return NewBatchWriterWithFn(ctx, client, cfg, nil)
}

// NewBatchWriterWithFn allows injecting a custom flush function for zero-mock unit testing.
func NewBatchWriterWithFn(
	ctx context.Context,
	client *Client,
	cfg BatchWriterConfig,
	flushFn func(ctx context.Context, batch []logging.LogEntry) error,
) *BatchWriter {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 5000
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 5 * time.Second
	}
	if cfg.ChannelCapacity <= 0 {
		cfg.ChannelCapacity = 10000
	}
	if cfg.TableName == "" {
		cfg.TableName = "cluster_logs"
	}

	w := &BatchWriter{
		client:     client,
		cfg:        cfg,
		logCh:      make(chan logging.LogEntry, cfg.ChannelCapacity),
		flushReqCh: make(chan flushRequest),
		doneCh:     make(chan struct{}),
		flushFn:    flushFn,
	}

	go w.flusherLoop(ctx)

	return w
}

// Write enqueues a log entry into the channel buffer.
func (w *BatchWriter) Write(entry logging.LogEntry) error {
	w.mu.RLock()
	if w.closed {
		w.mu.RUnlock()
		return errors.New("batch writer is closed")
	}
	w.mu.RUnlock()

	select {
	case w.logCh <- entry:
		return nil
	default:
		return fmt.Errorf("%w: batch writer buffer channel is full", logging.ErrLogIngestionFailed)
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

// Flush synchronously forces an immediate batch write of all buffered items.
func (w *BatchWriter) Flush(ctx context.Context) error {
	w.mu.RLock()
	if w.closed {
		w.mu.RUnlock()
		return errors.New("batch writer is closed")
	}
	w.mu.RUnlock()

	req := flushRequest{errCh: make(chan error, 1)}
	select {
	case w.flushReqCh <- req:
	case <-ctx.Done():
		return ctx.Err()
	case <-w.doneCh:
		return errors.New("batch writer flusher already terminated")
	}

	select {
	case err := <-req.errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close gracefully closes the log channel, drains all items, and flushes them to ClickHouse.
func (w *BatchWriter) Close() error {
	w.closeOnce.Do(func() {
		w.mu.Lock()
		w.closed = true
		w.mu.Unlock()
		close(w.logCh)
	})

	<-w.doneCh

	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.lastFlushErr
}

// flusherLoop runs the background ticker and batch accumulator with zero slice reallocation.
func (w *BatchWriter) flusherLoop(ctx context.Context) {
	defer close(w.doneCh)

	ticker := time.NewTicker(w.cfg.FlushInterval)
	defer ticker.Stop()

	// Pre-allocate buffer to prevent dynamic heap reallocations during streaming
	buf := make([]logging.LogEntry, 0, w.cfg.BatchSize)

	for {
		select {
		case <-ctx.Done():
			// Context canceled: gracefully drain any pending items in channel
			buf = w.drainRemaining(buf)
			if len(buf) > 0 {
				flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				if err := w.executeFlush(flushCtx, buf); err != nil {
					w.recordFlushError(err)
				}
				cancel()
			}
			return

		case req := <-w.flushReqCh:
			var err error
			if len(buf) > 0 {
				err = w.executeFlush(ctx, buf)
				buf = buf[:0] // Zero-allocation reset
				if err != nil {
					w.recordFlushError(err)
				}
			}
			req.errCh <- err

		case entry, ok := <-w.logCh:
			if !ok {
				// Channel closed via Close(): flush remaining buffer
				if len(buf) > 0 {
					flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					if err := w.executeFlush(flushCtx, buf); err != nil {
						w.recordFlushError(err)
					}
					cancel()
					buf = buf[:0]
				}
				return
			}

			buf = append(buf, entry)
			if len(buf) >= w.cfg.BatchSize {
				if err := w.executeFlush(ctx, buf); err != nil {
					w.recordFlushError(err)
				}
				buf = buf[:0] // Zero-allocation reset
			}

		case <-ticker.C:
			if len(buf) > 0 {
				if err := w.executeFlush(ctx, buf); err != nil {
					w.recordFlushError(err)
				}
				buf = buf[:0] // Zero-allocation reset
			}
		}
	}
}

// drainRemaining collects any items left in the channel without blocking.
func (w *BatchWriter) drainRemaining(buf []logging.LogEntry) []logging.LogEntry {
	for {
		select {
		case entry, ok := <-w.logCh:
			if !ok {
				return buf
			}
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

// executeFlush writes the given entries using either the custom flush function or native ClickHouse batch insert.
func (w *BatchWriter) executeFlush(ctx context.Context, entries []logging.LogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	if w.flushFn != nil {
		return w.flushFn(ctx, entries)
	}

	if w.client == nil {
		return errors.New("clickhouse client is nil in batch writer")
	}

	conn, err := w.client.Conn(ctx)
	if err != nil {
		return fmt.Errorf("getting connection for batch insert: %w", err)
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (timestamp, tenant_id, cluster_id, namespace, pod_name, container_name, stream, log_level, message, attributes)",
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
		); appendErr != nil {
			return fmt.Errorf("appending entry to batch: %w", appendErr)
		}
	}

	if sendErr := batch.Send(); sendErr != nil {
		return fmt.Errorf("sending native batch insert: %w", sendErr)
	}

	return nil
}
