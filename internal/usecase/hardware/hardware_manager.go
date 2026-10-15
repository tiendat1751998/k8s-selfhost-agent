// Package hardware implements application usecases and watchdog orchestration for bare-metal
// server hardware, including ECC memory EDAC auditing, CPU thermal throttling remediation,
// and dual-PSU power redundancy protection.
package hardware

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	domain "github.com/datdt/k8sselfhost/internal/domain/hardware"
)

// Config configures the bare-metal hardware manager usecase.
type Config struct {
	NodeName                   string
	ECCErrorRateHourlyLimit    float64
	MaxPackageTempC            float64
	FrequencyThrottleThreshold float64
	RAPLPowerCapRatio          float64
	DrainGracePeriod           time.Duration
}

// DefaultConfig provides conservative production defaults for enterprise server hardware.
func DefaultConfig() Config {
	return Config{
		NodeName:                   "localhost",
		ECCErrorRateHourlyLimit:    domain.ECCErrorRateThresholdPerHour,
		MaxPackageTempC:            domain.MaxAllowedPackageTempC,
		FrequencyThrottleThreshold: domain.FrequencyThrottleRatio,
		RAPLPowerCapRatio:          domain.RAPLPowerCapRatio,
		DrainGracePeriod:           domain.DefaultDrainGracePeriod,
	}
}

// HardwareManager orchestrates low-level bare-metal hardware health checks and automated remediations.
type HardwareManager struct {
	mu              sync.RWMutex
	cfg             Config
	edacReader      domain.EDACReader
	thermalReader   domain.CPUThermalReader
	psuReader       domain.PowerSupplyReader
	remediator      domain.NodeRemediator
	raplController  domain.RAPLController
	alertSink       domain.HardwareAlertSink
	logger          *zap.Logger
	lastAuditStatus map[string]interface{}
}

// NewHardwareManager creates a new HardwareManager instance.
func NewHardwareManager(
	cfg Config,
	edacReader domain.EDACReader,
	thermalReader domain.CPUThermalReader,
	psuReader domain.PowerSupplyReader,
	remediator domain.NodeRemediator,
	raplController domain.RAPLController,
	alertSink domain.HardwareAlertSink,
	loggers ...*zap.Logger,
) *HardwareManager {
	if cfg.NodeName == "" {
		cfg.NodeName = "localhost"
	}
	if cfg.ECCErrorRateHourlyLimit <= 0 {
		cfg.ECCErrorRateHourlyLimit = domain.ECCErrorRateThresholdPerHour
	}
	if cfg.MaxPackageTempC <= 0 {
		cfg.MaxPackageTempC = domain.MaxAllowedPackageTempC
	}
	if cfg.FrequencyThrottleThreshold <= 0 {
		cfg.FrequencyThrottleThreshold = domain.FrequencyThrottleRatio
	}
	if cfg.RAPLPowerCapRatio <= 0 {
		cfg.RAPLPowerCapRatio = domain.RAPLPowerCapRatio
	}
	if cfg.DrainGracePeriod <= 0 {
		cfg.DrainGracePeriod = domain.DefaultDrainGracePeriod
	}

	var l *zap.Logger
	if len(loggers) > 0 && loggers[0] != nil {
		l = loggers[0]
	} else {
		l = zap.NewNop()
	}

	return &HardwareManager{
		cfg:             cfg,
		edacReader:      edacReader,
		thermalReader:   thermalReader,
		psuReader:       psuReader,
		remediator:      remediator,
		raplController:  raplController,
		alertSink:       alertSink,
		logger:          l,
		lastAuditStatus: make(map[string]interface{}),
	}
}

