package dbsafeguard

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

// CONSTANTS & SYSTEM THRESHOLDS
const (
	// PostgresXIDEmergencyThreshold defines the XID age at which emergency autovacuum acceleration
	// and aggressive idle transaction termination are mandated (1.2 billion transactions).
	PostgresXIDEmergencyThreshold int64 = 1_200_000_000

	// PostgresXIDHardCeiling defines the absolute PostgreSQL transaction wraparound failure horizon
	// (2.0 billion transactions). Beyond this point, the PostgreSQL catalog risks data corruption and
	// Postgres forces an unskippable emergency read-only shutdown.
	PostgresXIDHardCeiling int64 = 2_000_000_000

	// AutovacuumCostLimitEmergency is the aggressive cost limit configured during XID freeze emergencies.
	// Default PostgreSQL autovacuum_cost_limit is 200; bumping to 2000 lifts disk throttling.
	AutovacuumCostLimitEmergency int64 = 2000

	// AutovacuumCostLimitDefault is the standard PostgreSQL autovacuum cost limit.
	AutovacuumCostLimitDefault int64 = 200

	// DefaultMaxIdleTxDuration defines the maximum allowed idle-in-transaction connection age (30 min).
	DefaultMaxIdleTxDuration time.Duration = 30 * time.Minute
)

// RFC7807ProblemDetail defines the RFC 7807 problem details specification for domain errors.
type RFC7807ProblemDetail struct {
	Type     string `json:"type,omitempty"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail"`
	Instance string `json:"instance,omitempty"`
	Code     string `json:"code"`
}

// Error implements the standard Go error interface.
func (p RFC7807ProblemDetail) Error() string {
	return fmt.Sprintf("[%s] %s: %s (status %d)", p.Code, p.Title, p.Detail, p.Status)
}

// STANDARD DOMAIN ERRORS (RFC 7807 COMPLIANT)
var (
	// ErrDBXIDWraparoundRisk indicates PostgreSQL transaction IDs are approaching wraparound horizon (> 1.2B).
	ErrDBXIDWraparoundRisk = &RFC7807ProblemDetail{
		Type:   "https://k8sselfhost.io/errors/ERR_DB_XID_WRAPAROUND_RISK",
		Title:  "PostgreSQL XID Wraparound Risk",
		Status: http.StatusInternalServerError, // 500
		Detail: "PostgreSQL XID age exceeded critical threshold (> 1.2 billion); emergency autovacuum and blocking transaction termination required",
		Code:   "ERR_DB_XID_WRAPAROUND_RISK",
	}

	// ErrKafkaPoisonPillDiverted indicates unprocessable message payload deserialization error was handled and diverted to DLQ.
	ErrKafkaPoisonPillDiverted = &RFC7807ProblemDetail{
		Type:   "https://k8sselfhost.io/errors/ERR_KAFKA_POISON_PILL_DIVERTED",
		Title:  "Kafka Poison Pill Diverted to DLQ",
		Status: http.StatusUnprocessableEntity, // 422
		Detail: "Unprocessable message payload deserialization error detected; diverted to dead letter queue (DLQ) to prevent consumer loop stall",
		Code:   "ERR_KAFKA_POISON_PILL_DIVERTED",
	}

	// ErrRedisResyncStormLoop indicates Redis replication backlog was depleted, provoking cyclic full resync fork storm.
	ErrRedisResyncStormLoop = &RFC7807ProblemDetail{
		Type:   "https://k8sselfhost.io/errors/ERR_REDIS_RESYNC_STORM_LOOP",
		Title:  "Redis Full Resync Storm Detected",
		Status: http.StatusInternalServerError, // 500
		Detail: "Replication backlog exceeded and replica output buffer overflowed, triggering cyclic BGSAVE fork storm",
		Code:   "ERR_REDIS_RESYNC_STORM_LOOP",
	}

	// ErrInvalidSafeguardInput is returned when input parameters violate domain validation invariants.
	ErrInvalidSafeguardInput = errors.New("invalid database safeguard input parameters")
)

// PostgresXIDHealth represents the transaction ID health and wraparound distance of a PostgreSQL database.
type PostgresXIDHealth struct {
	DatabaseName             string        `json:"database_name"`
	MaxXIDAge                int64         `json:"max_xid_age"`
	RemainingBeforeEmergency int64         `json:"remaining_before_emergency"`
	LongestIdleTx            time.Duration `json:"longest_idle_tx"`
	OldestTxPID              int           `json:"oldest_tx_pid"`
	IsEmergencyFreeze        bool          `json:"is_emergency_freeze"`
}

