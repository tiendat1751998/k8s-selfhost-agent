package storage

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Ceph OSD lifecycle and health status constants.
const (
	OSDStatusUp           = "UP"
	OSDStatusDown         = "DOWN"
	OSDStatusCrashLooping = "CRASHLOOPING"

	RocksDBHealthOK         = "OK"
	RocksDBHealthCorrupted  = "CORRUPTED"
	RocksDBHealthRepairing  = "REPAIRING"
)

// MinIO scanner drive speed tuning levels.
// Tuning speed controls disk I/O duty cycle during background metadata crawling & bit-rot detection.
const (
	ScannerSpeedSlowest = "slowest"
	ScannerSpeedSlow    = "slow"
	ScannerSpeedDefault = "default"
	ScannerSpeedFast    = "fast"
	ScannerSpeedFastest = "fastest"
)

// Domain sentinel errors for storage resiliency and safeguards.
var (
	ErrPrimaryDatabaseProtected = errors.New("destructive operation strictly forbidden: target is primary PostgreSQL node (pg_is_in_recovery = false)")
	ErrCephOSDNotFound          = errors.New("ceph osd not found in cluster topology")
	ErrCorruptedWALDetected     = errors.New("corrupted RocksDB WAL detected in BlueStore metadata")
	ErrHealingQueueFull         = errors.New("healing queue capacity exceeded; request throttled")
	ErrInvalidClusterID         = errors.New("clusterID cannot be empty")
	ErrInvalidOSDID             = errors.New("osdID must be non-negative")
	ErrInvalidDBConnString      = errors.New("database connection string cannot be empty")
	ErrInvalidScannerSpeed      = errors.New("invalid MinIO drive scanner speed")
	ErrNegativeIOPS             = errors.New("active client IOPS cannot be negative")
)

/*
HARDWARE BOTTLENECK & FAILURE MODE DOCUMENTATION:

1. NVMe-oF Latency & PCIe Lane Saturation:
   - High-throughput all-flash Ceph clusters run NVMe-oF (RDMA/TCP) fabrics. Under PCIe Gen4 x4 lane limits
     (~7.88 GB/s bi-directional), simultaneous RocksDB WAL flushes and 4KB random client IOPS saturate PCIe queues,
     causing NVMe tail latencies (p99) to surge from 150µs to >20ms.

2. RocksDB Write-Amplification & fsync Storms:
   - BlueStore uses RocksDB to track allocation maps and object metadata. Sudden power loss without supercapacitors
     (power loss protection / PLP) causes torn writes in the write-ahead log (WAL). Upon restart, the OSD crashes with
     "Corruption: bad entry in WAL". Repair requires orchestrating `ceph-bluestore-tool repair` with PointInTimeRecovery.

3. Kernel Page Cache Thrashing & MinIO Scanner Lock Starvation:
   - MinIO background scanner traverses directory hierarchies to verify data integrity and heal bit-rot.
     Unthrottled crawlers evict the operating system page cache and hold read-write inode locks, starving client
     GET/PUT requests and elevating request queuing from <5ms to >5000ms. Dynamic speed switching resolves this.

4. PostgreSQL Primary Accident Wipe Safeguard (GitLab 2017 Incident Prevention):
   - In distributed PostgreSQL clusters (Patroni / physical streaming replication), an operator or automated agent
     must NEVER run destructive commands (pg_basebackup, rm -rf $PGDATA, node wipe) against a writable primary node.
     Guarded by mandatory check: `SELECT pg_is_in_recovery();` MUST return `true` before any destructive resync.
*/

