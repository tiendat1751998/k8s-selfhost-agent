// Package nats provides JetStream telemetry ingestion, compression, and buffering.
package nats

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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

const binaryCodecMagic uint32 = 0x4B384A53

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
	bytesBufferPool = sync.Pool{
		New: func() any {
			return new(bytes.Buffer)
		},
	}
	decodeBufferPool = sync.Pool{
		New: func() any {
			b := make([]byte, 0, 16*1024)
			return &b
		},
	}
	recordSlicePool = sync.Pool{
		New: func() any {
			s := make([]TelemetryRecord, 0, 16)
			return &s
		},
	}

	nodeIDCacheMu sync.RWMutex
	nodeIDCache   = make(map[string]string, 4096)
)

func internNodeID(b []byte) string {
	nodeIDCacheMu.RLock()
	s, ok := nodeIDCache[string(b)]
	nodeIDCacheMu.RUnlock()
	if ok {
		return s
	}

	nodeIDCacheMu.Lock()
	if s, ok := nodeIDCache[string(b)]; ok {
		nodeIDCacheMu.Unlock()
		return s
	}
	s = string(b)
	nodeIDCache[s] = s
	nodeIDCacheMu.Unlock()
	return s
}

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

	buf := bytesBufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bytesBufferPool.Put(buf)

	// Binary Header: 4-byte magic + 4-byte count
	var header [8]byte
	binary.BigEndian.PutUint32(header[0:4], binaryCodecMagic)
	binary.BigEndian.PutUint32(header[4:8], uint32(len(p.records)))
	buf.Write(header[:])

	var recBuf [54]byte
	for _, r := range p.records {
		binary.BigEndian.PutUint16(recBuf[0:2], uint16(len(r.NodeID)))
		buf.Write(recBuf[0:2])
		buf.WriteString(r.NodeID)
		binary.BigEndian.PutUint64(recBuf[0:8], uint64(r.Timestamp))
		binary.BigEndian.PutUint64(recBuf[8:16], math.Float64bits(r.CPUUsagePct))
		binary.BigEndian.PutUint64(recBuf[16:24], math.Float64bits(r.MemoryUsagePct))
		binary.BigEndian.PutUint64(recBuf[24:32], math.Float64bits(r.DiskUsagePct))
		binary.BigEndian.PutUint64(recBuf[32:40], r.NetworkRxBytes)
		binary.BigEndian.PutUint64(recBuf[40:48], r.NetworkTxBytes)
		binary.BigEndian.PutUint32(recBuf[48:52], uint32(r.ActivePods))
		buf.Write(recBuf[0:52])
	}

	raw := buf.Bytes()

	enc := encoderPool.Get().(*zstd.Encoder)
	defer encoderPool.Put(enc)

	compressed := enc.EncodeAll(raw, make([]byte, 0, len(raw)/2))
	p.records = p.records[:0]
	return compressed, nil
}

func decompressBatchInto(compressed []byte, out *[]TelemetryRecord) error {
	if len(compressed) == 0 {
		*out = (*out)[:0]
		return nil
	}

	dec := decoderPool.Get().(*zstd.Decoder)
	defer decoderPool.Put(dec)

	bufPtr := decodeBufferPool.Get().(*[]byte)
	defer func() {
		*bufPtr = (*bufPtr)[:0]
		decodeBufferPool.Put(bufPtr)
	}()

	raw, err := dec.DecodeAll(compressed, (*bufPtr)[:0])
	if err != nil {
		return fmt.Errorf("decompressing zstd batch: %w", err)
	}

	if len(raw) >= 8 && binary.BigEndian.Uint32(raw[0:4]) == binaryCodecMagic {
		count := int(binary.BigEndian.Uint32(raw[4:8]))
		if cap(*out) < count {
			*out = make([]TelemetryRecord, count)
		} else {
			*out = (*out)[:count]
		}

		offset := 8
		for i := 0; i < count; i++ {
			if offset+2 > len(raw) {
				return errors.New("malformed binary batch: truncated nodeID len")
			}
			nodeIDLen := int(binary.BigEndian.Uint16(raw[offset : offset+2]))
			offset += 2

			if offset+nodeIDLen+52 > len(raw) {
				return errors.New("malformed binary batch: truncated record")
			}
			nodeID := internNodeID(raw[offset : offset+nodeIDLen])
			offset += nodeIDLen

			ts := int64(binary.BigEndian.Uint64(raw[offset : offset+8]))
			cpu := math.Float64frombits(binary.BigEndian.Uint64(raw[offset+8 : offset+16]))
			mem := math.Float64frombits(binary.BigEndian.Uint64(raw[offset+16 : offset+24]))
			disk := math.Float64frombits(binary.BigEndian.Uint64(raw[offset+24 : offset+32]))
			rx := binary.BigEndian.Uint64(raw[offset+32 : offset+40])
			tx := binary.BigEndian.Uint64(raw[offset+40 : offset+48])
			pods := int(int32(binary.BigEndian.Uint32(raw[offset+48 : offset+52])))
			offset += 52

			(*out)[i] = TelemetryRecord{
				NodeID:         nodeID,
				Timestamp:      ts,
				CPUUsagePct:    cpu,
				MemoryUsagePct: mem,
				DiskUsagePct:   disk,
				NetworkRxBytes: rx,
				NetworkTxBytes: tx,
				ActivePods:     pods,
			}
		}
		return nil
	}

	return json.Unmarshal(raw, out)
}

// DecompressBatch decodes a ZSTD compressed byte slice into telemetry records.
func DecompressBatch(compressed []byte) ([]TelemetryRecord, error) {
	if len(compressed) == 0 {
		return nil, nil
	}
	var batch []TelemetryRecord
	if err := decompressBatchInto(compressed, &batch); err != nil {
		return nil, err
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
	recPtr := recordSlicePool.Get().(*[]TelemetryRecord)
	*recPtr = (*recPtr)[:0]
	defer func() {
		*recPtr = (*recPtr)[:0]
		recordSlicePool.Put(recPtr)
	}()

	if err := decompressBatchInto(data, recPtr); err != nil {
		b.dropped.Add(1)
		return
	}

	flushCtx := b.ctx
	if flushCtx == nil {
		flushCtx = context.Background()
	}

	if err := b.sink.FlushBatch(flushCtx, *recPtr); err != nil {
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
