package numamemory_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	domain "github.com/datdt/k8sselfhost/internal/domain/numamemory"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/numamemory"
)

func TestNUMAManager_OptimizeZoneReclaimMode(t *testing.T) {
	ctx := context.Background()

	// Initial setup: Node 0 has zone_reclaim_mode = 1 (inducing spinlocks)
	mgr := usecase.NewNUMAMemoryManager(
		usecase.WithHostRAMBytes(64*1024*1024*1024), // 64 GB
		usecase.WithInitialNUMAZones([]domain.NUMAZoneStatus{
			{
				NodeID:           0,
				TotalRAMBytes:    32 * 1024 * 1024 * 1024,
				FreeRAMBytes:     10 * 1024 * 1024 * 1024,
				ZoneReclaimMode:  1,
				SpinlockDetected: true,
			},
			{
				NodeID:           1,
				TotalRAMBytes:    32 * 1024 * 1024 * 1024,
				FreeRAMBytes:     15 * 1024 * 1024 * 1024,
				ZoneReclaimMode:  0,
				SpinlockDetected: false,
			},
		}),
	)

	// Tune zone reclaim mode to 0
	err := mgr.TuneNUMAZoneReclaim(ctx, 0)
	if err != nil {
		t.Fatalf("unexpected error tuning zone reclaim: %v", err)
	}

	zones, err := mgr.AuditNUMAZones(ctx)
	if err != nil {
		t.Fatalf("unexpected error auditing zones: %v", err)
	}

	for _, z := range zones {
		if z.ZoneReclaimMode != 0 {
			t.Fatalf("expected zone_reclaim_mode 0 on node %d, got %d", z.NodeID, z.ZoneReclaimMode)
		}
		if z.SpinlockDetected {
			t.Fatalf("expected spinlock to be cleared on node %d after tuning", z.NodeID)
		}
	}
}

func TestNUMAManager_SpinlockDetection(t *testing.T) {
	ctx := context.Background()

	mgr := usecase.NewNUMAMemoryManager(
		usecase.WithInitialNUMAZones([]domain.NUMAZoneStatus{
			{
				NodeID:           0,
				TotalRAMBytes:    32 * 1024 * 1024 * 1024,
				FreeRAMBytes:     512 * 1024 * 1024,
				ZoneReclaimMode:  1,
				SpinlockDetected: true,
			},
		}),
	)

	_, err := mgr.AuditNUMAZones(ctx)
	if !errors.Is(err, domain.ErrNUMAZoneLocked) {
		t.Fatalf("expected ErrNUMAZoneLocked, got %v", err)
	}
}

func TestNUMAManager_DirtyPageByteLimits(t *testing.T) {
	ctx := context.Background()
	mgr := usecase.NewNUMAMemoryManager(
		usecase.WithHostRAMBytes(32 * 1024 * 1024 * 1024),
	)

	// Test 1: Convert risky dirty ratios to 256MB background / 1GB total limits
	err := mgr.ApplyDirtyPageByteLimits(ctx, 256*1024*1024, 1024*1024*1024)
	if err != nil {
		t.Fatalf("failed to apply dirty limits: %v", err)
	}

	cfg := mgr.GetDirtyPageConfig()
	if cfg.DirtyBackgroundBytes != 256*1024*1024 {
		t.Fatalf("expected 256MB dirty background bytes, got %d", cfg.DirtyBackgroundBytes)
	}
	if cfg.DirtyBytes != 1024*1024*1024 {
		t.Fatalf("expected 1GB dirty bytes, got %d", cfg.DirtyBytes)
	}

	// Test 2: Guardrail check: never lower vm.dirty_bytes below 128MB
	err = mgr.ApplyDirtyPageByteLimits(ctx, 50*1024*1024, 64*1024*1024) // 64MB is below 128MB guardrail
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg = mgr.GetDirtyPageConfig()
	if cfg.DirtyBytes < usecase.MinDirtyBytesGuardrail {
		t.Fatalf("guardrail breached: dirty_bytes %d is below min %d", cfg.DirtyBytes, usecase.MinDirtyBytesGuardrail)
	}

	// Test 3: Saturation detection
	mgr.SetCurrentDirtyBytes(2 * 1024 * 1024 * 1024) // 2GB dirty > 1GB threshold
	err = mgr.ApplyDirtyPageByteLimits(ctx, 256*1024*1024, 1024*1024*1024)
	if !errors.Is(err, domain.ErrDirtyPageFlushSaturation) {
		t.Fatalf("expected ErrDirtyPageFlushSaturation, got %v", err)
	}
}

