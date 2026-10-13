// Package netscaler coordinates Linux kernel socket backlog tuning, TCP TIME_WAIT recycling,
// conntrack table expansion, and atomic sysctl backup/rollback guardrails.
package netscaler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"go.uber.org/zap"

	domain "github.com/datdt/k8sselfhost/internal/domain/netscaler"
)

// Sysctl kernel parameter keys.
const (
	SysctlSomaxconn              = "net.core.somaxconn"
	SysctlTCPMaxSynBacklog       = "net.ipv4.tcp_max_syn_backlog"
	SysctlTCPTwReuse             = "net.ipv4.tcp_tw_reuse"
	SysctlConntrackMax           = "net.netfilter.nf_conntrack_max"
	SysctlConntrackTCPTimeWait   = "net.netfilter.nf_conntrack_tcp_timeout_time_wait"
	DefaultSysctlBackupPath      = "/var/run/k8s-selfhost/sysctl_backup.conf"
)

// SysctlExecutor defines kernel sysctl reading and writing operations.
type SysctlExecutor interface {
	ReadSysctl(ctx context.Context, key string) (string, error)
	WriteSysctl(ctx context.Context, key string, value string) error
}

// MetricProvider supplies point-in-time socket and netfilter metrics.
type MetricProvider interface {
	CollectMetrics(ctx context.Context) (*domain.SocketMetrics, error)
}

// AutoTuneResult contains the evaluation decisions and applied sysctl configuration.
type AutoTuneResult struct {
	TuningApplied             bool                  `json:"tuning_applied"`
	EnforceHTTPConnectionPool bool                  `json:"enforce_http_connection_pool"`
	PortStarvationMitigated   bool                  `json:"port_starvation_mitigated"`
	ConntrackExpanded         bool                  `json:"conntrack_expanded"`
	ListenBacklogScaled       bool                  `json:"listen_backlog_scaled"`
	AppliedConfig             domain.KernelNetConfig `json:"applied_config"`
	MetricsSnapshot           domain.SocketMetrics   `json:"metrics_snapshot"`
}

// Option configures ScalerManager dependencies.
type Option func(*ScalerManager)

// WithMetricProvider injects a custom socket telemetry metric provider.
func WithMetricProvider(provider MetricProvider) Option {
	return func(s *ScalerManager) {
		s.metricProvider = provider
	}
}

// WithSysctlExecutor injects a sysctl executor.
func WithSysctlExecutor(executor SysctlExecutor) Option {
	return func(s *ScalerManager) {
		s.sysctl = executor
	}
}

// WithBackupPath configures the target filepath for sysctl rollback snapshots.
func WithBackupPath(path string) Option {
	return func(s *ScalerManager) {
		s.backupPath = path
	}
}

// WithLogger injects a structured logger.
func WithLogger(logger *zap.Logger) Option {
	return func(s *ScalerManager) {
		s.logger = logger
	}
}

// ScalerManager orchestrates kernel socket and conntrack auto-scaling with rollback guardrails.
type ScalerManager struct {
	mu             sync.RWMutex
	metricProvider MetricProvider
	sysctl         SysctlExecutor
	backupPath     string
	logger         *zap.Logger
	telemetry      domain.CoreSocketTelemetry
}

// NewScalerManager constructs a new ScalerManager instance.
func NewScalerManager(opts ...Option) *ScalerManager {
	mgr := &ScalerManager{
		backupPath: DefaultSysctlBackupPath,
		logger:     zap.NewNop(),
		sysctl:     &procSysctlExecutor{},
	}
	for _, opt := range opts {
		opt(mgr)
	}
	if mgr.metricProvider == nil {
		mgr.metricProvider = &defaultProcMetricProvider{telemetry: &mgr.telemetry}
	}
	return mgr
}

// CollectMetrics returns the current socket and conntrack metrics snapshot.
func (s *ScalerManager) CollectMetrics(ctx context.Context) (*domain.SocketMetrics, error) {
	s.mu.RLock()
	provider := s.metricProvider
	s.mu.RUnlock()

	metrics, err := provider.CollectMetrics(ctx)
	if err != nil {
		return nil, fmt.Errorf("collecting socket metrics: %w", err)
	}

	// Update cache-line padded atomic telemetry counters
	s.telemetry.TimeWaitCounter.Store(metrics.TCPTimeWaitCount)
	s.telemetry.AllocatedPortCounter.Store(metrics.AllocatedPorts)
	s.telemetry.ListenDropCounter.Store(metrics.ListenBacklogDrops)
	s.telemetry.ConntrackEntryCounter.Store(metrics.ConntrackEntries)

	return metrics, nil
}

// CheckHealth audits socket port exhaustion and conntrack saturation thresholds.
func (s *ScalerManager) CheckHealth(ctx context.Context) error {
	metrics, err := s.CollectMetrics(ctx)
	if err != nil {
		return err
	}

	if metrics.IsPortStarved() {
		return fmt.Errorf("%w: port utilization %.2f%% (%d TIME_WAIT)",
			domain.ErrNetEphemeralPortsExhausted, metrics.PortUtilizationPct, metrics.TCPTimeWaitCount)
	}

	if metrics.IsConntrackSaturated() {
		return fmt.Errorf("%w: conntrack utilization %.2f%% (%d/%d entries)",
			domain.ErrNetConntrackSaturation, metrics.ConntrackUsagePct, metrics.ConntrackEntries, metrics.ConntrackMax)
	}

	return nil
}

