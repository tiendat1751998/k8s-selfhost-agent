package nodemetrics

import (
	"context"
	"fmt"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/nodemetrics"
)

// Standard threshold durations for cluster time drift.
const (
	TimeDriftWarningThreshold  = 100 * time.Millisecond
	TimeDriftCriticalThreshold = 500 * time.Millisecond
)

// RFC 7807 and SRE Error code constants.
const (
	ErrCodeTimeDriftExceeded     = "ERR_TIME_DRIFT_EXCEEDED"
	ErrCodeTimeDriftWarning      = "ERR_TIME_DRIFT_WARNING"
	ErrCodeSmartDegradation      = "ERR_SMART_DEGRADATION_DETECTED"
	ErrCodeOOMShieldUnprotected  = "ERR_OOM_SHIELD_UNPROTECTED"
)

// ResyncRunner defines the interface to trigger time resynchronization on a node.
type ResyncRunner interface {
	ResyncNodeTime(ctx context.Context, nodeName string) error
}

// HardwareUsecase manages hardware diagnostics, time drift watchdog thresholds, and SRE alerts.
type HardwareUsecase struct {
	watchdog     nodemetrics.HardwareWatchdog
	alertSink    nodemetrics.SREAlertSink
	resyncRunner ResyncRunner
}

// NewHardwareUsecase constructs a new HardwareUsecase instance.
func NewHardwareUsecase(
	watchdog nodemetrics.HardwareWatchdog,
	alertSink nodemetrics.SREAlertSink,
	resyncRunner ResyncRunner,
) *HardwareUsecase {
	return &HardwareUsecase{
		watchdog:     watchdog,
		alertSink:    alertSink,
		resyncRunner: resyncRunner,
	}
}

// EvaluateTimeDrift inspects cluster clock offsets and triggers alerts and resyncs on threshold violations.
func (u *HardwareUsecase) EvaluateTimeDrift(ctx context.Context, clusterID string) ([]nodemetrics.TimeDriftStatus, error) {
	drifts, err := u.watchdog.GetClusterTimeDrift(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("evaluating cluster time drift: %w", err)
	}

	var criticalNodes []string
	for i := range drifts {
		absOffset := drifts[i].OffsetDuration
		if absOffset < 0 {
			absOffset = -absOffset
		}

		if absOffset > TimeDriftCriticalThreshold {
			drifts[i].Status = nodemetrics.TimeDriftStatusCritical
			criticalNodes = append(criticalNodes, drifts[i].NodeName)

			if u.alertSink != nil {
				event := nodemetrics.SREAlertEvent{
					ID:        fmt.Sprintf("drift-crit-%s-%d", drifts[i].NodeName, time.Now().UnixNano()),
					NodeName:  drifts[i].NodeName,
					Severity:  "CRITICAL",
					ErrorCode: ErrCodeTimeDriftExceeded,
					Message: fmt.Sprintf(
						"Node '%s' time drift exceeded critical threshold (%s > 500ms). Resync required.",
						drifts[i].NodeName,
						drifts[i].OffsetDuration,
					),
					CreatedAt: time.Now().UTC(),
				}
				_ = u.alertSink.EmitAlert(ctx, event)
			}

			if u.resyncRunner != nil {
				_ = u.resyncRunner.ResyncNodeTime(ctx, drifts[i].NodeName)
			}
		} else if absOffset > TimeDriftWarningThreshold {
			drifts[i].Status = nodemetrics.TimeDriftStatusWarning

			if u.alertSink != nil {
				event := nodemetrics.SREAlertEvent{
					ID:        fmt.Sprintf("drift-warn-%s-%d", drifts[i].NodeName, time.Now().UnixNano()),
					NodeName:  drifts[i].NodeName,
					Severity:  "WARNING",
					ErrorCode: ErrCodeTimeDriftWarning,
					Message: fmt.Sprintf(
						"Node '%s' time drift elevated (%s > 100ms)",
						drifts[i].NodeName,
						drifts[i].OffsetDuration,
					),
					CreatedAt: time.Now().UTC(),
				}
				_ = u.alertSink.EmitAlert(ctx, event)
			}
		} else {
			drifts[i].Status = nodemetrics.TimeDriftStatusHealthy
		}
	}

	if len(criticalNodes) > 0 {
		return drifts, fmt.Errorf("%w: critical drift on nodes %v", nodemetrics.ErrTimeDriftExceeded, criticalNodes)
	}

	return drifts, nil
}

// EvaluateDiskHealth inspects S.M.A.R.T indicators on a node and emits SRE alerts for degraded or failing disks.
func (u *HardwareUsecase) EvaluateDiskHealth(ctx context.Context, nodeName string) ([]nodemetrics.DiskHealthReport, error) {
	reports, err := u.watchdog.InspectNodeDisks(ctx, nodeName)
	if err != nil {
		return nil, fmt.Errorf("inspecting disks on node %s: %w", nodeName, err)
	}

	var degradedDisks []string
	for _, disk := range reports {
		if disk.HealthStatus == nodemetrics.DiskHealthDegraded || disk.HealthStatus == nodemetrics.DiskHealthFailing {
			degradedDisks = append(degradedDisks, disk.DevicePath)

			if u.alertSink != nil {
				event := nodemetrics.SREAlertEvent{
					ID:        fmt.Sprintf("disk-deg-%s-%d", nodeName, time.Now().UnixNano()),
					NodeName:  nodeName,
					Severity:  "CRITICAL",
					ErrorCode: ErrCodeSmartDegradation,
					Message: fmt.Sprintf(
						"Disk '%s' on node '%s' is degraded (Bad sectors: %d, Media errors: %d).",
						disk.DevicePath,
						nodeName,
						disk.BadSectorsCount,
						disk.MediaErrors,
					),
					CreatedAt: time.Now().UTC(),
				}
				_ = u.alertSink.EmitAlert(ctx, event)
			}
		}
	}

	if len(degradedDisks) > 0 {
		return reports, fmt.Errorf("%w: degraded disks %v on node %s", nodemetrics.ErrSmartDegradationDetected, degradedDisks, nodeName)
	}

	return reports, nil
}

// VerifyOOMProtection validates whether critical daemons (kubelet, containerd) have kernel OOM immunity score (-999).
func (u *HardwareUsecase) VerifyOOMProtection(ctx context.Context, nodeName string) (bool, error) {
	protected, err := u.watchdog.VerifyOOMProtection(ctx, nodeName)
	if err != nil {
		return false, fmt.Errorf("verifying OOM protection on node %s: %w", nodeName, err)
	}

	if !protected {
		if u.alertSink != nil {
			event := nodemetrics.SREAlertEvent{
				ID:        fmt.Sprintf("oom-warn-%s-%d", nodeName, time.Now().UnixNano()),
				NodeName:  nodeName,
				Severity:  "WARNING",
				ErrorCode: ErrCodeOOMShieldUnprotected,
				Message: fmt.Sprintf(
					"Node '%s' has unprotected daemons with OOM score != -999",
					nodeName,
				),
				CreatedAt: time.Now().UTC(),
			}
			_ = u.alertSink.EmitAlert(ctx, event)
		}
		return false, nodemetrics.ErrOOMShieldFailed
	}

	return true, nil
}
