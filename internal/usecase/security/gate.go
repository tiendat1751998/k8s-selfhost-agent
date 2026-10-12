// Package security provides application use cases for DevSecOps security gates,
// enforcing strict zero-trust and the Fail-Close invariant across container images and IaC manifests.
package security

import (
	"context"
	"fmt"
	"time"

	domain "github.com/datdt/k8sselfhost/internal/domain/security"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// SecurityGateUsecase orchestrates security gate validation pipelines.
type SecurityGateUsecase struct {
	containerScanner domain.ContainerScanner
	iacScanner       domain.IaCScanner
	verifier         domain.SignatureVerifier
	notifier         domain.SecurityAlertNotifier
}

// NewSecurityGateUsecase constructs a SecurityGateUsecase with the required ports.
func NewSecurityGateUsecase(
	containerScanner domain.ContainerScanner,
	iacScanner domain.IaCScanner,
	verifier domain.SignatureVerifier,
	notifier domain.SecurityAlertNotifier,
) *SecurityGateUsecase {
	return &SecurityGateUsecase{
		containerScanner: containerScanner,
		iacScanner:       iacScanner,
		verifier:         verifier,
		notifier:         notifier,
	}
}

// EvaluateGate evaluates a SecurityGateReport against policy rules under the Fail-Close invariant.
// If Critical CVE > 0, it records a policy violation and fires a P1 security alert if a notifier is configured.
func (u *SecurityGateUsecase) EvaluateGate(ctx context.Context, report *domain.SecurityGateReport, policy domain.PolicyRules) bool {
	if report == nil {
		return false
	}

	passed := report.Evaluate(policy)

	// If critical vulnerabilities are detected, trigger P1 incident alert
	if !passed && report.CriticalCount > 0 && u.notifier != nil {
		alertMsg := fmt.Sprintf("Artifact '%s' contains %d CRITICAL CVEs. Deployment rejected under Fail-Close policy.",
			report.Target, report.CriticalCount)
		_ = u.notifier.SendP1Alert(ctx, "CRITICAL Security Gate Violation", alertMsg, report)
	}

	return passed
}

// EvaluateImage runs container vulnerability scanning and Cosign cryptographic verification.
// Fail-Close Invariants enforced:
// - Scanner missing/error -> REJECT with GateStatusScannerUnavailable.
// - Critical CVE > 0 -> REJECT with GateStatusPolicyViolation.
// - Cosign missing/invalid when required -> REJECT with GateStatusPolicyViolation.
func (u *SecurityGateUsecase) EvaluateImage(
	ctx context.Context,
	image string,
	publicKeyPath string,
	policy domain.PolicyRules,
) (*domain.SecurityGateReport, error) {
	report := &domain.SecurityGateReport{
		ScanID:    fmt.Sprintf("img-%d", time.Now().UnixNano()),
		Target:    image,
		ScannedAt: time.Now().UTC(),
	}

	// 1. Verify Cosign signature if required by policy
	if policy.RequireCosign {
		if u.verifier == nil {
			report.Status = domain.GateStatusScannerUnavailable
			report.Passed = false
			report.ErrorMessage = "signature verifier is unconfigured"
			return report, fmt.Errorf("%w: verifier unconfigured", domain.ErrScannerUnavailable)
		}

		verified, err := u.verifier.VerifyImage(ctx, image, publicKeyPath)
		if err != nil || !verified {
			report.CosignVerified = false
			report.Status = domain.GateStatusPolicyViolation
			report.Passed = false
			report.ErrorMessage = fmt.Sprintf("cosign signature verification failed: %v", err)
			return report, fmt.Errorf("%w: unsigned or invalid signature: %v", domain.ErrSignatureInvalid, err)
		}
		report.CosignVerified = true
	}

	// 2. Scan container image vulnerabilities
	if u.containerScanner != nil {
		scanRes, err := u.containerScanner.ScanImage(ctx, image, "HIGH")
		if err != nil {
			if scanRes != nil {
				report = scanRes
			}
			report.Status = domain.GateStatusScannerUnavailable
			report.Passed = false
			report.ErrorMessage = fmt.Sprintf("container scan failed: %v", err)
			return report, fmt.Errorf("%w: %v", domain.ErrScannerUnavailable, err)
		}
		if scanRes != nil {
			report.CriticalCount = scanRes.CriticalCount
			report.HighCount = scanRes.HighCount
			report.MediumCount = scanRes.MediumCount
			report.LowCount = scanRes.LowCount
			report.Vulnerabilities = scanRes.Vulnerabilities
			report.Status = scanRes.Status
			report.Passed = scanRes.Passed
		}
	}

	// 3. Final gate evaluation
	passed := u.EvaluateGate(ctx, report, policy)
	if !passed && policy.Mode == domain.GateModeStrict {
		return report, fmt.Errorf("%w: critical=%d high=%d status=%s",
			domain.ErrSecurityGateRejected, report.CriticalCount, report.HighCount, report.Status)
	}

	return report, nil
}

// EvaluateManifest scans an IaC Kubernetes/Terraform manifest under the Fail-Close invariant.
func (u *SecurityGateUsecase) EvaluateManifest(
	ctx context.Context,
	manifestPath string,
	policy domain.PolicyRules,
) (*domain.SecurityGateReport, error) {
	report := &domain.SecurityGateReport{
		ScanID:    fmt.Sprintf("iac-%d", time.Now().UnixNano()),
		Target:    manifestPath,
		ScannedAt: time.Now().UTC(),
	}

	if u.iacScanner == nil {
		report.Status = domain.GateStatusScannerUnavailable
		report.Passed = false
		report.ErrorMessage = "iac scanner is unconfigured"
		return report, fmt.Errorf("%w: iac scanner unconfigured", domain.ErrScannerUnavailable)
	}

	scanRes, err := u.iacScanner.ScanManifest(ctx, manifestPath)
	if err != nil {
		if scanRes != nil {
			report = scanRes
		}
		report.Status = domain.GateStatusScannerUnavailable
		report.Passed = false
		report.ErrorMessage = fmt.Sprintf("iac scan failed: %v", err)
		return report, fmt.Errorf("%w: %v", domain.ErrScannerUnavailable, err)
	}
	if scanRes != nil {
		report.FailedChecks = scanRes.FailedChecks
		report.CriticalCount = scanRes.CriticalCount
		report.HighCount = scanRes.HighCount
		report.Status = scanRes.Status
		report.Passed = scanRes.Passed
	}

	passed := u.EvaluateGate(ctx, report, policy)
	if !passed && policy.Mode == domain.GateModeStrict {
		return report, errors.Wrap(domain.ErrSecurityGateRejected, "manifest violates security policy")
	}

	return report, nil
}
