package nodemetrics

import (
	"context"
	"errors"
	"time"
)

// Standard domain error constants for hardware anomalies.
var (
	ErrTimeDriftExceeded        = errors.New("time drift exceeded critical threshold")
	ErrSmartDegradationDetected = errors.New("smart degradation detected")
	ErrOOMShieldFailed          = errors.New("oom shield adjustment failed")
)

// TimeDrift status constants.
const (
	TimeDriftStatusHealthy  = "HEALTHY"
	TimeDriftStatusWarning  = "WARNING"
	TimeDriftStatusCritical = "CRITICAL"
)

// S.M.A.R.T disk health status constants.
const (
	DiskHealthPassed   = "PASSED"
	DiskHealthWarning  = "WARNING"
	DiskHealthDegraded = "DEGRADED"
	DiskHealthFailing  = "FAILING"
)

// TimeDriftStatus represents system clock offset and NTP synchronization state on a node.
type TimeDriftStatus struct {
	NodeName       string        `json:"node_name"`
	OffsetDuration time.Duration `json:"offset_duration"` // Offset relative to master/NTP source
	IsSynchronized bool          `json:"is_synchronized"`
	NTPServer      string        `json:"ntp_server"`
	Status         string        `json:"status"` // HEALTHY, WARNING, CRITICAL
	UpdatedAt      time.Time     `json:"updated_at"`
}

// DiskHealthReport represents S.M.A.R.T diagnostic indicators and predictive failure metrics for a disk.
type DiskHealthReport struct {
	DevicePath      string `json:"device_path"`       // /dev/nvme0n1, /dev/sda
	Model           string `json:"model"`
	Serial          string `json:"serial"`
	TemperatureC    int    `json:"temperature_c"`
	BadSectorsCount int64  `json:"bad_sectors_count"` // Reallocated sectors
	MediaErrors     int64  `json:"media_errors"`      // NVMe Media & Data Integrity Errors
	PowerOnHours    int64  `json:"power_on_hours"`
	HealthStatus    string `json:"health_status"`     // PASSED, FAILING, DEGRADED, WARNING
}

// HardwareWatchdog defines low-level hardware inspection operations for a cluster node.
type HardwareWatchdog interface {
	GetClusterTimeDrift(ctx context.Context, clusterID string) ([]TimeDriftStatus, error)
	InspectNodeDisks(ctx context.Context, nodeName string) ([]DiskHealthReport, error)
	VerifyOOMProtection(ctx context.Context, nodeName string) (bool, error)
}

// SREAlertEvent represents an operational alert event triggered by hardware watchdogs.
type SREAlertEvent struct {
	ID        string    `json:"id"`
	NodeName  string    `json:"node_name"`
	Severity  string    `json:"severity"` // WARNING, CRITICAL
	ErrorCode string    `json:"error_code"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

// SREAlertSink defines an event sink or notifier for SRE hardware alerts.
type SREAlertSink interface {
	EmitAlert(ctx context.Context, event SREAlertEvent) error
}
