package security

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	domain "github.com/datdt/k8sselfhost/internal/domain/security"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// Vulnerability represents an individual CVE result from Trivy.
type Vulnerability struct {
	VulnerabilityID  string `json:"VulnerabilityID"`
	PkgName          string `json:"PkgName"`
	InstalledVersion string `json:"InstalledVersion"`
	FixedVersion     string `json:"FixedVersion"`
	Severity         string `json:"Severity"` // CRITICAL, HIGH, MEDIUM, LOW
	Title            string `json:"Title"`
	Description      string `json:"Description"`
	PrimaryURL       string `json:"PrimaryURL"`
}

// ScanSummary aggregates CVE counts by severity.
type ScanSummary struct {
	CriticalCount int `json:"critical_count"`
	HighCount     int `json:"high_count"`
	MediumCount   int `json:"medium_count"`
	LowCount      int `json:"low_count"`
	TotalCVEs     int `json:"total_cves"`
}

// ScanReport represents the result of a Trivy container image scan.
type ScanReport struct {
	Target           string          `json:"target"`
	ScannedAt        time.Time       `json:"scanned_at"`
	Summary          ScanSummary     `json:"summary"`
	Vulnerabilities  []Vulnerability `json:"vulnerabilities"`
	PassSecurityGate bool            `json:"pass_security_gate"`
	Status           GateStatus      `json:"status"`
	Passed           bool            `json:"passed"`
	ErrorMessage     string          `json:"error_message,omitempty"`
}

// ToDomainReport converts a ScanReport to domain.SecurityGateReport.
func (r *ScanReport) ToDomainReport(scanID string) *domain.SecurityGateReport {
	if r == nil {
		return nil
	}

	report := &domain.SecurityGateReport{
		ScanID:        scanID,
		Target:        r.Target,
		Status:        r.Status,
		Passed:        r.Passed && r.PassSecurityGate,
		CriticalCount: r.Summary.CriticalCount,
		HighCount:     r.Summary.HighCount,
		MediumCount:   r.Summary.MediumCount,
		LowCount:      r.Summary.LowCount,
		ErrorMessage:  r.ErrorMessage,
		ScannedAt:     r.ScannedAt,
	}

	for _, v := range r.Vulnerabilities {
		report.Vulnerabilities = append(report.Vulnerabilities, domain.Vulnerability{
			VulnerabilityID:  v.VulnerabilityID,
			PkgName:          v.PkgName,
			InstalledVersion: v.InstalledVersion,
			FixedVersion:     v.FixedVersion,
			Severity:         v.Severity,
			Title:            v.Title,
			Description:      v.Description,
			PrimaryURL:       v.PrimaryURL,
		})
	}

	return report
}

// TrivyScanner runs Trivy against container images.
type TrivyScanner struct {
	binaryPath string
}

// NewTrivyScanner creates a TrivyScanner reading TRIVY_BINARY or defaulting to "trivy".
func NewTrivyScanner() *TrivyScanner {
	bin := os.Getenv("TRIVY_BINARY")
	if bin == "" {
		bin = "trivy"
	}
	return &TrivyScanner{binaryPath: bin}
}

// NewTrivyScannerWithBinary creates a TrivyScanner with an explicit binary path.
func NewTrivyScannerWithBinary(binary string) *TrivyScanner {
	if binary == "" {
		binary = "trivy"
	}
	return &TrivyScanner{binaryPath: binary}
}

