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
