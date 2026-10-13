package finopsguard_test

import (
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/domain/finopsguard"
)

func TestFinOpsDomain_RFC7807Errors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		err        *finopsguard.RFC7807ProblemDetail
		expectedSt int
		expectedCd string
	}{
		{
			name:       "CrossAZWasteDetected",
			err:        finopsguard.ErrFinOpsCrossAZWasteDetected,
			expectedSt: http.StatusConflict,
			expectedCd: "ERR_FINOPS_CROSS_AZ_WASTE_DETECTED",
		},
		{
			name:       "ResourceNotApproved",
			err:        finopsguard.ErrResourceNotApproved,
			expectedSt: http.StatusBadRequest,
			expectedCd: "ERR_RESOURCE_NOT_APPROVED",
		},
		{
			name:       "SnapshotGenerationFailed",
			err:        finopsguard.ErrSnapshotGenerationFailed,
			expectedSt: http.StatusInternalServerError,
			expectedCd: "ERR_SNAPSHOT_GENERATION_FAILED",
		},
		{
			name:       "ResourceNotFound",
			err:        finopsguard.ErrResourceNotFound,
			expectedSt: http.StatusNotFound,
			expectedCd: "ERR_RESOURCE_NOT_FOUND",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expectedSt, tc.err.Status)
			assert.Equal(t, tc.expectedCd, tc.err.Code)
			assert.NotEmpty(t, tc.err.Title)
			assert.NotEmpty(t, tc.err.Detail)
			assert.Contains(t, tc.err.Error(), tc.expectedCd)
			assert.Contains(t, tc.err.Error(), tc.err.Title)
		})
	}

	// Verify errors.Is / errors.As compatibility
	var rfcErr *finopsguard.RFC7807ProblemDetail
	require.True(t, errors.As(finopsguard.ErrFinOpsCrossAZWasteDetected, &rfcErr))
	assert.Equal(t, http.StatusConflict, rfcErr.Status)
}

func TestFinOpsDomain_OrphanCloudResource(t *testing.T) {
	t.Parallel()

	idleTime := time.Now().Add(-50 * time.Hour)
	orphan := finopsguard.OrphanCloudResource{
		ResourceID:              "vol-0123456789abcdef0",
		Type:                    finopsguard.ResourceEBSVolume,
		CloudProvider:           "aws",
		ClusterTag:              "k8s-prod-fleet-01",
		OriginalNamespace:       "billing-archived",
		IdleSince:               idleTime,
		EstimatedMonthlyCostUSD: 160.0,
		ApprovalStatus:          finopsguard.ApprovalStatusPending,
		HasSnapshot:             false,
	}

	assert.True(t, orphan.IsIdleBeyond(48*time.Hour))
	assert.False(t, orphan.IsIdleBeyond(72*time.Hour))
	assert.False(t, orphan.IsApproved())

	orphan.ApprovalStatus = finopsguard.ApprovalStatusApproved
	assert.True(t, orphan.IsApproved())
}

func TestFinOpsTelemetryTracker_ConcurrentCachePaddedAccess(t *testing.T) {
	t.Parallel()

	tracker := finopsguard.NewFinOpsTelemetryTracker()
	const numGoroutines = 50
	const iterationsPerGoroutine = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 7)

	// Goroutine pool mutating crossAZEgressBytesSaved
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterationsPerGoroutine; j++ {
				tracker.AddCrossAZEgressBytesSaved(1024)
			}
		}()
	}

	// Goroutine pool mutating unhintedServicesScanned
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterationsPerGoroutine; j++ {
				tracker.IncUnhintedServicesScanned()
			}
		}()
	}

	// Goroutine pool mutating topologyHintsInjected
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterationsPerGoroutine; j++ {
				tracker.IncTopologyHintsInjected()
			}
		}()
	}

	// Goroutine pool mutating skewFallbacksTriggered
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterationsPerGoroutine; j++ {
				tracker.IncSkewFallbacksTriggered()
			}
		}()
	}

	// Goroutine pool mutating orphansDetected
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterationsPerGoroutine; j++ {
				tracker.IncOrphansDetected()
			}
		}()
	}

	// Goroutine pool mutating orphansReaped
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterationsPerGoroutine; j++ {
				tracker.IncOrphansReaped()
			}
		}()
	}

	// Goroutine pool mutating snapshotsGenerated
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterationsPerGoroutine; j++ {
				tracker.IncSnapshotsGenerated()
			}
		}()
	}

	wg.Wait()

	expectedCount := uint64(numGoroutines * iterationsPerGoroutine)
	assert.Equal(t, expectedCount*1024, tracker.GetCrossAZEgressBytesSaved())
	assert.Equal(t, expectedCount, tracker.GetUnhintedServicesScanned())
	assert.Equal(t, expectedCount, tracker.GetTopologyHintsInjected())
	assert.Equal(t, expectedCount, tracker.GetSkewFallbacksTriggered())
	assert.Equal(t, expectedCount, tracker.GetOrphansDetected())
	assert.Equal(t, expectedCount, tracker.GetOrphansReaped())
	assert.Equal(t, expectedCount, tracker.GetSnapshotsGenerated())
}