// ScanImage executes Trivy container scanning.
// Under the Fail-Close invariant, if Trivy execution fails or the binary is not found,
// it returns an explicit error and a report with Passed = false, PassSecurityGate = false,
// and Status = GateStatusScannerUnavailable.
func (s *TrivyScanner) ScanImage(ctx context.Context, image string, maxSeverityAllowed string) (*ScanReport, error) {
	start := time.Now().UTC()

	trimmed := strings.TrimSpace(image)
	if trimmed == "" || strings.HasPrefix(trimmed, "-") {
		return &ScanReport{
			Target:           image,
			ScannedAt:        start,
			PassSecurityGate: false,
			Passed:           false,
			Status:           GateStatusScannerUnavailable,
			ErrorMessage:     fmt.Sprintf("invalid container image %q: cannot be empty or start with '-'", image),
			Summary: ScanSummary{
				TotalCVEs:     0,
				CriticalCount: 0,
				HighCount:     0,
				MediumCount:   0,
				LowCount:      0,
			},
			Vulnerabilities: []Vulnerability{},
		}, fmt.Errorf("invalid container image %q: cannot be empty or start with '-'", image)
	}

	if _, lookErr := exec.LookPath(s.binaryPath); lookErr != nil {
		return &ScanReport{
			Target:           trimmed,
			ScannedAt:        start,
			PassSecurityGate: false,
			Passed:           false,
			Status:           GateStatusScannerUnavailable,
			ErrorMessage:     fmt.Sprintf("trivy binary not found: %v", lookErr),
			Summary: ScanSummary{
				TotalCVEs:     0,
				CriticalCount: 0,
				HighCount:     0,
				MediumCount:   0,
				LowCount:      0,
			},
			Vulnerabilities: []Vulnerability{},
		}, fmt.Errorf("security scanner unavailable: trivy binary not found: %w", lookErr)
	}

	cmd := exec.CommandContext(ctx, s.binaryPath, "image", "--format", "json", "--quiet", "--", trimmed)
	output, err := cmd.Output()
	if err != nil {
		return &ScanReport{
			Target:           trimmed,
			ScannedAt:        start,
			PassSecurityGate: false,
			Passed:           false,
			Status:           GateStatusScannerUnavailable,
			ErrorMessage:     fmt.Sprintf("trivy execution failed: %v", err),
			Summary: ScanSummary{
				TotalCVEs:     0,
				CriticalCount: 0,
				HighCount:     0,
				MediumCount:   0,
				LowCount:      0,
			},
			Vulnerabilities: []Vulnerability{},
		}, fmt.Errorf("security scanner unavailable: trivy execution failed: %w", err)
	}

	return s.ParseTrivyJSON(output, trimmed, maxSeverityAllowed, start)
}

// ParseTrivyJSON unmarshals Trivy output and evaluates gate thresholds.
func (s *TrivyScanner) ParseTrivyJSON(data []byte, target, maxSeverity string, scannedAt time.Time) (*ScanReport, error) {
	var trivyOut struct {
		Results []struct {
			Target          string          `json:"Target"`
			Vulnerabilities []Vulnerability `json:"Vulnerabilities"`
		} `json:"Results"`
	}

	if err := json.Unmarshal(data, &trivyOut); err != nil {
		return &ScanReport{
			Target:           target,
			ScannedAt:        scannedAt,
			PassSecurityGate: false,
			Passed:           false,
			Status:           GateStatusScannerUnavailable,
			ErrorMessage:     fmt.Sprintf("parsing trivy JSON report: %v", err),
			Summary:          ScanSummary{},
			Vulnerabilities:  []Vulnerability{},
		}, errors.Wrap(err, "parsing trivy JSON report")
	}

	report := &ScanReport{
		Target:    target,
		ScannedAt: scannedAt,
	}

	for _, res := range trivyOut.Results {
		for _, v := range res.Vulnerabilities {
			report.Vulnerabilities = append(report.Vulnerabilities, v)
			switch strings.ToUpper(v.Severity) {
			case "CRITICAL":
				report.Summary.CriticalCount++
			case "HIGH":
				report.Summary.HighCount++
			case "MEDIUM":
				report.Summary.MediumCount++
			case "LOW":
				report.Summary.LowCount++
			}
		}
	}
	report.Summary.TotalCVEs = len(report.Vulnerabilities)

	// Gate rule: If maxAllowed is HIGH, block on CRITICAL. If MEDIUM, block on CRITICAL & HIGH.
	// Critical CVE > 0 ALWAYS fails the security gate under strict zero-trust.
	if report.Summary.CriticalCount > 0 {
		report.PassSecurityGate = false
		report.Passed = false
		report.Status = GateStatusPolicyViolation
	} else if strings.ToUpper(maxSeverity) == "MEDIUM" && report.Summary.HighCount > 0 {
		report.PassSecurityGate = false
		report.Passed = false
		report.Status = GateStatusPolicyViolation
	} else {
		report.PassSecurityGate = true
		report.Passed = true
		report.Status = GateStatusPassed
	}

	return report, nil
}

// FormatSARIFSummary produces a human-readable summary of the scan.
func (s *TrivyScanner) FormatSARIFSummary(report *ScanReport) string {
	if report == nil {
		return "Report is nil"
	}
	status := "PASSED ✅"
	if !report.PassSecurityGate || !report.Passed {
		status = "FAILED ❌"
	}
	return fmt.Sprintf("Image: %s | Security Gate: %s | Critical: %d | High: %d | Medium: %d | Low: %d",
		report.Target, status, report.Summary.CriticalCount, report.Summary.HighCount, report.Summary.MediumCount, report.Summary.LowCount)
}
