// Package security provides domain entities, policy rules, and interfaces
// for DevSecOps supply chain security gates under the Fail-Close invariant.
package security

import (
	"context"
	"errors"
	"time"
)

// Sentinel domain errors for security gates.
var (
	ErrScannerUnavailable   = errors.New("security scanner unavailable")
	ErrSecurityGateRejected = errors.New("deployment rejected by security gate policy")
	ErrSignatureInvalid     = errors.New("artifact cryptographic signature verification failed")
)

// GateStatus represents the verdict of a security gate evaluation.
type GateStatus string

const (
	GateStatusPassed             GateStatus = "PASSED"
	GateStatusFailed             GateStatus = "FAILED"
	GateStatusScannerUnavailable GateStatus = "SCANNER_UNAVAILABLE"
	GateStatusPolicyViolation    GateStatus = "POLICY_VIOLATION"
)

// GateMode defines whether gate failures strictly reject or only warn.
type GateMode string

const (
	GateModeStrict     GateMode = "strict"
	GateModePermissive GateMode = "permissive"
)

// VulnerabilitySeverity represents standard CVSS severity ratings.
type VulnerabilitySeverity string

const (
	SeverityCritical VulnerabilitySeverity = "CRITICAL"
	SeverityHigh     VulnerabilitySeverity = "HIGH"
	SeverityMedium   VulnerabilitySeverity = "MEDIUM"
	SeverityLow      VulnerabilitySeverity = "LOW"
)

// Vulnerability represents a detected CVE or package vulnerability.
type Vulnerability struct {
	VulnerabilityID  string `json:"vulnerability_id"`
	PkgName          string `json:"pkg_name"`
	InstalledVersion string `json:"installed_version"`
	FixedVersion     string `json:"fixed_version"`
	Severity         string `json:"severity"` // CRITICAL, HIGH, MEDIUM, LOW
	Title            string `json:"title"`
	Description      string `json:"description"`
	PrimaryURL       string `json:"primary_url"`
}

// IaCFailedCheck represents an infrastructure-as-code misconfiguration.
type IaCFailedCheck struct {
	CheckID   string `json:"check_id"`
	CheckName string `json:"check_name"`
	Resource  string `json:"resource"`
	FilePath  string `json:"file_path"`
	Severity  string `json:"severity"`
	Guideline string `json:"guideline"`
}

// SecurityGateReport captures the comprehensive evaluation of an artifact.
type SecurityGateReport struct {
	ScanID          string           `json:"scan_id"`
	Target          string           `json:"target"` // Container image tag or Manifest path
	Status          GateStatus       `json:"status"`
	Passed          bool             `json:"passed"`
	CriticalCount   int              `json:"critical_count"`
	HighCount       int              `json:"high_count"`
	MediumCount     int              `json:"medium_count"`
	LowCount        int              `json:"low_count"`
	Vulnerabilities []Vulnerability  `json:"vulnerabilities,omitempty"`
	FailedChecks    []IaCFailedCheck `json:"failed_checks,omitempty"`
	CosignVerified  bool             `json:"cosign_verified"`
	ErrorMessage    string           `json:"error_message,omitempty"`
	ScannedAt       time.Time        `json:"scanned_at"`
}

// PolicyRules configures the thresholds and enforcement policies for gate evaluation.
type PolicyRules struct {
	MaxCriticalCVEAllowed int      `json:"max_critical_cve_allowed"`
	MaxHighCVEAllowed     int      `json:"max_high_cve_allowed"`
	RequireCosign         bool     `json:"require_cosign"`
	Mode                  GateMode `json:"mode"`
}

// DefaultStrictPolicy returns production-grade zero-trust policy rules.
func DefaultStrictPolicy() PolicyRules {
	return PolicyRules{
		MaxCriticalCVEAllowed: 0,
		MaxHighCVEAllowed:     0,
		RequireCosign:         false,
		Mode:                  GateModeStrict,
	}
}

// Evaluate applies the Fail-Close Invariant against this report:
// - Scanner error / missing -> REJECT (Status = GateStatusScannerUnavailable, Passed = false)
// - Critical CVE > MaxCriticalCVEAllowed -> REJECT (Status = GateStatusPolicyViolation, Passed = false)
// - High CVE > MaxHighCVEAllowed -> REJECT (Status = GateStatusPolicyViolation, Passed = false)
// - Cosign required but invalid/unsigned -> REJECT (Status = GateStatusPolicyViolation, Passed = false)
// - IaC failed checks -> REJECT (Status = GateStatusPolicyViolation, Passed = false)
// Permissive mode allows passing for local development environments, preserving error details.
func (r *SecurityGateReport) Evaluate(policy PolicyRules) bool {
	if r == nil {
		return false
	}

	if policy.Mode == GateModePermissive {
		r.Passed = true
		if r.Status == "" {
			r.Status = GateStatusPassed
		}
		return true
	}

	// Fail-Close: Scanner Unavailable / Process Error
	if r.Status == GateStatusScannerUnavailable || r.ErrorMessage != "" {
		r.Status = GateStatusScannerUnavailable
		r.Passed = false
		return false
	}

	// Fail-Close: Critical CVE violation
	if r.CriticalCount > policy.MaxCriticalCVEAllowed {
		r.Status = GateStatusPolicyViolation
		r.Passed = false
		return false
	}

	// Fail-Close: High CVE violation if configured
	if policy.MaxHighCVEAllowed >= 0 && r.HighCount > policy.MaxHighCVEAllowed {
		r.Status = GateStatusPolicyViolation
		r.Passed = false
		return false
	}

	// Fail-Close: Cosign signature enforcement
	if policy.RequireCosign && !r.CosignVerified {
		r.Status = GateStatusPolicyViolation
		r.Passed = false
		return false
	}

	// Fail-Close: IaC checks failed
	if len(r.FailedChecks) > 0 {
		r.Status = GateStatusPolicyViolation
		r.Passed = false
		return false
	}

	if r.Status == GateStatusFailed {
		r.Passed = false
		return false
	}

	// Passed all checks
	r.Status = GateStatusPassed
	r.Passed = true
	return true
}

// EvaluateGate is a package-level helper that evaluates a report against policy.
func EvaluateGate(report *SecurityGateReport, policy PolicyRules) bool {
	if report == nil {
		return false
	}
	return report.Evaluate(policy)
}

// ContainerScanner port for scanning container images.
type ContainerScanner interface {
	ScanImage(ctx context.Context, image string, maxSeverityAllowed string) (*SecurityGateReport, error)
}

// IaCScanner port for scanning Kubernetes / Terraform manifests.
type IaCScanner interface {
	ScanDirectory(ctx context.Context, dirPath string, framework string) (*SecurityGateReport, error)
	ScanManifest(ctx context.Context, manifestPath string) (*SecurityGateReport, error)
}

// SignatureVerifier port for validating cryptographic image signatures.
type SignatureVerifier interface {
	VerifyImage(ctx context.Context, image string, publicKeyPath string) (bool, error)
}

// SecurityAlertNotifier port for alerting SREs on critical policy breaches.
type SecurityAlertNotifier interface {
	SendP1Alert(ctx context.Context, title string, message string, report *SecurityGateReport) error
}
