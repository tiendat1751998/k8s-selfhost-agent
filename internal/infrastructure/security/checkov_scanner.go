package security

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	domain "github.com/datdt/k8sselfhost/internal/domain/security"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// GateStatus aliases domain.GateStatus for backward compatibility and clean API access.
type GateStatus = domain.GateStatus

const (
	GateStatusPassed             = domain.GateStatusPassed
	GateStatusFailed             = domain.GateStatusFailed
	GateStatusScannerUnavailable = domain.GateStatusScannerUnavailable
	GateStatusPolicyViolation    = domain.GateStatusPolicyViolation
)

// IaCCheckResult holds the outcome of an individual IaC policy check.
type IaCCheckResult struct {
	CheckID   string `json:"check_id"`
	CheckName string `json:"check_name"`
	Resource  string `json:"resource"`
	Filepath  string `json:"file_path"`
	Severity  string `json:"severity"` // HIGH, MEDIUM, LOW
	Status    string `json:"status"`   // PASSED, FAILED
	Guideline string `json:"guideline"`
}

// IaCScanReport summarizes an IaC scan across a directory or manifest file.
type IaCScanReport struct {
	Target           string           `json:"target,omitempty"`
	Framework        string           `json:"framework"` // terraform, kubernetes, helm
	PassedChecks     int              `json:"passed_checks"`
	FailedChecks     int              `json:"failed_checks"`
	ScorePercentage  float64          `json:"score_percentage"`
	Checks           []IaCCheckResult `json:"checks"`
	PassSecurityGate bool             `json:"pass_security_gate"`
	Status           GateStatus       `json:"status"`
	Passed           bool             `json:"passed"`
	ErrorMessage     string           `json:"error_message,omitempty"`
	ScannedAt        time.Time        `json:"scanned_at"`
}

// ToDomainReport converts IaCScanReport to domain.SecurityGateReport.
func (r *IaCScanReport) ToDomainReport(scanID string) *domain.SecurityGateReport {
	if r == nil {
		return nil
	}

	report := &domain.SecurityGateReport{
		ScanID:       scanID,
		Target:       r.Target,
		Status:       r.Status,
		Passed:       r.Passed && r.PassSecurityGate,
		ErrorMessage: r.ErrorMessage,
		ScannedAt:    r.ScannedAt,
	}

	for _, c := range r.Checks {
		if c.Status == "FAILED" {
			report.FailedChecks = append(report.FailedChecks, domain.IaCFailedCheck{
				CheckID:   c.CheckID,
				CheckName: c.CheckName,
				Resource:  c.Resource,
				FilePath:  c.Filepath,
				Severity:  c.Severity,
				Guideline: c.Guideline,
			})
			switch c.Severity {
			case "CRITICAL":
				report.CriticalCount++
			case "HIGH":
				report.HighCount++
			case "MEDIUM":
				report.MediumCount++
			case "LOW":
				report.LowCount++
			}
		}
	}

	return report
}

// CheckovScanner runs Checkov against infrastructure manifests.
type CheckovScanner struct {
	binaryPath string
}

// NewCheckovScanner creates a CheckovScanner reading CHECKOV_BINARY from env or defaulting to "checkov".
func NewCheckovScanner() *CheckovScanner {
	bin := os.Getenv("CHECKOV_BINARY")
	if bin == "" {
		bin = "checkov"
	}
	return &CheckovScanner{binaryPath: bin}
}

// NewCheckovScannerWithBinary creates a CheckovScanner with a custom binary path.
func NewCheckovScannerWithBinary(binary string) *CheckovScanner {
	if binary == "" {
		binary = "checkov"
	}
	return &CheckovScanner{binaryPath: binary}
}

// ScanDirectory scans a directory containing IaC code.
// Under the Fail-Close invariant, if the scanner binary fails or is unavailable,
// it returns an explicit error and a report with Passed = false and Status = GateStatusScannerUnavailable.
func (c *CheckovScanner) ScanDirectory(ctx context.Context, dirPath string, framework string) (*IaCScanReport, error) {
	start := time.Now().UTC()

	if _, err := exec.LookPath(c.binaryPath); err != nil {
		return &IaCScanReport{
			Target:           dirPath,
			Framework:        framework,
			ScannedAt:        start,
			PassedChecks:     0,
			FailedChecks:     0,
			ScorePercentage:  0.0,
			Checks:           []IaCCheckResult{},
			PassSecurityGate: false,
			Status:           GateStatusScannerUnavailable,
			Passed:           false,
			ErrorMessage:     fmt.Sprintf("checkov binary not found: %v", err),
		}, fmt.Errorf("security scanner unavailable: checkov binary not found: %w", err)
	}

	args := []string{"-d", dirPath, "--output", "json", "--quiet"}
	if framework != "" {
		args = append(args, "--framework", framework)
	}

	cmd := exec.CommandContext(ctx, c.binaryPath, args...)
	output, err := cmd.Output()
	if err != nil {
		if len(output) > 0 {
			if report, parseErr := c.ParseCheckovJSON(output, framework, start); parseErr == nil && report != nil {
				report.Target = dirPath
				return report, nil
			}
		}
		return &IaCScanReport{
			Target:           dirPath,
			Framework:        framework,
			ScannedAt:        start,
			PassedChecks:     0,
			FailedChecks:     0,
			ScorePercentage:  0.0,
			Checks:           []IaCCheckResult{},
			PassSecurityGate: false,
			Status:           GateStatusScannerUnavailable,
			Passed:           false,
			ErrorMessage:     fmt.Sprintf("checkov execution failed: %v", err),
		}, fmt.Errorf("security scanner unavailable: checkov execution failed: %w", err)
	}

	report, err := c.ParseCheckovJSON(output, framework, start)
	if report != nil {
		report.Target = dirPath
	}
	return report, err
}