// CephOSDStatus represents the operational and RocksDB health state of a Ceph Object Storage Daemon (OSD).
type CephOSDStatus struct {
	OSDID         int       `json:"osd_id"`
	Host          string    `json:"host"`
	Status        string    `json:"status"`         // UP, DOWN, CRASHLOOPING
	RocksDBHealth string    `json:"rocksdb_health"` // OK, CORRUPTED, REPAIRING
	LastCrashLog  string    `json:"last_crash_log,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// IsHealthy returns true if the OSD is currently UP and its RocksDB metadata is intact.
func (s *CephOSDStatus) IsHealthy() bool {
	return s.Status == OSDStatusUp && s.RocksDBHealth == RocksDBHealthOK
}

// RequiresRepair indicates whether an OSD suffers from crashlooping or metadata corruption requiring bluefs repair.
func (s *CephOSDStatus) RequiresRepair() bool {
	return s.Status == OSDStatusCrashLooping || s.RocksDBHealth == RocksDBHealthCorrupted
}

// Validate guards business invariants on CephOSDStatus.
func (s *CephOSDStatus) Validate() error {
	if s.OSDID < 0 {
		return ErrInvalidOSDID
	}
	if s.Host == "" {
		return errors.New("host cannot be empty")
	}
	switch s.Status {
	case OSDStatusUp, OSDStatusDown, OSDStatusCrashLooping:
	default:
		return fmt.Errorf("unknown ceph osd status: %s", s.Status)
	}
	switch s.RocksDBHealth {
	case RocksDBHealthOK, RocksDBHealthCorrupted, RocksDBHealthRepairing:
	default:
		return fmt.Errorf("unknown rocksdb health state: %s", s.RocksDBHealth)
	}
	return nil
}

// MinIOScannerTuning encapsulates active telemetry and autonomous drive scanner throttling.
type MinIOScannerTuning struct {
	CurrentSpeed     string    `json:"current_speed"` // slowest, slow, default, fast, fastest
	ActiveClientIOPS int64     `json:"active_client_iops"`
	HealQueueLength  int       `json:"heal_queue_length"`
	RecommendedSpeed string    `json:"recommended_speed"`
	AutoTunedAt      time.Time `json:"auto_tuned_at"`
}

// Validate verifies tuning telemetry parameters.
func (t *MinIOScannerTuning) Validate() error {
	if !isValidScannerSpeed(t.CurrentSpeed) {
		return fmt.Errorf("%w: current_speed '%s'", ErrInvalidScannerSpeed, t.CurrentSpeed)
	}
	if !isValidScannerSpeed(t.RecommendedSpeed) {
		return fmt.Errorf("%w: recommended_speed '%s'", ErrInvalidScannerSpeed, t.RecommendedSpeed)
	}
	if t.ActiveClientIOPS < 0 {
		return ErrNegativeIOPS
	}
	return nil
}

func isValidScannerSpeed(speed string) bool {
	switch speed {
	case ScannerSpeedSlowest, ScannerSpeedSlow, ScannerSpeedDefault, ScannerSpeedFast, ScannerSpeedFastest:
		return true
	default:
		return false
	}
}

// IOTracker maintains low-overhead, atomic I/O telemetry with 64-byte cache-line padding.
// Padding fields (_ [8]uint64, 8*8 = 64 bytes) eliminate CPU cache thrashing / false sharing
// between high-frequency client request workers and background scanner routines on multi-core NUMA sockets.
type IOTracker struct {
	_               [8]uint64 // 64-byte cache-line padding boundary
	clientIOPS      atomic.Int64
	_               [8]uint64 // 64-byte cache-line padding boundary
	waitingRequests atomic.Int64
	_               [8]uint64 // 64-byte cache-line padding boundary
	crawlerScans    atomic.Int64
	_               [8]uint64 // 64-byte cache-line padding boundary
}

// NewIOTracker initializes an isolated IOTracker.
func NewIOTracker() *IOTracker {
	return &IOTracker{}
}

// RecordClientIOPS atomically stores recent client IOPS measurements.
func (t *IOTracker) RecordClientIOPS(val int64) {
	t.clientIOPS.Store(val)
}

// GetClientIOPS reads current client IOPS atomically.
func (t *IOTracker) GetClientIOPS() int64 {
	return t.clientIOPS.Load()
}

// IncWaitingRequests increments requests waiting in backlog.
func (t *IOTracker) IncWaitingRequests() {
	t.waitingRequests.Add(1)
}

// DecWaitingRequests decrements requests waiting in backlog.
func (t *IOTracker) DecWaitingRequests() {
	t.waitingRequests.Add(-1)
}

// GetWaitingRequests reads current waiting request count.
func (t *IOTracker) GetWaitingRequests() int64 {
	return t.waitingRequests.Load()
}

// IncCrawlerScans tracks background crawler operations.
func (t *IOTracker) IncCrawlerScans() {
	t.crawlerScans.Add(1)
}

// GetCrawlerScans reads crawler execution count.
func (t *IOTracker) GetCrawlerScans() int64 {
	return t.crawlerScans.Load()
}

// HealTask defines a discrete background repair or scrub item.
type HealTask struct {
	ID        string    `json:"id"`
	ClusterID string    `json:"cluster_id"`
	Target    string    `json:"target"`
	Priority  int       `json:"priority"`
	CreatedAt time.Time `json:"created_at"`
}

// QueueMetrics holds telemetry on bounded queue behavior.
type QueueMetrics struct {
	Capacity  int    `json:"capacity"`
	Length    int    `json:"length"`
	Enqueued  uint64 `json:"enqueued"`
	Dequeued  uint64 `json:"dequeued"`
	Dropped   uint64 `json:"dropped"`
	Throttled uint64 `json:"throttled"`
}

// BoundedHealingQueue provides a thread-safe, memory-bounded task queue with drop/throttle metrics
// and 64-byte cache-line padding to prevent false sharing under high concurrency.
type BoundedHealingQueue struct {
	_              [8]uint64 // 64-byte cache-line padding
	mu             sync.Mutex
	tasks          []HealTask
	capacity       int
	_              [8]uint64 // 64-byte cache-line padding
	enqueuedCount  atomic.Uint64
	_              [8]uint64 // 64-byte cache-line padding
	dequeuedCount  atomic.Uint64
	_              [8]uint64 // 64-byte cache-line padding
	droppedCount   atomic.Uint64
	_              [8]uint64 // 64-byte cache-line padding
	throttledCount atomic.Uint64
	_              [8]uint64 // 64-byte cache-line padding
}

// NewBoundedHealingQueue constructs a new bounded healing queue with a fixed capacity limit.
func NewBoundedHealingQueue(capacity int) *BoundedHealingQueue {
	if capacity <= 0 {
		capacity = 1024
	}
	return &BoundedHealingQueue{
		tasks:    make([]HealTask, 0, capacity),
		capacity: capacity,
	}
}

// TryEnqueueOrDrop attempts to insert a healing task. If queue is full, increments drop & throttle metrics
// and returns ErrHealingQueueFull without unbounded heap allocation.
func (q *BoundedHealingQueue) TryEnqueueOrDrop(task HealTask) (bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.tasks) >= q.capacity {
		q.droppedCount.Add(1)
		q.throttledCount.Add(1)
		return false, ErrHealingQueueFull
	}

	q.tasks = append(q.tasks, task)
	q.enqueuedCount.Add(1)
	return true, nil
}

// Dequeue extracts the oldest pending task FIFO, returning false if empty.
func (q *BoundedHealingQueue) Dequeue() (HealTask, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.tasks) == 0 {
		return HealTask{}, false
	}

	task := q.tasks[0]
	q.tasks = q.tasks[1:]
	q.dequeuedCount.Add(1)
	return task, true
}

// Len returns current queue length.
func (q *BoundedHealingQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.tasks)
}

// Metrics provides snapshot telemetry of queue activity.
func (q *BoundedHealingQueue) Metrics() QueueMetrics {
	q.mu.Lock()
	length := len(q.tasks)
	capVal := q.capacity
	q.mu.Unlock()

	return QueueMetrics{
		Capacity:  capVal,
		Length:    length,
		Enqueued:  q.enqueuedCount.Load(),
		Dequeued:  q.dequeuedCount.Load(),
		Dropped:   q.droppedCount.Load(),
		Throttled: q.throttledCount.Load(),
	}
}

// StorageResiliencyManager specifies business contracts for autonomous storage healing and safeguards.
type StorageResiliencyManager interface {
	AuditCephCluster(ctx context.Context, clusterID string) ([]CephOSDStatus, error)
	TriggerCephOSDRepair(ctx context.Context, clusterID string, osdID int) error
	AutoTuneMinIOScanner(ctx context.Context, clusterID string) (*MinIOScannerTuning, error)
	VerifySafeDatabaseTarget(ctx context.Context, dbConnString string) (bool, error)
}
