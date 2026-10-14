package nats

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockBatchSink implements BatchSink for testing.
type mockBatchSink struct {
	mu           sync.Mutex
	flushed      []TelemetryRecord
	flushCount   atomic.Int64
	latencyTotal atomic.Int64
}

func (m *mockBatchSink) FlushBatch(ctx context.Context, batch []TelemetryRecord) error {
	m.mu.Lock()
	m.flushed = append(m.flushed, batch...)
	m.mu.Unlock()
	m.flushCount.Add(1)
	return nil
}

func (m *mockBatchSink) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.flushed)
}

func sampleTelemetry(nodeID string) TelemetryRecord {
	return TelemetryRecord{
		NodeID:         nodeID,
		Timestamp:      time.Now().UnixNano(),
		CPUUsagePct:    42.5,
		MemoryUsagePct: 65.2,
		DiskUsagePct:   55.0,
		NetworkRxBytes: 1048576,
		NetworkTxBytes: 524288,
		ActivePods:     18,
	}
}

func TestMicroBatchPusher_Compression(t *testing.T) {
	pusher, err := NewMicroBatchPusher(10)
	if err != nil {
		t.Fatalf("failed to create MicroBatchPusher: %v", err)
	}

	// Add 5 telemetry records for 5 distinct nodes
	for i := 0; i < 5; i++ {
		pusher.Add(sampleTelemetry(fmt.Sprintf("node-%04d", i)))
	}

	compressed, err := pusher.CompressAndFlush()
	if err != nil {
		t.Fatalf("CompressAndFlush failed: %v", err)
	}

	if len(compressed) == 0 {
		t.Fatal("expected non-empty compressed payload")
	}

	// Acceptance criteria: < 1 KB per node batch (< 5 KB for 5 nodes)
	if len(compressed) > 1024*5 {
		t.Fatalf("compressed payload too large: got %d bytes, want <= %d bytes", len(compressed), 1024*5)
	}

	// Decompress and verify content
	decompressed, err := DecompressBatch(compressed)
	if err != nil {
		t.Fatalf("DecompressBatch failed: %v", err)
	}

	if len(decompressed) != 5 {
		t.Fatalf("expected 5 records decompressed, got %d", len(decompressed))
	}
	if decompressed[0].NodeID != "node-0000" {
		t.Fatalf("expected first node node-0000, got %s", decompressed[0].NodeID)
	}
}

func TestJetStreamTelemetryBuffer_ZeroLossAndBackpressure(t *testing.T) {
	sink := &mockBatchSink{}
	cfg := BufferConfig{
		QueueCapacity: 5000,
		WorkerCount:   4,
	}

	buffer, err := NewJetStreamTelemetryBuffer(cfg, sink)
	if err != nil {
		t.Fatalf("failed to create buffer: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := buffer.Start(ctx); err != nil {
		t.Fatalf("failed to start buffer: %v", err)
	}

	// Push 500 compressed batches concurrently
	var wg sync.WaitGroup
	numBatches := 500
	wg.Add(numBatches)

	for i := 0; i < numBatches; i++ {
		go func(id int) {
			defer wg.Done()
			pusher, _ := NewMicroBatchPusher(2)
			pusher.Add(sampleTelemetry(fmt.Sprintf("node-%04d", id)))
			data, err := pusher.CompressAndFlush()
			if err != nil {
				t.Errorf("compress error: %v", err)
				return
			}
			if err := buffer.Enqueue(data); err != nil {
				t.Errorf("enqueue error: %v", err)
			}
		}(i)
	}

	wg.Wait()

	// Wait for workers to flush
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if buffer.ProcessedCount() == uint64(numBatches) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err := buffer.Stop(); err != nil {
		t.Fatalf("buffer stop failed: %v", err)
	}

	if buffer.DroppedCount() != 0 {
		t.Fatalf("expected 0 drops, got %d", buffer.DroppedCount())
	}
	if buffer.ProcessedCount() != uint64(numBatches) {
		t.Fatalf("expected %d processed, got %d", numBatches, buffer.ProcessedCount())
	}
	if sink.Count() != numBatches {
		t.Fatalf("expected sink to receive %d records, got %d", numBatches, sink.Count())
	}
}

func BenchmarkJetStreamIngestor_2000Nodes(b *testing.B) {
	sink := &mockBatchSink{}
	cfg := BufferConfig{
		QueueCapacity: 20000,
		WorkerCount:   8,
	}

	buffer, err := NewJetStreamTelemetryBuffer(cfg, sink)
	if err != nil {
		b.Fatalf("failed to create buffer: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := buffer.Start(ctx); err != nil {
		b.Fatalf("failed to start buffer: %v", err)
	}
	defer buffer.Stop()

	// Pre-create 2,000 compressed payloads
	const nodeCount = 2000
	payloads := make([][]byte, nodeCount)
	for i := 0; i < nodeCount; i++ {
		p, _ := NewMicroBatchPusher(1)
		p.Add(sampleTelemetry(fmt.Sprintf("node-%04d", i)))
		data, _ := p.CompressAndFlush()
		payloads[i] = data
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		target := buffer.ProcessedCount() + uint64(nodeCount)
		start := time.Now()
		for j := 0; j < nodeCount; j++ {
			if err := buffer.Enqueue(payloads[j]); err != nil {
				b.Fatalf("enqueue failed: %v", err)
			}
		}

		// Wait until all 2,000 are processed
		deadline := time.Now().Add(5 * time.Second)
		for buffer.ProcessedCount() < target {
			if time.Now().After(deadline) {
				b.Fatalf("timeout waiting for 2000 nodes to process: got %d, want %d", buffer.ProcessedCount(), target)
			}
			time.Sleep(200 * time.Microsecond)
		}

		elapsed := time.Since(start)
		// Latency requirement: < 80ms for batch of 2000 nodes
		if elapsed > 80*time.Millisecond {
			b.Logf("Notice: 2000 nodes took %v (>80ms)", elapsed)
		}
	}
}