// ScanManifest scans a single Kubernetes/Terraform manifest file.
// Under the Fail-Close invariant, if execution fails, it returns an explicit error with Status = GateStatusScannerUnavailable.
func (c *CheckovScanner) ScanManifest(ctx context.Context, manifestPath string) (*IaCScanReport, error) {
	start := time.Now().UTC()

	if _, err := exec.LookPath(c.binaryPath); err != nil {
		return &IaCScanReport{
			Target:           manifestPath,
			Framework:        "kubernetes",
			ScannedAt:        start,
			PassedChecks:     0,
			FailedChecks:     0,
			ScorePercentage:  0.0,
			Checks:           []IaCCheckResult{},
			PassSecurityGate: false,
			Status:           GateStatusScannerUnavailable,
			Passed:           false,
			ErrorMessage:     fmt.Sprintf("checkov binary not found: %v", err),
		}, fmt.Errorf("security scanner unavailable: checkov binary not found: %w", err)
	}

	cmd := exec.CommandContext(ctx, c.binaryPath, "-f", manifestPath, "--output", "json", "--quiet")
	output, err := cmd.Output()
	if err != nil {
		if len(output) > 0 {
			if report, parseErr := c.ParseCheckovJSON(output, "kubernetes", start); parseErr == nil && report != nil {
				report.Target = manifestPath
				return report, nil
			}
		}
		return &IaCScanReport{
			Target:           manifestPath,
			Framework:        "kubernetes",
			ScannedAt:        start,
			PassedChecks:     0,
			FailedChecks:     0,
			ScorePercentage:  0.0,
			Checks:           []IaCCheckResult{},
			PassSecurityGate: false,
			Status:           GateStatusScannerUnavailable,
			Passed:           false,
			ErrorMessage:     fmt.Sprintf("checkov execution failed: %v", err),
		}, fmt.Errorf("security scanner unavailable: checkov execution failed: %w", err)
	}

	report, err := c.ParseCheckovJSON(output, "kubernetes", start)
	if report != nil {
		report.Target = manifestPath
	}
	return report, err
}

// ParseCheckovJSON parses Checkov's JSON output into an IaCScanReport.
func (c *CheckovScanner) ParseCheckovJSON(data []byte, framework string, scannedAt time.Time) (*IaCScanReport, error) {
	var raw struct {
		Summary struct {
			Passed int `json:"passed"`
			Failed int `json:"failed"`
		} `json:"summary"`
		Results struct {
			PassedChecks []struct {
				CheckID   string `json:"check_id"`
				CheckName string `json:"check_name"`
				Resource  string `json:"resource"`
				FilePath  string `json:"file_path"`
				Guideline string `json:"guideline"`
			} `json:"passed_checks"`
			FailedChecks []struct {
				CheckID   string `json:"check_id"`
				CheckName string `json:"check_name"`
				Resource  string `json:"resource"`
				FilePath  string `json:"file_path"`
				Severity  string `json:"severity"`
				Guideline string `json:"guideline"`
			} `json:"failed_checks"`
		} `json:"results"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return &IaCScanReport{
			Framework:        framework,
			PassSecurityGate: false,
			Status:           GateStatusPolicyViolation,
			Passed:           false,
			ScorePercentage:  0.0,
			ScannedAt:        scannedAt,
			ErrorMessage:     fmt.Sprintf("parsing checkov json: %v", err),
		}, errors.Wrap(err, "parsing checkov json")
	}

	report := &IaCScanReport{
		Framework:    framework,
		PassedChecks: raw.Summary.Passed,
		FailedChecks: raw.Summary.Failed,
		ScannedAt:    scannedAt,
	}

	for _, p := range raw.Results.PassedChecks {
		report.Checks = append(report.Checks, IaCCheckResult{
			CheckID:   p.CheckID,
			CheckName: p.CheckName,
			Resource:  p.Resource,
			Filepath:  p.FilePath,
			Status:    "PASSED",
			Guideline: p.Guideline,
		})
	}

	for _, f := range raw.Results.FailedChecks {
		sev := f.Severity
		if sev == "" {
			sev = "HIGH"
		}
		report.Checks = append(report.Checks, IaCCheckResult{
			CheckID:   f.CheckID,
			CheckName: f.CheckName,
			Resource:  f.Resource,
			Filepath:  f.FilePath,
			Severity:  sev,
			Status:    "FAILED",
			Guideline: f.Guideline,
		})
	}

	total := report.PassedChecks + report.FailedChecks
	if total > 0 {
		report.ScorePercentage = float64(report.PassedChecks) / float64(total) * 100
	} else {
		report.ScorePercentage = 0.0
	}

	if report.FailedChecks > 0 || total == 0 {
		report.Status = GateStatusPolicyViolation
		report.Passed = false
		report.PassSecurityGate = false
	} else {
		report.Status = GateStatusPassed
		report.Passed = true
		report.PassSecurityGate = true
	}

	return report, nil
}
