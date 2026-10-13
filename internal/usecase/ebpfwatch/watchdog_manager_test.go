package ebpfwatch_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	domain "github.com/datdt/k8sselfhost/internal/domain/ebpfwatch"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/ebpfwatch"
	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

func TestWatchdogManager_AutoExpandOnHighPressure(t *testing.T) {
	ctx := context.Background()
	mgr := usecase.NewWatchdogManager(
		usecase.WithBTFPath("/nonexistent/btf"), // Default non-existent for mock/unit tests
	)

	probeName := "sys_enter_connect"
	mgr.RegisterProbe(probeName, 256)

	// Record telemetry with 75% usage, no lost events
	mgr.RecordTelemetry(probeName, domain.RingBufferTelemetry{
		ProbeName:       probeName,
		TotalEvents:     50000,
		LostEvents:      0,
		BufferUsagePct:  75.0,
		AvgProcTimeNs:   900,
		AllocatedPages:  256,
		SampleTimestamp: time.Now().UTC(),
	})

	telemetryList, err := mgr.AuditRingBufferPressure(ctx)
	if err != nil {
		t.Fatalf("unexpected error auditing pressure: %v", err)
	}

	if len(telemetryList) != 1 {
		t.Fatalf("expected 1 telemetry entry, got %d", len(telemetryList))
	}

	// Buffer should have auto-expanded from 256 to 512
	pages, err := mgr.GetProbePages(probeName)
	if err != nil {
		t.Fatalf("failed to get probe pages: %v", err)
	}
	if pages != 512 {
		t.Errorf("expected probe pages to be doubled to 512, got %d", pages)
	}
}

func TestWatchdogManager_EmergencySamplingOnLostEvents(t *testing.T) {
	ctx := context.Background()
	mgr := usecase.NewWatchdogManager()

	probeName := "tcp_v4_connect"
	mgr.RegisterProbe(probeName, 256)

	// Record lost events (> 0)
	mgr.RecordTelemetry(probeName, domain.RingBufferTelemetry{
		ProbeName:       probeName,
		TotalEvents:     100000,
		LostEvents:      42,
		BufferUsagePct:  85.0,
		AvgProcTimeNs:   1100,
		AllocatedPages:  256,
		SampleTimestamp: time.Now().UTC(),
	})

	_, err := mgr.AuditRingBufferPressure(ctx)
	// Expect ErrRingBufferOverflow to be returned as dropped events occurred
	if err == nil {
		t.Fatalf("expected ErrRingBufferOverflow error when lost events > 0")
	}

	var dErr *domainerrors.DomainError
	if !errors.As(err, &dErr) || dErr.Code != domain.CodeRingBufferOverflow {
		t.Errorf("expected domain error code %s, got: %v", domain.CodeRingBufferOverflow, err)
	}

	// Sampling rate should have dropped from 100% to emergency level (e.g., 25% or lower)
	samplingRate := mgr.GetSamplingRate(probeName)
	if samplingRate >= 100 {
		t.Errorf("expected emergency adaptive sampling to reduce rate below 100%%, got %d%%", samplingRate)
	}

	// Probe buffer pages should also have expanded
	pages, _ := mgr.GetProbePages(probeName)
	if pages != 512 {
		t.Errorf("expected pages to double to 512, got %d", pages)
	}
}

func TestWatchdogManager_CPUQuotaGuardrail(t *testing.T) {
	ctx := context.Background()
	mgr := usecase.NewWatchdogManager()

	probeName := "sched_process_exec"
	mgr.RegisterProbe(probeName, 256)

	// Check normal CPU usage (3.2%)
	err := mgr.EnforceCPUQuota(ctx, probeName, 3.2)
	if err != nil {
		t.Fatalf("unexpected error for 3.2%% CPU usage: %v", err)
	}
	if mgr.IsLightweightMode(probeName) {
		t.Errorf("expected probe not to be in lightweight mode at 3.2%% CPU")
	}

	// Exceed CPU quota (5.8% > 5.0%)
	err = mgr.EnforceCPUQuota(ctx, probeName, 5.8)
	if err == nil {
		t.Fatalf("expected ErrCPUQuotaExceeded for 5.8%% CPU usage")
	}

	var dErr *domainerrors.DomainError
	if !errors.As(err, &dErr) || dErr.Code != domain.CodeCPUQuotaExceeded {
		t.Errorf("expected CodeCPUQuotaExceeded, got: %v", err)
	}

	// Probe must be automatically downgraded to lightweight mode
	if !mgr.IsLightweightMode(probeName) {
		t.Errorf("expected probe to be downgraded to lightweight mode after CPU violation")
	}
}

func TestWatchdogManager_HookSafetyAndBTF(t *testing.T) {
	// Test case 1: BTF absent
	mgrNoBTF := usecase.NewWatchdogManager(
		usecase.WithBTFChecker(func(context.Context) bool { return false }),
	)

	// Raw kprobe without BTF support should be rejected or restricted
	err := mgrNoBTF.ValidateHookSafety(domain.HookKprobeRaw, "tcp_v4_rcv")
	if err == nil || !errors.Is(err, domain.ErrBTFMissing) {
		t.Errorf("expected ErrBTFMissing when attaching raw kprobe without BTF support, got: %v", err)
	}

	// Test case 2: BTF present
	mgrWithBTF := usecase.NewWatchdogManager(
		usecase.WithBTFChecker(func(context.Context) bool { return true }),
	)

	// Volatile symbol rejection
	err = mgrWithBTF.ValidateHookSafety(domain.HookKprobeRaw, "do_exit")
	if !errors.Is(err, domain.ErrUnstableKprobeRejected) {
		t.Errorf("expected ErrUnstableKprobeRejected for do_exit, got: %v", err)
	}

	err = mgrWithBTF.ValidateHookSafety(domain.HookKprobeRaw, "schedule")
	if !errors.Is(err, domain.ErrUnstableKprobeRejected) {
		t.Errorf("expected ErrUnstableKprobeRejected for schedule, got: %v", err)
	}

	// Safe tracepoint
	err = mgrWithBTF.ValidateHookSafety(domain.HookTracepoint, "raw_syscalls:sys_enter")
	if err != nil {
		t.Errorf("expected tracepoint sys_enter to be safe, got: %v", err)
	}

	// Safe fentry with BTF
	err = mgrWithBTF.ValidateHookSafety(domain.HookFentry, "tcp_v4_connect")
	if err != nil {
		t.Errorf("expected fentry tcp_v4_connect to be safe with BTF, got: %v", err)
	}
}

func TestWatchdogManager_HighConcurrency(t *testing.T) {
	ctx := context.Background()
	mgr := usecase.NewWatchdogManager()
	probeName := "concurrent_probe"
	mgr.RegisterProbe(probeName, 256)

	const numGoroutines = 30
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				mgr.RecordTelemetry(probeName, domain.RingBufferTelemetry{
					ProbeName:       probeName,
					TotalEvents:     uint64(idx*50 + j),
					LostEvents:      0,
					BufferUsagePct:  50.0 + float64(j%25),
					AvgProcTimeNs:   800,
					AllocatedPages:  256,
					SampleTimestamp: time.Now().UTC(),
				})
				_, _ = mgr.AuditRingBufferPressure(ctx)
			}
		}(i)
	}

	wg.Wait()

	telemetryList, err := mgr.AuditRingBufferPressure(ctx)
	if err != nil {
		t.Fatalf("unexpected error auditing pressure: %v", err)
	}
	if len(telemetryList) != 1 {
		t.Errorf("expected 1 telemetry entry, got %d", len(telemetryList))
	}
}
