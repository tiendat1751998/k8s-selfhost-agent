package dbsafeguard_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/domain/dbsafeguard"
)

func TestPostgresXIDHealth_ConstructorAndThresholds(t *testing.T) {
	t.Run("Normal state below threshold", func(t *testing.T) {
		health, err := dbsafeguard.NewPostgresXIDHealth("production_db", 500_000_000, 5*time.Minute, 1024)
		require.NoError(t, err)
		assert.Equal(t, "production_db", health.DatabaseName)
		assert.Equal(t, int64(500_000_000), health.MaxXIDAge)
		assert.Equal(t, int64(700_000_000), health.RemainingBeforeEmergency)
		assert.Equal(t, 5*time.Minute, health.LongestIdleTx)
		assert.Equal(t, 1024, health.OldestTxPID)
		assert.False(t, health.IsEmergencyFreeze)
	})

	t.Run("Emergency state above 1.2 billion threshold", func(t *testing.T) {
		health, err := dbsafeguard.NewPostgresXIDHealth("critical_db", 1_400_000_000, 35*time.Minute, 2048)
		require.NoError(t, err)
		assert.Equal(t, "critical_db", health.DatabaseName)
		assert.Equal(t, int64(1_400_000_000), health.MaxXIDAge)
		assert.Equal(t, int64(0), health.RemainingBeforeEmergency)
		assert.True(t, health.IsEmergencyFreeze)
	})

	t.Run("Validation errors on invalid input", func(t *testing.T) {
		_, errEmptyDB := dbsafeguard.NewPostgresXIDHealth("", 100, time.Minute, 1)
		assert.Error(t, errEmptyDB)
		assert.ErrorIs(t, errEmptyDB, dbsafeguard.ErrInvalidSafeguardInput)

		_, errNegativeAge := dbsafeguard.NewPostgresXIDHealth("db", -10, time.Minute, 1)
		assert.Error(t, errNegativeAge)
		assert.ErrorIs(t, errNegativeAge, dbsafeguard.ErrInvalidSafeguardInput)
	})
}

func TestRedisReplicationHealth_ConstructorAndValidation(t *testing.T) {
	t.Run("Valid replication health", func(t *testing.T) {
		repl, err := dbsafeguard.NewRedisReplicationHealth("redis-master:6379", 3, 67108864, 1024, 0, false)
		require.NoError(t, err)
		assert.Equal(t, "redis-master:6379", repl.MasterHost)
		assert.Equal(t, 3, repl.ConnectedReplicas)
		assert.Equal(t, int64(67108864), repl.ReplBacklogSize)
		assert.Equal(t, int64(1024), repl.ReplicationLagBytes)
		assert.Equal(t, int64(0), repl.BufferOverflowCount)
		assert.False(t, repl.IsFullResyncLooping)
	})

	t.Run("Validation errors on invalid input", func(t *testing.T) {
		_, errEmptyHost := dbsafeguard.NewRedisReplicationHealth("", 1, 1024, 0, 0, false)
		assert.Error(t, errEmptyHost)

		_, errNegativeRepl := dbsafeguard.NewRedisReplicationHealth("host", -1, 1024, 0, 0, false)
		assert.Error(t, errNegativeRepl)

		_, errNegativeLag := dbsafeguard.NewRedisReplicationHealth("host", 1, 1024, -5, 0, false)
		assert.Error(t, errNegativeLag)
	})
}

