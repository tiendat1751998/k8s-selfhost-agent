package netguard_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/datdt/k8sselfhost/internal/domain/netguard"
)

// TestCacheLinePadding asserts that high-frequency metric and state structs
// are strictly aligned to 64 bytes (the CPU L1/L2 cache line size) to eliminate
// false sharing under high-rate probe traffic (MESI/MOESI coherence protocol).
func TestCacheLinePadding(t *testing.T) {
	t.Run("PMTUSweepState is 64 bytes", func(t *testing.T) {
		size := unsafe.Sizeof(netguard.PMTUSweepState{})
		if size != 64 {
			t.Fatalf("expected PMTUSweepState size to be 64 bytes, got %d bytes (false sharing risk)", size)
		}
	})

	t.Run("FastLatencyTracker is 64 bytes", func(t *testing.T) {
		size := unsafe.Sizeof(netguard.FastLatencyTracker{})
		if size != 64 {
			t.Fatalf("expected FastLatencyTracker size to be 64 bytes, got %d bytes (false sharing risk)", size)
		}
	})

	t.Run("FastTunnelMetrics is 64 bytes", func(t *testing.T) {
		size := unsafe.Sizeof(netguard.FastTunnelMetrics{})
		if size != 64 {
			t.Fatalf("expected FastTunnelMetrics size to be 64 bytes, got %d bytes (false sharing risk)", size)
		}
	})
}

// TestFastLatencyTracker_ConcurrentNoRace tests concurrent atomic updates on FastLatencyTracker.
func TestFastLatencyTracker_ConcurrentNoRace(t *testing.T) {
	var tracker netguard.FastLatencyTracker
	const numGoroutines = 50
	const iterationsPerGoroutine = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(coreID int) {
			defer wg.Done()
			for j := 0; j < iterationsPerGoroutine; j++ {
				atomic.AddUint64(&tracker.ProbeCount, 1)
				atomic.AddUint64(&tracker.TotalLatencyUs, uint64(coreID+1))
			}
		}(i)
	}

	wg.Wait()

	expectedProbes := uint64(numGoroutines * iterationsPerGoroutine)
	if tracker.ProbeCount != expectedProbes {
		t.Fatalf("expected ProbeCount=%d, got %d", expectedProbes, tracker.ProbeCount)
	}
}

// TestHybridTunnelStatus_Fields verifies proper construction of tunnel status objects.
func TestHybridTunnelStatus_Fields(t *testing.T) {
	now := time.Now().UTC()
	status := netguard.HybridTunnelStatus{
		InterfaceName: "wg-dc1",
		TunnelType:    "wireguard",
		TransportMode: netguard.TransportP2PDirect,
		ConfiguredMTU: 1420,
		DetectedPMTU:  1420,
		MSSClampingOn: true,
		RPFilterMode:  2,
		RTTLatencyMs:  12.5,
		PacketLossPct: 0.0,
		LastTestedAt:  now,
	}

	if status.InterfaceName != "wg-dc1" {
		t.Errorf("expected InterfaceName wg-dc1, got %s", status.InterfaceName)
	}
	if status.TransportMode != netguard.TransportP2PDirect {
		t.Errorf("expected TransportMode %s, got %s", netguard.TransportP2PDirect, status.TransportMode)
	}
	if status.ConfiguredMTU != 1420 || status.DetectedPMTU != 1420 {
		t.Errorf("unexpected MTU values: configured=%d, detected=%d", status.ConfiguredMTU, status.DetectedPMTU)
	}
}