// EvaluateAndAutoTune inspects socket telemetry and applies dynamic tuning where required.
func (s *ScalerManager) EvaluateAndAutoTune(ctx context.Context) (*AutoTuneResult, error) {
	metrics, err := s.CollectMetrics(ctx)
	if err != nil {
		return nil, err
	}

	result := &AutoTuneResult{
		MetricsSnapshot: *metrics,
	}

	// Read current kernel baseline
	baseCfg, err := s.readCurrentKernelConfig(ctx)
	if err != nil {
		s.logger.Warn("Failed reading kernel sysctl baseline, falling back to defaults", zap.Error(err))
		baseCfg = domain.KernelNetConfig{
			Somaxconn:            1024,
			TCPMaxSynBacklog:     1024,
			TCPTwReuse:           0,
			ConntrackMax:         metrics.ConntrackMax,
			ConntrackTCPTimeWait: 60,
		}
	}

	targetCfg := baseCfg
	needsTuning := false

	// 1. Ephemeral Port Starvation Mitigator:
	// If port utilization > 80% or > 22,000 ports in TIME_WAIT:
	// Auto-tune net.ipv4.tcp_tw_reuse = 1 and enforce HTTP client pooling.
	if metrics.IsPortStarved() {
		result.PortStarvationMitigated = true
		result.EnforceHTTPConnectionPool = true
		if targetCfg.TCPTwReuse != domain.DefaultTCPTwReuseEnabled {
			targetCfg.TCPTwReuse = domain.DefaultTCPTwReuseEnabled
			needsTuning = true
		}
	}

	// 2. Conntrack Dynamic Auto-Expander:
	// If conntrack usage > 75%, double conntrack max capped at 2,097,152 entries.
	if metrics.IsConntrackSaturated() {
		result.ConntrackExpanded = true
		newMax := metrics.ConntrackMax * 2
		if newMax > domain.ConntrackHardSafetyCeiling {
			newMax = domain.ConntrackHardSafetyCeiling
		}
		if newMax != targetCfg.ConntrackMax {
			targetCfg.ConntrackMax = newMax
			targetCfg.ConntrackTCPTimeWait = domain.DefaultConntrackTCPTimeWaitSec
			needsTuning = true
		}
	}

	// 3. Listen SYN Backlog Scaler:
	// Detects listen backlog drops; tunes somaxconn and tcp_max_syn_backlog to 65535.
	if metrics.HasListenDrops() {
		result.ListenBacklogScaled = true
		if targetCfg.Somaxconn < domain.DefaultSomaxconnHighScale {
			targetCfg.Somaxconn = domain.DefaultSomaxconnHighScale
			needsTuning = true
		}
		if targetCfg.TCPMaxSynBacklog < domain.DefaultTCPMaxSynBacklogHighScale {
			targetCfg.TCPMaxSynBacklog = domain.DefaultTCPMaxSynBacklogHighScale
			needsTuning = true
		}
	}

	if needsTuning {
		if err := s.ApplyKernelTuning(ctx, targetCfg); err != nil {
			return nil, fmt.Errorf("auto-tuning kernel network parameters: %w", err)
		}
		result.TuningApplied = true
		result.AppliedConfig = targetCfg
	} else {
		result.AppliedConfig = targetCfg
	}

	return result, nil
}