// NewPostgresXIDHealth creates and validates a PostgresXIDHealth domain entity.
func NewPostgresXIDHealth(
	databaseName string,
	maxXIDAge int64,
	longestIdleTx time.Duration,
	oldestTxPID int,
) (*PostgresXIDHealth, error) {
	if databaseName == "" {
		return nil, fmt.Errorf("%w: database name cannot be empty", ErrInvalidSafeguardInput)
	}
	if maxXIDAge < 0 {
		return nil, fmt.Errorf("%w: maxXIDAge cannot be negative", ErrInvalidSafeguardInput)
	}
	if longestIdleTx < 0 {
		longestIdleTx = 0
	}
	if oldestTxPID < 0 {
		oldestTxPID = 0
	}

	remaining := PostgresXIDEmergencyThreshold - maxXIDAge
	if remaining < 0 {
		remaining = 0
	}
	isEmergency := maxXIDAge >= PostgresXIDEmergencyThreshold

	return &PostgresXIDHealth{
		DatabaseName:             databaseName,
		MaxXIDAge:                maxXIDAge,
		RemainingBeforeEmergency: remaining,
		LongestIdleTx:            longestIdleTx,
		OldestTxPID:              oldestTxPID,
		IsEmergencyFreeze:        isEmergency,
	}, nil
}

// RedisReplicationHealth represents the replication health, backlog, and buffer status of a Redis cluster.
type RedisReplicationHealth struct {
	MasterHost           string `json:"master_host"`
	ConnectedReplicas    int    `json:"connected_replicas"`
	ReplBacklogSize      int64  `json:"repl_backlog_size"`
	ReplicationLagBytes  int64  `json:"replication_lag_bytes"`
	BufferOverflowCount  int64  `json:"buffer_overflow_count"`
	IsFullResyncLooping  bool   `json:"is_full_resync_looping"`
}

// NewRedisReplicationHealth creates and validates a RedisReplicationHealth domain entity.
func NewRedisReplicationHealth(
	masterHost string,
	connectedReplicas int,
	replBacklogSize int64,
	replicationLagBytes int64,
	bufferOverflowCount int64,
	isFullResyncLooping bool,
) (*RedisReplicationHealth, error) {
	if masterHost == "" {
		return nil, fmt.Errorf("%w: masterHost cannot be empty", ErrInvalidSafeguardInput)
	}
	if connectedReplicas < 0 {
		return nil, fmt.Errorf("%w: connectedReplicas cannot be negative", ErrInvalidSafeguardInput)
	}
	if replBacklogSize < 0 || replicationLagBytes < 0 || bufferOverflowCount < 0 {
		return nil, fmt.Errorf("%w: replication memory/lag metrics cannot be negative", ErrInvalidSafeguardInput)
	}

	return &RedisReplicationHealth{
		MasterHost:          masterHost,
		ConnectedReplicas:   connectedReplicas,
		ReplBacklogSize:     replBacklogSize,
		ReplicationLagBytes: replicationLagBytes,
		BufferOverflowCount: bufferOverflowCount,
		IsFullResyncLooping: isFullResyncLooping,
	}, nil
}

// KafkaPoisonPillAction represents a diverted poison pill event to Dead Letter Queue (DLQ).
type KafkaPoisonPillAction struct {
	Topic           string    `json:"topic"`
	Partition       int32     `json:"partition"`
	Offset          int64     `json:"offset"`
	ErrorReason     string    `json:"error_reason"`
	RedirectedToDLQ bool      `json:"redirected_to_dlq"`
	HandledAt       time.Time `json:"handled_at"`
}

// NewKafkaPoisonPillAction creates and validates a KafkaPoisonPillAction domain entity.
func NewKafkaPoisonPillAction(
	topic string,
	partition int32,
	offset int64,
	errorReason string,
	redirectedToDLQ bool,
) (*KafkaPoisonPillAction, error) {
	if topic == "" {
		return nil, fmt.Errorf("%w: topic cannot be empty", ErrInvalidSafeguardInput)
	}
	if partition < 0 {
		return nil, fmt.Errorf("%w: partition cannot be negative", ErrInvalidSafeguardInput)
	}
	if offset < 0 {
		return nil, fmt.Errorf("%w: offset cannot be negative", ErrInvalidSafeguardInput)
	}
	if errorReason == "" {
		return nil, fmt.Errorf("%w: errorReason cannot be empty", ErrInvalidSafeguardInput)
	}

	return &KafkaPoisonPillAction{
		Topic:           topic,
		Partition:       partition,
		Offset:          offset,
		ErrorReason:     errorReason,
		RedirectedToDLQ: redirectedToDLQ,
		HandledAt:       time.Now().UTC(),
	}, nil
}

