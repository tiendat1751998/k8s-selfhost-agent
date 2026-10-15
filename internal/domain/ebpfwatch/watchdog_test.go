package ebpfwatch_test

import (
	"sync"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/ebpfwatch"
)

func TestPaddedRingBufferCounter_Initialization(t *testing.T) {
	var counter ebpfwatch.PaddedRingBufferCounter
	if counter.TotalEvents.Load() != 0 {
		t.Fatalf("expected 0, got %d", counter.TotalEvents.Load())
	}

	var controller ebpfwatch.PaddedSamplingController
	if controller.SamplingRatePct.Load() != 0 {
		t.Fatalf("expected 0, got %d", controller.SamplingRatePct.Load())
	}
}

func TestPaddedRingBufferCounter_HighConcurrency(t *testing.T) {
	var counter ebpfwatch.PaddedRingBufferCounter
	const numGoroutines = 50
	const incrementsPerGoroutine = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < incrementsPerGoroutine; j++ {
				counter.IncTotalEvents()
				counter.AddProcTime(150)
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < incrementsPerGoroutine; j++ {
				counter.IncLostEvents()
			}
		}()
	}

	wg.Wait()

	expectedTotal := uint64(numGoroutines * incrementsPerGoroutine)
	expectedLost := uint64(numGoroutines * incrementsPerGoroutine)

	if got := counter.TotalEvents.Load(); got != expectedTotal {
		t.Errorf("expected TotalEvents %d, got %d", expectedTotal, got)
	}
	if got := counter.LostEvents.Load(); got != expectedLost {
		t.Errorf("expected LostEvents %d, got %d", expectedLost, got)
	}
}

func TestProbeHookType_Validation(t *testing.T) {
	validHooks := []ebpfwatch.ProbeHookType{
		ebpfwatch.HookTracepoint,
		ebpfwatch.HookFentry,
		ebpfwatch.HookFexit,
		ebpfwatch.HookKprobeRaw,
	}

	for _, h := range validHooks {
		if !h.IsValid() {
			t.Errorf("expected hook type %s to be valid", h)
		}
	}

	invalidHook := ebpfwatch.ProbeHookType("uprobe_unknown")
	if invalidHook.IsValid() {
		t.Errorf("expected invalid hook type to be rejected")
	}
}

func TestVolatileSymbol_Rejection(t *testing.T) {
	volatileSymbols := []string{
		"do_exit",
		"schedule",
		"__schedule",
		"finish_task_switch",
		"panic",
	}

	for _, sym := range volatileSymbols {
		err := ebpfwatch.CheckSymbolSafety(ebpfwatch.HookKprobeRaw, sym)
		if err == nil {
			t.Errorf("expected volatile symbol %s to be rejected for kprobe_raw", sym)
		}
	}

	// Tracepoints on the same or other symbols are allowed
	err := ebpfwatch.CheckSymbolSafety(ebpfwatch.HookTracepoint, "raw_syscalls:sys_enter")
	if err != nil {
		t.Errorf("expected tracepoint sys_enter to be safe, got: %v", err)
	}
}

func TestRingBufferTelemetry_Evaluation(t *testing.T) {
	telemetry := ebpfwatch.RingBufferTelemetry{
		ProbeName:       "sys_enter_execve",
		TotalEvents:     10000,
		LostEvents:      5,
		BufferUsagePct:  75.5,
		AvgProcTimeNs:   1200,
		SampleTimestamp: time.Now().UTC(),
		AllocatedPages:  256,
	}

	if !telemetry.IsOverloaded() {
		t.Errorf("expected telemetry with 75.5%% usage and 5 lost events to be overloaded")
	}

	recommendedPages := telemetry.CalculateExpandedPages()
	if recommendedPages != 512 {
		t.Errorf("expected expanded pages to double from 256 to 512, got %d", recommendedPages)
	}
}