// AuditECCMemoryErrors inspects EDAC memory controller error counters.
// If correctable error rate exceeds the limit (>100 errors/hr) on any DIMM slot, it marks the node
// degraded, labels node.kubernetes.io/hardware-degraded=true, executes graceful cordon/drain (120s),
// emits a P0 SRE alert, and returns ErrECCMemoryCorrectableSurge.
func (m *HardwareManager) AuditECCMemoryErrors(ctx context.Context) ([]domain.ECCDIMMStatus, error) {
	if m.edacReader == nil {
		return nil, errors.New("hardware edac reader is uninitialized")
	}

	dimms, err := m.edacReader.ReadDIMMErrors(ctx)
	if err != nil {
		m.logger.Error("failed to read DIMM errors from EDAC sysfs", zap.Error(err))
		return nil, fmt.Errorf("reading edac errors: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	var degradedDIMMs []domain.ECCDIMMStatus
	hourlyLimit := m.cfg.ECCErrorRateHourlyLimit

	for i := range dimms {
		hourlyRate := dimms[i].ErrorRatePerMin * 60.0
		if hourlyRate > hourlyLimit || dimms[i].CorrectableCount > uint64(hourlyLimit) && dimms[i].ErrorRatePerMin > (hourlyLimit/60.0) {
			dimms[i].IsDegraded = true
			degradedDIMMs = append(degradedDIMMs, dimms[i])
		}
	}

	if len(degradedDIMMs) > 0 {
		firstDegraded := degradedDIMMs[0]
		m.logger.Warn("ECC memory correctable error surge detected on physical DIMM",
			zap.String("slot", firstDegraded.SlotID),
			zap.Float64("error_rate_per_min", firstDegraded.ErrorRatePerMin),
			zap.Uint64("correctable_count", firstDegraded.CorrectableCount),
		)

		if m.remediator != nil {
			if cordonErr := m.remediator.CordonNode(ctx, m.cfg.NodeName); cordonErr != nil {
				m.logger.Error("failed to cordon degraded node", zap.String("node", m.cfg.NodeName), zap.Error(cordonErr))
			}
			if labelErr := m.remediator.LabelNode(ctx, m.cfg.NodeName, domain.DegradedNodeLabelKey, domain.DegradedNodeLabelValue); labelErr != nil {
				m.logger.Error("failed to label degraded node", zap.String("node", m.cfg.NodeName), zap.Error(labelErr))
			}
			if drainErr := m.remediator.DrainNode(ctx, m.cfg.NodeName, m.cfg.DrainGracePeriod); drainErr != nil {
				m.logger.Error("failed to drain degraded node", zap.String("node", m.cfg.NodeName), zap.Error(drainErr))
			}
		}

		if m.alertSink != nil {
			alertMsg := fmt.Sprintf("P0 ALERT: Critical ECC CE error rate (%.2f/hr) on physical DIMM slot %s. Node %s cordoned and drained.",
				firstDegraded.ErrorRatePerMin*60.0, firstDegraded.SlotID, m.cfg.NodeName)
			_ = m.alertSink.EmitAlert(ctx, domain.HardwareAlert{
				Severity:  domain.SeverityP0,
				Component: "EDAC_MEMORY",
				Target:    firstDegraded.SlotID,
				Message:   alertMsg,
				Timestamp: time.Now(),
			})
		}

		return dimms, domain.ErrECCMemoryCorrectableSurge
	}

	return dimms, nil
}

// AuditCPUThermalThrottling checks coretemp and MSR thermal throttling telemetry.
// If package temp > 85°C OR current clock frequency < 50% of base frequency, it marks the CPU throttled,
// applies node.kubernetes.io/thermal-throttled:NoSchedule taint, evicts latency-critical workloads,
// and returns ErrCPUThermalThrottled.
func (m *HardwareManager) AuditCPUThermalThrottling(ctx context.Context) (*domain.CPUThermalStatus, error) {
	if m.thermalReader == nil {
		return nil, errors.New("hardware thermal reader is uninitialized")
	}

	status, err := m.thermalReader.ReadCPUThermal(ctx)
	if err != nil {
		m.logger.Error("failed to read CPU thermal telemetry", zap.Error(err))
		return nil, fmt.Errorf("reading cpu thermal: %w", err)
	}
	if status == nil {
		return nil, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	isOverTemp := status.PackageTempC > m.cfg.MaxPackageTempC
	isFreqDegraded := status.BaseFrequencyMHz > 0 && status.CurrentFrequencyMHz < (m.cfg.FrequencyThrottleThreshold*status.BaseFrequencyMHz)

	if isOverTemp || isFreqDegraded {
		status.IsThrottled = true

		m.logger.Warn("CPU thermal throttling detected",
			zap.Float64("temp_c", status.PackageTempC),
			zap.Float64("current_mhz", status.CurrentFrequencyMHz),
			zap.Float64("base_mhz", status.BaseFrequencyMHz),
		)

		if m.remediator != nil {
			if taintErr := m.remediator.TaintNode(ctx, m.cfg.NodeName, domain.ThermalThrottledTaintKey, "true", domain.ThermalThrottledTaintEffect); taintErr != nil {
				m.logger.Error("failed to taint thermal throttled node", zap.String("node", m.cfg.NodeName), zap.Error(taintErr))
			}
			evicted, evictErr := m.remediator.EvictLatencyCriticalPods(ctx, m.cfg.NodeName)
			if evictErr != nil {
				m.logger.Error("failed to evict latency critical pods", zap.String("node", m.cfg.NodeName), zap.Error(evictErr))
			} else {
				m.logger.Info("evicted latency-critical pods due to CPU thermal throttling", zap.Strings("evicted", evicted))
			}
		}

		if m.alertSink != nil {
			alertMsg := fmt.Sprintf("WARNING: CPU thermal throttling active on node %s (Temp: %.1f°C, Freq: %.0f/%.0f MHz). Applied NoSchedule taint and evicted latency-critical pods.",
				m.cfg.NodeName, status.PackageTempC, status.CurrentFrequencyMHz, status.BaseFrequencyMHz)
			_ = m.alertSink.EmitAlert(ctx, domain.HardwareAlert{
				Severity:  domain.SeverityWarning,
				Component: "CPU_THERMAL",
				Target:    m.cfg.NodeName,
				Message:   alertMsg,
				Timestamp: time.Now(),
			})
		}

		return status, domain.ErrCPUThermalThrottled
	}

	return status, nil
}

// AuditPowerSupplyRedundancy inspects dual-PSU power supply status via IPMI/BMC.
// If redundancy is lost, it immediately alerts Datacenter Ops and caps CPU RAPL power limits to 70%
// to prevent branch circuit breaker trips on the rack PDU.
func (m *HardwareManager) AuditPowerSupplyRedundancy(ctx context.Context) ([]domain.PowerSupplyStatus, error) {
	if m.psuReader == nil {
		return nil, errors.New("hardware psu reader is uninitialized")
	}

	supplies, err := m.psuReader.ReadPowerSupplies(ctx)
	if err != nil {
		m.logger.Error("failed to read power supply redundancy status", zap.Error(err))
		return nil, fmt.Errorf("reading power supplies: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	redundancyLost := false
	for _, psu := range supplies {
		if psu.RedundancyLost || !psu.IsPresent {
			redundancyLost = true
			break
		}
	}

	if redundancyLost {
		m.logger.Warn("Dual-PSU power redundancy lost. Initiating RAPL 70% power clamp to prevent PDU breaker trip",
			zap.String("node", m.cfg.NodeName),
		)

		if m.raplController != nil {
			if capErr := m.raplController.SetRAPLPowerLimit(ctx, m.cfg.RAPLPowerCapRatio); capErr != nil {
				m.logger.Error("failed to apply CPU RAPL power cap", zap.Error(capErr))
			} else {
				m.logger.Info("clamped CPU RAPL power limit to 70% to protect single-feed PDU capacity")
			}
		}

		if m.alertSink != nil {
			alertMsg := fmt.Sprintf("P0 ALERT: Dual-PSU redundancy failed on node %s. Imminent rack PDU overload risk under AVX compute spikes. RAPL clamped to 70%%.",
				m.cfg.NodeName)
			_ = m.alertSink.EmitAlert(ctx, domain.HardwareAlert{
				Severity:  domain.SeverityP0,
				Component: "POWER_SUPPLY",
				Target:    m.cfg.NodeName,
				Message:   alertMsg,
				Timestamp: time.Now(),
			})
		}

		return supplies, domain.ErrPSURedundancyFailed
	}

	return supplies, nil
}

// DrainAndCordonDegradedNode marks the node unschedulable, tags it as degraded, and gracefully drains pods.
func (m *HardwareManager) DrainAndCordonDegradedNode(ctx context.Context, nodeName string, reason string) error {
	targetNode := nodeName
	if targetNode == "" {
		targetNode = m.cfg.NodeName
	}

	if m.remediator == nil {
		return nil
	}

	m.logger.Info("initiating drain and cordon on degraded bare-metal node",
		zap.String("node", targetNode),
		zap.String("reason", reason),
	)

	if err := m.remediator.CordonNode(ctx, targetNode); err != nil {
		return fmt.Errorf("cordoning node %s: %w", targetNode, err)
	}

	if err := m.remediator.LabelNode(ctx, targetNode, domain.DegradedNodeLabelKey, domain.DegradedNodeLabelValue); err != nil {
		return fmt.Errorf("labeling node %s: %w", targetNode, err)
	}

	if err := m.remediator.DrainNode(ctx, targetNode, m.cfg.DrainGracePeriod); err != nil {
		return fmt.Errorf("draining node %s: %w", targetNode, err)
	}

	return nil
}

// TaintThermalThrottledNode applies the NoSchedule thermal throttle taint and evicts latency critical pods.
func (m *HardwareManager) TaintThermalThrottledNode(ctx context.Context, nodeName string) error {
	targetNode := nodeName
	if targetNode == "" {
		targetNode = m.cfg.NodeName
	}

	if m.remediator == nil {
		return nil
	}

	m.logger.Info("applying thermal throttled taint to node", zap.String("node", targetNode))

	if err := m.remediator.TaintNode(ctx, targetNode, domain.ThermalThrottledTaintKey, "true", domain.ThermalThrottledTaintEffect); err != nil {
		return fmt.Errorf("tainting node %s: %w", targetNode, err)
	}

	if _, err := m.remediator.EvictLatencyCriticalPods(ctx, targetNode); err != nil {
		return fmt.Errorf("evicting pods on node %s: %w", targetNode, err)
	}

	return nil
}
