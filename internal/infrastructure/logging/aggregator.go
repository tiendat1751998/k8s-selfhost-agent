package logging

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Namespace string    `json:"namespace"`
	Pod       string    `json:"pod"`
	Container string    `json:"container"`
	Stream    string    `json:"stream"` // stdout, stderr
	Level     string    `json:"level"`  // INFO, WARN, ERROR, DEBUG
	Message   string    `json:"message"`
}

type RingBuffer struct {
	mu         sync.RWMutex
	capacity   int
	entries    []LogEntry
	head       int
	full       bool
	lastAccess time.Time
}

func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 1000
	}
	return &RingBuffer{
		capacity:   capacity,
		entries:    make([]LogEntry, capacity),
		lastAccess: time.Now(),
	}
}

func (rb *RingBuffer) Push(entry LogEntry) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.entries[rb.head] = entry
	rb.head = (rb.head + 1) % rb.capacity
	if rb.head == 0 {
		rb.full = true
	}
	rb.lastAccess = time.Now()
}

func (rb *RingBuffer) GetAll() []LogEntry {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if !rb.full {
		result := make([]LogEntry, rb.head)
		copy(result, rb.entries[:rb.head])
		return result
	}

	result := make([]LogEntry, rb.capacity)
	copy(result, rb.entries[rb.head:])
	copy(result[rb.capacity-rb.head:], rb.entries[:rb.head])
	return result
}

func (rb *RingBuffer) Prune(ttl time.Duration) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	cutoff := time.Now().Add(-ttl)
	var newEntries []LogEntry

	if !rb.full {
		for i := 0; i < rb.head; i++ {
			if rb.entries[i].Timestamp.After(cutoff) {
				newEntries = append(newEntries, rb.entries[i])
			}
		}
	} else {
		for i := 0; i < rb.capacity; i++ {
			idx := (rb.head + i) % rb.capacity
			if rb.entries[idx].Timestamp.After(cutoff) {
				newEntries = append(newEntries, rb.entries[idx])
			}
		}
	}

	for i := range rb.entries {
		rb.entries[i] = LogEntry{}
	}
	copy(rb.entries, newEntries)
	rb.head = len(newEntries)
	if rb.head == rb.capacity {
		rb.head = 0
		rb.full = true
	} else {
		rb.full = false
	}
}

type LogFilter struct {
	Namespace string
	Pod       string
	Container string
	Level     string
	Keyword   string
}

func (f *LogFilter) Matches(entry LogEntry) bool {
	if f.Namespace != "" && entry.Namespace != f.Namespace {
		return false
	}
	if f.Pod != "" && entry.Pod != f.Pod {
		return false
	}
	if f.Container != "" && entry.Container != f.Container {
		return false
	}
	if f.Level != "" && strings.ToUpper(entry.Level) != strings.ToUpper(f.Level) {
		return false
	}
	if f.Keyword != "" && !strings.Contains(strings.ToLower(entry.Message), strings.ToLower(f.Keyword)) {
		return false
	}
	return true
}

type Subscriber struct {
	ID     string
	Filter LogFilter
	Ch     chan LogEntry
}

type LogAggregator struct {
	mu          sync.RWMutex
	buffers     map[string]*RingBuffer
	subscribers map[string]*Subscriber
	bufCapacity int
	ttl         time.Duration
	maxBuffers  int
}

func NewLogAggregator(bufCapacity int) *LogAggregator {
	if bufCapacity <= 0 {
		bufCapacity = 2000
	}
	a := &LogAggregator{
		buffers:     make(map[string]*RingBuffer),
		subscribers: make(map[string]*Subscriber),
		bufCapacity: bufCapacity,
		ttl:         time.Hour,
		maxBuffers:  1000,
	}
	go a.cleanupLoop()
	return a
}

func (a *LogAggregator) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		a.cleanup()
	}
}

func (a *LogAggregator) cleanup() {
	a.mu.Lock()
	defer a.mu.Unlock()

	for k, buf := range a.buffers {
		buf.Prune(a.ttl)
		buf.mu.RLock()
		empty := !buf.full && buf.head == 0
		buf.mu.RUnlock()
		if empty {
			delete(a.buffers, k)
		}
	}

	for len(a.buffers) > a.maxBuffers {
		var oldestKey string
		var oldestTime time.Time
		first := true
		for k, buf := range a.buffers {
			buf.mu.RLock()
			la := buf.lastAccess
			buf.mu.RUnlock()
			if first || la.Before(oldestTime) {
				oldestTime = la
				oldestKey = k
				first = false
			}
		}
		if !first {
			delete(a.buffers, oldestKey)
		}
	}
}

func (a *LogAggregator) key(namespace, pod string) string {
	return fmt.Sprintf("%s/%s", namespace, pod)
}

func (a *LogAggregator) Ingest(entry LogEntry) {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}

	k := a.key(entry.Namespace, entry.Pod)

	a.mu.Lock()
	buf, exists := a.buffers[k]
	if !exists {
		buf = NewRingBuffer(a.bufCapacity)
		a.buffers[k] = buf
	}
	buf.Push(entry)
	a.mu.Unlock()

	// Broadcast to active matching subscribers non-blockingly
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, sub := range a.subscribers {
		if sub.Filter.Matches(entry) {
			select {
			case sub.Ch <- entry:
			default:
				// Skip if client buffer is congested to avoid blocking ingestion
			}
		}
	}
}

func (a *LogAggregator) Subscribe(subID string, filter LogFilter, chSize int) (*Subscriber, []LogEntry) {
	if chSize <= 0 {
		chSize = 500
	}
	sub := &Subscriber{
		ID:     subID,
		Filter: filter,
		Ch:     make(chan LogEntry, chSize),
	}

	a.mu.Lock()
	a.subscribers[subID] = sub
	var historical []LogEntry
	k := a.key(filter.Namespace, filter.Pod)
	if buf, exists := a.buffers[k]; exists {
		for _, e := range buf.GetAll() {
			if filter.Matches(e) {
				historical = append(historical, e)
			}
		}
	}
	a.mu.Unlock()

	return sub, historical
}

func (a *LogAggregator) Unsubscribe(subID string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if sub, exists := a.subscribers[subID]; exists {
		close(sub.Ch)
		delete(a.subscribers, subID)
	}
}
