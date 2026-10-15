package numamemory_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/datdt/k8sselfhost/internal/domain/numamemory"
)

func TestNUMADomain_CounterInitialization(t *testing.T) {
	spinlockCounter := numamemory.NUMASpinlockCounter{}
	dirtyTracker := numamemory.DirtyPageTracker{}
	hpMetric := numamemory.HugePageAllocationMetric{}

	if spinlockCounter.SpinlockCount != 0 {
		t.Fatalf("expected 0, got %d", spinlockCounter.SpinlockCount)
	}
	if dirtyTracker.DirtyBytes != 0 {
		t.Fatalf("expected 0, got %d", dirtyTracker.DirtyBytes)
	}
	if hpMetric.AllocatedPages != 0 {
		t.Fatalf("expected 0, got %d", hpMetric.AllocatedPages)
	}
}

func TestNUMADomain_ConcurrentCounters(t *testing.T) {
	var counter numamemory.NUMASpinlockCounter
	var wg sync.WaitGroup
	workers := 16
	iterations := 1000

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				atomic.AddUint64(&counter.SpinlockCount, 1)
			}
		}()
	}
	wg.Wait()

	expected := uint64(workers * iterations)
	if counter.SpinlockCount != expected {
		t.Fatalf("expected spinlock count %d, got %d", expected, counter.SpinlockCount)
	}
}

func TestNUMADomain_Errors(t *testing.T) {
	if !errors.Is(numamemory.ErrNUMAZoneLocked, numamemory.ErrNUMAZoneLocked) {
		t.Fatal("ErrNUMAZoneLocked identity check failed")
	}
	if !errors.Is(numamemory.ErrDirtyPageFlushSaturation, numamemory.ErrDirtyPageFlushSaturation) {
		t.Fatal("ErrDirtyPageFlushSaturation identity check failed")
	}
	if !errors.Is(numamemory.ErrHugePagesFragmented, numamemory.ErrHugePagesFragmented) {
		t.Fatal("ErrHugePagesFragmented identity check failed")
	}
	if !errors.Is(numamemory.ErrSlabUnreclaimableLeak, numamemory.ErrSlabUnreclaimableLeak) {
		t.Fatal("ErrSlabUnreclaimableLeak identity check failed")
	}
}

// Compile-time interface check.
type mockSubsystemManager struct{}

var _ numamemory.LinuxMemorySubsystemManager = (*mockSubsystemManager)(nil)

func (m *mockSubsystemManager) AuditNUMAZones(ctx context.Context) ([]numamemory.NUMAZoneStatus, error) {
	return nil, nil
}
func (m *mockSubsystemManager) TuneNUMAZoneReclaim(ctx context.Context, targetMode int) error {
	return nil
}
func (m *mockSubsystemManager) ApplyDirtyPageByteLimits(ctx context.Context, backgroundBytes, totalBytes int64) error {
	return nil
}
func (m *mockSubsystemManager) CheckHugePageAvailability(ctx context.Context, sizeKB int, requestedPages int) (*numamemory.HugePagePoolStatus, error) {
	return nil, nil
}
func (m *mockSubsystemManager) TriggerCompactionIfFragmented(ctx context.Context, threshold float64) error {
	return nil
}
func (m *mockSubsystemManager) AuditSlabUnreclaimable(ctx context.Context) (int64, error) {
	return 0, nil
}
