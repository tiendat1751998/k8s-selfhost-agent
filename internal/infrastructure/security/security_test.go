package security_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/infrastructure/security"
)

func TestTrivyScanner_ParseAndGate(t *testing.T) {
	scanner := security.NewTrivyScanner()

	sampleJSON := []byte(`{
		"Results": [
			{
				"Target": "nginx:1.25.1 (debian 12.0)",
				"Vulnerabilities": [
					{
						"VulnerabilityID": "CVE-2023-44487",
						"PkgName": "libnghttp2-14",
						"InstalledVersion": "1.52.0-1",
						"FixedVersion": "1.52.0-1+deb12u1",
						"Severity": "CRITICAL",
						"Title": "HTTP/2 Rapid Reset vulnerability"
					},
					{
						"VulnerabilityID": "CVE-2023-38545",
						"PkgName": "curl",
						"InstalledVersion": "7.88.1-10",
						"FixedVersion": "7.88.1-10+deb12u4",
						"Severity": "HIGH",
						"Title": "SOCKS5 heap buffer overflow"
					}
				]
			}
		]
	}`)

	report, err := scanner.ParseTrivyJSON(sampleJSON, "nginx:1.25.1", "HIGH", time.Now())
	if err != nil {
		t.Fatalf("failed to parse trivy json: %v", err)
	}

	if report.Summary.CriticalCount != 1 {
		t.Errorf("expected 1 CRITICAL CVE, got %d", report.Summary.CriticalCount)
	}
	if report.Summary.HighCount != 1 {
		t.Errorf("expected 1 HIGH CVE, got %d", report.Summary.HighCount)
	}
	if report.Summary.TotalCVEs != 2 {
		t.Errorf("expected 2 total CVEs, got %d", report.Summary.TotalCVEs)
	}
	if report.PassSecurityGate {
		t.Error("expected PassSecurityGate to be FALSE due to CRITICAL CVE when max allowed is HIGH")
	}

	summaryStr := scanner.FormatSARIFSummary(report)
	if summaryStr == "" {
		t.Error("expected non-empty summary string")
	}
}

func TestCheckovScanner_ParseAndScore(t *testing.T) {
	scanner := security.NewCheckovScanner()

	sampleJSON := []byte(`{
		"summary": {
			"passed": 8,
			"failed": 2
		},
		"results": {
			"passed_checks": [
				{
					"check_id": "CKV_K8S_10",
					"check_name": "CPU requests should be set",
					"resource": "Deployment.default.nginx",
					"file_path": "/deploy/nginx.yaml",
					"guideline": "https://guide.bridgecrew.io"
				}
			],
			"failed_checks": [
				{
					"check_id": "CKV_K8S_14",
					"check_name": "Containers should not run as root",
					"resource": "Deployment.default.nginx",
					"file_path": "/deploy/nginx.yaml",
					"severity": "HIGH",
					"guideline": "https://guide.bridgecrew.io"
				}
			]
		}
	}`)

	report, err := scanner.ParseCheckovJSON(sampleJSON, "kubernetes", time.Now())
	if err != nil {
		t.Fatalf("failed to parse checkov JSON: %v", err)
	}

	if report.PassedChecks != 8 {
		t.Errorf("expected 8 passed checks, got %d", report.PassedChecks)
	}
	if report.FailedChecks != 2 {
		t.Errorf("expected 2 failed checks, got %d", report.FailedChecks)
	}
	if report.ScorePercentage != 80.0 {
		t.Errorf("expected 80.0%% score percentage, got %f", report.ScorePercentage)
	}
	if len(report.Checks) != 2 {
		t.Errorf("expected 2 checks in details list, got %d", len(report.Checks))
	}
	if report.Passed {
		t.Error("expected report.Passed == false when failed checks > 0")
	}
	if report.PassSecurityGate {
		t.Error("expected report.PassSecurityGate == false when failed checks > 0")
	}
	if report.Status != security.GateStatusPolicyViolation {
		t.Errorf("expected status %s, got %s", security.GateStatusPolicyViolation, report.Status)
	}

	domainReport := report.ToDomainReport("test-scan-123")
	if domainReport.Passed {
		t.Error("expected domain report Passed == false")
	}
	if len(domainReport.FailedChecks) != 1 {
		t.Errorf("expected 1 failed check in domain report, got %d", len(domainReport.FailedChecks))
	}
}

func TestVaultClient_MockOperations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Vault-Token")
		if token != "valid-test-token" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if r.URL.Path == "/v1/secret/data/production/database" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"data": {
					"data": {
						"username": "postgres_admin",
						"password": "super_secret_db_pass"
					}
				}
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := security.VaultConfig{
		Address: server.URL,
		Token:   "valid-test-token",
	}

	client := security.NewVaultClient(cfg)
	secret, err := client.ReadSecret(context.Background(), "secret", "production/database")
	if err != nil {
		t.Fatalf("unexpected error reading secret: %v", err)
	}

	if secret["username"] != "postgres_admin" {
		t.Errorf("expected username 'postgres_admin', got '%v'", secret["username"])
	}
	if secret["password"] != "super_secret_db_pass" {
		t.Errorf("expected password 'super_secret_db_pass', got '%v'", secret["password"])
	}
}

