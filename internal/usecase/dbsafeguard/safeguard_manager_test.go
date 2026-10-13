package dbsafeguard_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/datdt/k8sselfhost/internal/domain/dbsafeguard"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/dbsafeguard"
)

// In-memory test port doubles for zero mocks and fast, concurrency-safe unit tests.
type testPostgresPort struct {
	mu sync.Mutex

	dbName      string
	maxXIDAge   int64
	longestIdle time.Duration
	oldestPID   int
	statsErr    error

	transactions   []usecase.TransactionInfo
	listErr        error
	terminatedPIDs []int
	terminateErr   error

	costLimit int64
	costErr   error
}

func (p *testPostgresPort) GetDatabaseXIDStats(ctx context.Context) (string, int64, time.Duration, int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.dbName, p.maxXIDAge, p.longestIdle, p.oldestPID, p.statsErr
}

func (p *testPostgresPort) ListTransactions(ctx context.Context) ([]usecase.TransactionInfo, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.transactions, p.listErr
}

func (p *testPostgresPort) TerminateBackend(ctx context.Context, pid int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.terminateErr != nil {
		return p.terminateErr
	}
	p.terminatedPIDs = append(p.terminatedPIDs, pid)
	return nil
}

func (p *testPostgresPort) SetAutovacuumCostLimit(ctx context.Context, limit int64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.costLimit = limit
	return p.costErr
}

type testRedisPort struct {
	mu sync.Mutex

	masterHost        string
	connectedReplicas int
	backlogSize       int64
	lagBytes          int64
	bufferOverflows   int64
	infoErr           error

	expandedBacklogSize int64
	throttledAddr       string
	throttledRate       int64
}

func (r *testRedisPort) GetReplicationInfo(ctx context.Context) (string, int, int64, int64, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.masterHost, r.connectedReplicas, r.backlogSize, r.lagBytes, r.bufferOverflows, r.infoErr
}

func (r *testRedisPort) ExpandReplBacklogSize(ctx context.Context, newSize int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expandedBacklogSize = newSize
	return nil
}

func (r *testRedisPort) ThrottleReplica(ctx context.Context, addr string, rate int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.throttledAddr = addr
	r.throttledRate = rate
	return nil
}

type testKafkaPort struct {
	mu sync.Mutex

	producedDLQTopic    string
	producedPartition   int32
	producedOffset      int64
	producedPayload     []byte
	producedReason      string
	produceErr          error

	committedTopic      string
	committedPartition  int32
	committedOffset     int64
	commitErr           error
}

func (k *testKafkaPort) ProduceDLQ(ctx context.Context, dlqTopic string, partition int32, offset int64, payload []byte, reason string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.produceErr != nil {
		return k.produceErr
	}
	k.producedDLQTopic = dlqTopic
	k.producedPartition = partition
	k.producedOffset = offset
	k.producedPayload = payload
	k.producedReason = reason
	return nil
}

func (k *testKafkaPort) CommitOffset(ctx context.Context, topic string, partition int32, offset int64) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.commitErr != nil {
		return k.commitErr
	}
	k.committedTopic = topic
	k.committedPartition = partition
	k.committedOffset = offset
	return nil
}

func TestSafeguardManager_CheckPostgresXIDHealth(t *testing.T) {
	t.Run("Normal XID age below threshold does not trigger emergency acceleration", func(t *testing.T) {
		pg := &testPostgresPort{
			dbName:      "test_db",
			maxXIDAge:   450_000_000,
			longestIdle: 2 * time.Minute,
			oldestPID:   100,
		}
		mgr := usecase.NewSafeguardManager(pg, nil, nil, nil)

		health, err := mgr.CheckPostgresXIDHealth(context.Background())
		require.NoError(t, err)
		assert.Equal(t, int64(450_000_000), health.MaxXIDAge)
		assert.False(t, health.IsEmergencyFreeze)
		assert.Equal(t, int64(0), pg.costLimit) // Not tuned
	})

	t.Run("Emergency XID age > 1.2 billion triggers autovacuum acceleration and idle tx kill", func(t *testing.T) {
		pg := &testPostgresPort{
			dbName:      "critical_db",
			maxXIDAge:   1_350_000_000,
			longestIdle: 40 * time.Minute,
			oldestPID:   301,
			transactions: []usecase.TransactionInfo{
				{PID: 301, State: "idle in transaction", Duration: 40 * time.Minute, Query: "SELECT * FROM orders FOR UPDATE;"},
				{PID: 302, State: "active", Duration: 50 * time.Minute, Query: "SELECT * FROM large_report;"},
				{PID: 303, State: "idle in transaction", Duration: 10 * time.Minute, Query: "BEGIN;"},
			},
		}
		mgr := usecase.NewSafeguardManager(pg, nil, nil, nil)

		health, err := mgr.CheckPostgresXIDHealth(context.Background())
		assert.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrDBXIDWraparoundRisk)
		require.NotNil(t, health)
		assert.True(t, health.IsEmergencyFreeze)

		// Verified emergency acceleration tuned autovacuum_cost_limit to 2000
		assert.Equal(t, int64(2000), pg.costLimit)
		assert.Equal(t, int64(2000), mgr.Telemetry().GetAutovacuumCostLimit())

		// Verified blocking idle transaction (>30m) terminated, while preserving active query
		assert.Contains(t, pg.terminatedPIDs, 301)
		assert.NotContains(t, pg.terminatedPIDs, 302) // Active query preserved!
		assert.NotContains(t, pg.terminatedPIDs, 303) // Idle tx < 30m preserved
		assert.Equal(t, uint64(1), mgr.Telemetry().GetIdleTxTerminated())
		assert.Equal(t, uint64(1), mgr.Telemetry().GetActiveTxPreserved())
	})
}

