package hardware_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/hardware"
)

func TestCacheLinePaddedCounter_ConcurrencySafety(t *testing.T) {
	const numGoroutines = 64
	const iterationsPerGoroutine = 10000

	var counters [8]hardware.CacheLinePaddedCounter
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	startTime := time.Now()
	for i := 0; i < numGoroutines; i++ {
		coreID := i % len(counters)
		go func(cIdx int) {
			defer wg.Done()
			for j := 0; j < iterationsPerGoroutine; j++ {
				atomic.AddUint64(&counters[cIdx].Value, 1)
			}
		}(coreID)
	}
	wg.Wait()
	duration := time.Since(startTime)
	t.Logf("Completed %d concurrent atomic updates across %d padded counters in %v",
		numGoroutines*iterationsPerGoroutine, len(counters), duration)

	var total uint64
	for i := range counters {
		expectedPerCounter := uint64((numGoroutines / len(counters)) * iterationsPerGoroutine)
		val := atomic.LoadUint64(&counters[i].Value)
		if val != expectedPerCounter {
			t.Errorf("Counter %d: expected %d, got %d", i, expectedPerCounter, val)
		}
		total += val
	}

	expectedTotal := uint64(numGoroutines * iterationsPerGoroutine)
	if total != expectedTotal {
		t.Fatalf("Total count mismatch: expected %d, got %d", expectedTotal, total)
	}
}

func TestDomainStructs_FieldAndPaddingIntegrity(t *testing.T) {
	now := time.Now()
	dimm := hardware.ECCDIMMStatus{
		SlotID:           "mc0/dimm0",
		CorrectableCount: 150,
		ErrorRatePerMin:  2.5,
		IsDegraded:       true,
		LastEventTime:    now,
	}
	if dimm.SlotID != "mc0/dimm0" || dimm.CorrectableCount != 150 || !dimm.IsDegraded {
		t.Errorf("ECCDIMMStatus fields mismatch: %+v", dimm)
	}

	cpu := hardware.CPUThermalStatus{
		PackageTempC:        88.5,
		IsThrottled:         true,
		BaseFrequencyMHz:    3600.0,
		CurrentFrequencyMHz: 1200.0,
	}
	if cpu.PackageTempC != 88.5 || !cpu.IsThrottled || cpu.CurrentFrequencyMHz != 1200.0 {
		t.Errorf("CPUThermalStatus fields mismatch: %+v", cpu)
	}

	psu := hardware.PowerSupplyStatus{
		PSUID:          "PSU_1",
		IsPresent:      true,
		InputVoltage:   230.5,
		OutputPowerW:   750.0,
		RedundancyLost: true,
	}
	if psu.PSUID != "PSU_1" || !psu.RedundancyLost || psu.OutputPowerW != 750.0 {
		t.Errorf("PowerSupplyStatus fields mismatch: %+v", psu)
	}
}

func TestDomainErrors_DefinedCorrectly(t *testing.T) {
	if hardware.ErrECCMemoryCorrectableSurge == nil {
		t.Fatal("ErrECCMemoryCorrectableSurge should not be nil")
	}
	if hardware.ErrCPUThermalThrottled == nil {
		t.Fatal("ErrCPUThermalThrottled should not be nil")
	}
	if hardware.ErrPSURedundancyFailed == nil {
		t.Fatal("ErrPSURedundancyFailed should not be nil")
	}
}
