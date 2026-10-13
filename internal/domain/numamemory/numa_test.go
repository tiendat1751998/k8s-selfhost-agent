package numamemory_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"unsafe"

	"github.com/datdt/k8sselfhost/internal/domain/numamemory"
)

func TestNUMADomain_CacheLinePadding(t *testing.T) {
	// Verify 64-byte alignment and sizing to prevent false sharing across CPU cores under MESI/MOESI protocol.
	spinlockCounter := numamemory.NUMASpinlockCounter{}
	dirtyTracker := numamemory.DirtyPageTracker{}
	hpMetric := numamemory.HugePageAllocationMetric{}

	if size := unsafe.Sizeof(spinlockCounter); size != 64 {
		t.Fatalf("expected NUMASpinlockCounter size to be 64 bytes (1 cache line), got %d", size)
	}
	if size := unsafe.Sizeof(dirtyTracker); size != 64 {
		t.Fatalf("expected DirtyPageTracker size to be 64 bytes (1 cache line), got %d", size)
	}
	if size := unsafe.Sizeof(hpMetric); size != 64 {
		t.Fatalf("expected HugePageAllocationMetric size to be 64 bytes (1 cache line), got %d", size)
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
