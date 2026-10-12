package security_test

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "github.com/datdt/k8sselfhost/internal/domain/security"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/security"
)

// FakeContainerScanner implements domain.ContainerScanner for zero-mock testing.
type FakeContainerScanner struct {
	Report *domain.SecurityGateReport
	Err    error
}

func (f *FakeContainerScanner) ScanImage(ctx context.Context, image string, maxSeverityAllowed string) (*domain.SecurityGateReport, error) {
	if f.Err != nil {
		return f.Report, f.Err
	}
	return f.Report, nil
}

// FakeIaCScanner implements domain.IaCScanner.
type FakeIaCScanner struct {
	Report *domain.SecurityGateReport
	Err    error
}

func (f *FakeIaCScanner) ScanDirectory(ctx context.Context, dirPath string, framework string) (*domain.SecurityGateReport, error) {
	if f.Err != nil {
		return f.Report, f.Err
	}
	return f.Report, nil
}

func (f *FakeIaCScanner) ScanManifest(ctx context.Context, manifestPath string) (*domain.SecurityGateReport, error) {
	if f.Err != nil {
		return f.Report, f.Err
	}
	return f.Report, nil
}

// FakeSignatureVerifier implements domain.SignatureVerifier.
type FakeSignatureVerifier struct {
	Valid bool
	Err   error
}

func (f *FakeSignatureVerifier) VerifyImage(ctx context.Context, image string, publicKeyPath string) (bool, error) {
	return f.Valid, f.Err
}

// RecordingAlertNotifier records triggered P1 alerts.
type RecordingAlertNotifier struct {
	Alerts []string
}

func (r *RecordingAlertNotifier) SendP1Alert(ctx context.Context, title string, message string, report *domain.SecurityGateReport) error {
	r.Alerts = append(r.Alerts, title+": "+message)
	return nil
}

func TestSecurityGateUsecase_EvaluateGate_CriticalCVE_BlocksDeployment(t *testing.T) {
	notifier := &RecordingAlertNotifier{}
	uc := usecase.NewSecurityGateUsecase(nil, nil, nil, notifier)

	report := &domain.SecurityGateReport{
		ScanID:        "scan-critical-1",
		Target:        "registry.corp.net/billing:v2.1.0",
		Status:        domain.GateStatusPassed,
		CriticalCount: 1,
		HighCount:     0,
		Vulnerabilities: []domain.Vulnerability{
			{
				VulnerabilityID: "CVE-2023-99999",
				Severity:        "CRITICAL",
				Title:           "RCE in Auth Library",
			},
		},
		ScannedAt: time.Now().UTC(),
	}

	policy := domain.DefaultStrictPolicy()
	passed := uc.EvaluateGate(context.Background(), report, policy)

	if passed {
		t.Fatal("expected EvaluateGate to return false when Critical CVE > 0")
	}
	if report.Passed {
		t.Errorf("expected report.Passed == false")
	}
	if report.Status != domain.GateStatusPolicyViolation {
		t.Errorf("expected Status == %s, got %s", domain.GateStatusPolicyViolation, report.Status)
	}
	if len(notifier.Alerts) == 0 {
		t.Errorf("expected P1 security alert to be triggered for critical CVE violation")
	}
}

func TestSecurityGateUsecase_EvaluateGate_ScannerUnavailable_FailClose(t *testing.T) {
	uc := usecase.NewSecurityGateUsecase(nil, nil, nil, nil)

	report := &domain.SecurityGateReport{
		ScanID:       "scan-err-1",
		Target:       "registry.corp.net/billing:v2.1.0",
		Status:       domain.GateStatusScannerUnavailable,
		ErrorMessage: "trivy: exit status 1",
		ScannedAt:    time.Now().UTC(),
	}

	policy := domain.DefaultStrictPolicy()
	passed := uc.EvaluateGate(context.Background(), report, policy)

	if passed {
		t.Fatal("expected EvaluateGate to return false when scanner is unavailable")
	}
	if report.Passed {
		t.Errorf("expected report.Passed == false")
	}
	if report.Status != domain.GateStatusScannerUnavailable {
		t.Errorf("expected Status == %s, got %s", domain.GateStatusScannerUnavailable, report.Status)
	}
}