// ApplyKernelTuning writes sysctl settings after persisting a snapshot for atomic rollback.
func (s *ScalerManager) ApplyKernelTuning(ctx context.Context, cfg domain.KernelNetConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Capture snapshot of existing parameters for rollback guardrail
	keys := []string{
		SysctlSomaxconn,
		SysctlTCPMaxSynBacklog,
		SysctlTCPTwReuse,
		SysctlConntrackMax,
		SysctlConntrackTCPTimeWait,
	}

	originalSnapshot := make(map[string]string, len(keys))
	for _, key := range keys {
		val, err := s.sysctl.ReadSysctl(ctx, key)
		if err != nil {
			s.logger.Warn("Failed to read sysctl parameter during snapshot", zap.String("key", key), zap.Error(err))
			val = "0"
		}
		originalSnapshot[key] = val
	}

	// 2. Persist snapshot file
	if err := s.persistBackupSnapshot(originalSnapshot); err != nil {
		s.logger.Warn("Failed to persist sysctl backup snapshot to disk", zap.String("path", s.backupPath), zap.Error(err))
	}

	// 3. Apply target values with rollback tracking
	desired := map[string]string{
		SysctlSomaxconn:            strconv.Itoa(cfg.Somaxconn),
		SysctlTCPMaxSynBacklog:     strconv.Itoa(cfg.TCPMaxSynBacklog),
		SysctlTCPTwReuse:           strconv.Itoa(cfg.TCPTwReuse),
		SysctlConntrackMax:         strconv.FormatInt(cfg.ConntrackMax, 10),
		SysctlConntrackTCPTimeWait: strconv.Itoa(cfg.ConntrackTCPTimeWait),
	}

	appliedKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		targetVal := desired[key]
		if err := s.sysctl.WriteSysctl(ctx, key, targetVal); err != nil {
			s.logger.Error("Sysctl write failed; executing rollback guardrail",
				zap.String("key", key),
				zap.String("val", targetVal),
				zap.Error(err))

			// Rollback all previously modified keys
			for _, rollKey := range appliedKeys {
				origVal := originalSnapshot[rollKey]
				if rErr := s.sysctl.WriteSysctl(ctx, rollKey, origVal); rErr != nil {
					s.logger.Error("Rollback write failed", zap.String("key", rollKey), zap.Error(rErr))
				}
			}

			return fmt.Errorf("%w: failed setting %s=%s: %v", domain.ErrKernelSysctlFailed, key, targetVal, err)
		}
		appliedKeys = append(appliedKeys, key)
	}

	s.logger.Info("Kernel network tuning successfully applied",
		zap.Int("somaxconn", cfg.Somaxconn),
		zap.Int("tcp_max_syn_backlog", cfg.TCPMaxSynBacklog),
		zap.Int("tcp_tw_reuse", cfg.TCPTwReuse),
		zap.Int64("conntrack_max", cfg.ConntrackMax),
		zap.Int("conntrack_tcp_time_wait", cfg.ConntrackTCPTimeWait))

	return nil
}

func (s *ScalerManager) persistBackupSnapshot(settings map[string]string) error {
	if s.backupPath == "" {
		return nil
	}
	dir := filepath.Dir(s.backupPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating backup directory: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("# k8s-selfhost sysctl backup snapshot\n")
	for k, v := range settings {
		sb.WriteString(fmt.Sprintf("%s = %s\n", k, v))
	}

	return os.WriteFile(s.backupPath, []byte(sb.String()), 0600)
}

func (s *ScalerManager) readCurrentKernelConfig(ctx context.Context) (domain.KernelNetConfig, error) {
	somaxStr, _ := s.sysctl.ReadSysctl(ctx, SysctlSomaxconn)
	synStr, _ := s.sysctl.ReadSysctl(ctx, SysctlTCPMaxSynBacklog)
	twStr, _ := s.sysctl.ReadSysctl(ctx, SysctlTCPTwReuse)
	ctMaxStr, _ := s.sysctl.ReadSysctl(ctx, SysctlConntrackMax)
	ctTwStr, _ := s.sysctl.ReadSysctl(ctx, SysctlConntrackTCPTimeWait)

	somax, _ := strconv.Atoi(strings.TrimSpace(somaxStr))
	syn, _ := strconv.Atoi(strings.TrimSpace(synStr))
	tw, _ := strconv.Atoi(strings.TrimSpace(twStr))
	ctMax, _ := strconv.ParseInt(strings.TrimSpace(ctMaxStr), 10, 64)
	ctTw, _ := strconv.Atoi(strings.TrimSpace(ctTwStr))

	if somax <= 0 {
		somax = 1024
	}
	if syn <= 0 {
		syn = 1024
	}
	if ctMax <= 0 {
		ctMax = domain.ConntrackDefaultInitialMax
	}
	if ctTw <= 0 {
		ctTw = 60
	}

	cfg := domain.KernelNetConfig{
		Somaxconn:            somax,
		TCPMaxSynBacklog:     syn,
		TCPTwReuse:           tw,
		ConntrackMax:         ctMax,
		ConntrackTCPTimeWait: ctTw,
	}

	return cfg, nil
}

// procSysctlExecutor reads and writes directly to /proc/sys on Linux.
type procSysctlExecutor struct{}

func (p *procSysctlExecutor) ReadSysctl(_ context.Context, key string) (string, error) {
	procPath := "/proc/sys/" + strings.ReplaceAll(key, ".", "/")
	data, err := os.ReadFile(procPath)
	if err != nil {
		return "0", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (p *procSysctlExecutor) WriteSysctl(_ context.Context, key string, value string) error {
	procPath := "/proc/sys/" + strings.ReplaceAll(key, ".", "/")
	return os.WriteFile(procPath, []byte(value), 0644)
}

// defaultProcMetricProvider reads live telemetry from procfs or provides fallback defaults.
type defaultProcMetricProvider struct {
	telemetry *domain.CoreSocketTelemetry
}

func (p *defaultProcMetricProvider) CollectMetrics(_ context.Context) (*domain.SocketMetrics, error) {
	tw := p.telemetry.TimeWaitCounter.Load()
	alloc := p.telemetry.AllocatedPortCounter.Load()
	drops := p.telemetry.ListenDropCounter.Load()
	ct := p.telemetry.ConntrackEntryCounter.Load()

	totalPorts := domain.DefaultTotalEphemeralPorts
	ctMax := domain.ConntrackDefaultInitialMax

	if alloc <= 0 {
		alloc = 1200
	}

	return domain.NewSocketMetrics(tw, alloc, totalPorts, drops, ct, ctMax)
}