func TestKafkaPoisonPillAction_ConstructorAndValidation(t *testing.T) {
	t.Run("Valid poison pill action", func(t *testing.T) {
		action, err := dbsafeguard.NewKafkaPoisonPillAction("transactions.v1", 2, 45100, "corrupted json payload", true)
		require.NoError(t, err)
		assert.Equal(t, "transactions.v1", action.Topic)
		assert.Equal(t, int32(2), action.Partition)
		assert.Equal(t, int64(45100), action.Offset)
		assert.Equal(t, "corrupted json payload", action.ErrorReason)
		assert.True(t, action.RedirectedToDLQ)
		assert.False(t, action.HandledAt.IsZero())
	})

	t.Run("Validation errors on invalid input", func(t *testing.T) {
		_, errEmptyTopic := dbsafeguard.NewKafkaPoisonPillAction("", 0, 0, "err", true)
		assert.Error(t, errEmptyTopic)

		_, errNegativePartition := dbsafeguard.NewKafkaPoisonPillAction("top", -1, 0, "err", true)
		assert.Error(t, errNegativePartition)

		_, errNegativeOffset := dbsafeguard.NewKafkaPoisonPillAction("top", 0, -10, "err", true)
		assert.Error(t, errNegativeOffset)

		_, errEmptyReason := dbsafeguard.NewKafkaPoisonPillAction("top", 0, 0, "", true)
		assert.Error(t, errEmptyReason)
	})
}

func TestRFC7807ProblemDetailErrors(t *testing.T) {
	assert.Equal(t, 500, dbsafeguard.ErrDBXIDWraparoundRisk.Status)
	assert.Equal(t, "ERR_DB_XID_WRAPAROUND_RISK", dbsafeguard.ErrDBXIDWraparoundRisk.Code)
	assert.Contains(t, dbsafeguard.ErrDBXIDWraparoundRisk.Error(), "ERR_DB_XID_WRAPAROUND_RISK")

	assert.Equal(t, 422, dbsafeguard.ErrKafkaPoisonPillDiverted.Status)
	assert.Equal(t, "ERR_KAFKA_POISON_PILL_DIVERTED", dbsafeguard.ErrKafkaPoisonPillDiverted.Code)
	assert.Contains(t, dbsafeguard.ErrKafkaPoisonPillDiverted.Error(), "ERR_KAFKA_POISON_PILL_DIVERTED")

	assert.Equal(t, 500, dbsafeguard.ErrRedisResyncStormLoop.Status)
	assert.Equal(t, "ERR_REDIS_RESYNC_STORM_LOOP", dbsafeguard.ErrRedisResyncStormLoop.Code)
	assert.Contains(t, dbsafeguard.ErrRedisResyncStormLoop.Error(), "ERR_REDIS_RESYNC_STORM_LOOP")
}

func TestDBSafeguardTelemetryTracker_HighConcurrencyCacheLinePadding(t *testing.T) {
	tracker := dbsafeguard.NewDBSafeguardTelemetryTracker()
	require.NotNil(t, tracker)
	assert.Equal(t, int64(200), tracker.GetAutovacuumCostLimit())

	const goroutines = 32
	const iterations = 500

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		workerID := i
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				tracker.RecordXIDAge(int64((workerID+1)*1000 + j))
				tracker.IncIdleTxTerminated()
				tracker.IncActiveTxPreserved()
				tracker.SetAutovacuumCostLimit(2000)
				tracker.IncRedisBufferOverflow()
				tracker.SetRedisBacklogBytes(int64(128 * 1024 * 1024))
				tracker.IncKafkaPoisonPillsDLQ()
				tracker.IncKafkaOffsetsCommitted()
			}
		}()
	}

	wg.Wait()

	expectedCount := uint64(goroutines * iterations)
	assert.Equal(t, expectedCount, tracker.GetIdleTxTerminated())
	assert.Equal(t, expectedCount, tracker.GetActiveTxPreserved())
	assert.Equal(t, int64(2000), tracker.GetAutovacuumCostLimit())
	assert.Equal(t, expectedCount, tracker.GetRedisBufferOverflows())
	assert.Equal(t, int64(128*1024*1024), tracker.GetRedisBacklogBytes())
	assert.Equal(t, expectedCount, tracker.GetKafkaPoisonPillsDLQ())
	assert.Equal(t, expectedCount, tracker.GetKafkaOffsetsCommitted())
	assert.Greater(t, tracker.GetMaxXIDAgeObserved(), int64(0))
}
