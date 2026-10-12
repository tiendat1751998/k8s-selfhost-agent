package hardware

import (
	"context"
	"os"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/nodemetrics"
)

// WatchdogConfig configures the hardware watchdog dependencies.
type WatchdogConfig struct {
	NodeName    string
	NTPServer   string
	DiskDevices []string
	Clock       ClockReader
	SmartRunner SmartCommandRunner
	OOMShield   *OOMShield
}

// HardwareWatchdogImpl implements nodemetrics.HardwareWatchdog.
type HardwareWatchdogImpl struct {
	nodeName    string
	ntpServer   string
	diskDevices []string
	clock       ClockReader
	smartReader *SmartReader
	oomShield   *OOMShield
}

// NewHardwareWatchdog creates a new instance of HardwareWatchdogImpl.
func NewHardwareWatchdog(cfg WatchdogConfig) *HardwareWatchdogImpl {
	nodeName := cfg.NodeName
	if nodeName == "" {
		if envNode := os.Getenv("NODE_NAME"); envNode != "" {
			nodeName = envNode
		} else if h, err := os.Hostname(); err == nil {
			nodeName = h
		} else {
			nodeName = "unknown-node"
		}
	}

	clock := cfg.Clock
	if clock == nil {
		clock = NewClockReader()
	}

	smartReader := NewSmartReader(cfg.SmartRunner)

	oomShield := cfg.OOMShield
	if oomShield == nil {
		oomShield = NewOOMShield()
	}

	return &HardwareWatchdogImpl{
		nodeName:    nodeName,
		ntpServer:   cfg.NTPServer,
		diskDevices: cfg.DiskDevices,
		clock:       clock,
		smartReader: smartReader,
		oomShield:   oomShield,
	}
}

// GetClusterTimeDrift inspects host time offset using the ClockReader.
func (h *HardwareWatchdogImpl) GetClusterTimeDrift(ctx context.Context, clusterID string) ([]nodemetrics.TimeDriftStatus, error) {
	offset, isSync, err := h.clock.ReadTimeOffset()
	if err != nil {
		return nil, err
	}

	status := nodemetrics.TimeDriftStatusHealthy
	absOffset := offset
	if absOffset < 0 {
		absOffset = -absOffset
	}
	if absOffset > 500*time.Millisecond {
		status = nodemetrics.TimeDriftStatusCritical
	} else if absOffset > 100*time.Millisecond {
		status = nodemetrics.TimeDriftStatusWarning
	}

	return []nodemetrics.TimeDriftStatus{
		{
			NodeName:       h.nodeName,
			OffsetDuration: offset,
			IsSynchronized: isSync,
			NTPServer:      h.ntpServer,
			Status:         status,
			UpdatedAt:      time.Now().UTC(),
		},
	}, nil
}

// InspectNodeDisks checks the S.M.A.R.T diagnostic health of all configured disk block devices.
func (h *HardwareWatchdogImpl) InspectNodeDisks(ctx context.Context, nodeName string) ([]nodemetrics.DiskHealthReport, error) {
	devices := h.diskDevices
	if len(devices) == 0 {
		devices = []string{"/dev/sda"}
	}

	reports := make([]nodemetrics.DiskHealthReport, 0, len(devices))
	for _, dev := range devices {
		report, err := h.smartReader.ReadDiskHealth(ctx, dev)
		if err != nil {
			return nil, err
		}
		reports = append(reports, *report)
	}

	return reports, nil
}

// VerifyOOMProtection ensures critical daemons are shielded against kernel OOM kill (score -999).
func (h *HardwareWatchdogImpl) VerifyOOMProtection(ctx context.Context, nodeName string) (bool, error) {
	allProtected := true
	for _, daemon := range DefaultProtectedDaemons {
		score, isShielded, err := h.oomShield.VerifyDaemonOOMScore(daemon)
		if err != nil {
			// Daemon may not be installed; skip if absent
			continue
		}

		if !isShielded || score != ProtectedOOMScore {
			// Attempt to shield
			if err := h.oomShield.ShieldDaemon(daemon); err != nil {
				allProtected = false
			}
		}
	}

	return allProtected, nil
}
