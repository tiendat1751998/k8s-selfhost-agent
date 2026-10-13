package apfguard_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"unsafe"

	"github.com/datdt/k8sselfhost/internal/domain/apfguard"
)

func TestPaddedMetrics_CacheLineAlignment(t *testing.T) {
	var apfMetrics apfguard.PaddedAPFMetrics
	var compMetrics apfguard.PaddedCompactionMetrics

	if sz := unsafe.Sizeof(apfMetrics); sz%64 != 0 {
		t.Fatalf("PaddedAPFMetrics size %d is not a multiple of 64 bytes (cache line)", sz)
	}

	if sz := unsafe.Sizeof(compMetrics); sz%64 != 0 {
		t.Fatalf("PaddedCompactionMetrics size %d is not a multiple of 64 bytes (cache line)", sz)
	}
}

func TestAPFQueueStatus_StarvationEvaluation(t *testing.T) {
	// system-leader-election with queue depth > 0 is immediately starved
	leaderStarved := apfguard.NewAPFQueueStatus("system-leader-election", 1, 10, 10, 0)
	if !leaderStarved.IsStarved {
		t.Errorf("expected system-leader-election with queue > 0 to be marked starved")
	}

	// system-leader-election with queue == 0 is healthy
	leaderHealthy := apfguard.NewAPFQueueStatus("system-leader-election", 0, 5, 10, 0)
	if leaderHealthy.IsStarved {
		t.Errorf("expected system-leader-election with queue == 0 to not be starved")
	}

	// workload-low priority level with queue >= limit should be starved
	workloadSaturated := apfguard.NewAPFQueueStatus("workload-low", 50, 50, 50, 5)
	if !workloadSaturated.IsStarved {
		t.Errorf("expected workload-low with queue >= limit to be marked starved")
	}

	// workload-low with drops should be starved
	workloadDrops := apfguard.NewAPFQueueStatus("workload-low", 10, 20, 50, 2)
	if !workloadDrops.IsStarved {
		t.Errorf("expected workload-low with drop count > 0 to be marked starved")
	}

	// normal priority level within limits
	workloadHealthy := apfguard.NewAPFQueueStatus("workload-high", 5, 20, 50, 0)
	if workloadHealthy.IsStarved {
		t.Errorf("expected workload-high under limits with no drops to not be starved")
	}
}

func TestWatchCacheCompactionPolicy_DefaultsAndValidation(t *testing.T) {
	policy := apfguard.DefaultWatchCacheCompactionPolicy()
	if policy.BatchSize <= 0 {
		t.Errorf("expected default BatchSize > 0, got %d", policy.BatchSize)
	}
	if policy.MaxJitterWindow <= 0 {
		t.Errorf("expected default MaxJitterWindow > 0, got %v", policy.MaxJitterWindow)
	}
	if policy.CompactionAge <= 0 {
		t.Errorf("expected default CompactionAge > 0, got %v", policy.CompactionAge)
	}

	if err := policy.Validate(); err != nil {
		t.Fatalf("expected default policy to be valid, got: %v", err)
	}

	invalidPolicy := apfguard.WatchCacheCompactionPolicy{
		MaxJitterWindow: 0,
		BatchSize:       -1,
		CompactionAge:   0,
	}
	if err := invalidPolicy.Validate(); err == nil {
		t.Errorf("expected invalid policy to return error, got nil")
	}
}

func TestSentinelErrors(t *testing.T) {
	if apfguard.ErrAPFQueueSaturated == nil {
		t.Fatal("ErrAPFQueueSaturated must be defined")
	}
	if apfguard.ErrWatchCacheCompacted == nil {
		t.Fatal("ErrWatchCacheCompacted must be defined")
	}
	if apfguard.ErrMetricsServerUnavailable == nil {
		t.Fatal("ErrMetricsServerUnavailable must be defined")
	}
	if apfguard.ErrLeaderElectionStarved == nil {
		t.Fatal("ErrLeaderElectionStarved must be defined")
	}
	if apfguard.ErrUnprivilegedCapExceeded == nil {
		t.Fatal("ErrUnprivilegedCapExceeded must be defined")
	}
}

func TestPaddedMetrics_ConcurrentSafety(t *testing.T) {
	var apfMetrics apfguard.PaddedAPFMetrics
	var compMetrics apfguard.PaddedCompactionMetrics

	const goroutines = 50
	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				atomic.AddUint64(&apfMetrics.CurrentInQueue, 1)
				atomic.AddUint64(&apfMetrics.CurrentExecuting, 1)
				atomic.AddUint64(&apfMetrics.DropCount, 1)
			}
		}()

		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				atomic.AddUint64(&compMetrics.Compacted410Errors, 1)
				atomic.AddUint64(&compMetrics.PaginatedChunks, 1)
				atomic.AddUint64(&compMetrics.JitterWaitsTotal, 1)
			}
		}()
	}

	wg.Wait()

	expected := uint64(goroutines * iterations)
	if atomic.LoadUint64(&apfMetrics.DropCount) != expected {
		t.Fatalf("expected apf DropCount %d, got %d", expected, apfMetrics.DropCount)
	}
	if atomic.LoadUint64(&compMetrics.Compacted410Errors) != expected {
		t.Fatalf("expected comp Compacted410Errors %d, got %d", expected, compMetrics.Compacted410Errors)
	}
}