func TestNUMAManager_HugePagesCompaction(t *testing.T) {
	ctx := context.Background()
	mgr := usecase.NewNUMAMemoryManager(
		usecase.WithInitialHugePages([]domain.HugePagePoolStatus{
			{
				PageSizeKB:       1048576, // 1GiB
				TotalPages:       32,
				FreePages:        4,
				FragmentationIdx: 0.82, // > 0.70 threshold
			},
		}),
	)

	// Check availability for 2 pages; fragmentation > 0.70 should trigger compaction
	status, err := mgr.CheckHugePageAvailability(ctx, 1048576, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.FragmentationIdx > 0.70 {
		t.Fatalf("expected compaction to lower fragmentation index <= 0.70, got %f", status.FragmentationIdx)
	}

	// Requesting more pages than available should return ErrHugePagesFragmented
	_, err = mgr.CheckHugePageAvailability(ctx, 1048576, 100)
	if !errors.Is(err, domain.ErrHugePagesFragmented) {
		t.Fatalf("expected ErrHugePagesFragmented, got %v", err)
	}
}

func TestNUMAManager_SlabUnreclaimableLeak(t *testing.T) {
	ctx := context.Background()
	hostRAM := int64(100 * 1024 * 1024 * 1024) // 100GB

	// Leak case: SUnreclaim = 35GB (35% > 30% threshold)
	mgrLeak := usecase.NewNUMAMemoryManager(
		usecase.WithHostRAMBytes(hostRAM),
		usecase.WithSlabUnreclaimBytes(35*1024*1024*1024),
	)

	unreclaimMB, err := mgrLeak.AuditSlabUnreclaimable(ctx)
	if !errors.Is(err, domain.ErrSlabUnreclaimableLeak) {
		t.Fatalf("expected ErrSlabUnreclaimableLeak, got %v", err)
	}
	if unreclaimMB != 35*1024 {
		t.Fatalf("expected 35840 MB, got %d", unreclaimMB)
	}

	// Healthy case: SUnreclaim = 10GB (10% < 30%)
	mgrHealthy := usecase.NewNUMAMemoryManager(
		usecase.WithHostRAMBytes(hostRAM),
		usecase.WithSlabUnreclaimBytes(10*1024*1024*1024),
	)
	unreclaimMB, err = mgrHealthy.AuditSlabUnreclaimable(ctx)
	if err != nil {
		t.Fatalf("unexpected error in healthy slab audit: %v", err)
	}
	if unreclaimMB != 10*1024 {
		t.Fatalf("expected 10240 MB, got %d", unreclaimMB)
	}
}

func TestNUMAManager_ConcurrentSafety(t *testing.T) {
	ctx := context.Background()
	mgr := usecase.NewNUMAMemoryManager(
		usecase.WithHostRAMBytes(64*1024*1024*1024),
		usecase.WithInitialNUMAZones([]domain.NUMAZoneStatus{
			{NodeID: 0, TotalRAMBytes: 32 * 1024 * 1024 * 1024, FreeRAMBytes: 16 * 1024 * 1024 * 1024, ZoneReclaimMode: 0},
		}),
		usecase.WithInitialHugePages([]domain.HugePagePoolStatus{
			{PageSizeKB: 2048, TotalPages: 1024, FreePages: 512, FragmentationIdx: 0.2},
		}),
	)

	var wg sync.WaitGroup
	workers := 20
	iterations := 50

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_, _ = mgr.AuditNUMAZones(ctx)
				_ = mgr.ApplyDirtyPageByteLimits(ctx, 256*1024*1024, 1024*1024*1024)
				_, _ = mgr.CheckHugePageAvailability(ctx, 2048, 1)
				_, _ = mgr.AuditSlabUnreclaimable(ctx)
			}
		}(i)
	}
	wg.Wait()
}
