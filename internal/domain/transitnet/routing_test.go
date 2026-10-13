package transitnet_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/datdt/k8sselfhost/internal/domain/transitnet"
)

// TestCacheLinePadding asserts that high-frequency metric and monitor structs
// are strictly aligned to 64 bytes (the CPU L1/L2 cache line size) to eliminate
// false sharing across CPU cores under high-rate hybrid telemetry polling (MESI/MOESI protocol).
func TestCacheLinePadding(t *testing.T) {
	t.Run("FastRoutingLatencyTracker is 64 bytes", func(t *testing.T) {
		size := unsafe.Sizeof(transitnet.FastRoutingLatencyTracker{})
		if size != 64 {
			t.Fatalf("expected FastRoutingLatencyTracker size to be 64 bytes, got %d bytes (false sharing risk)", size)
		}
	})

	t.Run("FastPacketLossCounter is 64 bytes", func(t *testing.T) {
		size := unsafe.Sizeof(transitnet.FastPacketLossCounter{})
		if size != 64 {
			t.Fatalf("expected FastPacketLossCounter size to be 64 bytes, got %d bytes (false sharing risk)", size)
		}
	})

	t.Run("FastNLBDistributionMonitor is 64 bytes", func(t *testing.T) {
		size := unsafe.Sizeof(transitnet.FastNLBDistributionMonitor{})
		if size != 64 {
			t.Fatalf("expected FastNLBDistributionMonitor size to be 64 bytes, got %d bytes (false sharing risk)", size)
		}
	})
}

// TestFastRoutingLatencyTracker_ConcurrentNoRace verifies atomic lock-free updates under high goroutine load.
func TestFastRoutingLatencyTracker_ConcurrentNoRace(t *testing.T) {
	var tracker transitnet.FastRoutingLatencyTracker
	const numGoroutines = 50
	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				atomic.AddUint64(&tracker.ProbeCount, 1)
				atomic.AddUint64(&tracker.TotalLatencyUs, uint64(id+1))
				atomic.AddUint64(&tracker.AsymmetricDropCount, 1)
			}
		}(i)
	}

	wg.Wait()

	expectedProbes := uint64(numGoroutines * iterations)
	if tracker.ProbeCount != expectedProbes {
		t.Fatalf("expected ProbeCount=%d, got %d", expectedProbes, tracker.ProbeCount)
	}
	if tracker.AsymmetricDropCount != expectedProbes {
		t.Fatalf("expected AsymmetricDropCount=%d, got %d", expectedProbes, tracker.AsymmetricDropCount)
	}
}

// TestFastPacketLossCounter_ConcurrentNoRace verifies atomic loss counters under concurrency.
func TestFastPacketLossCounter_ConcurrentNoRace(t *testing.T) {
	var counter transitnet.FastPacketLossCounter
	const numGoroutines = 50
	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				atomic.AddUint64(&counter.TxPackets, 1)
				atomic.AddUint64(&counter.RxPackets, 1)
				atomic.AddUint64(&counter.LostPackets, 1)
			}
		}()
	}

	wg.Wait()

	expected := uint64(numGoroutines * iterations)
	if counter.TxPackets != expected || counter.RxPackets != expected || counter.LostPackets != expected {
		t.Fatalf("counter mismatch: tx=%d rx=%d lost=%d expected=%d",
			counter.TxPackets, counter.RxPackets, counter.LostPackets, expected)
	}
}

// TestFastNLBDistributionMonitor_ConcurrentNoRace verifies atomic NLB distribution counters under concurrency.
func TestFastNLBDistributionMonitor_ConcurrentNoRace(t *testing.T) {
	var monitor transitnet.FastNLBDistributionMonitor
	const numGoroutines = 30
	const iterations = 500

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(zone int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				atomic.AddUint64(&monitor.TotalRequests, 1)
				switch zone % 3 {
				case 0:
					atomic.AddUint64(&monitor.ZoneARequests, 1)
				case 1:
					atomic.AddUint64(&monitor.ZoneBRequests, 1)
				case 2:
					atomic.AddUint64(&monitor.ZoneCRequests, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	expectedTotal := uint64(numGoroutines * iterations)
	if monitor.TotalRequests != expectedTotal {
		t.Fatalf("expected TotalRequests=%d, got %d", expectedTotal, monitor.TotalRequests)
	}
	sumZones := monitor.ZoneARequests + monitor.ZoneBRequests + monitor.ZoneCRequests
	if sumZones != expectedTotal {
		t.Fatalf("zone sum %d != total %d", sumZones, expectedTotal)
	}
}

// TestDomainContracts_Entities verifies struct fields and domain error invariants.
func TestDomainContracts_Entities(t *testing.T) {
	// RoutingPathIntegrity
	rpi := transitnet.RoutingPathIntegrity{
		PrimaryPath:    "dx-conn-1001",
		ReturnPath:     "vpn-gw-2002",
		IsAsymmetric:   true,
		RTTLatency:     45 * time.Millisecond,
		PacketLossRate: 0.05,
	}
	if !rpi.IsAsymmetric || rpi.PrimaryPath != "dx-conn-1001" {
		t.Errorf("unexpected RoutingPathIntegrity values: %+v", rpi)
	}

	// PeeringMTUValidation
	pmv := transitnet.PeeringMTUValidation{
		SourceVPCID:       "vpc-jumbo-01",
		TargetVPCID:       "vpc-std-02",
		ConfiguredMSS:     1460,
		EffectivePathMTU:  1500,
		BlackHoleDetected: true,
	}
	if !pmv.BlackHoleDetected || pmv.EffectivePathMTU != 1500 {
		t.Errorf("unexpected PeeringMTUValidation values: %+v", pmv)
	}

	// NLBZoneDistribution
	nzd := transitnet.NLBZoneDistribution{
		LoadBalancerARN:   "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/net/prod-nlb/123",
		CrossZoneEnabled:  false,
		ZoneRequestShares: map[string]float64{"us-east-1a": 0.75, "us-east-1b": 0.25},
		ImbalanceDetected: true,
	}
	if !nzd.ImbalanceDetected || nzd.CrossZoneEnabled {
		t.Errorf("unexpected NLBZoneDistribution values: %+v", nzd)
	}

	// Domain Errors
	if transitnet.ErrBGPAsymmetricPath == nil {
		t.Error("ErrBGPAsymmetricPath must be defined")
	}
	if transitnet.ErrVPCPathMTUBlackHole == nil {
		t.Error("ErrVPCPathMTUBlackHole must be defined")
	}
	if transitnet.ErrNLBAZImbalance == nil {
		t.Error("ErrNLBAZImbalance must be defined")
	}
}