func TestSafeguardManager_TerminateBlockingTransactions_PreservesActiveQueries(t *testing.T) {
	pg := &testPostgresPort{
		transactions: []usecase.TransactionInfo{
			{PID: 101, State: "active", Duration: 45 * time.Minute, Query: "VACUUM ANALYZE;"},
			{PID: 102, State: "active", Duration: 60 * time.Minute, Query: "SELECT COUNT(*) FROM big_table;"},
			{PID: 103, State: "idle in transaction", Duration: 35 * time.Minute, Query: "UPDATE accounts SET balance = balance - 10;"},
			{PID: 104, State: "idle in transaction", Duration: 25 * time.Minute, Query: "INSERT INTO audit_logs;"},
			{PID: 105, State: "idle", Duration: 120 * time.Minute, Query: ""},
		},
	}
	mgr := usecase.NewSafeguardManager(pg, nil, nil, nil)

	err := mgr.TerminateBlockingTransactions(context.Background(), 30*time.Minute)
	require.NoError(t, err)

	// PID 103 was terminated (idle in transaction > 30m)
	assert.Equal(t, []int{103}, pg.terminatedPIDs)

	// Active queries (PIDs 101, 102) were strictly preserved
	assert.Equal(t, uint64(2), mgr.Telemetry().GetActiveTxPreserved())
	assert.Equal(t, uint64(1), mgr.Telemetry().GetIdleTxTerminated())
}

func TestSafeguardManager_AuditRedisReplicationBuffer(t *testing.T) {
	t.Run("Healthy replication returns no error", func(t *testing.T) {
		redis := &testRedisPort{
			masterHost:        "redis-node-0.redis:6379",
			connectedReplicas: 2,
			backlogSize:       64 * 1024 * 1024,
			lagBytes:          2048,
			bufferOverflows:   0,
		}
		mgr := usecase.NewSafeguardManager(nil, redis, nil, nil)

		health, err := mgr.AuditRedisReplicationBuffer(context.Background())
		require.NoError(t, err)
		assert.False(t, health.IsFullResyncLooping)
		assert.Equal(t, int64(0), redis.expandedBacklogSize)
	})

	t.Run("Replication buffer overflow triggers resync storm guard, expands backlog and throttles", func(t *testing.T) {
		redis := &testRedisPort{
			masterHost:        "redis-node-0.redis:6379",
			connectedReplicas: 2,
			backlogSize:       32 * 1024 * 1024,
			lagBytes:          50 * 1024 * 1024, // Lag exceeds backlog!
			bufferOverflows:   3,
		}
		mgr := usecase.NewSafeguardManager(nil, redis, nil, nil)

		health, err := mgr.AuditRedisReplicationBuffer(context.Background())
		assert.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrRedisResyncStormLoop)
		require.NotNil(t, health)
		assert.True(t, health.IsFullResyncLooping)

		// Verified backlog was expanded to at least 256MB floor
		assert.Equal(t, int64(256*1024*1024), redis.expandedBacklogSize)
		// Verified replica egress was throttled to 10MB/s
		assert.Equal(t, int64(10*1024*1024), redis.throttledRate)
		assert.Equal(t, uint64(1), mgr.Telemetry().GetRedisBufferOverflows())
	})
}

func TestSafeguardManager_HandleKafkaPoisonPill(t *testing.T) {
	t.Run("Successfully diverts poison pill to DLQ and commits offset", func(t *testing.T) {
		kafka := &testKafkaPort{}
		mgr := usecase.NewSafeguardManager(nil, nil, kafka, nil)

		payload := []byte(`{"order_id": NaN, "corrupt": \x00\x01}`)
		deserErr := errors.New("json: invalid character '\\x00' in numeric literal")

		err := mgr.HandleKafkaPoisonPill(context.Background(), "orders.events", 3, 98214, payload, deserErr)
		assert.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrKafkaPoisonPillDiverted)

		// Verified message written to DLQ
		assert.Equal(t, "orders.events.DLQ", kafka.producedDLQTopic)
		assert.Equal(t, int32(3), kafka.producedPartition)
		assert.Equal(t, int64(98214), kafka.producedOffset)
		assert.Equal(t, payload, kafka.producedPayload)
		assert.Equal(t, deserErr.Error(), kafka.producedReason)

		// Verified consumer offset was committed to unblock pipeline
		assert.Equal(t, "orders.events", kafka.committedTopic)
		assert.Equal(t, int32(3), kafka.committedPartition)
		assert.Equal(t, int64(98214), kafka.committedOffset)

		assert.Equal(t, uint64(1), mgr.Telemetry().GetKafkaPoisonPillsDLQ())
		assert.Equal(t, uint64(1), mgr.Telemetry().GetKafkaOffsetsCommitted())
	})

	t.Run("Produce DLQ failure aborts offset commit to prevent data loss", func(t *testing.T) {
		kafka := &testKafkaPort{
			produceErr: errors.New("kafka broker unavailable"),
		}
		mgr := usecase.NewSafeguardManager(nil, nil, kafka, nil)

		err := mgr.HandleKafkaPoisonPill(context.Background(), "payments", 0, 100, []byte("bad"), errors.New("bad payload"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed routing poison pill to DLQ topic")

		// Offset MUST NOT be committed if DLQ produce failed!
		assert.Equal(t, int64(0), kafka.committedOffset)
		assert.Equal(t, uint64(0), mgr.Telemetry().GetKafkaPoisonPillsDLQ())
		assert.Equal(t, uint64(0), mgr.Telemetry().GetKafkaOffsetsCommitted())
	})
}
