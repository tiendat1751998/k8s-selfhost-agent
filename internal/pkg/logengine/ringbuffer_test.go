package logengine

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRingBuffer_FieldLayout(t *testing.T) {
	var rb RingBuffer
	headOffset := unsafe.Offsetof(rb.head)
	tailOffset := unsafe.Offsetof(rb.tail)
	droppedOffset := unsafe.Offsetof(rb.dropped)

	t.Logf("head offset: %d, tail offset: %d, dropped offset: %d", headOffset, tailOffset, droppedOffset)

	// Verify compact layout without residual cache-line padding fields
	assert.Equal(t, uintptr(0), headOffset, "head offset must be 0")
	assert.Equal(t, uintptr(8), tailOffset, "tail offset must be 8")
	assert.Equal(t, uintptr(16), droppedOffset, "dropped offset must be 16")
}

func TestRingBuffer_MPSC_Concurrent(t *testing.T) {
	const capacity = DefaultRingBufferCapacity
	rb := NewRingBuffer(capacity)

	const numProducers = 8
	const itemsPerProducer = 10000
	const totalExpected = numProducers * itemsPerProducer

	var receivedCount int64
	doneConsumer := make(chan struct{})

	// Single consumer draining batches
	go func() {
		buf := make([]Entry, 512)
		for {
			n := rb.DrainBatch(buf)
			if n > 0 {
				atomic.AddInt64(&receivedCount, int64(n))
				if atomic.LoadInt64(&receivedCount) >= totalExpected {
					close(doneConsumer)
					return
				}
			} else {
				runtime.Gosched()
			}
		}
	}()

	// Multiple producers
	var wg sync.WaitGroup
	wg.Add(numProducers)
	startTime := time.Now()

	for p := 0; p < numProducers; p++ {
		producerID := p
		go func() {
			defer wg.Done()
			for i := 0; i < itemsPerProducer; i++ {
				entry := Entry{
					Timestamp: startTime,
					Service:   fmt.Sprintf("svc-%d", producerID),
					Level:     "info",
					Message:   fmt.Sprintf("item %d from producer %d", i, producerID),
				}
				for !rb.Push(entry) {
					runtime.Gosched()
				}
			}
		}()
	}

	wg.Wait()

	select {
	case <-doneConsumer:
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for consumer to receive all %d items (received %d)",
			totalExpected, atomic.LoadInt64(&receivedCount))
	}

	assert.Equal(t, int64(totalExpected), atomic.LoadInt64(&receivedCount))
	assert.Equal(t, uint64(0), rb.Dropped())
}

func TestRingBuffer_Backpressure_DropTracking(t *testing.T) {
	const capacity = 128
	rb := NewRingBuffer(capacity)

	// Fill buffer completely
	for i := 0; i < capacity; i++ {
		ok := rb.TryPush(Entry{
			Message: fmt.Sprintf("msg-%d", i),
		})
		require.True(t, ok, "should succeed until full")
	}

	// Next TryPush must fail and increment dropped
	ok := rb.TryPush(Entry{Message: "overflow"})
	assert.False(t, ok, "TryPush should fail when buffer is full")
	assert.Equal(t, uint64(1), rb.Dropped())

	// Drain half
	buf := make([]Entry, capacity/2)
	n := rb.DrainBatch(buf)
	assert.Equal(t, capacity/2, n)

	// Now TryPush should succeed again
	ok = rb.TryPush(Entry{Message: "recovered"})
	assert.True(t, ok, "TryPush should succeed after drain")
	assert.Equal(t, uint64(1), rb.Dropped())
}

func TestRingBuffer_CapacityWrapping(t *testing.T) {
	const capacity = 256
	rb := NewRingBuffer(capacity)

	const totalCycles = 1000
	drainBuf := make([]Entry, 32)

	for cycle := 0; cycle < totalCycles; cycle++ {
		for i := 0; i < 32; i++ {
			ok := rb.Push(Entry{
				Message: fmt.Sprintf("cycle-%d-item-%d", cycle, i),
			})
			require.True(t, ok)
		}

		n := rb.DrainBatch(drainBuf)
		require.Equal(t, 32, n)
		for i := 0; i < 32; i++ {
			expected := fmt.Sprintf("cycle-%d-item-%d", cycle, i)
			assert.Equal(t, expected, drainBuf[i].Message)
		}
	}
}

func BenchmarkRingBuffer_MPSC(b *testing.B) {
	rb := NewRingBuffer(DefaultRingBufferCapacity)
	entry := Entry{
		Timestamp: time.Now(),
		Service:   "bench-svc",
		Level:     "info",
		Message:   "benchmark ringbuffer high throughput message payload",
		Raw:       "raw log line",
	}

	done := make(chan struct{})
	go func() {
		buf := make([]Entry, 2048)
		for {
			select {
			case <-done:
				return
			default:
				n := rb.DrainBatch(buf)
				if n == 0 {
					runtime.Gosched()
				}
			}
		}
	}()

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for !rb.Push(entry) {
				runtime.Gosched()
			}
		}
	})

	b.StopTimer()
	close(done)
}

func TestRingBuffer_PushWait_BackpressureAndCancel(t *testing.T) {
	const capacity = 16
	rb := NewRingBuffer(capacity)

	// Fill buffer completely
	for i := 0; i < capacity; i++ {
		require.True(t, rb.Push(Entry{Message: fmt.Sprintf("fill-%d", i)}))
	}

	// PushWait with canceled context must abort promptly with ctx.Err()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Millisecond)
	defer cancel()

	err := rb.PushWait(ctx, Entry{Message: "overflow-blocked"})
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestRingBuffer_PushWait_SucceedsAfterDrain(t *testing.T) {
	const capacity = 16
	rb := NewRingBuffer(capacity)

	// Fill buffer completely
	for i := 0; i < capacity; i++ {
		require.True(t, rb.Push(Entry{Message: fmt.Sprintf("fill-%d", i)}))
	}

	// Drain in goroutine after short delay
	go func() {
		time.Sleep(5 * time.Millisecond)
		buf := make([]Entry, 4)
		_ = rb.DrainBatch(buf)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := rb.PushWait(ctx, Entry{Message: "unblocked-push"})
	assert.NoError(t, err)
}