func TestSecurityGateUsecase_EvaluateImage_CosignVerification(t *testing.T) {
	policy := domain.PolicyRules{
		MaxCriticalCVEAllowed: 0,
		MaxHighCVEAllowed:     0,
		RequireCosign:         true,
		Mode:                  domain.GateModeStrict,
	}

	t.Run("unsigned or invalid signature rejects deployment", func(t *testing.T) {
		scanner := &FakeContainerScanner{
			Report: &domain.SecurityGateReport{
				ScanID:        "scan-cosign-1",
				Target:        "redis:6.2",
				CriticalCount: 0,
				Status:        domain.GateStatusPassed,
				ScannedAt:     time.Now().UTC(),
			},
		}
		verifier := &FakeSignatureVerifier{
			Valid: false,
			Err:   errors.New("signature not found"),
		}

		uc := usecase.NewSecurityGateUsecase(scanner, nil, verifier, nil)
		report, err := uc.EvaluateImage(context.Background(), "redis:6.2", "/keys/cosign.pub", policy)

		if err == nil {
			t.Fatal("expected error rejecting unsigned image when Cosign verification required")
		}
		if report == nil {
			t.Fatal("expected non-nil report")
		}
		if report.Passed {
			t.Errorf("expected report.Passed == false")
		}
		if report.CosignVerified {
			t.Errorf("expected report.CosignVerified == false")
		}
		if report.Status != domain.GateStatusPolicyViolation {
			t.Errorf("expected Status == %s, got %s", domain.GateStatusPolicyViolation, report.Status)
		}
	})

	t.Run("valid signature and clean scan passes deployment", func(t *testing.T) {
		scanner := &FakeContainerScanner{
			Report: &domain.SecurityGateReport{
				ScanID:        "scan-cosign-2",
				Target:        "redis:6.2",
				CriticalCount: 0,
				Status:        domain.GateStatusPassed,
				ScannedAt:     time.Now().UTC(),
			},
		}
		verifier := &FakeSignatureVerifier{
			Valid: true,
			Err:   nil,
		}

		uc := usecase.NewSecurityGateUsecase(scanner, nil, verifier, nil)
		report, err := uc.EvaluateImage(context.Background(), "redis:6.2", "/keys/cosign.pub", policy)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !report.Passed {
			t.Errorf("expected report.Passed == true")
		}
		if !report.CosignVerified {
			t.Errorf("expected report.CosignVerified == true")
		}
		if report.Status != domain.GateStatusPassed {
			t.Errorf("expected Status == %s, got %s", domain.GateStatusPassed, report.Status)
		}
	})
}

func TestSecurityGateUsecase_EvaluateManifest_FailClose(t *testing.T) {
	policy := domain.DefaultStrictPolicy()

	t.Run("scanner unavailable rejects manifest", func(t *testing.T) {
		scanner := &FakeIaCScanner{
			Report: &domain.SecurityGateReport{
				ScanID:       "scan-iac-1",
				Target:       "/deploy/app.yaml",
				Status:       domain.GateStatusScannerUnavailable,
				Passed:       false,
				ErrorMessage: "checkov: binary missing",
			},
			Err: errors.New("checkov missing"),
		}

		uc := usecase.NewSecurityGateUsecase(nil, scanner, nil, nil)
		report, err := uc.EvaluateManifest(context.Background(), "/deploy/app.yaml", policy)

		if err == nil {
			t.Fatal("expected error when IaC scanner is unavailable")
		}
		if report.Passed {
			t.Errorf("expected report.Passed == false")
		}
		if report.Status != domain.GateStatusScannerUnavailable {
			t.Errorf("expected Status == %s, got %s", domain.GateStatusScannerUnavailable, report.Status)
		}
	})

	t.Run("clean manifest passes gate", func(t *testing.T) {
		scanner := &FakeIaCScanner{
			Report: &domain.SecurityGateReport{
				ScanID:       "scan-iac-2",
				Target:       "/deploy/clean.yaml",
				Status:       domain.GateStatusPassed,
				Passed:       true,
				FailedChecks: nil,
			},
			Err: nil,
		}

		uc := usecase.NewSecurityGateUsecase(nil, scanner, nil, nil)
		report, err := uc.EvaluateManifest(context.Background(), "/deploy/clean.yaml", policy)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !report.Passed {
			t.Errorf("expected report.Passed == true")
		}
		if report.Status != domain.GateStatusPassed {
			t.Errorf("expected Status == %s, got %s", domain.GateStatusPassed, report.Status)
		}
	})
}