func TestCheckov_BinaryMissing_FailClose(t *testing.T) {
	scanner := security.NewCheckovScannerWithBinary("non_existent_checkov_binary_path_12345")
	ctx := context.Background()

	report, err := scanner.ScanManifest(ctx, "/path/to/manifest.yaml")
	if err == nil {
		t.Fatal("expected non-nil error when checkov binary is missing")
	}
	if report == nil {
		t.Fatal("expected non-nil fail-close report")
	}
	if report.Passed {
		t.Errorf("expected report.Passed == false on binary missing, got true")
	}
	if report.Status != security.GateStatusScannerUnavailable {
		t.Errorf("expected Status == %s, got %s", security.GateStatusScannerUnavailable, report.Status)
	}
	if report.ScorePercentage == 100.0 && report.PassedChecks > 0 {
		t.Errorf("detected fake 100%% pass fallback report: PassedChecks=%d, Score=%f", report.PassedChecks, report.ScorePercentage)
	}
}

func TestTrivy_ExecutionError_FailClose(t *testing.T) {
	scanner := security.NewTrivyScannerWithBinary("non_existent_trivy_binary_path_12345")
	ctx := context.Background()

	report, err := scanner.ScanImage(ctx, "k8sselfhost/backend:latest", "HIGH")
	if err == nil {
		t.Fatal("expected non-nil error when trivy execution fails")
	}
	if report == nil {
		t.Fatal("expected non-nil fail-close report")
	}
	if report.Passed {
		t.Errorf("expected report.Passed == false on execution failure, got true")
	}
	if report.PassSecurityGate {
		t.Errorf("expected report.PassSecurityGate == false on execution failure, got true")
	}
	if report.Status != security.GateStatusScannerUnavailable {
		t.Errorf("expected Status == %s, got %s", security.GateStatusScannerUnavailable, report.Status)
	}
}

func TestCosign_BinaryMissing_FailClose(t *testing.T) {
	verifier := security.NewCosignVerifierWithBinary("non_existent_cosign_binary_path_12345")
	ctx := context.Background()

	ok, err := verifier.VerifyImage(ctx, "k8sselfhost/backend:latest", "/tmp/cosign.pub")
	if err == nil {
		t.Fatal("expected error when cosign binary is missing")
	}
	if ok {
		t.Errorf("expected verify to return false when cosign binary fails")
	}
}

func TestCheckov_ParseInvalidJSON_FailClose(t *testing.T) {
	scanner := security.NewCheckovScanner()

	report, err := scanner.ParseCheckovJSON([]byte(`{not-valid-json`), "kubernetes", time.Now())
	if err == nil {
		t.Fatal("expected error parsing invalid checkov JSON")
	}
	if report == nil {
		t.Fatal("expected non-nil fail-close report")
	}
	if report.Passed || report.PassSecurityGate {
		t.Errorf("expected Passed and PassSecurityGate to be false on invalid JSON")
	}
	if report.ScorePercentage != 0.0 {
		t.Errorf("expected ScorePercentage == 0.0, got %f", report.ScorePercentage)
	}
}

func TestCheckov_ZeroChecks_FailClose(t *testing.T) {
	scanner := security.NewCheckovScanner()

	emptyJSON := []byte(`{"summary": {"passed": 0, "failed": 0}, "results": {}}`)
	report, err := scanner.ParseCheckovJSON(emptyJSON, "kubernetes", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Passed || report.PassSecurityGate {
		t.Errorf("expected zero checks scan to fail security gate, got Passed=%v, PassGate=%v", report.Passed, report.PassSecurityGate)
	}
	if report.ScorePercentage != 0.0 {
		t.Errorf("expected ScorePercentage == 0.0, got %f", report.ScorePercentage)
	}
}

func TestCheckov_AllPassed(t *testing.T) {
	scanner := security.NewCheckovScanner()

	cleanJSON := []byte(`{
		"summary": {"passed": 5, "failed": 0},
		"results": {
			"passed_checks": [
				{"check_id": "CKV_1", "check_name": "clean", "resource": "res", "file_path": "f.yaml"}
			]
		}
	}`)
	report, err := scanner.ParseCheckovJSON(cleanJSON, "kubernetes", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.Passed || !report.PassSecurityGate {
		t.Errorf("expected clean scan to pass, got Passed=%v, PassGate=%v", report.Passed, report.PassSecurityGate)
	}
	if report.ScorePercentage != 100.0 {
		t.Errorf("expected ScorePercentage == 100.0, got %f", report.ScorePercentage)
	}
}

