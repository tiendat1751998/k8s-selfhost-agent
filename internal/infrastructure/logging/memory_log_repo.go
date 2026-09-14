package logging

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	domainLogging "github.com/datdt/k8sselfhost/internal/domain/logging"
)

// EngineStatus reports metadata regarding the active log storage backend.
type EngineStatus struct {
	Engine        string  `json:"engine"`
	Status        string  `json:"status"`
	LatencyMS     float64 `json:"latency_ms"`
	TotalRecords  int64   `json:"total_records"`
	RetentionDays int     `json:"retention_days"`
}

type tailSubscriber struct {
	id     uint64
	filter domainLogging.LogFilter
	ch     chan domainLogging.LogEntry
}

// MemoryLogRepo implements domainLogging.LogRepository using a thread-safe in-memory ring buffer.
type MemoryLogRepo struct {
	mu          sync.RWMutex
	capacity    int
	entries     []domainLogging.LogEntry
	start       int
	count       int
	subscribers map[uint64]*tailSubscriber
	nextSubID   uint64
}

// NewMemoryLogRepo constructs a MemoryLogRepo with the specified ring buffer capacity.
func NewMemoryLogRepo(capacity int) *MemoryLogRepo {
	if capacity <= 0 {
		capacity = 5000
	}
	return &MemoryLogRepo{
		capacity:    capacity,
		entries:     make([]domainLogging.LogEntry, capacity),
		subscribers: make(map[uint64]*tailSubscriber),
	}
}

// IngestBatch inserts a slice of log entries into the ring buffer, overwriting the oldest when full.
func (r *MemoryLogRepo) IngestBatch(ctx context.Context, entries []domainLogging.LogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, entry := range entries {
		if entry.Timestamp.IsZero() {
			entry.Timestamp = time.Now().UTC()
		}

		if r.count < r.capacity {
			idx := (r.start + r.count) % r.capacity
			r.entries[idx] = entry
			r.count++
		} else {
			r.entries[r.start] = entry
			r.start = (r.start + 1) % r.capacity
		}

		// Non-blocking broadcast to active tail subscribers
		for _, sub := range r.subscribers {
			if matchesFilter(entry, sub.filter) {
				select {
				case sub.ch <- entry:
				default:
					// Congested subscriber, drop to prevent backpressure on ingestion
				}
			}
		}
	}

	return nil
}

// QueryLogs searches logs according to the specified filter, ordered newest first (timestamp DESC).
func (r *MemoryLogRepo) QueryLogs(ctx context.Context, filter domainLogging.LogFilter) (*domainLogging.LogSearchResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var matched []domainLogging.LogEntry
	for i := 0; i < r.count; i++ {
		idx := (r.start + i) % r.capacity
		e := r.entries[idx]
		if matchesFilter(e, filter) {
			matched = append(matched, e)
		}
	}

	// Sort newest first (timestamp DESC)
	sort.SliceStable(matched, func(i, j int) bool {
		return matched[i].Timestamp.After(matched[j].Timestamp)
	})

	totalCount := int64(len(matched))
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	var paged []domainLogging.LogEntry
	if offset < len(matched) {
		end := offset + limit
		if end > len(matched) {
			end = len(matched)
		}
		paged = make([]domainLogging.LogEntry, end-offset)
		copy(paged, matched[offset:end])
	} else {
		paged = []domainLogging.LogEntry{}
	}

	return &domainLogging.LogSearchResult{
		Entries:    paged,
		TotalCount: totalCount,
		HasMore:    offset+len(paged) < len(matched),
	}, nil
}

