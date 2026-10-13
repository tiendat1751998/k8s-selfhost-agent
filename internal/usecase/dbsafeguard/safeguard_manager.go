package dbsafeguard

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/dbsafeguard"
)

// PostgresPort defines the data access contract for PostgreSQL health, backend inspection, and vacuum tuning.
type PostgresPort interface {
	GetDatabaseXIDStats(ctx context.Context) (dbName string, maxXIDAge int64, longestIdle time.Duration, oldestPID int, err error)
	ListTransactions(ctx context.Context) ([]TransactionInfo, error)
	TerminateBackend(ctx context.Context, pid int) error
	SetAutovacuumCostLimit(ctx context.Context, limit int64) error
}

// TransactionInfo contains snapshot telemetry for an individual PostgreSQL backend process.
type TransactionInfo struct {
	PID      int           `json:"pid"`
	State    string        `json:"state"` // "idle in transaction", "active", "idle"
	Duration time.Duration `json:"duration"`
	Query    string        `json:"query"`
}

// RedisPort defines the data access contract for Redis replication backlog monitoring and throttling.
type RedisPort interface {
	GetReplicationInfo(ctx context.Context) (masterHost string, connectedReplicas int, backlogSize int64, lagBytes int64, bufferOverflows int64, err error)
	ExpandReplBacklogSize(ctx context.Context, newBacklogSizeBytes int64) error
	ThrottleReplica(ctx context.Context, replicaAddr string, rateLimitBytesPerSec int64) error
}

// KafkaPort defines the messaging transport contract for dead-letter queue routing and offset commits.
type KafkaPort interface {
	ProduceDLQ(ctx context.Context, dlqTopic string, partition int32, originalOffset int64, payload []byte, errorReason string) error
	CommitOffset(ctx context.Context, topic string, partition int32, offset int64) error
}

// SafeguardManager orchestrates database reliability, XID wraparound defense, Redis resync storm mitigations,
// and Kafka poison pill DLQ diversion. Implements domain.DatabaseResilienceEngine.
type SafeguardManager struct {
	pgPort    PostgresPort
	redisPort RedisPort
	kafkaPort KafkaPort
	telemetry *dbsafeguard.DBSafeguardTelemetryTracker
}

var _ dbsafeguard.DatabaseResilienceEngine = (*SafeguardManager)(nil)

// NewSafeguardManager initializes a SafeguardManager with injected ports and telemetry tracker.
func NewSafeguardManager(
	pgPort PostgresPort,
	redisPort RedisPort,
	kafkaPort KafkaPort,
	telemetry *dbsafeguard.DBSafeguardTelemetryTracker,
) *SafeguardManager {
	if telemetry == nil {
		telemetry = dbsafeguard.NewDBSafeguardTelemetryTracker()
	}
	return &SafeguardManager{
		pgPort:    pgPort,
		redisPort: redisPort,
		kafkaPort: kafkaPort,
		telemetry: telemetry,
	}
}

// Telemetry returns the underlying cache-line padded atomic telemetry tracker.
func (m *SafeguardManager) Telemetry() *dbsafeguard.DBSafeguardTelemetryTracker {
	return m.telemetry
}

// CheckPostgresXIDHealth evaluates age(datfrozenxid) against the 1.2 billion emergency horizon.
// When MaxXIDAge > 1.2 billion, triggers emergency vacuum acceleration (autovacuum_cost_limit = 2000)
// and terminates transactions in 'idle in transaction' status > 30 minutes.
func (m *SafeguardManager) CheckPostgresXIDHealth(ctx context.Context) (*dbsafeguard.PostgresXIDHealth, error) {
	if m.pgPort == nil {
		return nil, errors.New("postgres port is not configured")
	}

	dbName, maxXIDAge, longestIdle, oldestPID, err := m.pgPort.GetDatabaseXIDStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("evaluating postgres XID stats: %w", err)
	}

	health, err := dbsafeguard.NewPostgresXIDHealth(dbName, maxXIDAge, longestIdle, oldestPID)
	if err != nil {
		return nil, fmt.Errorf("creating postgres XID health entity: %w", err)
	}

	m.telemetry.RecordXIDAge(maxXIDAge)

	if health.IsEmergencyFreeze {
		// Emergency acceleration: raise cost limit to 2000 to eliminate autovacuum sleep penalties
		_ = m.TuneAutovacuumAggressiveness(ctx)

		// Terminate idle in transaction connections older than 30 minutes to unblock vacuum freeze
		_ = m.TerminateBlockingTransactions(ctx, dbsafeguard.DefaultMaxIdleTxDuration)

		return health, dbsafeguard.ErrDBXIDWraparoundRisk
	}

	return health, nil
}

// TerminateBlockingTransactions terminates PostgreSQL backends lingering in 'idle in transaction' state
// longer than maxAge. Active running queries are strictly preserved by invariant guards.
func (m *SafeguardManager) TerminateBlockingTransactions(ctx context.Context, maxAge time.Duration) error {
	if m.pgPort == nil {
		return errors.New("postgres port is not configured")
	}

	if maxAge <= 0 {
		maxAge = dbsafeguard.DefaultMaxIdleTxDuration
	}

	transactions, err := m.pgPort.ListTransactions(ctx)
	if err != nil {
		return fmt.Errorf("listing database transactions: %w", err)
	}

	var terminationErrors []error
	for _, tx := range transactions {
		// GUARDRAIL: Never terminate actively executing queries
		if tx.State != "idle in transaction" {
			if tx.State == "active" {
				m.telemetry.IncActiveTxPreserved()
			}
			continue
		}

		// Only terminate if idle duration exceeds the defined threshold
		if tx.Duration >= maxAge {
			if err := m.pgPort.TerminateBackend(ctx, tx.PID); err != nil {
				terminationErrors = append(terminationErrors, fmt.Errorf("terminating PID %d: %w", tx.PID, err))
			} else {
				m.telemetry.IncIdleTxTerminated()
			}
		}
	}

	if len(terminationErrors) > 0 {
		return fmt.Errorf("failed terminating %d idle transactions: %v", len(terminationErrors), terminationErrors[0])
	}

	return nil
}

