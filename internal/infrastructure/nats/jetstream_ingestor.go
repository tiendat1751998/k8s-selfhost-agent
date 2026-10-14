// Package nats provides JetStream telemetry ingestion, compression, and buffering.
package nats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/klauspost/compress/zstd"
)

var (
	// ErrQueueFull is returned when the telemetry backpressure buffer capacity is exceeded.
	ErrQueueFull = errors.New("telemetry buffer queue full: payload dropped")
	// ErrBufferStopped is returned when enqueuing to a stopped buffer.
	ErrBufferStopped = errors.New("telemetry buffer is stopped")
)

var (
	encoderPool = sync.Pool{
		New: func() any {
			enc, _ := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedFastest))
			return enc
		},
	}
	decoderPool = sync.Pool{
		New: func() any {
			dec, _ := zstd.NewReader(nil)
			return dec
		},
	}
)

// TelemetryRecord represents a single point-in-time snapshot of edge node metrics.
type TelemetryRecord struct {
	NodeID         string  `json:"node_id"`
	Timestamp      int64   `json:"timestamp"`
	CPUUsagePct    float64 `json:"cpu_usage_pct"`
	MemoryUsagePct float64 `json:"memory_usage_pct"`
	DiskUsagePct   float64 `json:"disk_usage_pct"`
	NetworkRxBytes uint64  `json:"network_rx_bytes"`
	NetworkTxBytes uint64  `json:"network_tx_bytes"`
	ActivePods     int     `json:"active_pods"`
}

// BatchSink consumes decompressed telemetry batches.
type BatchSink interface {
	FlushBatch(ctx context.Context, batch []TelemetryRecord) error
}

// MicroBatchPusher gathers telemetry payloads and emits compressed ZSTD micro-batches.
type MicroBatchPusher struct {
	mu      sync.Mutex
	records []TelemetryRecord
	maxSize int
}

// NewMicroBatchPusher creates a new pusher with preallocated capacity.
func NewMicroBatchPusher(maxBatchSize int) (*MicroBatchPusher, error) {
	if maxBatchSize <= 0 {
		maxBatchSize = 100
	}
	return &MicroBatchPusher{
		records: make([]TelemetryRecord, 0, maxBatchSize),
		maxSize: maxBatchSize,
	}, nil
}

// Add appends a telemetry record into the current micro-batch.
func (p *MicroBatchPusher) Add(record TelemetryRecord) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.records = append(p.records, record)
}

// CompressAndFlush serializes current batch, compresses using ZSTD, and resets internal slice.
func (p *MicroBatchPusher) CompressAndFlush() ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.records) == 0 {
		return nil, nil
	}

	raw, err := json.Marshal(p.records)
	if err != nil {
		return nil, fmt.Errorf("marshaling telemetry batch: %w", err)
	}

	enc := encoderPool.Get().(*zstd.Encoder)
	defer encoderPool.Put(enc)

	compressed := enc.EncodeAll(raw, make([]byte, 0, len(raw)/2))
	p.records = p.records[:0]
	return compressed, nil
}

// DecompressBatch decodes a ZSTD compressed byte slice into telemetry records.
func DecompressBatch(compressed []byte) ([]TelemetryRecord, error) {
	if len(compressed) == 0 {
		return nil, nil
	}

	dec := decoderPool.Get().(*zstd.Decoder)
	defer decoderPool.Put(dec)

	raw, err := dec.DecodeAll(compressed, nil)
	if err != nil {
		return nil, fmt.Errorf("decompressing zstd batch: %w", err)
	}

	var batch []TelemetryRecord
	if err := json.Unmarshal(raw, &batch); err != nil {
		return nil, fmt.Errorf("unmarshaling telemetry records: %w", err)
	}

	return batch, nil
}

// BufferConfig configures the backpressure telemetry buffer.
type BufferConfig struct {
	QueueCapacity int
	WorkerCount   int
	FlushTimeout  time.Duration
}

// JetStreamTelemetryBuffer coordinates concurrent ingestion workers with bounded queue backpressure.
type JetStreamTelemetryBuffer struct {
	cfg       BufferConfig
	sink      BatchSink
	queue     chan []byte
	processed atomic.Uint64
	dropped   atomic.Uint64
	running   atomic.Bool

	workerWg sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewJetStreamTelemetryBuffer initializes the buffer with backpressure safeguards.
func NewJetStreamTelemetryBuffer(cfg BufferConfig, sink BatchSink) (*JetStreamTelemetryBuffer, error) {
	if sink == nil {
		return nil, errors.New("batch sink cannot be nil")
	}
	if cfg.QueueCapacity <= 0 {
		cfg.QueueCapacity = 10000
	}
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = 4
	}
	if cfg.FlushTimeout <= 0 {
		cfg.FlushTimeout = 5 * time.Second
	}

	return &JetStreamTelemetryBuffer{
		cfg:   cfg,
		sink:  sink,
		queue: make(chan []byte, cfg.QueueCapacity),
	}, nil
}

// Start launches background decompression workers.
func (b *JetStreamTelemetryBuffer) Start(ctx context.Context) error {
	if b.running.Swap(true) {
		return errors.New("telemetry buffer already started")
	}

	b.ctx, b.cancel = context.WithCancel(ctx)

	for i := 0; i < b.cfg.WorkerCount; i++ {
		b.workerWg.Add(1)
		go b.workerLoop()
	}

	return nil
}

// Enqueue delivers compressed telemetry payload to the backpressure buffer.
func (b *JetStreamTelemetryBuffer) Enqueue(data []byte) error {
	if !b.running.Load() {
		return ErrBufferStopped
	}

	select {
	case b.queue <- data:
		return nil
	default:
		b.dropped.Add(1)
		return ErrQueueFull
	}
}

func (b *JetStreamTelemetryBuffer) workerLoop() {
	defer b.workerWg.Done()

	for {
		select {
		case <-b.ctx.Done():
			// Drain remaining in queue before exit
			b.drainQueue()
			return
		case data, ok := <-b.queue:
			if !ok {
				return
			}
			b.processItem(data)
		}
	}
}

func (b *JetStreamTelemetryBuffer) drainQueue() {
	for {
		select {
		case data, ok := <-b.queue:
			if !ok {
				return
			}
			b.processItem(data)
		default:
			return
		}
	}
}

func (b *JetStreamTelemetryBuffer) processItem(data []byte) {
	records, err := DecompressBatch(data)
	if err != nil {
		b.dropped.Add(1)
		return
	}

	flushCtx, flushCancel := context.WithTimeout(context.Background(), b.cfg.FlushTimeout)
	defer flushCancel()

	if err := b.sink.FlushBatch(flushCtx, records); err != nil {
		b.dropped.Add(1)
		return
	}

	b.processed.Add(1)
}

// Stop terminates worker pool and flushes remaining queue items.
func (b *JetStreamTelemetryBuffer) Stop() error {
	if !b.running.Swap(false) {
		return nil
	}

	b.cancel()
	b.workerWg.Wait()
	return nil
}

// ProcessedCount returns number of successfully processed batches.
func (b *JetStreamTelemetryBuffer) ProcessedCount() uint64 {
	return b.processed.Load()
}

// DroppedCount returns number of dropped batches due to overflow or failure.
func (b *JetStreamTelemetryBuffer) DroppedCount() uint64 {
	return b.dropped.Load()
}

// BoundedQueueLen returns current count of buffered items in backpressure channel.
func (b *JetStreamTelemetryBuffer) BoundedQueueLen() int {
	return len(b.queue)
}
