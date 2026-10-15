package workloadguard_test

import (
	"sync"
	"testing"

	"github.com/datdt/k8sselfhost/internal/domain/workloadguard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaddedCounters_Initialization(t *testing.T) {
	assert.Equal(t, uint64(0), (&workloadguard.PaddedDigestResolutionCounter{}).Load())
	assert.Equal(t, uint64(0), (&workloadguard.PaddedStorageCheckCounter{}).Load())
	assert.Equal(t, uint64(0), (&workloadguard.PaddedPIDAdmissionCounter{}).Load())
}

// TestPaddedCounters_ConcurrentOperations asserts race-free atomic updates under high concurrency.
func TestPaddedCounters_ConcurrentOperations(t *testing.T) {
	var (
		digestCounter  workloadguard.PaddedDigestResolutionCounter
		storageCounter workloadguard.PaddedStorageCheckCounter
		pidCounter     workloadguard.PaddedPIDAdmissionCounter
		wg             sync.WaitGroup
		goroutines     = 50
		iterations     = 1000
	)

	wg.Add(goroutines * 3)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				digestCounter.Inc()
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				storageCounter.Add(2)
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				pidCounter.Inc()
			}
		}()
	}

	wg.Wait()

	expectedDigest := uint64(goroutines * iterations)
	expectedStorage := uint64(goroutines * iterations * 2)
	expectedPID := uint64(goroutines * iterations)

	assert.Equal(t, expectedDigest, digestCounter.Load())
	assert.Equal(t, expectedStorage, storageCounter.Load())
	assert.Equal(t, expectedPID, pidCounter.Load())

	// Test Reset
	digestCounter.Reset()
	assert.Equal(t, uint64(0), digestCounter.Load())
}

// TestDomainErrors_Formatting verifies RFC 7807 error codes, messages, and wrapping.
func TestDomainErrors_Formatting(t *testing.T) {
	require.NotNil(t, workloadguard.ErrMutableImageTagRejected)
	assert.Equal(t, 422, workloadguard.ErrMutableImageTagRejected.HTTPStatus)
	assert.Equal(t, workloadguard.CodeMutableImageTagRejected, workloadguard.ErrMutableImageTagRejected.Code)
	assert.Contains(t, workloadguard.ErrMutableImageTagRejected.Error(), workloadguard.CodeMutableImageTagRejected)
	assert.NotNil(t, workloadguard.ErrMutableImageTagRejected.Unwrap())

	require.NotNil(t, workloadguard.ErrEphemeralStorageLimitMissing)
	assert.Equal(t, 400, workloadguard.ErrEphemeralStorageLimitMissing.HTTPStatus)
	assert.Equal(t, workloadguard.CodeEphemeralStorageLimitMissing, workloadguard.ErrEphemeralStorageLimitMissing.Code)
	assert.Contains(t, workloadguard.ErrEphemeralStorageLimitMissing.Error(), workloadguard.CodeEphemeralStorageLimitMissing)

	require.NotNil(t, workloadguard.ErrPIDExhaustionRisk)
	assert.Equal(t, 429, workloadguard.ErrPIDExhaustionRisk.HTTPStatus)
	assert.Equal(t, workloadguard.CodePIDExhaustionRisk, workloadguard.ErrPIDExhaustionRisk.Code)
	assert.Contains(t, workloadguard.ErrPIDExhaustionRisk.Error(), workloadguard.CodePIDExhaustionRisk)
}