// TuneAutovacuumAggressiveness raises autovacuum_cost_limit to 2000 for emergency vacuum acceleration.
func (m *SafeguardManager) TuneAutovacuumAggressiveness(ctx context.Context) error {
	if m.pgPort == nil {
		return errors.New("postgres port is not configured")
	}

	if err := m.pgPort.SetAutovacuumCostLimit(ctx, dbsafeguard.AutovacuumCostLimitEmergency); err != nil {
		return fmt.Errorf("setting autovacuum_cost_limit to %d: %w", dbsafeguard.AutovacuumCostLimitEmergency, err)
	}

	m.telemetry.SetAutovacuumCostLimit(dbsafeguard.AutovacuumCostLimitEmergency)
	return nil
}

// AuditRedisReplicationBuffer audits replication backlog and detects client output buffer overflows.
// Expands repl-backlog-size and throttles failing replicas to prevent cyclic BGSAVE CoW fork storms.
func (m *SafeguardManager) AuditRedisReplicationBuffer(ctx context.Context) (*dbsafeguard.RedisReplicationHealth, error) {
	if m.redisPort == nil {
		return nil, errors.New("redis port is not configured")
	}

	masterHost, connectedReplicas, backlogSize, lagBytes, bufferOverflows, err := m.redisPort.GetReplicationInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching redis replication info: %w", err)
	}

	isLooping := bufferOverflows > 0 || (backlogSize > 0 && lagBytes > backlogSize)

	health, err := dbsafeguard.NewRedisReplicationHealth(
		masterHost,
		connectedReplicas,
		backlogSize,
		lagBytes,
		bufferOverflows,
		isLooping,
	)
	if err != nil {
		return nil, fmt.Errorf("creating redis replication health entity: %w", err)
	}

	m.telemetry.SetRedisBacklogBytes(backlogSize)

	if isLooping {
		m.telemetry.IncRedisBufferOverflow()

		// Expand replication backlog to avoid cyclic PSYNC failure and BGSAVE fork
		newBacklogSize := backlogSize * 2
		const minBacklogSizeBytes = int64(256 * 1024 * 1024) // 256MB floor
		if newBacklogSize < minBacklogSizeBytes {
			newBacklogSize = minBacklogSizeBytes
		}

		_ = m.redisPort.ExpandReplBacklogSize(ctx, newBacklogSize)
		m.telemetry.SetRedisBacklogBytes(newBacklogSize)

		// Throttle replica egress to prevent socket truncation by client-output-buffer-limit
		const throttleBytesPerSec = int64(10 * 1024 * 1024) // 10MB/s
		_ = m.redisPort.ThrottleReplica(ctx, "", throttleBytesPerSec)

		return health, dbsafeguard.ErrRedisResyncStormLoop
	}

	return health, nil
}

// HandleKafkaPoisonPill intercepts unprocessable message payload deserialization/schema errors,
// diverts the poison pill message to a Dead Letter Queue (DLQ) topic, and commits the offset to prevent
// head-of-line consumer restart-rebalance storms.
func (m *SafeguardManager) HandleKafkaPoisonPill(
	ctx context.Context,
	topic string,
	partition int32,
	offset int64,
	payload []byte,
	err error,
) error {
	if m.kafkaPort == nil {
		return errors.New("kafka port is not configured")
	}

	if topic == "" || partition < 0 || offset < 0 {
		return fmt.Errorf("%w: invalid kafka coordinate topic=%s, partition=%d, offset=%d",
			dbsafeguard.ErrInvalidSafeguardInput, topic, partition, offset)
	}

	errorReason := "unspecified deserialization error"
	if err != nil {
		errorReason = err.Error()
	}

	dlqTopic := fmt.Sprintf("%s.DLQ", topic)

	// Step 1: Safely write poison message to Dead Letter Queue (DLQ) topic
	if produceErr := m.kafkaPort.ProduceDLQ(ctx, dlqTopic, partition, offset, payload, errorReason); produceErr != nil {
		return fmt.Errorf("failed routing poison pill to DLQ topic %s: %w", dlqTopic, produceErr)
	}
	m.telemetry.IncKafkaPoisonPillsDLQ()

	// Step 2: Commit consumer offset to advance past poison pill and prevent pipeline head-of-line stall
	if commitErr := m.kafkaPort.CommitOffset(ctx, topic, partition, offset); commitErr != nil {
		return fmt.Errorf("failed committing offset %d after DLQ diversion: %w", offset, commitErr)
	}
	m.telemetry.IncKafkaOffsetsCommitted()

	// Return RFC 7807 ErrKafkaPoisonPillDiverted to indicate successful diversion of unprocessable entity
	return dbsafeguard.ErrKafkaPoisonPillDiverted
}
