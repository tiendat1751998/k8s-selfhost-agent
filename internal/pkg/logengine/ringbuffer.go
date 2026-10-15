package logengine

import (
	"context"
	"fmt"
	"runtime"
	"sync/atomic"
	"time"
)

const (
	// DefaultRingBufferCapacity is 65,536 slots (power of 2: 1 << 16).
	DefaultRingBufferCapacity = 65536
	// DefaultRingBufferMask is 65,535.
	DefaultRingBufferMask = DefaultRingBufferCapacity - 1
)

var spinSink int

func spinPause(n int) {
	var s int
	for i := 0; i < n; i++ {
		s += i
	}
	if s < 0 {
		spinSink = s
	}
}

func nextPowerOf2(n uint64) uint64 {
	if n <= 2 {
		return 2
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n |= n >> 32
	return n + 1
}

type ringSlot struct {
	sequence uint64
	entry    Entry
}

// RingBuffer is a lock-free Multi-Producer Single-Consumer (MPSC) ring buffer.
type RingBuffer struct {
	head     uint64
	tail     uint64
	dropped  uint64
	capacity uint64
	mask     uint64
	slots    []ringSlot
}

// NewRingBuffer allocates and pre-fills a lock-free MPSC ring buffer of the given capacity.
// Capacity is rounded up to the nearest power of 2.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = DefaultRingBufferCapacity
	}
	capPow2 := nextPowerOf2(uint64(capacity))

	rb := &RingBuffer{
		capacity: capPow2,
		mask:     capPow2 - 1,
		slots:    make([]ringSlot, capPow2),
	}

	for i := uint64(0); i < capPow2; i++ {
		rb.slots[i].sequence = i
	}

	return rb
}

// Capacity returns the maximum capacity of the ring buffer.
func (rb *RingBuffer) Capacity() int {
	return int(rb.capacity)
}

// Len returns the approximate number of entries in the buffer.
func (rb *RingBuffer) Len() int {
	head := atomic.LoadUint64(&rb.head)
	tail := atomic.LoadUint64(&rb.tail)
	if head >= tail {
		return int(head - tail)
	}
	return 0
}

// Dropped returns the total number of entries dropped due to backpressure / buffer full.
func (rb *RingBuffer) Dropped() uint64 {
	return atomic.LoadUint64(&rb.dropped)
}

// ResetDropped atomically resets the dropped counter to zero and returns the previous value.
func (rb *RingBuffer) ResetDropped() uint64 {
	return atomic.SwapUint64(&rb.dropped, 0)
}

// TryPush attempts to push an entry into the ring buffer without spinning.
// If the buffer is full, it atomically increments dropped counter and returns false.
// Guaranteed 0 B/op and 0 allocs/op on the hot path.
func (rb *RingBuffer) TryPush(entry Entry) bool {
	for {
		head := atomic.LoadUint64(&rb.head)
		slot := &rb.slots[head&rb.mask]
		seq := atomic.LoadUint64(&slot.sequence)
		diff := int64(seq) - int64(head)

		if diff == 0 {
			if atomic.CompareAndSwapUint64(&rb.head, head, head+1) {
				slot.entry = entry
				atomic.StoreUint64(&slot.sequence, head+1)
				return true
			}
			continue
		}

		if diff < 0 {
			// Buffer is full (head - tail >= capacity)
			atomic.AddUint64(&rb.dropped, 1)
			return false
		}

		// diff > 0: another producer claimed head, retry immediately
	}
}

// Push inserts an entry using a hybrid wait strategy:
// 100 iterations of CPU spin -> 50 iterations of runtime.Gosched().
// If the buffer is still full after backpressure iterations, it tracks drop and returns false.
// Guaranteed 0 B/op and 0 allocs/op on the hot path.
func (rb *RingBuffer) Push(entry Entry) bool {
	var spins int
	for {
		head := atomic.LoadUint64(&rb.head)
		slot := &rb.slots[head&rb.mask]
		seq := atomic.LoadUint64(&slot.sequence)
		diff := int64(seq) - int64(head)

		if diff == 0 {
			if atomic.CompareAndSwapUint64(&rb.head, head, head+1) {
				slot.entry = entry
				atomic.StoreUint64(&slot.sequence, head+1)
				return true
			}
			continue
		}

		if diff < 0 {
			if spins < 100 {
				spinPause(30)
				spins++
				continue
			}
			if spins < 150 {
				runtime.Gosched()
				spins++
				continue
			}
			// Buffer full after hybrid spin/yield
			atomic.AddUint64(&rb.dropped, 1)
			return false
		}

		// diff > 0: CAS race, retry
	}
}

