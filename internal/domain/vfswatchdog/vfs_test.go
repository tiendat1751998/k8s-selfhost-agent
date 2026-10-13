package vfswatchdog_test

import (
	"errors"
	"sync"
	"testing"
	"time"
	"unsafe"

	domain "github.com/datdt/k8sselfhost/internal/domain/vfswatchdog"
	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

func TestPaddedMetrics_CacheLineAlignment(t *testing.T) {
	// Modern multi-core x86_64 and ARM64 CPU cache line size is 64 bytes.
	const expectedCacheLineSize = 64

	inodeMetricSize := unsafe.Sizeof(domain.PaddedInodeMetric{})
	if inodeMetricSize != expectedCacheLineSize {
		t.Fatalf("PaddedInodeMetric size = %d bytes; want %d bytes (64-byte cache-line padded)",
			inodeMetricSize, expectedCacheLineSize)
	}

	dstateMetricSize := unsafe.Sizeof(domain.PaddedDStateMetric{})
	if dstateMetricSize != expectedCacheLineSize {
		t.Fatalf("PaddedDStateMetric size = %d bytes; want %d bytes (64-byte cache-line padded)",
			dstateMetricSize, expectedCacheLineSize)
	}

	pidMetricSize := unsafe.Sizeof(domain.PaddedPIDMetric{})
	if pidMetricSize != expectedCacheLineSize {
		t.Fatalf("PaddedPIDMetric size = %d bytes; want %d bytes (64-byte cache-line padded)",
			pidMetricSize, expectedCacheLineSize)
	}
}

func TestPaddedMetrics_ConcurrentSafety(t *testing.T) {
	var inodeMetric domain.PaddedInodeMetric
	var dstateMetric domain.PaddedDStateMetric
	var pidMetric domain.PaddedPIDMetric

	const numGoroutines = 100
	const iterationsPerGoroutine = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 3)

	for i := 0; i < numGoroutines; i++ {
		// Goroutines updating inode metric
		go func() {
			defer wg.Done()
			for j := 0; j < iterationsPerGoroutine; j++ {
				inodeMetric.AddScannedFiles(1)
				inodeMetric.RecordScanDuration(10)
				inodeMetric.SetInodesInUse(uint64(j))
			}
		}()

		// Goroutines updating D-state metric
		go func() {
			defer wg.Done()
			for j := 0; j < iterationsPerGoroutine; j++ {
				dstateMetric.IncDStateProcesses()
				dstateMetric.AddStuckDurationMs(5)
				dstateMetric.IncLazyUnmountTriggers()
			}
		}()

		// Goroutines updating PID metric
		go func() {
			defer wg.Done()
			for j := 0; j < iterationsPerGoroutine; j++ {
				pidMetric.AddZombiePIDs(1)
				pidMetric.IncThrottledPIDs()
				pidMetric.SetTotalPIDs(uint64(j * 2))
			}
		}()
	}

	wg.Wait()

	expectedTotal := uint64(numGoroutines * iterationsPerGoroutine)
	if inodeMetric.ScannedFiles.Load() != expectedTotal {
		t.Errorf("ScannedFiles = %d; want %d", inodeMetric.ScannedFiles.Load(), expectedTotal)
	}
	if dstateMetric.DStateProcessesDetected.Load() != expectedTotal {
		t.Errorf("DStateProcessesDetected = %d; want %d", dstateMetric.DStateProcessesDetected.Load(), expectedTotal)
	}
	if dstateMetric.LazyUnmountTriggers.Load() != expectedTotal {
		t.Errorf("LazyUnmountTriggers = %d; want %d", dstateMetric.LazyUnmountTriggers.Load(), expectedTotal)
	}
	if pidMetric.ZombiePIDs.Load() != expectedTotal {
		t.Errorf("ZombiePIDs = %d; want %d", pidMetric.ZombiePIDs.Load(), expectedTotal)
	}
	if pidMetric.ThrottledPIDs.Load() != expectedTotal {
		t.Errorf("ThrottledPIDs = %d; want %d", pidMetric.ThrottledPIDs.Load(), expectedTotal)
	}
}

func TestInodeReport_IsExhausted(t *testing.T) {
	tests := []struct {
		name     string
		report   domain.InodeReport
		expected bool
	}{
		{
			name: "Normal usage under threshold",
			report: domain.InodeReport{
				MountPoint:      "/data",
				TotalInodes:     1000000,
				FreeInodes:      500000,
				UsagePercentage: 50.0,
			},
			expected: false,
		},
		{
			name: "Critical usage exactly at 85%",
			report: domain.InodeReport{
				MountPoint:      "/var",
				TotalInodes:     1000000,
				FreeInodes:      150000,
				UsagePercentage: 85.0,
			},
			expected: true,
		},
		{
			name: "Critical usage over 85%",
			report: domain.InodeReport{
				MountPoint:      "/tmp",
				TotalInodes:     1000000,
				FreeInodes:      10000,
				UsagePercentage: 99.0,
			},
			expected: true,
		},
		{
			name: "0 free inodes despite percentage calculation edge case",
			report: domain.InodeReport{
				MountPoint:      "/cache",
				TotalInodes:     1000000,
				FreeInodes:      0,
				UsagePercentage: 100.0,
			},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.report.IsExhausted(); got != tc.expected {
				t.Errorf("IsExhausted() = %v; want %v", got, tc.expected)
			}
		})
	}
}

func TestDStateProcessInfo_IsStuck(t *testing.T) {
	proc := domain.DStateProcessInfo{
		PID:          4567,
		Command:      "nfs_sync_worker",
		WaitDuration: 6 * time.Minute,
		WchanSymbol:  "nfs_file_write",
		MountPoint:   "/mnt/storage",
	}

	if !proc.IsStuck(5 * time.Minute) {
		t.Errorf("expected process with 6m wait duration to be considered stuck at 5m threshold")
	}

	if proc.IsStuck(10 * time.Minute) {
		t.Errorf("expected process with 6m wait duration not to be stuck at 10m threshold")
	}

	// Default threshold fallback (5m)
	if !proc.IsStuck(0) {
		t.Errorf("expected process with 6m wait duration to be stuck using default threshold")
	}
}

func TestDomainErrors_CodesAndUnwrap(t *testing.T) {
	var dErr *domainerrors.DomainError

	if !errors.As(domain.ErrVFSInodeExhaustion, &dErr) {
		t.Fatalf("ErrVFSInodeExhaustion does not cast to DomainError")
	}
	if dErr.Code != domain.CodeVFSInodeExhaustion {
		t.Errorf("got code %s; want %s", dErr.Code, domain.CodeVFSInodeExhaustion)
	}

	if !errors.As(domain.ErrVFSDStateDeadlock, &dErr) {
		t.Fatalf("ErrVFSDStateDeadlock does not cast to DomainError")
	}
	if dErr.Code != domain.CodeVFSDStateDeadlock {
		t.Errorf("got code %s; want %s", dErr.Code, domain.CodeVFSDStateDeadlock)
	}

	if !errors.As(domain.ErrNFSUnmountFailed, &dErr) {
		t.Fatalf("ErrNFSUnmountFailed does not cast to DomainError")
	}
	if dErr.Code != domain.CodeNFSUnmountFailed {
		t.Errorf("got code %s; want %s", dErr.Code, domain.CodeNFSUnmountFailed)
	}
}