// DatabaseResilienceEngine defines the domain port for database resilience and streaming safeguards.
type DatabaseResilienceEngine interface {
	CheckPostgresXIDHealth(ctx context.Context) (*PostgresXIDHealth, error)
	TerminateBlockingTransactions(ctx context.Context, maxAge time.Duration) error
	TuneAutovacuumAggressiveness(ctx context.Context) error
	AuditRedisReplicationBuffer(ctx context.Context) (*RedisReplicationHealth, error)
	HandleKafkaPoisonPill(ctx context.Context, topic string, partition int32, offset int64, payload []byte, err error) error
}

// =================================================================================================
// PHYSICAL HARDWARE BOTTLENECK & REAL-WORLD FAILURE ANALYSIS
// =================================================================================================
//
// 1. PostgreSQL WAL Write Amplification During Aggressive Vacuuming:
//    When autovacuum accelerates from normal throttling (autovacuum_cost_limit = 200) to emergency mode
//    (autovacuum_cost_limit = 2000), vacuum workers scan heap pages without sleep penalties. In tables with
//    heavy update churn, every page with frozen tuple headers produces full-page writes (FPWs) into the
//    PostgreSQL Write-Ahead Log (WAL) if modified after the last checkpoint. This can amplify NVMe/SSD write
//    I/O by 5x-10x, competing with live OLTP transactions and saturating storage IOPS.
//
// 2. Linux Copy-on-Write (CoW) Memory Doubling During Redis bGSAVE fork():
//    When Redis initiates snapshotting via fork() for replica resynchronization, the child process shares
//    memory pages with the parent using kernel page table copy-on-write. Under heavy write workloads,
//    page modifications force the kernel to allocate new 4KB physical pages. If write throughput is elevated,
//    memory usage can double (2x resident set size [RSS]), causing the Linux Out-Of-Memory (OOM) killer to
//    terminate the Redis master node, taking down the entire cache tier.
//
// 3. Redis client-output-buffer-limit Replica Socket Truncation:
//    When a replica lags behind the master's write rate, unconsumed commands accumulate in the replica's
//    output buffer. If this buffer exceeds `client-output-buffer-limit replica <hard_limit>` or remains above
//    `<soft_limit>` for the grace period, the master abruptly disconnects the replica socket. The replica
//    reconnects, cannot perform partial resynchronization (PSYNC) because the backlog was overtaken, and
//    triggers another BGSAVE full resync (RDB fork) — entering an infinite, destructive resync storm.
//
// 4. Kafka Consumer Group Rebalance Storms During Unhandled Message Deserialization Errors:
//    When a consumer encounters a malformed payload (poison pill), crashing or blocking indefinitely on the
//    same partition causes heartbeat timeouts. The Kafka coordinator detects consumer failure and triggers a
//    group rebalance. Upon restart, the consumer reads the exact same uncommitted poisoned message, crashes
//    again, and initiates an infinite restart-rebalance cascade, halting data processing across the cluster.
//
// 5. PostgreSQL 32-bit XID Wraparound Catastrophic Read-Only Shutdown:
//    PostgreSQL transaction IDs (XIDs) are 32-bit circular integers (modulo 2^32 = ~4.29 billion). To maintain
//    MVCC visibility comparisons (`xmin < xmax`), older transactions are compared relative to a moving 2-billion
//    horizon. If a long-running transaction or abandoned replication slot prevents `vacuum freeze` from advancing
//    `datfrozenxid` past 2 billion transactions, PostgreSQL halts all write operations and forces an emergency
//    read-only shutdown to prevent data corruption.
//
// DBSafeguardTelemetryTracker provides atomic telemetry tracking for database safeguards.
type DBSafeguardTelemetryTracker struct {
	maxXIDAgeObserved     atomic.Int64
	idleTxTerminated      atomic.Uint64
	activeTxPreserved     atomic.Uint64
	autovacuumCostLimit   atomic.Int64
	redisBufferOverflows  atomic.Uint64
	redisBacklogBytes     atomic.Int64
	kafkaPoisonPillsDLQ   atomic.Uint64
	kafkaOffsetsCommitted atomic.Uint64
}