// PushWait blocks until the entry can be pushed, or ctx is canceled.
func (rb *RingBuffer) PushWait(ctx context.Context, entry Entry) error {
	var spins int
	var timer *time.Timer
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		head := atomic.LoadUint64(&rb.head)
		slot := &rb.slots[head&rb.mask]
		seq := atomic.LoadUint64(&slot.sequence)
		diff := int64(seq) - int64(head)

		if diff == 0 {
			if atomic.CompareAndSwapUint64(&rb.head, head, head+1) {
				slot.entry = entry
				atomic.StoreUint64(&slot.sequence, head+1)
				return nil
			}
			continue
		}

		if diff < 0 {
			if spins < 100 {
				spinPause(30)
				spins++
				continue
			}
			if spins < 150 {
				runtime.Gosched()
				spins++
				continue
			}
			if timer == nil {
				timer = time.NewTimer(50 * time.Microsecond)
			} else {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(50 * time.Microsecond)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				spins = 0
				continue
			}
		}
	}
}

// DrainBatch drains up to len(dst) committed entries from the ring buffer into dst.
// Must be called exclusively by the single background consumer thread.
// Guaranteed 0 B/op and 0 allocs/op when dst is preallocated and reused.
func (rb *RingBuffer) DrainBatch(dst []Entry) int {
	tail := atomic.LoadUint64(&rb.tail)
	count := 0
	maxCount := len(dst)

	for count < maxCount {
		slot := &rb.slots[tail&rb.mask]
		seq := atomic.LoadUint64(&slot.sequence)
		diff := int64(seq) - int64(tail+1)

		if diff == 0 {
			dst[count] = slot.entry
			slot.entry = Entry{} // clear pointer references
			atomic.StoreUint64(&slot.sequence, tail+rb.capacity)
			tail++
			count++
		} else if diff < 0 {
			head := atomic.LoadUint64(&rb.head)
			if tail < head {
				// Producer is actively in flight writing this slot; hybrid spin wait
				committed := false
				for spin := 0; spin < 100; spin++ {
					spinPause(10)
					if atomic.LoadUint64(&slot.sequence) == tail+1 {
						committed = true
						break
					}
				}
				if !committed {
					for s := 0; s < 50; s++ {
						runtime.Gosched()
						if atomic.LoadUint64(&slot.sequence) == tail+1 {
							committed = true
							break
						}
					}
				}
				if committed {
					dst[count] = slot.entry
					slot.entry = Entry{}
					atomic.StoreUint64(&slot.sequence, tail+rb.capacity)
					tail++
					count++
					continue
				}
			}
			break
		} else {
			break
		}
	}

	if count > 0 {
		atomic.StoreUint64(&rb.tail, tail)
	}
	return count
}

// Pop extracts a single entry from the ring buffer (single consumer).
func (rb *RingBuffer) Pop() (Entry, bool) {
	tail := atomic.LoadUint64(&rb.tail)
	slot := &rb.slots[tail&rb.mask]
	seq := atomic.LoadUint64(&slot.sequence)
	diff := int64(seq) - int64(tail+1)

	if diff == 0 {
		entry := slot.entry
		slot.entry = Entry{}
		atomic.StoreUint64(&slot.sequence, tail+rb.capacity)
		atomic.StoreUint64(&rb.tail, tail+1)
		return entry, true
	}
	return Entry{}, false
}

// String returns a brief summary of the RingBuffer cursors for diagnostics.
func (rb *RingBuffer) String() string {
	head := atomic.LoadUint64(&rb.head)
	tail := atomic.LoadUint64(&rb.tail)
	dropped := atomic.LoadUint64(&rb.dropped)
	return fmt.Sprintf("RingBuffer[cap=%d, head=%d, tail=%d, len=%d, dropped=%d]",
		rb.capacity, head, tail, rb.Len(), dropped)
}
