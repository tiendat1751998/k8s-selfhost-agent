package gpuwatch_test

import (
	"sync"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/gpuwatch"
)

func TestXIDClassification(t *testing.T) {
	tests := []struct {
		xid      int
		critical bool
	}{
		{xid: 31, critical: true}, // MMU Fault
		{xid: 43, critical: true}, // GPU stopped processing
		{xid: 45, critical: true}, // Engine reset failure
		{xid: 79, critical: true}, // GPU fallen off the bus
		{xid: 13, critical: false}, // Graphics exception (driver / app recovery)
		{xid: 32, critical: false}, // Push buffer corruption
		{xid: 62, critical: false}, // Microcontroller internal
		{xid: 0, critical: false},
	}

	for _, tt := range tests {
		got := gpuwatch.IsCriticalXID(tt.xid)
		if got != tt.critical {
			t.Errorf("IsCriticalXID(%d) = %v; want %v", tt.xid, got, tt.critical)
		}
	}
}

func TestParseDmesgXIDErrors(t *testing.T) {
	rawDmesg := `
[ 1024.123456] NVRM: Xid (PCI:0000:01:00.0): 31, pid=12345, name=python3, MMU Fault: ENGINE GRAPHICS GPCCL
[ 1025.654321] NVRM: Xid (PCI:0000:41:00.0): 13, pid=12346, name=python3, Graphics Exception: ESR 0x40f100
[ 1028.999999] [nvidia] unrelated kernel log line
[ 1030.111111] NVRM: Xid (PCI:0000:81:00.0): 79, GPU has fallen off the bus.
[ 1035.222222] NVRM: Xid (PCI:0000:c1:00.0): 43, GPU stopped processing
`

	events := gpuwatch.ParseDmesgXIDErrors(rawDmesg)
	if len(events) != 4 {
		t.Fatalf("ParseDmesgXIDErrors returned %d events; want 4", len(events))
	}

	// First event: XID 31 (critical)
	if events[0].XID != 31 || events[0].PCIAddress != "0000:01:00.0" || !events[0].IsCritical {
		t.Errorf("Event 0 unexpected: %+v", events[0])
	}

	// Second event: XID 13 (non-critical)
	if events[1].XID != 13 || events[1].PCIAddress != "0000:41:00.0" || events[1].IsCritical {
		t.Errorf("Event 1 unexpected: %+v", events[1])
	}

	// Third event: XID 79 (critical)
	if events[2].XID != 79 || events[2].PCIAddress != "0000:81:00.0" || !events[2].IsCritical {
		t.Errorf("Event 2 unexpected: %+v", events[2])
	}

	// Fourth event: XID 43 (critical)
	if events[3].XID != 43 || events[3].PCIAddress != "0000:c1:00.0" || !events[3].IsCritical {
		t.Errorf("Event 3 unexpected: %+v", events[3])
	}
}

func TestKVCachePressureEvaluation(t *testing.T) {
	tests := []struct {
		usagePct float64
		wantOOM  bool
	}{
		{usagePct: 50.0, wantOOM: false},
		{usagePct: 84.99, wantOOM: false},
		{usagePct: 85.0, wantOOM: true},
		{usagePct: 92.4, wantOOM: true},
		{usagePct: 100.0, wantOOM: true},
	}

	for _, tt := range tests {
		err := gpuwatch.EvaluateKVCacheRisk(tt.usagePct)
		if tt.wantOOM && err != gpuwatch.ErrKVCacheOOMRisk {
			t.Errorf("EvaluateKVCacheRisk(%f) = %v; want ErrKVCacheOOMRisk", tt.usagePct, err)
		}
		if !tt.wantOOM && err != nil {
			t.Errorf("EvaluateKVCacheRisk(%f) = %v; want nil", tt.usagePct, err)
		}
	}
}

func TestNCCLHangEvaluation(t *testing.T) {
	// Hang should trigger if idle duration exceeds 5 minutes (300 seconds)
	if !gpuwatch.IsNCCLHang(5*time.Minute + 1*time.Second) {
		t.Errorf("IsNCCLHang(5m1s) = false; want true")
	}
	if gpuwatch.IsNCCLHang(4 * time.Minute) {
		t.Errorf("IsNCCLHang(4m) = true; want false")
	}
}

func TestPaddedGPUCountersConcurrency(t *testing.T) {
	var counters gpuwatch.PaddedGPUCounters

	const goroutines = 50
	const increments = 1000
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < increments; j++ {
				counters.RecordPoll(1024*1024*100, 85500)
			}
		}()
	}
	wg.Wait()

	if counters.GetPollCount() != uint64(goroutines*increments) {
		t.Errorf("GetPollCount() = %d; want %d", counters.GetPollCount(), goroutines*increments)
	}
	if counters.GetVRAMUsedBytes() != 1024*1024*100 {
		t.Errorf("GetVRAMUsedBytes() = %d; want %d", counters.GetVRAMUsedBytes(), 1024*1024*100)
	}
}