// NewDBSafeguardTelemetryTracker initializes atomic telemetry counters with default cost limit.
func NewDBSafeguardTelemetryTracker() *DBSafeguardTelemetryTracker {
	tracker := &DBSafeguardTelemetryTracker{}
	tracker.autovacuumCostLimit.Store(AutovacuumCostLimitDefault)
	return tracker
}

// RecordXIDAge updates the maximum observed XID age atomically.
func (t *DBSafeguardTelemetryTracker) RecordXIDAge(age int64) {
	current := t.maxXIDAgeObserved.Load()
	for age > current {
		if t.maxXIDAgeObserved.CompareAndSwap(current, age) {
			break
		}
		current = t.maxXIDAgeObserved.Load()
	}
}

// IncIdleTxTerminated increments the count of killed idle-in-transaction connections.
func (t *DBSafeguardTelemetryTracker) IncIdleTxTerminated() {
	t.idleTxTerminated.Add(1)
}

// IncActiveTxPreserved increments the counter of preserved active queries.
func (t *DBSafeguardTelemetryTracker) IncActiveTxPreserved() {
	t.activeTxPreserved.Add(1)
}

// SetAutovacuumCostLimit updates the current autovacuum cost limit setting.
func (t *DBSafeguardTelemetryTracker) SetAutovacuumCostLimit(limit int64) {
	t.autovacuumCostLimit.Store(limit)
}

// IncRedisBufferOverflow increments the count of detected Redis buffer overflows.
func (t *DBSafeguardTelemetryTracker) IncRedisBufferOverflow() {
	t.redisBufferOverflows.Add(1)
}

// SetRedisBacklogBytes updates the current replication backlog size.
func (t *DBSafeguardTelemetryTracker) SetRedisBacklogBytes(size int64) {
	t.redisBacklogBytes.Store(size)
}

// IncKafkaPoisonPillsDLQ increments the count of poison pill messages diverted to DLQ.
func (t *DBSafeguardTelemetryTracker) IncKafkaPoisonPillsDLQ() {
	t.kafkaPoisonPillsDLQ.Add(1)
}

// IncKafkaOffsetsCommitted increments the count of committed offsets post DLQ diversion.
func (t *DBSafeguardTelemetryTracker) IncKafkaOffsetsCommitted() {
	t.kafkaOffsetsCommitted.Add(1)
}

// GetMaxXIDAgeObserved returns the maximum observed XID age.
func (t *DBSafeguardTelemetryTracker) GetMaxXIDAgeObserved() int64 {
	return t.maxXIDAgeObserved.Load()
}

// GetIdleTxTerminated returns the count of killed idle transactions.
func (t *DBSafeguardTelemetryTracker) GetIdleTxTerminated() uint64 {
	return t.idleTxTerminated.Load()
}

// GetActiveTxPreserved returns the count of preserved active queries.
func (t *DBSafeguardTelemetryTracker) GetActiveTxPreserved() uint64 {
	return t.activeTxPreserved.Load()
}

// GetAutovacuumCostLimit returns the current autovacuum cost limit.
func (t *DBSafeguardTelemetryTracker) GetAutovacuumCostLimit() int64 {
	return t.autovacuumCostLimit.Load()
}

// GetRedisBufferOverflows returns the count of buffer overflows.
func (t *DBSafeguardTelemetryTracker) GetRedisBufferOverflows() uint64 {
	return t.redisBufferOverflows.Load()
}

// GetRedisBacklogBytes returns the current backlog byte count.
func (t *DBSafeguardTelemetryTracker) GetRedisBacklogBytes() int64 {
	return t.redisBacklogBytes.Load()
}

// GetKafkaPoisonPillsDLQ returns the count of poison pills diverted.
func (t *DBSafeguardTelemetryTracker) GetKafkaPoisonPillsDLQ() uint64 {
	return t.kafkaPoisonPillsDLQ.Load()
}

// GetKafkaOffsetsCommitted returns the count of offsets committed.
func (t *DBSafeguardTelemetryTracker) GetKafkaOffsetsCommitted() uint64 {
	return t.kafkaOffsetsCommitted.Load()
}
