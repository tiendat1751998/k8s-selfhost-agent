package hardware

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/datdt/k8sselfhost/internal/domain/nodemetrics"
)

// smartctlOutput defines the JSON schema emitted by `smartctl -j`.
type smartctlOutput struct {
	Device struct {
		Name     string `json:"name"`
		InfoName string `json:"info_name"`
		Type     string `json:"type"`
		Protocol string `json:"protocol"`
	} `json:"device"`
	ModelName    string `json:"model_name"`
	SerialNumber string `json:"serial_number"`
	SmartStatus  struct {
		Passed bool `json:"passed"`
	} `json:"smart_status"`
	Temperature struct {
		Current int `json:"current"`
	} `json:"temperature"`
	PowerOnTime struct {
		Hours int64 `json:"hours"`
	} `json:"power_on_time"`
	ATASmartAttributes struct {
		Table []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
			Raw  struct {
				Value int64 `json:"value"`
			} `json:"raw"`
		} `json:"table"`
	} `json:"ata_smart_attributes"`
	NVMeSmartHealthInformationLog struct {
		CriticalWarning int   `json:"critical_warning"`
		Temperature     int   `json:"temperature"`
		MediaErrors     int64 `json:"media_errors"`
		PowerOnHours    int64 `json:"power_on_hours"`
	} `json:"nvme_smart_health_information_log"`
}

// ParseSmartCTLJSON parses raw JSON output from `smartctl -H -A -j` into a DiskHealthReport.
func ParseSmartCTLJSON(data []byte) (*nodemetrics.DiskHealthReport, error) {
	var out smartctlOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("unmarshaling smartctl json: %w", err)
	}

	report := &nodemetrics.DiskHealthReport{
		DevicePath:   out.Device.Name,
		Model:        out.ModelName,
		Serial:       out.SerialNumber,
		TemperatureC: out.Temperature.Current,
		PowerOnHours: out.PowerOnTime.Hours,
	}

	// Temperature fallback for NVMe
	if report.TemperatureC == 0 && out.NVMeSmartHealthInformationLog.Temperature > 0 {
		report.TemperatureC = out.NVMeSmartHealthInformationLog.Temperature
	}
	// PowerOnHours fallback for NVMe
	if report.PowerOnHours == 0 && out.NVMeSmartHealthInformationLog.PowerOnHours > 0 {
		report.PowerOnHours = out.NVMeSmartHealthInformationLog.PowerOnHours
	}

	report.MediaErrors = out.NVMeSmartHealthInformationLog.MediaErrors

	var reallocatedSectors int64
	var pendingSectors int64
	for _, attr := range out.ATASmartAttributes.Table {
		switch attr.ID {
		case 5: // Reallocated_Sector_Ct
			reallocatedSectors = attr.Raw.Value
		case 197: // Current_Pending_Sector
			pendingSectors = attr.Raw.Value
		}
	}
	report.BadSectorsCount = reallocatedSectors

	// Evaluate HealthStatus
	if !out.SmartStatus.Passed {
		report.HealthStatus = nodemetrics.DiskHealthFailing
	} else if report.MediaErrors > 0 || reallocatedSectors > 10 || pendingSectors > 0 {
		report.HealthStatus = nodemetrics.DiskHealthDegraded
	} else if reallocatedSectors > 0 {
		report.HealthStatus = nodemetrics.DiskHealthWarning
	} else {
		report.HealthStatus = nodemetrics.DiskHealthPassed
	}

	return report, nil
}

// SmartCommandRunner defines the executor interface for running smartctl.
type SmartCommandRunner interface {
	RunSmartCTL(ctx context.Context, devicePath string) ([]byte, error)
}

// OSSmartCommandRunner implements SmartCommandRunner using local CLI invocation.
type OSSmartCommandRunner struct{}

// RunSmartCTL executes smartctl with read-only inspection flags (-H -A -j).
func (r *OSSmartCommandRunner) RunSmartCTL(ctx context.Context, devicePath string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "smartctl", "-H", "-A", "-j", devicePath)
	out, err := cmd.Output()
	if err != nil {
		// smartctl returns exit status bitmasks; if output contains valid JSON, return it
		if len(out) > 0 && json.Valid(out) {
			return out, nil
		}
		return nil, fmt.Errorf("running smartctl on %s: %w", devicePath, err)
	}
	return out, nil
}

// SmartReader encapsulates running smartctl and converting metrics into domain reports.
type SmartReader struct {
	runner SmartCommandRunner
}

// NewSmartReader constructs a new SmartReader instance.
func NewSmartReader(runner SmartCommandRunner) *SmartReader {
	if runner == nil {
		runner = &OSSmartCommandRunner{}
	}
	return &SmartReader{runner: runner}
}

// ReadDiskHealth inspects a single disk device and returns its health diagnostic report.
func (s *SmartReader) ReadDiskHealth(ctx context.Context, devicePath string) (*nodemetrics.DiskHealthReport, error) {
	data, err := s.runner.RunSmartCTL(ctx, devicePath)
	if err != nil {
		return nil, err
	}
	report, err := ParseSmartCTLJSON(data)
	if err != nil {
		return nil, fmt.Errorf("evaluating smartctl output for %s: %w", devicePath, err)
	}
	if report.DevicePath == "" {
		report.DevicePath = devicePath
	}
	return report, nil
}