// GetHistogram aggregates log counts into discrete time interval buckets.
func (r *MemoryLogRepo) GetHistogram(
	ctx context.Context, filter domainLogging.LogFilter, intervalSeconds int,
) ([]domainLogging.LogAggregationBucket, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if intervalSeconds <= 0 {
		intervalSeconds = 60
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	bucketMap := make(map[time.Time]*domainLogging.LogAggregationBucket)
	var orderedTimes []time.Time

	for i := 0; i < r.count; i++ {
		idx := (r.start + i) % r.capacity
		e := r.entries[idx]
		if !matchesFilter(e, filter) {
			continue
		}

		sec := (e.Timestamp.Unix() / int64(intervalSeconds)) * int64(intervalSeconds)
		bucketTime := time.Unix(sec, 0).UTC()

		b, exists := bucketMap[bucketTime]
		if !exists {
			b = &domainLogging.LogAggregationBucket{
				TimeBucket: bucketTime,
				LevelCount: make(map[string]uint64),
			}
			bucketMap[bucketTime] = b
			orderedTimes = append(orderedTimes, bucketTime)
		}
		b.TotalCount++
		lvl := strings.ToLower(string(e.LogLevel))
		if lvl == "" {
			lvl = "info"
		}
		b.LevelCount[lvl]++
	}

	sort.Slice(orderedTimes, func(i, j int) bool {
		return orderedTimes[i].Before(orderedTimes[j])
	})

	result := make([]domainLogging.LogAggregationBucket, len(orderedTimes))
	for i, t := range orderedTimes {
		result[i] = *bucketMap[t]
	}

	return result, nil
}

// TailLogs registers a streaming channel that delivers matching log events in real time.
func (r *MemoryLogRepo) TailLogs(ctx context.Context, filter domainLogging.LogFilter) (<-chan domainLogging.LogEntry, error) {
	ch := make(chan domainLogging.LogEntry, 100)

	r.mu.Lock()
	r.nextSubID++
	subID := r.nextSubID
	sub := &tailSubscriber{
		id:     subID,
		filter: filter,
		ch:     ch,
	}
	r.subscribers[subID] = sub
	r.mu.Unlock()

	go func() {
		<-ctx.Done()
		r.mu.Lock()
		if existing, found := r.subscribers[subID]; found {
			delete(r.subscribers, subID)
			close(existing.ch)
		}
		r.mu.Unlock()
	}()

	return ch, nil
}

// GetStatus returns the current engine metadata and log buffer statistics.
func (r *MemoryLogRepo) GetStatus() EngineStatus {
	r.mu.RLock()
	total := r.count
	r.mu.RUnlock()

	return EngineStatus{
		Engine:        "In-Memory RingBuffer (Fallback)",
		Status:        "fallback",
		LatencyMS:     0.1,
		TotalRecords:  int64(total),
		RetentionDays: 30,
	}
}

func matchesFilter(entry domainLogging.LogEntry, f domainLogging.LogFilter) bool {
	if f.TenantID != "" && entry.TenantID != f.TenantID {
		return false
	}
	if f.ClusterID != "" && entry.ClusterID != f.ClusterID {
		return false
	}
	if f.Namespace != "" && entry.Namespace != f.Namespace {
		return false
	}
	if f.PodName != "" && entry.PodName != f.PodName {
		return false
	}
	if f.ContainerName != "" && entry.ContainerName != f.ContainerName {
		return false
	}
	if f.Stream != "" && entry.Stream != f.Stream {
		return false
	}
	if f.LogLevel != "" && !strings.EqualFold(string(entry.LogLevel), string(f.LogLevel)) {
		return false
	}
	if f.SearchText != "" && !strings.Contains(strings.ToLower(entry.Message), strings.ToLower(f.SearchText)) {
		return false
	}
	if !f.StartTime.IsZero() && entry.Timestamp.Before(f.StartTime) {
		return false
	}
	if !f.EndTime.IsZero() && entry.Timestamp.After(f.EndTime) {
		return false
	}
	if len(f.Attributes) > 0 {
		if entry.Attributes == nil {
			return false
		}
		for k, v := range f.Attributes {
			if entry.Attributes[k] != v {
				return false
			}
		}
	}
	return true
}
