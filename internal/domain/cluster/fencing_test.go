package cluster_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/domain/cluster"
)

func TestCacheLinePadding_64ByteAlignment(t *testing.T) {
	// 64 bytes is the standard CPU cache line size (x86_64 and arm64).
	// To eliminate false sharing in MESI/MOESI cache coherence protocols,
	// telemetry counters and heartbeat state trackers must be at least 64 bytes
	// or align to 64-byte multiples.
	counterSize := unsafe.Sizeof(cluster.PaddedTelemetryCounter{})
	assert.Equal(t, uintptr(64), counterSize, "PaddedTelemetryCounter must be exactly 64 bytes")

	heartbeatSize := unsafe.Sizeof(cluster.HeartbeatStateTracker{})
	assert.Equal(t, uintptr(64), heartbeatSize, "HeartbeatStateTracker must be exactly 64 bytes")

	fencingTrackerSize := unsafe.Sizeof(cluster.FencingExecutionTracker{})
	assert.Equal(t, uintptr(64), fencingTrackerSize, "FencingExecutionTracker must be exactly 64 bytes")
}

func TestPaddedTelemetryCounter_ConcurrentRaceSafety(t *testing.T) {
	counter := &cluster.PaddedTelemetryCounter{}
	var wg sync.WaitGroup

	const goroutines = 16
	const iterations = 1000

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				atomic.AddUint64(&counter.HeartbeatChecks, 1)
				atomic.AddUint64(&counter.HeartbeatMisses, 1)
				atomic.AddUint64(&counter.FencingAttempts, 1)
				atomic.AddUint64(&counter.FencingSuccesses, 1)
				atomic.AddUint64(&counter.FencingFailures, 1)
			}
		}()
	}

	wg.Wait()

	expected := uint64(goroutines * iterations)
	assert.Equal(t, expected, atomic.LoadUint64(&counter.HeartbeatChecks))
	assert.Equal(t, expected, atomic.LoadUint64(&counter.HeartbeatMisses))
	assert.Equal(t, expected, atomic.LoadUint64(&counter.FencingAttempts))
	assert.Equal(t, expected, atomic.LoadUint64(&counter.FencingSuccesses))
	assert.Equal(t, expected, atomic.LoadUint64(&counter.FencingFailures))
}

func TestHeartbeatStateTracker_StateTransitions(t *testing.T) {
	tracker := cluster.NewHeartbeatStateTracker()
	require.NotNil(t, tracker)

	assert.Equal(t, cluster.HeartbeatStatusHealthy, tracker.GetState())

	// Mark missed beat
	tracker.RecordMissedBeat()
	assert.Equal(t, uint64(1), tracker.GetMissedBeats())

	// Mark suspect
	tracker.SetState(cluster.HeartbeatStatusSuspect)
	assert.Equal(t, cluster.HeartbeatStatusSuspect, tracker.GetState())

	// Record beat recovery
	tracker.RecordHeartbeatSuccess(time.Now())
	assert.Equal(t, cluster.HeartbeatStatusHealthy, tracker.GetState())
	assert.Equal(t, uint64(0), tracker.GetMissedBeats())
}

func TestDomainContracts_NodeFencingReportAndReadiness(t *testing.T) {
	now := time.Now().UTC()
	report := cluster.NewNodeFencingReport("node-db-01", "dc-east", cluster.IPMI_POWER_OFF, true, now, 1200*time.Millisecond, []string{"IPMI power off issued", "Chassis power status confirmed: off"})

	assert.Equal(t, "node-db-01", report.TargetNodeID)
	assert.Equal(t, "dc-east", report.Datacenter)
	assert.Equal(t, cluster.IPMI_POWER_OFF, report.Method)
	assert.True(t, report.IsFenced)
	assert.Equal(t, 2, len(report.OutputLog))

	readiness := cluster.NewFailoverReadinessStatus(true, true, 5*time.Millisecond, true, "")
	assert.True(t, readiness.CanPromote)
	assert.True(t, readiness.OldPrimaryFenced)
	assert.True(t, readiness.GraceDelayPassed)
	assert.Empty(t, readiness.RejectReason)

	unready := cluster.NewFailoverReadinessStatus(false, false, 500*time.Millisecond, false, "Old primary node 'node-db-01' is not confirmed fenced. Promoting replica risks split-brain writes.")
	assert.False(t, unready.CanPromote)
	assert.False(t, unready.OldPrimaryFenced)
	assert.Contains(t, unready.RejectReason, "not confirmed fenced")
}

func TestDomainSentinelErrors(t *testing.T) {
	assert.EqualError(t, cluster.ErrFailoverPrimaryNotFenced, "failover rejected: old primary is not confirmed fenced")
	assert.EqualError(t, cluster.ErrIMDSAccessBlocked, "imds access blocked: insecure metadata access detected")
	assert.EqualError(t, cluster.ErrGracePeriodActive, "failover postponed: transient grace period active")
}
