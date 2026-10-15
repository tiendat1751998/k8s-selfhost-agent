// Package ebpfwatch provides usecase orchestration for eBPF ring buffer pressure auditing,
// dynamic memory expansion, emergency adaptive sampling, kernel probe validation, and CPU guardrails.
package ebpfwatch

import (
	"context"
	"fmt"
	"os"
	"sync"

	"go.uber.org/zap"

	domain "github.com/datdt/k8sselfhost/internal/domain/ebpfwatch"
	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

var _ domain.KernelSafetyValidator = (*WatchdogManager)(nil)

// Option configures WatchdogManager.
type Option func(*WatchdogManager)

// BTFCheckerFunc is an abstraction for determining if BTF /vmlinux is available in the kernel.
type BTFCheckerFunc func(ctx context.Context) bool

type probeState struct {
	mu            sync.RWMutex
	name          string
	pages         int
	counter       domain.PaddedRingBufferCounter
	sampling      domain.PaddedSamplingController
	lastTelemetry domain.RingBufferTelemetry
}

// WatchdogManager implements domain.KernelSafetyValidator and orchestrates
// eBPF kernel runtime monitoring, safety guardrails, and telemetry adaptation.
type WatchdogManager struct {
	mu         sync.RWMutex
	probes     map[string]*probeState
	btfChecker BTFCheckerFunc
	logger     *zap.Logger
}

// WithLogger sets a zap logger for WatchdogManager.
func WithLogger(logger *zap.Logger) Option {
	return func(m *WatchdogManager) {
		if logger != nil {
			m.logger = logger
		}
	}
}

// WithBTFChecker sets a custom BTF check callback.
func WithBTFChecker(fn BTFCheckerFunc) Option {
	return func(m *WatchdogManager) {
		m.btfChecker = fn
	}
}

// VerifyBTFIntegrity checks that a BTF/vmlinux file exists, has valid size,
// and begins with either ELF magic (0x7F 'E' 'L' 'F') or raw BTF magic (0xeb9f).
func VerifyBTFIntegrity(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("opening btf file %s: %w", path, err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return false, fmt.Errorf("statting btf file %s: %w", path, err)
	}
	if stat.Size() < 4 {
		return false, fmt.Errorf("btf file %s too small (%d bytes)", path, stat.Size())
	}

	header := make([]byte, 4)
	if _, err := f.Read(header); err != nil {
		return false, fmt.Errorf("reading btf file header %s: %w", path, err)
	}

	// Check ELF magic: 0x7f, 'E', 'L', 'F'
	if header[0] == 0x7f && header[1] == 'E' && header[2] == 'L' && header[3] == 'F' {
		return true, nil
	}

	// Check raw BTF magic: 0xeb9f (little-endian or big-endian)
	if (header[0] == 0x9f && header[1] == 0xeb) || (header[0] == 0xeb && header[1] == 0x9f) {
		return true, nil
	}

	return false, fmt.Errorf("invalid btf magic in %s: %02x%02x%02x%02x", path, header[0], header[1], header[2], header[3])
}

// WithBTFPath sets a filesystem path to inspect for BTF vmlinux and verifies its integrity.
func WithBTFPath(path string) Option {
	return func(m *WatchdogManager) {
		m.btfChecker = func(ctx context.Context) bool {
			valid, _ := VerifyBTFIntegrity(path)
			return valid
		}
	}
}

// NewWatchdogManager creates a new WatchdogManager with sane defaults.
func NewWatchdogManager(opts ...Option) *WatchdogManager {
	mgr := &WatchdogManager{
		probes: make(map[string]*probeState),
		logger: zap.NewNop(),
		btfChecker: func(ctx context.Context) bool {
			valid, _ := VerifyBTFIntegrity("/sys/kernel/btf/vmlinux")
			return valid
		},
	}

	for _, opt := range opts {
		opt(mgr)
	}

	return mgr
}

// RegisterProbe registers a probe with initial buffer page allocation.
func (m *WatchdogManager) RegisterProbe(name string, initialPages int) {
	if initialPages <= 0 {
		initialPages = domain.DefaultRingBufferPages
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	state := &probeState{
		name:  name,
		pages: initialPages,
	}
	state.sampling.SetSamplingRate(100)
	state.sampling.SetLightweight(false)
	m.probes[name] = state
}

// RecordTelemetry records real-time telemetry metrics for a probe.
func (m *WatchdogManager) RecordTelemetry(probe string, t domain.RingBufferTelemetry) {
	m.mu.Lock()
	state, exists := m.probes[probe]
	if !exists {
		state = &probeState{
			name:  probe,
			pages: t.AllocatedPages,
		}
		if state.pages <= 0 {
			state.pages = domain.DefaultRingBufferPages
		}
		state.sampling.SetSamplingRate(100)
		m.probes[probe] = state
	}
	m.mu.Unlock()

	state.mu.Lock()
	defer state.mu.Unlock()

	state.lastTelemetry = t
	if t.AllocatedPages > 0 {
		state.pages = t.AllocatedPages
	}
	state.counter.TotalEvents.Store(t.TotalEvents)
	state.counter.LostEvents.Store(t.LostEvents)
	if t.AvgProcTimeNs > 0 && t.TotalEvents > 0 {
		state.counter.ProcTimeNs.Store(t.AvgProcTimeNs * int64(t.TotalEvents))
	}
}

// GetProbePages returns the current page allocation of a probe.
func (m *WatchdogManager) GetProbePages(probe string) (int, error) {
	m.mu.RLock()
	state, exists := m.probes[probe]
	m.mu.RUnlock()

	if !exists {
		return 0, domainerrors.NewNotFound("probe", probe)
	}

	state.mu.RLock()
	defer state.mu.RUnlock()
	return state.pages, nil
}

// GetSamplingRate returns the current event sampling rate percentage of a probe.
func (m *WatchdogManager) GetSamplingRate(probe string) uint32 {
	m.mu.RLock()
	state, exists := m.probes[probe]
	m.mu.RUnlock()

	if !exists {
		return 100
	}
	return state.sampling.GetSamplingRate()
}

// IsLightweightMode returns whether lightweight audit mode is active for a probe.
func (m *WatchdogManager) IsLightweightMode(probe string) bool {
	m.mu.RLock()
	state, exists := m.probes[probe]
	m.mu.RUnlock()

	if !exists {
		return false
	}
	return state.sampling.IsLightweight()
}

// CheckBTFSupport checks if the Linux kernel has BTF (/sys/kernel/btf/vmlinux) support.
func (m *WatchdogManager) CheckBTFSupport(ctx context.Context) (bool, error) {
	if m.btfChecker != nil {
		return m.btfChecker(ctx), nil
	}
	_, err := os.Stat("/sys/kernel/btf/vmlinux")
	return err == nil, nil
}

// AutoExpandRingBuffer doubles the ring buffer pages for a probe, capped at MaxRingBufferPages.
func (m *WatchdogManager) AutoExpandRingBuffer(ctx context.Context, probe string, newPages int) error {
	m.mu.RLock()
	state, exists := m.probes[probe]
	m.mu.RUnlock()

	if !exists {
		return domainerrors.NewNotFound("probe", probe)
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	if newPages <= 0 {
		newPages = state.pages * 2
	}
	if newPages&(newPages-1) != 0 {
		return fmt.Errorf("ring buffer pages must be a power of two, got %d", newPages)
	}
	if newPages > domain.MaxRingBufferPages {
		newPages = domain.MaxRingBufferPages
	}
	if newPages < domain.MinRingBufferPages {
		newPages = domain.MinRingBufferPages
	}

	m.logger.Info("Expanding eBPF ring buffer pages",
		zap.String("probe", probe),
		zap.Int("old_pages", state.pages),
		zap.Int("new_pages", newPages),
	)

	state.pages = newPages
	state.lastTelemetry.AllocatedPages = newPages
	return nil
}

// AuditRingBufferPressure collects all ring buffer telemetries, automatically expands
// pages when saturation >= 70% or dropped events are detected, and activates emergency sampling.
func (m *WatchdogManager) AuditRingBufferPressure(ctx context.Context) ([]domain.RingBufferTelemetry, error) {
	m.mu.RLock()
	probes := make([]*probeState, 0, len(m.probes))
	for _, p := range m.probes {
		probes = append(probes, p)
	}
	m.mu.RUnlock()

	var hasOverflow bool
	telemetryList := make([]domain.RingBufferTelemetry, 0, len(probes))

	for _, p := range probes {
		p.mu.Lock()
		t := p.lastTelemetry
		t.AllocatedPages = p.pages
		t.SamplingRatePct = float64(p.sampling.GetSamplingRate())
		telemetryList = append(telemetryList, t)

		// Pressure and overflow guardrail checks
		if t.IsOverloaded() {
			expanded := t.CalculateExpandedPages()
			if expanded > p.pages {
				m.logger.Warn("High buffer saturation detected; triggering auto-expansion",
					zap.String("probe", p.name),
					zap.Float64("usage_pct", t.BufferUsagePct),
					zap.Uint64("lost_events", t.LostEvents),
					zap.Int("current_pages", p.pages),
					zap.Int("expanded_pages", expanded),
				)
				p.pages = expanded
			}

			// If events were dropped in kernel space, activate emergency adaptive sampling and report overflow
			if t.LostEvents > 0 {
				hasOverflow = true
				m.logger.Error("Kernel ringbuffer dropped events; activating emergency sampling",
					zap.String("probe", p.name),
					zap.Uint64("lost_events", t.LostEvents),
				)
				p.sampling.SetSamplingRate(25) // Reduce to 25% sampling rate
			}
		}
		p.mu.Unlock()
	}

	if hasOverflow {
		return telemetryList, domain.ErrRingBufferOverflow
	}

	return telemetryList, nil
}

// ValidateHookSafety validates whether a probe hook type and kernel symbol are safe to attach to.
func (m *WatchdogManager) ValidateHookSafety(hookType domain.ProbeHookType, symbol string) error {
	if !hookType.IsValid() {
		return domainerrors.NewValidation("hook_type", fmt.Sprintf("unsupported hook type: %s", hookType))
	}

	// Volatile symbol check
	if err := domain.CheckSymbolSafety(hookType, symbol); err != nil {
		return err
	}

	// BTF validation for kprobe_raw, fentry, and fexit
	btfSupported, _ := m.CheckBTFSupport(context.Background())
	if !btfSupported {
		if hookType == domain.HookKprobeRaw || hookType == domain.HookFentry || hookType == domain.HookFexit {
			return domain.ErrBTFMissing
		}
	}

	return nil
}

// EnforceCPUQuota enforces the 5% node CPU quota guardrail for eBPF probes.
func (m *WatchdogManager) EnforceCPUQuota(ctx context.Context, probe string, cpuUsagePct float64) error {
	m.mu.RLock()
	state, exists := m.probes[probe]
	m.mu.RUnlock()

	if !exists {
		return domainerrors.NewNotFound("probe", probe)
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	if cpuUsagePct > domain.MaxEBPFCPUQuotaPct {
		m.logger.Warn("eBPF CPU utilization exceeded 5% quota; downgrading to lightweight mode",
			zap.String("probe", probe),
			zap.Float64("cpu_usage_pct", cpuUsagePct),
		)
		state.sampling.SetLightweight(true)
		state.sampling.SetSamplingRate(10) // 10% sampling in lightweight audit mode
		return domain.ErrCPUQuotaExceeded
	}

	return nil
}
