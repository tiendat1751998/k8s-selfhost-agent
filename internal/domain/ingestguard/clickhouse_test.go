package ingestguard_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/domain/ingestguard"
)

func TestBatchFlushConfig_DefaultsAndValidation(t *testing.T) {
	cfg := ingestguard.DefaultBatchFlushConfig()
	assert.Equal(t, 10000, cfg.MaxBatchSize)
	assert.Equal(t, 5*time.Second, cfg.FlushInterval)
	assert.Equal(t, ingestguard.BufferTypeRing, cfg.BufferType)

	require.NoError(t, cfg.Validate())

	invalidCfg := ingestguard.BatchFlushConfig{
		MaxBatchSize:  0,
		FlushInterval: -1 * time.Second,
		BufferType:    "invalid",
	}
	err := invalidCfg.Validate()
	require.Error(t, err)
}

func TestClickHouseTableHealth_ThresholdEvaluation(t *testing.T) {
	now := time.Now()

	// Healthy (< 150 parts)
	h1 := ingestguard.NewClickHouseTableHealth("default", "cluster_logs", 120, false, now)
	assert.Equal(t, ingestguard.HealthStatusHealthy, h1.HealthStatus)
	assert.False(t, h1.IsSaturated())
	assert.False(t, h1.ShouldThrottle())

	// Degraded / Throttle (200 - 279 parts)
	h2 := ingestguard.NewClickHouseTableHealth("default", "cluster_logs", 220, false, now)
	assert.Equal(t, ingestguard.HealthStatusDegraded, h2.HealthStatus)
	assert.False(t, h2.IsSaturated())
	assert.True(t, h2.ShouldThrottle())

	// Critical / Saturation (>= 280 parts)
	h3 := ingestguard.NewClickHouseTableHealth("default", "cluster_logs", 285, true, now)
	assert.Equal(t, ingestguard.HealthStatusCritical, h3.HealthStatus)
	assert.True(t, h3.IsSaturated())
	assert.True(t, h3.ShouldThrottle())
}

func TestPaddedCounter_Operations(t *testing.T) {
	counter := &ingestguard.PaddedCounter{}
	assert.Equal(t, uint64(0), counter.Load())

	// Concurrent atomic operations test
	var wg sync.WaitGroup
	const goroutines = 50
	const iterations = 1000

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				counter.Inc()
			}
		}()
	}

	wg.Wait()
	assert.Equal(t, uint64(goroutines*iterations), counter.Load())
}

func TestStandardErrors(t *testing.T) {
	assert.ErrorIs(t, ingestguard.ErrClickHousePartsSaturation, ingestguard.ErrClickHousePartsSaturation)
	assert.ErrorIs(t, ingestguard.ErrUnboundedAggQuery, ingestguard.ErrUnboundedAggQuery)
	assert.ErrorIs(t, ingestguard.ErrBufferOverflow, ingestguard.ErrBufferOverflow)
}
