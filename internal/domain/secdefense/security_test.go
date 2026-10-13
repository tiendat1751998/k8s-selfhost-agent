package secdefense

import (
	"sync"
	"testing"
	"unsafe"
)

func TestPaddedCounterCacheLineAlignment(t *testing.T) {
	// A standard x86_64 / arm64 cache line is 64 bytes.
	// PaddedCounter must be at least 64 bytes to guarantee no adjacent variables
	// share the same cache line in CPU L1/L2 caches under MESI/MOESI coherence.
	size := unsafe.Sizeof(PaddedCounter{})
	if size < 64 {
		t.Fatalf("expected PaddedCounter size to be at least 64 bytes, got %d", size)
	}
}

func TestPaddedCounterConcurrentAccess(t *testing.T) {
	var counter PaddedCounter
	const numGoroutines = 100
	const incrementsPerGoroutine = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < incrementsPerGoroutine; j++ {
				counter.Inc()
			}
		}()
	}

	wg.Wait()

	expected := uint64(numGoroutines * incrementsPerGoroutine)
	if actual := counter.Load(); actual != expected {
		t.Fatalf("expected counter value %d, got %d", expected, actual)
	}
}

func TestDefenseMetricsConcurrentLoad(t *testing.T) {
	metrics := NewDefenseMetrics()
	const numWorkers = 50
	const iterations = 500

	var wg sync.WaitGroup
	wg.Add(numWorkers * 4)

	// Worker 1: SSRF Validations
	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				metrics.SSRFValidationsTotal.Inc()
			}
		}()
	}

	// Worker 2: SSRF Blocked
	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				metrics.SSRFBlockedTotal.Inc()
			}
		}()
	}

	// Worker 3: Metadata Blocked
	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				metrics.MetadataBlockedTotal.Inc()
			}
		}()
	}

	// Worker 4: Confusion Alerts
	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				metrics.ConfusionAlertsTotal.Inc()
			}
		}()
	}

	wg.Wait()

	expected := uint64(numWorkers * iterations)
	if metrics.SSRFValidationsTotal.Load() != expected {
		t.Errorf("SSRFValidationsTotal: expected %d, got %d", expected, metrics.SSRFValidationsTotal.Load())
	}
	if metrics.SSRFBlockedTotal.Load() != expected {
		t.Errorf("SSRFBlockedTotal: expected %d, got %d", expected, metrics.SSRFBlockedTotal.Load())
	}
	if metrics.MetadataBlockedTotal.Load() != expected {
		t.Errorf("MetadataBlockedTotal: expected %d, got %d", expected, metrics.MetadataBlockedTotal.Load())
	}
	if metrics.ConfusionAlertsTotal.Load() != expected {
		t.Errorf("ConfusionAlertsTotal: expected %d, got %d", expected, metrics.ConfusionAlertsTotal.Load())
	}
}

func TestDomainErrorsDefined(t *testing.T) {
	if ErrSecSSRFMetadataAttempt == nil {
		t.Fatal("ErrSecSSRFMetadataAttempt must be non-nil")
	}
	if ErrSecDependencyConfusion == nil {
		t.Fatal("ErrSecDependencyConfusion must be non-nil")
	}
	if ErrInvalidSidecarOrder == nil {
		t.Fatal("ErrInvalidSidecarOrder must be non-nil")
	}
}

func TestDependencyAuditReport_IsCleanCalculation(t *testing.T) {
	report := &DependencyAuditReport{
		TotalPackages:      10,
		InternalNamespaces: []string{"@internal"},
		SuspiciousPackages: []SuspiciousPackage{},
		IsClean:            true,
	}
	if !report.IsClean {
		t.Error("expected report to be clean when no suspicious packages exist")
	}

	report.SuspiciousPackages = append(report.SuspiciousPackages, SuspiciousPackage{
		Name:             "@internal/auth-sdk",
		ResolvedVersion:  "99.1.0",
		ResolvedRegistry: "https://registry.npmjs.org",
		InternalPrefix:   "@internal",
		Reason:           "Resolved to public registry with suspicious high version",
	})
	report.Evaluate()

	if report.IsClean {
		t.Error("expected report not to be clean after adding suspicious package")
	}
}
