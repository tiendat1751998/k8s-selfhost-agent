package security_test

import (
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/security"
)

func TestEvaluateGate_FailClose_ScannerUnavailable(t *testing.T) {
	report := &security.SecurityGateReport{
		ScanID:       "scan-001",
		Target:       "nginx:latest",
		Status:       security.GateStatusScannerUnavailable,
		Passed:       false,
		ErrorMessage: "trivy: binary not found in PATH",
		ScannedAt:    time.Now().UTC(),
	}

	policy := security.DefaultStrictPolicy()
	passed := security.EvaluateGate(report, policy)

	if passed {
		t.Fatal("expected EvaluateGate to return false on scanner unavailable under strict policy")
	}
	if report.Passed {
		t.Errorf("expected report.Passed == false, got true")
	}
	if report.Status != security.GateStatusScannerUnavailable {
		t.Errorf("expected Status == %s, got %s", security.GateStatusScannerUnavailable, report.Status)
	}
}

func TestEvaluateGate_CriticalCVE_BlocksDeployment(t *testing.T) {
	report := &security.SecurityGateReport{
		ScanID:        "scan-002",
		Target:        "redis:6.0",
		Status:        security.GateStatusPassed,
		CriticalCount: 1,
		HighCount:     2,
		Vulnerabilities: []security.Vulnerability{
			{
				VulnerabilityID: "CVE-2023-12345",
				PkgName:         "redis-server",
				Severity:        string(security.SeverityCritical),
				Title:           "Remote Code Execution",
			},
		},
		ScannedAt: time.Now().UTC(),
	}

	policy := security.DefaultStrictPolicy()
	passed := security.EvaluateGate(report, policy)

	if passed {
		t.Fatal("expected EvaluateGate to REJECT deployment when Critical CVE > 0")
	}
	if report.Passed {
		t.Errorf("expected report.Passed == false")
	}
	if report.Status != security.GateStatusPolicyViolation {
		t.Errorf("expected Status == %s, got %s", security.GateStatusPolicyViolation, report.Status)
	}
}

func TestEvaluateGate_CosignUnsigned_Rejects(t *testing.T) {
	report := &security.SecurityGateReport{
		ScanID:         "scan-003",
		Target:         "internal/payment-service:v1.0.0",
		Status:         security.GateStatusPassed,
		CriticalCount:  0,
		HighCount:      0,
		CosignVerified: false,
		ScannedAt:      time.Now().UTC(),
	}

	policy := security.PolicyRules{
		MaxCriticalCVEAllowed: 0,
		MaxHighCVEAllowed:     0,
		RequireCosign:         true,
		Mode:                  security.GateModeStrict,
	}

	passed := security.EvaluateGate(report, policy)

	if passed {
		t.Fatal("expected EvaluateGate to REJECT deployment when Cosign signature is required but missing")
	}
	if report.Passed {
		t.Errorf("expected report.Passed == false")
	}
	if report.Status != security.GateStatusPolicyViolation {
		t.Errorf("expected Status == %s, got %s", security.GateStatusPolicyViolation, report.Status)
	}
}

func TestEvaluateGate_CleanReport_Passes(t *testing.T) {
	report := &security.SecurityGateReport{
		ScanID:         "scan-004",
		Target:         "internal/payment-service:v1.0.0",
		Status:         security.GateStatusPassed,
		CriticalCount:  0,
		HighCount:      0,
		CosignVerified: true,
		ScannedAt:      time.Now().UTC(),
	}

	policy := security.PolicyRules{
		MaxCriticalCVEAllowed: 0,
		MaxHighCVEAllowed:     0,
		RequireCosign:         true,
		Mode:                  security.GateModeStrict,
	}

	passed := security.EvaluateGate(report, policy)

	if !passed {
		t.Fatal("expected EvaluateGate to PASS clean, verified artifact")
	}
	if !report.Passed {
		t.Errorf("expected report.Passed == true")
	}
	if report.Status != security.GateStatusPassed {
		t.Errorf("expected Status == %s, got %s", security.GateStatusPassed, report.Status)
	}
}

func TestEvaluateGate_PermissiveMode_AllowsWarning(t *testing.T) {
	report := &security.SecurityGateReport{
		ScanID:        "scan-005",
		Target:        "dev/app:v0.1",
		Status:        security.GateStatusScannerUnavailable,
		ErrorMessage:  "scanner timed out",
		CriticalCount: 1,
		ScannedAt:     time.Now().UTC(),
	}

	policy := security.PolicyRules{
		MaxCriticalCVEAllowed: 0,
		MaxHighCVEAllowed:     0,
		RequireCosign:         false,
		Mode:                  security.GateModePermissive,
	}

	passed := security.EvaluateGate(report, policy)

	if !passed {
		t.Fatal("expected EvaluateGate to allow pass in permissive mode for local dev")
	}
	if !report.Passed {
		t.Errorf("expected report.Passed == true in permissive mode")
	}
}
